// SPDX-License-Identifier: Unlicense OR MIT

package gesture

import (
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

// Hold detects a multi-finger hold gesture from raw touch events. Hold
// begins after Duration without significant movement and ends when a
// participating finger is released.
type Hold struct {
	// MinFingers is the minimum number of touches required. Zero means 1.
	MinFingers int
	// Duration is the time without significant movement before the hold
	// begins. Zero means 500ms.
	Duration time.Duration
	// Slop is the maximum movement in pixels before the hold is cancelled.
	// Zero means 3.
	Slop float32

	points    map[pointer.ID]f32.Point
	waiting   bool
	active    bool
	gestureID uint32
	source    pointer.Source
	start     f32.Point
	center    f32.Point
	fingers   int
	startedAt time.Time
	pending   []pointer.Event
}

// Add registers the gesture to receive pointer events.
func (h *Hold) Add(ops *op.Ops) {
	event.Op(ops, h)
}

// Update consumes raw pointer events and checks the hold timer. The caller
// should invoke Update every frame with the current time.
func (h *Hold) Update(q input.Source, now time.Time) (pointer.Event, bool) {
	if len(h.pending) > 0 {
		e := h.pending[0]
		h.pending = h.pending[1:]
		return e, true
	}
	for {
		ev, ok := q.Event(pointer.Filter{
			Target: h,
			Kinds:  pointer.Press | pointer.Move | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		h.handle(e, now)
	}
	if h.waiting && !h.active && now.Sub(h.startedAt) >= h.duration() {
		h.waiting = false
		h.active = true
		h.center = h.centroid()
		h.pending = append(h.pending, pointer.Event{
			Kind:      pointer.Hold,
			Source:    h.source,
			GestureID: h.gestureID,
			Phase:     pointer.GestureBegin,
			Fingers:   uint8(h.fingers),
			Position:  h.center,
			Time:      now.Sub(h.startedAt),
		})
	}
	if len(h.pending) == 0 {
		return pointer.Event{}, false
	}
	e := h.pending[0]
	h.pending = h.pending[1:]
	return e, true
}

func (h *Hold) minFingers() int {
	if h.MinFingers <= 0 {
		return 1
	}
	return h.MinFingers
}

func (h *Hold) duration() time.Duration {
	if h.Duration <= 0 {
		return 500 * time.Millisecond
	}
	return h.Duration
}

func (h *Hold) slop() float32 {
	if h.Slop <= 0 {
		return 3
	}
	return h.Slop
}

func (h *Hold) handle(e pointer.Event, now time.Time) {
	if e.Kind != pointer.Cancel && e.Source != pointer.Touch && e.Source != pointer.Trackpad {
		return
	}
	if h.points == nil {
		h.points = make(map[pointer.ID]f32.Point)
	}
	switch e.Kind {
	case pointer.Press:
		h.points[e.PointerID] = e.Position
		if !h.waiting && !h.active && len(h.points) >= h.minFingers() {
			h.waiting = true
			h.source = e.Source
			h.fingers = len(h.points)
			h.center = h.centroid()
			h.start = h.center
			h.startedAt = now
			h.gestureID++
		}
	case pointer.Move, pointer.Drag:
		if _, ok := h.points[e.PointerID]; !ok {
			return
		}
		h.points[e.PointerID] = e.Position
		if h.waiting || h.active {
			h.fingers = len(h.points)
			h.center = h.centroid()
			if h.moved() > h.slop() {
				if h.active {
					h.pending = append(h.pending, pointer.Event{
						Kind:      pointer.Hold,
						Source:    h.source,
						GestureID: h.gestureID,
						Phase:     pointer.GestureCancel,
						Fingers:   uint8(h.fingers),
						Position:  h.center,
						Time:      e.Time,
					})
				}
				h.waiting = false
				h.active = false
			}
		}
	case pointer.Release, pointer.Cancel:
		if e.Kind == pointer.Cancel && e.PointerID == 0 {
			if h.active {
				h.pending = append(h.pending, pointer.Event{
					Kind:      pointer.Hold,
					Source:    h.source,
					GestureID: h.gestureID,
					Phase:     pointer.GestureCancel,
					Fingers:   uint8(h.fingers),
					Position:  h.center,
					Time:      e.Time,
				})
			}
			h.waiting = false
			h.active = false
			clear(h.points)
			return
		}
		delete(h.points, e.PointerID)
		if h.active {
			h.fingers = len(h.points)
			h.center = h.centroid()
			h.pending = append(h.pending, pointer.Event{
				Kind:      pointer.Hold,
				Source:    h.source,
				GestureID: h.gestureID,
				Phase:     pointer.GestureEnd,
				Fingers:   uint8(h.fingers),
				Position:  h.center,
				Time:      e.Time,
			})
			h.active = false
			h.waiting = false
		}
		if len(h.points) < h.minFingers() {
			h.waiting = false
			h.active = false
		}
	}
}

func (h *Hold) moved() float32 {
	d := h.center.Sub(h.start)
	return float32(math.Hypot(float64(d.X), float64(d.Y)))
}

func (h *Hold) centroid() f32.Point {
	var p f32.Point
	for _, v := range h.points {
		p = p.Add(v)
	}
	if len(h.points) > 0 {
		p = p.Mul(1 / float32(len(h.points)))
	}
	return p
}
