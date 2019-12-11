// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"encoding/binary"
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/internal/path"
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
	ops       *op.Ops
	contour   int
	pen       f32.Point
	bounds    f32.Rectangle
	hasBounds bool
	macro     op.MacroOp
}

// Op sets the current clip to the intersection of
// the existing clip with this clip.
//
// If you need to reset the clip to its previous values after
// applying a Op, use op.StackOp.
type Op struct {
	macro  op.MacroOp
	bounds f32.Rectangle
}

func (p Op) Add(o *op.Ops) {
	p.macro.Add()
	data := o.Write(opconst.TypeClipLen)
	data[0] = byte(opconst.TypeClip)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(p.bounds.Min.X))
	bo.PutUint32(data[5:], math.Float32bits(p.bounds.Min.Y))
	bo.PutUint32(data[9:], math.Float32bits(p.bounds.Max.X))
	bo.PutUint32(data[13:], math.Float32bits(p.bounds.Max.Y))
}

// Begin the path, storing the path data and final Op into ops.
func (p *Path) Begin(ops *op.Ops) {
	p.ops = ops
	p.macro.Record(ops)
	// Write the TypeAux opcode and a byte for marking whether the
	// path has had its MaxY filled out. If not, the gpu will fill it
	// before using it.
	data := ops.Write(2)
	data[0] = byte(opconst.TypeAux)
}

// MoveTo moves the pen to the given position.
func (p *Path) Move(to f32.Point) {
	p.end()
	to = to.Add(p.pen)
	p.pen = to
}

// end completes the current contour.
func (p *Path) end() {
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
	// Zero width curves don't contribute to stenciling.
	if p.pen.X == to.X && p.pen.X == ctrl.X {
		p.pen = to
		return
	}

	bounds := f32.Rectangle{
		Min: p.pen,
		Max: to,
	}.Canon()

	// If the curve contain areas where a vertical line
	// intersects it twice, split the curve in two x monotone
	// lower and upper curves. The stencil fragment program
	// expects only one intersection per curve.

	// Find the t where the derivative in x is 0.
	v0 := ctrl.Sub(p.pen)
	v1 := to.Sub(ctrl)
	d := v0.X - v1.X
	// t = v0 / d. Split if t is in ]0;1[.
	if v0.X > 0 && d > v0.X || v0.X < 0 && d < v0.X {
		t := v0.X / d
		ctrl0 := p.pen.Mul(1 - t).Add(ctrl.Mul(t))
		ctrl1 := ctrl.Mul(1 - t).Add(to.Mul(t))
		mid := ctrl0.Mul(1 - t).Add(ctrl1.Mul(t))
		p.simpleQuadTo(ctrl0, mid)
		p.simpleQuadTo(ctrl1, to)
		if mid.X > bounds.Max.X {
			bounds.Max.X = mid.X
		}
		if mid.X < bounds.Min.X {
			bounds.Min.X = mid.X
		}
	} else {
		p.simpleQuadTo(ctrl, to)
	}
	// Find the y extremum, if any.
	d = v0.Y - v1.Y
	if v0.Y > 0 && d > v0.Y || v0.Y < 0 && d < v0.Y {
		t := v0.Y / d
		y := (1-t)*(1-t)*p.pen.Y + 2*(1-t)*t*ctrl.Y + t*t*to.Y
		if y > bounds.Max.Y {
			bounds.Max.Y = y
		}
		if y < bounds.Min.Y {
			bounds.Min.Y = y
		}
	}
	p.expand(bounds)
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

func (p *Path) expand(b f32.Rectangle) {
	if !p.hasBounds {
		p.hasBounds = true
		inf := float32(math.Inf(+1))
		p.bounds = f32.Rectangle{
			Min: f32.Point{X: inf, Y: inf},
			Max: f32.Point{X: -inf, Y: -inf},
		}
	}
	p.bounds = p.bounds.Union(b)
}

func (p *Path) vertex(cornerx, cornery int16, ctrl, to f32.Point) {
	v := path.Vertex{
		CornerX: cornerx,
		CornerY: cornery,
		FromX:   p.pen.X,
		FromY:   p.pen.Y,
		CtrlX:   ctrl.X,
		CtrlY:   ctrl.Y,
		ToX:     to.X,
		ToY:     to.Y,
	}
	data := p.ops.Write(path.VertStride)
	bo := binary.LittleEndian
	data[0] = byte(uint16(v.CornerX))
	data[1] = byte(uint16(v.CornerX) >> 8)
	data[2] = byte(uint16(v.CornerY))
	data[3] = byte(uint16(v.CornerY) >> 8)
	// Put the contour index in MaxY.
	bo.PutUint32(data[4:], uint32(p.contour))
	bo.PutUint32(data[8:], math.Float32bits(v.FromX))
	bo.PutUint32(data[12:], math.Float32bits(v.FromY))
	bo.PutUint32(data[16:], math.Float32bits(v.CtrlX))
	bo.PutUint32(data[20:], math.Float32bits(v.CtrlY))
	bo.PutUint32(data[24:], math.Float32bits(v.ToX))
	bo.PutUint32(data[28:], math.Float32bits(v.ToY))
}

func (p *Path) simpleQuadTo(ctrl, to f32.Point) {
	// NW.
	p.vertex(-1, 1, ctrl, to)
	// NE.
	p.vertex(1, 1, ctrl, to)
	// SW.
	p.vertex(-1, -1, ctrl, to)
	// SE.
	p.vertex(1, -1, ctrl, to)
	p.pen = to
}

// End the path and return a clip operation that represents it.
func (p *Path) End() Op {
	p.end()
	p.macro.Stop()
	return Op{
		macro:  p.macro,
		bounds: p.bounds,
	}
}

// Rect represents the clip area of a rectangle with rounded
// corners.The origin is in the upper left
// corner.
// Specify a square with corner radii equal to half the square size to
// construct a circular clip area.
type Rect struct {
	Rect f32.Rectangle
	// The corner radii.
	SE, SW, NW, NE float32
}

// Op returns the Op for the rectangle.
func (rr Rect) Op(ops *op.Ops) Op {
	r := rr.Rect
	// Optimize for the common pixel aligned rectangle with no
	// corner rounding.
	if rr.SE == 0 && rr.SW == 0 && rr.NW == 0 && rr.NE == 0 {
		ri := image.Rectangle{
			Min: image.Point{X: int(r.Min.X), Y: int(r.Min.Y)},
			Max: image.Point{X: int(r.Max.X), Y: int(r.Max.Y)},
		}
		// Optimize pixel-aligned rectangles to just its bounds.
		if r == toRectF(ri) {
			return Op{bounds: r}
		}
	}
	return roundRect(ops, r, rr.SE, rr.SW, rr.NW, rr.NE)
}

// roundRect returns the clip area of a rectangle with rounded
// corners defined by their radii.
func roundRect(ops *op.Ops, r f32.Rectangle, se, sw, nw, ne float32) Op {
	size := r.Size()
	// https://pomax.github.io/bezierinfo/#circles_cubic.
	w, h := float32(size.X), float32(size.Y)
	const c = 0.55228475 // 4*(sqrt(2)-1)/3
	var p Path
	p.Begin(ops)
	p.Move(r.Min)
	p.Move(f32.Point{X: w, Y: h - se})
	p.Cube(f32.Point{X: 0, Y: se * c}, f32.Point{X: -se + se*c, Y: se}, f32.Point{X: -se, Y: se}) // SE
	p.Line(f32.Point{X: sw - w + se, Y: 0})
	p.Cube(f32.Point{X: -sw * c, Y: 0}, f32.Point{X: -sw, Y: -sw + sw*c}, f32.Point{X: -sw, Y: -sw}) // SW
	p.Line(f32.Point{X: 0, Y: nw - h + sw})
	p.Cube(f32.Point{X: 0, Y: -nw * c}, f32.Point{X: nw - nw*c, Y: -nw}, f32.Point{X: nw, Y: -nw}) // NW
	p.Line(f32.Point{X: w - ne - nw, Y: 0})
	p.Cube(f32.Point{X: ne * c, Y: 0}, f32.Point{X: ne, Y: ne - ne*c}, f32.Point{X: ne, Y: ne}) // NE
	return p.End()
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{X: float32(r.Min.X), Y: float32(r.Min.Y)},
		Max: f32.Point{X: float32(r.Max.X), Y: float32(r.Max.Y)},
	}
}
