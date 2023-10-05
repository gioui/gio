// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
	"fmt"
	"image"
	"reflect"
	"strings"
	"testing"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/io/transfer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func TestPointerWakeup(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	addPointerHandler(&ops, handler, image.Rect(0, 0, 100, 100))

	var r Router
	// Test that merely adding a handler doesn't trigger redraw.
	r.Frame(&ops)
	if _, wake := r.WakeupTime(); wake {
		t.Errorf("adding pointer.InputOp triggered a redraw")
	}
	// However, adding a handler queues a Cancel event.
	assertEventPointerTypeSequence(t, r.Events(handler), pointer.Cancel)
}

func TestPointerDrag(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	addPointerHandler(&ops, handler, image.Rect(0, 0, 100, 100))

	var r Router
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
	assertEventPointerTypeSequence(t, r.Events(handler), pointer.Cancel, pointer.Enter, pointer.Press, pointer.Leave, pointer.Drag)
}

func TestPointerDragNegative(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	addPointerHandler(&ops, handler, image.Rect(-100, -100, 0, 0))

	var r Router
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
	assertEventPointerTypeSequence(t, r.Events(handler), pointer.Cancel, pointer.Enter, pointer.Press, pointer.Leave, pointer.Drag)
}

func TestPointerGrab(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	handler3 := new(int)
	var ops op.Ops

	types := pointer.Press | pointer.Release

	pointer.InputOp{Tag: handler1, Kinds: types, Grab: true}.Add(&ops)
	pointer.InputOp{Tag: handler2, Kinds: types}.Add(&ops)
	pointer.InputOp{Tag: handler3, Kinds: types}.Add(&ops)

	var r Router
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Cancel, pointer.Press)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Cancel, pointer.Press)
	assertEventPointerTypeSequence(t, r.Events(handler3), pointer.Cancel, pointer.Press)
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Release,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Release)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Cancel)
	assertEventPointerTypeSequence(t, r.Events(handler3), pointer.Cancel)
}

func TestPointerGrabSameHandlerTwice(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	types := pointer.Press | pointer.Release

	pointer.InputOp{Tag: handler1, Kinds: types, Grab: true}.Add(&ops)
	pointer.InputOp{Tag: handler1, Kinds: types}.Add(&ops)
	pointer.InputOp{Tag: handler2, Kinds: types}.Add(&ops)

	var r Router
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Cancel, pointer.Press)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Cancel, pointer.Press)
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Release,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Release)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Cancel)
}

func TestPointerMove(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	types := pointer.Move | pointer.Enter | pointer.Leave

	// Handler 1 area: (0, 0) - (100, 100)
	r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	pointer.InputOp{Tag: handler1, Kinds: types}.Add(&ops)
	// Handler 2 area: (50, 50) - (100, 100) (areas intersect).
	r2 := clip.Rect(image.Rect(50, 50, 200, 200)).Push(&ops)
	pointer.InputOp{Tag: handler2, Kinds: types}.Add(&ops)
	r2.Pop()
	r1.Pop()

	var r Router
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
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Cancel, pointer.Enter, pointer.Move, pointer.Move, pointer.Leave, pointer.Cancel)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Cancel, pointer.Enter, pointer.Move, pointer.Leave, pointer.Cancel)
}

func TestPointerTypes(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	pointer.InputOp{
		Tag:   handler,
		Kinds: pointer.Press | pointer.Release,
	}.Add(&ops)
	r1.Pop()

	var r Router
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
	assertEventPointerTypeSequence(t, r.Events(handler), pointer.Cancel, pointer.Press, pointer.Release)
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

	r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	pointer.InputOp{
		Tag:          handler1,
		Kinds:        pointer.Scroll,
		ScrollBounds: image.Rectangle{Max: image.Point{X: 100}},
	}.Add(&ops)

	r2 := clip.Rect(image.Rect(0, 0, 100, 50)).Push(&ops)
	pointer.InputOp{
		Tag:          handler2,
		Kinds:        pointer.Scroll,
		ScrollBounds: image.Rectangle{Max: image.Point{X: 20}},
	}.Add(&ops)
	r2.Pop()
	r1.Pop()

	r3 := clip.Rect(image.Rect(0, 100, 100, 200)).Push(&ops)
	pointer.InputOp{
		Tag:          handler3,
		Kinds:        pointer.Scroll,
		ScrollBounds: image.Rectangle{Min: image.Point{X: -20, Y: -40}},
	}.Add(&ops)
	r3.Pop()

	var r Router
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

	hev1 := r.Events(handler1)
	hev2 := r.Events(handler2)
	hev3 := r.Events(handler3)
	assertEventPointerTypeSequence(t, hev1, pointer.Cancel, pointer.Scroll, pointer.Scroll)
	assertEventPointerTypeSequence(t, hev2, pointer.Cancel, pointer.Scroll)
	assertEventPointerTypeSequence(t, hev3, pointer.Cancel, pointer.Scroll)
	assertEventPriorities(t, hev1, pointer.Shared, pointer.Shared, pointer.Foremost)
	assertEventPriorities(t, hev2, pointer.Shared, pointer.Foremost)
	assertEventPriorities(t, hev3, pointer.Shared, pointer.Foremost)
	assertScrollEvent(t, hev1[1], f32.Pt(30, 0))
	assertScrollEvent(t, hev2[1], f32.Pt(20, 0))
	assertScrollEvent(t, hev1[2], f32.Pt(50, 0))
	assertScrollEvent(t, hev3[1], f32.Pt(-20, -30))
}

func TestPointerEnterLeave(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	// Handler 1 area: (0, 0) - (100, 100)
	addPointerHandler(&ops, handler1, image.Rect(0, 0, 100, 100))

	// Handler 2 area: (50, 50) - (200, 200) (areas overlap).
	addPointerHandler(&ops, handler2, image.Rect(50, 50, 200, 200))

	var r Router
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
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Cancel)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Cancel, pointer.Enter, pointer.Move)

	// Leave the second area by moving into the first.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(45, 45),
		},
	)
	// The cursor leaves handler2 and enters handler1.
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Enter, pointer.Move)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Leave)

	// Move, but stay within the same hit area.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(40, 40),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Move)
	assertEventPointerTypeSequence(t, r.Events(handler2))

	// Move outside of both inputs.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(300, 300),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Leave)
	assertEventPointerTypeSequence(t, r.Events(handler2))

	// Check that a Press event generates Enter Events.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(125, 125),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1))
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Enter, pointer.Press)

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
	assertEventPointerTypeSequence(t, r.Events(handler1))
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Leave, pointer.Drag, pointer.Enter, pointer.Drag)

	// Check that a Release event generates Enter/Leave Events.
	r.Queue(
		pointer.Event{
			Kind: pointer.Release,
			Position: f32.Pt(25,
				25),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Enter)
	// The second handler gets the release event because the press started inside it.
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Release, pointer.Leave)

}

func TestMultipleAreas(t *testing.T) {
	handler := new(int)

	var ops op.Ops

	addPointerHandler(&ops, handler, image.Rect(0, 0, 100, 100))
	r1 := clip.Rect(image.Rect(50, 50, 200, 200)).Push(&ops)
	// Second area has no Types set, yet should receive events because
	// Types for the same handles are or-ed together.
	pointer.InputOp{Tag: handler}.Add(&ops)
	r1.Pop()

	var r Router
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
	assertEventPointerTypeSequence(t, r.Events(handler), pointer.Cancel, pointer.Enter, pointer.Move, pointer.Move, pointer.Move)
}

func TestPointerEnterLeaveNested(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	types := pointer.Press | pointer.Move | pointer.Release | pointer.Enter | pointer.Leave

	// Handler 1 area: (0, 0) - (100, 100)
	r1 := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	pointer.InputOp{Tag: handler1, Kinds: types}.Add(&ops)

	// Handler 2 area: (25, 25) - (75, 75) (nested within first).
	r2 := clip.Rect(image.Rect(25, 25, 75, 75)).Push(&ops)
	pointer.InputOp{Tag: handler2, Kinds: types}.Add(&ops)
	r2.Pop()
	r1.Pop()

	var r Router
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
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Cancel, pointer.Enter, pointer.Move)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Cancel, pointer.Enter, pointer.Move)

	// Leave the second area by moving into the first.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(20, 20),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Move)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Leave)

	// Move, but stay within the same hit area.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(10, 10),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Move)
	assertEventPointerTypeSequence(t, r.Events(handler2))

	// Move outside of both inputs.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(200, 200),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Leave)
	assertEventPointerTypeSequence(t, r.Events(handler2))

	// Check that a Press event generates Enter Events.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Enter, pointer.Press)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Enter, pointer.Press)

	// Check that a Release event generates Enter/Leave Events.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Release,
			Position: f32.Pt(20, 20),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Release)
	assertEventPointerTypeSequence(t, r.Events(handler2), pointer.Release, pointer.Leave)
}

func TestPointerActiveInputDisappears(t *testing.T) {
	handler1 := new(int)
	var ops op.Ops
	var r Router

	// Draw handler.
	ops.Reset()
	addPointerHandler(&ops, handler1, image.Rect(0, 0, 100, 100))
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1), pointer.Cancel, pointer.Enter, pointer.Move)

	// Re-render with handler missing.
	ops.Reset()
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
	)
	assertEventPointerTypeSequence(t, r.Events(handler1))
}

func TestMultitouch(t *testing.T) {
	var ops op.Ops

	// Add two separate handlers.
	h1, h2 := new(int), new(int)
	addPointerHandler(&ops, h1, image.Rect(0, 0, 100, 100))
	addPointerHandler(&ops, h2, image.Rect(0, 100, 100, 200))

	h1pt, h2pt := f32.Pt(0, 0), f32.Pt(0, 100)
	var p1, p2 pointer.ID = 0, 1

	var r Router
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
	assertEventPointerTypeSequence(t, r.Events(h1), pointer.Cancel, pointer.Enter, pointer.Press)
	assertEventPointerTypeSequence(t, r.Events(h2), pointer.Cancel, pointer.Enter, pointer.Press, pointer.Release)
}

func TestCursor(t *testing.T) {
	ops := new(op.Ops)
	var r Router
	var h, h2 int
	var widget2 func()
	widget := func() {
		// This is the area where the cursor is changed to CursorPointer.
		defer clip.Rect(image.Rectangle{Max: image.Pt(100, 100)}).Push(ops).Pop()
		// The cursor is checked and changed upon cursor movement.
		pointer.InputOp{Tag: &h}.Add(ops)
		pointer.CursorPointer.Add(ops)
		if widget2 != nil {
			widget2()
		}
	}
	// Register the handlers.
	widget()
	// No cursor change as the mouse has not moved yet.
	if got, want := r.Cursor(), pointer.CursorDefault; got != want {
		t.Errorf("got %q; want %q", got, want)
	}

	_at := func(x, y float32) pointer.Event {
		return pointer.Event{
			Kind:     pointer.Move,
			Source:   pointer.Mouse,
			Buttons:  pointer.ButtonPrimary,
			Position: f32.Pt(x, y),
		}
	}
	for _, tc := range []struct {
		label string
		event interface{}
		want  pointer.Cursor
	}{
		{label: "move inside",
			event: _at(50, 50),
			want:  pointer.CursorPointer,
		},
		{label: "move outside",
			event: _at(200, 200),
			want:  pointer.CursorDefault,
		},
		{label: "move back inside",
			event: _at(50, 50),
			want:  pointer.CursorPointer,
		},
		{label: "send key events while inside",
			event: []event.Event{
				key.Event{Name: "A", State: key.Press},
				key.Event{Name: "A", State: key.Release},
			},
			want: pointer.CursorPointer,
		},
		{label: "send key events while outside",
			event: []event.Event{
				_at(200, 200),
				key.Event{Name: "A", State: key.Press},
				key.Event{Name: "A", State: key.Release},
			},
			want: pointer.CursorDefault,
		},
		{label: "add new input on top while inside",
			event: func() []event.Event {
				widget2 = func() {
					pointer.InputOp{Tag: &h2}.Add(ops)
					pointer.CursorCrosshair.Add(ops)
				}
				return []event.Event{
					_at(50, 50),
					key.Event{
						Name:  "A",
						State: key.Press,
					},
				}
			},
			want: pointer.CursorCrosshair,
		},
		{label: "remove input on top while inside",
			event: func() []event.Event {
				widget2 = nil
				return []event.Event{
					_at(50, 50),
					key.Event{
						Name:  "A",
						State: key.Press,
					},
				}
			},
			want: pointer.CursorPointer,
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			ops.Reset()
			widget()
			r.Frame(ops)
			switch ev := tc.event.(type) {
			case event.Event:
				r.Queue(ev)
			case []event.Event:
				r.Queue(ev...)
			case func() event.Event:
				r.Queue(ev())
			case func() []event.Event:
				r.Queue(ev()...)
			default:
				panic(fmt.Sprintf("unknown event %T", ev))
			}
			widget()
			r.Frame(ops)
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
	pointer.InputOp{Tag: h1, Kinds: pointer.Press}.Add(&ops)
	child1 := area.Push(&ops)
	pointer.InputOp{Tag: h2, Kinds: pointer.Press}.Add(&ops)
	child1.Pop()
	child2 := area.Push(&ops)
	pass := pointer.PassOp{}.Push(&ops)
	pointer.InputOp{Tag: h3, Kinds: pointer.Press}.Add(&ops)
	pointer.InputOp{Tag: h4, Kinds: pointer.Press}.Add(&ops)
	pass.Pop()
	child2.Pop()
	root.Pop()

	var r Router
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind: pointer.Press,
		},
	)
	assertEventPointerTypeSequence(t, r.Events(h1), pointer.Cancel, pointer.Press)
	assertEventPointerTypeSequence(t, r.Events(h2), pointer.Cancel, pointer.Press)
	assertEventPointerTypeSequence(t, r.Events(h3), pointer.Cancel, pointer.Press)
	assertEventPointerTypeSequence(t, r.Events(h4), pointer.Cancel, pointer.Press)
}

func TestAreaPassthrough(t *testing.T) {
	var ops op.Ops

	h := new(int)
	pointer.InputOp{Tag: h, Kinds: pointer.Press}.Add(&ops)
	clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops).Pop()
	var r Router
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Kind: pointer.Press,
		},
	)
	assertEventPointerTypeSequence(t, r.Events(h), pointer.Cancel, pointer.Press)
}

func TestEllipse(t *testing.T) {
	var ops op.Ops

	h := new(int)
	cl := clip.Ellipse(image.Rect(0, 0, 100, 100)).Push(&ops)
	pointer.InputOp{Tag: h, Kinds: pointer.Press}.Add(&ops)
	cl.Pop()
	var r Router
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
	assertEventPointerTypeSequence(t, r.Events(h), pointer.Cancel, pointer.Press)
}

func TestTransfer(t *testing.T) {
	srcArea := image.Rect(0, 0, 20, 20)
	tgtArea := srcArea.Add(image.Pt(40, 0))
	setup := func(ops *op.Ops, srcType, tgtType string) (src, tgt event.Tag) {
		src, tgt = new(int), new(int)

		srcStack := clip.Rect(srcArea).Push(ops)
		transfer.SourceOp{
			Tag:  src,
			Type: srcType,
		}.Add(ops)
		srcStack.Pop()

		tgt1Stack := clip.Rect(tgtArea).Push(ops)
		transfer.TargetOp{
			Tag:  tgt,
			Type: tgtType,
		}.Add(ops)
		tgt1Stack.Pop()

		return src, tgt
	}
	// Cancel is received when the pointer is first seen.
	cancel := pointer.Event{Kind: pointer.Cancel}

	t.Run("transfer.Offer should panic on nil Data", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("expected panic upon invalid data")
			}
		}()
		transfer.OfferOp{}.Add(new(op.Ops))
	})

	t.Run("drop on no target", func(t *testing.T) {
		ops := new(op.Ops)
		src, tgt := setup(ops, "file", "file")
		var r Router
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
		assertEventSequence(t, r.Events(src), cancel)
		assertEventSequence(t, r.Events(tgt), cancel, transfer.InitiateEvent{})

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
		assertEventSequence(t, r.Events(src), transfer.CancelEvent{})
		assertEventSequence(t, r.Events(tgt), transfer.CancelEvent{})
	})

	t.Run("drag with valid and invalid targets", func(t *testing.T) {
		ops := new(op.Ops)
		src, tgt1 := setup(ops, "file", "file")
		tgt2 := new(int)
		stack := clip.Rect(tgtArea).Push(ops)
		transfer.TargetOp{
			Tag:  tgt2,
			Type: "nofile",
		}.Add(ops)
		stack.Pop()
		var r Router
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
		assertEventSequence(t, r.Events(src), cancel)
		assertEventSequence(t, r.Events(tgt1), cancel, transfer.InitiateEvent{})
		assertEventSequence(t, r.Events(tgt2), cancel)
	})

	t.Run("drop on invalid target", func(t *testing.T) {
		ops := new(op.Ops)
		src, tgt := setup(ops, "file", "nofile")
		var r Router
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
		assertEventSequence(t, r.Events(src), cancel)
		assertEventSequence(t, r.Events(tgt), cancel)

		// Drop.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(40, 10),
				Kind:     pointer.Release,
			},
		)
		assertEventSequence(t, r.Events(src), transfer.CancelEvent{})
		assertEventSequence(t, r.Events(tgt))
	})

	t.Run("drop on valid target", func(t *testing.T) {
		ops := new(op.Ops)
		src, tgt := setup(ops, "file", "file")
		// Make the target also a source. This should have no effect.
		stack := clip.Rect(tgtArea).Push(ops)
		transfer.SourceOp{
			Tag:  tgt,
			Type: "file",
		}.Add(ops)
		stack.Pop()
		var r Router
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
		assertEventSequence(t, r.Events(src), cancel)
		assertEventSequence(t, r.Events(tgt), cancel, transfer.InitiateEvent{})

		// Drop.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(40, 10),
				Kind:     pointer.Release,
			},
		)
		assertEventSequence(t, r.Events(src), transfer.RequestEvent{Type: "file"})

		// Offer valid type and data.
		ofr := &offer{data: "hello"}
		transfer.OfferOp{
			Tag:  src,
			Type: "file",
			Data: ofr,
		}.Add(ops)
		r.Frame(ops)
		evs := r.Events(tgt)
		if len(evs) != 1 {
			t.Fatalf("unexpected number of events: %d, want 1", len(evs))
		}
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
		r.Frame(ops)
		assertEventSequence(t, r.Events(src), transfer.CancelEvent{})
		assertEventSequence(t, r.Events(tgt), transfer.CancelEvent{})
	})

	t.Run("drop on valid target, DataEvent not used", func(t *testing.T) {
		ops := new(op.Ops)
		src, tgt := setup(ops, "file", "file")
		// Make the target also a source. This should have no effect.
		stack := clip.Rect(tgtArea).Push(ops)
		transfer.SourceOp{
			Tag:  tgt,
			Type: "file",
		}.Add(ops)
		stack.Pop()
		var r Router
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
		transfer.OfferOp{
			Tag:  src,
			Type: "file",
			Data: ofr,
		}.Add(ops)
		r.Frame(ops)
		// DataEvent should be used here. The next frame should close it as unused.
		r.Frame(ops)
		assertEventSequence(t, r.Events(src), transfer.CancelEvent{})
		assertEventSequence(t, r.Events(tgt), transfer.CancelEvent{})
		if !ofr.closed {
			t.Error("offer was not closed")
		}
	})

	t.Run("valid target enter/leave events", func(t *testing.T) {
		ops := new(op.Ops)
		src, _ := setup(ops, "file", "file")
		var hover gesture.Hover
		pass := pointer.PassOp{}.Push(ops)
		stack := clip.Rect(tgtArea).Push(ops)
		hover.Add(ops)
		stack.Pop()
		pass.Pop()

		var r Router
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
				Kind:     pointer.Move,
			},
		)
		assertEventPointerTypeSequence(t, r.Events(&hover), pointer.Cancel, pointer.Enter)

		// Drop.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(40, 10),
				Kind:     pointer.Release,
			},
		)

		// Offer valid type and data.
		ofr := &offer{data: "hello"}
		transfer.OfferOp{
			Tag:  src,
			Type: "file",
			Data: ofr,
		}.Add(ops)
		r.Frame(ops)
		assertEventPointerTypeSequence(t, r.Events(&hover), pointer.Leave)
	})

	t.Run("invalid target NO enter/leave events", func(t *testing.T) {
		ops := new(op.Ops)
		src, _ := setup(ops, "file", "nofile")
		var hover gesture.Hover
		pass := pointer.PassOp{}.Push(ops)
		stack := clip.Rect(tgtArea).Push(ops)
		hover.Add(ops)
		stack.Pop()
		pass.Pop()

		var r Router
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
				Kind:     pointer.Move,
			},
		)
		assertEventPointerTypeSequence(t, r.Events(&hover), pointer.Cancel)

		// Drop.
		r.Queue(
			pointer.Event{
				Position: f32.Pt(40, 10),
				Kind:     pointer.Release,
			},
		)

		// Offer valid type and data.
		ofr := &offer{data: "hello"}
		transfer.OfferOp{
			Tag:  src,
			Type: "file",
			Data: ofr,
		}.Add(ops)
		r.Frame(ops)
		assertEventPointerTypeSequence(t, r.Events(&hover), pointer.Leave)
	})
}

func TestDeferredInputOp(t *testing.T) {
	var ops op.Ops

	var r Router
	m := op.Record(&ops)
	key.InputOp{Tag: new(int)}.Add(&ops)
	call := m.Stop()

	op.Defer(&ops, call)
	r.Frame(&ops)
}

func TestPassCursor(t *testing.T) {
	var ops op.Ops
	var r Router

	rect := clip.Rect(image.Rect(0, 0, 100, 100))
	background := rect.Push(&ops)
	pointer.InputOp{Tag: 1}.Add(&ops)
	pointer.CursorDefault.Add(&ops)
	background.Pop()

	overlayPass := pointer.PassOp{}.Push(&ops)
	overlay := rect.Push(&ops)
	pointer.InputOp{Tag: 2}.Add(&ops)
	want := pointer.CursorPointer
	want.Add(&ops)
	overlay.Pop()
	overlayPass.Pop()
	r.Frame(&ops)
	r.Queue(pointer.Event{
		Position: f32.Pt(10, 10),
		Kind:     pointer.Move,
	})
	if got := r.Cursor(); want != got {
		t.Errorf("got cursor %v, want %v", got, want)
	}
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
func addPointerHandler(ops *op.Ops, tag event.Tag, area image.Rectangle) {
	defer clip.Rect(area).Push(ops).Pop()
	pointer.InputOp{
		Tag:   tag,
		Kinds: pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Enter | pointer.Leave,
	}.Add(ops)
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

			for i := range handlers {
				clip.Rect(image.Rectangle{
					Max: image.Point{
						X: 100,
						Y: 100,
					},
				}).
					Push(&ops)
				pointer.InputOp{
					Tag:   handlers[i],
					Kinds: pointer.Move,
				}.Add(&ops)
			}
			var r Router
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
