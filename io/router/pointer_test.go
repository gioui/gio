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
