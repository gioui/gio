// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import (
	"testing"
	"time"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

func TestOHOSKeyNamesExtended(t *testing.T) {
	cases := []struct {
		code int
		want key.Name
	}{
		{ohosKeyNumpadDivide, "/"},
		{ohosKeyNumpadMul, "*"},
		{ohosKeyNumpadSub, "-"},
		{ohosKeyNumpadAdd, "+"},
		{ohosKeyNumpadDot, "."},
		{ohosKeyNumpadComma, ","},
		{ohosKeyNumpadEquals, "="},
		{ohosKeyNumpadLParen, "("},
		{ohosKeyNumpadRParen, ")"},
		{ohosKeyCapsLock, "CapsLock"},
		{ohosKeyScrollLock, "ScrollLock"},
		{ohosKeyNumLock, "NumLock"},
		{ohosKeyMenu, "Menu"},
		{ohosKeyHelp, "Help"},
		{ohosKeyPrint, "PrintScreen"},
	}
	for _, c := range cases {
		got, ok := ohosKeyName(c.code)
		if !ok || got != c.want {
			t.Errorf("ohosKeyName(%d) = %q,%v; want %q", c.code, got, ok, c.want)
		}
	}
	if got := ohosModifiers(ohosModMeta); got != key.ModSuper {
		t.Fatalf("meta = %v", got)
	}
}

func TestOHOSPrintableNumpad(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{ohosKeyNumpadDivide, "/"},
		{ohosKeyNumpadMul, "*"},
		{ohosKeyNumpadSub, "-"},
		{ohosKeyNumpadAdd, "+"},
		{ohosKeyNumpadDot, "."},
		{ohosKeyNumpadComma, ","},
		{ohosKeyNumpadEquals, "="},
		{ohosKeyNumpadLParen, "("},
		{ohosKeyNumpadRParen, ")"},
	}
	for _, c := range cases {
		got, ok := ohosPrintable(c.code, 0)
		if !ok || got != c.want {
			t.Errorf("ohosPrintable(%d) = %q,%v; want %q", c.code, got, ok, c.want)
		}
	}
	if _, ok := ohosPrintable(ohosKeyA, ohosModMeta); ok {
		t.Fatal("meta chord must not insert text")
	}
}

func TestOHOSCursorMapping(t *testing.T) {
	cases := []struct {
		cursor pointer.Cursor
		want   int
	}{
		{pointer.CursorDefault, 0},
		{pointer.CursorNone, -1},
		{pointer.CursorText, 1},
		{pointer.CursorPointer, 2},
		{pointer.CursorCrosshair, 3},
		{pointer.CursorAllScroll, 10},
		{pointer.CursorNorthWestResize, 11},
		{pointer.CursorNorthEastResize, 12},
		{pointer.CursorSouthWestResize, 13},
		{pointer.CursorSouthEastResize, 14},
		{pointer.CursorNorthEastSouthWestResize, 15},
		{pointer.CursorNorthWestSouthEastResize, 16},
	}
	for _, c := range cases {
		if got := ohosCursorNumber(c.cursor); got != c.want {
			t.Errorf("cursor(%v) = %d; want %d", c.cursor, got, c.want)
		}
	}
}

func TestOHOSMouseButtons(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.events = nil

	p.Handle(ohosEvent{kind: ohosEvMouse, action: 0, code: 1, x: 1, y: 1})
	p.Handle(ohosEvent{kind: ohosEvMouse, action: 2, x: 2, y: 2})
	p.Handle(ohosEvent{kind: ohosEvMouse, action: 0, code: 2, x: 3, y: 3})
	p.Handle(ohosEvent{kind: ohosEvMouse, action: 1, code: 1, x: 4, y: 4})
	p.Handle(ohosEvent{kind: ohosEvMouse, action: 1, code: 2, x: 5, y: 5})
	if len(f.events) != 5 {
		t.Fatalf("events = %d", len(f.events))
	}
	press := f.events[0].(pointer.Event)
	if press.Buttons != pointer.ButtonPrimary {
		t.Fatalf("press = %v", press.Buttons)
	}
	move := f.events[1].(pointer.Event)
	if move.Buttons != pointer.ButtonPrimary {
		t.Fatalf("move = %v", move.Buttons)
	}
	both := f.events[2].(pointer.Event)
	if both.Buttons != pointer.ButtonPrimary|pointer.ButtonSecondary {
		t.Fatalf("both = %v", both.Buttons)
	}
	release := f.events[3].(pointer.Event)
	if release.Buttons != pointer.ButtonSecondary {
		t.Fatalf("release left = %v", release.Buttons)
	}
	last := f.events[4].(pointer.Event)
	if last.Buttons != 0 {
		t.Fatalf("release all = %v", last.Buttons)
	}
}

func TestOHOSPinchSuppressionWindow(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvTouch, action: 0, id: 1, x: 1, y: 1})
	p.axis(ohosEvent{scale: 1.2, x: 1, y: 1})
	if len(f.events) != 1 {
		t.Fatalf("pinch after touch must be suppressed: %#v", f.events)
	}
	// After the suppression window and with no active touches, a touchpad
	// pinch is accepted again.
	p.Handle(ohosEvent{kind: ohosEvTouch, action: 2, id: 1, x: 1, y: 1})
	p.lastTouch = time.Now().Add(-time.Second)
	f.events = nil
	p.axis(ohosEvent{scale: 1.2, x: 1, y: 1})
	p.axis(ohosEvent{scale: 1.44, x: 1, y: 1})
	if len(f.events) != 1 {
		t.Fatalf("touchpad pinch must resume: %#v", f.events)
	}
}

func TestOHOSTextDedupe(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.inserted = nil
	p.Handle(ohosEvent{kind: ohosEvKey, code: ohosKeyA, action: 0})
	if len(f.inserted) != 1 || f.inserted[0] != "a" {
		t.Fatalf("inserted = %#v", f.inserted)
	}
	p.Handle(ohosEvent{kind: ohosEvText, text: "a"})
	if len(f.inserted) != 1 {
		t.Fatalf("duplicate text must be dropped: %#v", f.inserted)
	}
	p.Handle(ohosEvent{kind: ohosEvText, text: "b"})
	if len(f.inserted) != 2 {
		t.Fatalf("different text must be inserted: %#v", f.inserted)
	}
}
