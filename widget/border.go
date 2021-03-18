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
	defer op.Save(gtx.Ops).Load()

	width := float32(gtx.Px(b.Width))

	op.Offset(f32.Pt(width, width)).Add(gtx.Ops)
	gtx.Constraints.Max.X -= int(2 * width)
	gtx.Constraints.Max.Y -= int(2 * width)
	dims := w(gtx)

	dims.Size.X += int(2 * width)
	dims.Size.Y += int(2 * width)
	dims.Baseline += int(width)

	sz := layout.FPt(dims.Size)

	rr := float32(gtx.Px(b.CornerRadius))
	sz.X -= width
	sz.Y -= width

	r := f32.Rectangle{Max: sz}
	r = r.Add(f32.Point{X: -width * 0.5, Y: -width * 0.5})

	paint.FillShape(gtx.Ops,
		b.Color,
		clip.Stroke{
			Path:  clip.UniformRRect(r, rr).Path(gtx.Ops),
			Style: clip.StrokeStyle{Width: width},
		}.Op(),
	)

	return dims
}
