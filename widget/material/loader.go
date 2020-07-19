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
	paint.PaintOp{
		Rect: f32.Rectangle{Max: layout.FPt(sz)},
	}.Add(gtx.Ops)
	op.InvalidateOp{}.Add(gtx.Ops)
	return layout.Dimensions{
		Size: sz,
	}
}

func clipLoader(ops *op.Ops, startAngle, endAngle, radius float64) {
	const thickness = .25

	outer := float32(radius)
	inner := float32(radius) * (1. - thickness)

	var p clip.Path
	p.Begin(ops)

	vy, vx := math.Sincos(startAngle)

	start := f32.Pt(float32(vx), float32(vy))

	// Use quadratic beziér curves to approximate a circle arc and
	// minimize the error by capping the length of each curve segment.

	nsegments := math.Round(20 * math.Pi / (endAngle - startAngle))

	θ := (endAngle - startAngle) / nsegments

	// To avoid a math.Sincos for every segment, compute a clockwise
	// rotation matrix once and apply for each segment.
	//
	// [ cos θ -sin θ]
	// [sin θ cos θ]
	sinθ64, cosθ64 := math.Sincos(θ)
	sinθ, cosθ := float32(sinθ64), float32(cosθ64)
	rotate := func(clockwise float32, p f32.Point) f32.Point {
		return f32.Point{
			X: p.X*cosθ - p.Y*clockwise*sinθ,
			Y: p.X*clockwise*sinθ + p.Y*cosθ,
		}
	}

	// Compute control point C according to
	// https://pomax.github.io/bezierinfo/#circles.
	// If S is the starting point, S' is the orthogonal
	// tangent, θ is clockwise:
	//
	// C = S + b*S', b = (cos θ - 1)/sin θ
	//
	b := (cosθ - 1.) / sinθ

	control := func(clockwise float32, S f32.Point) f32.Point {
		tangent := f32.Pt(-S.Y, S.X)
		return S.Add(tangent.Mul(b * -clockwise))
	}

	pen := start.Mul(outer)
	p.Move(pen)

	end := start
	arc := func(clockwise float32, radius float32) {
		for i := 0; i < int(nsegments); i++ {
			ctrl := control(clockwise, end)
			end = rotate(clockwise, end)
			p.Quad(ctrl.Mul(radius).Sub(pen), end.Mul(radius).Sub(pen))
			pen = end.Mul(radius)
		}
	}
	// Outer arc, clockwise.
	arc(+1, outer)

	// Arc cap.
	cap := end.Mul(inner)
	p.Line(cap.Sub(pen))
	pen = cap

	// Inner arc, counter-clockwise.
	arc(-1, inner)

	// Second arc cap automatically completed by End.
	p.End().Add(ops)
}
