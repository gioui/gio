// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"io"
	"slices"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func TestNoFilterAllocs(t *testing.T) {
	b := testing.Benchmark(func(b *testing.B) {
		var r Router
		s := r.Source()
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			s.Event(pointer.Filter{})
		}
	})
	if allocs := b.AllocsPerOp(); allocs != 0 {
		t.Fatalf("expected 0 AllocsPerOp, got %d", allocs)
	}
}

func TestRouterWakeup(t *testing.T) {
	r := new(Router)
	r.Source().Execute(op.InvalidateCmd{})
	r.Frame(new(op.Ops))
	if _, wake := r.WakeupTime(); !wake {
		t.Errorf("InvalidateCmd did not trigger a redraw")
	}
}

func TestRouterAcceptsTransfer(t *testing.T) {
	var r Router
	var ops op.Ops
	var tag int
	for _, test := range []struct {
		name   string
		filter event.Filter
		area   bool
		want   bool
	}{
		{name: "empty"},
		{name: "source", filter: transfer.SourceFilter{Target: &tag, Type: "text/uri-list"}, area: true},
		{name: "target without area", filter: transfer.TargetFilter{Target: &tag, Type: "text/uri-list"}},
		{name: "file target", filter: transfer.TargetFilter{Target: &tag, Type: "text/uri-list"}, area: true, want: true},
		{name: "text target", filter: transfer.TargetFilter{Target: &tag, Type: "text/plain"}, area: true},
		{name: "file target returns", filter: transfer.TargetFilter{Target: &tag, Type: "text/uri-list"}, area: true, want: true},
		{name: "filter removed", area: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ops.Reset()
			if test.filter != nil {
				r.Event(test.filter)
			}
			if test.area {
				event.Op(&ops, &tag)
			}
			r.Frame(&ops)
			if got := r.AcceptsTransfer("text/uri-list"); got != test.want {
				t.Errorf("AcceptsTransfer = %v, want %v", got, test.want)
			}
		})
	}
}

// drop returns the events a platform queues when data of type mime
// is dropped at pos from another application.
func drop(pos f32.Point, mime string, open func() io.ReadCloser) []event.Event {
	return []event.Event{
		transfer.InitiateEvent{Types: []string{mime}},
		transfer.DataEvent{ExternalDrop: true, Position: pos, Type: mime, Open: open},
	}
}

func TestExternalDrop(t *testing.T) {
	for _, test := range []struct {
		name       string
		position   f32.Point
		overlap    bool
		pass       bool
		secondType string
		want       int
	}{
		{name: "left", position: f32.Pt(30, 30), want: 0},
		{name: "right", position: f32.Pt(80, 30), want: 1},
		{name: "outside", position: f32.Pt(130, 30), want: -1},
		{name: "clipped", position: f32.Pt(110, 30), want: -1},
		{name: "mismatch", position: f32.Pt(80, 30), secondType: "text/plain", want: -1},
		{name: "overlap", position: f32.Pt(30, 30), overlap: true, want: 1},
		{name: "pass matching", position: f32.Pt(30, 30), overlap: true, pass: true, want: 1},
		{name: "opaque mismatch", position: f32.Pt(30, 30), overlap: true, secondType: "text/plain", want: -1},
		{name: "pass mismatch", position: f32.Pt(30, 30), overlap: true, pass: true, secondType: "text/plain", want: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var r Router
			var ops op.Ops
			var tags [2]int
			filters := [2]transfer.TargetFilter{
				{Target: &tags[0], Type: "text/uri-list"},
				{Target: &tags[1], Type: "text/uri-list"},
			}
			if test.secondType != "" {
				filters[1].Type = test.secondType
			}
			// Drop positions are in window coordinates, even inside an offset clip.
			offset := op.Offset(image.Pt(20, 20)).Push(&ops)
			parent := clip.Rect(image.Rect(0, 0, 80, 50)).Push(&ops)
			for i := range tags {
				x := i * 50
				if test.overlap {
					x = 0
				}
				var pass pointer.PassStack
				if i == 1 && test.pass {
					pass = pointer.PassOp{}.Push(&ops)
				}
				area := clip.Rect(image.Rect(x, 0, x+50, 50)).Push(&ops)
				event.Op(&ops, &tags[i])
				area.Pop()
				if i == 1 && test.pass {
					pass.Pop()
				}
				r.Event(filters[i])
			}
			parent.Pop()
			offset.Pop()
			r.Frame(&ops)
			const content = "file:///tmp/a.txt\r\nfile:///tmp/b.png\r\n"
			opened := 0
			if got, want := r.AcceptsTransferAt("text/uri-list", test.position), test.want >= 0; got != want {
				t.Errorf("AcceptsTransferAt = %v, want %v", got, want)
			}
			r.Queue(drop(test.position, "text/uri-list", func() io.ReadCloser {
				opened++
				return io.NopCloser(strings.NewReader(content))
			})...)
			for i, filter := range filters {
				evs := events(&r, -1, filter)
				if filter.Type != "text/uri-list" {
					assertEventSequence(t, evs)
					continue
				}
				if i != test.want {
					if i == 1 && filter.Type != "text/uri-list" {
						assertEventSequence(t, evs)
					} else {
						assertEventSequence(t, evs, transfer.InitiateEvent{Types: []string{"text/uri-list"}}, transfer.CancelEvent{})
					}
					continue
				}
				if len(evs) != 3 {
					t.Fatalf("target %d received %v; want initiate, data, cancel", i, evs)
				}
				assertEventSequence(t, evs[:1], transfer.InitiateEvent{Types: []string{"text/uri-list"}})
				assertEventSequence(t, evs[2:], transfer.CancelEvent{})
				de, ok := evs[1].(transfer.DataEvent)
				if !ok || de.Type != "text/uri-list" {
					t.Fatalf("unexpected event: %#v", evs[1])
				}
				reader := de.Open()
				data, err := io.ReadAll(reader)
				if err := reader.Close(); err != nil {
					t.Fatal(err)
				}
				if err != nil || string(data) != content {
					t.Fatalf("unexpected data: %q, %v", data, err)
				}
			}
			wantOpened := 0
			if test.want >= 0 {
				wantOpened = 1
			}
			if opened != wantOpened {
				t.Fatalf("opened %d readers, want %d", opened, wantOpened)
			}
		})
	}
}

func TestExternalDropUnclaimed(t *testing.T) {
	var r Router
	var ops op.Ops
	var tag int
	filter := transfer.TargetFilter{Target: &tag, Type: "text/uri-list"}
	area := clip.Rect(image.Rect(0, 0, 50, 50)).Push(&ops)
	event.Op(&ops, &tag)
	area.Pop()
	r.Event(filter)
	r.Frame(&ops)
	reader := &dropReader{Reader: strings.NewReader("file:///tmp/a.txt\r\n")}
	r.Queue(drop(f32.Pt(10, 10), filter.Type, func() io.ReadCloser { return reader })...)
	if reader.closed {
		t.Fatal("reader closed before the receiving frame")
	}
	evs := events(&r, -1, filter) // Leave the data unread.
	if len(evs) != 3 {
		t.Fatalf("received %v; want initiate, data, cancel", evs)
	}
	r.Frame(&ops)
	if !reader.closed {
		t.Fatal("unclaimed reader was not closed")
	}
}

func TestExternalDropReplay(t *testing.T) {
	var r Router
	var ops op.Ops
	var tag int
	filter := transfer.TargetFilter{Target: &tag, Type: "text/uri-list"}
	event.Op(&ops, &tag)
	r.Event(filter)
	r.Frame(&ops)
	var readers []*dropReader
	r.Queue(drop(f32.Pt(0, 0), filter.Type, func() io.ReadCloser {
		reader := &dropReader{Reader: strings.NewReader("file:///tmp/a.txt\r\n")}
		readers = append(readers, reader)
		return reader
	})...)
	// Executing a command reprocesses pending input before it is consumed.
	r.Source().Execute(op.InvalidateCmd{})
	evs := events(&r, -1, filter)
	if len(evs) != 3 {
		t.Fatalf("received %v; want initiate, data, cancel", evs)
	}
	assertEventSequence(t, evs[:1], transfer.InitiateEvent{Types: []string{"text/uri-list"}})
	assertEventSequence(t, evs[2:], transfer.CancelEvent{})
	reader := evs[1].(transfer.DataEvent).Open()
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "file:///tmp/a.txt\r\n" {
		t.Fatalf("unexpected replayed data: %q, %v", data, err)
	}
	if _, ok := r.Event(filter); ok {
		t.Fatal("replayed drop delivered twice")
	}
	r.Frame(&ops)
	if len(readers) != 2 || !readers[0].closed || readers[1].closed {
		t.Fatal("replay must close the unused reader and preserve the claimed reader")
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExternalDropDeferred(t *testing.T) {
	var r Router
	var ops op.Ops
	var tag int
	filter := transfer.TargetFilter{Target: &tag, Type: "text/uri-list"}
	focus := key.FocusFilter{Target: &tag}
	event.Op(&ops, &tag)
	events(&r, -1, filter, focus)
	r.Frame(&ops)
	events(&r, -1, focus)
	var readers []*dropReader
	r.Queue(drop(f32.Pt(0, 0), filter.Type, func() io.ReadCloser {
		reader := &dropReader{Reader: strings.NewReader("file:///tmp/a.txt\r\n")}
		readers = append(readers, reader)
		return reader
	})...)
	// Focus was already polled, so this command must wait for the next frame.
	r.Source().Execute(key.FocusCmd{Tag: &tag})
	assertEventSequence(t, events(&r, -1, filter))
	r.Frame(&ops)
	evs := events(&r, -1, filter)
	if len(evs) != 3 {
		t.Fatalf("received %v; want initiate, data, cancel", evs)
	}
	assertEventSequence(t, evs[:1], transfer.InitiateEvent{Types: []string{"text/uri-list"}})
	assertEventSequence(t, evs[2:], transfer.CancelEvent{})
	reader := evs[1].(transfer.DataEvent).Open()
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != "file:///tmp/a.txt\r\n" {
		t.Fatalf("deferred data = %q, %v", data, err)
	}
	if len(readers) != 2 || !readers[0].closed || readers[1].closed {
		t.Fatal("deferral must close the unused reader and preserve the claimed reader")
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClipboardDuringExternalDrag(t *testing.T) {
	var r Router
	var ops op.Ops
	var target, receiver int
	const mime = "text/uri-list"
	dropFilter := transfer.TargetFilter{Target: &target, Type: mime}
	clipFilter := transfer.TargetFilter{Target: &receiver, Type: mime}
	event.Op(&ops, &target)
	events(&r, -1, dropFilter)
	r.Frame(&ops)
	r.Source().Execute(clipboard.ReadCmd{Tag: &receiver})
	evts := drop(f32.Pt(5, 5), mime, payload("drop"))
	// The clipboard data arrives while the file is dragged.
	evts = slices.Insert(evts, 1, event.Event(transfer.DataEvent{Type: mime, Open: payload("clipboard")}))
	r.Queue(evts...)
	clip := events(&r, -1, clipFilter)
	assertEventTypeSequence(t, clip, transfer.DataEvent{})
	dropped := events(&r, -1, dropFilter)
	assertEventTypeSequence(t, dropped, transfer.InitiateEvent{}, transfer.DataEvent{}, transfer.CancelEvent{})
	if t.Failed() {
		return
	}
	assertData(t, clip[0], "clipboard")
	assertData(t, dropped[1], "drop")
}

func TestExternalTransferCancel(t *testing.T) {
	var r Router
	var ops op.Ops
	var target int
	const mime = "text/uri-list"
	targetFilter := transfer.TargetFilter{Target: &target, Type: mime}
	hoverFilter := pointer.Filter{Target: &target, Kinds: pointer.Enter | pointer.Leave | pointer.Drag}
	area := clip.Rect(image.Rect(0, 0, 10, 10)).Push(&ops)
	event.Op(&ops, &target)
	area.Pop()
	events(&r, -1, targetFilter, hoverFilter)
	r.Frame(&ops)
	start := transfer.InitiateEvent{Types: []string{mime}}
	move := pointer.Event{Kind: pointer.Drag, Source: pointer.Mouse, Position: f32.Pt(5, 5)}
	leave := pointer.Event{Kind: pointer.Drag, Source: pointer.Mouse, Position: f32.Pt(15, 5)}
	r.Queue(start, move, leave, transfer.CancelEvent{})
	assertEventTypeSequence(t, events(&r, -1, targetFilter), transfer.InitiateEvent{}, transfer.CancelEvent{})
	assertEventPointerTypeSequence(t, events(&r, -1, hoverFilter), pointer.Enter, pointer.Drag, pointer.Leave)
	// A platform drag after cancellation is ignored; regular movement still routes.
	r.Queue(move)
	assertEventSequence(t, events(&r, -1, hoverFilter))
	r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(5, 5)})
	assertEventPointerTypeSequence(t, events(&r, -1, hoverFilter), pointer.Enter)
	r.Queue(start, move)
	assertEventTypeSequence(t, events(&r, -1, targetFilter), transfer.InitiateEvent{})
	assertEventPointerTypeSequence(t, events(&r, -1, hoverFilter), pointer.Enter, pointer.Drag)
}

func TestExternalTransferMIMEIntersection(t *testing.T) {
	var r Router
	var ops op.Ops
	var imageTarget, textTarget int
	imageFilter := transfer.TargetFilter{Target: &imageTarget, Type: "image/png"}
	textFilter := transfer.TargetFilter{Target: &textTarget, Type: "text/plain"}
	event.Op(&ops, &imageTarget)
	event.Op(&ops, &textTarget)
	events(&r, -1, imageFilter, textFilter)
	r.Frame(&ops)
	r.Queue(transfer.InitiateEvent{Types: []string{"text/plain", "image/png", "image/png"}})
	assertEventSequence(t, events(&r, -1, imageFilter), transfer.InitiateEvent{Types: []string{"image/png"}})
	assertEventSequence(t, events(&r, -1, textFilter), transfer.InitiateEvent{Types: []string{"text/plain"}})
	r.Queue(transfer.CancelEvent{})
	assertEventTypeSequence(t, events(&r, -1, imageFilter), transfer.CancelEvent{})
	assertEventTypeSequence(t, events(&r, -1, textFilter), transfer.CancelEvent{})
}

func TestPlatformDrag(t *testing.T) {
	const mime = "text/plain"
	for _, test := range []string{"same window", "another window", "release before handoff", "source removed", "cancel"} {
		t.Run(test, func(t *testing.T) {
			sourceWindow, receiver, source, target, ops := platformDragSetup(test == "another window", mime)
			sourceWindow.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(5, 5)}, pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(15, 5)})
			assertEventSequence(t, events(sourceWindow, -1, source), transfer.RequestEvent{Type: mime})
			assertEventSequence(t, events(receiver, -1, target))
			sourceWindow.Source().Execute(transfer.OfferCmd{Tag: source.Target, Type: mime, Data: payload("hello")()})
			if test == "release before handoff" {
				sourceWindow.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(25, 5)})
			}
			if test == "source removed" {
				ops.Reset()
			}
			sourceWindow.Frame(ops)
			data := sourceWindow.TakeDrag()
			if test == "release before handoff" || test == "source removed" {
				if data != nil {
					t.Fatalf("started drag after %s: %v", test, data)
				}
				return
			}
			if string(data[mime]) != "hello" || sourceWindow.TakeDrag() != nil {
				t.Fatal("expected a single platform handoff")
			}
			if test != "cancel" {
				receiver.Queue(drop(f32.Pt(25, 5), mime, payload(string(data[mime])))...)
				evts := events(receiver, -1, target)
				assertEventTypeSequence(t, evts, transfer.InitiateEvent{}, transfer.DataEvent{}, transfer.CancelEvent{})
				assertData(t, evts[1], "hello")
			}
			sourceWindow.EndDrag()
			sourceWindow.EndDrag()
			assertEventSequence(t, events(sourceWindow, -1, source), transfer.CancelEvent{})
			assertEventSequence(t, events(receiver, -1, target))
		})
	}
}

func platformDragSetup(anotherWindow bool, mime string) (*Router, *Router, transfer.SourceFilter, transfer.TargetFilter, *op.Ops) {
	sourceWindow := new(Router)
	sourceWindow.SetPlatformDrags(true)
	receiver := sourceWindow
	if anotherWindow {
		receiver = new(Router)
	}
	source := transfer.SourceFilter{Target: new(int), Type: mime}
	target := transfer.TargetFilter{Target: new(int), Type: mime}
	sourceOps := new(op.Ops)
	area := clip.Rect(image.Rect(0, 0, 10, 10)).Push(sourceOps)
	event.Op(sourceOps, source.Target)
	area.Pop()
	targetOps := sourceOps
	if anotherWindow {
		targetOps = new(op.Ops)
	}
	receiver.Event(target)
	area = clip.Rect(image.Rect(20, 0, 30, 10)).Push(targetOps)
	event.Op(targetOps, target.Target)
	area.Pop()
	if anotherWindow {
		receiver.Frame(targetOps)
	}
	sourceWindow.Event(source)
	sourceWindow.Frame(sourceOps)
	return sourceWindow, receiver, source, target, sourceOps
}

func TestExternalTransferTargetsUpdateWithFrames(t *testing.T) {
	var r Router
	var ops op.Ops
	var first, second int
	firstFilter := transfer.TargetFilter{Target: &first, Type: "file"}
	secondFilter := transfer.TargetFilter{Target: &second, Type: "file"}
	event.Op(&ops, &first)
	r.Event(firstFilter)
	r.Frame(&ops)
	r.Queue(transfer.InitiateEvent{Types: []string{"file"}})
	assertEventTypeSequence(t, events(&r, -1, firstFilter), transfer.InitiateEvent{})

	ops.Reset()
	event.Op(&ops, &second)
	r.Event(firstFilter)
	r.Event(secondFilter)
	r.Frame(&ops)
	assertEventTypeSequence(t, events(&r, -1, firstFilter), transfer.CancelEvent{})
	assertEventTypeSequence(t, events(&r, -1, secondFilter), transfer.InitiateEvent{})

	r.Queue(transfer.CancelEvent{})
	assertEventTypeSequence(t, events(&r, -1, secondFilter), transfer.CancelEvent{})
}

func TestExternalDropPointerHover(t *testing.T) {
	var r Router
	var ops op.Ops
	var target int
	const mime = "text/uri-list"
	filter := transfer.TargetFilter{Target: &target, Type: mime}
	hoverFilter := pointer.Filter{Target: &target, Kinds: pointer.Enter | pointer.Leave | pointer.Drag}
	area := clip.Rect(image.Rect(0, 0, 10, 10)).Push(&ops)
	event.Op(&ops, &target)
	area.Pop()
	events(&r, -1, filter, hoverFilter)
	r.Frame(&ops)
	evts := drop(f32.Pt(5, 5), mime, payload("file"))
	evts = slices.Insert(evts, 1,
		event.Event(pointer.Event{Kind: pointer.Drag, Source: pointer.Mouse, Position: f32.Pt(5, 5)}),
		event.Event(pointer.Event{Kind: pointer.Drag, Source: pointer.Mouse, Position: f32.Pt(15, 5)}),
		event.Event(pointer.Event{Kind: pointer.Drag, Source: pointer.Mouse, Position: f32.Pt(5, 5)}),
		event.Event(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Position: f32.Pt(15, 5)}),
		event.Event(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(15, 5)}),
		event.Event(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(15, 5)}),
	)
	r.Queue(evts...)
	evs := events(&r, -1, filter)
	assertEventTypeSequence(t, evs, transfer.InitiateEvent{}, transfer.DataEvent{}, transfer.CancelEvent{})
	if !t.Failed() {
		assertData(t, evs[1], "file")
	}
	assertEventPointerTypeSequence(t, events(&r, -1, hoverFilter), pointer.Enter, pointer.Drag, pointer.Leave, pointer.Enter, pointer.Drag, pointer.Leave)
}

// payload returns a DataEvent.Open function for data.
func payload(data string) func() io.ReadCloser {
	return func() io.ReadCloser {
		return io.NopCloser(strings.NewReader(data))
	}
}

func assertData(t *testing.T, e event.Event, want string) {
	t.Helper()
	r := e.(transfer.DataEvent).Open()
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil || string(data) != want {
		t.Errorf("data = %q, %v; want %q", data, err, want)
	}
}

type dropReader struct {
	*strings.Reader
	closed bool
}

func (r *dropReader) Close() error {
	r.closed = true
	return nil
}
