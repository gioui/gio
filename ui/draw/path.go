// SPDX-License-Identifier: Unlicense OR MIT

package draw

import (
	"encoding/binary"
	"math"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/internal/ops"
	"gioui.org/ui/internal/path"
)

type OpClip struct {
	Path *Path
}

type Path struct {
	data *path.Path
}

type PathBuilder struct {
	verts     []path.Vertex
	firstVert int
	maxy      float32
	pen       f32.Point
	bounds    f32.Rectangle
	hasBounds bool
}

// Data is for internal use only.
func (p *Path) Data() interface{} {
	return p.data
}

func (c OpClip) Add(o *ui.Ops) {
	data := make([]byte, ops.TypeClipLen)
	data[0] = byte(ops.TypeClip)
	bo := binary.LittleEndian
	ref := o.Ref(c.Path)
	bo.PutUint32(data[1:], uint32(ref))
	o.Write(data)
}

func (c *OpClip) Decode(d []byte, refs []interface{}) {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypeClip {
		panic("invalid op")
	}
	ref := int(bo.Uint32(d[1:]))
	*c = OpClip{
		Path: refs[ref].(*Path),
	}
}

// MoveTo moves the pen to the given position.
func (p *PathBuilder) Move(to f32.Point) {
	p.end()
	to = to.Add(p.pen)
	p.maxy = to.Y
	p.pen = to
}

// end completes the current contour.
func (p *PathBuilder) end() {
	// Fill in maximal Y coordinates of the NW and NE corners
	// and offset their curve coordinates.
	for i := p.firstVert; i < len(p.verts); i++ {
		p.verts[i].MaxY = p.maxy
	}
	p.firstVert = len(p.verts)
}

// Line records a line from the pen to end.
func (p *PathBuilder) Line(to f32.Point) {
	to = to.Add(p.pen)
	p.lineTo(to)
}

func (p *PathBuilder) lineTo(to f32.Point) {
	// Model lines as degenerate quadratic beziers.
	p.quadTo(to.Add(p.pen).Mul(.5), to)
}

// Quad records a quadratic bezier from the pen to end
// with the control point ctrl.
func (p *PathBuilder) Quad(ctrl, to f32.Point) {
	ctrl = ctrl.Add(p.pen)
	to = to.Add(p.pen)
	p.quadTo(ctrl, to)
}

func (p *PathBuilder) quadTo(ctrl, to f32.Point) {
	// Zero width curve don't contribute to stenciling.
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

// Cube records a cubic bezier from the pen through
// two control points ending in to.
func (p *PathBuilder) Cube(ctrl0, ctrl1, to f32.Point) {
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

// approxCube approximates a cubic beziér by a series of quadratic
// curves.
func (p *PathBuilder) approxCubeTo(splits int, maxDist float32, ctrl0, ctrl1, to f32.Point) int {
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
	// The reverse cubic that is anchored at the end point has the polynomial
	//
	// P'(t) = to + 3t(ctrl1 - to) + 3t²(ctrl0 - 2ctrl1 + to) + t³(pen - 3ctrl0 + 3ctrl1 - to)
	//
	// The corresponding quadratic Q2 that shares the end point, to, with P has control
	// point
	//
	// C2 = (3ctrl1 - to)/2
	//
	// The combined quadratic beziér, Q, shares both start and end points with its cubic
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

func (p *PathBuilder) expand(b f32.Rectangle) {
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

func (p *PathBuilder) vertex(cornerx, cornery int16, ctrl, to f32.Point) {
	p.verts = append(p.verts, path.Vertex{
		CornerX: cornerx,
		CornerY: cornery,
		FromX:   p.pen.X,
		FromY:   p.pen.Y,
		CtrlX:   ctrl.X,
		CtrlY:   ctrl.Y,
		ToX:     to.X,
		ToY:     to.Y,
	})
}

func (p *PathBuilder) simpleQuadTo(ctrl, to f32.Point) {
	if p.pen.Y > p.maxy {
		p.maxy = p.pen.Y
	}
	if ctrl.Y > p.maxy {
		p.maxy = ctrl.Y
	}
	if to.Y > p.maxy {
		p.maxy = to.Y
	}
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

func (p *PathBuilder) Path() *Path {
	p.end()
	data := &Path{
		data: &path.Path{
			Bounds: p.bounds,
		},
	}
	if !p.bounds.Empty() {
		data.data.Vertices = p.verts
	}
	return data
}

func (p OpClip) ImplementsOp() {}
