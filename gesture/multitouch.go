// SPDX-License-Identifier: Unlicense OR MIT

package gesture

import (
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

// PointerSource is the event-source subset needed by the multi-touch
// recognizers. input.Source implements it; tests and backends can provide
// their own replay source.
type PointerSource interface {
	Event(filters ...event.Filter) (event.Event, bool)
}

// Pinch detects two-finger pinch gestures from raw touch events and emits
// semantic pointer.Pinch events. It is intended as a fallback for platforms
// without native semantic gestures (Android, iOS, Web, X11).
//
// Update consumes all currently available pointer events and returns one
// semantic event per call. Callers should loop until ok is false.
type Pinch struct {
	id1, id2  pointer.ID
	hasTwo    bool
	active    bool
	source    pointer.Source
	startDist float32
	center    f32.Point
	gestureID uint32
	points    map[pointer.ID]f32.Point
	pending   []pointer.Event
}

// Add registers the gesture to receive pointer events.
func (p *Pinch) Add(ops *op.Ops) {
	event.Op(ops, p)
}

// Update consumes raw pointer events and returns the next semantic Pinch
// event, if any.
func (p *Pinch) Update(q PointerSource) (pointer.Event, bool) {
	if len(p.pending) > 0 {
		e := p.pending[0]
		p.pending = p.pending[1:]
		return e, true
	}
	for {
		ev, ok := q.Event(pointer.Filter{
			Target: p,
			Kinds:  pointer.Press | pointer.Move | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		p.handle(e)
	}
	if len(p.pending) == 0 {
		return pointer.Event{}, false
	}
	e := p.pending[0]
	p.pending = p.pending[1:]
	return e, true
}

func (p *Pinch) handle(e pointer.Event) {
	if e.Kind != pointer.Cancel && e.Source != pointer.Touch && e.Source != pointer.Trackpad {
		return
	}
	if p.points == nil {
		p.points = make(map[pointer.ID]f32.Point)
	}
	switch e.Kind {
	case pointer.Press:
		p.points[e.PointerID] = e.Position
		if !p.hasTwo {
			p.source = e.Source
			p.pickPair()
		}
	case pointer.Move, pointer.Drag:
		if _, ok := p.points[e.PointerID]; !ok {
			return
		}
		p.points[e.PointerID] = e.Position
		if p.active {
			p.emitUpdate(e)
		}
	case pointer.Release, pointer.Cancel:
		if e.Kind == pointer.Cancel && e.PointerID == 0 {
			if p.active {
				p.emit(pointer.GestureCancel, e)
			}
			p.active = false
			p.hasTwo = false
			clear(p.points)
			return
		}
		if !p.active {
			delete(p.points, e.PointerID)
			p.hasTwo = false
			p.pickPair()
			return
		}
		if e.PointerID != p.id1 && e.PointerID != p.id2 {
			delete(p.points, e.PointerID)
			return
		}
		phase := pointer.GestureEnd
		if e.Kind == pointer.Cancel {
			phase = pointer.GestureCancel
		}
		p.emit(phase, e)
		p.active = false
		p.hasTwo = false
		delete(p.points, e.PointerID)
		p.pickPair()
	}
}

func (p *Pinch) pickPair() {
	if p.hasTwo || len(p.points) < 2 {
		return
	}
	first := true
	for id := range p.points {
		if first {
			p.id1 = id
			first = false
		} else {
			p.id2 = id
			break
		}
	}
	p.hasTwo = true
	p.startDist = p.distance()
	if p.startDist < 1 {
		p.startDist = 1
	}
	p.center = p.midpoint()
	p.gestureID++
	p.active = true
	p.pending = append(p.pending, pointer.Event{
		Kind:      pointer.Pinch,
		Source:    p.source,
		GestureID: p.gestureID,
		Phase:     pointer.GestureBegin,
		Scale:     1,
		Fingers:   2,
		Position:  p.center,
	})
}

func (p *Pinch) emitUpdate(e pointer.Event) {
	if !p.hasTwo {
		return
	}
	scale := p.distance() / p.startDist
	p.center = p.midpoint()
	p.pending = append(p.pending, pointer.Event{
		Kind:      pointer.Pinch,
		Source:    p.source,
		GestureID: p.gestureID,
		Phase:     pointer.GestureUpdate,
		Scale:     scale,
		Fingers:   2,
		Position:  p.center,
		Time:      e.Time,
	})
}

func (p *Pinch) emit(phase pointer.GesturePhase, e pointer.Event) {
	p.pending = append(p.pending, pointer.Event{
		Kind:      pointer.Pinch,
		Source:    p.source,
		GestureID: p.gestureID,
		Phase:     phase,
		Scale:     p.distance() / p.startDist,
		Fingers:   2,
		Position:  p.center,
		Time:      e.Time,
	})
}

func (p *Pinch) distance() float32 {
	a := p.points[p.id1]
	b := p.points[p.id2]
	dx, dy := a.X-b.X, a.Y-b.Y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

func (p *Pinch) midpoint() f32.Point {
	a := p.points[p.id1]
	b := p.points[p.id2]
	return f32.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
}

// Rotate detects two-finger rotation gestures from raw touch events and
// emits semantic pointer.Rotate events.
type Rotate struct {
	id1, id2  pointer.ID
	hasTwo    bool
	active    bool
	source    pointer.Source
	lastAngle float32
	rotation  float32
	center    f32.Point
	gestureID uint32
	points    map[pointer.ID]f32.Point
	pending   []pointer.Event
}

// Add registers the gesture to receive pointer events.
func (r *Rotate) Add(ops *op.Ops) {
	event.Op(ops, r)
}

// Update consumes raw pointer events and returns the next semantic Rotate
// event, if any.
func (r *Rotate) Update(q PointerSource) (pointer.Event, bool) {
	if len(r.pending) > 0 {
		e := r.pending[0]
		r.pending = r.pending[1:]
		return e, true
	}
	for {
		ev, ok := q.Event(pointer.Filter{
			Target: r,
			Kinds:  pointer.Press | pointer.Move | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		r.handle(e)
	}
	if len(r.pending) == 0 {
		return pointer.Event{}, false
	}
	e := r.pending[0]
	r.pending = r.pending[1:]
	return e, true
}

func (r *Rotate) handle(e pointer.Event) {
	if e.Kind != pointer.Cancel && e.Source != pointer.Touch && e.Source != pointer.Trackpad {
		return
	}
	if r.points == nil {
		r.points = make(map[pointer.ID]f32.Point)
	}
	switch e.Kind {
	case pointer.Press:
		r.points[e.PointerID] = e.Position
		if !r.hasTwo {
			r.source = e.Source
			r.pickPair()
		}
	case pointer.Move, pointer.Drag:
		if _, ok := r.points[e.PointerID]; !ok {
			return
		}
		r.points[e.PointerID] = e.Position
		if r.active {
			r.emitUpdate(e)
		}
	case pointer.Release, pointer.Cancel:
		if e.Kind == pointer.Cancel && e.PointerID == 0 {
			if r.active {
				r.emit(pointer.GestureCancel, e)
			}
			r.active = false
			r.hasTwo = false
			clear(r.points)
			return
		}
		if !r.active {
			delete(r.points, e.PointerID)
			r.hasTwo = false
			r.pickPair()
			return
		}
		if e.PointerID != r.id1 && e.PointerID != r.id2 {
			delete(r.points, e.PointerID)
			return
		}
		phase := pointer.GestureEnd
		if e.Kind == pointer.Cancel {
			phase = pointer.GestureCancel
		}
		r.emit(phase, e)
		r.active = false
		r.hasTwo = false
		delete(r.points, e.PointerID)
		r.pickPair()
	}
}

func (r *Rotate) pickPair() {
	if r.hasTwo || len(r.points) < 2 {
		return
	}
	first := true
	for id := range r.points {
		if first {
			r.id1 = id
			first = false
		} else {
			r.id2 = id
			break
		}
	}
	r.hasTwo = true
	r.lastAngle = r.angle()
	r.rotation = 0
	r.center = r.midpoint()
	r.gestureID++
	r.active = true
	r.pending = append(r.pending, pointer.Event{
		Kind:      pointer.Rotate,
		Source:    r.source,
		GestureID: r.gestureID,
		Phase:     pointer.GestureBegin,
		Fingers:   2,
		Position:  r.center,
	})
}

func (r *Rotate) emitUpdate(e pointer.Event) {
	a := r.angle()
	delta := a - r.lastAngle
	for delta > math.Pi {
		delta -= 2 * math.Pi
	}
	for delta < -math.Pi {
		delta += 2 * math.Pi
	}
	r.lastAngle = a
	r.rotation += delta
	r.center = r.midpoint()
	r.pending = append(r.pending, pointer.Event{
		Kind:      pointer.Rotate,
		Source:    r.source,
		GestureID: r.gestureID,
		Phase:     pointer.GestureUpdate,
		Rotation:  r.rotation,
		Fingers:   2,
		Position:  r.center,
		Time:      e.Time,
	})
}

func (r *Rotate) emit(phase pointer.GesturePhase, e pointer.Event) {
	r.pending = append(r.pending, pointer.Event{
		Kind:      pointer.Rotate,
		Source:    r.source,
		GestureID: r.gestureID,
		Phase:     phase,
		Rotation:  r.rotation,
		Fingers:   2,
		Position:  r.center,
		Time:      e.Time,
	})
}

func (r *Rotate) angle() float32 {
	a := r.points[r.id1]
	b := r.points[r.id2]
	return float32(math.Atan2(float64(a.Y-b.Y), float64(a.X-b.X)))
}

func (r *Rotate) midpoint() f32.Point {
	a := r.points[r.id1]
	b := r.points[r.id2]
	return f32.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
}
