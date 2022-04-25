package widget

import (
	"fmt"
	"image"
	"math/bits"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// Decorations handles the states of window decorations.
type Decorations struct {
	move   gesture.Drag
	clicks []Clickable
	resize [8]struct {
		gesture.Hover
		gesture.Drag
	}
	actions   system.Action
	maximized bool
}

// LayoutMove lays out the widget that makes a window movable.
func (d *Decorations) LayoutMove(gtx layout.Context, w layout.Widget) layout.Dimensions {
	dims := w(gtx)
	d.move.Events(gtx.Metric, gtx, gesture.Both)
	st := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
	d.move.Add(gtx.Ops)
	if d.move.Pressed() {
		d.actions |= system.ActionMove
	}
	st.Pop()
	return dims
}

// Clickable returns the clickable for the given single action.
func (d *Decorations) Clickable(action system.Action) *Clickable {
	if bits.OnesCount(uint(action)) != 1 {
		panic(fmt.Errorf("not a single action"))
	}
	idx := bits.TrailingZeros(uint(action))
	if n := idx - len(d.clicks); n >= 0 {
		d.clicks = append(d.clicks, make([]Clickable, n+1)...)
	}
	click := &d.clicks[idx]
	if click.Clicked() {
		if action == system.ActionMaximize {
			if d.maximized {
				d.maximized = false
				d.actions |= system.ActionUnmaximize
			} else {
				d.maximized = true
				d.actions |= system.ActionMaximize
			}
		} else {
			d.actions |= action
		}
	}
	return click
}

// LayoutResize lays out the resize actions.
func (d *Decorations) LayoutResize(gtx layout.Context, actions system.Action) {
	cs := gtx.Constraints.Max
	wh := gtx.Dp(10)
	s := []struct {
		system.Action
		image.Rectangle
	}{
		{system.ActionResizeNorth, image.Rect(0, 0, cs.X, wh)},
		{system.ActionResizeSouth, image.Rect(0, cs.Y-wh, cs.X, cs.Y)},
		{system.ActionResizeWest, image.Rect(cs.X-wh, 0, cs.X, cs.Y)},
		{system.ActionResizeEast, image.Rect(0, 0, wh, cs.Y)},
		{system.ActionResizeNorthWest, image.Rect(0, 0, wh, wh)},
		{system.ActionResizeNorthEast, image.Rect(cs.X-wh, 0, cs.X, wh)},
		{system.ActionResizeSouthWest, image.Rect(0, cs.Y-wh, wh, cs.Y)},
		{system.ActionResizeSouthEast, image.Rect(cs.X-wh, cs.Y-wh, cs.X, cs.Y)},
	}
	for i, data := range s {
		action := data.Action
		if actions&action == 0 {
			continue
		}
		rsz := &d.resize[i]
		rsz.Events(gtx.Metric, gtx, gesture.Both)
		if rsz.Drag.Dragging() {
			d.actions |= action
		}
		st := clip.Rect(data.Rectangle).Push(gtx.Ops)
		if rsz.Hover.Hovered(gtx) {
			action.Cursor().Add(gtx.Ops)
		}
		rsz.Drag.Add(gtx.Ops)
		pass := pointer.PassOp{}.Push(gtx.Ops)
		rsz.Hover.Add(gtx.Ops)
		pass.Pop()
		st.Pop()
	}
}

// Perform updates the decorations as if the specified actions were
// performed by the user.
func (d *Decorations) Perform(actions system.Action) {
	if actions&system.ActionMaximize != 0 {
		d.maximized = true
	}
	if actions&(system.ActionUnmaximize|system.ActionMinimize|system.ActionFullscreen) != 0 {
		d.maximized = false
	}
}

// Actions returns the set of actions activated by the user.
func (d *Decorations) Actions() system.Action {
	a := d.actions
	d.actions = 0
	return a
}

// Maximized returns whether the window is maximized.
func (d *Decorations) Maximized() bool {
	return d.maximized
}
