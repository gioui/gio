// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import (
	"image"
	"io"
	"testing"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/unit"
)

type fakeSink struct {
	events   []event.Event
	frames   int
	inserted []string
	pread    []string
	deleted  [][2]int
	moved    []int
	sel      [][2]int
	enter    []int
	imeDirty int
	picks    []string
	pickErr  string
	logs     []string
}

func (f *fakeSink) ohosDeliver(e event.Event)              { f.events = append(f.events, e) }
func (f *fakeSink) ohosRequestFrame()                      { f.frames++ }
func (f *fakeSink) ohosEditorInsert(s string)              { f.inserted = append(f.inserted, s) }
func (f *fakeSink) ohosEditorPreedit(s string, a, b int)   { f.pread = append(f.pread, s) }
func (f *fakeSink) ohosEditorDelete(a, b int)              { f.deleted = append(f.deleted, [2]int{a, b}) }
func (f *fakeSink) ohosEditorMove(d int)                   { f.moved = append(f.moved, d) }
func (f *fakeSink) ohosEditorSelect(a, b int)              { f.sel = append(f.sel, [2]int{a, b}) }
func (f *fakeSink) ohosEditorEnter(t int)                  { f.enter = append(f.enter, t) }
func (f *fakeSink) ohosIMEChanged()                        { f.imeDirty++ }
func (f *fakeSink) ohosPickResult(mode int, uris []string) { f.picks = append(f.picks, uris...) }
func (f *fakeSink) ohosPickError(mode int, msg string)     { f.pickErr = msg }
func (f *fakeSink) ohosLog(msg string)                     { f.logs = append(f.logs, msg) }

func newTestPump() (*ohosPump, *fakeSink) {
	f := new(fakeSink)
	return newOhosPump(f, unit.Metric{PxPerDp: 2, PxPerSp: 2}), f
}

func TestOHOSKeyNames(t *testing.T) {
	cases := []struct {
		code int
		want key.Name
	}{
		{ohosKeyA, "A"}, {ohosKeyZ, "Z"}, {ohosKey0, "0"}, {ohosKey9, "9"},
		{ohosKeyF1, key.NameF1}, {ohosKeyF12, key.NameF12},
		{ohosKeyEnter, key.NameReturn}, {ohosKeyNumpadEnter, key.NameEnter},
		{ohosKeyDel, key.NameDeleteBackward}, {ohosKeyForwardDel, key.NameDeleteForward},
		{ohosKeyCtrlLeft, key.NameCtrl}, {ohosKeyShiftRight, key.NameShift},
		{ohosKeyAltRight, key.NameAlt}, {ohosKeyMetaLeft, key.NameSuper},
		{ohosKeyDpadUp, key.NameUpArrow}, {ohosKeyDpadDown, key.NameDownArrow},
		{ohosKeyPageUp, key.NamePageUp}, {ohosKeyPageDown, key.NamePageDown},
		{ohosKeyEscape, key.NameEscape}, {ohosKeySpace, key.NameSpace},
		{ohosKeyBackslash, "\\"}, {ohosKeyGrave, "`"}, {ohosKeySlash, "/"},
	}
	for _, c := range cases {
		got, ok := ohosKeyName(c.code)
		if !ok || got != c.want {
			t.Errorf("ohosKeyName(%d) = %q,%v; want %q", c.code, got, ok, c.want)
		}
	}
	if _, ok := ohosKeyName(999999); ok {
		t.Error("unknown key must not map")
	}
}

func TestOHOSTouchAndAxisRequestFrame(t *testing.T) {
	p, f := newTestPump()
	p.touch(ohosEvent{action: 1, id: 1, x: 10, y: 20})
	if f.frames != 1 {
		t.Fatalf("touch Move must request a frame, got %d", f.frames)
	}
	p.axis(ohosEvent{dy: 12, x: 10, y: 20})
	if f.frames != 2 {
		t.Fatalf("axis scroll must request a frame, got %d", f.frames)
	}
}

func TestOHOSPinchRequiresTouchpad(t *testing.T) {
	p, f := newTestPump()
	p.touch(ohosEvent{action: 0, id: 1, x: 10, y: 10})
	f.events = nil
	p.axis(ohosEvent{scale: 1.2, x: 10, y: 10})
	if len(f.events) != 0 {
		t.Fatalf("single touch must ignore axis pinch: %#v", f.events)
	}
	p.touch(ohosEvent{action: 0, id: 2, x: 20, y: 10})
	f.events = nil
	p.axis(ohosEvent{scale: 1.3, x: 15, y: 10})
	if len(f.events) != 0 {
		t.Fatalf("two touches must ignore axis pinch and use pointer pinches: %#v", f.events)
	}
	p.touch(ohosEvent{action: 2, id: 1, x: 10, y: 10})
	p.touch(ohosEvent{action: 2, id: 2, x: 20, y: 10})
	f.events = nil
	// 触摸设备即使抬手后也不再接受 axis pinch（系统会在 UP 后补发 pinch）。
	p.axis(ohosEvent{scale: 1.2, x: 10, y: 10})
	p.axis(ohosEvent{scale: 1.44, x: 10, y: 10})
	if len(f.events) != 0 {
		t.Fatalf("touch device must ignore axis pinch even after release: %#v", f.events)
	}

	// 纯触控板窗口从未出现 touch，axis pinch 仍可用。
	p2, f2 := newTestPump()
	p2.axis(ohosEvent{scale: 1.2, x: 10, y: 10})
	if len(f2.events) != 0 {
		t.Fatalf("first touchpad sample must only establish baseline: %#v", f2.events)
	}
	p2.axis(ohosEvent{scale: 1.44, x: 10, y: 10})
	if len(f2.events) != 1 {
		t.Fatalf("second touchpad sample must emit one scroll: %#v", f2.events)
	}
	pe, ok := f2.events[0].(pointer.Event)
	if !ok || pe.Scroll.Y >= 0 || !pe.Modifiers.Contain(key.ModCtrl) {
		t.Fatalf("touchpad pinch must zoom in: %#v", f2.events[0])
	}
}

func TestOHOSPrintable(t *testing.T) {
	cases := []struct {
		code int
		mods int
		want string
		ok   bool
	}{
		{ohosKeyA, 0, "a", true},
		{ohosKeyA, ohosModShift, "A", true},
		{ohosKey0, 0, "0", true},
		{ohosKey0 + 1, ohosModShift, "!", true},
		{ohosKeyMinus, ohosModShift, "_", true},
		{ohosKeySlash, 0, "/", true},
		{ohosKeySpace, 0, " ", true},
		{ohosKeyEnter, 0, "", false},
		{ohosKeyA, ohosModCtrl, "", false},
		{ohosKeyA, ohosModAlt, "", false},
	}
	for _, c := range cases {
		got, ok := ohosPrintable(c.code, c.mods)
		if got != c.want || ok != c.ok {
			t.Errorf("ohosPrintable(%d,%d) = %q,%v; want %q,%v", c.code, c.mods, got, ok, c.want, c.ok)
		}
	}
}

func TestOHOSModifiers(t *testing.T) {
	if got := ohosModifiers(ohosModCtrl | ohosModShift | ohosModAlt); got != key.ModCtrl|key.ModShift|key.ModAlt {
		t.Fatalf("modifiers = %v", got)
	}
	if got := ohosModifiers(ohosModFn); got != 0 {
		t.Fatalf("Fn must not map to a Gio modifier: %v", got)
	}
}

func TestOHOSPinchDirection(t *testing.T) {
	open, mods := ohosAxisScroll(0, 0, 1.1, 0)
	if open.Y >= 0 {
		t.Fatalf("opening pinch must produce negative Scroll.Y (zoom in), got %v", open)
	}
	if mods&key.ModCtrl == 0 {
		t.Fatalf("pinch must set Ctrl so the canvas zooms: %v", mods)
	}
	close, _ := ohosAxisScroll(0, 0, 0.9, 0)
	if close.Y <= 0 {
		t.Fatalf("closing pinch must produce positive Scroll.Y (zoom out), got %v", close)
	}
	plain, mods := ohosAxisScroll(30, -12, 0, 0)
	if plain.Y != 30 || plain.X != -12 || mods != 0 {
		t.Fatalf("plain scroll = %v, mods %v", plain, mods)
	}
}

func TestOHOSPumpFirstFrameAndZeroSize(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 0, h: 0})
	if len(f.events) != 0 {
		t.Fatalf("zero-sized surface must not emit events: %#v", f.events)
	}
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 800, h: 600})
	if len(f.events) != 3 {
		t.Fatalf("want view+config+frame, got %#v", f.events)
	}
	if v, ok := f.events[0].(OHOSViewEvent); !ok || v.Window != 7 {
		t.Fatalf("first event must be a valid view event: %#v", f.events[0])
	}
	if _, ok := f.events[1].(ConfigEvent); !ok {
		t.Fatalf("second event must be ConfigEvent: %#v", f.events[1])
	}
	fe, ok := f.events[2].(frameEvent)
	if !ok || !fe.Sync || fe.Size != image.Pt(800, 600) {
		t.Fatalf("third event must be a sync frame: %#v", f.events[2])
	}
	if got := fe.Insets; got != (Insets{}) {
		t.Fatalf("empty insets must be zero: %v", got)
	}
}

func TestOHOSPumpBackgroundStopsFrames(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 800, h: 600})
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvLifecycle, life: ohosBackground})
	p.Invalidate()
	p.SetAnimating(true)
	p.Handle(ohosEvent{kind: ohosEvFrame})
	if len(f.events) != 0 || f.frames != 0 {
		t.Fatalf("background must not draw or request frames: events=%#v frames=%d", f.events, f.frames)
	}
	p.Handle(ohosEvent{kind: ohosEvLifecycle, life: ohosForeground})
	if len(f.events) != 1 {
		t.Fatalf("foreground must draw once: %#v", f.events)
	}
}

func TestOHOSPumpAnimation(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	f.events, f.frames = nil, 0
	p.SetAnimating(true)
	if f.frames != 1 {
		t.Fatalf("SetAnimating(true) must request a vsync frame, got %d", f.frames)
	}
	p.Handle(ohosEvent{kind: ohosEvFrame})
	if len(f.events) != 1 || f.frames != 2 {
		t.Fatalf("tick must draw and request the next frame: events=%d frames=%d", len(f.events), f.frames)
	}
	p.SetAnimating(false)
	f.events, f.frames = nil, 0
	p.Handle(ohosEvent{kind: ohosEvFrame})
	if len(f.events) != 1 || f.frames != 0 {
		t.Fatalf("stray vsync must draw once without starting another loop: events=%#v frames=%d", f.events, f.frames)
	}
}

func TestOHOSPumpDestroyOnce(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvLifecycle, life: ohosDestroyed})
	p.Handle(ohosEvent{kind: ohosEvLifecycle, life: ohosDestroyed})
	if len(f.events) != 1 {
		t.Fatalf("DestroyEvent must be delivered once: %#v", f.events)
	}
	if _, ok := f.events[0].(DestroyEvent); !ok {
		t.Fatalf("want DestroyEvent, got %#v", f.events[0])
	}
}

func TestOHOSPumpInput(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	f.events = nil

	p.Handle(ohosEvent{kind: ohosEvTouch, action: 0, id: 3, x: 10, y: 20, mods: ohosModShift})
	p.Handle(ohosEvent{kind: ohosEvTouch, action: 1, id: 3, x: 11, y: 21})
	p.Handle(ohosEvent{kind: ohosEvTouch, action: 2, id: 3, x: 12, y: 22})
	p.Handle(ohosEvent{kind: ohosEvTouch, action: 3, id: 3})
	if len(f.events) != 4 {
		t.Fatalf("touch events: %#v", f.events)
	}
	down := f.events[0].(pointer.Event)
	if down.Kind != pointer.Press || down.Source != pointer.Touch || down.PointerID != 3 || down.Position.X != 10 {
		t.Fatalf("down event = %+v", down)
	}
	if f.events[3].(pointer.Event).Kind != pointer.Cancel {
		t.Fatal("cancel must map to pointer.Cancel")
	}
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvMouse, action: 0, code: 1, x: 5, y: 6})
	me := f.events[0].(pointer.Event)
	if me.Source != pointer.Mouse || me.Buttons != pointer.ButtonPrimary {
		t.Fatalf("mouse event = %+v", me)
	}
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvAxis, dx: 4, dy: 9})
	se := f.events[0].(pointer.Event)
	if se.Kind != pointer.Scroll || se.Scroll.X != 4 || se.Scroll.Y != 9 {
		t.Fatalf("scroll event = %+v", se)
	}
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvKey, code: ohosKeyA, action: 0})
	p.Handle(ohosEvent{kind: ohosEvKey, code: ohosKeyA, action: 1})
	if len(f.events) != 2 || len(f.inserted) != 1 || f.inserted[0] != "a" {
		t.Fatalf("key events=%#v inserted=%#v", f.events, f.inserted)
	}
	ke := f.events[0].(key.Event)
	if ke.Name != "A" || ke.State != key.Press {
		t.Fatalf("key event = %+v", ke)
	}
	if f.events[1].(key.Event).State != key.Release {
		t.Fatal("release must map to key.Release")
	}
}

func TestOHOSPumpClipboardAndInsets(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvClipboard, text: "text/plain", text2: "hello 中文"})
	de := f.events[0].(transfer.DataEvent)
	b, _ := io.ReadAll(de.Open())
	if string(b) != "hello 中文" {
		t.Fatalf("clipboard = %q", b)
	}
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvInsets, top: 20, right: 0, bottom: 40, left: 0})
	if len(f.events) != 1 {
		t.Fatalf("insets must trigger a sync frame: %#v", f.events)
	}
	fe := f.events[0].(frameEvent)
	if fe.Insets.Bottom != unit.Dp(20) || fe.Insets.Top != unit.Dp(10) {
		t.Fatalf("insets = %+v", fe.Insets)
	}
}

func TestOHOSPumpIMEAndPicker(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvPreedit, text: "ni", a: 1, b: 2})
	p.Handle(ohosEvent{kind: ohosEvIMEDelete, a: 1, b: 2})
	p.Handle(ohosEvent{kind: ohosEvIMEMove, a: 3})
	p.Handle(ohosEvent{kind: ohosEvIMESelection, a: 4, b: 5})
	p.Handle(ohosEvent{kind: ohosEvIMEEnter, a: 6})
	if len(f.pread) != 1 || f.pread[0] != "ni" || f.imeDirty != 5 {
		t.Fatalf("preedit=%#v dirty=%d", f.pread, f.imeDirty)
	}
	if len(f.deleted) != 1 || len(f.moved) != 1 || len(f.sel) != 1 || len(f.enter) != 1 {
		t.Fatalf("ime calls: %#v %#v %#v %#v", f.deleted, f.moved, f.sel, f.enter)
	}
	p.Handle(ohosEvent{kind: ohosEvPickResult, action: 0, text: "file://docs/a.pdf\n\nfile://docs/b.ofd\n"})
	if len(f.picks) != 2 || f.picks[0] != "file://docs/a.pdf" || f.picks[1] != "file://docs/b.ofd" {
		t.Fatalf("picks = %#v", f.picks)
	}
	p.Handle(ohosEvent{kind: ohosEvPickError, action: 1, text: "denied"})
	if f.pickErr != "denied" {
		t.Fatalf("pickErr = %q", f.pickErr)
	}
}

func TestOHOSNoPerEventLogs(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvTouch, action: 1, id: 1, x: 1, y: 2})
	p.Handle(ohosEvent{kind: ohosEvMouse, action: 2, x: 3, y: 4})
	p.Handle(ohosEvent{kind: ohosEvAxis, dx: 1, dy: 2})
	if len(f.logs) != 0 {
		t.Fatalf("input and frame paths must not log: %#v", f.logs)
	}
}

func TestOHOSDebugFlag(t *testing.T) {
	t.Setenv("GIO_OHOS_DEBUG", "")
	if ohosDebugEnabled() {
		t.Fatal("debug must be off by default")
	}
	t.Setenv("GIO_OHOS_DEBUG", "1")
	if !ohosDebugEnabled() {
		t.Fatal("GIO_OHOS_DEBUG=1 must enable diagnostics")
	}
	t.Setenv("GIO_OHOS_DEBUG", "false")
	if ohosDebugEnabled() {
		t.Fatal("GIO_OHOS_DEBUG=false must disable diagnostics")
	}
}

func TestOHOSPumpStopIsIdempotent(t *testing.T) {
	p, f := newTestPump()
	p.Stop()
	p.Stop()
	if len(f.events) != 1 {
		t.Fatalf("Stop must deliver one DestroyEvent: %#v", f.events)
	}
	if _, ok := f.events[0].(DestroyEvent); !ok {
		t.Fatalf("want DestroyEvent, got %#v", f.events)
	}
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvTouch, action: 0, id: 1})
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 1, w: 10, h: 10})
	if len(f.events) != 0 || f.frames != 0 {
		t.Fatalf("pump must be inert after Stop: events=%#v frames=%d", f.events, f.frames)
	}
}
