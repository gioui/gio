// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/op"
)

// Rect represents the clip area of a pixel-aligned rectangle.
type Rect image.Rectangle

// Op returns the op for the rectangle.
func (r Rect) Op() Op {
	return Op{
		bounds:  image.Rectangle(r),
		outline: true,
	}
}

// Add the clip operation.
func (r Rect) Add(ops *op.Ops) {
	r.Op().Add(ops)
}

// UniformRRect returns an RRect with all corner radii set to the
// provided radius.
func UniformRRect(rect f32.Rectangle, radius float32) RRect {
	return RRect{
		Rect: rect,
		SE:   radius,
		SW:   radius,
		NE:   radius,
		NW:   radius,
	}
}

// RRect represents the clip area of a rectangle with rounded
// corners.
//
// Specify a square with corner radii equal to half the square size to
// construct a circular clip area.
type RRect struct {
	Rect f32.Rectangle
	// The corner radii.
	SE, SW, NW, NE float32
}

// Op returns the op for the rounded rectangle.
func (rr RRect) Op(ops *op.Ops) Op {
	var p Path
	p.Begin(ops)
	p.Move(rr.Rect.Min)
	roundRect(&p, rr.Rect.Size(), rr.SE, rr.SW, rr.NW, rr.NE)
	p.Close()

	return Outline{
		Path: p.End(),
	}.Op()
}

// Add the rectangle clip.
func (rr RRect) Add(ops *op.Ops) {
	rr.Op(ops).Add(ops)
}

// Border represents a rectangular border.
type Border struct {
	// Rect is the bounds of the border.
	Rect f32.Rectangle
	// Width of the line tracing Rect.
	Width  float32
	Dashes DashSpec
	// The corner radii.
	SE, SW, NW, NE float32
}

// Op returns the clip operation for the border. Its area corresponds to a
// stroked line that traces the border rectangle, optionally with rounded
// corners and dashes.
func (b Border) Op(ops *op.Ops) Op {
	var p Path
	p.Begin(ops)
	p.Move(b.Rect.Min)
	roundRect(&p, b.Rect.Size(), b.SE, b.SW, b.NW, b.NE)
	p.Close()

	return Stroke{
		Path: p.End(),
		Style: StrokeStyle{
			Width: b.Width,
		},
		Dashes: b.Dashes,
	}.Op()
}

// Add the border clip.
func (rr Border) Add(ops *op.Ops) {
	rr.Op(ops).Add(ops)
}

// Circle represents the clip area of a circle.
type Circle struct {
	Center f32.Point
	Radius float32
}

// Op returns the op for the circle.
func (c Circle) Op(ops *op.Ops) Op {
	return Outline{Path: c.path(ops)}.Op()
}

// path returns the path spec for the circle.
func (c Circle) path(ops *op.Ops) PathSpec {
	var p Path
	p.Begin(ops)

	center := c.Center
	r := c.Radius

	// https://pomax.github.io/bezierinfo/#circles_cubic.
	const q = 4 * (math.Sqrt2 - 1) / 3 // 4*(sqrt(2)-1)/3

	curve := r * q
	top := f32.Point{X: center.X, Y: center.Y - r}

	p.MoveTo(top)
	p.CubeTo(
		f32.Point{X: center.X + curve, Y: center.Y - r},
		f32.Point{X: center.X + r, Y: center.Y - curve},
		f32.Point{X: center.X + r, Y: center.Y},
	)
	p.CubeTo(
		f32.Point{X: center.X + r, Y: center.Y + curve},
		f32.Point{X: center.X + curve, Y: center.Y + r},
		f32.Point{X: center.X, Y: center.Y + r},
	)
	p.CubeTo(
		f32.Point{X: center.X - curve, Y: center.Y + r},
		f32.Point{X: center.X - r, Y: center.Y + curve},
		f32.Point{X: center.X - r, Y: center.Y},
	)
	p.CubeTo(
		f32.Point{X: center.X - r, Y: center.Y - curve},
		f32.Point{X: center.X - curve, Y: center.Y - r},
		top,
	)
	return p.End()
}

// roundRect adds the outline of a rectangle with rounded corners to a
// path.
func roundRect(p *Path, size f32.Point, se, sw, nw, ne float32) {
	// https://pomax.github.io/bezierinfo/#circles_cubic.
	w, h := size.X, size.Y
	const c = 0.55228475 // 4*(sqrt(2)-1)/3
	p.Move(f32.Point{X: w, Y: h - se})
	p.Cube(f32.Point{X: 0, Y: se * c}, f32.Point{X: -se + se*c, Y: se}, f32.Point{X: -se, Y: se}) // SE
	p.Line(f32.Point{X: sw - w + se, Y: 0})
	p.Cube(f32.Point{X: -sw * c, Y: 0}, f32.Point{X: -sw, Y: -sw + sw*c}, f32.Point{X: -sw, Y: -sw}) // SW
	p.Line(f32.Point{X: 0, Y: nw - h + sw})
	p.Cube(f32.Point{X: 0, Y: -nw * c}, f32.Point{X: nw - nw*c, Y: -nw}, f32.Point{X: nw, Y: -nw}) // NW
	p.Line(f32.Point{X: w - ne - nw, Y: 0})
	p.Cube(f32.Point{X: ne * c, Y: 0}, f32.Point{X: ne, Y: ne - ne*c}, f32.Point{X: ne, Y: ne}) // NE
	p.Line(f32.Point{X: 0, Y: -(ne - h + se)})
}
