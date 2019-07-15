// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
	"gioui.org/ui/f32"
)

type Flex struct {
	Axis               Axis
	MainAxisAlignment  MainAxisAlignment
	CrossAxisAlignment CrossAxisAlignment

	ops         *ui.Ops
	cs          Constraints
	mode        flexMode
	size        int
	rigidSize   int
	maxCross    int
	maxBaseline int
}

type FlexChild struct {
	macro ui.MacroOp
	dims  Dimens
}

type MainAxisAlignment uint8
type CrossAxisAlignment uint8

type flexMode uint8

const (
	Start = 100 + iota
	End
	Center

	SpaceAround MainAxisAlignment = iota
	SpaceBetween
	SpaceEvenly

	Baseline CrossAxisAlignment = iota
)

const (
	modeNone flexMode = iota
	modeBegun
	modeRigid
	modeFlex
)

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
	f.ops.Record()
}

func (f *Flex) Rigid() Constraints {
	f.begin(modeRigid)
	mainc := axisMainConstraint(f.Axis, f.cs)
	mainMax := mainc.Max
	if mainc.Max != ui.Inf {
		mainMax -= f.size
	}
	return axisConstraints(f.Axis, Constraint{Max: mainMax}, axisCrossConstraint(f.Axis, f.cs))
}

func (f *Flex) Flexible(weight float32) Constraints {
	f.begin(modeFlex)
	mainc := axisMainConstraint(f.Axis, f.cs)
	var flexSize int
	if mainc.Max != ui.Inf && mainc.Max > f.size {
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

func (f *Flex) End(dims Dimens) FlexChild {
	if f.mode <= modeBegun {
		panic("End called without an active child")
	}
	macro := f.ops.Stop()
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
	return FlexChild{macro, dims}
}

func (f *Flex) Layout(children ...FlexChild) Dimens {
	mainc := axisMainConstraint(f.Axis, f.cs)
	crossSize := axisCrossConstraint(f.Axis, f.cs).Constrain(f.maxCross)
	var space int
	if mainc.Min > f.size {
		space = mainc.Min - f.size
	}
	var mainSize int
	var baseline int
	switch f.MainAxisAlignment {
	case Center:
		mainSize += space / 2
	case End:
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
		switch f.CrossAxisAlignment {
		case End:
			cross = crossSize - axisCross(f.Axis, dims.Size)
		case Center:
			cross = (crossSize - axisCross(f.Axis, dims.Size)) / 2
		case Baseline:
			if f.Axis == Horizontal {
				cross = f.maxBaseline - b
			}
		}
		ui.PushOp{}.Add(f.ops)
		ui.TransformOp{
			Transform: ui.Offset(toPointF(axisPoint(f.Axis, mainSize, cross))),
		}.Add(f.ops)
		child.macro.Add(f.ops)
		ui.PopOp{}.Add(f.ops)
		mainSize += axisMain(f.Axis, dims.Size)
		if i < len(children)-1 {
			switch f.MainAxisAlignment {
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
	switch f.MainAxisAlignment {
	case Center:
		mainSize += space / 2
	case Start:
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
