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
	Color        color.NRGBA
	CornerRadius unit.Value
	Width        unit.Value
}

func (b Border) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	dims := w(gtx)
	sz := dims.Size
	rr := float32(gtx.Px(b.CornerRadius))
	st := op.Save(gtx.Ops)
	width := gtx.Px(b.Width)
	sz.X -= width
	sz.Y -= width
	op.Offset(f32.Point{
		X: float32(width) * 0.5,
		Y: float32(width) * 0.5,
	}).Add(gtx.Ops)
	clip.Border{
		Rect: f32.Rectangle{
			Max: layout.FPt(sz),
		},
		NE: rr, NW: rr, SE: rr, SW: rr,
		Width: float32(width),
	}.Add(gtx.Ops)
	paint.ColorOp{Color: b.Color}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Load()
	return dims
}
