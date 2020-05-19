// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/unit"
)

// Constraints represent the minimum and maximum size of a widget.
type Constraints struct {
	Min, Max image.Point
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

// Exact returns the Constraints with the minimum and maximum size
// set to size.
func Exact(size image.Point) Constraints {
	return Constraints{
		Min: size, Max: size,
	}
}

// FPt converts an point to a f32.Point.
func FPt(p image.Point) f32.Point {
	return f32.Point{
		X: float32(p.X), Y: float32(p.Y),
	}
}

// FRect converts a rectangle to a f32.Rectangle.
func FRect(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: FPt(r.Min), Max: FPt(r.Max),
	}
}

// Constrain a size so each dimension is in the range [min;max].
func (c Constraints) Constrain(size image.Point) image.Point {
	if min := c.Min.X; size.X < min {
		size.X = min
	}
	if min := c.Min.Y; size.Y < min {
		size.Y = min
	}
	if max := c.Max.X; size.X > max {
		size.X = max
	}
	if max := c.Max.Y; size.Y > max {
		size.Y = max
	}
	return size
}

// Inset adds space around a widget.
type Inset struct {
	Top, Right, Bottom, Left unit.Value
}

// Layout a widget.
func (in Inset) Layout(gtx *Context, w Widget) {
	top := gtx.Px(in.Top)
	right := gtx.Px(in.Right)
	bottom := gtx.Px(in.Bottom)
	left := gtx.Px(in.Left)
	mcs := gtx.Constraints
	mcs.Max.X -= left + right
	if mcs.Max.X < 0 {
		left = 0
		right = 0
		mcs.Max.X = 0
	}
	if mcs.Min.X > mcs.Max.X {
		mcs.Min.X = mcs.Max.X
	}
	mcs.Max.Y -= top + bottom
	if mcs.Max.Y < 0 {
		bottom = 0
		top = 0
		mcs.Max.Y = 0
	}
	if mcs.Min.Y > mcs.Max.Y {
		mcs.Min.Y = mcs.Max.Y
	}
	var stack op.StackOp
	stack.Push(gtx.Ops)
	op.TransformOp{}.Offset(FPt(image.Point{X: left, Y: top})).Add(gtx.Ops)
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

// Layout a widget according to the direction.
func (a Direction) Layout(gtx *Context, w Widget) {
	var macro op.MacroOp
	macro.Record(gtx.Ops)
	cs := gtx.Constraints
	mcs := cs
	mcs.Min = image.Point{}
	dims := ctxLayout(gtx, mcs, w)
	macro.Stop()
	sz := dims.Size
	if sz.X < cs.Min.X {
		sz.X = cs.Min.X
	}
	if sz.Y < cs.Min.Y {
		sz.Y = cs.Min.Y
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
	op.TransformOp{}.Offset(FPt(p)).Add(gtx.Ops)
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
