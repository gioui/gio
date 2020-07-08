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
	Color color.RGBA
}

func Loader(th *Theme) LoaderStyle {
	return LoaderStyle{
		Color: th.Color.Primary,
	}
}

func (l LoaderStyle) Layout(gtx layout.Context) layout.Dimensions {
	diam := gtx.Px(unit.Dp(24))
	if minX := gtx.Constraints.Min.X; minX > diam {
		diam = minX
	}
	if minY := gtx.Constraints.Min.Y; minY > diam {
		diam = minY
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
	paint.PaintOp{
		Rect: f32.Rectangle{Max: layout.FPt(sz)},
	}.Add(gtx.Ops)
	op.InvalidateOp{}.Add(gtx.Ops)
	return layout.Dimensions{
		Size: sz,
	}
}

func clipLoader(ops *op.Ops, start, end, radius float64) {
	const thickness = .2

	outer := float32(radius)
	inner := float32(radius) * (1. - thickness)

	var p clip.Path
	p.Begin(ops)

	sine, cose := math.Sincos(start)

	pen := f32.Pt(float32(cose), float32(sine)).Mul(outer)
	p.Move(pen)
	angle := start
	// The clip path uses quadratic beziér curves to approximate
	// a circle arc. Minimize the error by capping the length of
	// each curve segment.
	arcPrRadian := radius * math.Pi
	const maxArcLen = 20.
	anglePerSegment := maxArcLen / arcPrRadian
	// Outer arc.
	for angle < end {
		angle += anglePerSegment
		if angle > end {
			angle = end
		}
		sins, coss := sine, cose
		sine, cose = math.Sincos(angle)

		// https://pomax.github.io/bezierinfo/#circles
		div := 1. / (coss*sine - cose*sins)
		ctrl := f32.Point{
			X: float32((sine - sins) * div),
			Y: -float32((cose - coss) * div),
		}.Mul(outer)

		endPt := f32.Pt(float32(cose), float32(sine)).Mul(outer)

		p.Quad(ctrl.Sub(pen), endPt.Sub(pen))
		pen = endPt
	}

	// Arc cap.
	cap := f32.Pt(float32(cose), float32(sine)).Mul(inner)
	p.Line(cap.Sub(pen))
	pen = cap

	// Inner arc.
	for angle > start {
		angle -= anglePerSegment
		if angle < start {
			angle = start
		}
		sins, coss := sine, cose
		sine, cose = math.Sincos(angle)

		div := 1. / (coss*sine - cose*sins)
		ctrl := f32.Point{
			X: float32((sine - sins) * div),
			Y: -float32((cose - coss) * div),
		}.Mul(inner)

		endPt := f32.Pt(float32(cose), float32(sine)).Mul(inner)

		p.Quad(ctrl.Sub(pen), endPt.Sub(pen))
		pen = endPt
	}

	// Second arc cap automatically completed by End.
	p.End().Add(ops)
}
