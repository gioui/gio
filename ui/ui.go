// SPDX-License-Identifier: Unlicense OR MIT

package ui

import (
	"encoding/binary"
	"math"
	"time"

	"gioui.org/ui/f32"
	"gioui.org/ui/internal/opconst"
)

// Config represents the essential configuration for
// updating and drawing a user interface.
type Config interface {
	// Now returns the current animation time.
	Now() time.Time
	// Px converts a Value to pixels.
	Px(v Value) int
}

// InvalidateOp requests a redraw at the given time. Use
// the zero value to request an immediate redraw.
type InvalidateOp struct {
	At time.Time
}

// TransformOp applies a transform to later ops.
type TransformOp struct {
	// TODO: general transformations.
	offset f32.Point
}

func (r InvalidateOp) Add(o *Ops) {
	data := make([]byte, opconst.TypeRedrawLen)
	data[0] = byte(opconst.TypeInvalidate)
	bo := binary.LittleEndian
	// UnixNano cannot represent the zero time.
	if t := r.At; !t.IsZero() {
		nanos := t.UnixNano()
		if nanos > 0 {
			bo.PutUint64(data[1:], uint64(nanos))
		}
	}
	o.Write(data)
}

// Offset the transformation.
func (t TransformOp) Offset(o f32.Point) TransformOp {
	return t.Multiply(TransformOp{o})
}

// Invert the transformation.
func (t TransformOp) Invert() TransformOp {
	return TransformOp{offset: t.offset.Mul(-1)}
}

// Transform a point.
func (t TransformOp) Transform(p f32.Point) f32.Point {
	return p.Add(t.offset)
}

// Multiply by a transformation.
func (t TransformOp) Multiply(t2 TransformOp) TransformOp {
	return TransformOp{
		offset: t.offset.Add(t2.offset),
	}
}

func (t TransformOp) Add(o *Ops) {
	data := make([]byte, opconst.TypeTransformLen)
	data[0] = byte(opconst.TypeTransform)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(t.offset.X))
	bo.PutUint32(data[5:], math.Float32bits(t.offset.Y))
	o.Write(data)
}
