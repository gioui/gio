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

	macro       ui.MacroOp
	ops         *ui.Ops
	cs          Constraints
	mode        flexMode
	size        int
	rigidSize   int
	maxCross    int
	maxBaseline int
}

// FlexChild is the layout result of a call End.
type FlexChild struct {
	macro ui.MacroOp
	dims  Dimens
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
func (f *Flex) Init(ops *ui.Ops, cs Constraints) *Flex {
	if f.mode > modeBegun {
		panic("must End the current child before calling Init again")
	}
	f.mode = modeBegun
	f.ops = ops
	f.cs = cs
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
	f.macro.Record(f.ops)
}

// Rigid begins a child and return its constraints. The main axis is constrained
// to the range from 0 to the remaining space.
func (f *Flex) Rigid() Constraints {
	f.begin(modeRigid)
	mainc := axisMainConstraint(f.Axis, f.cs)
	mainMax := mainc.Max - f.size
	if mainMax < 0 {
		mainMax = 0
	}
	return axisConstraints(f.Axis, Constraint{Max: mainMax}, axisCrossConstraint(f.Axis, f.cs))
}

// Flexible is like Rigid, where the main axis size is also constrained to a
// fraction of the space not taken up by Rigid children.
func (f *Flex) Flexible(weight float32) Constraints {
	f.begin(modeFlex)
	mainc := axisMainConstraint(f.Axis, f.cs)
	var flexSize int
	if mainc.Max > f.size {
		maxSize := mainc.Max - f.size
		flexSize = mainc.Max - f.rigidSize
		flexSize = int(float32(flexSize)*weight + .5)
		if flexSize > maxSize {
			flexSize = maxSize
		}
	}
	submainc := Constraint{Max: flexSize}
	return axisConstraints(f.Axis, submainc, axisCrossConstraint(f.Axis, f.cs))
}

// End a child by specifying its dimensions. Pass the returned layout result
// to Layout.
func (f *Flex) End(dims Dimens) FlexChild {
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
func (f *Flex) Layout(children ...FlexChild) Dimens {
	mainc := axisMainConstraint(f.Axis, f.cs)
	crossSize := axisCrossConstraint(f.Axis, f.cs).Constrain(f.maxCross)
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
		stack.Push(f.ops)
		ui.TransformOp{}.Offset(toPointF(axisPoint(f.Axis, mainSize, cross))).Add(f.ops)
		child.macro.Add(f.ops)
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
	return Dimens{Size: sz, Baseline: baseline}
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
		return Constraints{mainc, crossc}
	} else {
		return Constraints{crossc, mainc}
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
