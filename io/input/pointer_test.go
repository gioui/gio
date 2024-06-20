// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"fmt"
	"image"
	"reflect"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/io/transfer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func TestFilterReset(t *testing.T) {
	r := new(Router)
	if _, ok := r.Event(pointer.Filter{}); ok {
		t.Fatal("empty filter matched reset event")
	}
	if _, ok := r.Event(pointer.Filter{Kinds: pointer.Cancel}); ok {
		t.Fatal("second call to Event matched reset event")
	}
}

func TestPointerNilTarget(t *testing.T) {
	r := new(Router)
	r.Event(pointer.Filter{Kinds: pointer.Press})
	r.Frame(new(op.Ops))
	r.Queue(pointer.Event{Kind: pointer.Press})
	// Nil Targets should not receive events.
	if _, ok := r.Event(pointer.Filter{Kinds: pointer.Press}); ok {
		t.Errorf("nil target received event")
	}
}

func TestPointerWakeup(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	var r Router
	addPointerHandler(&r, &ops, handler, image.Rect(0, 0, 100, 100))

	// Test that merely adding a handler doesn't trigger redraw.
	r.Frame(&ops)
	if _, wake := r.WakeupTime(); wake {
		t.Errorf("adding pointer.InputOp triggered a redraw")
	}
}

func TestPointerDrag(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	var r Router
	f := addPointerHandler(&r, &ops, handler, image.Rect(0, 0, 100, 100))

	r.Frame(&ops)
	r.Queue(
		// Press.
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
		// Move outside the area.
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(150, 150),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Enter, pointer.Press, pointer.Leave, pointer.Drag)
}

func TestPointerDragNegative(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	var r Router
	f := addPointerHandler(&r, &ops, handler, image.Rect(-100, -100, 0, 0))

	r.Frame(&ops)
	r.Queue(
		// Press.
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(-50, -50),
		},
		// Move outside the area.
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(-150, -150),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Enter, pointer.Press, pointer.Leave, pointer.Drag)
}

func TestPointerGrab(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	handler3 := new(int)
	var ops op.Ops

	filter := func(t event.Tag) event.Filter {
		return pointer.Filter{Target: t, Kinds: pointer.Press | pointer.Release | pointer.Cancel}
	}

	event.Op(&ops, handler1)
	event.Op(&ops, handler2)
	event.Op(&ops, handler3)

	var r Router
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler3)), pointer.Cancel)
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, 1, filter(handler1)), pointer.Press)
	assertEventPointerTypeSequence(t, events(&r, 1, filter(handler2)), pointer.Press)
	assertEventPointerTypeSequence(t, events(&r, 1, filter(handler3)), pointer.Press)
	r.Source().Execute(pointer.GrabCmd{Tag: handler1})
	r.Queue(
		pointer.Event{
			Kind:     pointer.Release,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Release)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler3)), pointer.Cancel)
}

func TestPointerGrabSameHandlerTwice(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	filter := func(t event.Tag) event.Filter {
		return pointer.Filter{Target: t, Kinds: pointer.Press | pointer.Release | pointer.Cancel}
	}

	event.Op(&ops, handler1)
	event.Op(&ops, handler1)
	event.Op(&ops, handler2)

	var r Router
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Cancel)
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, 1, filter(handler1)), pointer.Press)
	assertEventPointerTypeSequence(t, events(&r, 1, filter(handler2)), pointer.Press)
	r.Source().Execute(pointer.GrabCmd{Tag: handler1})
	r.Queue(
		pointer.Event{
			Kind:     pointer.Release,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Release)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Cancel)
}

func TestPointerMove(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	filter := func(t event.Tag) event.Filter {
		return pointer.Filter{
			Target: t,
			Kinds:  pointer.Move | pointer.Enter | pointer.Leave | pointer.Cancel,
		}
	}

	// Handler 1 area: (0, 0) - (100, 100)
	r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, handler1)
	// Handler 2 area: (50, 50) - (100, 100) (areas intersect).
	r2 := clip.Rect(image.Rect(50, 50, 200, 200)).Push(&ops)
	event.Op(&ops, handler2)
	r2.Pop()
	r1.Pop()

	var r Router
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Cancel)
	r.Frame(&ops)
	r.Queue(
		// Hit both handlers.
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
		// Hit handler 1.
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(49, 50),
		},
		// Hit no handlers.
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(100, 50),
		},
		pointer.Event{
			Kind: pointer.Cancel,
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Enter, pointer.Move, pointer.Move, pointer.Leave, pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Enter, pointer.Move, pointer.Leave, pointer.Cancel)
}

func TestPointerTypes(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	f := pointer.Filter{
		Target: handler,
		Kinds:  pointer.Press | pointer.Release | pointer.Cancel,
	}
	event.Op(&ops, handler)
	r1.Pop()

	var r Router
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Cancel)
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(150, 150),
		},
		pointer.Event{
			Kind:     pointer.Release,
			Position: f32.Pt(150, 150),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Press, pointer.Release)
}

func TestPointerSystemAction(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		var ops op.Ops
		r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
		system.ActionInputOp(system.ActionMove).Add(&ops)
		r1.Pop()

		var r Router
		r.Frame(&ops)
		assertActionAt(t, r, f32.Pt(50, 50), system.ActionMove)
	})
	t.Run("covered by another clip", func(t *testing.T) {
		var ops op.Ops
		r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
		system.ActionInputOp(system.ActionMove).Add(&ops)
		clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops).Pop()
		r1.Pop()

		var r Router
		r.Frame(&ops)
		assertActionAt(t, r, f32.Pt(50, 50), system.ActionMove)
	})
	t.Run("uses topmost action op", func(t *testing.T) {
		var ops op.Ops
		r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
		system.ActionInputOp(system.ActionMove).Add(&ops)
		r2 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
		system.ActionInputOp(system.ActionClose).Add(&ops)
		r2.Pop()
		r1.Pop()

		var r Router
		r.Frame(&ops)
		assertActionAt(t, r, f32.Pt(50, 50), system.ActionClose)
	})
}

func TestPointerPriority(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	handler3 := new(int)
	var ops op.Ops
	var r Router

	r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	f1 := func(t event.Tag) event.Filter {
		return pointer.Filter{
			Target:  t,
			Kinds:   pointer.Scroll,
			ScrollX: pointer.ScrollRange{Max: 100},
		}
	}
	events(&r, -1, f1(handler1))
	event.Op(&ops, handler1)

	r2 := clip.Rect(image.Rect(0, 0, 100, 50)).Push(&ops)
	f2 := func(t event.Tag) event.Filter {
		return pointer.Filter{
			Target:  t,
			Kinds:   pointer.Scroll,
			ScrollX: pointer.ScrollRange{Max: 20},
		}
	}
	events(&r, -1, f2(handler2))
	event.Op(&ops, handler2)
	r2.Pop()
	r1.Pop()

	r3 := clip.Rect(image.Rect(0, 100, 100, 200)).Push(&ops)
	f3 := func(t event.Tag) event.Filter {
		return pointer.Filter{
			Target:  t,
			Kinds:   pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -20},
			ScrollY: pointer.ScrollRange{Min: -40},
		}
	}
	events(&r, -1, f3(handler3))
	event.Op(&ops, handler3)
	r3.Pop()

	r.Frame(&ops)
	r.Queue(
		// Hit handler 1 and 2.
		pointer.Event{
			Kind:     pointer.Scroll,
			Position: f32.Pt(50, 25),
			Scroll:   f32.Pt(50, 0),
		},
		// Hit handler 1.
		pointer.Event{
			Kind:     pointer.Scroll,
			Position: f32.Pt(50, 75),
			Scroll:   f32.Pt(50, 50),
		},
		// Hit handler 3.
		pointer.Event{
			Kind:     pointer.Scroll,
			Position: f32.Pt(50, 150),
			Scroll:   f32.Pt(-30, -30),
		},
		// Hit no handlers.
		pointer.Event{
			Kind:     pointer.Scroll,
			Position: f32.Pt(50, 225),
		},
	)

	hev1 := events(&r, -1, f1(handler1))
	hev2 := events(&r, -1, f2(handler2))
	hev3 := events(&r, -1, f3(handler3))
	assertEventPointerTypeSequence(t, hev1, pointer.Scroll, pointer.Scroll)
	assertEventPointerTypeSequence(t, hev2, pointer.Scroll)
	assertEventPointerTypeSequence(t, hev3, pointer.Scroll)
	assertEventPriorities(t, hev1, pointer.Shared, pointer.Foremost)
	assertEventPriorities(t, hev2, pointer.Foremost)
	assertEventPriorities(t, hev3, pointer.Foremost)
	assertScrollEvent(t, hev1[0], f32.Pt(30, 0))
	assertScrollEvent(t, hev2[0], f32.Pt(20, 0))
	assertScrollEvent(t, hev1[1], f32.Pt(50, 0))
	assertScrollEvent(t, hev3[0], f32.Pt(-20, -30))
}

func TestPointerEnterLeave(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops
	var r Router

	// Handler 1 area: (0, 0) - (100, 100)
	f1 := addPointerHandler(&r, &ops, handler1, image.Rect(0, 0, 100, 100))

	// Handler 2 area: (50, 50) - (200, 200) (areas overlap).
	f2 := addPointerHandler(&r, &ops, handler2, image.Rect(50, 50, 200, 200))

	r.Frame(&ops)
	// Hit both handlers.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
	)
	// First event for a handler is always a Cancel.
	// Only handler2 should receive the enter/move events because it is on top
	// and handler1 is not an ancestor in the hit tree.
	assertEventPointerTypeSequence(t, events(&r, -1, f1))
	assertEventPointerTypeSequence(t, events(&r, -1, f2), pointer.Enter, pointer.Move)

	// Leave the second area by moving into the first.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(45, 45),
		},
	)
	// The cursor leaves handler2 and enters handler1.
	assertEventPointerTypeSequence(t, events(&r, -1, f1), pointer.Enter, pointer.Move)
	assertEventPointerTypeSequence(t, events(&r, -1, f2), pointer.Leave)

	// Move, but stay within the same hit area.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(40, 40),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f1), pointer.Move)
	assertEventPointerTypeSequence(t, events(&r, -1, f2))

	// Move outside of both inputs.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(300, 300),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f1), pointer.Leave)
	assertEventPointerTypeSequence(t, events(&r, -1, f2))

	// Check that a Press event generates Enter Events.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(125, 125),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f1))
	assertEventPointerTypeSequence(t, events(&r, -1, f2), pointer.Enter, pointer.Press)

	// Check that a drag only affects the participating handlers.
	r.Queue(
		// Leave
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
		// Enter
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f1))
	assertEventPointerTypeSequence(t, events(&r, -1, f2), pointer.Leave, pointer.Drag, pointer.Enter, pointer.Drag)

	// Check that a Release event generates Enter/Leave Events.
	r.Queue(
		pointer.Event{
			Kind: pointer.Release,
			Position: f32.Pt(25,
				25),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f1), pointer.Enter)
	// The second handler gets the release event because the press started inside it.
	assertEventPointerTypeSequence(t, events(&r, -1, f2), pointer.Release, pointer.Leave)
}

func TestMultipleAreas(t *testing.T) {
	handler := new(int)

	var ops op.Ops
	var r Router

	f := addPointerHandler(&r, &ops, handler, image.Rect(0, 0, 100, 100))
	r1 := clip.Rect(image.Rect(50, 50, 200, 200)).Push(&ops)
	// Test that declaring a handler twice doesn't affect event handling.
	event.Op(&ops, handler)
	r1.Pop()

	assertEventPointerTypeSequence(t, events(&r, -1, f))
	r.Frame(&ops)
	// Hit first area, then second area, then both.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(150, 150),
		},
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Enter, pointer.Move, pointer.Move, pointer.Move)
}

func TestPointerEnterLeaveNested(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	filter := func(t event.Tag) event.Filter {
		return pointer.Filter{
			Target: t,
			Kinds:  pointer.Press | pointer.Move | pointer.Release | pointer.Enter | pointer.Leave | pointer.Cancel,
		}
	}

	// Handler 1 area: (0, 0) - (100, 100)
	r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, handler1)

	// Handler 2 area: (25, 25) - (75, 75) (nested within first).
	r2 := clip.Rect(image.Rect(25, 25, 75, 75)).Push(&ops)
	event.Op(&ops, handler2)
	r2.Pop()
	r1.Pop()

	var r Router
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Cancel)
	r.Frame(&ops)
	// Hit both handlers.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
	)
	// First event for a handler is always a Cancel.
	// Both handlers should receive the Enter and Move events because handler2 is a child of handler1.
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Enter, pointer.Move)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Enter, pointer.Move)

	// Leave the second area by moving into the first.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(20, 20),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Move)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Leave)

	// Move, but stay within the same hit area.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(10, 10),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Move)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)))

	// Move outside of both inputs.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(200, 200),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Leave)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)))

	// Check that a Press event generates Enter Events.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Enter, pointer.Press)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Enter, pointer.Press)

	// Check that a Release event generates Enter/Leave Events.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Release,
			Position: f32.Pt(20, 20),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler1)), pointer.Release)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(handler2)), pointer.Release, pointer.Leave)
}

func TestPointerActiveInputDisappears(t *testing.T) {
	handler1 := new(int)
	var ops op.Ops
	var r Router

	// Draw handler.
	ops.Reset()
	f := addPointerHandler(&r, &ops, handler1, image.Rect(0, 0, 100, 100))
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Enter, pointer.Move)
	r.Frame(&ops)

	// Re-render with handler missing.
	ops.Reset()
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Cancel)
}

func TestMultitouch(t *testing.T) {
	var ops op.Ops
	var r Router

	// Add two separate handlers.
	h1, h2 := new(int), new(int)
	f1 := addPointerHandler(&r, &ops, h1, image.Rect(0, 0, 100, 100))
	f2 := addPointerHandler(&r, &ops, h2, image.Rect(0, 100, 100, 200))

	h1pt, h2pt := f32.Pt(0, 0), f32.Pt(0, 100)
	var p1, p2 pointer.ID = 0, 1

	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:      pointer.Press,
			Position:  h1pt,
			PointerID: p1,
		},
	)
	r.Queue(
		pointer.Event{
			Kind:      pointer.Press,
			Position:  h2pt,
			PointerID: p2,
		},
	)
	r.Queue(
		pointer.Event{
			Kind:      pointer.Release,
			Position:  h2pt,
			PointerID: p2,
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f1), pointer.Enter, pointer.Press)
	assertEventPointerTypeSequence(t, events(&r, -1, f2), pointer.Enter, pointer.Press, pointer.Release)
}

func TestCursor(t *testing.T) {
	_at := func(x, y float32) []event.Event {
		return []event.Event{pointer.Event{
			Kind:     pointer.Move,
			Source:   pointer.Mouse,
			Buttons:  pointer.ButtonPrimary,
			Position: f32.Pt(x, y),
		}}
	}
	ops := new(op.Ops)
	var r Router
	for _, tc := range []struct {
		label   string
		events  []event.Event
		cursors []pointer.Cursor
		want    pointer.Cursor
	}{
		{label: "no movement",
			cursors: []pointer.Cursor{pointer.CursorPointer},
			want:    pointer.CursorDefault,
		},
		{label: "move inside",
			cursors: []pointer.Cursor{pointer.CursorPointer},
			events:  _at(50, 50),
			want:    pointer.CursorPointer,
		},
		{label: "move outside",
			cursors: []pointer.Cursor{pointer.CursorPointer},
			events:  _at(200, 200),
			want:    pointer.CursorDefault,
		},
		{label: "move back inside",
			cursors: []pointer.Cursor{pointer.CursorPointer},
			events:  _at(50, 50),
			want:    pointer.CursorPointer,
		},
		{label: "send key events while inside",
			cursors: []pointer.Cursor{pointer.CursorPointer},
			events: []event.Event{
				key.Event{Name: "A", State: key.Press},
				key.Event{Name: "A", State: key.Release},
			},
			want: pointer.CursorPointer,
		},
		{label: "send key events while outside",
			cursors: []pointer.Cursor{pointer.CursorPointer},
			events: append(
				_at(200, 200),
				key.Event{Name: "A", State: key.Press},
				key.Event{Name: "A", State: key.Release},
			),
			want: pointer.CursorDefault,
		},
		{label: "add new input on top while inside",
			cursors: []pointer.Cursor{pointer.CursorPointer, pointer.CursorCrosshair},
			events: append(
				_at(50, 50),
				key.Event{
					Name:  "A",
					State: key.Press,
				},
			),
			want: pointer.CursorCrosshair,
		},
		{label: "remove input on top while inside",
			cursors: []pointer.Cursor{pointer.CursorPointer},
			events: append(
				_at(50, 50),
				key.Event{
					Name:  "A",
					State: key.Press,
				},
			),
			want: pointer.CursorPointer,
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			ops.Reset()
			defer clip.Rect(image.Rectangle{Max: image.Pt(100, 100)}).Push(ops).Pop()
			for _, c := range tc.cursors {
				c.Add(ops)
			}
			r.Frame(ops)
			r.Queue(tc.events...)
			// The cursor should now have been changed if the mouse moved over the declared area.
			if got, want := r.Cursor(), tc.want; got != want {
				t.Errorf("got %q; want %q", got, want)
			}
		})
	}
}

func TestPassOp(t *testing.T) {
	var ops op.Ops

	h1, h2, h3, h4 := new(int), new(int), new(int), new(int)
	area := clip.Rect(image.Rect(0, 0, 100, 100))
	root := area.Push(&ops)
	event.Op(&ops, &h1)
	event.Op(&ops, h1)
	child1 := area.Push(&ops)
	event.Op(&ops, h2)
	child1.Pop()
	child2 := area.Push(&ops)
	pass := pointer.PassOp{}.Push(&ops)
	event.Op(&ops, h3)
	event.Op(&ops, h4)
	pass.Pop()
	child2.Pop()
	root.Pop()

	var r Router
	filter := func(t event.Tag) event.Filter {
		return pointer.Filter{Target: t, Kinds: pointer.Press | pointer.Cancel}
	}
	assertEventPointerTypeSequence(t, events(&r, -1, filter(h1)), pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(h2)), pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(h3)), pointer.Cancel)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(h4)), pointer.Cancel)
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind: pointer.Press,
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(h1)), pointer.Press)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(h2)), pointer.Press)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(h3)), pointer.Press)
	assertEventPointerTypeSequence(t, events(&r, -1, filter(h4)), pointer.Press)
}

func TestAreaPassthrough(t *testing.T) {
	var ops op.Ops

	h := new(int)
	event.Op(&ops, h)
	clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops).Pop()
	var r Router
	f := pointer.Filter{
		Target: h,
		Kinds:  pointer.Press | pointer.Cancel,
	}
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Cancel)
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind: pointer.Press,
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Press)
}

func TestEllipse(t *testing.T) {
	var ops op.Ops

	h := new(int)
	cl := clip.Ellipse(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, h)
	cl.Pop()
	var r Router
	f := pointer.Filter{
		Target: h,
		Kinds:  pointer.Press | pointer.Cancel,
	}
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Cancel)
	r.Frame(&ops)
	r.Queue(
		// Outside ellipse.
		pointer.Event{
			Position: f32.Pt(10, 10),
			Kind:     pointer.Press,
		},
		pointer.Event{
			Kind: pointer.Release,
		},
		// Inside ellipse.
		pointer.Event{
			Position: f32.Pt(50, 50),
			Kind:     pointer.Press,
		},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Press)
}

func TestTransfer(t *testing.T) {
	srcArea := image.Rect(0, 0, 20, 20)
	tgtArea := srcArea.Add(image.Pt(40, 0))
	setup := func(r *Router, ops *op.Ops, srcType, tgtType string) (src, tgt event.Tag) {
		src, tgt = new(int), new(int)
		events(r, -1, transfer.SourceFilter{Target: src, Type: srcType})
		events(r, -1, transfer.TargetFilter{Target: tgt, Type: tgtType})

		srcStack := clip.Rect(srcArea).Push(ops)
		event.Op(ops, src)
		srcStack.Pop()

		tgt1Stack := clip.Rect(tgtArea).Push(ops)
		event.Op(ops, tgt)
		tgt1Stack.Pop()

		return src, tgt
	}

	t.Run("drop on no target", func(t *testing.T) {
		ops := new(op.Ops)
		var r Router
		src, tgt := setup(&r, ops, "file", "file")
		r.Frame(ops)
		// Initiate a drag.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Press,
			},
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Move,
			},
		)
		assertEventSequence(t, events(&r, -1, transfer.SourceFilter{Target: src, Type: "file"}))
		assertEventSequence(t, events(&r, -1, transfer.TargetFilter{Target: tgt, Type: "file"}), transfer.InitiateEvent{})

		// Drop.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(30, 10),
				Kind:     pointer.Move,
			},
			pointer.Event{
				Position: f32.Pt(30, 10),
				Kind:     pointer.Release,
			},
		)
		assertEventSequence(t, events(&r, -1, transfer.SourceFilter{Target: src, Type: "file"}), transfer.CancelEvent{})
		assertEventSequence(t, events(&r, -1, transfer.TargetFilter{Target: tgt, Type: "file"}), transfer.CancelEvent{})
	})

	t.Run("drag with valid and invalid targets", func(t *testing.T) {
		ops := new(op.Ops)
		var r Router
		src, tgt1 := setup(&r, ops, "file", "file")
		tgt2 := new(int)
		events(&r, -1, transfer.TargetFilter{Target: tgt2, Type: "nofile"})
		stack := clip.Rect(tgtArea).Push(ops)
		event.Op(ops, tgt2)
		stack.Pop()
		r.Frame(ops)
		// Initiate a drag.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Press,
			},
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Move,
			},
		)
		assertEventSequence(t, events(&r, -1, transfer.SourceFilter{Target: src, Type: "file"}))
		assertEventSequence(t, events(&r, -1, transfer.TargetFilter{Target: tgt1, Type: "file"}), transfer.InitiateEvent{})
		assertEventSequence(t, events(&r, -1, transfer.TargetFilter{Target: tgt2, Type: "nofile"}))
	})

	t.Run("drop on invalid target", func(t *testing.T) {
		ops := new(op.Ops)
		var r Router
		src, tgt := setup(&r, ops, "file", "nofile")
		r.Frame(ops)
		// Drag.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Press,
			},
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Move,
			},
		)
		assertEventSequence(t, events(&r, -1, transfer.SourceFilter{Target: src, Type: "file"}))
		assertEventSequence(t, events(&r, -1, transfer.TargetFilter{Target: tgt, Type: "nofile"}))

		// Drop.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(40, 10),
				Kind:     pointer.Release,
			},
		)
		assertEventSequence(t, events(&r, -1, transfer.SourceFilter{Target: src, Type: "file"}), transfer.CancelEvent{})
		assertEventSequence(t, events(&r, -1, transfer.TargetFilter{Target: tgt, Type: "nofile"}))
	})

	t.Run("drop on valid target", func(t *testing.T) {
		ops := new(op.Ops)
		var r Router
		src, tgt := setup(&r, ops, "file", "file")
		// Make the target also a source. This should have no effect.
		events(&r, -1, transfer.SourceFilter{Target: tgt, Type: "file"})
		r.Frame(ops)
		// Drag.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Press,
			},
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Move,
			},
		)
		assertEventSequence(t, events(&r, 1, transfer.TargetFilter{Target: tgt, Type: "file"}), transfer.InitiateEvent{})

		// Drop.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(40, 10),
				Kind:     pointer.Release,
			},
		)
		assertEventSequence(t, events(&r, 1, transfer.SourceFilter{Target: src, Type: "file"}), transfer.RequestEvent{Type: "file"})

		// Offer valid type and data.
		ofr := &offer{data: "hello"}
		r.Source().Execute(transfer.OfferCmd{Tag: src, Type: "file", Data: ofr})
		assertEventSequence(t, events(&r, -1, transfer.SourceFilter{Target: src, Type: "file"}), transfer.CancelEvent{})
		evs := events(&r, -1, transfer.TargetFilter{Target: tgt, Type: "file"})
		if len(evs) != 2 {
			t.Fatalf("unexpected number of events: %d, want 2", len(evs))
		}
		assertEventSequence(t, evs[1:], transfer.CancelEvent{})
		dataEvent, ok := evs[0].(transfer.DataEvent)
		if !ok {
			t.Fatalf("unexpected event type: %T, want %T", dataEvent, transfer.DataEvent{})
		}
		if got, want := dataEvent.Type, "file"; got != want {
			t.Fatalf("got %s; want %s", got, want)
		}
		if got, want := dataEvent.Open(), ofr; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}

		// Drag and drop complete.
		if ofr.closed {
			t.Error("offer closed prematurely")
		}
	})

	t.Run("drop on valid target, DataEvent not used", func(t *testing.T) {
		ops := new(op.Ops)
		var r Router
		src, tgt := setup(&r, ops, "file", "file")
		// Make the target also a source. This should have no effect.
		events(&r, -1, transfer.SourceFilter{Target: tgt, Type: "file"})
		r.Frame(ops)
		// Drag.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Press,
			},
			pointer.Event{
				Position: f32.Pt(10, 10),
				Kind:     pointer.Move,
			},
			pointer.Event{
				Position: f32.Pt(40, 10),
				Kind:     pointer.Release,
			},
		)
		ofr := &offer{data: "hello"}
		events(&r, -1, transfer.SourceFilter{Target: src, Type: "file"})
		events(&r, -1, transfer.TargetFilter{Target: tgt, Type: "file"})
		r.Frame(ops)
		r.Source().Execute(transfer.OfferCmd{Tag: src, Type: "file", Data: ofr})
		assertEventSequence(t, events(&r, -1, transfer.SourceFilter{Target: src, Type: "file"}), transfer.CancelEvent{})
		// Ignore DataEvent and verify that the next frame closes it as unused.
		assertEventSequence(t, events(&r, -1, transfer.TargetFilter{Target: tgt, Type: "file"})[1:], transfer.CancelEvent{})
		r.Frame(ops)
		if !ofr.closed {
			t.Error("offer was not closed")
		}
	})
}

func TestDeferredInputOp(t *testing.T) {
	var ops op.Ops

	var r Router
	m := op.Record(&ops)
	event.Op(&ops, new(int))
	call := m.Stop()

	op.Defer(&ops, call)
	r.Frame(&ops)
}

func TestPassCursor(t *testing.T) {
	var ops op.Ops
	var r Router

	rect := clip.Rect(image.Rect(0, 0, 100, 100))
	background := rect.Push(&ops)
	event.Op(&ops, 1)
	pointer.CursorDefault.Add(&ops)
	background.Pop()

	overlayPass := pointer.PassOp{}.Push(&ops)
	overlay := rect.Push(&ops)
	event.Op(&ops, 2)
	want := pointer.CursorPointer
	want.Add(&ops)
	overlay.Pop()
	overlayPass.Pop()
	r.Frame(&ops)
	r.Queue(pointer.Event{
		Position: f32.Pt(10, 10),
		Kind:     pointer.Move,
	})
	r.Frame(&ops)
	if got := r.Cursor(); want != got {
		t.Errorf("got cursor %v, want %v", got, want)
	}
}

func TestPartialEvent(t *testing.T) {
	var ops op.Ops
	var r Router

	rect := clip.Rect(image.Rect(0, 0, 100, 100))
	background := rect.Push(&ops)
	event.Op(&ops, 1)
	background.Pop()

	overlayPass := pointer.PassOp{}.Push(&ops)
	overlay := rect.Push(&ops)
	event.Op(&ops, 2)
	overlay.Pop()
	overlayPass.Pop()
	assertEventSequence(t, events(&r, -1, pointer.Filter{Target: 1, Kinds: pointer.Press}))
	assertEventSequence(t, events(&r, -1, pointer.Filter{Target: 2, Kinds: pointer.Press}))
	r.Frame(&ops)
	r.Queue(pointer.Event{
		Kind: pointer.Press,
	})
	assertEventSequence(t, events(&r, -1, pointer.Filter{Target: 1, Kinds: pointer.Press}, key.FocusFilter{Target: 1}),
		key.FocusEvent{}, pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Priority: pointer.Shared})
	r.Source().Execute(key.FocusCmd{Tag: 1})
	assertEventSequence(t, events(&r, -1, pointer.Filter{Target: 2, Kinds: pointer.Press}),
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Priority: pointer.Foremost})
}

// offer satisfies io.ReadCloser for use in data transfers.
type offer struct {
	data   string
	closed bool
}

func (offer) Read([]byte) (int, error) { return 0, nil }
func (o *offer) Close() error {
	o.closed = true
	return nil
}

// addPointerHandler adds a pointer.InputOp for the tag in a
// rectangular area.
func addPointerHandler(r *Router, ops *op.Ops, tag event.Tag, area image.Rectangle) pointer.Filter {
	f := pointer.Filter{
		Target: tag,
		Kinds:  pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Enter | pointer.Leave | pointer.Cancel,
	}
	events(r, -1, f)
	defer clip.Rect(area).Push(ops).Pop()
	event.Op(ops, tag)
	return f
}

// pointerTypes converts a sequence of event.Event to their pointer.Types. It assumes
// that all input events are of underlying type pointer.Event, and thus will
// panic if some are not.
func pointerTypes(events []event.Event) []pointer.Kind {
	var types []pointer.Kind
	for _, e := range events {
		if e, ok := e.(pointer.Event); ok {
			types = append(types, e.Kind)
		}
	}
	return types
}

// assertEventPointerTypeSequence checks that the provided events match the expected pointer event types
// in the provided order.
func assertEventPointerTypeSequence(t *testing.T, events []event.Event, expected ...pointer.Kind) {
	t.Helper()
	got := pointerTypes(events)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %v events, got %v", expected, got)
	}
}

// assertEventSequence checks that the provided events match the expected ones
// in the provided order.
func assertEventSequence(t *testing.T, got []event.Event, expected ...event.Event) {
	t.Helper()
	if len(expected) == 0 {
		if len(got) > 0 {
			t.Errorf("unexpected events: %v", eventsToString(got))
		}
		return
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %s events, got %s", eventsToString(expected), eventsToString(got))
	}
}

// assertEventTypeSequence checks that the provided event types match expected.
func assertEventTypeSequence(t *testing.T, got []event.Event, expected ...event.Event) {
	t.Helper()
	match := len(expected) == len(got)
	if match {
		for i, ge := range got {
			exp := expected[i]
			match = match && reflect.TypeOf(ge) == reflect.TypeOf(exp)
		}
	}
	if !match {
		t.Errorf("expected event types %s, got %s", eventTypesToString(expected), eventTypesToString(got))
	}
}

func eventTypesToString(evs []event.Event) string {
	var s []string
	for _, e := range evs {
		s = append(s, fmt.Sprintf("%T", e))
	}
	return "[" + strings.Join(s, ",") + "]"
}

func eventsToString(evs []event.Event) string {
	var s []string
	for _, ev := range evs {
		switch e := ev.(type) {
		case pointer.Event:
			s = append(s, fmt.Sprintf("%T{%s}", e, e.Kind.String()))
		default:
			s = append(s, fmt.Sprintf("{%T}", e))
		}
	}
	return "[" + strings.Join(s, ",") + "]"
}

// assertEventPriorities checks that the pointer.Event priorities of events match prios.
func assertEventPriorities(t *testing.T, events []event.Event, prios ...pointer.Priority) {
	t.Helper()
	var got []pointer.Priority
	for _, e := range events {
		if e, ok := e.(pointer.Event); ok {
			got = append(got, e.Priority)
		}
	}
	if !reflect.DeepEqual(got, prios) {
		t.Errorf("expected priorities %v, got %v", prios, got)
	}
}

// assertScrollEvent checks that the event scrolling amount matches the supplied value.
func assertScrollEvent(t *testing.T, ev event.Event, scroll f32.Point) {
	t.Helper()
	if got, want := ev.(pointer.Event).Scroll, scroll; got != want {
		t.Errorf("got %v; want %v", got, want)
	}
}

// assertActionAt checks that the router has a system action of the expected type at point.
func assertActionAt(t *testing.T, q Router, point f32.Point, expected system.Action) {
	t.Helper()
	action, ok := q.ActionAt(point)
	if !ok {
		t.Errorf("expected action %v at %v, got no action", expected, point)
	} else if action != expected {
		t.Errorf("expected action %v at %v, got %v", expected, point, action)
	}
}

func BenchmarkRouterAdd(b *testing.B) {
	// Set this to the number of overlapping handlers that you want to
	// evaluate performance for. Typical values for the example applications
	// are 1-3, though checking highers values helps evaluate performance for
	// more complex applications.
	const startingHandlerCount = 3
	const maxHandlerCount = 100
	for i := startingHandlerCount; i < maxHandlerCount; i *= 3 {
		handlerCount := i
		b.Run(fmt.Sprintf("%d-handlers", i), func(b *testing.B) {
			handlers := make([]event.Tag, handlerCount)
			for i := 0; i < handlerCount; i++ {
				h := new(int)
				*h = i
				handlers[i] = h
			}
			var ops op.Ops

			var r Router
			for i := range handlers {
				clip.Rect(image.Rectangle{
					Max: image.Point{
						X: 100,
						Y: 100,
					},
				}).
					Push(&ops)
				events(&r, -1, pointer.Filter{Target: handlers[i], Kinds: pointer.Move})
				event.Op(&ops, handlers[i])
			}
			r.Frame(&ops)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Queue(
					pointer.Event{
						Kind:     pointer.Move,
						Position: f32.Pt(50, 50),
					},
				)
			}
		})
	}
}

func events(r *Router, n int, filters ...event.Filter) []event.Event {
	var events []event.Event
	for {
		if n != -1 && len(events) == n {
			break
		}
		e, ok := r.Event(filters...)
		if !ok {
			break
		}
		events = append(events, e)
	}
	return events
}
