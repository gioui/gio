// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
)

type Stack struct {
	Alignment   Direction
	Constraints Constraints

	maxSZ    image.Point
	baseline int
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

func (s *Stack) Rigid(ops *ui.Ops, w Widget) StackChild {
	ops.Begin()
	ui.OpLayer{}.Add(ops)
	dims := w(ops, s.Constraints)
	b := ops.End()
	if w := dims.Size.X; w > s.maxSZ.X {
		s.maxSZ.X = w
	}
	if h := dims.Size.Y; h > s.maxSZ.Y {
		s.maxSZ.Y = h
	}
	s.addjustBaseline(dims)
	return StackChild{b, dims}
}

func (s *Stack) Expand(ops *ui.Ops, w Widget) StackChild {
	cs := Constraints{
		Width:  Constraint{Min: s.maxSZ.X, Max: s.maxSZ.X},
		Height: Constraint{Min: s.maxSZ.Y, Max: s.maxSZ.Y},
	}
	ops.Begin()
	ui.OpLayer{}.Add(ops)
	dims := w(ops, cs)
	b := ops.End()
	s.addjustBaseline(dims)
	return StackChild{b, dims}
}

func (s *Stack) addjustBaseline(dims Dimens) {
	if s.baseline == 0 {
		if b := dims.Baseline; b != dims.Size.Y {
			s.baseline = b
		}
	}
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
