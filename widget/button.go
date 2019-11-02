// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"time"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
)

type Button struct {
	click gesture.Click
	// clicks tracks the number of unreported clicks.
	clicks int
	// prevClicks tracks the number of unreported clicks
	// that belong to the previous frame.
	prevClicks int
	history    []Click
}

// Click represents a historic click.
type Click struct {
	Position f32.Point
	Time     time.Time
}

func (b *Button) Clicked(gtx *layout.Context) bool {
	b.processEvents(gtx)
	if b.clicks > 0 {
		b.clicks--
		if b.prevClicks > 0 {
			b.prevClicks--
		}
		if b.clicks > 0 {
			// Ensure timely delivery of remaining clicks.
			op.InvalidateOp{}.Add(gtx.Ops)
		}
		return true
	}
	return false
}

func (b *Button) History() []Click {
	return b.history
}

func (b *Button) Layout(gtx *layout.Context) {
	// Flush clicks from before the previous frame.
	b.clicks -= b.prevClicks
	b.prevClicks = 0
	b.processEvents(gtx)
	b.click.Add(gtx.Ops)
	for len(b.history) > 0 {
		c := b.history[0]
		if gtx.Now().Sub(c.Time) < 1*time.Second {
			break
		}
		copy(b.history, b.history[1:])
		b.history = b.history[:len(b.history)-1]
	}
}

func (b *Button) processEvents(gtx *layout.Context) {
	for _, e := range b.click.Events(gtx) {
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
}
