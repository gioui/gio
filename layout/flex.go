// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

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
}

// FlexChild is the descriptor for a Flex child.
type FlexChild struct {
	flex   bool
	weight float32

	widget Widget

	// Scratch space.
	call op.CallOp
	dims Dimensions
}

// Spacing determine the spacing mode for a Flex.
type Spacing uint8

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

// Rigid returns a Flex child with a maximal constraint of the
// remaining space.
func Rigid(widget Widget) FlexChild {
	return FlexChild{
		widget: widget,
	}
}

// Flexed returns a Flex child forced to take up a fraction of
// the remaining space.
func Flexed(weight float32, widget Widget) FlexChild {
	return FlexChild{
		flex:   true,
		weight: weight,
		widget: widget,
	}
}

// Layout a list of children. The position of the children are
// determined by the specified order, but Rigid children are laid out
// before Flexed children.
func (f Flex) Layout(gtx Context, children ...FlexChild) Dimensions {
	size := 0
	// Lay out Rigid children.
	for i, child := range children {
		if child.flex {
			continue
		}
		cs := gtx.Constraints
		_, mainMax := axisMainConstraint(f.Axis, cs)
		mainMax -= size
		if mainMax < 0 {
			mainMax = 0
		}
		crossMin, crossMax := axisCrossConstraint(f.Axis, cs)
		cs = axisConstraints(f.Axis, 0, mainMax, crossMin, crossMax)
		macro := op.Record(gtx.Ops)
		gtx := gtx
		gtx.Constraints = cs
		dims := child.widget(gtx)
		c := macro.Stop()
		sz := axisMain(f.Axis, dims.Size)
		size += sz
		children[i].call = c
		children[i].dims = dims
	}
	rigidSize := size
	// fraction is the rounding error from a Flex weighting.
	var fraction float32
	// Lay out Flexed children.
	for i, child := range children {
		if !child.flex {
			continue
		}
		cs := gtx.Constraints
		_, mainMax := axisMainConstraint(f.Axis, cs)
		var flexSize int
		if mainMax > size {
			flexSize = mainMax - rigidSize
			// Apply weight and add any leftover fraction from a
			// previous Flexed.
			childSize := float32(flexSize)*child.weight + fraction
			flexSize = int(childSize + .5)
			fraction = childSize - float32(flexSize)
			if max := mainMax - size; flexSize > max {
				flexSize = max
			}
		}
		crossMin, crossMax := axisCrossConstraint(f.Axis, cs)
		cs = axisConstraints(f.Axis, flexSize, flexSize, crossMin, crossMax)
		macro := op.Record(gtx.Ops)
		gtx := gtx
		gtx.Constraints = cs
		dims := child.widget(gtx)
		c := macro.Stop()
		sz := axisMain(f.Axis, dims.Size)
		size += sz
		children[i].call = c
		children[i].dims = dims
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
	mainMin, _ := axisMainConstraint(f.Axis, cs)
	var space int
	if mainMin > size {
		space = mainMin - size
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
		stack := op.Push(gtx.Ops)
		op.TransformOp{}.Offset(FPt(axisPoint(f.Axis, mainSize, cross))).Add(gtx.Ops)
		child.call.Add(gtx.Ops)
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
	return Dimensions{Size: sz, Baseline: sz.Y - maxBaseline}
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

func axisMainConstraint(a Axis, cs Constraints) (int, int) {
	if a == Horizontal {
		return cs.Min.X, cs.Max.X
	} else {
		return cs.Min.Y, cs.Max.Y
	}
}

func axisCrossConstraint(a Axis, cs Constraints) (int, int) {
	if a == Horizontal {
		return cs.Min.Y, cs.Max.Y
	} else {
		return cs.Min.X, cs.Max.X
	}
}

func axisConstraints(a Axis, mainMin, mainMax, crossMin, crossMax int) Constraints {
	if a == Horizontal {
		return Constraints{Min: image.Pt(mainMin, crossMin), Max: image.Pt(mainMax, crossMax)}
	} else {
		return Constraints{Min: image.Pt(crossMin, mainMin), Max: image.Pt(crossMax, mainMax)}
	}
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
