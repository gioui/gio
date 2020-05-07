// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

func TestPointerDrag(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	pointer.Rect(image.Rectangle{
		Max: image.Point{
			X: 100,
			Y: 100,
		},
	}).Add(&ops)
	pointer.InputOp{Key: handler}.Add(&ops)

	var r Router
	r.Frame(&ops)
	r.Add(
		// Press.
		pointer.Event{
			Type: pointer.Press,
			Position: f32.Point{
				X: 50,
				Y: 50,
			},
		},
		// Move outside the area.
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 150,
				Y: 150,
			},
		},
	)
	ev := r.Events(handler)
	if moves := countPointerEvents(pointer.Move, ev); moves != 1 {
		t.Errorf("got %d move events, expected 1", moves)
	}
}

func TestPointerMove(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	// Handler 1 area: (0, 0) - (100, 100)
	pointer.Rect(image.Rectangle{
		Max: image.Point{
			X: 100,
			Y: 100,
		},
	}).Add(&ops)
	pointer.InputOp{Key: handler1}.Add(&ops)
	// Handler 2 area: (50, 50) - (100, 100) (areas intersect).
	pointer.Rect(image.Rectangle{
		Min: image.Point{
			X: 50,
			Y: 50,
		},
		Max: image.Point{
			X: 200,
			Y: 200,
		},
	}).Add(&ops)
	pointer.InputOp{Key: handler2}.Add(&ops)

	var r Router
	r.Frame(&ops)
	r.Add(
		// Hit both handlers.
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 50,
				Y: 50,
			},
		},
		// Hit handler 1.
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 49,
				Y: 50,
			},
		},
		// Hit no handlers.
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 100,
				Y: 50,
			},
		},
	)
	ev1 := r.Events(handler1)
	if cancels := countPointerEvents(pointer.Cancel, ev1); cancels != 1 {
		t.Errorf("got %d cancel events, expected 1", cancels)
	}
	ev2 := r.Events(handler2)
	if cancels := countPointerEvents(pointer.Cancel, ev2); cancels != 1 {
		t.Errorf("got %d cancel events, expected 1", cancels)
	}
	if moves := countPointerEvents(pointer.Move, ev1); moves != 2 {
		t.Errorf("got %d move events, expected 2", moves)
	}
	if moves := countPointerEvents(pointer.Move, ev2); moves != 1 {
		t.Errorf("got %d move events, expected 1", moves)
	}
}

func countPointerEvents(typ pointer.Type, events []event.Event) int {
	c := 0
	for _, e := range events {
		if e, ok := e.(pointer.Event); ok {
			if e.Type == typ {
				c++
			}
		}
	}
	return c
}

func TestPointerEnterLeave(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	var stack op.StackOp
	stack.Push(&ops)
	// Handler 1 area: (0, 0) - (100, 100)
	pointer.Rect(image.Rectangle{
		Max: image.Point{
			X: 100,
			Y: 100,
		},
	}).Add(&ops)
	pointer.InputOp{Key: handler1}.Add(&ops)
	stack.Pop()

	// Handler 2 area: (50, 50) - (100, 100) (areas intersect).
	stack.Push(&ops)
	pointer.Rect(image.Rectangle{
		Min: image.Point{
			X: 50,
			Y: 50,
		},
		Max: image.Point{
			X: 200,
			Y: 200,
		},
	}).Add(&ops)
	pointer.InputOp{Key: handler2}.Add(&ops)
	stack.Pop()

	var r Router
	r.Frame(&ops)
	// Hit both handlers.
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 50,
				Y: 50,
			},
		},
	)
	// First event for a handler is always a Cancel.
	// Only handler2 should receive the enter/move events because it is on top
	// and handler1 is not an ancestor in the hit tree.
	assertEventSequence(t, r.Events(handler1), pointer.Cancel)
	assertEventSequence(t, r.Events(handler2), pointer.Cancel, pointer.Enter, pointer.Move)

	// Leave the second area by moving into the first.
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 45,
				Y: 45,
			},
		},
	)
	// The cursor leaves handler2 and enters handler1.
	assertEventSequence(t, r.Events(handler1), pointer.Enter, pointer.Move)
	assertEventSequence(t, r.Events(handler2), pointer.Leave)

	// Move, but stay within the same hit area.
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 40,
				Y: 40,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Move)
	assertEventSequence(t, r.Events(handler2))

	// Move outside of both inputs.
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 300,
				Y: 300,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Leave)
	assertEventSequence(t, r.Events(handler2))

	// Check that a Press event generates Enter Events.
	r.Add(
		pointer.Event{
			Type: pointer.Press,
			Position: f32.Point{
				X: 125,
				Y: 125,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1))
	assertEventSequence(t, r.Events(handler2), pointer.Enter, pointer.Press)

	// Check that a Release event generates Enter/Leave Events.
	r.Add(
		pointer.Event{
			Type: pointer.Release,
			Position: f32.Point{
				// Move out of the second hit area and into the first.
				X: 25,
				Y: 25,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Enter)
	// The second handler gets the release event because the press started inside it.
	assertEventSequence(t, r.Events(handler2), pointer.Leave, pointer.Release)

}

func TestPointerEnterLeaveNested(t *testing.T) {
	handler1 := new(int)
	handler2 := new(int)
	var ops op.Ops

	// Handler 1 area: (0, 0) - (100, 100)
	pointer.Rect(image.Rectangle{
		Max: image.Point{
			X: 100,
			Y: 100,
		},
	}).Add(&ops)
	pointer.InputOp{Key: handler1}.Add(&ops)

	// Handler 2 area: (25, 25) - (75, 75) (nested within first).
	pointer.Rect(image.Rectangle{
		Min: image.Point{
			X: 25,
			Y: 25,
		},
		Max: image.Point{
			X: 75,
			Y: 75,
		},
	}).Add(&ops)
	pointer.InputOp{Key: handler2}.Add(&ops)

	var r Router
	r.Frame(&ops)
	// Hit both handlers.
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 50,
				Y: 50,
			},
		},
	)
	// First event for a handler is always a Cancel.
	// Both handlers should receive the Enter and Move events because handler2 is a child of handler1.
	assertEventSequence(t, r.Events(handler1), pointer.Cancel, pointer.Enter, pointer.Move)
	assertEventSequence(t, r.Events(handler2), pointer.Cancel, pointer.Enter, pointer.Move)

	// Leave the second area by moving into the first.
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 20,
				Y: 20,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Move)
	assertEventSequence(t, r.Events(handler2), pointer.Leave)

	// Move, but stay within the same hit area.
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 10,
				Y: 10,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Move)
	assertEventSequence(t, r.Events(handler2))

	// Move outside of both inputs.
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 200,
				Y: 200,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Leave)
	assertEventSequence(t, r.Events(handler2))

	// Check that a Press event generates Enter Events.
	r.Add(
		pointer.Event{
			Type: pointer.Press,
			Position: f32.Point{
				X: 50,
				Y: 50,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Enter, pointer.Press)
	assertEventSequence(t, r.Events(handler2), pointer.Enter, pointer.Press)

	// Check that a Release event generates Enter/Leave Events.
	r.Add(
		pointer.Event{
			Type: pointer.Release,
			Position: f32.Point{
				// Move out of the second hit area and into the first.
				X: 20,
				Y: 20,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Release)
	assertEventSequence(t, r.Events(handler2), pointer.Leave, pointer.Release)
}

func TestPointerActiveInputDisappears(t *testing.T) {
	handler1 := new(int)
	// Save this logic so we can redo it later.
	renderHandler1 := func(ops *op.Ops) {
		var stack op.StackOp
		stack.Push(ops)
		// Handler 1 area: (0, 0) - (100, 100)
		pointer.Rect(image.Rectangle{
			Max: image.Point{
				X: 100,
				Y: 100,
			},
		}).Add(ops)
		pointer.InputOp{Key: handler1}.Add(ops)
		stack.Pop()
	}

	var ops op.Ops
	var r Router

	renderHandler1(&ops)

	// Draw handler.
	ops.Reset()
	renderHandler1(&ops)
	r.Frame(&ops)
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 25,
				Y: 25,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Cancel, pointer.Enter, pointer.Move)

	// Re-render with handler missing.
	ops.Reset()
	r.Frame(&ops)
	r.Add(
		pointer.Event{
			Type: pointer.Move,
			Position: f32.Point{
				X: 25,
				Y: 25,
			},
		},
	)
	assertEventSequence(t, r.Events(handler1), pointer.Cancel)
}

// toTypes converts a sequence of event.Event to their pointer.Types. It assumes
// that all input events are of underlying type pointer.Event, and thus will
// panic if some are not.
func toTypes(events []event.Event) []pointer.Type {
	out := make([]pointer.Type, len(events))
	for i, event := range events {
		out[i] = event.(pointer.Event).Type
	}
	return out
}

// assertEventSequence ensures that the provided actualEvents match the expected event types
// in the provided order
func assertEventSequence(t *testing.T, actualEvents []event.Event, expected ...pointer.Type) {
	if len(actualEvents) != len(expected) {
		t.Errorf("expected %v events, got %v", expected, toTypes(actualEvents))
	}
	for i, event := range actualEvents {
		pointerEvent, ok := event.(pointer.Event)
		if !ok {
			t.Errorf("actualEvents[%d] is not a pointer event, type %T", i, event)
			continue
		}
		if len(expected) <= i {
			continue
		}
		if pointerEvent.Type != expected[i] {
			t.Errorf("actualEvents[%d] has type %s, expected %s", i, pointerEvent.Type.String(), expected[i].String())
			continue
		}
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
			handlers := make([]event.Key, handlerCount)
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
				pointer.InputOp{Key: handlers[i]}.Add(&ops)
			}
			var r Router
			r.Frame(&ops)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.Add(
					pointer.Event{
						Type: pointer.Move,
						Position: f32.Point{
							X: 50,
							Y: 50,
						},
					},
				)
			}
		})
	}
}
