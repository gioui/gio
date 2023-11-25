// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// Clickable represents a clickable area.
type Clickable struct {
	click   gesture.Click
	history []Press

	keyTag        struct{}
	requestClicks int
	focused       bool
	pressedKey    key.Name
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

// Clicked calls Update and reports whether a click was registered.
func (b *Clickable) Clicked(gtx layout.Context) bool {
	_, clicked := b.Update(gtx)
	return clicked
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
func (b *Clickable) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: &b.keyTag})
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
	for {
		_, ok := b.Update(gtx)
		if !ok {
			break
		}
	}
	m := op.Record(gtx.Ops)
	dims := w(gtx)
	c := m.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	semantic.EnabledOp(gtx.Enabled()).Add(gtx.Ops)
	b.click.Add(gtx.Ops)
	event.InputOp(gtx.Ops, &b.keyTag)
	c.Add(gtx.Ops)
	return dims
}

// Update the button state by processing events, and return the next
// click, if any.
func (b *Clickable) Update(gtx layout.Context) (Click, bool) {
	if !gtx.Enabled() {
		b.focused = false
	}
	for len(b.history) > 0 {
		c := b.history[0]
		if c.End.IsZero() || gtx.Now.Sub(c.End) < 1*time.Second {
			break
		}
		n := copy(b.history, b.history[1:])
		b.history = b.history[:n]
	}
	if c := b.requestClicks; c > 0 {
		b.requestClicks = 0
		return Click{
			NumClicks: c,
		}, true
	}
	for _, e := range b.click.Update(gtx.Source) {
		switch e.Kind {
		case gesture.KindClick:
			if l := len(b.history); l > 0 {
				b.history[l-1].End = gtx.Now
			}
			return Click{
				Modifiers: e.Modifiers,
				NumClicks: e.NumClicks,
			}, true
		case gesture.KindCancel:
			for i := range b.history {
				b.history[i].Cancelled = true
				if b.history[i].End.IsZero() {
					b.history[i].End = gtx.Now
				}
			}
		case gesture.KindPress:
			if e.Source == pointer.Mouse {
				gtx.Execute(key.FocusCmd{Tag: &b.keyTag})
			}
			b.history = append(b.history, Press{
				Position: e.Position,
				Start:    gtx.Now,
			})
		}
	}
	filters := []event.Filter{
		key.FocusFilter{},
	}
	if b.focused {
		filters = append(filters, key.Filter{Name: key.NameReturn}, key.Filter{Name: key.NameSpace})
	}
	for {
		e, ok := gtx.Event(&b.keyTag, filters...)
		if !ok {
			break
		}
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
				return Click{
					Modifiers: e.Modifiers,
					NumClicks: 1,
				}, true
			}
		}
	}
	return Click{}, false
}
