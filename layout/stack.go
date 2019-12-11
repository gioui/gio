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
}

// StackChild represents a child for a Stack layout.
type StackChild struct {
	expanded bool
	widget   Widget

	// Scratch space.
	macro op.MacroOp
	dims  Dimensions
}

// Stacked returns a Stack child that laid out with the same maximum
// constraints as the Stack.
func Stacked(w Widget) StackChild {
	return StackChild{
		widget: w,
	}
}

// Expanded returns a Stack child that is forced to take up at least
// the the space as the largest Stacked.
func Expanded(w Widget) StackChild {
	return StackChild{
		expanded: true,
		widget:   w,
	}
}

// Layout a stack of children. The position of the children are
// determined by the specified order, but Stacked children are laid out
// before Expanded children.
func (s Stack) Layout(gtx *Context, children ...StackChild) {
	var maxSZ image.Point
	// First lay out Stacked children.
	for i, w := range children {
		if w.expanded {
			continue
		}
		cs := gtx.Constraints
		cs.Width.Min = 0
		cs.Height.Min = 0
		var m op.MacroOp
		m.Record(gtx.Ops)
		dims := ctxLayout(gtx, cs, w.widget)
		m.Stop()
		if w := dims.Size.X; w > maxSZ.X {
			maxSZ.X = w
		}
		if h := dims.Size.Y; h > maxSZ.Y {
			maxSZ.Y = h
		}
		children[i].macro = m
		children[i].dims = dims
	}
	maxSZ = gtx.Constraints.Constrain(maxSZ)
	// Then lay out Expanded children.
	for i, w := range children {
		if !w.expanded {
			continue
		}
		var m op.MacroOp
		m.Record(gtx.Ops)
		cs := Constraints{
			Width:  Constraint{Min: maxSZ.X, Max: gtx.Constraints.Width.Max},
			Height: Constraint{Min: maxSZ.Y, Max: gtx.Constraints.Height.Max},
		}
		dims := ctxLayout(gtx, cs, w.widget)
		m.Stop()
		children[i].macro = m
		children[i].dims = dims
	}

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
		ch.macro.Add()
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
