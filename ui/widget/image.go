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
	d := image.Point{X: cs.Width.Max, Y: cs.Height.Max}
	if d.X == ui.Inf {
		d.X = cs.Width.Min
	}
	if d.Y == ui.Inf {
		d.Y = cs.Height.Min
	}
	dr := f32.Rectangle{
		Max: f32.Point{X: float32(d.X), Y: float32(d.Y)},
	}
	draw.OpImage{Rect: dr, Src: im.Src, SrcRect: im.Rect}.Add(ops)
	return layout.Dimens{Size: d, Baseline: d.Y}
}
