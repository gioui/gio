// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

// Float is for selecting a value in a range.
type Float struct {
	// Value is the value of the Float, in the [0; 1] range.
	Value float32

	drag   gesture.Drag
	axis   layout.Axis
	length float32
}

// Dragging returns whether the value is being interacted with.
func (f *Float) Dragging() bool { return f.drag.Dragging() }

func (f *Float) Layout(gtx layout.Context, axis layout.Axis, pointerMargin unit.Dp) layout.Dimensions {
	f.Update(gtx)
	size := gtx.Constraints.Min
	f.length = float32(axis.Convert(size).X)
	f.axis = axis

	margin := axis.Convert(image.Pt(gtx.Dp(pointerMargin), 0))
	rect := image.Rectangle{
		Min: margin.Mul(-1),
		Max: size.Add(margin),
	}
	defer clip.Rect(rect).Push(gtx.Ops).Pop()
	f.drag.Add(gtx.Ops)

	return layout.Dimensions{Size: size}
}

// Update the Value according to drag events along the f's main axis.
// The return value reports whether the value was changed.
//
// The range of f is set by the minimum constraints main axis value.
func (f *Float) Update(gtx layout.Context) bool {
	changed := false
	for {
		e, ok := f.drag.Update(gtx.Metric, gtx.Source, gesture.Axis(f.axis))
		if !ok {
			break
		}
		if f.length > 0 && (e.Kind == pointer.Press || e.Kind == pointer.Drag) {
			pos := e.Position.X
			if f.axis == layout.Vertical {
				pos = f.length - e.Position.Y
			}
			f.Value = pos / f.length
			if f.Value < 0 {
				f.Value = 0
			} else if f.Value > 1 {
				f.Value = 1
			}
			changed = true
		}
	}
	return changed
}
