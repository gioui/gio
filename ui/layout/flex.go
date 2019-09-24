// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
	"gioui.org/ui/f32"
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

	ctx       *Context
	macro     ui.MacroOp
	mode      flexMode
	size      int
	rigidSize int
	// fraction is the rounding error from a Flexible weighting.
	fraction    float32
	maxCross    int
	maxBaseline int
}

// FlexChild is the layout result of a call End.
type FlexChild struct {
	macro ui.MacroOp
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

const (
	modeNone flexMode = iota
	modeBegun
	modeRigid
	modeFlex
)

// Init must be called before Rigid or Flexible.
func (f *Flex) Init(c *Context) *Flex {
	if f.mode > modeBegun {
		panic("must End the current child before calling Init again")
	}
	f.mode = modeBegun
	f.ctx = c
	f.size = 0
	f.rigidSize = 0
	f.maxCross = 0
	f.maxBaseline = 0
	return f
}

func (f *Flex) begin(mode flexMode) {
	switch {
	case f.mode == modeNone:
		panic("must Init before adding a child")
	case f.mode > modeBegun:
		panic("must End before adding a child")
	}
	f.mode = mode
	f.macro.Record(f.ctx.Ops)
}

// Rigid lays out a widget with the main axis constrained to the range
// from 0 to the remaining space.
func (f *Flex) Rigid(w Widget) FlexChild {
	f.begin(modeRigid)
	cs := f.ctx.Constraints
	mainc := axisMainConstraint(f.Axis, cs)
	mainMax := mainc.Max - f.size
	if mainMax < 0 {
		mainMax = 0
	}
	cs = axisConstraints(f.Axis, Constraint{Max: mainMax}, axisCrossConstraint(f.Axis, cs))
	dims := f.ctx.Layout(cs, w)
	return f.end(dims)
}

// Flexible is like Rigid, where the main axis size is also constrained to a
// fraction of the space not taken up by Rigid children.
func (f *Flex) Flexible(weight float32, w Widget) FlexChild {
	f.begin(modeFlex)
	cs := f.ctx.Constraints
	mainc := axisMainConstraint(f.Axis, cs)
	var flexSize int
	if mainc.Max > f.size {
		flexSize = mainc.Max - f.rigidSize
		// Apply weight and add any leftover fraction from a
		// previous Flexible.
		size := float32(flexSize)*weight + f.fraction
		flexSize = int(size + .5)
		f.fraction = size - float32(flexSize)
		if max := mainc.Max - f.size; flexSize > max {
			flexSize = max
		}
	}
	submainc := Constraint{Max: flexSize}
	cs = axisConstraints(f.Axis, submainc, axisCrossConstraint(f.Axis, cs))
	dims := f.ctx.Layout(cs, w)
	return f.end(dims)
}

// End a child by specifying its dimensions. Pass the returned layout result
// to Layout.
func (f *Flex) end(dims Dimensions) FlexChild {
	if f.mode <= modeBegun {
		panic("End called without an active child")
	}
	f.macro.Stop()
	sz := axisMain(f.Axis, dims.Size)
	f.size += sz
	if f.mode == modeRigid {
		f.rigidSize += sz
	}
	f.mode = modeBegun
	if c := axisCross(f.Axis, dims.Size); c > f.maxCross {
		f.maxCross = c
	}
	if b := dims.Baseline; b > f.maxBaseline {
		f.maxBaseline = b
	}
	return FlexChild{f.macro, dims}
}

// Layout a list of children. The order of the children determines their laid
// out order.
func (f *Flex) Layout(children ...FlexChild) {
	cs := f.ctx.Constraints
	mainc := axisMainConstraint(f.Axis, cs)
	crossSize := axisCrossConstraint(f.Axis, cs).Constrain(f.maxCross)
	var space int
	if mainc.Min > f.size {
		space = mainc.Min - f.size
	}
	var mainSize int
	var baseline int
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
		b := dims.Baseline
		var cross int
		switch f.Alignment {
		case End:
			cross = crossSize - axisCross(f.Axis, dims.Size)
		case Middle:
			cross = (crossSize - axisCross(f.Axis, dims.Size)) / 2
		case Baseline:
			if f.Axis == Horizontal {
				cross = f.maxBaseline - b
			}
		}
		var stack ui.StackOp
		stack.Push(f.ctx.Ops)
		ui.TransformOp{}.Offset(toPointF(axisPoint(f.Axis, mainSize, cross))).Add(f.ctx.Ops)
		child.macro.Add(f.ctx.Ops)
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
		if b != dims.Size.Y {
			baseline = b
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
	sz := axisPoint(f.Axis, mainSize, crossSize)
	if baseline == 0 {
		baseline = sz.Y
	}
	f.ctx.Dimensions = Dimensions{Size: sz, Baseline: baseline}
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
