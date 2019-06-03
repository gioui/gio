// SPDX-License-Identifier: Unlicense OR MIT

package ui

import (
	"encoding/binary"
	"math"
	"time"

	"gioui.org/ui/f32"
	"gioui.org/ui/internal/ops"
)

// Config contain the context for updating and
// drawing a user interface.
type Config struct {
	// Device pixels per dp.
	PxPerDp float32
	// Device pixels per sp.
	PxPerSp float32
	// The current time for animation.
	Now time.Time
}

// Dp converts a value in dp units to pixels.
func (c *Config) Dp(dp float32) float32 {
	return c.PxPerDp * dp
}

// Sp converts a value in sp units to pixels.
func (c *Config) Sp(sp float32) float32 {
	return c.PxPerSp * sp
}

// Val converts a value to pixels.
func (c *Config) Val(v Value) float32 {
	switch v.U {
	case UnitPx:
		return v.V
	case UnitDp:
		return c.PxPerDp * v.V
	case UnitSp:
		return c.PxPerSp * v.V
	default:
		panic("unknown unit")
	}
}

// OpLayer represents a semantic layer of UI.
type OpLayer struct {
}

// OpRedraw requests a redraw at the given time. Use
// the zero value to request an immediate redraw.
type OpRedraw struct {
	At time.Time
}

// OpTransform transforms an op.
type OpTransform struct {
	Transform Transform
}

type Transform struct {
	// TODO: general transforms.
	offset f32.Point
}

func (r OpRedraw) Add(o *Ops) {
	data := make([]byte, ops.TypeRedrawLen)
	data[0] = byte(ops.TypeRedraw)
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

func (r *OpRedraw) Decode(d []byte) {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypeRedraw {
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

func (t OpTransform) Add(o *Ops) {
	data := make([]byte, ops.TypeTransformLen)
	data[0] = byte(ops.TypeTransform)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(t.Transform.offset.X))
	bo.PutUint32(data[5:], math.Float32bits(t.Transform.offset.Y))
	o.Write(data)
}

func (t *OpTransform) Decode(d []byte) {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypeTransform {
		panic("invalid op")
	}
	*t = OpTransform{
		Transform: Offset(f32.Point{
			X: math.Float32frombits(bo.Uint32(d[1:])),
			Y: math.Float32frombits(bo.Uint32(d[5:])),
		}),
	}
}

func (l OpLayer) Add(o *Ops) {
	data := make([]byte, ops.TypeLayerLen)
	data[0] = byte(ops.TypeLayer)
	o.Write(data)
}

func (l *OpLayer) Decode(d []byte) {
	if ops.OpType(d[0]) != ops.TypeLayer {
		panic("invalid op")
	}
	*l = OpLayer{}
}

func Offset(o f32.Point) Transform {
	return Transform{o}
}

// Inf is the int value that represents an unbounded maximum constraint.
const Inf = int(^uint(0) >> 1)
