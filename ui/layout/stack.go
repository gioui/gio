// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
)

// Stack lays out child elements on top of each other,
// according to an alignment direction.
type Stack struct {
	// Alignment is the direction to align children
	// smaller than the available space.
	Alignment Direction

	macro       ui.MacroOp
	ops         *ui.Ops
	constrained bool
	cs          Constraints
	begun       bool
	maxSZ       image.Point
	baseline    int
}

// StackChild is the layout result of a call to End.
type StackChild struct {
	macro ui.MacroOp
	dims  Dimensions
}

// Init a stack before calling Rigid or Expand.
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
	s.macro.Record(s.ops)
}

// Rigid lays out a widget with the same constraints that were
// passed to Init.
func (s *Stack) Rigid(w Widget) StackChild {
	s.begin()
	return s.end(w(s.cs))
}

// Expand lays out a widget with constraints that exactly match
// the biggest child previously added.
func (s *Stack) Expand(w Widget) StackChild {
	s.begin()
	cs := Constraints{
		Width:  Constraint{Min: s.maxSZ.X, Max: s.maxSZ.X},
		Height: Constraint{Min: s.maxSZ.Y, Max: s.maxSZ.Y},
	}
	return s.end(w(cs))
}

// End a child by specifying its dimensions.
func (s *Stack) end(dims Dimensions) StackChild {
	s.macro.Stop()
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
	return StackChild{s.macro, dims}
}

// Layout a list of children. The order of the children determines their laid
// out order.
func (s *Stack) Layout(children ...StackChild) Dimensions {
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
		var stack ui.StackOp
		stack.Push(s.ops)
		ui.TransformOp{}.Offset(toPointF(p)).Add(s.ops)
		ch.macro.Add(s.ops)
		stack.Pop()
	}
	b := s.baseline
	if b == 0 {
		b = s.maxSZ.Y
	}
	return Dimensions{
		Size:     s.maxSZ,
		Baseline: b,
	}
}
