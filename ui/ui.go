// SPDX-License-Identifier: Unlicense OR MIT

package ui

import (
	"encoding/binary"
	"math"
	"time"

	"gioui.org/ui/f32"
	"gioui.org/ui/internal/ops"
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
	Transform Transform
}

type Transform struct {
	// TODO: general transforms.
	offset f32.Point
}

// Inf is the int value that represents an unbounded maximum constraint.
const Inf = int(^uint(0) >> 1)

func (r InvalidateOp) Add(o *Ops) {
	data := make([]byte, ops.TypeRedrawLen)
	data[0] = byte(ops.TypeInvalidate)
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

func (r *InvalidateOp) Decode(d []byte) {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypeInvalidate {
		panic("invalid op")
	}
	if nanos := bo.Uint64(d[1:]); nanos > 0 {
		r.At = time.Unix(0, int64(nanos))
	}
}

func (t Transform) InvTransform(p f32.Point) f32.Point {
	return p.Sub(t.offset)
}

func (t Transform) Transform(p f32.Point) f32.Point {
	return p.Add(t.offset)
}

func (t Transform) Mul(t2 Transform) Transform {
	return Transform{
		offset: t.offset.Add(t2.offset),
	}
}

func (t TransformOp) Add(o *Ops) {
	data := make([]byte, ops.TypeTransformLen)
	data[0] = byte(ops.TypeTransform)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(t.Transform.offset.X))
	bo.PutUint32(data[5:], math.Float32bits(t.Transform.offset.Y))
	o.Write(data)
}

func (t *TransformOp) Decode(d []byte) {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypeTransform {
		panic("invalid op")
	}
	*t = TransformOp{
		Transform: Offset(f32.Point{
			X: math.Float32frombits(bo.Uint32(d[1:])),
			Y: math.Float32frombits(bo.Uint32(d[5:])),
		}),
	}
}

func Offset(o f32.Point) Transform {
	return Transform{o}
}
