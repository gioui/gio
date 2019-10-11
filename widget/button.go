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
	click   gesture.Click
	clicks  int
	history []Click
}

// Click represents a historic click.
type Click struct {
	Position f32.Point
	Time     time.Time
}

func (b *Button) Clicked(gtx *layout.Context) bool {
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

func (b *Button) Active() bool {
	return b.click.Active()
}

func (b *Button) History() []Click {
	return b.history
}

func (b *Button) Layout(gtx *layout.Context) {
	b.click.Add(gtx.Ops)
	if !b.Active() {
		b.clicks = 0
	}
	for len(b.history) > 0 {
		c := b.history[0]
		if gtx.Now().Sub(c.Time) < 1*time.Second {
			break
		}
		copy(b.history, b.history[1:])
		b.history = b.history[:len(b.history)-1]
	}
}
