// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Border lays out a widget and draws a border inside it.
type Border struct {
	Color        color.RGBA
	CornerRadius unit.Value
	Width        unit.Value
}

func (b Border) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	dims := w(gtx)
	sz := dims.Size
	rr := float32(gtx.Px(b.CornerRadius))
	st := op.Push(gtx.Ops)
	width := gtx.Px(b.Width)
	clip.Border{
		Rect: f32.Rectangle{
			Max: layout.FPt(sz),
		},
		NE: rr, NW: rr, SE: rr, SW: rr,
		Width: float32(width),
	}.Add(gtx.Ops)
	dr := f32.Rectangle{
		Max: layout.FPt(sz),
	}
	paint.ColorOp{Color: b.Color}.Add(gtx.Ops)
	paint.PaintOp{Rect: dr}.Add(gtx.Ops)
	st.Pop()
	return dims
}
