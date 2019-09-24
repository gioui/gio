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

// Dimensions are the resolved size and baseline for a widget.
type Dimensions struct {
	Size     image.Point
	Baseline int
}

// Axis is the Horizontal or Vertical direction.
type Axis uint8

// Alignment is the mutual alignment of a list of widgets.
type Alignment uint8

// Direction is the alignment of widgets relative to a containing
// space.
type Direction uint8

// Widget is a function scope for drawing, processing input and
// computing dimensions for a user interface element.
type Widget func()

// Context tracks the current constraints and dimensions during
// layout.
type Context struct {
	Constraints Constraints
	Dimensions  Dimensions
}

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

// Layout a widget with a set of constraints and return its
// dimensions. The previous constraints are restored after layout.
func (s *Context) Layout(cs Constraints, w Widget) Dimensions {
	saved := s.Constraints
	s.Constraints = cs
	s.Dimensions = Dimensions{}
	w()
	s.Constraints = saved
	return s.Dimensions
}

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

// Inset adds space around a widget.
type Inset struct {
	Top, Right, Bottom, Left ui.Value
}

// Align aligns a widget in the available space.
type Align Direction

// Layout a widget.
func (in Inset) Layout(c ui.Config, ops *ui.Ops, ctx *Context, w Widget) {
	top := c.Px(in.Top)
	right := c.Px(in.Right)
	bottom := c.Px(in.Bottom)
	left := c.Px(in.Left)
	mcs := ctx.Constraints
	mcs.Width.Min -= left + right
	mcs.Width.Max -= left + right
	if mcs.Width.Min < 0 {
		mcs.Width.Min = 0
	}
	if mcs.Width.Max < mcs.Width.Min {
		mcs.Width.Max = mcs.Width.Min
	}
	mcs.Height.Min -= top + bottom
	mcs.Height.Max -= top + bottom
	if mcs.Height.Min < 0 {
		mcs.Height.Min = 0
	}
	if mcs.Height.Max < mcs.Height.Min {
		mcs.Height.Max = mcs.Height.Min
	}
	var stack ui.StackOp
	stack.Push(ops)
	ui.TransformOp{}.Offset(toPointF(image.Point{X: left, Y: top})).Add(ops)
	dims := ctx.Layout(mcs, w)
	stack.Pop()
	ctx.Dimensions = Dimensions{
		Size:     ctx.Constraints.Constrain(dims.Size.Add(image.Point{X: right + left, Y: top + bottom})),
		Baseline: dims.Baseline + top,
	}
}

// UniformInset returns an Inset with a single inset applied to all
// edges.
func UniformInset(v ui.Value) Inset {
	return Inset{Top: v, Right: v, Bottom: v, Left: v}
}

// Layout a widget.
func (a Align) Layout(ops *ui.Ops, st *Context, w Widget) {
	var macro ui.MacroOp
	macro.Record(ops)
	cs := st.Constraints
	mcs := cs
	mcs.Width.Min = 0
	mcs.Height.Min = 0
	dims := st.Layout(mcs, w)
	macro.Stop()
	sz := dims.Size
	if sz.X < cs.Width.Min {
		sz.X = cs.Width.Min
	}
	if sz.Y < cs.Height.Min {
		sz.Y = cs.Height.Min
	}
	var p image.Point
	switch Direction(a) {
	case N, S, Center:
		p.X = (sz.X - dims.Size.X) / 2
	case NE, SE, E:
		p.X = sz.X - dims.Size.X
	}
	switch Direction(a) {
	case W, Center, E:
		p.Y = (sz.Y - dims.Size.Y) / 2
	case SW, S, SE:
		p.Y = sz.Y - dims.Size.Y
	}
	var stack ui.StackOp
	stack.Push(ops)
	ui.TransformOp{}.Offset(toPointF(p)).Add(ops)
	macro.Add(ops)
	stack.Pop()
	st.Dimensions = Dimensions{
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
