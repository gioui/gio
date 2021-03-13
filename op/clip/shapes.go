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
	return Outline{Path: rr.path(ops)}.Op()
}

// path returns the path spec for the rounded rectangle.
func (rr RRect) path(ops *op.Ops) PathSpec {
	var p Path
	p.Begin(ops)

	// https://pomax.github.io/bezierinfo/#circles_cubic.
	const q = 4 * (math.Sqrt2 - 1) / 3
	const iq = 1 - q

	se, sw, nw, ne := rr.SE, rr.SW, rr.NW, rr.NE
	w, n, e, s := rr.Rect.Min.X, rr.Rect.Min.Y, rr.Rect.Max.X, rr.Rect.Max.Y

	p.MoveTo(f32.Point{X: w + nw, Y: n})
	p.LineTo(f32.Point{X: e - ne, Y: n}) // N
	p.CubeTo(                            // NE
		f32.Point{X: e - ne*iq, Y: n},
		f32.Point{X: e, Y: n + ne*iq},
		f32.Point{X: e, Y: n + ne})
	p.LineTo(f32.Point{X: e, Y: s - se}) // E
	p.CubeTo(                            // SE
		f32.Point{X: e, Y: s - se*iq},
		f32.Point{X: e - se*iq, Y: s},
		f32.Point{X: e - se, Y: s})
	p.LineTo(f32.Point{X: w + sw, Y: s}) // S
	p.CubeTo(                            // SW
		f32.Point{X: w + sw*iq, Y: s},
		f32.Point{X: w, Y: s - sw*iq},
		f32.Point{X: w, Y: s - sw})
	p.LineTo(f32.Point{X: w, Y: n + nw}) // W
	p.CubeTo(                            // NW
		f32.Point{X: w, Y: n + nw*iq},
		f32.Point{X: w + nw*iq, Y: n},
		f32.Point{X: w + nw, Y: n})

	return p.End()
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
	return Stroke{
		Path: RRect{
			Rect: b.Rect,
			SE:   b.SE, SW: b.SW, NW: b.NW, NE: b.NE,
		}.path(ops),
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
	const q = 4 * (math.Sqrt2 - 1) / 3

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
