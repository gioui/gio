// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"io"
	"strings"
	"testing"

	"gioui.org/f32"
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
			r.Queue(DropEvent{
				Position: test.position,
				Type:     "text/uri-list",
				Open: func() io.ReadCloser {
					opened++
					return io.NopCloser(strings.NewReader(content))
				},
			})
			for i, filter := range filters {
				evs := events(&r, -1, filter)
				if filter.Type != "text/uri-list" {
					assertEventSequence(t, evs)
					continue
				}
				if i != test.want {
					assertEventSequence(t, evs, transfer.InitiateEvent{}, transfer.CancelEvent{})
					continue
				}
				if len(evs) != 3 {
					t.Fatalf("target %d received %v; want initiate, data, cancel", i, evs)
				}
				assertEventSequence(t, evs[:1], transfer.InitiateEvent{})
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
	r.Queue(DropEvent{
		Position: f32.Pt(10, 10),
		Type:     filter.Type,
		Open:     func() io.ReadCloser { return reader },
	})
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
	r.Queue(DropEvent{
		Type: filter.Type,
		Open: func() io.ReadCloser {
			reader := &dropReader{Reader: strings.NewReader("file:///tmp/a.txt\r\n")}
			readers = append(readers, reader)
			return reader
		},
	})
	// Executing a command reprocesses pending input before it is consumed.
	r.Source().Execute(op.InvalidateCmd{})
	evs := events(&r, -1, filter)
	if len(evs) != 3 {
		t.Fatalf("received %v; want initiate, data, cancel", evs)
	}
	assertEventSequence(t, evs[:1], transfer.InitiateEvent{})
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
	r.Queue(DropEvent{
		Type: filter.Type,
		Open: func() io.ReadCloser {
			reader := &dropReader{Reader: strings.NewReader("file:///tmp/a.txt\r\n")}
			readers = append(readers, reader)
			return reader
		},
	})
	// Focus was already polled, so this command must wait for the next frame.
	r.Source().Execute(key.FocusCmd{Tag: &tag})
	assertEventSequence(t, events(&r, -1, filter))
	r.Frame(&ops)
	evs := events(&r, -1, filter)
	if len(evs) != 3 {
		t.Fatalf("received %v; want initiate, data, cancel", evs)
	}
	assertEventSequence(t, evs[:1], transfer.InitiateEvent{})
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

type dropReader struct {
	*strings.Reader
	closed bool
}

func (r *dropReader) Close() error {
	r.closed = true
	return nil
}
