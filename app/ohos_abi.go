// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony

package app

/*
#include <stdlib.h>
#include "ohos_glue.h"
*/
import "C"

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"gioui.org/unit"
)

// ---- ABI 1 exports ----

//export gio_ohos_install_host
func gio_ohos_install_host(host *C.gio_ohos_host) {
	C.gio_ohos_store_host(host)
}

//export gio_ohos_install_host_v3
func gio_ohos_install_host_v3(host *C.gio_ohos_host_v3, size C.size_t) {
	C.gio_ohos_store_host_v3(host, size)
}

//export gio_ohos_install_host_v4
func gio_ohos_install_host_v4(host *C.gio_ohos_host_v4, size C.size_t) {
	C.gio_ohos_store_host_v4(host, size)
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

// ---- ABI 4 window-scoped events ----

//export gio_ohos_surface_created_w
func gio_ohos_surface_created_w(id C.uint64_t, win unsafe.Pointer, width, height C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvSurfaceCreated, win: uintptr(win), w: int(width), h: int(height)})
}

//export gio_ohos_surface_changed_w
func gio_ohos_surface_changed_w(id C.uint64_t, win unsafe.Pointer, width, height C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvSurfaceChanged, win: uintptr(win), w: int(width), h: int(height)})
}

//export gio_ohos_surface_destroyed_w
func gio_ohos_surface_destroyed_w(id C.uint64_t) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvSurfaceDestroyed})
}

//export gio_ohos_touch_w
func gio_ohos_touch_w(id C.uint64_t, action, pointerID C.int, x, y C.float, mods C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvTouch, action: int(action), id: int(pointerID), x: float32(x), y: float32(y), mods: int(mods)})
}

//export gio_ohos_mouse_w
func gio_ohos_mouse_w(id C.uint64_t, action, button C.int, x, y C.float, mods C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvMouse, action: int(action), code: int(button), x: float32(x), y: float32(y), mods: int(mods)})
}

//export gio_ohos_axis_w
func gio_ohos_axis_w(id C.uint64_t, x, y, dx, dy C.float, mods C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvAxis, x: float32(x), y: float32(y), dx: float32(dx), dy: float32(dy), mods: int(mods)})
}

//export gio_ohos_pinch_w
func gio_ohos_pinch_w(id C.uint64_t, x, y, scale C.float, mods C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvAxis, x: float32(x), y: float32(y), scale: float32(scale), mods: int(mods)})
}

//export gio_ohos_key_w
func gio_ohos_key_w(id C.uint64_t, code, action, mods C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvKey, code: int(code), action: int(action), mods: int(mods)})
}

//export gio_ohos_text_w
func gio_ohos_text_w(id C.uint64_t, utf8 *C.char) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvText, text: cString(utf8)})
}

//export gio_ohos_focus_w
func gio_ohos_focus_w(id C.uint64_t, focused C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvFocus, focus: focused != 0})
}

//export gio_ohos_clipboard_w
func gio_ohos_clipboard_w(id C.uint64_t, mime, text *C.char) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvClipboard, text: cString(mime), text2: cString(text)})
}

//export gio_ohos_clipboard_data_w
func gio_ohos_clipboard_data_w(id C.uint64_t, mime *C.char, data unsafe.Pointer, length C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvClipboardData, text: cString(mime), data: C.GoBytes(data, length)})
}

//export gio_ohos_lifecycle_w
func gio_ohos_lifecycle_w(id C.uint64_t, state C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvLifecycle, life: ohosLifecycle(state)})
}

//export gio_ohos_insets_w
func gio_ohos_insets_w(id C.uint64_t, top, right, bottom, left C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvInsets, top: int(top), right: int(right), bottom: int(bottom), left: int(left)})
}

//export gio_ohos_frame_w
func gio_ohos_frame_w(id C.uint64_t) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvFrame})
}

//export gio_ohos_window_mode_w
func gio_ohos_window_mode_w(id C.uint64_t, mode C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvWindowMode, a: int(mode)})
}

//export gio_ohos_orientation_w
func gio_ohos_orientation_w(id C.uint64_t, orientation C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvOrientation, a: int(orientation)})
}

//export gio_ohos_preedit_w
func gio_ohos_preedit_w(id C.uint64_t, utf8 *C.char, selStart, selEnd C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvPreedit, text: cString(utf8), a: int(selStart), b: int(selEnd)})
}

//export gio_ohos_ime_delete_w
func gio_ohos_ime_delete_w(id C.uint64_t, before, after C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvIMEDelete, a: int(before), b: int(after)})
}

//export gio_ohos_ime_move_w
func gio_ohos_ime_move_w(id C.uint64_t, direction C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvIMEMove, a: int(direction)})
}

//export gio_ohos_ime_set_selection_w
func gio_ohos_ime_set_selection_w(id C.uint64_t, start, end C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvIMESelection, a: int(start), b: int(end)})
}

//export gio_ohos_ime_enter_w
func gio_ohos_ime_enter_w(id C.uint64_t, enterType C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvIMEEnter, a: int(enterType)})
}

//export gio_ohos_ime_keyboard_w
func gio_ohos_ime_keyboard_w(id C.uint64_t, status C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvIMEKeyboard, a: int(status)})
}

//export gio_ohos_drop_w
func gio_ohos_drop_w(id C.uint64_t, uris *C.char) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvDrop, text: cString(uris)})
}

//export gio_ohos_semantic_action_w
func gio_ohos_semantic_action_w(id C.uint64_t, nodeID, action C.int) {
	enqueueOhosFor(uint64(id), ohosEvent{kind: ohosEvSemanticAction, a: int(nodeID), b: int(action)})
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
	targets := ohosState.router.all()
	ohosState.mu.Unlock()
	for _, w := range targets {
		w.enqueue(ohosEvent{kind: ohosEvMetric, metric: m})
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

//export gio_ohos_clipboard_data
func gio_ohos_clipboard_data(mime *C.char, data unsafe.Pointer, length C.int) {
	enqueueOhos(ohosEvent{
		kind: ohosEvClipboardData,
		text: cString(mime),
		data: C.GoBytes(data, length),
	})
}

//export gio_ohos_drop
func gio_ohos_drop(uris *C.char) {
	enqueueOhos(ohosEvent{kind: ohosEvDrop, text: cString(uris)})
}

//export gio_ohos_semantic_action
func gio_ohos_semantic_action(nodeID, action C.int) {
	enqueueOhos(ohosEvent{kind: ohosEvSemanticAction, a: int(nodeID), b: int(action)})
}

//export gio_ohos_permission_result
func gio_ohos_permission_result(requestID, granted C.int, errMsg *C.char) {
	enqueueOhos(ohosEvent{
		kind: ohosEvPermission,
		a:    int(requestID),
		b:    int(granted),
		text: cString(errMsg),
	})
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

// OHOSPickFileWithOptions asks for a picker filtered by MIME types. multiple
// allows selecting more than one file. Requires shim ABI 3; older shims
// fall back to OHOSPickFile behavior.
func OHOSPickFileWithOptions(mode int, mimes []string, multiple bool) {
	cmimes := C.CString(strings.Join(mimes, ","))
	defer C.free(unsafe.Pointer(cmimes))
	C.gio_call_pick_file_ex(C.int(mode), cmimes, boolInt(multiple))
}

// OHOSAnnounce asks the platform accessibility service to announce text.
func OHOSAnnounce(text string) {
	if !ohosA11yEnabled() {
		return
	}
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	C.gio_call_semantics_announce(ctext)
}

// OHOSLog forwards a message to hilog through the shim.
func OHOSLog(msg string) {
	cmsg := C.CString(msg)
	defer C.free(unsafe.Pointer(cmsg))
	C.gio_call_log(cmsg)
}
