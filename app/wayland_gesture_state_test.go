// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"math"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/pointer"
)

func TestWaylandGestureState(t *testing.T) {
	var s waylandGestureState
	begin := s.begin(pointer.Pinch, 2)
	if begin.Kind != pointer.Pinch || begin.Phase != pointer.GestureBegin || begin.Fingers != 2 || begin.GestureID == 0 {
		t.Fatalf("begin = %+v", begin)
	}
	if begin.Source != pointer.Trackpad {
		t.Fatalf("source = %v", begin.Source)
	}
	up := s.pinchUpdate(1.5, 45, 3, 4)
	if up.Phase != pointer.GestureUpdate || up.Scale != 1.5 || up.Rotation != float32(math.Pi/4) || up.Swipe != f32.Pt(3, 4) {
		t.Fatalf("pinch update = %+v", up)
	}
	up2 := s.pinchUpdate(2, -45, 0, 0)
	if up2.Rotation != 0 {
		t.Fatalf("rotation should cancel, got %v", up2.Rotation)
	}
	end := s.end(false)
	if end.Phase != pointer.GestureEnd || end.Kind != pointer.Pinch || end.GestureID != up2.GestureID {
		t.Fatalf("end = %+v", end)
	}

	var w waylandGestureState
	w.begin(pointer.Swipe, 3)
	s1 := w.swipeUpdate(10, 20)
	s2 := w.swipeUpdate(5, -2)
	if s1.Swipe != f32.Pt(10, 20) || s2.Swipe != f32.Pt(15, 18) {
		t.Fatalf("swipe accumulation = %v %v", s1.Swipe, s2.Swipe)
	}
	wc := w.end(true)
	if wc.Phase != pointer.GestureCancel || wc.Swipe != f32.Pt(15, 18) || wc.Fingers != 3 {
		t.Fatalf("swipe cancel = %+v", wc)
	}
}
