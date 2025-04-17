package widget

import (
	"gioui.org/io/pointer"
	"io"
	"iter"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// Draggable makes a widget draggable.
type Draggable struct {
	// Type contains the MIME type and matches transfer.SourceOp.
	Type string

	drag  gesture.Drag
	click f32.Point
	pos   f32.Point
}

func (d *Draggable) Layout(gtx layout.Context, w, drag layout.Widget) layout.Dimensions {
	if !gtx.Enabled() {
		return w(gtx)
	}
	dims := w(gtx)

	stack := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	d.drag.Add(gtx.Ops)
	event.Op(gtx.Ops, d)
	stack.Pop()

	if drag != nil && d.drag.Pressed() {
		rec := op.Record(gtx.Ops)
		op.Offset(d.pos.Round()).Add(gtx.Ops)
		drag(gtx)
		op.Defer(gtx.Ops, rec.Stop())
	}

	return dims
}

// Dragging returns whether d is being dragged.
func (d *Draggable) Dragging() bool {
	return d.drag.Dragging()
}

// Update the draggable and returns the MIME type for which the Draggable was
// requested to offer data, if any
// func (d *Draggable) Update(gtx layout.Context) (mime string, requested bool) {
func (d *Draggable) Update(gtx layout.Context) iter.Seq[string] {
	return func(yield func(string) bool) {
		pos := d.pos
		for ev := range d.drag.Update(gtx.Metric, gtx.Source, gesture.Both) {
			switch ev.Kind {
			case pointer.Press:
				d.click = ev.Position
				pos = f32.Point{}
			case pointer.Drag, pointer.Release:
				pos = ev.Position.Sub(d.click)
			}
		}
		d.pos = pos
		for e := range gtx.Events(transfer.SourceFilter{Target: d, Type: d.Type}) {
			if e, ok := e.(transfer.RequestEvent); ok {
				if !yield(e.Type) {
					return
				}
			}
		}
		//return "", false
	}
}

// Offer the data ready for a drop. Must be called after being Requested.
// The mime must be one in the requested list.
func (d *Draggable) Offer(gtx layout.Context, mime string, data io.ReadCloser) {
	gtx.Execute(transfer.OfferCmd{Tag: d, Type: mime, Data: data})
}

// Pos returns the drag position relative to its initial click position.
func (d *Draggable) Pos() f32.Point {
	return d.pos
}
