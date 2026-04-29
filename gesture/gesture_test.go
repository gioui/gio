// SPDX-License-Identifier: Unlicense OR MIT

package gesture

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func TestHover(t *testing.T) {
	ops := new(op.Ops)
	var h Hover
	rect := image.Rect(20, 20, 40, 40)
	stack := clip.Rect(rect).Push(ops)
	h.Add(ops)
	stack.Pop()
	r := new(input.Router)
	h.Update(r.Source())
	r.Frame(ops)

	r.Queue(
		pointer.Event{Kind: pointer.Move, Position: f32.Pt(30, 30)},
	)
	if !h.Update(r.Source()) {
		t.Fatal("expected hovered")
	}

	r.Queue(
		pointer.Event{Kind: pointer.Move, Position: f32.Pt(50, 50)},
	)
	if h.Update(r.Source()) {
		t.Fatal("expected not hovered")
	}
}

func TestMouseClicks(t *testing.T) {
	for _, tc := range []struct {
		label  string
		events []event.Event
		clicks []int // number of combined clicks per click (single, double...)
	}{
		{
			label:  "single click",
			events: mouseClickEvents(200 * time.Millisecond),
			clicks: []int{1},
		},
		{
			label: "double click",
			events: mouseClickEvents(
				100*time.Millisecond,
				100*time.Millisecond+doubleClickDuration-1),
			clicks: []int{1, 2},
		},
		{
			label: "two single clicks",
			events: mouseClickEvents(
				100*time.Millisecond,
				100*time.Millisecond+doubleClickDuration+1),
			clicks: []int{1, 1},
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			var click Click
			var ops op.Ops
			click.Add(&ops)

			var r input.Router
			click.Update(r.Source())
			r.Frame(&ops)
			r.Queue(tc.events...)

			var clicks []ClickEvent
			for {
				ev, ok := click.Update(r.Source())
				if !ok {
					break
				}
				if ev.Kind == KindClick {
					clicks = append(clicks, ev)
				}
			}
			if got, want := len(clicks), len(tc.clicks); got != want {
				t.Fatalf("got %d mouse clicks, expected %d", got, want)
			}

			for i, click := range clicks {
				if got, want := click.NumClicks, tc.clicks[i]; got != want {
					t.Errorf("got %d combined mouse clicks, expected %d", got, want)
				}
			}
		})
	}
}

func TestClickPointerIDReassignment(t *testing.T) {
	// A Click must accept a Press from a PointerID that differs from the
	// one its hovered state was previously associated with. Some backends
	// reassign a single physical pointer's ID over its lifetime — e.g. the
	// Windows pointer API across focus changes — and locking the gesture
	// to the first observed ID would silently drop every subsequent press.
	//
	// The sequence below puts the gesture into the buggy state through
	// public events alone: a press under PointerID 1 starts an active
	// press cycle, a Move under PointerID 2 arrives mid-press (which the
	// router routes as an Enter for PID 2 but the gesture's Enter handler
	// is a no-op for pid while pressed), then PID 1 releases. After this,
	// the router has the gesture entered for PID 2 (so the next event
	// under PID 2 won't trigger another Enter) but the gesture itself
	// still has pid=1.
	var click Click
	var ops op.Ops
	rect := image.Rect(0, 0, 100, 100)
	stack := clip.Rect(rect).Push(&ops)
	click.Add(&ops)
	stack.Pop()

	var r input.Router
	click.Update(r.Source())
	r.Frame(&ops)

	drain := func() {
		for {
			if _, ok := click.Update(r.Source()); !ok {
				return
			}
		}
	}

	// Press under PointerID 1.
	r.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(50, 50), PointerID: 1},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(50, 50), PointerID: 1},
	)
	drain()

	// Move under PointerID 2 while PointerID 1 is still pressed. The
	// router records the gesture as entered for PointerID 2 but the
	// gesture's Enter handler is a no-op for pid because c.pressed.
	r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(50, 50), PointerID: 2})
	drain()

	// Release PointerID 1. PointerID 1's press tracking ends; the
	// gesture's recorded pid stays at 1.
	r.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(50, 50), PointerID: 1})
	drain()

	// Press under PointerID 2. The router won't refire Enter for PID 2
	// (the gesture is already in PID 2's entered set), so the gesture's
	// only chance to refresh its pid is the Press handler itself.
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(50, 50), PointerID: 2})

	var sawPress bool
	for {
		ev, ok := click.Update(r.Source())
		if !ok {
			break
		}
		if ev.Kind == KindPress {
			sawPress = true
		}
	}
	if !sawPress {
		t.Fatal("expected KindPress for press under reassigned PointerID; gesture dropped the press because of stale recorded pid")
	}
}

func mouseClickEvents(times ...time.Duration) []event.Event {
	press := pointer.Event{
		Kind:    pointer.Press,
		Source:  pointer.Mouse,
		Buttons: pointer.ButtonPrimary,
	}
	events := make([]event.Event, 0, 2*len(times))
	for _, t := range times {
		press := press
		press.Time = t
		release := press
		release.Kind = pointer.Release
		events = append(events, press, release)
	}
	return events
}
