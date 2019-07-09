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
	Src  image.Image
	Rect image.Rectangle
}

func (im Image) Layout(ops *ui.Ops, cs layout.Constraints) layout.Dimens {
	size := im.Src.Bounds()
	w, h := size.Dx(), size.Dy()
	if w == 0 || h == 0 {
		return layout.Dimens{}
	}
	d := image.Point{X: cs.Width.Max, Y: cs.Height.Max}
	if d.X == ui.Inf {
		d.X = cs.Width.Min
	}
	if d.Y == ui.Inf {
		d.Y = cs.Height.Min
	}
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
