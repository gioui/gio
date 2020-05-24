// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

// Clickable represents a clickable area.
type Clickable struct {
	click gesture.Click
	// clicks tracks the number of unreported clicks.
	clicks  int
	history []Press
}

// Press represents a past pointer press.
type Press struct {
	Position f32.Point
	Time     time.Time
}

// Clicked and reports whether the button was clicked since the last
// call to Clicked. Clicked returns true once per click.
func (b *Clickable) Clicked() bool {
	if b.clicks > 0 {
		b.clicks--
		return true
	}
	return false
}

// History is the past pointer presses useful for drawing markers.
// History is retained for a short duration (about a second).
func (b *Clickable) History() []Press {
	return b.history
}

func (b *Clickable) Layout(gtx layout.Context) layout.Dimensions {
	b.update(gtx)
	var st op.StackOp
	st.Push(gtx.Ops)
	pointer.Rect(image.Rectangle{Max: gtx.Constraints.Min}).Add(gtx.Ops)
	b.click.Add(gtx.Ops)
	st.Pop()
	for len(b.history) > 0 {
		c := b.history[0]
		if gtx.Now().Sub(c.Time) < 1*time.Second {
			break
		}
		n := copy(b.history, b.history[1:])
		b.history = b.history[:n]
	}
	return layout.Dimensions{Size: gtx.Constraints.Min}
}

// update the button state by processing events.
func (b *Clickable) update(gtx layout.Context) {
	for _, e := range b.click.Events(gtx) {
		switch e.Type {
		case gesture.TypeClick:
			b.clicks++
		case gesture.TypePress:
			b.history = append(b.history, Press{
				Position: e.Position,
				Time:     gtx.Now(),
			})
		}
	}
}
