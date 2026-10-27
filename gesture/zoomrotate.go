// SPDX-License-Identifier: Unlicense OR MIT

package gesture

import (
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

// ZoomRotate recognizes a single two-finger gesture as either a pinch or a
// rotation. Unlike running Pinch and Rotate side by side, it locks the
// dominant motion for the lifetime of the gesture; the other kind is never
// emitted.
//
// The default thresholds are 5% scale change and 0.08 rad (~4.6 degrees)
// rotation, with a 1.5x dominance ratio. Set a field to zero to use its
// default. Once locked, the mode is stable until all fingers are released.
type ZoomRotate struct {
	MinScaleDelta    float32
	MinRotationDelta float32
	Dominance        float32

	id1, id2   pointer.ID
	hasPair    bool
	blocked    bool
	mode       uint8
	source     pointer.Source
	startDist  float32
	startAngle float32
	scale      float32
	rotation   float32
	center     f32.Point
	gestureID  uint32
	points     map[pointer.ID]f32.Point
	moved      map[pointer.ID]bool
	pending    []pointer.Event
}

const (
	zoomRotateNone uint8 = iota
	zoomRotatePinch
	zoomRotateRotate
)

// Add registers the gesture to receive pointer events.
func (z *ZoomRotate) Add(ops *op.Ops) {
	event.Op(ops, z)
}

// Update consumes raw pointer events and returns the next semantic event.
func (z *ZoomRotate) Update(q PointerSource) (pointer.Event, bool) {
	if len(z.pending) > 0 {
		e := z.pending[0]
		z.pending = z.pending[1:]
		return e, true
	}
	for {
		ev, ok := q.Event(pointer.Filter{
			Target: z,
			Kinds:  pointer.Press | pointer.Move | pointer.Drag | pointer.Release | pointer.Cancel,
		})
		if !ok {
			break
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		z.handle(e)
	}
	if len(z.pending) == 0 {
		return pointer.Event{}, false
	}
	e := z.pending[0]
	z.pending = z.pending[1:]
	return e, true
}

// Reset clears all gesture state while preserving thresholds.
func (z *ZoomRotate) Reset() {
	minScale, minRot, dom := z.MinScaleDelta, z.MinRotationDelta, z.Dominance
	*z = ZoomRotate{MinScaleDelta: minScale, MinRotationDelta: minRot, Dominance: dom}
}

func (z *ZoomRotate) minScale() float32 {
	if z.MinScaleDelta <= 0 {
		return 0.05
	}
	return z.MinScaleDelta
}

func (z *ZoomRotate) minRotation() float32 {
	if z.MinRotationDelta <= 0 {
		return 0.08
	}
	return z.MinRotationDelta
}

func (z *ZoomRotate) dominance() float32 {
	if z.Dominance <= 0 {
		return 1.5
	}
	return z.Dominance
}

func (z *ZoomRotate) handle(e pointer.Event) {
	if e.Kind != pointer.Cancel && e.Source != pointer.Touch && e.Source != pointer.Trackpad {
		return
	}
	if z.points == nil {
		z.points = make(map[pointer.ID]f32.Point)
	}
	switch e.Kind {
	case pointer.Press:
		z.points[e.PointerID] = e.Position
		z.source = e.Source
		if len(z.points) > 2 {
			if z.mode != zoomRotateNone {
				z.emit(pointer.GestureCancel, e)
			}
			z.mode = zoomRotateNone
			z.hasPair = false
			z.blocked = true
			return
		}
		if !z.blocked && !z.hasPair && len(z.points) == 2 {
			z.beginPair()
		}
	case pointer.Move, pointer.Drag:
		if z.blocked || !z.hasPair {
			return
		}
		if _, ok := z.points[e.PointerID]; !ok {
			return
		}
		z.points[e.PointerID] = e.Position
		if z.moved == nil {
			z.moved = make(map[pointer.ID]bool)
		}
		z.moved[e.PointerID] = true
		z.update(e)
	case pointer.Release, pointer.Cancel:
		if e.Kind == pointer.Cancel && e.PointerID == 0 {
			if z.hasPair && z.mode != zoomRotateNone {
				z.emit(pointer.GestureCancel, e)
			}
			z.resetPair()
			clear(z.points)
			z.blocked = false
			return
		}
		if z.blocked {
			delete(z.points, e.PointerID)
			if len(z.points) == 0 {
				z.blocked = false
			}
			return
		}
		if !z.hasPair {
			delete(z.points, e.PointerID)
			return
		}
		if e.PointerID != z.id1 && e.PointerID != z.id2 {
			delete(z.points, e.PointerID)
			return
		}
		if z.mode != zoomRotateNone {
			phase := pointer.GestureEnd
			if e.Kind == pointer.Cancel {
				phase = pointer.GestureCancel
			}
			z.emit(phase, e)
		}
		delete(z.points, e.PointerID)
		z.resetPair()
	}
}

func (z *ZoomRotate) beginPair() {
	// Two points exist; pick a stable pair. Pointer ID 0 is valid.
	first := true
	for id := range z.points {
		if first {
			z.id1 = id
			first = false
		} else {
			z.id2 = id
			break
		}
	}
	z.hasPair = true
	z.mode = zoomRotateNone
	z.startDist = z.distance()
	if z.startDist < 1 {
		z.startDist = 1
	}
	z.startAngle = z.angle()
	z.scale = 1
	z.rotation = 0
	z.moved = make(map[pointer.ID]bool)
	z.center = z.midpoint()
}

func (z *ZoomRotate) update(e pointer.Event) {
	scale := z.distance() / z.startDist
	rotation := z.angle() - z.startAngle
	for rotation > math.Pi {
		rotation -= 2 * math.Pi
	}
	for rotation < -math.Pi {
		rotation += 2 * math.Pi
	}
	z.scale = scale
	z.rotation = rotation
	z.center = z.midpoint()

	if z.mode == zoomRotateNone {
		// Wait until both fingers have moved. Evaluating after a single
		// pointer move makes a pure rotation look like a pinch or vice
		// versa, because the pair's distance and angle are not final yet.
		if len(z.moved) < 2 {
			return
		}
		zoom := abs32(scale - 1)
		rot := abs32(rotation)
		dom := z.dominance()
		switch {
		case zoom >= z.minScale() && zoom >= rot*dom:
			z.lock(zoomRotatePinch, e)
		case rot >= z.minRotation() && rot >= zoom*dom:
			z.lock(zoomRotateRotate, e)
		}
		return
	}
	z.emitUpdate(e)
}

func (z *ZoomRotate) lock(mode uint8, e pointer.Event) {
	z.mode = mode
	z.gestureID++
	ev := pointer.Event{
		GestureID: z.gestureID,
		Phase:     pointer.GestureBegin,
		Fingers:   2,
		Position:  z.center,
		Source:    z.source,
		Time:      e.Time,
	}
	if mode == zoomRotatePinch {
		ev.Kind = pointer.Pinch
		ev.Scale = z.scale
	} else {
		ev.Kind = pointer.Rotate
		ev.Rotation = z.rotation
	}
	z.pending = append(z.pending, ev)
}

func (z *ZoomRotate) emitUpdate(e pointer.Event) {
	ev := pointer.Event{
		GestureID: z.gestureID,
		Phase:     pointer.GestureUpdate,
		Fingers:   2,
		Position:  z.center,
		Source:    z.source,
		Time:      e.Time,
	}
	if z.mode == zoomRotatePinch {
		ev.Kind = pointer.Pinch
		ev.Scale = z.scale
	} else {
		ev.Kind = pointer.Rotate
		ev.Rotation = z.rotation
	}
	z.pending = append(z.pending, ev)
}

func (z *ZoomRotate) emit(phase pointer.GesturePhase, e pointer.Event) {
	ev := pointer.Event{
		GestureID: z.gestureID,
		Phase:     phase,
		Fingers:   2,
		Position:  z.center,
		Source:    z.source,
		Time:      e.Time,
	}
	if z.mode == zoomRotatePinch {
		ev.Kind = pointer.Pinch
		ev.Scale = z.scale
	} else {
		ev.Kind = pointer.Rotate
		ev.Rotation = z.rotation
	}
	z.pending = append(z.pending, ev)
}

func (z *ZoomRotate) resetPair() {
	z.hasPair = false
	z.mode = zoomRotateNone
	z.moved = nil
	z.id1, z.id2 = 0, 0
}

func (z *ZoomRotate) distance() float32 {
	a := z.points[z.id1]
	b := z.points[z.id2]
	dx, dy := a.X-b.X, a.Y-b.Y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

func (z *ZoomRotate) angle() float32 {
	a := z.points[z.id1]
	b := z.points[z.id2]
	return float32(math.Atan2(float64(a.Y-b.Y), float64(a.X-b.X)))
}

func (z *ZoomRotate) midpoint() f32.Point {
	a := z.points[z.id1]
	b := z.points[z.id2]
	return f32.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
}
