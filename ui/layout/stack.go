// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
)

type Stack struct {
	Alignment Direction

	constrained bool
	cs          Constraints
	begun       bool
	maxSZ       image.Point
	baseline    int
}

type StackChild struct {
	block ui.OpBlock
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

func (s *Stack) Init(cs Constraints) {
	s.cs = cs
	s.constrained = true
	s.maxSZ = image.Point{}
	s.baseline = 0
}

func (s *Stack) begin(ops *ui.Ops) {
	if !s.constrained {
		panic("must Constrain before adding a child")
	}
	if s.begun {
		panic("must End before adding a child")
	}
	s.begun = true
	ops.Begin()
	ui.OpLayer{}.Add(ops)
}

func (s *Stack) Rigid(ops *ui.Ops) Constraints {
	ops.Begin()
	ui.OpLayer{}.Add(ops)
	return s.cs
}

func (s *Stack) Expand(ops *ui.Ops) Constraints {
	cs := Constraints{
		Width:  Constraint{Min: s.maxSZ.X, Max: s.maxSZ.X},
		Height: Constraint{Min: s.maxSZ.Y, Max: s.maxSZ.Y},
	}
	ops.Begin()
	ui.OpLayer{}.Add(ops)
	return cs
}

func (s *Stack) End(ops *ui.Ops, dims Dimens) StackChild {
	b := ops.End()
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

func (s *Stack) Layout(ops *ui.Ops, children ...StackChild) Dimens {
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
		ops.Begin()
		ui.OpTransform{Transform: ui.Offset(toPointF(p))}.Add(ops)
		ch.block.Add(ops)
		ops.End().Add(ops)
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
