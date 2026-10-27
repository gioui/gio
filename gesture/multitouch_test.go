// SPDX-License-Identifier: Unlicense OR MIT

package gesture

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func drainPinch(p *Pinch, q input.Source) []pointer.Event {
	var out []pointer.Event
	for {
		e, ok := p.Update(q)
		if !ok {
			return out
		}
		out = append(out, e)
	}
}

func drainRotate(r *Rotate, q input.Source) []pointer.Event {
	var out []pointer.Event
	for {
		e, ok := r.Update(q)
		if !ok {
			return out
		}
		out = append(out, e)
	}
}

func TestPinchFallback(t *testing.T) {
	var ops op.Ops
	var r input.Router
	p := new(Pinch)
	clipStack := clip.Rect(image.Rect(0, 0, 200, 200)).Push(&ops)
	p.Add(&ops)
	clipStack.Pop()
	r.Frame(&ops)
	p.Update(r.Source()) // register the filter before queuing

	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Trackpad, PointerID: 1, Position: f32.Pt(10, 10)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Trackpad, PointerID: 2, Position: f32.Pt(20, 10)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Trackpad, PointerID: 2, Position: f32.Pt(30, 10), Time: time.Second},
		pointer.Event{Kind: pointer.Release, Source: pointer.Trackpad, PointerID: 2, Position: f32.Pt(30, 10), Time: 2 * time.Second},
	)
	got := drainPinch(p, r.Source())
	if len(got) != 3 {
		t.Fatalf("events = %d, want 3: %+v", len(got), got)
	}
	begin, update, end := got[0], got[1], got[2]
	if begin.Kind != pointer.Pinch || begin.Phase != pointer.GestureBegin || begin.Scale != 1 || begin.Fingers != 2 {
		t.Fatalf("begin = %+v", begin)
	}
	if begin.Source != pointer.Trackpad || begin.GestureID == 0 {
		t.Fatalf("begin source/id = %+v", begin)
	}
	if update.Phase != pointer.GestureUpdate || update.Scale < 1.99 || update.Scale > 2.01 {
		t.Fatalf("update = %+v", update)
	}
	if end.Phase != pointer.GestureEnd || end.Scale < 1.99 || end.Scale > 2.01 {
		t.Fatalf("end = %+v", end)
	}
	if begin.GestureID != update.GestureID || begin.GestureID != end.GestureID {
		t.Fatalf("gesture IDs differ")
	}
}

func TestPinchFallbackCancel(t *testing.T) {
	var ops op.Ops
	var r input.Router
	p := new(Pinch)
	clipStack := clip.Rect(image.Rect(0, 0, 200, 200)).Push(&ops)
	p.Add(&ops)
	clipStack.Pop()
	r.Frame(&ops)
	p.Update(r.Source())
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(10, 10)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(20, 10)},
		pointer.Event{Kind: pointer.Cancel, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(10, 10)},
	)
	got := drainPinch(p, r.Source())
	if len(got) != 2 || got[1].Phase != pointer.GestureCancel {
		t.Fatalf("events = %+v", got)
	}
}

func TestRotateFallback(t *testing.T) {
	var ops op.Ops
	var r input.Router
	g := new(Rotate)
	clipStack := clip.Rect(image.Rect(0, 0, 200, 200)).Push(&ops)
	g.Add(&ops)
	clipStack.Pop()
	r.Frame(&ops)
	g.Update(r.Source())

	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(10, 10)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(20, 10)},
		// Rotate the pair 90 degrees around its midpoint.
		pointer.Event{Kind: pointer.Move, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(15, 20)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 2, Position: f32.Pt(15, 20)},
	)
	got := drainRotate(g, r.Source())
	if len(got) != 3 {
		t.Fatalf("events = %d, want 3: %+v", len(got), got)
	}
	if got[0].Phase != pointer.GestureBegin || got[0].Rotation != 0 {
		t.Fatalf("begin = %+v", got[0])
	}
	if got[1].Phase != pointer.GestureUpdate || got[1].Rotation == 0 {
		t.Fatalf("update = %+v", got[1])
	}
	if got[2].Phase != pointer.GestureEnd {
		t.Fatalf("end = %+v", got[2])
	}
	if got[0].GestureID != got[1].GestureID || got[0].GestureID != got[2].GestureID {
		t.Fatalf("gesture IDs differ")
	}
}

func TestPinchDoesNotConsumeMouse(t *testing.T) {
	var r input.Router
	p := new(Pinch)
	var ops op.Ops
	clipStack := clip.Rect(image.Rect(0, 0, 200, 200)).Push(&ops)
	p.Add(&ops)
	clipStack.Pop()
	r.Frame(&ops)
	p.Update(r.Source())
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, PointerID: 1, Position: f32.Pt(10, 10)})
	if got := drainPinch(p, r.Source()); len(got) != 0 {
		t.Fatalf("mouse event produced a pinch: %+v", got)
	}
}

func drainSwipe(s *Swipe, q input.Source) []pointer.Event {
	var out []pointer.Event
	for {
		e, ok := s.Update(q)
		if !ok {
			return out
		}
		out = append(out, e)
	}
}

func TestSwipeFallback(t *testing.T) {
	var r input.Router
	s := new(Swipe)
	var ops op.Ops
	clipStack := clip.Rect(image.Rect(0, 0, 200, 200)).Push(&ops)
	s.Add(&ops)
	clipStack.Pop()
	r.Frame(&ops)
	s.Update(r.Source())

	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Trackpad, PointerID: 1, Position: f32.Pt(10, 10)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Trackpad, PointerID: 2, Position: f32.Pt(20, 10)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Trackpad, PointerID: 3, Position: f32.Pt(30, 10)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Trackpad, PointerID: 3, Position: f32.Pt(50, 10)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Trackpad, PointerID: 3, Position: f32.Pt(50, 10)},
	)
	got := drainSwipe(s, r.Source())
	if len(got) != 3 {
		t.Fatalf("events = %d, want 3: %+v", len(got), got)
	}
	begin, update, end := got[0], got[1], got[2]
	if begin.Kind != pointer.Swipe || begin.Phase != pointer.GestureBegin || begin.Fingers != 3 {
		t.Fatalf("begin = %+v", begin)
	}
	if update.Phase != pointer.GestureUpdate || update.Swipe.X == 0 && update.Swipe.Y == 0 {
		t.Fatalf("update = %+v", update)
	}
	if end.Phase != pointer.GestureEnd {
		t.Fatalf("end = %+v", end)
	}
}

func drainHold(h *Hold, q input.Source, now time.Time) []pointer.Event {
	var out []pointer.Event
	for {
		e, ok := h.Update(q, now)
		if !ok {
			return out
		}
		out = append(out, e)
	}
}

func TestHoldFallback(t *testing.T) {
	var r input.Router
	h := new(Hold)
	var ops op.Ops
	clipStack := clip.Rect(image.Rect(0, 0, 200, 200)).Push(&ops)
	h.Add(&ops)
	clipStack.Pop()
	r.Frame(&ops)
	start := time.Unix(0, 0)
	h.Update(r.Source(), start)
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(10, 10)})
	if got := drainHold(h, r.Source(), start); len(got) != 0 {
		t.Fatalf("early hold = %+v", got)
	}
	if got := drainHold(h, r.Source(), start.Add(100*time.Millisecond)); len(got) != 0 {
		t.Fatalf("hold before duration = %+v", got)
	}
	got := drainHold(h, r.Source(), start.Add(600*time.Millisecond))
	if len(got) != 1 || got[0].Phase != pointer.GestureBegin || got[0].Fingers != 1 {
		t.Fatalf("hold begin = %+v", got)
	}
	r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(10, 10)})
	got = drainHold(h, r.Source(), start.Add(700*time.Millisecond))
	if len(got) != 1 || got[0].Phase != pointer.GestureEnd {
		t.Fatalf("hold end = %+v", got)
	}
}

func TestHoldCancelledByMovement(t *testing.T) {
	var r input.Router
	h := new(Hold)
	var ops op.Ops
	clipStack := clip.Rect(image.Rect(0, 0, 200, 200)).Push(&ops)
	h.Add(&ops)
	clipStack.Pop()
	r.Frame(&ops)
	start := time.Unix(0, 0)
	h.Update(r.Source(), start)
	r.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(10, 10)},
		pointer.Event{Kind: pointer.Move, Source: pointer.Touch, PointerID: 1, Position: f32.Pt(60, 60)},
	)
	if got := drainHold(h, r.Source(), start.Add(100*time.Millisecond)); len(got) != 0 {
		t.Fatalf("movement produced hold = %+v", got)
	}
	if got := drainHold(h, r.Source(), start.Add(600*time.Millisecond)); len(got) != 0 {
		t.Fatalf("cancelled hold began = %+v", got)
	}
}
