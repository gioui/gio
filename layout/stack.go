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
	constrained bool
	ctx         *Context
	maxSZ       image.Point
	baseline    int
}

// StackChild is the layout result of a call to End.
type StackChild struct {
	macro ui.MacroOp
	dims  Dimensions
}

// Init a stack before calling Rigid or Expand.
func (s *Stack) Init(gtx *Context) *Stack {
	s.ctx = gtx
	s.constrained = true
	s.maxSZ = image.Point{}
	s.baseline = 0
	return s
}

func (s *Stack) begin() {
	if !s.constrained {
		panic("must Init before adding a child")
	}
	s.macro.Record(s.ctx.Ops)
}

// Rigid lays out a widget with the same constraints that were
// passed to Init.
func (s *Stack) Rigid(w Widget) StackChild {
	s.begin()
	dims := s.ctx.Layout(s.ctx.Constraints, w)
	return s.end(dims)
}

// Expand lays out a widget with constraints that exactly match
// the biggest child previously added.
func (s *Stack) Expand(w Widget) StackChild {
	s.begin()
	cs := Constraints{
		Width:  Constraint{Min: s.maxSZ.X, Max: s.maxSZ.X},
		Height: Constraint{Min: s.maxSZ.Y, Max: s.maxSZ.Y},
	}
	dims := s.ctx.Layout(cs, w)
	return s.end(dims)
}

// End a child by specifying its dimensions.
func (s *Stack) end(dims Dimensions) StackChild {
	s.macro.Stop()
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
func (s *Stack) Layout(children ...StackChild) {
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
		stack.Push(s.ctx.Ops)
		ui.TransformOp{}.Offset(toPointF(p)).Add(s.ctx.Ops)
		ch.macro.Add(s.ctx.Ops)
		stack.Pop()
	}
	b := s.baseline
	if b == 0 {
		b = s.maxSZ.Y
	}
	s.ctx.Dimensions = Dimensions{
		Size:     s.maxSZ,
		Baseline: b,
	}
}
