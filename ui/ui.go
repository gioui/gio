// SPDX-License-Identifier: Unlicense OR MIT

package ui

import (
	"time"

	"gioui.org/ui/f32"
)

type Config struct {
	PxPerDp float32
	PxPerSp float32
	Now     time.Time
}

type Op interface {
	ImplementsOp()
}

type OpLayer struct {
	Op Op
}

type OpRedraw struct {
	At time.Time
}

type Ops []Op

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
