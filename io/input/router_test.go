// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"io"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
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

func TestExternalDropReplay(t *testing.T) {
	var router Router
	var ops op.Ops
	var left, right int
	const mime = "text/uri-list"
	leftFilter := transfer.TargetFilter{Target: &left, Type: mime}
	rightFilter := transfer.TargetFilter{Target: &right, Type: mime}
	leftArea := clip.Rect(image.Rect(0, 0, 10, 10)).Push(&ops)
	event.Op(&ops, &left)
	leftArea.Pop()
	rightArea := clip.Rect(image.Rect(10, 0, 20, 10)).Push(&ops)
	event.Op(&ops, &right)
	rightArea.Pop()
	router.Event(leftFilter, rightFilter)
	router.Frame(&ops)
	var readers []*dropReader
	const payload = "file:///tmp/a.txt\r\n"
	router.Queue(
		transfer.InitiateEvent{Types: []string{mime}},
		pointer.Event{Kind: pointer.Drag, Source: pointer.Mouse, Position: f32.Pt(15, 5)},
		transfer.DataEvent{Type: mime, Open: func() io.ReadCloser {
			reader := &dropReader{Reader: strings.NewReader(payload)}
			readers = append(readers, reader)
			return reader
		}},
	)
	// Replaying pending input must preserve the last drag position.
	router.Source().Execute(op.InvalidateCmd{})
	assertEventTypeSequence(t, events(&router, -1, leftFilter), transfer.InitiateEvent{}, transfer.CancelEvent{})
	received := events(&router, -1, rightFilter)
	assertEventTypeSequence(t, received, transfer.InitiateEvent{}, transfer.DataEvent{}, transfer.CancelEvent{})
	reader := received[1].(transfer.DataEvent).Open()
	data, err := io.ReadAll(reader)
	if err != nil || string(data) != payload {
		t.Fatalf("replayed drop data = %q, %v; want %q", data, err, payload)
	}
	router.Frame(&ops)
	if len(readers) != 2 || !readers[0].closed || readers[1].closed {
		t.Fatal("replay must close the unused reader and preserve the claimed reader")
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExternalDragAfterCancel(t *testing.T) {
	var router Router
	var ops op.Ops
	var target int
	const mime = "text/uri-list"
	targetFilter := transfer.TargetFilter{Target: &target, Type: mime}
	pointerFilter := pointer.Filter{Target: &target, Kinds: pointer.Enter | pointer.Leave | pointer.Drag}
	event.Op(&ops, &target)
	events(&router, -1, targetFilter, pointerFilter)
	router.Frame(&ops)
	start := transfer.InitiateEvent{Types: []string{mime}}
	motion := pointer.Event{Kind: pointer.Drag, Source: pointer.Mouse, Position: f32.Pt(5, 5)}
	router.Queue(start, motion, transfer.CancelEvent{})
	assertEventTypeSequence(t, events(&router, -1, targetFilter), transfer.InitiateEvent{}, transfer.CancelEvent{})
	assertPointerKinds := func(want ...pointer.Kind) {
		t.Helper()
		received := events(&router, -1, pointerFilter)
		if len(received) != len(want) {
			t.Fatalf("pointer events = %v, want kinds %v", received, want)
		}
		for i, received := range received {
			if received.(pointer.Event).Kind != want[i] {
				t.Fatalf("pointer events = %v, want kinds %v", received, want)
			}
		}
	}
	assertPointerKinds(pointer.Enter, pointer.Drag, pointer.Leave)
	router.Queue(motion)
	assertPointerKinds()
	router.Queue(start, motion)
	assertEventTypeSequence(t, events(&router, -1, targetFilter), transfer.InitiateEvent{})
	assertPointerKinds(pointer.Enter, pointer.Drag)
}

type dropReader struct {
	*strings.Reader
	closed bool
}

func (r *dropReader) Close() error {
	r.closed = true
	return nil
}
