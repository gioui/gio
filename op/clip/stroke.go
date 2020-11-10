// SPDX-License-Identifier: Unlicense OR MIT

package clip

// StrokeStyle describes how a stroked path should be drawn.
// StrokeStyle zero value draws a Bevel-joined and Flat-capped stroked path.
type StrokeStyle struct {
	Cap StrokeCap
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
