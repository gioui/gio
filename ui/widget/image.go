// SPDX-License-Identifier: Unlicense OR MIT

// Package widget implements common widgets.
package widget

import (
	"image"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/layout"
	"gioui.org/ui/paint"
)

// Image is a widget that displays an image.
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

func (im Image) Layout(c ui.Config, ops *ui.Ops, cs layout.Constraints) layout.Dimensions {
	size := im.Src.Bounds()
	wf, hf := float32(size.Dx()), float32(size.Dy())
	var w, h int
	if im.Scale == 0 {
		const dpPrPx = 160 / 72
		w, h = c.Px(ui.Dp(wf*dpPrPx)), c.Px(ui.Dp(hf*dpPrPx))
	} else {
		w, h = int(wf*im.Scale+.5), int(hf*im.Scale+.5)
	}
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
	paint.ImageOp{Src: im.Src, Rect: im.Rect}.Add(ops)
	paint.PaintOp{Rect: dr}.Add(ops)
	return layout.Dimensions{Size: d, Baseline: d.Y}
}
