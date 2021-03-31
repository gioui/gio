// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
	"fmt"
	"image"
	"reflect"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
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
	assertEventSequence(t, r.Events(handler), pointer.Cancel)
	// Verify that r.Events does trigger a redraw.
	r.Frame(&ops)
	if _, wake := r.WakeupTime(); !wake {
		t.Errorf("pointer.Cancel event didn't trigger a redraw")
	}
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
			Type:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
		// Move outside the area.
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(150, 150),
		},
	)
	assertEventSequence(t, r.Events(handler), pointer.Cancel, pointer.Enter, pointer.Press, pointer.Leave, pointer.Drag)
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
			Type:     pointer.Press,
			Position: f32.Pt(-50, -50),
		},
		// Move outside the area.
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(-150, -150),
		},
	)
	assertEventSequence(t, r.Events(handler), pointer.Cancel, pointer.Enter, pointer.Press, pointer.Leave, pointer.Drag)
}

func TestPointerGrab(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	handler3 := new(int)
	var ops op.Ops

	types := pointer.Press | pointer.Release

	pointer.InputOp{Tag: handler1, Types: types, Grab: true}.Add(&ops)
	pointer.InputOp{Tag: handler2, Types: types}.Add(&ops)
	pointer.InputOp{Tag: handler3, Types: types}.Add(&ops)

	var r Router
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Type:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Cancel, pointer.Press)
	assertEventSequence(t, r.Events(handler2), pointer.Cancel, pointer.Press)
	assertEventSequence(t, r.Events(handler3), pointer.Cancel, pointer.Press)
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Type:     pointer.Release,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Release)
	assertEventSequence(t, r.Events(handler2), pointer.Cancel)
	assertEventSequence(t, r.Events(handler3), pointer.Cancel)
}

func TestPointerMove(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	types := pointer.Move | pointer.Enter | pointer.Leave

	// Handler 1 area: (0, 0) - (100, 100)
	pointer.Rect(image.Rect(0, 0, 100, 100)).Add(&ops)
	pointer.InputOp{Tag: handler1, Types: types}.Add(&ops)
	// Handler 2 area: (50, 50) - (100, 100) (areas intersect).
	pointer.Rect(image.Rect(50, 50, 200, 200)).Add(&ops)
	pointer.InputOp{Tag: handler2, Types: types}.Add(&ops)

	var r Router
	r.Frame(&ops)
	r.Queue(
		// Hit both handlers.
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
		// Hit handler 1.
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(49, 50),
		},
		// Hit no handlers.
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(100, 50),
		},
		pointer.Event{
			Type: pointer.Cancel,
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Cancel, pointer.Enter, pointer.Move, pointer.Move, pointer.Leave, pointer.Cancel)
	assertEventSequence(t, r.Events(handler2), pointer.Cancel, pointer.Enter, pointer.Move, pointer.Leave, pointer.Cancel)
}

func TestPointerTypes(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	pointer.Rect(image.Rect(0, 0, 100, 100)).Add(&ops)
	pointer.InputOp{
		Tag:   handler,
		Types: pointer.Press | pointer.Release,
	}.Add(&ops)

	var r Router
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Type:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(150, 150),
		},
		pointer.Event{
			Type:     pointer.Release,
			Position: f32.Pt(150, 150),
		},
	)
	assertEventSequence(t, r.Events(handler), pointer.Cancel, pointer.Press, pointer.Release)
}

func TestPointerPriority(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	handler3 := new(int)
	var ops op.Ops

	st := op.Save(&ops)
	pointer.Rect(image.Rect(0, 0, 100, 100)).Add(&ops)
	pointer.InputOp{
		Tag:          handler1,
		Types:        pointer.Scroll,
		ScrollBounds: image.Rectangle{Max: image.Point{X: 100}},
	}.Add(&ops)

	pointer.Rect(image.Rect(0, 0, 100, 50)).Add(&ops)
	pointer.InputOp{
		Tag:          handler2,
		Types:        pointer.Scroll,
		ScrollBounds: image.Rectangle{Max: image.Point{X: 20}},
	}.Add(&ops)
	st.Load()

	pointer.Rect(image.Rect(0, 100, 100, 200)).Add(&ops)
	pointer.InputOp{
		Tag:          handler3,
		Types:        pointer.Scroll,
		ScrollBounds: image.Rectangle{Min: image.Point{X: -20, Y: -40}},
	}.Add(&ops)

	var r Router
	r.Frame(&ops)
	r.Queue(
		// Hit handler 1 and 2.
		pointer.Event{
			Type:     pointer.Scroll,
			Position: f32.Pt(50, 25),
			Scroll:   f32.Pt(50, 0),
		},
		// Hit handler 1.
		pointer.Event{
			Type:     pointer.Scroll,
			Position: f32.Pt(50, 75),
			Scroll:   f32.Pt(50, 50),
		},
		// Hit handler 3.
		pointer.Event{
			Type:     pointer.Scroll,
			Position: f32.Pt(50, 150),
			Scroll:   f32.Pt(-30, -30),
		},
		// Hit no handlers.
		pointer.Event{
			Type:     pointer.Scroll,
			Position: f32.Pt(50, 225),
		},
	)

	hev1 := r.Events(handler1)
	hev2 := r.Events(handler2)
	hev3 := r.Events(handler3)
	assertEventSequence(t, hev1, pointer.Cancel, pointer.Scroll, pointer.Scroll)
	assertEventSequence(t, hev2, pointer.Cancel, pointer.Scroll)
	assertEventSequence(t, hev3, pointer.Cancel, pointer.Scroll)
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
			Type:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
	)
	// First event for a handler is always a Cancel.
	// Only handler2 should receive the enter/move events because it is on top
	// and handler1 is not an ancestor in the hit tree.
	assertEventSequence(t, r.Events(handler1), pointer.Cancel)
	assertEventSequence(t, r.Events(handler2), pointer.Cancel, pointer.Enter, pointer.Move)

	// Leave the second area by moving into the first.
	r.Queue(
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(45, 45),
		},
	)
	// The cursor leaves handler2 and enters handler1.
	assertEventSequence(t, r.Events(handler1), pointer.Enter, pointer.Move)
	assertEventSequence(t, r.Events(handler2), pointer.Leave)

	// Move, but stay within the same hit area.
	r.Queue(
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(40, 40),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Move)
	assertEventSequence(t, r.Events(handler2))

	// Move outside of both inputs.
	r.Queue(
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(300, 300),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Leave)
	assertEventSequence(t, r.Events(handler2))

	// Check that a Press event generates Enter Events.
	r.Queue(
		pointer.Event{
			Type:     pointer.Press,
			Position: f32.Pt(125, 125),
		},
	)
	assertEventSequence(t, r.Events(handler1))
	assertEventSequence(t, r.Events(handler2), pointer.Enter, pointer.Press)

	// Check that a drag only affects the participating handlers.
	r.Queue(
		// Leave
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
		// Enter
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventSequence(t, r.Events(handler1))
	assertEventSequence(t, r.Events(handler2), pointer.Leave, pointer.Drag, pointer.Enter, pointer.Drag)

	// Check that a Release event generates Enter/Leave Events.
	r.Queue(
		pointer.Event{
			Type: pointer.Release,
			Position: f32.Pt(25,
				25),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Enter)
	// The second handler gets the release event because the press started inside it.
	assertEventSequence(t, r.Events(handler2), pointer.Release, pointer.Leave)

}

func TestMultipleAreas(t *testing.T) {
	handler := new(int)

	var ops op.Ops

	addPointerHandler(&ops, handler, image.Rect(0, 0, 100, 100))
	st := op.Save(&ops)
	pointer.Rect(image.Rect(50, 50, 200, 200)).Add(&ops)
	// Second area has no Types set, yet should receive events because
	// Types for the same handles are or-ed together.
	pointer.InputOp{Tag: handler}.Add(&ops)
	st.Load()

	var r Router
	r.Frame(&ops)
	// Hit first area, then second area, then both.
	r.Queue(
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(150, 150),
		},
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventSequence(t, r.Events(handler), pointer.Cancel, pointer.Enter, pointer.Move, pointer.Move, pointer.Move)
}

func TestPointerEnterLeaveNested(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	types := pointer.Press | pointer.Move | pointer.Release | pointer.Enter | pointer.Leave

	// Handler 1 area: (0, 0) - (100, 100)
	pointer.Rect(image.Rect(0, 0, 100, 100)).Add(&ops)
	pointer.InputOp{Tag: handler1, Types: types}.Add(&ops)

	// Handler 2 area: (25, 25) - (75, 75) (nested within first).
	pointer.Rect(image.Rect(25, 25, 75, 75)).Add(&ops)
	pointer.InputOp{Tag: handler2, Types: types}.Add(&ops)

	var r Router
	r.Frame(&ops)
	// Hit both handlers.
	r.Queue(
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(50, 50),
		},
	)
	// First event for a handler is always a Cancel.
	// Both handlers should receive the Enter and Move events because handler2 is a child of handler1.
	assertEventSequence(t, r.Events(handler1), pointer.Cancel, pointer.Enter, pointer.Move)
	assertEventSequence(t, r.Events(handler2), pointer.Cancel, pointer.Enter, pointer.Move)

	// Leave the second area by moving into the first.
	r.Queue(
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(20, 20),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Move)
	assertEventSequence(t, r.Events(handler2), pointer.Leave)

	// Move, but stay within the same hit area.
	r.Queue(
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(10, 10),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Move)
	assertEventSequence(t, r.Events(handler2))

	// Move outside of both inputs.
	r.Queue(
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(200, 200),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Leave)
	assertEventSequence(t, r.Events(handler2))

	// Check that a Press event generates Enter Events.
	r.Queue(
		pointer.Event{
			Type:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Enter, pointer.Press)
	assertEventSequence(t, r.Events(handler2), pointer.Enter, pointer.Press)

	// Check that a Release event generates Enter/Leave Events.
	r.Queue(
		pointer.Event{
			Type:     pointer.Release,
			Position: f32.Pt(20, 20),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Release)
	assertEventSequence(t, r.Events(handler2), pointer.Release, pointer.Leave)
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
			Type:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Cancel, pointer.Enter, pointer.Move)

	// Re-render with handler missing.
	ops.Reset()
	r.Frame(&ops)
	r.Queue(
		pointer.Event{
			Type:     pointer.Move,
			Position: f32.Pt(25, 25),
		},
	)
	assertEventSequence(t, r.Events(handler1))
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
			Type:      pointer.Press,
			Position:  h1pt,
			PointerID: p1,
		},
	)
	r.Queue(
		pointer.Event{
			Type:      pointer.Press,
			Position:  h2pt,
			PointerID: p2,
		},
	)
	r.Queue(
		pointer.Event{
			Type:      pointer.Release,
			Position:  h2pt,
			PointerID: p2,
		},
	)
	assertEventSequence(t, r.Events(h1), pointer.Cancel, pointer.Enter, pointer.Press)
	assertEventSequence(t, r.Events(h2), pointer.Cancel, pointer.Enter, pointer.Press, pointer.Release)
}

func TestCursorNameOp(t *testing.T) {
	ops := new(op.Ops)
	var r Router
	var h, h2 int
	var widget2 func()
	widget := func() {
		// This is the area where the cursor is changed to CursorPointer.
		pointer.Rect(image.Rectangle{Max: image.Pt(100, 100)}).Add(ops)
		// The cursor is checked and changed upon cursor movement.
		pointer.InputOp{Tag: &h}.Add(ops)
		pointer.CursorNameOp{Name: pointer.CursorPointer}.Add(ops)
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
			Type:     pointer.Move,
			Source:   pointer.Mouse,
			Buttons:  pointer.ButtonPrimary,
			Position: f32.Pt(x, y),
		}
	}
	for _, tc := range []struct {
		label string
		event interface{}
		want  pointer.CursorName
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
					pointer.CursorNameOp{Name: pointer.CursorCrossHair}.Add(ops)
				}
				return []event.Event{
					_at(50, 50),
					key.Event{
						Name:  "A",
						State: key.Press,
					},
				}
			},
			want: pointer.CursorCrossHair,
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
				panic(fmt.Sprintf("unkown event %T", ev))
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

// addPointerHandler adds a pointer.InputOp for the tag in a
// rectangular area.
func addPointerHandler(ops *op.Ops, tag event.Tag, area image.Rectangle) {
	defer op.Save(ops).Load()
	pointer.Rect(area).Add(ops)
	pointer.InputOp{
		Tag:   tag,
		Types: pointer.Press | pointer.Release | pointer.Move | pointer.Drag | pointer.Enter | pointer.Leave,
	}.Add(ops)
}

// pointerTypes converts a sequence of event.Event to their pointer.Types. It assumes
// that all input events are of underlying type pointer.Event, and thus will
// panic if some are not.
func pointerTypes(events []event.Event) []pointer.Type {
	var types []pointer.Type
	for _, e := range events {
		if e, ok := e.(pointer.Event); ok {
			types = append(types, e.Type)
		}
	}
	return types
}

// assertEventSequence checks that the provided events match the expected pointer event types
// in the provided order.
func assertEventSequence(t *testing.T, events []event.Event, expected ...pointer.Type) {
	t.Helper()
	got := pointerTypes(events)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected %v events, got %v", expected, got)
	}
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
				pointer.Rect(image.Rectangle{
					Max: image.Point{
						X: 100,
						Y: 100,
					},
				}).Add(&ops)
				pointer.InputOp{
					Tag:   handlers[i],
					Types: pointer.Move,
				}.Add(&ops)
			}
			var r Router
			r.Frame(&ops)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Queue(
					pointer.Event{
						Type:     pointer.Move,
						Position: f32.Pt(50, 50),
					},
				)
			}
		})
	}
}

var benchAreaOp areaOp

func BenchmarkAreaOp_Decode(b *testing.B) {
	ops := new(op.Ops)
	pointer.Rect(image.Rectangle{Max: image.Pt(100, 100)}).Add(ops)
	for i := 0; i < b.N; i++ {
		benchAreaOp.Decode(ops.Data())
	}
}

func BenchmarkAreaOp_Hit(b *testing.B) {
	ops := new(op.Ops)
	pointer.Rect(image.Rectangle{Max: image.Pt(100, 100)}).Add(ops)
	benchAreaOp.Decode(ops.Data())
	for i := 0; i < b.N; i++ {
		benchAreaOp.Hit(f32.Pt(50, 50))
	}
}
