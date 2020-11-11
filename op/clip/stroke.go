// SPDX-License-Identifier: Unlicense OR MIT

package clip

// StrokeStyle describes how a stroked path should be drawn.
// The zero value of StrokeStyle represents bevel-joined and flat-capped
// strokes.
type StrokeStyle struct {
	Cap  StrokeCap
	Join StrokeJoin

	// Miter is the limit to apply to a miter joint.
	// The zero Miter disables the miter joint; setting Miter to +∞
	// unconditionally enables the miter joint.
	Miter float32
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
