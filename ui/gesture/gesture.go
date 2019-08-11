// SPDX-License-Identifier: Unlicense OR MIT

/*
Package gesture implements common pointer gestures.

Gestures accept low level pointer Events from an input
Queue and detect higher level actions such as clicks
and scrolling.
*/
package gesture

import (
	"math"
	"runtime"
	"time"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/input"
	"gioui.org/ui/pointer"
)

// Click detects click gestures in the form
// of ClickEvents.
type Click struct {
	// state tracks the gesture state.
	state ClickState
}

type ClickState uint8

// ClickEvent represent a click action, either a
// TypePress for the beginning of a click or a
// TypeClick for a completed click.
type ClickEvent struct {
	Type     ClickType
	Position f32.Point
	Source   pointer.Source
}

type ClickType uint8

// Scroll detects scroll gestures and reduces them to
// scroll distances. Scroll recognizes mouse wheel
// movements as well as drag and fling touch gestures.
type Scroll struct {
	dragging  bool
	axis      Axis
	estimator estimator
	flinger   flinger
	pid       pointer.ID
	grab      bool
	last      int
	// Leftover scroll.
	scroll float32
}

type ScrollState uint8

type flinger struct {
	// Current offset in pixels.
	x float32
	// Initial time.
	t0 time.Time
	// Initial velocity in pixels pr second.
	v0 float32
}

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
	// TypeClick is reporoted when a click action
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

var (
	touchSlop = ui.Dp(3)
	// Pixels/second.
	minFlingVelocity = ui.Dp(50)
	maxFlingVelocity = ui.Dp(8000)
)

const (
	thresholdVelocity = 1
)

// Add the handler to the operation list to receive click events.
func (c *Click) Add(ops *ui.Ops) {
	op := pointer.HandlerOp{Key: c}
	op.Add(ops)
}

// State reports the click state.
func (c *Click) State() ClickState {
	return c.state
}

// Events reports all click events for the available events.
func (c *Click) Events(q input.Queue) []ClickEvent {
	var events []ClickEvent
	for evt, ok := q.Next(c); ok; evt, ok = q.Next(c) {
		e, ok := evt.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Type {
		case pointer.Release:
			wasPressed := c.state == StatePressed
			c.state = StateNormal
			if wasPressed {
				events = append(events, ClickEvent{Type: TypeClick, Position: e.Position, Source: e.Source})
			}
		case pointer.Cancel:
			c.state = StateNormal
		case pointer.Press:
			if c.state == StatePressed || !e.Hit {
				break
			}
			c.state = StatePressed
			events = append(events, ClickEvent{Type: TypePress, Position: e.Position, Source: e.Source})
		case pointer.Move:
			if c.state == StatePressed && !e.Hit {
				c.state = StateNormal
			} else if c.state < StateFocused {
				c.state = StateFocused
			}
		}
	}
	return events
}

// Add the handler to the operation list to receive scroll events.
func (s *Scroll) Add(ops *ui.Ops) {
	oph := pointer.HandlerOp{Key: s, Grab: s.grab}
	oph.Add(ops)
	if s.flinger.Active() {
		ui.InvalidateOp{}.Add(ops)
	}
}

// Stop any remaining fling movement.
func (s *Scroll) Stop() {
	s.flinger = flinger{}
}

// Scroll detects the scrolling distance from the available events and
// ongoing fling gestures.
func (s *Scroll) Scroll(cfg ui.Config, q input.Queue, axis Axis) int {
	if s.axis != axis {
		s.axis = axis
		return 0
	}
	total := 0
	for evt, ok := q.Next(s); ok; evt, ok = q.Next(s) {
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
			s.estimator = estimator{}
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
			if slop, d := float32(cfg.Px(touchSlop)), fling.Distance; d >= slop || -slop >= d {
				if min, v := float32(cfg.Px(minFlingVelocity)), fling.Velocity; v >= min || -min >= v {
					max := float32(cfg.Px(maxFlingVelocity))
					if v > max {
						v = max
					} else if v < -max {
						v = -max
					}
					s.flinger.Init(cfg.Now(), v)
				}
			}
			fallthrough
		case pointer.Cancel:
			s.dragging = false
			s.grab = false
		case pointer.Move:
			// Scroll
			switch s.axis {
			case Horizontal:
				s.scroll += e.Scroll.X
			case Vertical:
				s.scroll += e.Scroll.Y
			}
			iscroll := int(math.Round(float64(s.scroll)))
			s.scroll -= float32(iscroll)
			total += iscroll
			if !s.dragging || s.pid != e.PointerID {
				continue
			}
			// Drag
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
	total += s.flinger.Tick(cfg.Now())
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

func (f *flinger) Init(now time.Time, v0 float32) {
	f.t0 = now
	f.v0 = v0
	f.x = 0
}

func (f *flinger) Active() bool {
	return f.v0 != 0
}

// Tick computes and returns a fling distance since
// the last time Tick was called.
func (f *flinger) Tick(now time.Time) int {
	if !f.Active() {
		return 0
	}
	var k float32
	if runtime.GOOS == "darwin" {
		k = -2 // iOS
	} else {
		k = -4.2 // Android and default
	}
	t := now.Sub(f.t0)
	// The acceleration x''(t) of a point mass with a drag
	// force, f, proportional with velocity, x'(t), is
	// governed by the equation
	//
	// x''(t) = kx'(t)
	//
	// Given the starting position x(0) = 0, the starting
	// velocity x'(0) = v0, the position is then
	// given by
	//
	// x(t) = v0*e^(k*t)/k - v0/k
	//
	ekt := float32(math.Exp(float64(k) * t.Seconds()))
	x := f.v0*ekt/k - f.v0/k
	dist := x - f.x
	idist := int(math.Round(float64(dist)))
	f.x += float32(idist)
	// Solving for the velocity x'(t) gives us
	//
	// x'(t) = v0*e^(k*t)
	v := f.v0 * ekt
	if v < thresholdVelocity && v > -thresholdVelocity {
		f.v0 = 0
	}
	return idist
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
