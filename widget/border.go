// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"image/color"

	"gioui.org/layout"
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

	rr := gtx.Px(b.CornerRadius)
	width := gtx.Px(b.Width)
	sz.X -= width
	sz.Y -= width

	r := image.Rectangle{Max: sz}
	r = r.Add(image.Point{X: width / 2, Y: width / 2})

	paint.FillShape(gtx.Ops,
		b.Color,
		clip.Stroke{
			Path:  clip.UniformRRect(r, rr).Path(gtx.Ops),
			Width: float32(width),
		}.Op(),
	)

	return dims
}
