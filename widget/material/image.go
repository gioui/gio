// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Image is a widget that displays an image.
type Image struct {
	// Src is the image to display.
	Src image.Image
	// Rect is the source rectangle.
	Rect image.Rectangle
	// Scale is the ratio of image pixels to
	// dps.
	Scale float32
}

func (t *Theme) Image(img image.Image) Image {
	return Image{
		Src:   img,
		Rect:  img.Bounds(),
		Scale: 160 / 72, // About 72 DPI.
	}
}

func (im Image) Layout(gtx *layout.Context) {
	size := im.Src.Bounds()
	wf, hf := float32(size.Dx()), float32(size.Dy())
	w, h := gtx.Px(unit.Dp(wf*im.Scale)), gtx.Px(unit.Dp(hf*im.Scale))
	cs := gtx.Constraints
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
	paint.ImageOp{Src: im.Src, Rect: im.Rect}.Add(gtx.Ops)
	paint.PaintOp{Rect: dr}.Add(gtx.Ops)
	gtx.Dimensions = layout.Dimensions{Size: d, Baseline: d.Y}
}
