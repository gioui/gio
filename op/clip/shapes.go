// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"gioui.org/f32"
	"gioui.org/op"
)

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

	return Outline{
		Path: p.End(),
	}.Op()
}

// Add the rectangle clip.
func (rr RRect) Add(ops *op.Ops) {
	rr.Op(ops).Add(ops)
}

// Border represents the clip area of a rectangular border.
type Border struct {
	// Rect is the bounds of the border.
	Rect   f32.Rectangle
	Width  float32
	Dashes DashSpec
	// The corner radii.
	SE, SW, NW, NE float32
}

// Op returns the Op for the border.
func (b Border) Op(ops *op.Ops) Op {
	var p Path
	p.Begin(ops)
	p.Move(b.Rect.Min)
	roundRect(&p, b.Rect.Size(), b.SE, b.SW, b.NW, b.NE)

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
