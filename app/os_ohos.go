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
	"sync/atomic"
	"unsafe"

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

	qmu    sync.Mutex
	queue  ohosQueue
	closed atomic.Bool

	wakeupCh    chan struct{}
	done        chan struct{}
	destroyOnce sync.Once

	inputHint key.InputHint
	textInput bool
	animating bool
	ime       ohosIME
}

var ohosState = struct {
	mu       sync.Mutex
	win      *window
	staging  []ohosEvent
	metric   unit.Metric
	dataDir  string
	cacheDir string
}{
	metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
}

// enqueueOhos routes a platform callback to the current window. Callbacks
// that arrive before the first window are staged and handed over atomically
// by newWindow; callbacks after destroy are dropped.
func enqueueOhos(e ohosEvent) {
	ohosState.mu.Lock()
	w := ohosState.win
	if w == nil {
		if len(ohosState.staging) < ohosQueueLimit || ohosEventCritical(e.kind) {
			ohosState.staging = append(ohosState.staging, e)
		}
		ohosState.mu.Unlock()
		return
	}
	ohosState.mu.Unlock()
	w.enqueue(e)
}

// enqueue appends to this window's queue and wakes its loop.
func (w *window) enqueue(e ohosEvent) {
	if w.closed.Load() {
		return
	}
	w.qmu.Lock()
	if w.closed.Load() {
		w.qmu.Unlock()
		return
	}
	ok := w.queue.push(e)
	w.qmu.Unlock()
	if !ok {
		return
	}
	select {
	case w.wakeupCh <- struct{}{}:
	default:
	}
}

// drainEvents handles one batch. It returns false when the queue is empty.
func (w *window) drainEvents() bool {
	w.qmu.Lock()
	batch := w.queue.take()
	w.qmu.Unlock()
	if len(batch) == 0 {
		return false
	}
	for _, e := range batch {
		w.pump.Handle(e)
	}
	w.qmu.Lock()
	w.queue.recycle(batch)
	w.qmu.Unlock()
	return true
}

func (w *window) loopRun(options []Option) {
	w.pump.ApplyOptions(options)
	for {
		for w.drainEvents() {
			if w.pump.destroyed {
				w.close()
				return
			}
		}
		select {
		case <-w.wakeupCh:
			w.loop.Wakeup()
			w.loop.FlushEvents()
		case <-w.done:
			return
		}
	}
}

// close stops the loop and releases the window's queue. It is idempotent.
func (w *window) close() {
	w.destroyOnce.Do(func() {
		ohosState.mu.Lock()
		if ohosState.win == w {
			ohosState.win = nil
			ohosState.staging = nil
		}
		w.qmu.Lock()
		w.closed.Store(true)
		w.queue.clear()
		w.qmu.Unlock()
		ohosState.mu.Unlock()
		w.loop.Close()
		close(w.done)
	})
}

func newWindow(callbacks *callbacks, options []Option) {
	w := &window{
		callbacks: callbacks,
		wakeupCh:  make(chan struct{}, 1),
		done:      make(chan struct{}),
		inputHint: key.HintAny,
	}
	w.loop = newEventLoop(callbacks, w.wakeup)
	callbacks.SetDriver(w)
	if abi := int(C.gio_ohos_host_abi_version()); abi < int(C.GIO_OHOS_ABI_VERSION) {
		ohosWarnf("host-abi", "OHOS shim ABI %d < %d; extended features disabled", abi, int(C.GIO_OHOS_ABI_VERSION))
	}

	ohosState.mu.Lock()
	metric := ohosState.metric
	if metric.PxPerDp <= 0 {
		metric.PxPerDp = 1
	}
	if metric.PxPerSp <= 0 {
		metric.PxPerSp = metric.PxPerDp
	}
	w.pump = newOhosPump(w, w, metric)
	staging := ohosState.staging
	ohosState.staging = nil
	ohosState.win = w
	w.qmu.Lock()
	w.queue.items = append(w.queue.items, staging...)
	w.qmu.Unlock()
	ohosState.mu.Unlock()

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
	if w.closed.Load() {
		if _, ok := e.(DestroyEvent); !ok {
			return
		}
	}
	// Always flush: ConfigEvent and other coalesced events may report
	// "not handled" but still need to reach the client.
	w.callbacks.ProcessEvent(e)
	w.loop.FlushEvents()
}

func (w *window) Event() event.Event { return w.loop.Event() }

func (w *window) Invalidate() {
	w.enqueue(ohosEvent{kind: ohosEvInvalidate})
}

func (w *window) Run(f func()) {
	if w.closed.Load() {
		return
	}
	w.loop.Run(f)
}

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
	w.enqueue(ohosEvent{kind: ohosEvSetAnimating, anim: anim})
}

func (w *window) Configure(options []Option) {
	w.enqueue(ohosEvent{kind: ohosEvConfigure, opts: options})
}

// ---- ohosHost ----

func (w *window) Apply(d ohosConfigDelta) {
	if d.ModeSet {
		C.gio_call_set_window_mode(C.int(ohosWindowMode(d.Mode)))
	}
	if d.TitleSet {
		ct := C.CString(d.Title)
		C.gio_call_set_title(ct)
		C.free(unsafe.Pointer(ct))
	}
	if d.OrientationSet {
		C.gio_call_set_orientation(C.int(ohosOrientation(d.Orientation)))
	}
	if d.StatusSet {
		C.gio_call_set_status_color(C.uint32_t(ohosColor(d.Status)))
	}
	if d.NavigationSet {
		C.gio_call_set_navigation_color(C.uint32_t(ohosColor(d.Navigation)))
	}
	if d.TopMostSet {
		C.gio_call_set_top_most(boolInt(d.TopMost))
	}
	if d.LimitsSet {
		C.gio_call_set_window_limits(C.int(d.Min.X), C.int(d.Min.Y), C.int(d.Max.X), C.int(d.Max.Y))
	}
}

func (w *window) HostSetAnimating(anim bool) {
	C.gio_call_set_animating(boolInt(anim))
}

func (w *window) RequestClose() bool {
	return C.gio_call_request_close() != 0
}

func (w *window) RaiseWindow() {
	C.gio_call_raise_window()
}

func (w *window) CenterWindow() {
	C.gio_call_center_window()
}

func (w *window) StartWindowMove() {
	C.gio_call_start_window_move()
}

func (w *window) SetCursor(cursor pointer.Cursor) {
	if w.closed.Load() {
		return
	}
	C.gio_call_set_cursor(C.int(ohosCursorNumber(cursor)))
}

func (w *window) ShowTextInput(show bool) {
	if w.closed.Load() {
		return
	}
	w.textInput = show
	w.ime.keyboard = show
	w.ime.resetWire()
	C.gio_call_show_keyboard(boolInt(show))
	C.gio_call_ime_keyboard_status(boolInt(show))
	if show {
		w.updateIME()
	}
}

func (w *window) SetInputHint(hint key.InputHint) {
	if w.closed.Load() {
		return
	}
	w.inputHint = hint
	w.ime.setHint(hint)
	w.updateIME()
}

func (w *window) ReadClipboard() {
	if w.closed.Load() {
		return
	}
	mime := C.CString("text/plain")
	defer C.free(unsafe.Pointer(mime))
	C.gio_call_get_clipboard(mime)
}

func (w *window) WriteClipboard(mime string, s []byte) {
	if w.closed.Load() {
		return
	}
	cmime := C.CString(mime)
	defer C.free(unsafe.Pointer(cmime))
	ctext := C.CString(string(s))
	defer C.free(unsafe.Pointer(ctext))
	C.gio_call_set_clipboard(cmime, ctext)
}

func (w *window) Perform(actions system.Action) {
	if w.closed.Load() {
		return
	}
	if actions != 0 {
		w.enqueue(ohosEvent{kind: ohosEvAction, sysAction: actions})
	}
}

// ---- ohosSink ----

func (w *window) ohosDeliver(e event.Event) { w.deliver(e) }

func (w *window) ohosRequestFrame() { C.gio_call_invalidate() }

func (w *window) ohosEditorInsert(text string) {
	if text == "" {
		return
	}
	w.ime.commit(w.callbacks, text)
	w.updateIME()
}

func (w *window) ohosEditorPreedit(text string, selStart, selEnd int) {
	w.ime.preedit(w.callbacks, text, selStart, selEnd)
	w.updateIME()
}

func (w *window) ohosEditorDelete(before, after int) {
	if !w.ime.delete(w.callbacks, before, after) {
		st := w.callbacks.EditorState()
		if rng, ok := ohosDeleteRange(st, before, after); ok && rng.Start != rng.End {
			w.callbacks.EditorReplace(rng, "")
			w.callbacks.SetEditorSelection(key.Range{Start: rng.Start, End: rng.Start})
		}
	}
	w.updateIME()
}

func (w *window) ohosEditorMove(direction int) {
	st := w.callbacks.EditorState()
	if rng, ok := ohosMoveRange(st, direction); ok {
		w.callbacks.SetEditorSelection(rng)
	}
}

func (w *window) ohosEditorSelect(start, end int) {
	w.ime.selectRange(w.callbacks, start, end)
	w.updateIME()
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
	if w.closed.Load() {
		return
	}
	if !w.textInput {
		C.gio_call_ime_config(C.int(w.ime.hint))
		return
	}
	st := w.callbacks.EditorState()
	sn, status := textInputSurrounding(st)
	if status == surroundingAwaitingSnippet {
		requestSurroundingText(w.callbacks)
		return
	}
	if status != surroundingReady {
		return
	}
	wire, ok := w.ime.build(st, sn, w.textInput)
	if !ok || !w.ime.changed(wire) {
		return
	}
	ctext := C.CString(wire.text)
	defer C.free(unsafe.Pointer(ctext))
	C.gio_call_ime_selection(ctext, C.int(wire.length), C.int(wire.start), C.int(wire.end))
	r := wire.caret
	C.gio_call_ime_cursor(C.float(r.Min.X), C.float(r.Min.Y), C.float(r.Dx()), C.float(r.Dy()))
	C.gio_call_ime_config(C.int(wire.hint))
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

// ---- ABI 1 exports ----

//export gio_ohos_install_host
func gio_ohos_install_host(host *C.gio_ohos_host) {
	C.gio_ohos_store_host(host)
}

//export gio_ohos_install_host_v3
func gio_ohos_install_host_v3(host *C.gio_ohos_host_v3, size C.size_t) {
	C.gio_ohos_store_host_v3(host, size)
}

//export gio_ohos_abi_version
func gio_ohos_abi_version() C.int {
	return C.GIO_OHOS_ABI_VERSION
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

	ohosState.mu.Lock()
	ohosState.dataDir = d
	ohosState.cacheDir = c
	ohosState.mu.Unlock()
	ohosApplyMetric(ohosMetric(float32(density), float32(fontScale)))
}

//export gio_ohos_density
func gio_ohos_density(density, fontScale C.float) {
	ohosApplyMetric(ohosMetric(float32(density), float32(fontScale)))
}

func ohosMetric(density, fontScale float32) unit.Metric {
	m := unit.Metric{PxPerDp: density, PxPerSp: density * fontScale}
	if m.PxPerDp <= 0 {
		m.PxPerDp = 1
	}
	if m.PxPerSp <= 0 {
		m.PxPerSp = m.PxPerDp
	}
	return m
}

func ohosApplyMetric(m unit.Metric) {
	ohosState.mu.Lock()
	ohosState.metric = m
	live := ohosState.win != nil
	ohosState.mu.Unlock()
	if live {
		enqueueOhos(ohosEvent{kind: ohosEvMetric, metric: m})
	}
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

//export gio_ohos_window_mode
func gio_ohos_window_mode(mode C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvWindowMode, a: int(mode)})
}

//export gio_ohos_orientation
func gio_ohos_orientation(orientation C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvOrientation, a: int(orientation)})
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
