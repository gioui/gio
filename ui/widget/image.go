// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"

	"gioui.org/ui"
	"gioui.org/ui/draw"
	"gioui.org/ui/f32"
	"gioui.org/ui/layout"
)

type Image struct {
	// Src is the image to display.
	Src image.Image
	// Rect is the source rectangle.
	Rect image.Rectangle
	// Scale is the ratio of image pixels to
	// device pixels. If zero, a scale that
	// makes the image appear at approximately
	// 72 DPI is used.
	Scale float32
}

func (im Image) Layout(c *ui.Config, ops *ui.Ops, cs layout.Constraints) layout.Dimens {
	size := im.Src.Bounds()
	scale := im.Scale
	if scale == 0 {
		const dpPrPx = 160 / 72
		scale = c.Val(ui.Dp(dpPrPx))
	}
	w, h := int(float32(size.Dx())*scale+.5), int(float32(size.Dy())*scale+.5)
	d := image.Point{X: cs.Width.Constrain(w), Y: cs.Height.Constrain(h)}
	aspect := float32(w) / float32(h)
	dw, dh := float32(d.X), float32(d.Y)
	dAspect := dw / dh
	if aspect < dAspect {
		d.X = int(dh*aspect + 0.5)
	} else {
		d.Y = int(dw/aspect + 0.5)
	}
	dr := f32.Rectangle{
		Max: f32.Point{X: float32(d.X), Y: float32(d.Y)},
	}
	draw.ImageOp{Img: im.Src, Rect: im.Rect}.Add(ops)
	draw.DrawOp{Rect: dr}.Add(ops)
	return layout.Dimens{Size: d, Baseline: d.Y}
}
