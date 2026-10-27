// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"math"

	"gioui.org/io/pointer"
)

// windowsGestureState converts Windows gesture arguments into semantic
// pointer events. It is platform-independent so it can be tested on host.
type windowsGestureState struct {
	nextID uint32
	zoomID uint32
	base   uint64
	scale  float32
	rotID  uint32
	rot    float32
}

func (s *windowsGestureState) zoom(args uint64, begin, end bool) (uint32, pointer.GesturePhase, float32) {
	if begin || s.zoomID == 0 {
		s.nextID++
		s.zoomID = s.nextID
		s.base = args
		if s.base == 0 {
			s.base = 1
		}
		s.scale = 1
	} else {
		s.scale = float32(float64(args) / float64(s.base))
	}
	id, scale := s.zoomID, s.scale
	phase := pointer.GestureUpdate
	if begin {
		phase = pointer.GestureBegin
	}
	if end {
		phase = pointer.GestureEnd
		s.zoomID = 0
	}
	return id, phase, scale
}

func (s *windowsGestureState) rotate(args uint64, begin, end bool) (uint32, pointer.GesturePhase, float32) {
	if begin || s.rotID == 0 {
		s.nextID++
		s.rotID = s.nextID
		s.rot = 0
	} else {
		// GID_ROTATE uses 32768 units per full circle.
		s.rot = float32(float64(int64(args)) / 32768 * 2 * math.Pi)
	}
	id, rotation := s.rotID, s.rot
	phase := pointer.GestureUpdate
	if begin {
		phase = pointer.GestureBegin
	}
	if end {
		phase = pointer.GestureEnd
		s.rotID = 0
	}
	return id, phase, rotation
}
