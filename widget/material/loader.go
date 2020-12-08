// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"
	"image/color"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type LoaderStyle struct {
	Color color.NRGBA
}

func Loader(th *Theme) LoaderStyle {
	return LoaderStyle{
		Color: th.Palette.ContrastBg,
	}
}

func (l LoaderStyle) Layout(gtx layout.Context) layout.Dimensions {
	diam := gtx.Constraints.Min.X
	if minY := gtx.Constraints.Min.Y; minY > diam {
		diam = minY
	}
	if diam == 0 {
		diam = gtx.Px(unit.Dp(24))
	}
	sz := gtx.Constraints.Constrain(image.Pt(diam, diam))
	radius := float64(sz.X) * .5
	defer op.Push(gtx.Ops).Pop()
	op.Offset(f32.Pt(float32(radius), float32(radius))).Add(gtx.Ops)

	dt := (time.Duration(gtx.Now.UnixNano()) % (time.Second)).Seconds()
	startAngle := dt * math.Pi * 2
	endAngle := startAngle + math.Pi*1.5

	clipLoader(gtx.Ops, startAngle, endAngle, radius)
	paint.ColorOp{
		Color: l.Color,
	}.Add(gtx.Ops)
	op.Offset(f32.Pt(-float32(radius), -float32(radius))).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	op.InvalidateOp{}.Add(gtx.Ops)
	return layout.Dimensions{
		Size: sz,
	}
}

func clipLoader(ops *op.Ops, startAngle, endAngle, radius float64) {
	const thickness = .25

	var (
		width = float32(radius * thickness)
		delta = float32(endAngle - startAngle)

		vy, vx = math.Sincos(startAngle)

		pen    = f32.Pt(float32(vx), float32(vy)).Mul(float32(radius))
		center = f32.Pt(0, 0).Sub(pen)

		p clip.Path
	)

	p.Begin(ops)
	p.Move(pen)
	p.Arc(center, center, delta)
	clip.Stroke{
		Path: p.End(),
		Style: clip.StrokeStyle{
			Width: width,
			Cap:   clip.FlatCap,
		},
	}.Op().Add(ops)
}
