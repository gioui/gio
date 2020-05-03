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

func (im Image) Layout(gtx *layout.Context) {
	scale := im.Scale
	if scale == 0 {
		scale = 160.0 / 72.0
	}
	size := im.Src.Rect.Size()
	wf, hf := float32(size.X), float32(size.Y)
	w, h := gtx.Px(unit.Dp(wf*scale)), gtx.Px(unit.Dp(hf*scale))
	cs := gtx.Constraints
	d := image.Point{X: cs.Width.Constrain(w), Y: cs.Height.Constrain(h)}
	var s op.StackOp
	s.Push(gtx.Ops)
	clip.Rect{Rect: f32.Rectangle{Max: toPointF(d)}}.Op(gtx.Ops).Add(gtx.Ops)
	im.Src.Add(gtx.Ops)
	paint.PaintOp{Rect: f32.Rectangle{Max: f32.Point{X: float32(w), Y: float32(h)}}}.Add(gtx.Ops)
	s.Pop()
	gtx.Dimensions = layout.Dimensions{Size: d}
}

func toPointF(p image.Point) f32.Point {
	return f32.Point{X: float32(p.X), Y: float32(p.Y)}
}
