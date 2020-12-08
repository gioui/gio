// SPDX-License-Identifier: Unlicense OR MIT

package clip

// Stroke represents a stroked path.
type Stroke struct {
	Path  PathSpec
	Style StrokeStyle
}

// Op returns a clip operation representing the stroke.
func (s Stroke) Op() Op {
	return Op{
		path:   s.Path,
		stroke: s.Style,
	}
}

// StrokeStyle describes how a path should be stroked.
type StrokeStyle struct {
	Width float32 // Width of the stroked path.

	// Miter is the limit to apply to a miter joint.
	// The zero Miter disables the miter joint; setting Miter to +∞
	// unconditionally enables the miter joint.
	Miter float32
	Cap   StrokeCap  // Cap describes the head or tail of a stroked path.
	Join  StrokeJoin // Join describes how stroked paths are collated.
}

// StrokeCap describes the head or tail of a stroked path.
type StrokeCap uint8

const (
	// FlatCap caps stroked paths with a flat cap, joining the right-hand
	// and left-hand sides of a stroked path with a straight line.
	FlatCap StrokeCap = iota

	// SquareCap caps stroked paths with a square cap, joining the right-hand
	// and left-hand sides of a stroked path with a half square of length
	// the stroked path's width.
	SquareCap

	// RoundCap caps stroked paths with a round cap, joining the right-hand and
	// left-hand sides of a stroked path with a half disc of diameter the
	// stroked path's width.
	RoundCap
)

// StrokeJoin describes how stroked paths are collated.
type StrokeJoin uint8

const (
	// BevelJoin joins path segments with sharp bevels.
	BevelJoin StrokeJoin = iota

	// RoundJoin joins path segments with a round segment.
	RoundJoin
)
