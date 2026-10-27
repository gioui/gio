// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"math"

	"gioui.org/f32"
	"gioui.org/io/pointer"
)

// waylandGestureState converts the zwp_pointer_gestures_v1 payloads into
// semantic pointer events. It is deliberately free of Wayland/C types so the
// phase, accumulation and finger semantics can be unit tested on any host.
type waylandGestureState struct {
	id       uint32
	kind     pointer.Kind
	fingers  uint8
	swipe    f32.Point
	rotation float32
	scale    float32
}

func (s *waylandGestureState) begin(kind pointer.Kind, fingers uint32) pointer.Event {
	s.id++
	if fingers > 255 {
		fingers = 255
	}
	s.kind = kind
	s.fingers = uint8(fingers)
	s.swipe = f32.Point{}
	s.rotation = 0
	s.scale = 1
	return pointer.Event{
		Kind:      kind,
		Source:    pointer.Trackpad,
		GestureID: s.id,
		Phase:     pointer.GestureBegin,
		Fingers:   s.fingers,
	}
}

func (s *waylandGestureState) pinchUpdate(scale, rotationDeg, dx, dy float32) pointer.Event {
	s.rotation += rotationDeg * math.Pi / 180
	s.scale = scale
	return pointer.Event{
		Kind:      pointer.Pinch,
		Source:    pointer.Trackpad,
		GestureID: s.id,
		Phase:     pointer.GestureUpdate,
		Scale:     scale,
		Rotation:  s.rotation,
		Swipe:     f32.Pt(dx, dy),
		Fingers:   s.fingers,
	}
}

func (s *waylandGestureState) swipeUpdate(dx, dy float32) pointer.Event {
	s.swipe = s.swipe.Add(f32.Pt(dx, dy))
	return pointer.Event{
		Kind:      pointer.Swipe,
		Source:    pointer.Trackpad,
		GestureID: s.id,
		Phase:     pointer.GestureUpdate,
		Swipe:     s.swipe,
		Fingers:   s.fingers,
	}
}

func (s *waylandGestureState) end(cancelled bool) pointer.Event {
	phase := pointer.GestureEnd
	if cancelled {
		phase = pointer.GestureCancel
	}
	ev := pointer.Event{
		Kind:      s.kind,
		Source:    pointer.Trackpad,
		GestureID: s.id,
		Phase:     phase,
		Fingers:   s.fingers,
	}
	if s.kind == pointer.Pinch {
		ev.Rotation = s.rotation
		ev.Scale = s.scale
	} else if s.kind == pointer.Swipe {
		ev.Swipe = s.swipe
	}
	return ev
}
