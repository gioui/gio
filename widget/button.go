// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

// Clickable represents a clickable area.
type Clickable struct {
	click  gesture.Click
	clicks []Click
	// prevClicks is the index into clicks that marks the clicks
	// from the most recent Layout call. prevClicks is used to keep
	// clicks bounded.
	prevClicks int
	history    []Press
}

// Click represents a click.
type Click struct {
	Modifiers key.Modifiers
	NumClicks int
}

// Press represents a past pointer press.
type Press struct {
	Position f32.Point
	Time     time.Time
}

// Clicked reports whether there are pending clicks as would be
// reported by Clicks. If so, Clicked removes the earliest click.
func (b *Clickable) Clicked() bool {
	if len(b.clicks) == 0 {
		return false
	}
	n := copy(b.clicks, b.clicks[1:])
	b.clicks = b.clicks[:n]
	if b.prevClicks > 0 {
		b.prevClicks--
	}
	return true
}

// Clicks returns and clear the clicks since the last call to Clicks.
func (b *Clickable) Clicks() []Click {
	clicks := b.clicks
	b.clicks = nil
	b.prevClicks = 0
	return clicks
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
	// Flush clicks from before the last update.
	n := copy(b.clicks, b.clicks[b.prevClicks:])
	b.clicks = b.clicks[:n]
	b.prevClicks = n

	for _, e := range b.click.Events(gtx) {
		switch e.Type {
		case gesture.TypeClick:
			b.clicks = append(b.clicks, Click{
				Modifiers: e.Modifiers,
				NumClicks: e.NumClicks,
			})
		case gesture.TypePress:
			b.history = append(b.history, Press{
				Position: e.Position,
				Time:     gtx.Now(),
			})
		}
	}
}
