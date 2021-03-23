// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"encoding/binary"
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/internal/scene"
	"gioui.org/internal/stroke"
	"gioui.org/op"
)

// Op represents a clip area. Op intersects the current clip area with
// itself.
type Op struct {
	bounds image.Rectangle
	path   PathSpec

	outline bool
	stroke  StrokeStyle
	dashes  DashSpec
}

func (p Op) Add(o *op.Ops) {
	if p.path.hasSegments {
		data := o.Write(opconst.TypePathLen)
		data[0] = byte(opconst.TypePath)
		p.path.spec.Add(o)
	}

	if p.stroke.Width > 0 {
		data := o.Write(opconst.TypeStrokeLen)
		data[0] = byte(opconst.TypeStroke)
		bo := binary.LittleEndian
		bo.PutUint32(data[1:], math.Float32bits(p.stroke.Width))
		bo.PutUint32(data[5:], math.Float32bits(p.stroke.Miter))
		data[9] = uint8(p.stroke.Cap)
		data[10] = uint8(p.stroke.Join)
	}

	if p.dashes.phase != 0 || p.dashes.size > 0 {
		data := o.Write(opconst.TypeDashLen)
		data[0] = byte(opconst.TypeDash)
		bo := binary.LittleEndian
		bo.PutUint32(data[1:], math.Float32bits(p.dashes.phase))
		data[5] = p.dashes.size // FIXME(sbinet) uint16? uint32?
		p.dashes.spec.Add(o)
	}

	data := o.Write(opconst.TypeClipLen)
	data[0] = byte(opconst.TypeClip)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(p.bounds.Min.X))
	bo.PutUint32(data[5:], uint32(p.bounds.Min.Y))
	bo.PutUint32(data[9:], uint32(p.bounds.Max.X))
	bo.PutUint32(data[13:], uint32(p.bounds.Max.Y))
	if p.outline {
		data[17] = byte(1)
	}
}

type PathSpec struct {
	spec op.CallOp
	// open is true if any path contour is not closed. A closed contour starts
	// and ends in the same point.
	open bool
	// hasSegments tracks whether there is more than one path segment in the path.
	hasSegments bool
}

// Path constructs a Op clip path described by lines and
// Bézier curves, where drawing outside the Path is discarded.
// The inside-ness of a pixel is determines by the non-zero winding rule,
// similar to the SVG rule of the same name.
//
// Path generates no garbage and can be used for dynamic paths; path
// data is stored directly in the Ops list supplied to Begin.
type Path struct {
	ops         *op.Ops
	open        bool
	contour     int
	pen         f32.Point
	macro       op.MacroOp
	start       f32.Point
	hasSegments bool
}

// Pos returns the current pen position.
func (p *Path) Pos() f32.Point { return p.pen }

// Begin the path, storing the path data and final Op into ops.
func (p *Path) Begin(ops *op.Ops) {
	p.ops = ops
	p.macro = op.Record(ops)
	// Write the TypeAux opcode
	data := ops.Write(opconst.TypeAuxLen)
	data[0] = byte(opconst.TypeAux)
}

// End returns a PathSpec ready to use in clipping operations.
func (p *Path) End() PathSpec {
	c := p.macro.Stop()
	return PathSpec{
		spec:        c,
		open:        p.open || p.pen != p.start,
		hasSegments: p.hasSegments,
	}
}

// Move moves the pen by the amount specified by delta.
func (p *Path) Move(delta f32.Point) {
	to := delta.Add(p.pen)
	p.MoveTo(to)
}

// MoveTo moves the pen to the specified absolute coordinate.
func (p *Path) MoveTo(to f32.Point) {
	p.open = p.open || p.pen != p.start
	p.end()
	p.pen = to
	p.start = to
}

// end completes the current contour.
func (p *Path) end() {
	p.contour++
}

// Line moves the pen by the amount specified by delta, recording a line.
func (p *Path) Line(delta f32.Point) {
	to := delta.Add(p.pen)
	p.LineTo(to)
}

// LineTo moves the pen to the absolute point specified, recording a line.
func (p *Path) LineTo(to f32.Point) {
	data := p.ops.Write(scene.CommandSize + 4)
	bo := binary.LittleEndian
	bo.PutUint32(data[0:], uint32(p.contour))
	ops.EncodeCommand(data[4:], scene.Line(p.pen, to))
	p.pen = to
	p.hasSegments = true
}

// Quad records a quadratic Bézier from the pen to end
// with the control point ctrl.
func (p *Path) Quad(ctrl, to f32.Point) {
	ctrl = ctrl.Add(p.pen)
	to = to.Add(p.pen)
	p.QuadTo(ctrl, to)
}

// QuadTo records a quadratic Bézier from the pen to end
// with the control point ctrl, with absolute coordinates.
func (p *Path) QuadTo(ctrl, to f32.Point) {
	data := p.ops.Write(scene.CommandSize + 4)
	bo := binary.LittleEndian
	bo.PutUint32(data[0:], uint32(p.contour))
	ops.EncodeCommand(data[4:], scene.Quad(p.pen, ctrl, to))
	p.pen = to
	p.hasSegments = true
}

// Arc adds an elliptical arc to the path. The implied ellipse is defined
// by its focus points f1 and f2.
// The arc starts in the current point and ends angle radians along the ellipse boundary.
// The sign of angle determines the direction; positive being counter-clockwise,
// negative clockwise.
func (p *Path) Arc(f1, f2 f32.Point, angle float32) {
	f1 = f1.Add(p.pen)
	f2 = f2.Add(p.pen)
	const segments = 16
	m := stroke.ArcTransform(p.pen, f1, f2, angle, segments)

	for i := 0; i < segments; i++ {
		p0 := p.pen
		p1 := m.Transform(p0)
		p2 := m.Transform(p1)
		ctl := p1.Mul(2).Sub(p0.Add(p2).Mul(.5))
		p.QuadTo(ctl, p2)
	}
}

// Cube records a cubic Bézier from the pen through
// two control points ending in to.
func (p *Path) Cube(ctrl0, ctrl1, to f32.Point) {
	p.CubeTo(p.pen.Add(ctrl0), p.pen.Add(ctrl1), p.pen.Add(to))
}

// CubeTo records a cubic Bézier from the pen through
// two control points ending in to, with absolute coordinates.
func (p *Path) CubeTo(ctrl0, ctrl1, to f32.Point) {
	if ctrl0 == p.pen && ctrl1 == p.pen && to == p.pen {
		return
	}
	data := p.ops.Write(scene.CommandSize + 4)
	bo := binary.LittleEndian
	bo.PutUint32(data[0:], uint32(p.contour))
	ops.EncodeCommand(data[4:], scene.Cubic(p.pen, ctrl0, ctrl1, to))
	p.pen = to
	p.hasSegments = true
}

// Close closes the path contour.
func (p *Path) Close() {
	if p.pen != p.start {
		p.LineTo(p.start)
	}
	p.end()
}

// Outline represents the area inside of a path, according to the
// non-zero winding rule.
type Outline struct {
	Path PathSpec
}

// Op returns a clip operation representing the outline.
func (o Outline) Op() Op {
	if o.Path.open {
		panic("not all path contours are closed")
	}
	return Op{
		path:    o.Path,
		outline: true,
	}
}
