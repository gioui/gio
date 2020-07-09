// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"gioui.org/f32"
	"gioui.org/op"
)

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

// op returns the op for the rectangle.
func (rr RRect) op(ops *op.Ops) Op {
	return roundRect(ops, rr.Rect, rr.SE, rr.SW, rr.NW, rr.NE)
}

// Add the rectangle clip.
func (rr RRect) Add(ops *op.Ops) {
	rr.op(ops).Add(ops)
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
