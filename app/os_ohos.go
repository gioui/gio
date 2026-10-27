// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony

package app

/*
#cgo CFLAGS: -Werror
#cgo LDFLAGS: -Wl,--no-as-needed -lEGL -lGLESv3 -lnative_window -lnative_vsync -lace_ndk.z -Wl,--as-needed

#include <stdlib.h>
#include <EGL/egl.h>
#include <native_buffer/buffer_common.h>
#include <native_window/external_window.h>
#include <ace/xcomponent/native_xcomponent_key_event.h>
#include "ohos_glue.h"
*/
import "C"

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/unit"
)

// Compile-time checks that the key codes copied into ohos_input.go still match
// the NDK header.  A mismatch makes the array index negative or out of range.
var (
	_ = [1]struct{}{}[int(C.KEY_A)-ohosKeyA]
	_ = [1]struct{}{}[int(C.KEY_Z)-ohosKeyZ]
	_ = [1]struct{}{}[int(C.KEY_0)-ohosKey0]
	_ = [1]struct{}{}[int(C.KEY_9)-ohosKey9]
	_ = [1]struct{}{}[int(C.KEY_F1)-ohosKeyF1]
	_ = [1]struct{}{}[int(C.KEY_F12)-ohosKeyF12]
	_ = [1]struct{}{}[int(C.KEY_ENTER)-ohosKeyEnter]
	_ = [1]struct{}{}[int(C.KEY_NUMPAD_ENTER)-ohosKeyNumpadEnter]
	_ = [1]struct{}{}[int(C.KEY_DEL)-ohosKeyDel]
	_ = [1]struct{}{}[int(C.KEY_FORWARD_DEL)-ohosKeyForwardDel]
	_ = [1]struct{}{}[int(C.KEY_CTRL_LEFT)-ohosKeyCtrlLeft]
	_ = [1]struct{}{}[int(C.KEY_CTRL_RIGHT)-ohosKeyCtrlRight]
	_ = [1]struct{}{}[int(C.KEY_SHIFT_LEFT)-ohosKeyShiftLeft]
	_ = [1]struct{}{}[int(C.KEY_SHIFT_RIGHT)-ohosKeyShiftRight]
	_ = [1]struct{}{}[int(C.KEY_ALT_LEFT)-ohosKeyAltLeft]
	_ = [1]struct{}{}[int(C.KEY_ALT_RIGHT)-ohosKeyAltRight]
	_ = [1]struct{}{}[int(C.KEY_META_LEFT)-ohosKeyMetaLeft]
	_ = [1]struct{}{}[int(C.KEY_META_RIGHT)-ohosKeyMetaRight]
	_ = [1]struct{}{}[int(C.KEY_TAB)-ohosKeyTab]
	_ = [1]struct{}{}[int(C.KEY_SPACE)-ohosKeySpace]
	_ = [1]struct{}{}[int(C.KEY_ESCAPE)-ohosKeyEscape]
	_ = [1]struct{}{}[int(C.KEY_MOVE_HOME)-ohosKeyMoveHome]
	_ = [1]struct{}{}[int(C.KEY_MOVE_END)-ohosKeyMoveEnd]
	_ = [1]struct{}{}[int(C.KEY_PAGE_UP)-ohosKeyPageUp]
	_ = [1]struct{}{}[int(C.KEY_PAGE_DOWN)-ohosKeyPageDown]
	_ = [1]struct{}{}[int(C.KEY_DPAD_UP)-ohosKeyDpadUp]
	_ = [1]struct{}{}[int(C.KEY_DPAD_DOWN)-ohosKeyDpadDown]
	_ = [1]struct{}{}[int(C.KEY_DPAD_LEFT)-ohosKeyDpadLeft]
	_ = [1]struct{}{}[int(C.KEY_DPAD_RIGHT)-ohosKeyDpadRight]
	_ = [1]struct{}{}[int(C.KEY_BACK)-ohosKeyBack]
	_ = [1]struct{}{}[int(C.KEY_INSERT)-ohosKeyInsert]
)

type window struct {
	callbacks *callbacks
	loop      *eventLoop
	pump      *ohosPump

	wakeupCh chan struct{}

	inputHint key.InputHint
	textInput bool
	animating bool
}

var ohosState = struct {
	mu       sync.Mutex
	queue    []ohosEvent
	notify   chan struct{}
	metric   unit.Metric
	dataDir  string
	cacheDir string
}{
	notify: make(chan struct{}, 1),
	metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
}

func enqueueOhos(e ohosEvent) {
	ohosState.mu.Lock()
	ohosState.queue = append(ohosState.queue, e)
	ohosState.mu.Unlock()
	select {
	case ohosState.notify <- struct{}{}:
	default:
	}
}

// drainEvents delivers every queued callback on the loop goroutine.
func (w *window) drainEvents() {
	for {
		ohosState.mu.Lock()
		if len(ohosState.queue) == 0 {
			ohosState.mu.Unlock()
			return
		}
		evs := ohosState.queue
		ohosState.queue = nil
		ohosState.mu.Unlock()
		for _, e := range evs {
			w.pump.Handle(e)
		}
	}
}

func (w *window) loopRun(options []Option) {
	w.pump.ApplyOptions(options)
	for {
		w.drainEvents()
		select {
		case <-ohosState.notify:
		case <-w.wakeupCh:
			w.loop.Wakeup()
			w.loop.FlushEvents()
		}
	}
}

func newWindow(callbacks *callbacks, options []Option) {
	w := &window{
		callbacks: callbacks,
		wakeupCh:  make(chan struct{}, 1),
		inputHint: key.HintAny,
	}
	w.loop = newEventLoop(callbacks, w.wakeup)
	callbacks.SetDriver(w)

	ohosState.mu.Lock()
	metric := ohosState.metric
	if metric.PxPerDp <= 0 {
		metric.PxPerDp = 1
	}
	if metric.PxPerSp <= 0 {
		metric.PxPerSp = metric.PxPerDp
	}
	ohosState.mu.Unlock()

	w.pump = newOhosPump(w, metric)
	go w.loopRun(options)
}

func osMain() {
	// The HarmonyOS runtime owns the process main thread; Go main runs from
	// gio_ohos_run via runMain.
	select {}
}

func (w *window) ProcessEvent(e event.Event) { w.deliver(e) }

// deliver feeds one event to Gio.  It runs on the loop goroutine; re-entrant
// calls from driver functions are safe because eventLoop.FlushEvents guards
// against nested delivery.
func (w *window) deliver(e event.Event) {
	if !w.callbacks.ProcessEvent(e) {
		return
	}
	w.loop.FlushEvents()
}

func (w *window) Event() event.Event { return w.loop.Event() }

func (w *window) Invalidate() { w.loop.Invalidate() }

func (w *window) Run(f func()) { w.loop.Run(f) }

func (w *window) Frame(frame *op.Ops) { w.loop.Frame(frame) }

func (w *window) wakeup() {
	select {
	case w.wakeupCh <- struct{}{}:
	default:
	}
}

func (w *window) NewContext() (context, error) {
	if newOHOSGLESContext == nil {
		return nil, errors.New("ohos: OpenGL support is disabled in this build")
	}
	return newOHOSGLESContext(w)
}

func (w *window) SetAnimating(anim bool) {
	w.animating = anim
	enqueueOhos(ohosEvent{kind: ohosEvSetAnimating, anim: anim})
}

func (w *window) Configure(options []Option) {
	enqueueOhos(ohosEvent{kind: ohosEvConfigure, opts: options})
}

func (w *window) SetCursor(cursor pointer.Cursor) {
	C.gio_call_set_cursor(C.int(ohosCursorNumber(cursor)))
}

func (w *window) ShowTextInput(show bool) {
	w.textInput = show
	C.gio_call_show_keyboard(boolInt(show))
}

func (w *window) SetInputHint(hint key.InputHint) {
	w.inputHint = hint
	C.gio_call_ime_config(C.int(ohosInputHint(hint)))
}

func (w *window) ReadClipboard() {
	mime := C.CString("text/plain")
	defer C.free(unsafe.Pointer(mime))
	C.gio_call_get_clipboard(mime)
}

func (w *window) WriteClipboard(mime string, s []byte) {
	cmime := C.CString(mime)
	defer C.free(unsafe.Pointer(cmime))
	ctext := C.CString(string(s))
	defer C.free(unsafe.Pointer(ctext))
	C.gio_call_set_clipboard(cmime, ctext)
}

func (w *window) Perform(actions system.Action) {
	if actions&system.ActionClose != 0 {
		w.deliver(DestroyEvent{})
	}
}

// ---- ohosSink ----

func (w *window) ohosDeliver(e event.Event) { w.deliver(e) }

func (w *window) ohosRequestFrame() { C.gio_call_invalidate() }

func (w *window) ohosEditorInsert(text string) {
	if text == "" {
		return
	}
	w.callbacks.EditorInsert(text)
}

func (w *window) ohosEditorPreedit(text string, selStart, selEnd int) {
	st := w.callbacks.EditorState()
	if st.compose.Start != -1 {
		w.callbacks.EditorReplace(normRange(st.compose), "")
		w.callbacks.SetComposingRegion(key.Range{Start: -1, End: -1})
	}
	rng := normRange(st.Selection.Range)
	runes := []rune(text)
	if text != "" {
		w.callbacks.EditorReplace(rng, text)
	}
	start := rng.Start
	end := start + len(runes)
	if text == "" {
		end = start
	}
	w.callbacks.SetComposingRegion(key.Range{Start: start, End: end})
	selStart += start
	selEnd += start
	if selStart < start || selStart > end {
		selStart = end
	}
	if selEnd < start || selEnd > end {
		selEnd = selStart
	}
	w.callbacks.SetEditorSelection(key.Range{Start: selStart, End: selEnd})
}

func (w *window) ohosEditorDelete(before, after int) {
	st := w.callbacks.EditorState()
	if rng, ok := ohosDeleteRange(st, before, after); ok && rng.Start != rng.End {
		w.callbacks.EditorReplace(rng, "")
		pos := rng.Start
		w.callbacks.SetEditorSelection(key.Range{Start: pos, End: pos})
	}
}

func (w *window) ohosEditorMove(direction int) {
	st := w.callbacks.EditorState()
	if rng, ok := ohosMoveRange(st, direction); ok {
		w.callbacks.SetEditorSelection(rng)
	}
}

func (w *window) ohosEditorSelect(start, end int) {
	w.callbacks.SetEditorSelection(key.Range{Start: start, End: end})
}

func (w *window) ohosEditorEnter(enterType int) {
	if enterType == 8 { // IME_ENTER_KEY_NEWLINE
		w.callbacks.EditorInsert("\n")
		return
	}
	w.ohosDeliver(key.Event{Name: key.NameReturn, State: key.Press})
	w.ohosDeliver(key.Event{Name: key.NameReturn, State: key.Release})
}

func (w *window) ohosIMEChanged() { w.updateIME() }

func (w *window) ohosPickResult(mode int, uris []string) {
	ohosPick.deliver(mode, uris, "")
}

func (w *window) ohosPickError(mode int, msg string) {
	ohosPick.deliver(mode, nil, msg)
}

func (w *window) ohosLog(msg string) {
	ohosDebugf("log:"+msg, "%s", msg)
}

// updateIME reports the surrounding text, selection and caret rectangle to
// the platform input method.  It runs on the loop goroutine.
func (w *window) updateIME() {
	if !w.textInput {
		C.gio_call_ime_config(C.int(ohosInputHint(w.inputHint)))
		return
	}
	st := w.callbacks.EditorState()
	sn := st.Snippet
	text := sn.Text
	sel := normRange(st.Selection.Range)
	start := sel.Start - sn.Start
	end := sel.End - sn.Start
	runes := []rune(text)
	if start < 0 || end < 0 || start > len(runes) || end > len(runes) {
		start, end = 0, 0
	}
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	C.gio_call_ime_selection(ctext, C.int(len(text)), C.int(start), C.int(end))

	c := st.Selection
	top := c.Transform.Transform(c.Caret.Pos.Sub(f32.Pt(0, c.Caret.Ascent)))
	bottom := c.Transform.Transform(c.Caret.Pos.Add(f32.Pt(0, c.Caret.Descent)))
	x0, x1 := min32(top.X, bottom.X), max32(top.X, bottom.X)
	y0, y1 := min32(top.Y, bottom.Y), max32(top.Y, bottom.Y)
	C.gio_call_ime_cursor(C.float(x0), C.float(y0), C.float(x1-x0), C.float(y1-y0))
	C.gio_call_ime_config(C.int(ohosInputHint(w.inputHint)))
}

func (w *window) EditorStateChanged(old, new editorState) {
	w.updateIME()
}

// ---- helpers ----

func boolInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

func ohosInputHint(h key.InputHint) int {
	switch h {
	case key.HintText:
		return 1
	case key.HintNumeric:
		return 2
	case key.HintEmail:
		return 3
	case key.HintURL:
		return 4
	case key.HintTelephone:
		return 5
	case key.HintPassword:
		return 6
	}
	return 0
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func ohosCursorNumber(cursor pointer.Cursor) int {
	// Values mirror the ArkUI cursor enum used by the shim; 0 restores the
	// default arrow.
	switch cursor {
	case pointer.CursorText, pointer.CursorVerticalText:
		return 1
	case pointer.CursorPointer:
		return 2
	case pointer.CursorCrosshair:
		return 3
	case pointer.CursorWait:
		return 4
	case pointer.CursorProgress:
		return 5
	case pointer.CursorGrab, pointer.CursorGrabbing:
		return 6
	case pointer.CursorColResize, pointer.CursorEastWestResize, pointer.CursorWestResize, pointer.CursorEastResize:
		return 7
	case pointer.CursorRowResize, pointer.CursorNorthSouthResize, pointer.CursorNorthResize, pointer.CursorSouthResize:
		return 8
	case pointer.CursorNotAllowed:
		return 9
	case pointer.CursorNone:
		return -1
	}
	return 0
}

// ---- ABI 1 exports ----

//export gio_ohos_install_host
func gio_ohos_install_host(host *C.gio_ohos_host) {
	C.gio_ohos_store_host(host)
}

//export gio_ohos_configure
func gio_ohos_configure(dataDir, cacheDir *C.char, density, fontScale C.float) {
	d := cString(dataDir)
	c := cString(cacheDir)
	if d == "" {
		return
	}
	if c == "" {
		c = filepath.Join(d, "cache")
	}
	_ = os.MkdirAll(d, 0o750)
	_ = os.MkdirAll(c, 0o750)
	setEnvDefault("HOME", d)
	setEnvDefault("XDG_CONFIG_HOME", filepath.Join(d, "config"))
	setEnvDefault("XDG_CACHE_HOME", c)
	setEnvDefault("TMPDIR", filepath.Join(c, "tmp"))
	_ = os.MkdirAll(filepath.Join(c, "tmp"), 0o750)

	m := unit.Metric{PxPerDp: float32(density), PxPerSp: float32(density) * float32(fontScale)}
	if m.PxPerDp <= 0 {
		m.PxPerDp = 1
	}
	if m.PxPerSp <= 0 {
		m.PxPerSp = m.PxPerDp
	}
	ohosState.mu.Lock()
	ohosState.dataDir = d
	ohosState.cacheDir = c
	ohosState.metric = m
	ohosState.mu.Unlock()
}

//export gio_ohos_run
func gio_ohos_run() {
	runMain()
}

//export gio_ohos_surface_created
func gio_ohos_surface_created(win unsafe.Pointer, width, height C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvSurfaceCreated, win: uintptr(win), w: int(width), h: int(height)})
}

//export gio_ohos_surface_changed
func gio_ohos_surface_changed(win unsafe.Pointer, width, height C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvSurfaceChanged, win: uintptr(win), w: int(width), h: int(height)})
}

//export gio_ohos_surface_destroyed
func gio_ohos_surface_destroyed() {
	enqueueOhos(ohosEvent{kind: ohosEvSurfaceDestroyed})
}

//export gio_ohos_touch
func gio_ohos_touch(action, id C.int, x, y C.float, mods C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvTouch, action: int(action), id: int(id), x: float32(x), y: float32(y), mods: int(mods)})
}

//export gio_ohos_mouse
func gio_ohos_mouse(action, button C.int, x, y C.float, mods C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvMouse, action: int(action), code: int(button), x: float32(x), y: float32(y), mods: int(mods)})
}

//export gio_ohos_axis
func gio_ohos_axis(x, y, dx, dy C.float, mods C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvAxis, x: float32(x), y: float32(y), dx: float32(dx), dy: float32(dy), mods: int(mods)})
}

//export gio_ohos_pinch
func gio_ohos_pinch(x, y, scale C.float, mods C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvAxis, x: float32(x), y: float32(y), scale: float32(scale), mods: int(mods)})
}

//export gio_ohos_key
func gio_ohos_key(code, action, mods C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvKey, code: int(code), action: int(action), mods: int(mods)})
}

//export gio_ohos_text
func gio_ohos_text(utf8 *C.char) {
	enqueueOhos(ohosEvent{kind: ohosEvText, text: cString(utf8)})
}

//export gio_ohos_focus
func gio_ohos_focus(focused C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvFocus, focus: focused != 0})
}

//export gio_ohos_clipboard
func gio_ohos_clipboard(mime, text *C.char) {
	enqueueOhos(ohosEvent{kind: ohosEvClipboard, text: cString(mime), text2: cString(text)})
}

//export gio_ohos_lifecycle
func gio_ohos_lifecycle(state C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvLifecycle, life: ohosLifecycle(state)})
}

//export gio_ohos_log
func gio_ohos_log(msg *C.char) {
	s := cString(msg)
	if s != "" {
		ohosDebugf("shimlog:"+s, "%s", s)
	}
}

// ---- ABI 2 exports ----

//export gio_ohos_frame
func gio_ohos_frame() {
	enqueueOhos(ohosEvent{kind: ohosEvFrame})
}

//export gio_ohos_insets
func gio_ohos_insets(top, right, bottom, left C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvInsets, top: int(top), right: int(right), bottom: int(bottom), left: int(left)})
}

//export gio_ohos_preedit
func gio_ohos_preedit(utf8 *C.char, selStart, selEnd C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvPreedit, text: cString(utf8), a: int(selStart), b: int(selEnd)})
}

//export gio_ohos_ime_delete
func gio_ohos_ime_delete(before, after C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvIMEDelete, a: int(before), b: int(after)})
}

//export gio_ohos_ime_move
func gio_ohos_ime_move(direction C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvIMEMove, a: int(direction)})
}

//export gio_ohos_ime_set_selection
func gio_ohos_ime_set_selection(start, end C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvIMESelection, a: int(start), b: int(end)})
}

//export gio_ohos_ime_enter
func gio_ohos_ime_enter(enterType C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvIMEEnter, a: int(enterType)})
}

//export gio_ohos_ime_keyboard
func gio_ohos_ime_keyboard(status C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvIMEKeyboard, a: int(status)})
}

//export gio_ohos_pick_result
func gio_ohos_pick_result(mode C.int, uris *C.char) {
	enqueueOhos(ohosEvent{kind: ohosEvPickResult, action: int(mode), text: cString(uris)})
}

//export gio_ohos_pick_error
func gio_ohos_pick_error(mode C.int, msg *C.char) {
	enqueueOhos(ohosEvent{kind: ohosEvPickError, action: int(mode), text: cString(msg)})
}

func cString(s *C.char) string {
	if s == nil {
		return ""
	}
	return C.GoString(s)
}

func setEnvDefault(key, val string) {
	if _, ok := os.LookupEnv(key); !ok {
		_ = os.Setenv(key, val)
	}
}

// ---- picker bridge (used by gpofd) ----

type ohosPickBridge struct {
	mu sync.Mutex
	f  func(mode int, uris []string, errMsg string)
}

var ohosPick ohosPickBridge

// SetOHOSPickHandler installs the receiver for system file-picker results.
// mode is 0 for open and 1 for save; errMsg is non-empty on failure.
func SetOHOSPickHandler(f func(mode int, uris []string, errMsg string)) {
	ohosPick.mu.Lock()
	ohosPick.f = f
	ohosPick.mu.Unlock()
}

func (p *ohosPickBridge) deliver(mode int, uris []string, errMsg string) {
	ohosPick.mu.Lock()
	f := ohosPick.f
	ohosPick.mu.Unlock()
	if f != nil {
		f(mode, append([]string(nil), uris...), errMsg)
	}
}

// OHOSPickFile asks the platform to show its system file picker.  Results are
// delivered to the handler installed by SetOHOSPickHandler.
func OHOSPickFile(mode int) {
	C.gio_call_pick_file(C.int(mode))
}

// OHOSLog forwards a message to hilog through the shim.
func OHOSLog(msg string) {
	cmsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cmsg))
	C.gio_call_log(cmsg)
}
