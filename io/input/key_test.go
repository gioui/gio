// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"reflect"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func TestInputWakeup(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	event.InputOp(&ops, handler)

	var r Router
	// Test that merely adding a handler doesn't trigger redraw.
	evts := r.Events(handler, key.FocusFilter{})
	r.Frame(&ops)
	if _, wake := r.WakeupTime(); wake {
		t.Errorf("adding key.InputOp triggered a redraw")
	}
	if len(evts) != 1 {
		t.Errorf("no Focus event for newly registered key.InputOp")
	}
}

func TestKeyMultiples(t *testing.T) {
	handlers := make([]int, 3)
	ops := new(op.Ops)
	r := new(Router)

	r.Source().Execute(key.SoftKeyboardCmd{Show: true})
	event.InputOp(ops, &handlers[0])
	event.InputOp(ops, &handlers[1])

	// The last one must be focused:
	event.InputOp(ops, &handlers[2])

	for i := range handlers {
		assertKeyEvent(t, r.Events(&handlers[i], key.FocusFilter{}), false)
	}

	r.Frame(ops)

	r.Source().Execute(key.FocusCmd{Tag: &handlers[2]})
	assertKeyEvent(t, r.Events(&handlers[2], key.FocusFilter{}), true)
	assertFocus(t, r, &handlers[2])

	assertKeyboard(t, r, TextInputOpen)
}

func TestKeyStacked(t *testing.T) {
	handlers := make([]int, 4)
	ops := new(op.Ops)
	r := new(Router)

	event.InputOp(ops, &handlers[0])
	r.Source().Execute(key.FocusCmd{})
	r.Source().Execute(key.SoftKeyboardCmd{Show: false})
	event.InputOp(ops, &handlers[1])
	r.Source().Execute(key.FocusCmd{Tag: &handlers[1]})
	event.InputOp(ops, &handlers[2])
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})
	event.InputOp(ops, &handlers[3])

	for i := range handlers {
		assertKeyEvent(t, r.Events(&handlers[i], key.FocusFilter{}), false)
	}

	r.Frame(ops)

	assertKeyEvent(t, r.Events(&handlers[1], key.FocusFilter{}), true)
	assertFocus(t, r, &handlers[1])
	assertKeyboard(t, r, TextInputOpen)
}

func TestKeySoftKeyboardNoFocus(t *testing.T) {
	ops := new(op.Ops)
	r := new(Router)

	// It's possible to open the keyboard
	// without any active focus:
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})

	r.Frame(ops)

	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputOpen)
}

func TestKeyRemoveFocus(t *testing.T) {
	handlers := make([]int, 2)
	ops := new(op.Ops)
	r := new(Router)

	// New InputOp with Focus and Keyboard:
	event.InputOp(ops, &handlers[0])
	r.Source().Execute(key.FocusCmd{Tag: &handlers[0]})
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})

	// New InputOp without any focus:
	event.InputOp(ops, &handlers[1])

	filters := []event.Filter{
		key.FocusFilter{},
		key.Filter{Name: key.NameTab, Required: key.ModShortcut},
	}
	for i := range handlers {
		assertKeyEvent(t, r.Events(&handlers[i], filters...), false)
	}

	r.Frame(ops)

	// Add some key events:
	evt := event.Event(key.Event{Name: key.NameTab, Modifiers: key.ModShortcut, State: key.Press})
	r.Queue(evt)

	assertKeyEvent(t, r.Events(&handlers[0], filters...), true, evt)
	assertFocus(t, r, &handlers[0])
	assertKeyboard(t, r, TextInputOpen)

	ops.Reset()

	// Will get the focus removed:
	event.InputOp(ops, &handlers[0])

	// Unchanged:
	event.InputOp(ops, &handlers[1])

	// Remove focus by focusing on a tag that don't exist.
	r.Source().Execute(key.FocusCmd{Tag: new(int)})

	r.Frame(ops)

	assertKeyEventUnexpected(t, r.Events(&handlers[1], key.FocusFilter{}))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)

	ops.Reset()

	event.InputOp(ops, &handlers[0])
	event.InputOp(ops, &handlers[1])

	assertKeyEventUnexpected(t, r.Events(&handlers[0], key.FocusFilter{}))
	assertKeyEventUnexpected(t, r.Events(&handlers[1], key.FocusFilter{}))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)

	r.Frame(ops)
	ops.Reset()

	// Set focus to InputOp which already
	// exists in the previous frame:
	r.Source().Execute(key.FocusCmd{Tag: &handlers[0]})
	event.InputOp(ops, &handlers[0])
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})
	assertFocus(t, r, &handlers[0])

	ops.Reset()

	// Remove focus.
	event.InputOp(ops, &handlers[1])
	r.Source().Execute(key.FocusCmd{})

	assertKeyEventUnexpected(t, r.Events(&handlers[1], key.FocusFilter{}))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)
}

func TestKeyFocusedInvisible(t *testing.T) {
	handlers := make([]int, 2)
	ops := new(op.Ops)
	r := new(Router)

	// Set new InputOp with focus:
	r.Source().Execute(key.FocusCmd{Tag: &handlers[0]})
	event.InputOp(ops, &handlers[0])
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})

	// Set new InputOp without focus:
	event.InputOp(ops, &handlers[1])

	for i := range handlers {
		assertKeyEvent(t, r.Events(&handlers[i], key.FocusFilter{}), false)
	}

	r.Frame(ops)

	assertKeyEvent(t, r.Events(&handlers[0], key.FocusFilter{}), true)
	assertFocus(t, r, &handlers[0])
	assertKeyboard(t, r, TextInputOpen)

	ops.Reset()

	//
	// Removed first (focused) element!
	//

	// Unchanged:
	event.InputOp(ops, &handlers[1])

	r.Frame(ops)

	assertKeyEventUnexpected(t, r.Events(&handlers[0], key.FocusFilter{}))
	assertKeyEventUnexpected(t, r.Events(&handlers[1], key.FocusFilter{}))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)

	r.Frame(ops)

	// Unchanged
	event.InputOp(ops, &handlers[1])

	r.Frame(ops)

	ops.Reset()

	// Respawn the first element:
	// It must receive one `Event{Focus: false}`.
	event.InputOp(ops, &handlers[0])

	// Unchanged
	event.InputOp(ops, &handlers[1])

	for i := range handlers {
		assertKeyEventUnexpected(t, r.Events(&handlers[i], key.FocusFilter{}))
	}

	r.Frame(ops)

	assertKeyEventUnexpected(t, r.Events(&handlers[1], key.FocusFilter{}))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)
}

func TestNoOps(t *testing.T) {
	r := new(Router)
	r.Frame(nil)
}

func TestDirectionalFocus(t *testing.T) {
	ops := new(op.Ops)
	r := new(Router)
	handlers := []image.Rectangle{
		image.Rect(10, 10, 50, 50),
		image.Rect(50, 20, 100, 80),
		image.Rect(20, 26, 60, 80),
		image.Rect(10, 60, 50, 100),
	}

	for i, bounds := range handlers {
		cl := clip.Rect(bounds).Push(ops)
		event.InputOp(ops, &handlers[i])
		cl.Pop()
		r.Events(&handlers[i], key.FocusFilter{})
	}
	r.Frame(ops)

	r.MoveFocus(key.FocusLeft)
	assertFocus(t, r, &handlers[0])
	r.MoveFocus(key.FocusLeft)
	assertFocus(t, r, &handlers[0])
	r.MoveFocus(key.FocusRight)
	assertFocus(t, r, &handlers[1])
	r.MoveFocus(key.FocusRight)
	assertFocus(t, r, &handlers[1])
	r.MoveFocus(key.FocusDown)
	assertFocus(t, r, &handlers[2])
	r.MoveFocus(key.FocusDown)
	assertFocus(t, r, &handlers[2])
	r.MoveFocus(key.FocusLeft)
	assertFocus(t, r, &handlers[3])
	r.MoveFocus(key.FocusUp)
	assertFocus(t, r, &handlers[0])

	r.MoveFocus(key.FocusForward)
	assertFocus(t, r, &handlers[1])
	r.MoveFocus(key.FocusBackward)
	assertFocus(t, r, &handlers[0])
}

func TestFocusScroll(t *testing.T) {
	ops := new(op.Ops)
	r := new(Router)
	h := new(int)

	filters := []event.Filter{
		key.FocusFilter{},
		pointer.Filter{
			Kinds:        pointer.Scroll,
			ScrollBounds: image.Rect(-100, -100, 100, 100),
		},
	}
	r.Events(h, filters...)
	parent := clip.Rect(image.Rect(1, 1, 14, 39)).Push(ops)
	cl := clip.Rect(image.Rect(10, -20, 20, 30)).Push(ops)
	event.InputOp(ops, h)
	// Test that h is scrolled even if behind another handler.
	event.InputOp(ops, new(int))
	cl.Pop()
	parent.Pop()
	r.Frame(ops)

	r.MoveFocus(key.FocusLeft)
	r.RevealFocus(image.Rect(0, 0, 15, 40))
	evts := r.Events(h, filters...)
	assertScrollEvent(t, evts[len(evts)-1], f32.Pt(6, -9))
}

func TestFocusClick(t *testing.T) {
	ops := new(op.Ops)
	r := new(Router)
	h := new(int)

	filters := []event.Filter{
		key.FocusFilter{},
		pointer.Filter{
			Kinds: pointer.Press | pointer.Release,
		},
	}
	assertEventPointerTypeSequence(t, r.Events(h, filters...), pointer.Cancel)
	cl := clip.Rect(image.Rect(0, 0, 10, 10)).Push(ops)
	event.InputOp(ops, h)
	cl.Pop()
	r.Frame(ops)

	r.MoveFocus(key.FocusLeft)
	r.ClickFocus()

	assertEventPointerTypeSequence(t, r.Events(h, filters...), pointer.Press, pointer.Release)
}

func TestNoFocus(t *testing.T) {
	r := new(Router)
	r.MoveFocus(key.FocusForward)
}

func TestKeyRouting(t *testing.T) {
	handlers := make([]int, 5)
	ops := new(op.Ops)
	macroOps := new(op.Ops)
	r := new(Router)

	rect := clip.Rect{Max: image.Pt(10, 10)}

	macro := op.Record(macroOps)
	event.InputOp(ops, &handlers[0])
	cl1 := rect.Push(ops)
	event.InputOp(ops, &handlers[1])
	event.InputOp(ops, &handlers[2])
	cl1.Pop()
	cl2 := rect.Push(ops)
	event.InputOp(ops, &handlers[3])
	event.InputOp(ops, &handlers[4])
	cl2.Pop()
	call := macro.Stop()
	call.Add(ops)

	fa := []event.Filter{
		key.FocusFilter{},
		key.Filter{Name: "A"},
	}
	fb := []event.Filter{
		key.FocusFilter{},
		key.Filter{Name: "B"},
	}
	r.Events(&handlers[0], fa...)
	r.Events(&handlers[1], fb...)
	r.Events(&handlers[2], fa...)
	r.Events(&handlers[3], key.FocusFilter{})
	r.Events(&handlers[4], fa...)

	r.Frame(ops)

	A, B := key.Event{Name: "A"}, key.Event{Name: "B"}
	r.Queue(A, B)

	// With no focus, the events should traverse the final branch of the hit tree
	// searching for handlers.
	if evts := r.Events(&handlers[4], fa...); len(evts) != 1 || evts[0] != A {
		t.Errorf("expected key event")
	}
	r.Events(&handlers[3], key.FocusFilter{})
	r.Events(&handlers[2], fa...)
	if evts := r.Events(&handlers[1], fb...); len(evts) != 1 || evts[0] != B {
		t.Errorf("expected key event")
	}
	r.Events(&handlers[0], fa...)

	r2 := new(Router)

	r2.Events(&handlers[0], fa...)
	r2.Events(&handlers[1], fb...)
	r2.Events(&handlers[2], fa...)
	r2.Events(&handlers[3], key.FocusFilter{})
	r2.Events(&handlers[4], fa...)

	r2.Source().Execute(key.FocusCmd{Tag: &handlers[3]})
	r2.Frame(ops)

	r2.Queue(A, B)

	// With focus, the events should traverse the branch of the hit tree
	// containing the focused element.
	assertKeyEvent(t, r2.Events(&handlers[3], key.FocusFilter{}), true)
	if evts := r2.Events(&handlers[0], fa...); len(evts) != 1 || evts[0] != A {
		t.Errorf("expected key event")
	}
}

func assertKeyEvent(t *testing.T, events []event.Event, expectedFocus bool, expectedInputs ...event.Event) {
	t.Helper()
	var evtFocus int
	var evtKeyPress int
	for _, e := range events {
		switch ev := e.(type) {
		case key.FocusEvent:
			if ev.Focus != expectedFocus {
				t.Errorf("focus is expected to be %v, got %v", expectedFocus, ev.Focus)
			}
			evtFocus++
		case key.Event, key.EditEvent:
			if len(expectedInputs) <= evtKeyPress {
				t.Fatalf("unexpected key events")
			}
			if !reflect.DeepEqual(ev, expectedInputs[evtKeyPress]) {
				t.Errorf("expected %v events, got %v", expectedInputs[evtKeyPress], ev)
			}
			evtKeyPress++
		}
	}
	if evtFocus <= 0 {
		t.Errorf("expected focus event")
	}
	if evtFocus > 1 {
		t.Errorf("expected single focus event")
	}
	if evtKeyPress != len(expectedInputs) {
		t.Errorf("expected key events")
	}
}

func assertKeyEventUnexpected(t *testing.T, events []event.Event) {
	t.Helper()
	var evtFocus int
	for _, e := range events {
		switch e.(type) {
		case key.FocusEvent:
			evtFocus++
		}
	}
	if evtFocus > 1 {
		t.Errorf("unexpected focus event")
	}
}

func assertFocus(t *testing.T, router *Router, expected event.Tag) {
	t.Helper()
	if got := router.lastState().focus; got != expected {
		t.Errorf("expected %v to be focused, got %v", expected, got)
	}
}

func assertKeyboard(t *testing.T, router *Router, expected TextInputState) {
	t.Helper()
	if got := router.lastState().state; got != expected {
		t.Errorf("expected %v keyboard, got %v", expected, got)
	}
}
