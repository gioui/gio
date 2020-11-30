// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
	"reflect"
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/op"
)

func TestKeyMultiples(t *testing.T) {
	handlers := make([]int, 3)
	ops := new(op.Ops)
	r := new(Router)

	key.SoftKeyboardOp{Show: true}.Add(ops)
	key.InputOp{Tag: &handlers[0]}.Add(ops)
	key.FocusOp{Focus: true}.Add(ops)
	key.InputOp{Tag: &handlers[1]}.Add(ops)

	// The last one must be focused:
	key.InputOp{Tag: &handlers[2]}.Add(ops)

	r.Frame(ops)

	assertKeyEvent(t, r.Events(&handlers[0]), false)
	assertKeyEvent(t, r.Events(&handlers[1]), false)
	assertKeyEvent(t, r.Events(&handlers[2]), true)
	assertFocus(t, r, &handlers[2])
	assertKeyboard(t, r, TextInputOpen)
}

func TestKeyStacked(t *testing.T) {
	handlers := make([]int, 4)
	ops := new(op.Ops)
	r := new(Router)

	s := op.Push(ops)
	key.InputOp{Tag: &handlers[0]}.Add(ops)
	// FocusOp must not overwrite the
	// FocusOp{Focus: true}.
	key.FocusOp{Focus: false}.Add(ops)
	s.Pop()
	s = op.Push(ops)
	key.SoftKeyboardOp{Show: false}.Add(ops)
	key.InputOp{Tag: &handlers[1]}.Add(ops)
	key.FocusOp{Focus: true}.Add(ops)
	s.Pop()
	s = op.Push(ops)
	key.InputOp{Tag: &handlers[2]}.Add(ops)
	// SoftwareKeyboardOp will open the keyboard,
	// overwriting `SoftKeyboardOp{Show: false}`.
	key.SoftKeyboardOp{Show: true}.Add(ops)
	s.Pop()
	s = op.Push(ops)
	key.SoftKeyboardOp{Show: false}.Add(ops)
	key.InputOp{Tag: &handlers[3]}.Add(ops)
	// FocusOp must not overwrite the
	// FocusOp{Focus: true}.
	key.FocusOp{Focus: false}.Add(ops)
	s.Pop()

	r.Frame(ops)

	assertKeyEvent(t, r.Events(&handlers[0]), false)
	assertKeyEvent(t, r.Events(&handlers[1]), true)
	assertKeyEvent(t, r.Events(&handlers[2]), false)
	assertKeyEvent(t, r.Events(&handlers[3]), false)
	assertFocus(t, r, &handlers[1])
	assertKeyboard(t, r, TextInputOpen)
}

func TestKeySoftKeyboardNoFocus(t *testing.T) {
	ops := new(op.Ops)
	r := new(Router)

	// It's possible to open the keyboard
	// without any active focus:
	key.SoftKeyboardOp{Show: true}.Add(ops)

	r.Frame(ops)

	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputOpen)
}

func TestKeyRemoveFocus(t *testing.T) {
	handlers := make([]int, 2)
	ops := new(op.Ops)
	r := new(Router)

	// New InputOp with Focus and Keyboard:
	s := op.Push(ops)
	key.InputOp{Tag: &handlers[0]}.Add(ops)
	key.FocusOp{Focus: true}.Add(ops)
	key.SoftKeyboardOp{Show: true}.Add(ops)
	s.Pop()

	// New InputOp without any focus:
	s = op.Push(ops)
	key.InputOp{Tag: &handlers[1]}.Add(ops)
	s.Pop()

	r.Frame(ops)

	// Add some key events:
	event := event.Event(key.Event{Name: key.NameTab, Modifiers: key.ModShortcut, State: key.Press})
	r.Add(event)

	assertKeyEvent(t, r.Events(&handlers[0]), true, event)
	assertKeyEvent(t, r.Events(&handlers[1]), false)
	assertFocus(t, r, &handlers[0])
	assertKeyboard(t, r, TextInputOpen)

	ops.Reset()

	// Will get the focus removed:
	s = op.Push(ops)
	key.InputOp{Tag: &handlers[0]}.Add(ops)
	s.Pop()

	// Unchanged:
	s = op.Push(ops)
	key.InputOp{Tag: &handlers[1]}.Add(ops)
	s.Pop()

	// Removing any Focus:
	s = op.Push(ops)
	key.FocusOp{Focus: false}.Add(ops)
	s.Pop()

	r.Frame(ops)

	assertKeyEvent(t, r.Events(&handlers[0]), false)
	assertKeyEventUnexpected(t, r.Events(&handlers[1]))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)

	ops.Reset()

	s = op.Push(ops)
	key.InputOp{Tag: &handlers[0]}.Add(ops)
	s.Pop()

	// Setting Focus without InputOp:
	s = op.Push(ops)
	key.FocusOp{Focus: true}.Add(ops)
	s.Pop()

	s = op.Push(ops)
	key.InputOp{Tag: &handlers[1]}.Add(ops)
	s.Pop()

	r.Frame(ops)

	assertKeyEventUnexpected(t, r.Events(&handlers[0]))
	assertKeyEventUnexpected(t, r.Events(&handlers[1]))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputKeep)

	ops.Reset()

	// Set focus to InputOp which already
	// exists in the previous frame:
	s = op.Push(ops)
	key.FocusOp{Focus: true}.Add(ops)
	key.InputOp{Tag: &handlers[0]}.Add(ops)
	key.SoftKeyboardOp{Show: true}.Add(ops)
	s.Pop()

	// Tries to remove focus:
	// It must not overwrite the previous `FocusOp`.
	s = op.Push(ops)
	key.InputOp{Tag: &handlers[1]}.Add(ops)
	key.FocusOp{Focus: false}.Add(ops)
	s.Pop()

	r.Frame(ops)

	assertKeyEvent(t, r.Events(&handlers[0]), true)
	assertKeyEventUnexpected(t, r.Events(&handlers[1]))
	assertFocus(t, r, &handlers[0])
	assertKeyboard(t, r, TextInputOpen)
}

func TestKeyFocusedInvisible(t *testing.T) {
	handlers := make([]int, 2)
	ops := new(op.Ops)
	r := new(Router)

	// Set new InputOp with focus:
	s := op.Push(ops)
	key.FocusOp{Focus: true}.Add(ops)
	key.InputOp{Tag: &handlers[0]}.Add(ops)
	key.SoftKeyboardOp{Show: true}.Add(ops)
	s.Pop()

	// Set new InputOp without focus:
	s = op.Push(ops)
	key.InputOp{Tag: &handlers[1]}.Add(ops)
	s.Pop()

	r.Frame(ops)

	assertKeyEvent(t, r.Events(&handlers[0]), true)
	assertKeyEvent(t, r.Events(&handlers[1]), false)
	assertFocus(t, r, &handlers[0])
	assertKeyboard(t, r, TextInputOpen)

	ops.Reset()

	//
	// Removed first (focused) element!
	//

	// Unchanged:
	s = op.Push(ops)
	key.InputOp{Tag: &handlers[1]}.Add(ops)
	s.Pop()

	r.Frame(ops)

	assertKeyEventUnexpected(t, r.Events(&handlers[0]))
	assertKeyEventUnexpected(t, r.Events(&handlers[1]))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)

	ops.Reset()

	// Respawn the first element:
	// It must receive one `Event{Focus: false}`.
	s = op.Push(ops)
	key.InputOp{Tag: &handlers[0]}.Add(ops)
	s.Pop()

	// Unchanged
	s = op.Push(ops)
	key.InputOp{Tag: &handlers[1]}.Add(ops)
	s.Pop()

	r.Frame(ops)

	assertKeyEvent(t, r.Events(&handlers[0]), false)
	assertKeyEventUnexpected(t, r.Events(&handlers[1]))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputKeep)

}

func assertKeyEvent(t *testing.T, events []event.Event, expected bool, expectedInputs ...event.Event) {
	t.Helper()
	var evtFocus int
	var evtKeyPress int
	for _, e := range events {
		switch ev := e.(type) {
		case key.FocusEvent:
			if ev.Focus != expected {
				t.Errorf("focus is expected to be %v, got %v", expected, ev.Focus)
			}
			evtFocus++
		case key.Event, key.EditEvent:
			if len(expectedInputs) <= evtKeyPress {
				t.Errorf("unexpected key events")
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
	if router.kqueue.focus != expected {
		t.Errorf("expected %v to be focused, got %v", expected, router.kqueue.focus)
	}
}

func assertKeyboard(t *testing.T, router *Router, expected TextInputState) {
	t.Helper()
	if router.kqueue.state != expected {
		t.Errorf("expected %v keyboard, got %v", expected, router.kqueue.state)
	}
}
