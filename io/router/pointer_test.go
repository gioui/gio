// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
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
	assertEventSequence(t, r.Events(handler1), pointer.Cancel, pointer.Enter, pointer.Move)
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
	assertEventSequence(t, r.Events(handler1), pointer.Move)
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
	assertEventSequence(t, r.Events(handler1), pointer.Enter, pointer.Release)
	assertEventSequence(t, r.Events(handler2), pointer.Leave)
}

// assertEventSequence ensures that the provided actualEvents match the expected event types
// in the provided order
func assertEventSequence(t *testing.T, actualEvents []event.Event, expected ...pointer.Type) {
	for i, event := range actualEvents {
		pointerEvent, ok := event.(pointer.Event)
		if !ok {
			t.Errorf("actualEvents[%d] is not a pointer event, type %T", i, event)
			continue
		}
		if len(expected) <= i {
			t.Errorf("actualEvents is longer than expected, has len %d, expected len %d", len(actualEvents), len(expected))
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
	const handlerCount = 3
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
}
