// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
)

type Constraints struct {
	Width  Constraint
	Height Constraint
}

type Constraint struct {
	Min, Max int
}

type Dimens struct {
	Size     image.Point
	Baseline int
}

type Axis uint8

const (
	Horizontal Axis = iota
	Vertical
)

func (c Constraint) Constrain(v int) int {
	if v < c.Min {
		return c.Min
	} else if v > c.Max {
		return c.Max
	}
	return v
}

func (c Constraints) Constrain(p image.Point) image.Point {
	return image.Point{X: c.Width.Constrain(p.X), Y: c.Height.Constrain(p.Y)}
}

func (c Constraints) Expand() Constraints {
	return Constraints{Width: c.Width.expand(), Height: c.Height.expand()}
}

func (c Constraints) Loose() Constraints {
	return Constraints{Width: c.Width.loose(), Height: c.Height.loose()}
}

func (c Constraint) expand() Constraint {
	return Constraint{Min: c.Max, Max: c.Max}
}

func (c Constraint) loose() Constraint {
	return Constraint{Max: c.Max}
}

// RigidConstraints returns the constraints that can only be
// satisfied by the given dimensions.
func RigidConstraints(size image.Point) Constraints {
	return Constraints{
		Width:  Constraint{Min: size.X, Max: size.X},
		Height: Constraint{Min: size.Y, Max: size.Y},
	}
}

type Insets struct {
	Top, Right, Bottom, Left ui.Value

	top, right, bottom, left int
	ops                      *ui.Ops
	begun                    bool
	cs                       Constraints
}

func (in *Insets) Begin(c ui.Config, ops *ui.Ops, cs Constraints) Constraints {
	if in.begun {
		panic("must End before Begin")
	}
	in.top = c.Px(in.Top)
	in.right = c.Px(in.Right)
	in.bottom = c.Px(in.Bottom)
	in.left = c.Px(in.Left)
	in.begun = true
	in.ops = ops
	in.cs = cs
	mcs := cs
	if mcs.Width.Max != ui.Inf {
		mcs.Width.Min -= in.left + in.right
		mcs.Width.Max -= in.left + in.right
		if mcs.Width.Min < 0 {
			mcs.Width.Min = 0
		}
		if mcs.Width.Max < mcs.Width.Min {
			mcs.Width.Max = mcs.Width.Min
		}
	}
	if mcs.Height.Max != ui.Inf {
		mcs.Height.Min -= in.top + in.bottom
		mcs.Height.Max -= in.top + in.bottom
		if mcs.Height.Min < 0 {
			mcs.Height.Min = 0
		}
		if mcs.Height.Max < mcs.Height.Min {
			mcs.Height.Max = mcs.Height.Min
		}
	}
	ui.PushOp{}.Add(ops)
	ui.TransformOp{Transform: ui.Offset(toPointF(image.Point{X: in.left, Y: in.top}))}.Add(ops)
	return mcs
}

func (in *Insets) End(dims Dimens) Dimens {
	if !in.begun {
		panic("must Begin before End")
	}
	in.begun = false
	ops := in.ops
	ui.PopOp{}.Add(ops)
	return Dimens{
		Size:     in.cs.Constrain(dims.Size.Add(image.Point{X: in.right + in.left, Y: in.top + in.bottom})),
		Baseline: dims.Baseline + in.top,
	}
}

func EqualInsets(v ui.Value) Insets {
	return Insets{Top: v, Right: v, Bottom: v, Left: v}
}

type Align struct {
	Alignment Direction

	ops   *ui.Ops
	begun bool
	cs    Constraints
}

func (a *Align) Begin(ops *ui.Ops, cs Constraints) Constraints {
	if a.begun {
		panic("must End before Begin")
	}
	a.begun = true
	a.ops = ops
	a.cs = cs
	ops.Begin()
	return cs.Loose()
}

func (a *Align) End(dims Dimens) Dimens {
	if !a.begun {
		panic("must Begin before End")
	}
	a.begun = false
	ops := a.ops
	block := ops.End()
	sz := dims.Size
	if sz.X < a.cs.Width.Min {
		sz.X = a.cs.Width.Min
	}
	if sz.Y < a.cs.Height.Min {
		sz.Y = a.cs.Height.Min
	}
	var p image.Point
	switch a.Alignment {
	case N, S, Center:
		p.X = (sz.X - dims.Size.X) / 2
	case NE, SE, E:
		p.X = sz.X - dims.Size.X
	}
	switch a.Alignment {
	case W, Center, E:
		p.Y = (sz.Y - dims.Size.Y) / 2
	case SW, S, SE:
		p.Y = sz.Y - dims.Size.Y
	}
	ui.PushOp{}.Add(ops)
	ui.TransformOp{Transform: ui.Offset(toPointF(p))}.Add(ops)
	block.Add(ops)
	ui.PopOp{}.Add(ops)
	return Dimens{
		Size:     sz,
		Baseline: dims.Baseline,
	}
}
