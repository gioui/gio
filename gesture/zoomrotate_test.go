// SPDX-License-Identifier: Unlicense OR MIT

package gesture

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func drainZoomRotate(z *ZoomRotate, q input.Source) []pointer.Event {
	var out []pointer.Event
	for {
		e, ok := z.Update(q)
		if !ok {
			return out
		}
		out = append(out, e)
	}
}

func queueZoomRotate(r *input.Router, events ...event.Event) {
	r.Queue(events...)
}

func TestZoomRotatePinchExclusive(t *testing.T) {
	var ops op.Ops
	var r input.Router
	z := new(ZoomRotate)
	clipStack := clip.Rect(image.Rect(0, 0, 300, 300)).Push(&ops)
	z.Add(&ops)
	clipStack.Pop()
	r.Frame(&ops)
	z.Update(r.Source())

	queueZoomRotate(&r,
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(50, 0)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(150, 0)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(0, 0)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(200, 0)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(0, 0)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(200, 0)},
	)
	got := drainZoomRotate(z, r.Source())
	if len(got) != 2 {
		t.Fatalf("events = %+v, want Begin/End", got)
	}
	for _, e := range got {
		if e.Kind != pointer.Pinch {
			t.Fatalf("pure pinch emitted %v: %+v", e.Kind, got)
		}
	}
	if got[0].Phase != pointer.GestureBegin || got[1].Phase != pointer.GestureEnd {
		t.Fatalf("phases = %+v", got)
	}
	if got[0].Scale < 1.9 || got[0].Scale > 2.1 {
		t.Fatalf("scale = %v", got[0].Scale)
	}
}

func TestZoomRotateRotateExclusive(t *testing.T) {
	var ops op.Ops
	var r input.Router
	z := new(ZoomRotate)
	clipStack := clip.Rect(image.Rect(0, 0, 300, 300)).Push(&ops)
	z.Add(&ops)
	clipStack.Pop()
	r.Frame(&ops)
	z.Update(r.Source())

	queueZoomRotate(&r,
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(50, 0)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(150, 0)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(100, -50)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(100, 50)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(100, -50)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(100, 50)},
	)
	got := drainZoomRotate(z, r.Source())
	if len(got) != 2 {
		t.Fatalf("events = %+v, want Begin/End", got)
	}
	for _, e := range got {
		if e.Kind != pointer.Rotate {
			t.Fatalf("pure rotation emitted %v: %+v", e.Kind, got)
		}
	}
	if got[0].Phase != pointer.GestureBegin || got[1].Phase != pointer.GestureEnd {
		t.Fatalf("phases = %+v", got)
	}
	rot := got[0].Rotation
	if rot < 1.4 && rot > -1.4 {
		t.Fatalf("rotation = %v", rot)
	}
}
