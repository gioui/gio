// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
)

type Stack struct {
	Alignment Direction

	ops         *ui.Ops
	constrained bool
	cs          Constraints
	begun       bool
	maxSZ       image.Point
	baseline    int
}

type StackChild struct {
	macro ui.MacroOp
	dims  Dimens
}

type Direction uint8

const (
	NW Direction = iota
	N
	NE
	E
	SE
	S
	SW
	W
)

func (s *Stack) Init(ops *ui.Ops, cs Constraints) *Stack {
	s.ops = ops
	s.cs = cs
	s.constrained = true
	s.maxSZ = image.Point{}
	s.baseline = 0
	return s
}

func (s *Stack) begin() {
	if !s.constrained {
		panic("must Init before adding a child")
	}
	if s.begun {
		panic("must End before adding a child")
	}
	s.begun = true
	s.ops.Record()
}

func (s *Stack) Rigid() Constraints {
	s.begin()
	return s.cs
}

func (s *Stack) Expand() Constraints {
	s.begin()
	return Constraints{
		Width:  Constraint{Min: s.maxSZ.X, Max: s.maxSZ.X},
		Height: Constraint{Min: s.maxSZ.Y, Max: s.maxSZ.Y},
	}
}

func (s *Stack) End(dims Dimens) StackChild {
	b := s.ops.Stop()
	s.begun = false
	if w := dims.Size.X; w > s.maxSZ.X {
		s.maxSZ.X = w
	}
	if h := dims.Size.Y; h > s.maxSZ.Y {
		s.maxSZ.Y = h
	}
	if s.baseline == 0 {
		if b := dims.Baseline; b != dims.Size.Y {
			s.baseline = b
		}
	}
	return StackChild{b, dims}
}

func (s *Stack) Layout(children ...StackChild) Dimens {
	for _, ch := range children {
		sz := ch.dims.Size
		var p image.Point
		switch s.Alignment {
		case N, S, Center:
			p.X = (s.maxSZ.X - sz.X) / 2
		case NE, SE, E:
			p.X = s.maxSZ.X - sz.X
		}
		switch s.Alignment {
		case W, Center, E:
			p.Y = (s.maxSZ.Y - sz.Y) / 2
		case SW, S, SE:
			p.Y = s.maxSZ.Y - sz.Y
		}
		ui.PushOp{}.Add(s.ops)
		ui.TransformOp{Transform: ui.Offset(toPointF(p))}.Add(s.ops)
		ch.macro.Add(s.ops)
		ui.PopOp{}.Add(s.ops)
	}
	b := s.baseline
	if b == 0 {
		b = s.maxSZ.Y
	}
	return Dimens{
		Size:     s.maxSZ,
		Baseline: b,
	}
}
