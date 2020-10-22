// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"encoding/binary"
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/op"
)

// Path constructs a Op clip path described by lines and
// Bézier curves, where drawing outside the Path is discarded.
// The inside-ness of a pixel is determines by the even-odd rule,
// similar to the SVG rule of the same name.
//
// Path generates no garbage and can be used for dynamic paths; path
// data is stored directly in the Ops list supplied to Begin.
type Path struct {
	ops     *op.Ops
	contour int
	pen     f32.Point
	macro   op.MacroOp
	start   f32.Point
}

// Pos returns the current pen position.
func (p *Path) Pos() f32.Point { return p.pen }

// Op sets the current clip to the intersection of
// the existing clip with this clip.
//
// If you need to reset the clip to its previous values after
// applying a Op, use op.StackOp.
type Op struct {
	call   op.CallOp
	bounds image.Rectangle
}

func (p Op) Add(o *op.Ops) {
	p.call.Add(o)
	data := o.Write(opconst.TypeClipLen)
	data[0] = byte(opconst.TypeClip)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(p.bounds.Min.X))
	bo.PutUint32(data[5:], uint32(p.bounds.Min.Y))
	bo.PutUint32(data[9:], uint32(p.bounds.Max.X))
	bo.PutUint32(data[13:], uint32(p.bounds.Max.Y))
}

// Begin the path, storing the path data and final Op into ops.
func (p *Path) Begin(ops *op.Ops) {
	p.ops = ops
	p.macro = op.Record(ops)
	// Write the TypeAux opcode
	data := ops.Write(opconst.TypeAuxLen)
	data[0] = byte(opconst.TypeAux)
}

// MoveTo moves the pen to the given position.
func (p *Path) Move(to f32.Point) {
	to = to.Add(p.pen)
	p.end()
	p.pen = to
	p.start = to
}

// end completes the current contour.
func (p *Path) end() {
	if p.pen != p.start {
		p.lineTo(p.start)
	}
	p.contour++
}

// Line moves the pen by the amount specified by delta, recording a line.
func (p *Path) Line(delta f32.Point) {
	to := delta.Add(p.pen)
	p.lineTo(to)
}

func (p *Path) lineTo(to f32.Point) {
	// Model lines as degenerate quadratic Béziers.
	p.quadTo(to.Add(p.pen).Mul(.5), to)
}

// Quad records a quadratic Bézier from the pen to end
// with the control point ctrl.
func (p *Path) Quad(ctrl, to f32.Point) {
	ctrl = ctrl.Add(p.pen)
	to = to.Add(p.pen)
	p.quadTo(ctrl, to)
}

func (p *Path) quadTo(ctrl, to f32.Point) {
	data := p.ops.Write(ops.QuadSize + 4)
	bo := binary.LittleEndian
	bo.PutUint32(data[0:], uint32(p.contour))
	ops.EncodeQuad(data[4:], ops.Quad{
		From: p.pen,
		Ctrl: ctrl,
		To:   to,
	})
	p.pen = to
}

// Arc adds an elliptical arc to the path. The implied ellipse is defined
// by its focus points f1 and f2.
// The arc starts in the current point and ends angle radians along the ellipse boundary.
// The sign of angle determines the direction; positive being counter-clockwise,
// negative clockwise.
func (p *Path) Arc(f1, f2 f32.Point, angle float32) {
	f1 = f1.Add(p.pen)
	f2 = f2.Add(p.pen)
	c, rx, ry, beg, alpha := arcFrom(f1, f2, p.pen)
	p.arc(alpha, c, rx, ry, beg, float64(angle))
}

func dist(p1, p2 f32.Point) float64 {
	var (
		x1 = float64(p1.X)
		y1 = float64(p1.Y)
		x2 = float64(p2.X)
		y2 = float64(p2.Y)
		dx = x2 - x1
		dy = y2 - y1
	)
	return math.Hypot(dx, dy)
}

func arcFrom(f1, f2, p f32.Point) (c f32.Point, rx, ry, start, alpha float64) {
	c = f32.Point{
		X: 0.5 * (f1.X + f2.X),
		Y: 0.5 * (f1.Y + f2.Y),
	}

	// semi-major axis: 2a = |PF1| + |PF2|
	a := 0.5 * (dist(f1, p) + dist(f2, p))

	// semi-minor axis: c^2 = a^2+b^2 (c: focal distance)
	f := dist(f1, c)
	b := math.Sqrt(a*a - f*f)

	switch {
	case a > b:
		rx = a
		ry = b
	default:
		rx = b
		ry = a
	}

	var x float64
	switch {
	case f1 == c || f2 == c:
		// degenerate case of a circle.
		alpha = 0
	default:
		switch {
		case f1.X > c.X:
			x = float64(f1.X - c.X)
			alpha = math.Acos(x / f)
		case f1.X < c.X:
			x = float64(f2.X - c.X)
			alpha = math.Acos(x / f)
		case f1.X == c.X:
			// special case of a "vertical" ellipse.
			alpha = math.Pi / 2
			if f1.Y < c.Y {
				alpha = -alpha
			}
		}
	}

	start = math.Acos(float64(p.X-c.X) / dist(c, p))
	if c.Y > p.Y {
		start = -start
	}
	start -= alpha

	return c, rx, ry, start, alpha
}

// arc records an elliptical arc centered at c, with radii rx and ry,
// starting at angle beg and stopping at end, in radians.
//
// The math is extracted from the following paper:
//  "Drawing an elliptical arc using polylines, quadratic or
//   cubic Bezier curves", L. Maisonobe
// An electronic version may be found at:
//  http://spaceroots.org/documents/ellipse/elliptical-arc.pdf
func (p *Path) arc(alpha float64, c f32.Point, rx, ry, beg, delta float64) {
	const n = 16
	var (
		θ   = delta / n
		ref f32.Affine2D // transform from absolute frame to ellipse-based one
		rot f32.Affine2D // rotation matrix for each segment
		inv f32.Affine2D // transform from ellipse-based frame to absolute one
	)
	ref = ref.Offset(f32.Point{}.Sub(c))
	ref = ref.Rotate(f32.Point{}, float32(-alpha))
	ref = ref.Scale(f32.Point{}, f32.Point{
		X: float32(1 / rx),
		Y: float32(1 / ry),
	})
	inv = ref.Invert()
	rot = rot.Rotate(f32.Point{}, float32(0.5*θ))

	// Instead of invoking math.Sincos for every segment, compute a rotation
	// matrix once and apply for each segment.
	// Before applying the rotation matrix rot, transform the coordinates
	// to a frame centered to the ellipse (and warped into a unit circle), then rotate.
	// Finally, transform back into the original frame.
	step := func(p f32.Point) f32.Point {
		q := ref.Transform(p)
		q = rot.Transform(q)
		q = inv.Transform(q)
		return q
	}

	for i := 0; i < n; i++ {
		p0 := p.pen
		p1 := step(p0)
		p2 := step(p1)
		ctl := f32.Pt(
			2*p1.X-0.5*(p0.X+p2.X),
			2*p1.Y-0.5*(p0.Y+p2.Y),
		)
		p.quadTo(ctl, p2)
	}
}

// Cube records a cubic Bézier from the pen through
// two control points ending in to.
func (p *Path) Cube(ctrl0, ctrl1, to f32.Point) {
	ctrl0 = ctrl0.Add(p.pen)
	ctrl1 = ctrl1.Add(p.pen)
	to = to.Add(p.pen)
	// Set the maximum distance proportionally to the longest side
	// of the bounding rectangle.
	hull := f32.Rectangle{
		Min: p.pen,
		Max: ctrl0,
	}.Canon().Add(ctrl1).Add(to)
	l := hull.Dx()
	if h := hull.Dy(); h > l {
		l = h
	}
	p.approxCubeTo(0, l*0.001, ctrl0, ctrl1, to)
}

// approxCube approximates a cubic Bézier by a series of quadratic
// curves.
func (p *Path) approxCubeTo(splits int, maxDist float32, ctrl0, ctrl1, to f32.Point) int {
	// The idea is from
	// https://caffeineowl.com/graphics/2d/vectorial/cubic2quad01.html
	// where a quadratic approximates a cubic by eliminating its t³ term
	// from its polynomial expression anchored at the starting point:
	//
	// P(t) = pen + 3t(ctrl0 - pen) + 3t²(ctrl1 - 2ctrl0 + pen) + t³(to - 3ctrl1 + 3ctrl0 - pen)
	//
	// The control point for the new quadratic Q1 that shares starting point, pen, with P is
	//
	// C1 = (3ctrl0 - pen)/2
	//
	// The reverse cubic anchored at the end point has the polynomial
	//
	// P'(t) = to + 3t(ctrl1 - to) + 3t²(ctrl0 - 2ctrl1 + to) + t³(pen - 3ctrl0 + 3ctrl1 - to)
	//
	// The corresponding quadratic Q2 that shares the end point, to, with P has control
	// point
	//
	// C2 = (3ctrl1 - to)/2
	//
	// The combined quadratic Bézier, Q, shares both start and end points with its cubic
	// and use the midpoint between the two curves Q1 and Q2 as control point:
	//
	// C = (3ctrl0 - pen + 3ctrl1 - to)/4
	c := ctrl0.Mul(3).Sub(p.pen).Add(ctrl1.Mul(3)).Sub(to).Mul(1.0 / 4.0)
	const maxSplits = 32
	if splits >= maxSplits {
		p.quadTo(c, to)
		return splits
	}
	// The maximum distance between the cubic P and its approximation Q given t
	// can be shown to be
	//
	// d = sqrt(3)/36*|to - 3ctrl1 + 3ctrl0 - pen|
	//
	// To save a square root, compare d² with the squared tolerance.
	v := to.Sub(ctrl1.Mul(3)).Add(ctrl0.Mul(3)).Sub(p.pen)
	d2 := (v.X*v.X + v.Y*v.Y) * 3 / (36 * 36)
	if d2 <= maxDist*maxDist {
		p.quadTo(c, to)
		return splits
	}
	// De Casteljau split the curve and approximate the halves.
	t := float32(0.5)
	c0 := p.pen.Add(ctrl0.Sub(p.pen).Mul(t))
	c1 := ctrl0.Add(ctrl1.Sub(ctrl0).Mul(t))
	c2 := ctrl1.Add(to.Sub(ctrl1).Mul(t))
	c01 := c0.Add(c1.Sub(c0).Mul(t))
	c12 := c1.Add(c2.Sub(c1).Mul(t))
	c0112 := c01.Add(c12.Sub(c01).Mul(t))
	splits++
	splits = p.approxCubeTo(splits, maxDist, c0, c01, c0112)
	splits = p.approxCubeTo(splits, maxDist, c12, c2, to)
	return splits
}

// End the path and return a clip operation that represents it.
func (p *Path) End() Op {
	p.end()
	c := p.macro.Stop()
	return Op{
		call: c,
	}
}

// Rect represents the clip area of a pixel-aligned rectangle.
type Rect image.Rectangle

// Op returns the op for the rectangle.
func (r Rect) Op(ops *op.Ops) Op {
	return Op{bounds: image.Rectangle(r)}
}

// Add the clip operation.
func (r Rect) Add(ops *op.Ops) {
	r.Op(ops).Add(ops)
}
