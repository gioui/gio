// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
)

type Stack struct {
	Alignment Direction

	cs       Constraints
	children []stackChild
	maxSZ    image.Point
	baseline int

	ccache  [10]stackChild
	opCache [10]ui.Op
}

type stackChild struct {
	op   ui.Op
	dims Dimens
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

func (s *Stack) Init(cs Constraints) *Stack {
	if s.children == nil {
		s.children = s.ccache[:0]
	}
	s.children = s.children[:0]
	s.maxSZ = image.Point{}
	s.baseline = 0
	s.cs = cs
	return s
}

func (s *Stack) Rigid(w Widget) *Stack {
	op, dims := w.Layout(s.cs)
	if w := dims.Size.X; w > s.maxSZ.X {
		s.maxSZ.X = w
	}
	if h := dims.Size.Y; h > s.maxSZ.Y {
		s.maxSZ.Y = h
	}
	s.add(op, dims)
	return s
}

func (s *Stack) Expand(idx int, w Widget) *Stack {
	cs := Constraints{
		Width:  Constraint{Min: s.maxSZ.X, Max: s.maxSZ.X},
		Height: Constraint{Min: s.maxSZ.Y, Max: s.maxSZ.Y},
	}
	s.add(w.Layout(cs))
	if idx < 0 {
		idx += len(s.children)
	}
	s.children[idx], s.children[len(s.children)-1] = s.children[len(s.children)-1], s.children[idx]
	return s
}

func (s *Stack) add(op ui.Op, dims Dimens) {
	s.children = append(s.children, stackChild{op, dims})
	if s.baseline == 0 {
		if b := dims.Baseline; b != dims.Size.Y {
			s.baseline = b
		}
	}
}

func (s *Stack) Layout() (ui.Op, Dimens) {
	var ops ui.Ops
	if len(s.children) > len(s.opCache) {
		ops = make([]ui.Op, len(s.children))
	} else {
		ops = s.opCache[:len(s.children)]
	}
	for i, ch := range s.children {
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
		ops[i] = ui.OpLayer{Op: ui.OpTransform{Transform: ui.Offset(toPointF(p)), Op: ch.op}}
	}
	b := s.baseline
	if b == 0 {
		b = s.maxSZ.Y
	}
	return ops, Dimens{
		Size:     s.maxSZ,
		Baseline: b,
	}
}
