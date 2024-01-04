package widget

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/io/router"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func TestDraggable(t *testing.T) {
	var r router.Router
	gtx := layout.Context{
		Constraints: layout.Exact(image.Pt(100, 100)),
		Queue:       &r,
		Ops:         new(op.Ops),
	}

	drag := &Draggable{
		Type: "file",
	}
	defer pointer.PassOp{}.Push(gtx.Ops).Pop()
	dims := drag.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	}, nil)
	stack := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	transfer.TargetOp{
		Tag:  drag,
		Type: drag.Type,
	}.Add(gtx.Ops)
	stack.Pop()

	r.Frame(gtx.Ops)
	r.Queue(
		pointer.Event{
			Position: f32.Pt(10, 10),
			Kind:     pointer.Press,
		},
		pointer.Event{
			Position: f32.Pt(20, 10),
			Kind:     pointer.Move,
		},
		pointer.Event{
			Position: f32.Pt(20, 10),
			Kind:     pointer.Release,
		},
	)
	ofr := &offer{data: "hello"}
	drag.Offer(gtx.Ops, "file", ofr)
	r.Frame(gtx.Ops)

	evs := r.Events(drag)
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	ev := evs[0].(transfer.DataEvent)
	ev.Open = nil
	if got, want := ev.Type, "file"; got != want {
		t.Errorf("expected %v; got %v", got, want)
	}
	if ofr.closed {
		t.Error("offer closed prematurely")
	}
	r.Frame(gtx.Ops)
	if !ofr.closed {
		t.Error("offer was not closed")
	}
}

// offer satisfies io.ReadCloser for use in data transfers.
type offer struct {
	data   string
	closed bool
}

func (*offer) Read([]byte) (int, error) { return 0, nil }

func (o *offer) Close() error {
	o.closed = true
	return nil
}
