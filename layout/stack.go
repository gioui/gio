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
}

// Stacked returns a Stack child that is laid out with no minimum
// constraints and the maximum constraints passed to Stack.Layout.
func Stacked(w Widget) StackChild {
	return StackChild{
		widget: w,
	}
}

// Expanded returns a Stack child with the minimum constraints set
// to the largest Stacked child. The maximum constraints are set to
// the same as passed to Stack.Layout.
func Expanded(w Widget) StackChild {
	return StackChild{
		expanded: true,
		widget:   w,
	}
}

// Layout a stack of children. The position of the children are
// determined by the specified order, but Stacked children are laid out
// before Expanded children.
func (s Stack) Layout(gtx Context, children ...StackChild) Dimensions {
	var maxSZ image.Point
	// First lay out Stacked children.
	cgtx := gtx
	cgtx.Constraints.Min = image.Point{}
	// Note: previously the scratch space was inside StackChild.
	// child.call.Add(gtx.Ops) confused the go escape analysis and caused the
	// entired children slice to be allocated on the heap, including all widgets
	// in it. This produced a lot of object allocations. Now the scratch space
	// is separate from children, and for cases len(children) <= 32, we will
	// allocate the scratch space on the stack. For cases len(children) > 32,
	// only the scratch space gets allocated from the heap, during append.
	type scratchSpace struct {
		call op.CallOp
		dims Dimensions
	}
	var scratchArray [32]scratchSpace
	scratch := scratchArray[:0]
	scratch = append(scratch, make([]scratchSpace, len(children))...)
	for i, w := range children {
		if w.expanded {
			continue
		}
		macro := op.Record(gtx.Ops)
		dims := w.widget(cgtx)
		call := macro.Stop()
		if w := dims.Size.X; w > maxSZ.X {
			maxSZ.X = w
		}
		if h := dims.Size.Y; h > maxSZ.Y {
			maxSZ.Y = h
		}
		scratch[i].call = call
		scratch[i].dims = dims
	}
	// Then lay out Expanded children.
	for i, w := range children {
		if !w.expanded {
			continue
		}
		macro := op.Record(gtx.Ops)
		cgtx.Constraints.Min = maxSZ
		dims := w.widget(cgtx)
		call := macro.Stop()
		if w := dims.Size.X; w > maxSZ.X {
			maxSZ.X = w
		}
		if h := dims.Size.Y; h > maxSZ.Y {
			maxSZ.Y = h
		}
		scratch[i].call = call
		scratch[i].dims = dims
	}

	maxSZ = gtx.Constraints.Constrain(maxSZ)
	var baseline int
	for _, scratchChild := range scratch {
		sz := scratchChild.dims.Size
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
		trans := op.Offset(p).Push(gtx.Ops)
		scratchChild.call.Add(gtx.Ops)
		trans.Pop()
		if baseline == 0 {
			if b := scratchChild.dims.Baseline; b != 0 {
				baseline = b + maxSZ.Y - sz.Y - p.Y
			}
		}
	}
	return Dimensions{
		Size:     maxSZ,
		Baseline: baseline,
	}
}

// Background lays out single child widget on top of a background,
// centering, if necessary.
type Background struct{}

// Layout a widget and then add a background to it.
func (Background) Layout(gtx Context, background, widget Widget) Dimensions {
	macro := op.Record(gtx.Ops)
	wdims := widget(gtx)
	baseline := wdims.Baseline
	call := macro.Stop()

	cgtx := gtx
	cgtx.Constraints.Min = gtx.Constraints.Constrain(wdims.Size)
	bdims := background(cgtx)

	if bdims.Size != wdims.Size {
		p := image.Point{
			X: (bdims.Size.X - wdims.Size.X) / 2,
			Y: (bdims.Size.Y - wdims.Size.Y) / 2,
		}
		baseline += (bdims.Size.Y - wdims.Size.Y) / 2
		trans := op.Offset(p).Push(gtx.Ops)
		defer trans.Pop()
	}

	call.Add(gtx.Ops)

	return Dimensions{
		Size:     bdims.Size,
		Baseline: baseline,
	}
}
