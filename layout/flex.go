// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/f32"
	"gioui.org/op"
)

// Flex lays out child elements along an axis,
// according to alignment and weights.
type Flex struct {
	// Axis is the main axis, either Horizontal or Vertical.
	Axis Axis
	// Spacing controls the distribution of space left after
	// layout.
	Spacing Spacing
	// Alignment is the alignment in the cross axis.
	Alignment Alignment

	size      int
	rigidSize int
	// fraction is the rounding error from a Flex weighting.
	fraction float32

	// Use an empty StackOp for tracking whether Rigid, Flex
	// is called in the same layout scope as Layout.
	begun bool
	stack op.StackOp
}

// FlexChild is the layout result of a call End.
type FlexChild struct {
	macro op.MacroOp
	dims  Dimensions
}

// Spacing determine the spacing mode for a Flex.
type Spacing uint8

type flexMode uint8

const (
	// SpaceEnd leaves space at the end.
	SpaceEnd Spacing = iota
	// SpaceStart leaves space at the start.
	SpaceStart
	// SpaceSides shares space between the start and end.
	SpaceSides
	// SpaceAround distributes space evenly between children,
	// with half as much space at the start and end.
	SpaceAround
	// SpaceBetween distributes space evenly between children,
	// leaving no space at the start and end.
	SpaceBetween
	// SpaceEvenly distributes space evenly between children and
	// at the start and end.
	SpaceEvenly
)

// Rigid lays out a widget with the main axis constrained to the range
// from 0 to the remaining space.
func (f *Flex) Rigid(gtx *Context, w Widget) FlexChild {
	f.begin(gtx.Ops)
	cs := gtx.Constraints
	mainc := axisMainConstraint(f.Axis, cs)
	mainMax := mainc.Max - f.size
	if mainMax < 0 {
		mainMax = 0
	}
	cs = axisConstraints(f.Axis, Constraint{Max: mainMax}, axisCrossConstraint(f.Axis, cs))
	var m op.MacroOp
	m.Record(gtx.Ops)
	dims := ctxLayout(gtx, cs, w)
	m.Stop()
	f.rigidSize += axisMain(f.Axis, dims.Size)
	f.expand(dims)
	return FlexChild{m, dims}
}

func (f *Flex) begin(ops *op.Ops) {
	if f.begun {
		return
	}
	f.stack.Push(ops)
	f.begun = true
}

// Flex is like Rigid, where the main axis size is also constrained to a
// fraction of the space not taken up by Rigid children.
func (f *Flex) Flex(gtx *Context, weight float32, w Widget) FlexChild {
	f.begin(gtx.Ops)
	cs := gtx.Constraints
	mainc := axisMainConstraint(f.Axis, cs)
	var flexSize int
	if mainc.Max > f.size {
		flexSize = mainc.Max - f.rigidSize
		// Apply weight and add any leftover fraction from a
		// previous Flex.
		size := float32(flexSize)*weight + f.fraction
		flexSize = int(size + .5)
		f.fraction = size - float32(flexSize)
		if max := mainc.Max - f.size; flexSize > max {
			flexSize = max
		}
	}
	submainc := Constraint{Min: flexSize, Max: flexSize}
	cs = axisConstraints(f.Axis, submainc, axisCrossConstraint(f.Axis, cs))
	var m op.MacroOp
	m.Record(gtx.Ops)
	dims := ctxLayout(gtx, cs, w)
	m.Stop()
	f.expand(dims)
	return FlexChild{m, dims}
}

// End a child by specifying its dimensions. Pass the returned layout result
// to Layout.
func (f *Flex) expand(dims Dimensions) {
	sz := axisMain(f.Axis, dims.Size)
	f.size += sz
}

// Layout a list of children. The order of the children determines their laid
// out order.
func (f *Flex) Layout(gtx *Context, children ...FlexChild) {
	if len(children) > 0 {
		f.stack.Pop()
	}
	var maxCross int
	var maxBaseline int
	for _, child := range children {
		if c := axisCross(f.Axis, child.dims.Size); c > maxCross {
			maxCross = c
		}
		if b := child.dims.Size.Y - child.dims.Baseline; b > maxBaseline {
			maxBaseline = b
		}
	}
	cs := gtx.Constraints
	mainc := axisMainConstraint(f.Axis, cs)
	var space int
	if mainc.Min > f.size {
		space = mainc.Min - f.size
	}
	var mainSize int
	switch f.Spacing {
	case SpaceSides:
		mainSize += space / 2
	case SpaceStart:
		mainSize += space
	case SpaceEvenly:
		mainSize += space / (1 + len(children))
	case SpaceAround:
		mainSize += space / (len(children) * 2)
	}
	for i, child := range children {
		dims := child.dims
		b := dims.Size.Y - dims.Baseline
		var cross int
		switch f.Alignment {
		case End:
			cross = maxCross - axisCross(f.Axis, dims.Size)
		case Middle:
			cross = (maxCross - axisCross(f.Axis, dims.Size)) / 2
		case Baseline:
			if f.Axis == Horizontal {
				cross = maxBaseline - b
			}
		}
		var stack op.StackOp
		stack.Push(gtx.Ops)
		op.TransformOp{}.Offset(toPointF(axisPoint(f.Axis, mainSize, cross))).Add(gtx.Ops)
		child.macro.Add(gtx.Ops)
		stack.Pop()
		mainSize += axisMain(f.Axis, dims.Size)
		if i < len(children)-1 {
			switch f.Spacing {
			case SpaceEvenly:
				mainSize += space / (1 + len(children))
			case SpaceAround:
				mainSize += space / len(children)
			case SpaceBetween:
				mainSize += space / (len(children) - 1)
			}
		}
	}
	switch f.Spacing {
	case SpaceSides:
		mainSize += space / 2
	case SpaceEnd:
		mainSize += space
	case SpaceEvenly:
		mainSize += space / (1 + len(children))
	case SpaceAround:
		mainSize += space / (len(children) * 2)
	}
	sz := axisPoint(f.Axis, mainSize, maxCross)
	gtx.Dimensions = Dimensions{Size: sz, Baseline: sz.Y - maxBaseline}
	f.begun = false
	f.size = 0
	f.rigidSize = 0
}

func axisPoint(a Axis, main, cross int) image.Point {
	if a == Horizontal {
		return image.Point{main, cross}
	} else {
		return image.Point{cross, main}
	}
}

func axisMain(a Axis, sz image.Point) int {
	if a == Horizontal {
		return sz.X
	} else {
		return sz.Y
	}
}

func axisCross(a Axis, sz image.Point) int {
	if a == Horizontal {
		return sz.Y
	} else {
		return sz.X
	}
}

func axisMainConstraint(a Axis, cs Constraints) Constraint {
	if a == Horizontal {
		return cs.Width
	} else {
		return cs.Height
	}
}

func axisCrossConstraint(a Axis, cs Constraints) Constraint {
	if a == Horizontal {
		return cs.Height
	} else {
		return cs.Width
	}
}

func axisConstraints(a Axis, mainc, crossc Constraint) Constraints {
	if a == Horizontal {
		return Constraints{Width: mainc, Height: crossc}
	} else {
		return Constraints{Width: crossc, Height: mainc}
	}
}

func toPointF(p image.Point) f32.Point {
	return f32.Point{X: float32(p.X), Y: float32(p.Y)}
}

func (s Spacing) String() string {
	switch s {
	case SpaceEnd:
		return "SpaceEnd"
	case SpaceStart:
		return "SpaceStart"
	case SpaceSides:
		return "SpaceSides"
	case SpaceAround:
		return "SpaceAround"
	case SpaceBetween:
		return "SpaceAround"
	case SpaceEvenly:
		return "SpaceEvenly"
	default:
		panic("unreachable")
	}
}
