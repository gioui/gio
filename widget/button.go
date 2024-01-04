// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// Clickable represents a clickable area.
type Clickable struct {
	click gesture.Click
	// clicks is for saved clicks to support Clicked.
	clicks  []Click
	history []Press

	keyTag        struct{}
	requestFocus  bool
	requestClicks int
	focused       bool
	pressedKey    string
}

// Click represents a click.
type Click struct {
	Modifiers key.Modifiers
	NumClicks int
}

// Press represents a past pointer press.
type Press struct {
	// Position of the press.
	Position image.Point
	// Start is when the press began.
	Start time.Time
	// End is when the press was ended by a release or cancel.
	// A zero End means it hasn't ended yet.
	End time.Time
	// Cancelled is true for cancelled presses.
	Cancelled bool
}

// Click executes a simple programmatic click.
func (b *Clickable) Click() {
	b.requestClicks++
}

// Clicked reports whether there are pending clicks. If so, Clicked
// removes the earliest click.
func (b *Clickable) Clicked(gtx layout.Context) bool {
	if len(b.clicks) > 0 {
		b.clicks = b.clicks[1:]
		return true
	}
	b.clicks = b.Update(gtx)
	if len(b.clicks) > 0 {
		b.clicks = b.clicks[1:]
		return true
	}
	return false
}

// Hovered reports whether a pointer is over the element.
func (b *Clickable) Hovered() bool {
	return b.click.Hovered()
}

// Pressed reports whether a pointer is pressing the element.
func (b *Clickable) Pressed() bool {
	return b.click.Pressed()
}

// Focus requests the input focus for the element.
func (b *Clickable) Focus() {
	b.requestFocus = true
}

// Focused reports whether b has focus.
func (b *Clickable) Focused() bool {
	return b.focused
}

// History is the past pointer presses useful for drawing markers.
// History is retained for a short duration (about a second).
func (b *Clickable) History() []Press {
	return b.history
}

// Layout and update the button state.
func (b *Clickable) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	b.Update(gtx)
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	c := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	enabled := gtx.Queue != nil
	semantic.EnabledOp(enabled).Add(gtx.Ops)
	b.click.Add(gtx.Ops)
	if enabled {
		keys := key.Set("⏎|Space")
		if !b.focused {
			keys = ""
		}
		key.InputOp{Tag: &b.keyTag, Keys: keys}.Add(gtx.Ops)
	}
	c.Add(gtx.Ops)
	return dims
}

// Update the button state by processing events, and return the resulting
// clicks, if any.
func (b *Clickable) Update(gtx layout.Context) []Click {
	b.clicks = nil
	if gtx.Queue == nil {
		b.focused = false
	}
	if b.requestFocus {
		key.FocusOp{Tag: &b.keyTag}.Add(gtx.Ops)
		b.requestFocus = false
	}
	for len(b.history) > 0 {
		c := b.history[0]
		if c.End.IsZero() || gtx.Now.Sub(c.End) < 1*time.Second {
			break
		}
		n := copy(b.history, b.history[1:])
		b.history = b.history[:n]
	}
	var clicks []Click
	if c := b.requestClicks; c > 0 {
		b.requestClicks = 0
		clicks = append(clicks, Click{
			NumClicks: c,
		})
	}
	for _, e := range b.click.Update(gtx) {
		switch e.Kind {
		case gesture.KindClick:
			if l := len(b.history); l > 0 {
				b.history[l-1].End = gtx.Now
			}
			clicks = append(clicks, Click{
				Modifiers: e.Modifiers,
				NumClicks: e.NumClicks,
			})
		case gesture.KindCancel:
			for i := range b.history {
				b.history[i].Cancelled = true
				if b.history[i].End.IsZero() {
					b.history[i].End = gtx.Now
				}
			}
		case gesture.KindPress:
			if e.Source == pointer.Mouse {
				key.FocusOp{Tag: &b.keyTag}.Add(gtx.Ops)
			}
			b.history = append(b.history, Press{
				Position: e.Position,
				Start:    gtx.Now,
			})
		}
	}
	for _, e := range gtx.Events(&b.keyTag) {
		switch e := e.(type) {
		case key.FocusEvent:
			b.focused = e.Focus
			if !b.focused {
				b.pressedKey = ""
			}
		case key.Event:
			if !b.focused {
				break
			}
			if e.Name != key.NameReturn && e.Name != key.NameSpace {
				break
			}
			switch e.State {
			case key.Press:
				b.pressedKey = e.Name
			case key.Release:
				if b.pressedKey != e.Name {
					break
				}
				// only register a key as a click if the key was pressed and released while this button was focused
				b.pressedKey = ""
				clicks = append(clicks, Click{
					Modifiers: e.Modifiers,
					NumClicks: 1,
				})
			}
		}
	}
	return clicks
}
