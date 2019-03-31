// SPDX-License-Identifier: Unlicense OR MIT

package ui

import (
	"time"

	"gioui.org/ui/f32"
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

// Pixels converts a value to unitless device pixels.
func (c *Config) Pixels(v Value) float32 {
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

// Op is implemented by all known drawing and control
// operations.
type Op interface {
	ImplementsOp()
}

// OpLayer represents a semantic layer of UI.
type OpLayer struct {
	Op Op
}

// OpRedraw requests a redraw at the given time. Use
// the zero value to request an immediate redraw.
type OpRedraw struct {
	At time.Time
}

// Ops is the operation for a list of ops.
type Ops []Op

// OpTransform transforms an op.
type OpTransform struct {
	Transform Transform
	Op        Op
}

type Transform struct {
	// TODO: general transforms.
	offset f32.Point
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

func (t OpTransform) ChildOp() Op {
	return t.Op
}

func (o OpLayer) ChildOp() Op {
	return o.Op
}

func Offset(o f32.Point) Transform {
	return Transform{o}
}

// Inf is the int value that represents an unbounded maximum constraint.
const Inf = int(^uint(0) >> 1)

func (Ops) ImplementsOp()         {}
func (OpLayer) ImplementsOp()     {}
func (OpTransform) ImplementsOp() {}
func (OpRedraw) ImplementsOp()    {}
