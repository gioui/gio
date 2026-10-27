// SPDX-License-Identifier: Unlicense OR MIT

package gesture

import (
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

// Swipe detects multi-finger swipe gestures from raw touch events. The
// default minimum finger count is three.
type Swipe struct {
	// MinFingers is the minimum number of touches required. Zero means 3.
	MinFingers int

	points    map[pointer.ID]f32.Point
	active    bool
	gestureID uint32
	source    pointer.Source
	start     f32.Point
	center    f32.Point
	fingers   int
	pending   []pointer.Event
}

// Add registers the gesture to receive pointer events.
func (s *Swipe) Add(ops *op.Ops) {
	event.Op(ops, s)
}

// Update consumes raw pointer events and returns the next semantic Swipe
// event, if any.
func (s *Swipe) Update(q PointerSource) (pointer.Event, bool) {
	if len(s.pending) > 0 {
		e := s.pending[0]
		s.pending = s.pending[1:]
		return e, true
	}
	for {
		ev, ok := q.Event(pointer.Filter{
			Target: s,
			Kinds:  pointer.Press | pointer.Move | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		s.handle(e)
	}
	if len(s.pending) == 0 {
		return pointer.Event{}, false
	}
	e := s.pending[0]
	s.pending = s.pending[1:]
	return e, true
}

func (s *Swipe) minFingers() int {
	if s.MinFingers <= 0 {
		return 3
	}
	return s.MinFingers
}

func (s *Swipe) handle(e pointer.Event) {
	if e.Kind != pointer.Cancel && e.Source != pointer.Touch && e.Source != pointer.Trackpad {
		return
	}
	if s.points == nil {
		s.points = make(map[pointer.ID]f32.Point)
	}
	switch e.Kind {
	case pointer.Press:
		s.points[e.PointerID] = e.Position
		if !s.active && len(s.points) >= s.minFingers() {
			s.active = true
			s.source = e.Source
			s.fingers = len(s.points)
			s.center = s.centroid()
			s.start = s.center
			s.gestureID++
			s.pending = append(s.pending, pointer.Event{
				Kind:      pointer.Swipe,
				Source:    s.source,
				GestureID: s.gestureID,
				Phase:     pointer.GestureBegin,
				Fingers:   uint8(s.fingers),
				Position:  s.center,
			})
		}
	case pointer.Move, pointer.Drag:
		if _, ok := s.points[e.PointerID]; !ok {
			return
		}
		s.points[e.PointerID] = e.Position
		if s.active {
			s.fingers = len(s.points)
			s.center = s.centroid()
			s.pending = append(s.pending, pointer.Event{
				Kind:      pointer.Swipe,
				Source:    s.source,
				GestureID: s.gestureID,
				Phase:     pointer.GestureUpdate,
				Swipe:     s.center.Sub(s.start),
				Fingers:   uint8(s.fingers),
				Position:  s.center,
				Time:      e.Time,
			})
		}
	case pointer.Release, pointer.Cancel:
		if e.Kind == pointer.Cancel && e.PointerID == 0 {
			if s.active {
				s.pending = append(s.pending, pointer.Event{
					Kind:      pointer.Swipe,
					Source:    s.source,
					GestureID: s.gestureID,
					Phase:     pointer.GestureCancel,
					Swipe:     s.center.Sub(s.start),
					Fingers:   uint8(s.fingers),
					Position:  s.center,
					Time:      e.Time,
				})
			}
			s.active = false
			clear(s.points)
			return
		}
		delete(s.points, e.PointerID)
		if !s.active {
			return
		}
		s.fingers = len(s.points)
		s.center = s.centroid()
		if s.fingers < s.minFingers() {
			s.pending = append(s.pending, pointer.Event{
				Kind:      pointer.Swipe,
				Source:    s.source,
				GestureID: s.gestureID,
				Phase:     pointer.GestureEnd,
				Swipe:     s.center.Sub(s.start),
				Fingers:   uint8(s.fingers),
				Position:  s.center,
				Time:      e.Time,
			})
			s.active = false
			clear(s.points)
		}
	}
}

func (s *Swipe) centroid() f32.Point {
	var p f32.Point
	for _, v := range s.points {
		p = p.Add(v)
	}
	if len(s.points) > 0 {
		p = p.Mul(1 / float32(len(s.points)))
	}
	return p
}
