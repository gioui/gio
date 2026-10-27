// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"testing"

	"gioui.org/io/pointer"
)

func TestWindowsGestureState(t *testing.T) {
	var s windowsGestureState
	id, phase, scale := s.zoom(100, true, false)
	if id == 0 || phase != pointer.GestureBegin || scale != 1 {
		t.Fatalf("zoom begin = %d %v %v", id, phase, scale)
	}
	if _, phase, scale = s.zoom(200, false, false); phase != pointer.GestureUpdate || scale != 2 {
		t.Fatalf("zoom update = %v %v", phase, scale)
	}
	if _, phase, scale = s.zoom(200, false, true); phase != pointer.GestureEnd || scale != 2 {
		t.Fatalf("zoom end = %v %v", phase, scale)
	}
	if s.zoomID != 0 {
		t.Fatal("zoom state not reset")
	}

	var r windowsGestureState
	rid, phase, rotation := r.rotate(0, true, false)
	if rid == 0 || phase != pointer.GestureBegin || rotation != 0 {
		t.Fatalf("rotate begin = %d %v %v", rid, phase, rotation)
	}
	if _, phase, rotation = r.rotate(8192, false, false); phase != pointer.GestureUpdate || rotation < 1.5 || rotation > 1.6 {
		t.Fatalf("rotate update = %v %v", phase, rotation)
	}
	if _, phase, _ = r.rotate(0, false, true); phase != pointer.GestureEnd {
		t.Fatalf("rotate end = %v", phase)
	}
}
