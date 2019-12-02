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

	maxSZ image.Point
	// Use an empty StackOp for tracking whether Rigid, Flex
	// is called in the same layout scope as Layout.
	begun bool
	stack op.StackOp
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
	dims := ctxLayout(gtx, cs, w)
	m.Stop()
	s.expand(gtx.Ops, dims)
	return StackChild{m, dims}
}

// Expand lays out a widget.
func (s *Stack) Expand(gtx *Context, w Widget) StackChild {
	var m op.MacroOp
	m.Record(gtx.Ops)
	cs := Constraints{
		Width:  Constraint{Min: s.maxSZ.X, Max: gtx.Constraints.Width.Max},
		Height: Constraint{Min: s.maxSZ.Y, Max: gtx.Constraints.Height.Max},
	}
	dims := ctxLayout(gtx, cs, w)
	m.Stop()
	s.expand(gtx.Ops, dims)
	return StackChild{m, dims}
}

func (s *Stack) expand(ops *op.Ops, dims Dimensions) {
	if !s.begun {
		s.stack.Push(ops)
		s.begun = true
	}
	if w := dims.Size.X; w > s.maxSZ.X {
		s.maxSZ.X = w
	}
	if h := dims.Size.Y; h > s.maxSZ.Y {
		s.maxSZ.Y = h
	}
}

// Layout a list of children. The order of the children determines their laid
// out order.
func (s *Stack) Layout(gtx *Context, children ...StackChild) {
	if len(children) > 0 {
		s.stack.Pop()
	}
	maxSZ := gtx.Constraints.Constrain(s.maxSZ)
	s.maxSZ = image.Point{}
	s.begun = false
	var baseline int
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
		if baseline == 0 {
			if b := ch.dims.Baseline; b != 0 {
				baseline = b + maxSZ.Y - sz.Y - p.Y
			}
		}
	}
	gtx.Dimensions = Dimensions{
		Size:     maxSZ,
		Baseline: baseline,
	}
}
