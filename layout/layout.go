// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/op"
	"gioui.org/unit"
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

// Widget is a function scope for drawing, processing events and
// computing dimensions for a user interface element.
type Widget func()

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

// Inset adds space around a widget.
type Inset struct {
	Top, Right, Bottom, Left unit.Value
}

// Align aligns a widget in the available space.
type Align Direction

// Layout a widget.
func (in Inset) Layout(gtx *Context, w Widget) {
	top := gtx.Px(in.Top)
	right := gtx.Px(in.Right)
	bottom := gtx.Px(in.Bottom)
	left := gtx.Px(in.Left)
	mcs := gtx.Constraints
	mcs.Width.Max -= left + right
	if mcs.Width.Max < 0 {
		left = 0
		right = 0
		mcs.Width.Max = 0
	}
	if mcs.Width.Min > mcs.Width.Max {
		mcs.Width.Min = mcs.Width.Max
	}
	mcs.Height.Max -= top + bottom
	if mcs.Height.Max < 0 {
		bottom = 0
		top = 0
		mcs.Height.Max = 0
	}
	if mcs.Height.Min > mcs.Height.Max {
		mcs.Height.Min = mcs.Height.Max
	}
	var stack op.StackOp
	stack.Push(gtx.Ops)
	op.TransformOp{}.Offset(toPointF(image.Point{X: left, Y: top})).Add(gtx.Ops)
	dims := ctxLayout(gtx, mcs, w)
	stack.Pop()
	gtx.Dimensions = Dimensions{
		Size:     dims.Size.Add(image.Point{X: right + left, Y: top + bottom}),
		Baseline: dims.Baseline + bottom,
	}
}

// UniformInset returns an Inset with a single inset applied to all
// edges.
func UniformInset(v unit.Value) Inset {
	return Inset{Top: v, Right: v, Bottom: v, Left: v}
}

// Layout a widget.
func (a Align) Layout(gtx *Context, w Widget) {
	var macro op.MacroOp
	macro.Record(gtx.Ops)
	cs := gtx.Constraints
	mcs := cs
	mcs.Width.Min = 0
	mcs.Height.Min = 0
	dims := ctxLayout(gtx, mcs, w)
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
	var stack op.StackOp
	stack.Push(gtx.Ops)
	op.TransformOp{}.Offset(toPointF(p)).Add(gtx.Ops)
	macro.Add()
	stack.Pop()
	gtx.Dimensions = Dimensions{
		Size:     sz,
		Baseline: dims.Baseline + sz.Y - dims.Size.Y - p.Y,
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
