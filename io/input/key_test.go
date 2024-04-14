// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func TestAllMatchKeyFilter(t *testing.T) {
	r := new(Router)
	r.Event(key.Filter{})
	ke := key.Event{Name: "A"}
	r.Queue(ke)
	// Catch-all gets all non-system events.
	assertEventSequence(t, events(r, -1, key.Filter{}), ke)

	r = new(Router)
	r.Event(key.Filter{Name: "A"})
	r.Queue(SystemEvent{ke})
	if _, handled := r.WakeupTime(); !handled {
		t.Errorf("system event was unexpectedly ignored")
	}
	// Only specific filters match system events.
	assertEventSequence(t, events(r, -1, key.Filter{Name: "A"}), ke)
}

func TestInputHint(t *testing.T) {
	r := new(Router)
	if hint, changed := r.TextInputHint(); hint != key.HintAny || changed {
		t.Fatal("unexpected hint")
	}
	ops := new(op.Ops)
	h := new(int)
	key.InputHintOp{Tag: h, Hint: key.HintEmail}.Add(ops)
	r.Frame(ops)
	if hint, changed := r.TextInputHint(); hint != key.HintAny || changed {
		t.Fatal("unexpected hint")
	}
	r.Source().Execute(key.FocusCmd{Tag: h})
	if hint, changed := r.TextInputHint(); hint != key.HintEmail || !changed {
		t.Fatal("unexpected hint")
	}
}

func TestDeferred(t *testing.T) {
	r := new(Router)
	h := new(int)
	f := []event.Filter{
		key.FocusFilter{Target: h},
		key.Filter{Name: "A"},
	}
	// Provoke deferring by exhausting events for h.
	events(r, -1, f...)
	r.Source().Execute(key.FocusCmd{Tag: h})
	ke := key.Event{Name: "A"}
	r.Queue(ke)
	// All events are deferred at this point.
	assertEventSequence(t, events(r, -1, f...))
	r.Frame(new(op.Ops))
	// But delivered after a frame.
	assertEventSequence(t, events(r, -1, f...), key.FocusEvent{Focus: true}, ke)
}

func TestInputWakeup(t *testing.T) {
	handler := new(int)
	var ops op.Ops
	// InputOps shouldn't trigger redraws.
	event.Op(&ops, handler)

	var r Router
	// Reset events shouldn't either.
	evts := events(&r, -1, key.FocusFilter{Target: new(int)}, key.Filter{Name: "A"})
	assertEventSequence(t, evts, key.FocusEvent{Focus: false})
	r.Frame(&ops)
	if _, wake := r.WakeupTime(); wake {
		t.Errorf("InputOp or the resetting FocusEvent triggered a wakeup")
	}
	// And neither does events that don't match anything.
	r.Queue(key.SnippetEvent{})
	if _, handled := r.WakeupTime(); handled {
		t.Errorf("a not-matching event triggered a wakeup")
	}
	// However, events that does match should trigger wakeup.
	r.Queue(key.Event{Name: "A"})
	if _, handled := r.WakeupTime(); !handled {
		t.Errorf("a key.Event didn't trigger redraw")
	}
}

func TestKeyMultiples(t *testing.T) {
	handlers := make([]int, 3)
	r := new(Router)
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})
	for i := range handlers {
		assertEventSequence(t, events(r, 1, key.FocusFilter{Target: &handlers[i]}), key.FocusEvent{Focus: false})
	}
	r.Source().Execute(key.FocusCmd{Tag: &handlers[2]})
	assertEventSequence(t, events(r, -1, key.FocusFilter{Target: &handlers[2]}), key.FocusEvent{Focus: true})
	assertFocus(t, r, &handlers[2])

	assertKeyboard(t, r, TextInputOpen)
}

func TestKeySoftKeyboardNoFocus(t *testing.T) {
	r := new(Router)

	// It's possible to open the keyboard
	// without any active focus:
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})

	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputOpen)
}

func TestKeyRemoveFocus(t *testing.T) {
	handlers := make([]int, 2)
	r := new(Router)

	filters := func(h event.Tag) []event.Filter {
		return []event.Filter{
			key.FocusFilter{Target: h},
			key.Filter{Focus: h, Name: key.NameTab, Required: key.ModShortcut},
		}
	}
	var all []event.Filter
	for i := range handlers {
		all = append(all, filters(&handlers[i])...)
	}
	assertEventSequence(t, events(r, len(handlers), all...), key.FocusEvent{}, key.FocusEvent{})
	r.Source().Execute(key.FocusCmd{Tag: &handlers[0]})
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})

	evt := key.Event{Name: key.NameTab, Modifiers: key.ModShortcut, State: key.Press}
	r.Queue(evt)

	assertEventSequence(t, events(r, 2, filters(&handlers[0])...), key.FocusEvent{Focus: true}, evt)
	assertFocus(t, r, &handlers[0])
	assertKeyboard(t, r, TextInputOpen)

	// Frame removes focus from tags that don't filter for focus events nor mentioned in an InputOp.
	r.Source().Execute(key.FocusCmd{Tag: new(int)})
	r.Frame(new(op.Ops))

	assertEventSequence(t, events(r, -1, filters(&handlers[1])...))
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)

	// Set focus to InputOp which already
	// exists in the previous frame:
	r.Source().Execute(key.FocusCmd{Tag: &handlers[0]})
	assertFocus(t, r, &handlers[0])
}

func TestKeyFocusedInvisible(t *testing.T) {
	handlers := make([]int, 2)
	ops := new(op.Ops)
	r := new(Router)

	for i := range handlers {
		assertEventSequence(t, events(r, 1, key.FocusFilter{Target: &handlers[i]}), key.FocusEvent{Focus: false})
	}

	// Set new InputOp with focus:
	r.Source().Execute(key.FocusCmd{Tag: &handlers[0]})
	r.Source().Execute(key.SoftKeyboardCmd{Show: true})

	assertEventSequence(t, events(r, 1, key.FocusFilter{Target: &handlers[0]}), key.FocusEvent{Focus: true})
	assertFocus(t, r, &handlers[0])
	assertKeyboard(t, r, TextInputOpen)

	// Frame will clear the focus because the handler is not visible.
	r.Frame(ops)

	for i := range handlers {
		assertEventSequence(t, events(r, -1, key.FocusFilter{Target: &handlers[i]}))
	}
	assertFocus(t, r, nil)
	assertKeyboard(t, r, TextInputClose)

	r.Frame(ops)
	r.Frame(ops)

	ops.Reset()

	// Respawn the first element:
	// It must receive one `Event{Focus: false}`.
	event.Op(ops, &handlers[0])

	assertEventSequence(t, events(r, -1, key.FocusFilter{Target: &handlers[0]}), key.FocusEvent{Focus: false})
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
		event.Op(ops, &handlers[i])
		cl.Pop()
		events(r, -1, key.FocusFilter{Target: &handlers[i]})
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
		key.FocusFilter{Target: h},
		pointer.Filter{
			Target:  h,
			Kinds:   pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -100, Max: +100},
			ScrollY: pointer.ScrollRange{Min: -100, Max: +100},
		},
	}
	events(r, -1, filters...)
	parent := clip.Rect(image.Rect(1, 1, 14, 39)).Push(ops)
	cl := clip.Rect(image.Rect(10, -20, 20, 30)).Push(ops)
	event.Op(ops, h)
	// Test that h is scrolled even if behind another handler.
	event.Op(ops, new(int))
	cl.Pop()
	parent.Pop()
	r.Frame(ops)

	r.MoveFocus(key.FocusLeft)
	r.RevealFocus(image.Rect(0, 0, 15, 40))
	evts := events(r, -1, filters...)
	assertScrollEvent(t, evts[len(evts)-1], f32.Pt(6, -9))
}

func TestFocusClick(t *testing.T) {
	ops := new(op.Ops)
	r := new(Router)
	h := new(int)

	filters := []event.Filter{
		key.FocusFilter{Target: h},
		pointer.Filter{
			Target: h,
			Kinds:  pointer.Press | pointer.Release | pointer.Cancel,
		},
	}
	assertEventPointerTypeSequence(t, events(r, -1, filters...), pointer.Cancel)
	cl := clip.Rect(image.Rect(0, 0, 10, 10)).Push(ops)
	event.Op(ops, h)
	cl.Pop()
	r.Frame(ops)

	r.MoveFocus(key.FocusLeft)
	r.ClickFocus()

	assertEventPointerTypeSequence(t, events(r, -1, filters...), pointer.Press, pointer.Release)
}

func TestNoFocus(t *testing.T) {
	r := new(Router)
	r.MoveFocus(key.FocusForward)
}

func TestKeyRouting(t *testing.T) {
	r := new(Router)
	h := new(int)
	A, B := key.Event{Name: "A"}, key.Event{Name: "B"}
	// Register filters.
	events(r, -1, key.Filter{Name: "A"}, key.Filter{Name: "B"})
	r.Frame(new(op.Ops))
	r.Queue(A, B)
	// The handler is not focused, so only B is delivered.
	assertEventSequence(t, events(r, -1, key.Filter{Focus: h, Name: "A"}, key.Filter{Name: "B"}), B)
	r.Source().Execute(key.FocusCmd{Tag: h})
	// A is delivered to the focused handler.
	assertEventSequence(t, events(r, -1, key.Filter{Focus: h, Name: "A"}, key.Filter{Name: "B"}), A)
}

func assertFocus(t *testing.T, router *Router, expected event.Tag) {
	t.Helper()
	if !router.Source().Focused(expected) {
		t.Errorf("expected %v to be focused", expected)
	}
}

func assertKeyboard(t *testing.T, router *Router, expected TextInputState) {
	t.Helper()
	if got := router.state().state; got != expected {
		t.Errorf("expected %v keyboard, got %v", expected, got)
	}
}
