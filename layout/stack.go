// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/op"
)

// Stack lays out child elements on top of each other,
// according to an alignment direction.
type Stack struct {
	// Alignment is the direction to align children
	// smaller than the available space.
	Alignment Direction

	maxSZ    image.Point
	baseline int
}

// StackChild is the layout result of a call to End.
type StackChild struct {
	macro op.MacroOp
	dims  Dimensions
}

// Rigid lays out a widget with the same constraints that were
// passed to Init.
func (s *Stack) Rigid(gtx *Context, w Widget) StackChild {
	cs := gtx.Constraints
	cs.Width.Min = 0
	cs.Height.Min = 0
	var m op.MacroOp
	m.Record(gtx.Ops)
	dims := gtx.Layout(cs, w)
	m.Stop()
	s.expand(dims)
	return StackChild{m, dims}
}

// Expand lays out a widget.
func (s *Stack) Expand(gtx *Context, w Widget) StackChild {
	var m op.MacroOp
	m.Record(gtx.Ops)
	cs := Constraints{
		Width:  Constraint{Min: s.maxSZ.X, Max: s.maxSZ.X},
		Height: Constraint{Min: s.maxSZ.Y, Max: s.maxSZ.Y},
	}
	dims := gtx.Layout(cs, w)
	m.Stop()
	s.expand(dims)
	return StackChild{m, dims}
}

func (s *Stack) expand(dims Dimensions) {
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
}

// Layout a list of children. The order of the children determines their laid
// out order.
func (s *Stack) Layout(gtx *Context, children ...StackChild) {
	maxSZ := gtx.Constraints.Constrain(s.maxSZ)
	s.maxSZ = image.Point{}
	for _, ch := range children {
		sz := ch.dims.Size
		var p image.Point
		switch s.Alignment {
		case N, S, Center:
			p.X = (maxSZ.X - sz.X) / 2
		case NE, SE, E:
			p.X = maxSZ.X - sz.X
		}
		switch s.Alignment {
		case W, Center, E:
			p.Y = (maxSZ.Y - sz.Y) / 2
		case SW, S, SE:
			p.Y = maxSZ.Y - sz.Y
		}
		var stack op.StackOp
		stack.Push(gtx.Ops)
		op.TransformOp{}.Offset(toPointF(p)).Add(gtx.Ops)
		ch.macro.Add(gtx.Ops)
		stack.Pop()
	}
	b := s.baseline
	if b == 0 {
		b = maxSZ.Y
	}
	gtx.Dimensions = Dimensions{
		Size:     maxSZ,
		Baseline: b,
	}
	s.baseline = 0
}
