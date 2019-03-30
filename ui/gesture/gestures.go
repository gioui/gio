// SPDX-License-Identifier: Unlicense OR MIT

package gesture

import (
	"image"
	"math"
	"runtime"
	"time"

	"gioui.org/ui/f32"
	"gioui.org/ui/pointer"
	"gioui.org/ui"
)

type ClickEvent struct {
	Type     ClickType
	Position f32.Point
}

type ClickState uint8
type ClickType uint8

type Click struct {
	State ClickState
}

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
	StateNormal ClickState = iota
	StateFocused
	StatePressed
)

const (
	TypePress ClickType = iota
	TypeClick
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

func (c *Click) Op(a pointer.Area) pointer.OpHandler {
	return pointer.OpHandler{Area: a, Key: c}
}

func (c *Click) Update(q pointer.Events) []ClickEvent {
	var events []ClickEvent
	for _, e := range q.For(c) {
		switch e.Type {
		case pointer.Release:
			if c.State == StatePressed {
				events = append(events, ClickEvent{Type: TypeClick, Position: e.Position})
			}
			c.State = StateNormal
		case pointer.Cancel:
			c.State = StateNormal
		case pointer.Press:
			if c.State == StatePressed || !e.Hit {
				break
			}
			c.State = StatePressed
			events = append(events, ClickEvent{Type: TypePress, Position: e.Position})
		case pointer.Move:
			if c.State == StatePressed && !e.Hit {
				c.State = StateNormal
			} else if c.State < StateFocused {
				c.State = StateFocused
			}
		}
	}
	return events
}

func (s *Scroll) Op(a pointer.Area) ui.Op {
	oph := pointer.OpHandler{Area: a, Key: s, Grab: s.grab}
	if !s.flinger.Active() {
		return oph
	}
	return ui.Ops{oph, ui.OpRedraw{}}
}

func (s *Scroll) Stop() {
	s.flinger = flinger{}
}

func (s *Scroll) Dragging() bool {
	return s.dragging
}

func (s *Scroll) Scroll(cfg *ui.Config, q pointer.Events, axis Axis) int {
	if s.axis != axis {
		s.axis = axis
		return 0
	}
	total := 0
	for _, e := range q.For(s) {
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
			if slop, d := cfg.Pixels(touchSlop), fling.Distance; d >= slop || -slop >= d {
				if min, v := cfg.Pixels(minFlingVelocity), fling.Velocity; v >= min || -min >= v {
					max := cfg.Pixels(maxFlingVelocity)
					if v > max {
						v = max
					} else if v < -max {
						v = -max
					}
					s.flinger.Init(cfg.Now, v)
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
				slop := cfg.Pixels(touchSlop)
				if dist := float32(dist); dist >= slop || -slop >= dist {
					s.grab = true
				}
			} else {
				s.last = v
				total += dist
			}
		}
	}
	total += s.flinger.Tick(cfg.Now)
	return total
}

func (s *Scroll) val(p f32.Point) float32 {
	if s.axis == Horizontal {
		return p.X
	} else {
		return p.Y
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

func Rect(sz image.Point) pointer.Area {
	return func(pos f32.Point) pointer.HitResult {
		if 0 <= pos.X && pos.X < float32(sz.X) &&
			0 <= pos.Y && pos.Y < float32(sz.Y) {
			return pointer.HitOpaque
		} else {
			return pointer.HitNone
		}
	}
}

func Ellipse(sz image.Point) pointer.Area {
	return func(pos f32.Point) pointer.HitResult {
		rx := float32(sz.X) / 2
		ry := float32(sz.Y) / 2
		rx2 := rx * rx
		ry2 := ry * ry
		xh := pos.X - rx
		yk := pos.Y - ry
		if xh*xh*ry2+yk*yk*rx2 <= rx2*ry2 {
			return pointer.HitOpaque
		} else {
			return pointer.HitNone
		}
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
