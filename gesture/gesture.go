// SPDX-License-Identifier: Unlicense OR MIT

/*
Package gesture implements common pointer gestures.

Gestures accept low level pointer Events from an event
Queue and detect higher level actions such as clicks
and scrolling.
*/
package gesture

import (
	"image"
	"math"
	"runtime"
	"time"

	"gioui.org/f32"
	"gioui.org/internal/fling"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/unit"
)

// The duration is somewhat arbitrary.
const doubleClickDuration = 200 * time.Millisecond

// Hover detects the hover gesture for a pointer area.
type Hover struct {
	// entered tracks whether the pointer is inside the gesture.
	entered bool
	// pid is the pointer.ID.
	pid pointer.ID
}

// Add the gesture to detect hovering over the current pointer area.
func (h *Hover) Add(ops *op.Ops) {
	event.Op(ops, h)
}

// Update state and report whether a pointer is inside the area.
func (h *Hover) Update(q input.Source) bool {
	for {
		ev, ok := q.Event(pointer.Filter{
			Target: h,
			Kinds:  pointer.Enter | pointer.Leave | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Leave, pointer.Cancel:
			if h.entered && h.pid == e.PointerID {
				h.entered = false
			}
		case pointer.Enter:
			if !h.entered {
				h.pid = e.PointerID
			}
			if h.pid == e.PointerID {
				h.entered = true
			}
		}
	}
	return h.entered
}

// Click detects click gestures in the form
// of ClickEvents.
type Click struct {
	// clickedAt is the timestamp at which
	// the last click occurred.
	clickedAt time.Duration
	// clicks is incremented if successive clicks
	// are performed within a fixed duration.
	clicks int
	// pressed tracks whether the pointer is pressed.
	pressed bool
	// hovered tracks whether the pointer is inside the gesture.
	hovered bool
	// entered tracks whether an Enter event has been received.
	entered bool
	// pid is the pointer.ID.
	pid pointer.ID
}

// ClickEvent represent a click action, either a
// KindPress for the beginning of a click or a
// KindClick for a completed click.
type ClickEvent struct {
	Kind      ClickKind
	Position  image.Point
	Source    pointer.Source
	Modifiers key.Modifiers
	// NumClicks records successive clicks occurring
	// within a short duration of each other.
	NumClicks int
}

type ClickKind uint8

// Drag detects drag gestures in the form of pointer.Drag events.
type Drag struct {
	dragging bool
	pressed  bool
	pid      pointer.ID
	start    f32.Point
}

// Scroll detects scroll gestures and reduces them to
// scroll distances. Scroll recognizes mouse wheel
// movements as well as drag and fling touch gestures.
type Scroll struct {
	dragging  bool
	estimator fling.Extrapolation
	flinger   fling.Animation
	pid       pointer.ID
	last      int
	// Leftover scroll.
	scroll float32
}

type ScrollState uint8

type Axis uint8

const (
	Horizontal Axis = iota
	Vertical
	Both
)

const (
	// KindPress is reported for the first pointer
	// press.
	KindPress ClickKind = iota
	// KindClick is reported when a click action
	// is complete.
	KindClick
	// KindCancel is reported when the gesture is
	// cancelled.
	KindCancel
)

const (
	// StateIdle is the default scroll state.
	StateIdle ScrollState = iota
	// StateDragging is reported during drag gestures.
	StateDragging
	// StateFlinging is reported when a fling is
	// in progress.
	StateFlinging
)

const touchSlop = unit.Dp(3)

// Add the handler to the operation list to receive click events.
func (c *Click) Add(ops *op.Ops) {
	event.Op(ops, c)
}

// Hovered returns whether a pointer is inside the area.
func (c *Click) Hovered() bool {
	return c.hovered
}

// Pressed returns whether a pointer is pressing.
func (c *Click) Pressed() bool {
	return c.pressed
}

// Update state and return the next click events, if any.
func (c *Click) Update(q input.Source) (ClickEvent, bool) {
	for {
		evt, ok := q.Event(pointer.Filter{
			Target: c,
			Kinds:  pointer.Press | pointer.Release | pointer.Enter | pointer.Leave | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := evt.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Release:
			if !c.pressed || c.pid != e.PointerID {
				break
			}
			c.pressed = false
			if !c.entered || c.hovered {
				return ClickEvent{
					Kind:      KindClick,
					Position:  e.Position.Round(),
					Source:    e.Source,
					Modifiers: e.Modifiers,
					NumClicks: c.clicks,
				}, true
			} else {
				return ClickEvent{Kind: KindCancel}, true
			}
		case pointer.Cancel:
			wasPressed := c.pressed
			c.pressed = false
			c.hovered = false
			c.entered = false
			if wasPressed {
				return ClickEvent{Kind: KindCancel}, true
			}
		case pointer.Press:
			if c.pressed {
				break
			}
			if e.Source == pointer.Mouse && e.Buttons != pointer.ButtonPrimary {
				break
			}
			if !c.hovered {
				c.pid = e.PointerID
			}
			if c.pid != e.PointerID {
				break
			}
			c.pressed = true
			if e.Time-c.clickedAt < doubleClickDuration {
				c.clicks++
			} else {
				c.clicks = 1
			}
			c.clickedAt = e.Time
			return ClickEvent{Kind: KindPress, Position: e.Position.Round(), Source: e.Source, Modifiers: e.Modifiers, NumClicks: c.clicks}, true
		case pointer.Leave:
			if !c.pressed {
				c.pid = e.PointerID
			}
			if c.pid == e.PointerID {
				c.hovered = false
			}
		case pointer.Enter:
			if !c.pressed {
				c.pid = e.PointerID
			}
			if c.pid == e.PointerID {
				c.hovered = true
				c.entered = true
			}
		}
	}
	return ClickEvent{}, false
}

func (ClickEvent) ImplementsEvent() {}

// Add the handler to the operation list to receive scroll events.
// The bounds variable refers to the scrolling boundaries
// as defined in [pointer.Filter].
func (s *Scroll) Add(ops *op.Ops) {
	event.Op(ops, s)
}

// Stop any remaining fling movement.
func (s *Scroll) Stop() {
	s.flinger = fling.Animation{}
}

// Update state and report the scroll distance along axis.
func (s *Scroll) Update(cfg unit.Metric, q input.Source, t time.Time, axis Axis, scrollx, scrolly pointer.ScrollRange) int {
	total := 0
	f := pointer.Filter{
		Target:  s,
		Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Scroll | pointer.Cancel,
		ScrollX: scrollx,
		ScrollY: scrolly,
	}
	for {
		evt, ok := q.Event(f)
		if !ok {
			break
		}
		e, ok := evt.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			if s.dragging {
				break
			}
			// Only scroll on touch drags, or on Android where mice
			// drags also scroll by convention.
			if e.Source != pointer.Touch && runtime.GOOS != "android" {
				break
			}
			s.Stop()
			s.estimator = fling.Extrapolation{}
			v := s.val(axis, e.Position)
			s.last = int(math.Round(float64(v)))
			s.estimator.Sample(e.Time, v)
			s.dragging = true
			s.pid = e.PointerID
		case pointer.Release:
			if s.pid != e.PointerID {
				break
			}
			fling := s.estimator.Estimate()
			if slop, d := float32(cfg.Dp(touchSlop)), fling.Distance; d < -slop || d > slop {
				s.flinger.Start(cfg, t, fling.Velocity)
			}
			fallthrough
		case pointer.Cancel:
			s.dragging = false
		case pointer.Scroll:
			switch axis {
			case Horizontal:
				s.scroll += e.Scroll.X
			case Vertical:
				s.scroll += e.Scroll.Y
			}
			iscroll := int(s.scroll)
			s.scroll -= float32(iscroll)
			total += iscroll
		case pointer.Drag:
			if !s.dragging || s.pid != e.PointerID {
				continue
			}
			val := s.val(axis, e.Position)
			s.estimator.Sample(e.Time, val)
			v := int(math.Round(float64(val)))
			dist := s.last - v
			if e.Priority < pointer.Grabbed {
				slop := cfg.Dp(touchSlop)
				if dist := dist; dist >= slop || -slop >= dist {
					q.Execute(pointer.GrabCmd{Tag: s, ID: e.PointerID})
				}
			} else {
				s.last = v
				total += dist
			}
		}
	}
	total += s.flinger.Tick(t)
	if s.flinger.Active() {
		q.Execute(op.InvalidateCmd{})
	}
	return total
}

func (s *Scroll) val(axis Axis, p f32.Point) float32 {
	if axis == Horizontal {
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

// Add the handler to the operation list to receive drag events.
func (d *Drag) Add(ops *op.Ops) {
	event.Op(ops, d)
}

// Update state and return the next drag event, if any.
func (d *Drag) Update(cfg unit.Metric, q input.Source, axis Axis) (pointer.Event, bool) {
	for {
		ev, ok := q.Event(pointer.Filter{
			Target: d,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}

		switch e.Kind {
		case pointer.Press:
			if !(e.Buttons == pointer.ButtonPrimary || e.Source == pointer.Touch) {
				continue
			}
			d.pressed = true
			if d.dragging {
				continue
			}
			d.dragging = true
			d.pid = e.PointerID
			d.start = e.Position
		case pointer.Drag:
			if !d.dragging || e.PointerID != d.pid {
				continue
			}
			switch axis {
			case Horizontal:
				e.Position.Y = d.start.Y
			case Vertical:
				e.Position.X = d.start.X
			case Both:
				// Do nothing
			}
			if e.Priority < pointer.Grabbed {
				diff := e.Position.Sub(d.start)
				slop := cfg.Dp(touchSlop)
				if diff.X*diff.X+diff.Y*diff.Y > float32(slop*slop) {
					q.Execute(pointer.GrabCmd{Tag: d, ID: e.PointerID})
				}
			}
		case pointer.Release, pointer.Cancel:
			d.pressed = false
			if !d.dragging || e.PointerID != d.pid {
				continue
			}
			d.dragging = false
		}

		return e, true
	}

	return pointer.Event{}, false
}

// Dragging reports whether it is currently in use.
func (d *Drag) Dragging() bool { return d.dragging }

// Pressed returns whether a pointer is pressing.
func (d *Drag) Pressed() bool { return d.pressed }

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

func (ct ClickKind) String() string {
	switch ct {
	case KindPress:
		return "KindPress"
	case KindClick:
		return "KindClick"
	case KindCancel:
		return "KindCancel"
	default:
		panic("invalid ClickKind")
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
