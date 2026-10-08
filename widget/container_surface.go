// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"sort"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

// ContainerSurface lays out child widgets at absolute positions with z-depth layering.
// Children are rendered in order of ascending Depth (lower Depth = behind higher Depth),
// similar to CSS position:absolute + z-index.
//
// Usage:
//
//	cs := widget.ContainerSurface{Width: 400, Height: 300}
//	cs.Children = []widget.SurfaceChild{
//	    {Widget: backgroundWidget, Pos: image.Pt(0, 0), Depth: 0},
//	    {Widget: buttonWidget,    Pos: image.Pt(50, 100), Depth: 1},
//	    {Widget: labelWidget,     Pos: image.Pt(20, 20), Depth: 2},
//	}
//	cs.Layout(gtx)
type ContainerSurface struct {
	// Width and Height in dp define the virtual size of the container.
	// If zero, the container fills the available constraints.
	Width, Height unit.Dp

	// Children holds the items to render.
	// Layout sorts them by Depth (ascending) before rendering.
	// A child with Depth 0 renders first (bottom), higher values render on top.
	Children []SurfaceChild
}

// SurfaceChild is a positioned child widget within a ContainerSurface.
type SurfaceChild struct {
	// Pos is the position (in dp) of the child's top-left corner
	// relative to the container's origin.
	Pos image.Point

	// Depth controls the rendering order (z-index).
	// Lower values are rendered first (behind), higher values on top.
	// Equal depths maintain insertion order.
	Depth int

	// Widget is the child element to render.
	Widget layout.Widget
}

// sortedChildren returns a copy of Children sorted by ascending Depth,
// preserving original order for equal depths (stable sort).
func (cs *ContainerSurface) sortedChildren() []SurfaceChild {
	sorted := make([]SurfaceChild, len(cs.Children))
	copy(sorted, cs.Children)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Depth < sorted[j].Depth
	})
	return sorted
}

// Layout renders all children at their specified positions, layered by Depth.
// The container's total size is the maximum of (its declared Width/Height)
// and the bounding box of all children.
func (cs *ContainerSurface) Layout(gtx layout.Context) layout.Dimensions {
	// Determine container size.
	var containerW, containerH int
	if cs.Width > 0 {
		containerW = gtx.Dp(cs.Width)
	}
	if cs.Height > 0 {
		containerH = gtx.Dp(cs.Height)
	}

	children := cs.sortedChildren()

	// If no explicit size, compute from children.
	if containerW == 0 || containerH == 0 {
		for _, ch := range children {
			posX := gtx.Dp(unit.Dp(ch.Pos.X))
			posY := gtx.Dp(unit.Dp(ch.Pos.Y))
			// We don't know child sizes here without laying them out,
			// so just use position as a minimum.
			if posX+gtx.Constraints.Min.X > containerW {
				containerW = posX + gtx.Constraints.Min.X
			}
			if posY+gtx.Constraints.Min.Y > containerH {
				containerH = posY + gtx.Constraints.Min.Y
			}
		}
		// Fall back to available space if still zero.
		if containerW == 0 {
			containerW = gtx.Constraints.Max.X
		}
		if containerH == 0 {
			containerH = gtx.Constraints.Max.Y
		}
	}

	containerSize := image.Pt(containerW, containerH)
	containerSize = gtx.Constraints.Constrain(containerSize)

	// Clip to the container bounds.
	defer clip.Rect(image.Rectangle{Max: containerSize}).Push(gtx.Ops).Pop()

	// Render each child at its position, lowest Depth first.
	for _, ch := range children {
		px := gtx.Dp(unit.Dp(ch.Pos.X))
		py := gtx.Dp(unit.Dp(ch.Pos.Y))

		// Create a child context with constraints unbounded in both directions
		// so children can render at their natural size.
		childGtx := gtx
		childGtx.Constraints = layout.Constraints{
			Min: image.Point{},
			Max: image.Point{
				X: containerSize.X - px,
				Y: containerSize.Y - py,
			},
		}
		// Ensure max doesn't go negative.
		if childGtx.Constraints.Max.X < 0 {
			childGtx.Constraints.Max.X = 0
		}
		if childGtx.Constraints.Max.Y < 0 {
			childGtx.Constraints.Max.Y = 0
		}

		// Record child ops, then replay at the offset position.
		macro := op.Record(childGtx.Ops)
		ch.Widget(childGtx)
		call := macro.Stop()

		trans := op.Offset(image.Pt(px, py)).Push(childGtx.Ops)
		call.Add(childGtx.Ops)
		trans.Pop()
	}

	return layout.Dimensions{Size: containerSize}
}

// Add appends a child at the given position and depth.
func (cs *ContainerSurface) Add(pos image.Point, depth int, w layout.Widget) {
	cs.Children = append(cs.Children, SurfaceChild{
		Pos:    pos,
		Depth:  depth,
		Widget: w,
	})
}

// RemoveAt removes the child at the given index.
// It does nothing if index is out of range.
func (cs *ContainerSurface) RemoveAt(index int) {
	if index < 0 || index >= len(cs.Children) {
		return
	}
	cs.Children = append(cs.Children[:index], cs.Children[index+1:]...)
}

// Clear removes all children.
func (cs *ContainerSurface) Clear() {
	cs.Children = cs.Children[:0]
}

// Len returns the number of children.
func (cs *ContainerSurface) Len() int {
	return len(cs.Children)
}
