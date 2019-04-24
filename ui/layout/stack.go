// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
)

type Stack struct {
	Alignment Direction

	ops      *ui.Ops
	cs       Constraints
	children []stackChild
	maxSZ    image.Point
	baseline int

	ccache [10]stackChild
}

type stackChild struct {
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

func (s *Stack) Init(ops *ui.Ops, cs Constraints) *Stack {
	if s.children == nil {
		s.children = s.ccache[:0]
	}
	s.children = s.children[:0]
	s.maxSZ = image.Point{}
	s.baseline = 0
	s.ops = ops
	s.cs = cs
	return s
}

func (s *Stack) Rigid(w Widget) *Stack {
	s.ops.Begin()
	ui.OpLayer{}.Add(s.ops)
	dims := w.Layout(s.ops, s.cs)
	b := s.ops.End()
	if w := dims.Size.X; w > s.maxSZ.X {
		s.maxSZ.X = w
	}
	if h := dims.Size.Y; h > s.maxSZ.Y {
		s.maxSZ.Y = h
	}
	s.addjustBaseline(dims)
	s.children = append(s.children, stackChild{b, dims})
	return s
}

func (s *Stack) Expand(idx int, w Widget) *Stack {
	cs := Constraints{
		Width:  Constraint{Min: s.maxSZ.X, Max: s.maxSZ.X},
		Height: Constraint{Min: s.maxSZ.Y, Max: s.maxSZ.Y},
	}
	s.ops.Begin()
	ui.OpLayer{}.Add(s.ops)
	dims := w.Layout(s.ops, cs)
	b := s.ops.End()
	s.addjustBaseline(dims)
	if idx < 0 {
		idx += len(s.children) + 1
	}
	s.children = append(s.children, stackChild{})
	copy(s.children[idx+1:], s.children[idx:])
	s.children[idx] = stackChild{b, dims}
	return s
}

func (s *Stack) addjustBaseline(dims Dimens) {
	if s.baseline == 0 {
		if b := dims.Baseline; b != dims.Size.Y {
			s.baseline = b
		}
	}
}

func (s *Stack) Layout() Dimens {
	for _, ch := range s.children {
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
		s.ops.Begin()
		ui.OpTransform{Transform: ui.Offset(toPointF(p))}.Add(s.ops)
		ch.block.Add(s.ops)
		s.ops.End().Add(s.ops)
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
