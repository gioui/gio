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
	history []Click
}

// Click represents a past click.
type Click struct {
	Position f32.Point
	Time     time.Time
}

// Clicked calls Update and reports whether the button was
// clicked since the last call. Multiple clicks result in Clicked
// returning true once per click.
func (b *Clickable) Clicked(gtx layout.Context) bool {
	b.Update(gtx)
	if b.clicks > 0 {
		b.clicks--
		if b.clicks > 0 {
			// Ensure timely delivery of remaining clicks.
			op.InvalidateOp{}.Add(gtx.Ops)
		}
		return true
	}
	return false
}

// History is the past clicks useful for drawing click markers.
// Clicks are retained for a short duration (about a second).
func (b *Clickable) History() []Click {
	return b.history
}

func (b *Clickable) Layout(gtx layout.Context) layout.Dimensions {
	// Flush clicks from before the previous frame.
	b.Update(gtx)
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

// Update the button state by processing events. The underlying
// gesture events are returned for use beyond what Clicked offers.
func (b *Clickable) Update(gtx layout.Context) []gesture.ClickEvent {
	evts := b.click.Events(gtx)
	for _, e := range evts {
		switch e.Type {
		case gesture.TypeClick:
			b.clicks++
		case gesture.TypePress:
			b.history = append(b.history, Click{
				Position: e.Position,
				Time:     gtx.Now(),
			})
		}
	}
	return evts
}
