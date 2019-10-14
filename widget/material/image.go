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
	Src paint.ImageOp
	// Scale is the ratio of image pixels to
	// dps.
	Scale float32
}

func (t *Theme) Image(img paint.ImageOp) Image {
	return Image{
		Src:   img,
		Scale: 160 / 72, // About 72 DPI.
	}
}

func (im Image) Layout(gtx *layout.Context) {
	size := im.Src.Size()
	wf, hf := float32(size.X), float32(size.Y)
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
	im.Src.Add(gtx.Ops)
	paint.PaintOp{Rect: dr}.Add(gtx.Ops)
	gtx.Dimensions = layout.Dimensions{Size: d, Baseline: d.Y}
}
