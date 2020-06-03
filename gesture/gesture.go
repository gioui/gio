// SPDX-License-Identifier: Unlicense OR MIT

/*
Package gesture implements common pointer gestures.

Gestures accept low level pointer Events from an event
Queue and detect higher level actions such as clicks
and scrolling.
*/
package gesture

import (
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/internal/fling"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/unit"
)

// The duration is somewhat arbitrary.
const doubleClickDuration = 200 * time.Millisecond

// Click detects click gestures in the form
// of ClickEvents.
type Click struct {
	// state tracks the gesture state.
	state ClickState
	// clickedAt is the timestamp at which
	// the last click occurred.
	clickedAt time.Duration
	// clicks is incremented if successive clicks
	// are performed within a fixed duration.
	clicks int
}

type ClickState uint8

// ClickEvent represent a click action, either a
// TypePress for the beginning of a click or a
// TypeClick for a completed click.
type ClickEvent struct {
	Type      ClickType
	Position  f32.Point
	Source    pointer.Source
	Modifiers key.Modifiers
	// NumClicks records successive clicks occurring
	// within a short duration of each other.
	NumClicks int
}

type ClickType uint8

// Scroll detects scroll gestures and reduces them to
// scroll distances. Scroll recognizes mouse wheel
// movements as well as drag and fling touch gestures.
type Scroll struct {
	dragging  bool
	axis      Axis
	estimator fling.Extrapolation
	flinger   fling.Animation
	pid       pointer.ID
	grab      bool
	last      int
	// Leftover scroll.
	scroll float32
}

type ScrollState uint8

type Axis uint8

const (
	Horizontal Axis = iota
	Vertical
)

const (
	// StateNormal is the default click state.
	StateNormal ClickState = iota
	// StateFocused is reported when a pointer
	// is hovering over the handler.
	StateFocused
	// StatePressed is then a pointer is pressed.
	StatePressed
)

const (
	// TypePress is reported for the first pointer
	// press.
	TypePress ClickType = iota
	// TypeClick is reported when a click action
	// is complete.
	TypeClick
)

const (
	// StateIdle is the default scroll state.
	StateIdle ScrollState = iota
	// StateDrag is reported during drag gestures.
	StateDragging
	// StateFlinging is reported when a fling is
	// in progress.
	StateFlinging
)

var touchSlop = unit.Dp(3)

// Add the handler to the operation list to receive click events.
func (c *Click) Add(ops *op.Ops) {
	op := pointer.InputOp{
		Tag:   c,
		Types: pointer.Press | pointer.Release | pointer.Enter | pointer.Leave,
	}
	op.Add(ops)
}

// State reports the click state.
func (c *Click) State() ClickState {
	return c.state
}

// Events returns the next click event, if any.
func (c *Click) Events(q event.Queue) []ClickEvent {
	var events []ClickEvent
	for _, evt := range q.Events(c) {
		e, ok := evt.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Type {
		case pointer.Release:
			wasPressed := c.state == StatePressed
			c.state = StateNormal
			if wasPressed {
				if e.Time-c.clickedAt < doubleClickDuration {
					c.clicks++
				} else {
					c.clicks = 1
				}
				c.clickedAt = e.Time
				events = append(events, ClickEvent{Type: TypeClick, Position: e.Position, Source: e.Source, Modifiers: e.Modifiers, NumClicks: c.clicks})
			}
		case pointer.Cancel:
			c.state = StateNormal
		case pointer.Press:
			if c.state == StatePressed {
				break
			}
			if e.Source == pointer.Mouse && e.Buttons != pointer.ButtonLeft {
				break
			}
			c.state = StatePressed
			events = append(events, ClickEvent{Type: TypePress, Position: e.Position, Source: e.Source, Modifiers: e.Modifiers})
		case pointer.Leave:
			if c.state == StatePressed {
				c.state = StateNormal
			}
		case pointer.Enter:
			if c.state < StateFocused {
				c.state = StateFocused
			}
		}
	}
	return events
}

// Add the handler to the operation list to receive scroll events.
func (s *Scroll) Add(ops *op.Ops) {
	oph := pointer.InputOp{
		Tag:   s,
		Grab:  s.grab,
		Types: pointer.Press | pointer.Move | pointer.Release | pointer.Scroll,
	}
	oph.Add(ops)
	if s.flinger.Active() {
		op.InvalidateOp{}.Add(ops)
	}
}

// Stop any remaining fling movement.
func (s *Scroll) Stop() {
	s.flinger = fling.Animation{}
}

// Scroll detects the scrolling distance from the available events and
// ongoing fling gestures.
func (s *Scroll) Scroll(cfg unit.Converter, q event.Queue, t time.Time, axis Axis) int {
	if s.axis != axis {
		s.axis = axis
		return 0
	}
	total := 0
	for _, evt := range q.Events(s) {
		e, ok := evt.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Type {
		case pointer.Press:
			if s.dragging || e.Source != pointer.Touch {
				break
			}
			s.Stop()
			s.estimator = fling.Extrapolation{}
			v := s.val(e.Position)
			s.last = int(math.Round(float64(v)))
			s.estimator.Sample(e.Time, v)
			s.dragging = true
			s.pid = e.PointerID
		case pointer.Release:
			if s.pid != e.PointerID {
				break
			}
			fling := s.estimator.Estimate()
			if slop, d := float32(cfg.Px(touchSlop)), fling.Distance; d < -slop || d > slop {
				s.flinger.Start(cfg, t, fling.Velocity)
			}
			fallthrough
		case pointer.Cancel:
			s.dragging = false
			s.grab = false
		case pointer.Scroll:
			switch s.axis {
			case Horizontal:
				s.scroll += e.Scroll.X
			case Vertical:
				s.scroll += e.Scroll.Y
			}
			iscroll := int(s.scroll)
			s.scroll -= float32(iscroll)
			total += iscroll
		case pointer.Move:
			if !s.dragging || s.pid != e.PointerID {
				continue
			}
			val := s.val(e.Position)
			s.estimator.Sample(e.Time, val)
			v := int(math.Round(float64(val)))
			dist := s.last - v
			if e.Priority < pointer.Grabbed {
				slop := cfg.Px(touchSlop)
				if dist := dist; dist >= slop || -slop >= dist {
					s.grab = true
				}
			} else {
				s.last = v
				total += dist
			}
		}
	}
	total += s.flinger.Tick(t)
	return total
}

func (s *Scroll) val(p f32.Point) float32 {
	if s.axis == Horizontal {
		return p.X
	} else {
		return p.Y
	}
}

// State reports the scroll state.
func (s *Scroll) State() ScrollState {
	switch {
	case s.flinger.Active():
		return StateFlinging
	case s.dragging:
		return StateDragging
	default:
		return StateIdle
	}
}

func (a Axis) String() string {
	switch a {
	case Horizontal:
		return "Horizontal"
	case Vertical:
		return "Vertical"
	default:
		panic("invalid Axis")
	}
}

func (ct ClickType) String() string {
	switch ct {
	case TypePress:
		return "TypePress"
	case TypeClick:
		return "TypeClick"
	default:
		panic("invalid ClickType")
	}
}

func (cs ClickState) String() string {
	switch cs {
	case StateNormal:
		return "StateNormal"
	case StateFocused:
		return "StateFocused"
	case StatePressed:
		return "StatePressed"
	default:
		panic("invalid ClickState")
	}
}

func (s ScrollState) String() string {
	switch s {
	case StateIdle:
		return "StateIdle"
	case StateDragging:
		return "StateDragging"
	case StateFlinging:
		return "StateFlinging"
	default:
		panic("unreachable")
	}
}
