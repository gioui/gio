// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Image is a widget that displays an image.
type Image struct {
	// Src is the image to display.
	Src paint.ImageOp
	// Scale is the ratio of image pixels to
	// dps. If Scale is zero Image falls back to
	// a scale that match a standard 72 DPI.
	Scale float32
}

const defaultScale = float32(160.0 / 72.0)

func (im Image) Layout(gtx layout.Context) layout.Dimensions {
	scale := im.Scale
	if scale == 0 {
		scale = defaultScale
	}
	size := im.Src.Size()
	wf, hf := float32(size.X), float32(size.Y)
	w, h := gtx.Px(unit.Dp(wf*scale)), gtx.Px(unit.Dp(hf*scale))
	cs := gtx.Constraints
	d := cs.Constrain(image.Pt(w, h))
	stack := op.Save(gtx.Ops)
	pixelScale := scale * gtx.Metric.PxPerDp
	op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(pixelScale, pixelScale))).Add(gtx.Ops)
	clip.Rect(image.Rectangle{Max: d}).Add(gtx.Ops)
	im.Src.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Load()
	return layout.Dimensions{Size: d}
}
