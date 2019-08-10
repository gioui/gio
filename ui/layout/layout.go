// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
)

// Constraints represent a set of acceptable ranges for
// a widget's width and height.
type Constraints struct {
	Width  Constraint
	Height Constraint
}

// Constraint is a range of acceptable sizes in a single
// dimension.
type Constraint struct {
	Min, Max int
}

// Dimens are the resolved size and baseline for a user
// interface element.
type Dimens struct {
	Size     image.Point
	Baseline int
}

// Axis is the the Horizontal or Vertical direction.
type Axis uint8

// Alignment is the relative alignment of a list of
// interface elements.
type Alignment uint8

// Direction is the alignment of a set of interface elements
// relative to a containing space.
type Direction uint8

const (
	Start Alignment = iota
	End
	Middle
	Baseline
)

const (
	NW Direction = iota
	N
	NE
	E
	SE
	S
	SW
	W
	Center
)

const (
	Horizontal Axis = iota
	Vertical
)

// Constrain a value to the range [Min; Max].
func (c Constraint) Constrain(v int) int {
	if v < c.Min {
		return c.Min
	} else if v > c.Max {
		return c.Max
	}
	return v
}

// Constrain a size to the Width and Height ranges.
func (c Constraints) Constrain(size image.Point) image.Point {
	return image.Point{X: c.Width.Constrain(size.X), Y: c.Height.Constrain(size.Y)}
}

// RigidConstraints returns the constraints that can only be
// satisfied by the given dimensions.
func RigidConstraints(size image.Point) Constraints {
	return Constraints{
		Width:  Constraint{Min: size.X, Max: size.X},
		Height: Constraint{Min: size.Y, Max: size.Y},
	}
}

// Inset adds space around an interface element.
type Inset struct {
	Top, Right, Bottom, Left ui.Value

	stack                    ui.StackOp
	top, right, bottom, left int
	begun                    bool
	cs                       Constraints
}

// Align aligns an interface element in the available space.
type Align struct {
	Alignment Direction

	macro ui.MacroOp
	ops   *ui.Ops
	begun bool
	cs    Constraints
}

func (in *Inset) Begin(c ui.Config, ops *ui.Ops, cs Constraints) Constraints {
	if in.begun {
		panic("must End before Begin")
	}
	in.top = c.Px(in.Top)
	in.right = c.Px(in.Right)
	in.bottom = c.Px(in.Bottom)
	in.left = c.Px(in.Left)
	in.begun = true
	in.cs = cs
	mcs := cs
	mcs.Width.Min -= in.left + in.right
	mcs.Width.Max -= in.left + in.right
	if mcs.Width.Min < 0 {
		mcs.Width.Min = 0
	}
	if mcs.Width.Max < mcs.Width.Min {
		mcs.Width.Max = mcs.Width.Min
	}
	mcs.Height.Min -= in.top + in.bottom
	mcs.Height.Max -= in.top + in.bottom
	if mcs.Height.Min < 0 {
		mcs.Height.Min = 0
	}
	if mcs.Height.Max < mcs.Height.Min {
		mcs.Height.Max = mcs.Height.Min
	}
	in.stack.Push(ops)
	ui.TransformOp{}.Offset(toPointF(image.Point{X: in.left, Y: in.top})).Add(ops)
	return mcs
}

func (in *Inset) End(dims Dimens) Dimens {
	if !in.begun {
		panic("must Begin before End")
	}
	in.begun = false
	in.stack.Pop()
	return Dimens{
		Size:     in.cs.Constrain(dims.Size.Add(image.Point{X: in.right + in.left, Y: in.top + in.bottom})),
		Baseline: dims.Baseline + in.top,
	}
}

// UniformInset returns an Inset with a single inset applied to all
// edges.
func UniformInset(v ui.Value) Inset {
	return Inset{Top: v, Right: v, Bottom: v, Left: v}
}

func (a *Align) Begin(ops *ui.Ops, cs Constraints) Constraints {
	if a.begun {
		panic("must End before Begin")
	}
	a.begun = true
	a.ops = ops
	a.cs = cs
	a.macro.Record(ops)
	cs.Width.Min = 0
	cs.Height.Min = 0
	return cs
}

func (a *Align) End(dims Dimens) Dimens {
	if !a.begun {
		panic("must Begin before End")
	}
	a.begun = false
	ops := a.ops
	a.macro.Stop()
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
	var stack ui.StackOp
	stack.Push(ops)
	ui.TransformOp{}.Offset(toPointF(p)).Add(ops)
	a.macro.Add(ops)
	stack.Pop()
	return Dimens{
		Size:     sz,
		Baseline: dims.Baseline,
	}
}

func (a Alignment) String() string {
	switch a {
	case Start:
		return "Start"
	case End:
		return "End"
	case Middle:
		return "Middle"
	case Baseline:
		return "Baseline"
	default:
		panic("unreachable")
	}
}

func (a Axis) String() string {
	switch a {
	case Horizontal:
		return "Horizontal"
	case Vertical:
		return "Vertical"
	default:
		panic("unreachable")
	}
}

func (d Direction) String() string {
	switch d {
	case NW:
		return "NW"
	case N:
		return "N"
	case NE:
		return "NE"
	case E:
		return "E"
	case SE:
		return "SE"
	case S:
		return "S"
	case SW:
		return "SW"
	case W:
		return "W"
	case Center:
		return "Center"
	default:
		panic("unreachable")
	}
}
