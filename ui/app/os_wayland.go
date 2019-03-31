// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android

package app

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"gioui.org/ui/f32"
	"gioui.org/ui/key"
	"gioui.org/ui/pointer"
	"gioui.org/ui"
	syscall "golang.org/x/sys/unix"
)

// Use wayland-scanner to generate glue code for the xdg-shell and xdg-decoration extensions.
//go:generate wayland-scanner client-header /usr/share/wayland-protocols/stable/xdg-shell/xdg-shell.xml wayland_xdg_shell.h
//go:generate wayland-scanner private-code /usr/share/wayland-protocols/stable/xdg-shell/xdg-shell.xml wayland_xdg_shell.c

//go:generate wayland-scanner client-header /usr/share/wayland-protocols/unstable/text-input/text-input-unstable-v3.xml wayland_text_input.h
//go:generate wayland-scanner private-code /usr/share/wayland-protocols/unstable/text-input/text-input-unstable-v3.xml wayland_text_input.c

//go:generate wayland-scanner client-header /usr/share/wayland-protocols/unstable/xdg-decoration/xdg-decoration-unstable-v1.xml wayland_xdg_decoration.h
//go:generate wayland-scanner private-code /usr/share/wayland-protocols/unstable/xdg-decoration/xdg-decoration-unstable-v1.xml wayland_xdg_decoration.c

//go:generate sed -i "1s;^;// +build linux,!android\\n\\n;" wayland_xdg_shell.c
//go:generate sed -i "1s;^;// +build linux,!android\\n\\n;" wayland_xdg_decoration.c
//go:generate sed -i "1s;^;// +build linux,!android\\n\\n;" wayland_text_input.c

/*
#cgo LDFLAGS: -lwayland-client -lwayland-cursor -lxkbcommon

#include <stdlib.h>
#include <wayland-client.h>
#include <wayland-cursor.h>
#include <xkbcommon/xkbcommon.h>
#include <xkbcommon/xkbcommon-compose.h>
#include "wayland_text_input.h"
#include "wayland_xdg_shell.h"
#include "wayland_xdg_decoration.h"
#include "os_wayland.h"
*/
import "C"

type wlConn struct {
	disp         *C.struct_wl_display
	compositor   *C.struct_wl_compositor
	wm           *C.struct_xdg_wm_base
	imm          *C.struct_zwp_text_input_manager_v3
	im           *C.struct_zwp_text_input_v3
	shm          *C.struct_wl_shm
	cursorTheme  *C.struct_wl_cursor_theme
	cursor       *C.struct_wl_cursor
	cursorSurf   *C.struct_wl_surface
	decor        *C.struct_zxdg_decoration_manager_v1
	seat         *C.struct_wl_seat
	seatName     C.uint32_t
	pointer      *C.struct_wl_pointer
	touch        *C.struct_wl_touch
	keyboard     *C.struct_wl_keyboard
	xkb          *C.struct_xkb_context
	xkbMap       *C.struct_xkb_keymap
	xkbState     *C.struct_xkb_state
	xkbCompTable *C.struct_xkb_compose_table
	xkbCompState *C.struct_xkb_compose_state
	utf8Buf      []byte
	repeatRate   int
	repeatDelay  time.Duration
	repeatStop   chan struct{}

	// Cached strings
	_XKB_MOD_NAME_CTRL *C.char
}

type window struct {
	w      *Window
	disp   *C.struct_wl_display
	surf   *C.struct_wl_surface
	wmSurf *C.struct_xdg_surface
	topLvl *C.struct_xdg_toplevel
	decor  *C.struct_zxdg_toplevel_decoration_v1
	// Notification pipe fds.
	notRead, notWrite int
	ppdp, ppsp        float32
	scrollTime        time.Duration
	discScroll        struct {
		x, y int
	}
	scroll    f32.Point
	lastPos   f32.Point
	lastTouch f32.Point

	stage             Stage
	lastFrameCallback *C.struct_wl_callback

	mu        sync.Mutex
	animating bool
	needAck   bool
	// The last configure serial waiting to be ack'ed.
	serial   C.uint32_t
	width    int
	height   int
	newScale bool
	scale    int
}

type wlOutput struct {
	width      int
	height     int
	physWidth  int
	physHeight int
	transform  C.int32_t
	scale      int
	windows    []*window
}

var connMu sync.Mutex
var conn *wlConn
var mainDone = make(chan struct{})

var (
	winMap       = make(map[interface{}]*window)
	outputMap    = make(map[C.uint32_t]*C.struct_wl_output)
	outputConfig = make(map[*C.struct_wl_output]*wlOutput)
)

func Main() {
	<-mainDone
}

func createWindow(opts *WindowOptions) (*Window, error) {
	connMu.Lock()
	defer connMu.Unlock()
	if len(winMap) > 0 {
		panic("multiple windows are not supported")
	}
	if err := waylandConnect(); err != nil {
		return nil, err
	}
	w, err := createNativeWindow(opts)
	if err != nil {
		conn.destroy()
		return nil, err
	}
	go func() {
		w.setStage(StageVisible)
		w.loop()
		w.destroy()
		conn.destroy()
		close(mainDone)
	}()
	return w.w, nil
}

func createNativeWindow(opts *WindowOptions) (*window, error) {
	pipe := make([]int, 2)
	if err := syscall.Pipe2(pipe, syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		return nil, fmt.Errorf("createNativeWindow: failed to create pipe: %v", err)
	}

	fontScale := detectFontScale()
	var ppmm float32
	var scale int
	for _, conf := range outputConfig {
		if d, err := conf.ppmm(); err == nil && d > ppmm {
			ppmm = d
		}
		if s := conf.scale; s > scale {
			scale = s
		}
	}
	ppdp := ppmm * mmPrDp * monitorScale
	if ppdp < minDensity {
		ppdp = minDensity
	}

	w := &window{
		disp:     conn.disp,
		scale:    scale,
		newScale: scale != 1,
		ppdp:     ppdp,
		ppsp:     ppdp * fontScale,
		notRead:  pipe[0],
		notWrite: pipe[1],
	}
	w.surf = C.wl_compositor_create_surface(conn.compositor)
	if w.surf == nil {
		w.destroy()
		return nil, errors.New("wayland: wl_compositor_create_surface failed")
	}
	w.wmSurf = C.xdg_wm_base_get_xdg_surface(conn.wm, w.surf)
	if w.wmSurf == nil {
		w.destroy()
		return nil, errors.New("wayland: xdg_wm_base_get_xdg_surface failed")
	}
	w.topLvl = C.xdg_surface_get_toplevel(w.wmSurf)
	if w.topLvl == nil {
		w.destroy()
		return nil, errors.New("wayland: xdg_surface_get_toplevel failed")
	}
	C.gio_xdg_wm_base_add_listener(conn.wm)
	C.gio_wl_surface_add_listener(w.surf)
	C.gio_xdg_surface_add_listener(w.wmSurf)
	C.gio_xdg_toplevel_add_listener(w.topLvl)
	title := C.CString(opts.Title)
	C.xdg_toplevel_set_title(w.topLvl, title)
	C.free(unsafe.Pointer(title))

	_, _, cfg := w.config()
	w.width = int(cfg.Pixels(opts.Width) + .5)
	w.height = int(cfg.Pixels(opts.Height) + .5)
	if conn.decor != nil {
		// Request server side decorations.
		w.decor = C.zxdg_decoration_manager_v1_get_toplevel_decoration(conn.decor, w.topLvl)
		C.zxdg_toplevel_decoration_v1_set_mode(w.decor, C.ZXDG_TOPLEVEL_DECORATION_V1_MODE_SERVER_SIDE)
	}
	w.updateOpaqueRegion()
	C.wl_surface_commit(w.surf)
	ow := newWindow(w)
	w.w = ow
	winMap[w.topLvl] = w
	winMap[w.surf] = w
	winMap[w.wmSurf] = w
	return w, nil
}

//export gio_onSeatCapabilities
func gio_onSeatCapabilities(data unsafe.Pointer, seat *C.struct_wl_seat, caps C.uint32_t) {
	if seat != conn.seat {
		panic("unexpected seat")
	}
	if conn.im == nil && conn.imm != nil {
		conn.im = C.zwp_text_input_manager_v3_get_text_input(conn.imm, conn.seat)
		C.gio_zwp_text_input_v3_add_listener(conn.im)
	}
	switch {
	case conn.pointer == nil && caps&C.WL_SEAT_CAPABILITY_POINTER != 0:
		conn.pointer = C.wl_seat_get_pointer(seat)
		C.gio_wl_pointer_add_listener(conn.pointer)
	case conn.pointer != nil && caps&C.WL_SEAT_CAPABILITY_POINTER == 0:
		C.wl_pointer_release(conn.pointer)
		conn.pointer = nil
	}
	switch {
	case conn.touch == nil && caps&C.WL_SEAT_CAPABILITY_TOUCH != 0:
		conn.touch = C.wl_seat_get_touch(seat)
		C.gio_wl_touch_add_listener(conn.touch)
	case conn.touch != nil && caps&C.WL_SEAT_CAPABILITY_TOUCH == 0:
		C.wl_touch_release(conn.touch)
		conn.touch = nil
	}
	switch {
	case conn.keyboard == nil && caps&C.WL_SEAT_CAPABILITY_KEYBOARD != 0:
		conn.keyboard = C.wl_seat_get_keyboard(seat)
		C.gio_wl_keyboard_add_listener(conn.keyboard)
	case conn.keyboard != nil && caps&C.WL_SEAT_CAPABILITY_KEYBOARD == 0:
		C.wl_keyboard_release(conn.keyboard)
		conn.keyboard = nil
	}
}

//export gio_onSeatName
func gio_onSeatName(data unsafe.Pointer, seat *C.struct_wl_seat, name *C.char) {
}

//export gio_onXdgSurfaceConfigure
func gio_onXdgSurfaceConfigure(data unsafe.Pointer, wmSurf *C.struct_xdg_surface, serial C.uint32_t) {
	w := winMap[wmSurf]
	w.mu.Lock()
	w.serial = serial
	w.needAck = true
	w.mu.Unlock()
	w.draw(true)
}

//export gio_onToplevelClose
func gio_onToplevelClose(data unsafe.Pointer, topLvl *C.struct_xdg_toplevel) {
	w := winMap[topLvl]
	w.setStage(StageDead)
}

//export gio_onToplevelConfigure
func gio_onToplevelConfigure(data unsafe.Pointer, topLvl *C.struct_xdg_toplevel, width, height C.int32_t, states *C.struct_wl_array) {
	w := winMap[topLvl]
	if width != 0 && height != 0 {
		w.mu.Lock()
		defer w.mu.Unlock()
		w.width = int(width)
		w.height = int(height)
		w.updateOpaqueRegion()
	}
}

//export gio_onOutputMode
func gio_onOutputMode(data unsafe.Pointer, output *C.struct_wl_output, flags C.uint32_t, width, height, refresh C.int32_t) {
	if flags&C.WL_OUTPUT_MODE_CURRENT == 0 {
		return
	}
	c := outputConfig[output]
	c.width = int(width)
	c.height = int(height)
}

//export gio_onOutputGeometry
func gio_onOutputGeometry(data unsafe.Pointer, output *C.struct_wl_output, x, y, physWidth, physHeight, subpixel C.int32_t, make, model *C.char, transform C.int32_t) {
	c := outputConfig[output]
	c.transform = transform
	c.physWidth = int(physWidth)
	c.physHeight = int(physHeight)
}

//export gio_onOutputScale
func gio_onOutputScale(data unsafe.Pointer, output *C.struct_wl_output, scale C.int32_t) {
	c := outputConfig[output]
	c.scale = int(scale)
}

//export gio_onOutputDone
func gio_onOutputDone(data unsafe.Pointer, output *C.struct_wl_output) {
	conf := outputConfig[output]
	for _, w := range conf.windows {
		w.draw(true)
	}
}

//export gio_onSurfaceEnter
func gio_onSurfaceEnter(data unsafe.Pointer, surf *C.struct_wl_surface, output *C.struct_wl_output) {
	w := winMap[surf]
	conf := outputConfig[output]
	var found bool
	for _, w2 := range conf.windows {
		if w2 == w {
			found = true
			break
		}
	}
	if !found {
		conf.windows = append(conf.windows, w)
	}
	w.updateOutputs()
}

//export gio_onSurfaceLeave
func gio_onSurfaceLeave(data unsafe.Pointer, surf *C.struct_wl_surface, output *C.struct_wl_output) {
	w := winMap[surf]
	conf := outputConfig[output]
	for i, w2 := range conf.windows {
		if w2 == w {
			conf.windows = append(conf.windows[:i], conf.windows[i+1:]...)
			break
		}
	}
	w.updateOutputs()
}

//export gio_onRegistryGlobal
func gio_onRegistryGlobal(data unsafe.Pointer, reg *C.struct_wl_registry, name C.uint32_t, cintf *C.char, version C.uint32_t) {
	switch C.GoString(cintf) {
	case "wl_compositor":
		conn.compositor = (*C.struct_wl_compositor)(C.wl_registry_bind(reg, name, &C.wl_compositor_interface, 3))
	case "wl_output":
		output := (*C.struct_wl_output)(C.wl_registry_bind(reg, name, &C.wl_output_interface, 2))
		C.gio_wl_output_add_listener(output)
		outputMap[name] = output
		outputConfig[output] = new(wlOutput)
	case "wl_seat":
		if conn.seat == nil {
			conn.seatName = name
			conn.seat = (*C.struct_wl_seat)(C.wl_registry_bind(reg, name, &C.wl_seat_interface, 5))
			C.gio_wl_seat_add_listener(conn.seat)
		}
	case "wl_shm":
		conn.shm = (*C.struct_wl_shm)(C.wl_registry_bind(reg, name, &C.wl_shm_interface, 1))
	case "xdg_wm_base":
		conn.wm = (*C.struct_xdg_wm_base)(C.wl_registry_bind(reg, name, &C.xdg_wm_base_interface, 1))
	case "zxdg_decoration_manager_v1":
		conn.decor = (*C.struct_zxdg_decoration_manager_v1)(C.wl_registry_bind(reg, name, &C.zxdg_decoration_manager_v1_interface, 1))
		// TODO: Implement and test text-input support.
		/*case "zwp_text_input_manager_v3":
		conn.imm = (*C.struct_zwp_text_input_manager_v3)(C.wl_registry_bind(reg, name, &C.zwp_text_input_manager_v3_interface, 1))*/
	}
}

//export gio_onRegistryGlobalRemove
func gio_onRegistryGlobalRemove(data unsafe.Pointer, reg *C.struct_wl_registry, name C.uint32_t) {
	if conn.seat != nil && name == conn.seatName {
		if conn.im != nil {
			C.zwp_text_input_v3_destroy(conn.im)
			conn.im = nil
		}
		if conn.pointer != nil {
			delete(winMap, conn.pointer)
		}
		if conn.touch != nil {
			delete(winMap, conn.touch)
		}
		if conn.keyboard != nil {
			delete(winMap, conn.keyboard)
		}
		C.wl_seat_release(conn.seat)
		conn.seat = nil
	}
	if output, exists := outputMap[name]; exists {
		C.wl_output_destroy(output)
		delete(outputMap, name)
		delete(outputConfig, output)
	}
}

//export gio_onTouchDown
func gio_onTouchDown(data unsafe.Pointer, touch *C.struct_wl_touch, serial, t C.uint32_t, surf *C.struct_wl_surface, id C.int32_t, x, y C.wl_fixed_t) {
	w := winMap[surf]
	winMap[touch] = w
	w.lastTouch = f32.Point{X: fromFixed(x), Y: fromFixed(y)}
	w.w.event(pointer.Event{
		Type:      pointer.Press,
		Source:    pointer.Touch,
		Position:  w.lastTouch,
		PointerID: pointer.ID(id),
		Time:      time.Duration(t) * time.Millisecond,
	})
}

//export gio_onTouchUp
func gio_onTouchUp(data unsafe.Pointer, touch *C.struct_wl_touch, serial, t C.uint32_t, id C.int32_t) {
	w := winMap[touch]
	w.w.event(pointer.Event{
		Type:      pointer.Release,
		Source:    pointer.Touch,
		Position:  w.lastTouch,
		PointerID: pointer.ID(id),
		Time:      time.Duration(t) * time.Millisecond,
	})
}

//export gio_onTouchMotion
func gio_onTouchMotion(data unsafe.Pointer, touch *C.struct_wl_touch, t C.uint32_t, id C.int32_t, x, y C.wl_fixed_t) {
	w := winMap[touch]
	w.lastTouch = f32.Point{X: fromFixed(x), Y: fromFixed(y)}
	w.w.event(pointer.Event{
		Type:      pointer.Move,
		Position:  w.lastTouch,
		Source:    pointer.Touch,
		PointerID: pointer.ID(id),
		Time:      time.Duration(t) * time.Millisecond,
	})
}

//export gio_onTouchFrame
func gio_onTouchFrame(data unsafe.Pointer, touch *C.struct_wl_touch) {
}

//export gio_onTouchCancel
func gio_onTouchCancel(data unsafe.Pointer, touch *C.struct_wl_touch) {
	w := winMap[touch]
	w.w.event(pointer.Event{
		Type:   pointer.Cancel,
		Source: pointer.Touch,
	})
}

//export gio_onTouchShape
func gio_onTouchShape(data unsafe.Pointer, touch *C.struct_wl_touch, id C.int32_t, major, minor C.wl_fixed_t) {
}

//export gio_onTouchOrientation
func gio_onTouchOrientation(data unsafe.Pointer, touch *C.struct_wl_touch, id C.int32_t, orientation C.wl_fixed_t) {
}

//export gio_onPointerEnter
func gio_onPointerEnter(data unsafe.Pointer, pointer *C.struct_wl_pointer, serial C.uint32_t, surf *C.struct_wl_surface, x, y C.wl_fixed_t) {
	// Get images[0].
	img := *conn.cursor.images
	buf := C.wl_cursor_image_get_buffer(img)
	if buf == nil {
		return
	}
	C.wl_pointer_set_cursor(pointer, serial, conn.cursorSurf, C.int32_t(img.hotspot_x), C.int32_t(img.hotspot_y))
	C.wl_surface_attach(conn.cursorSurf, buf, 0, 0)
	C.wl_surface_damage(conn.cursorSurf, 0, 0, C.int32_t(img.width), C.int32_t(img.height))
	C.wl_surface_commit(conn.cursorSurf)
	w := winMap[surf]
	winMap[pointer] = w
	w.lastPos = f32.Point{X: fromFixed(x), Y: fromFixed(y)}
}

//export gio_onPointerLeave
func gio_onPointerLeave(data unsafe.Pointer, p *C.struct_wl_pointer, serial C.uint32_t, surface *C.struct_wl_surface) {
}

//export gio_onPointerMotion
func gio_onPointerMotion(data unsafe.Pointer, p *C.struct_wl_pointer, t C.uint32_t, x, y C.wl_fixed_t) {
	w := winMap[p]
	w.onPointerMotion(x, y, t)
}

//export gio_onPointerButton
func gio_onPointerButton(data unsafe.Pointer, p *C.struct_wl_pointer, serial, t, button, state C.uint32_t) {
	w := winMap[p]
	// From linux-event-codes.h.
	const BTN_LEFT = 0x110
	if button != BTN_LEFT {
		return
	}
	var typ pointer.Type
	switch state {
	case 0:
		typ = pointer.Release
	case 1:
		typ = pointer.Press
	}
	w.flushScroll()
	w.w.event(pointer.Event{
		Type:     typ,
		Source:   pointer.Mouse,
		Position: w.lastPos,
		Time:     time.Duration(t) * time.Millisecond,
	})
}

//export gio_onPointerAxis
func gio_onPointerAxis(data unsafe.Pointer, ptr *C.struct_wl_pointer, t, axis C.uint32_t, value C.wl_fixed_t) {
	w := winMap[ptr]
	v := fromFixed(value)
	if w.scroll == (f32.Point{}) {
		w.scrollTime = time.Duration(t) * time.Millisecond
	}
	switch axis {
	case C.WL_POINTER_AXIS_HORIZONTAL_SCROLL:
		w.scroll.X += v
	case C.WL_POINTER_AXIS_VERTICAL_SCROLL:
		w.scroll.Y += v
	}
}

//export gio_onPointerFrame
func gio_onPointerFrame(data unsafe.Pointer, pointer *C.struct_wl_pointer) {
	w := winMap[pointer]
	w.flushScroll()
}

//export gio_onPointerAxisSource
func gio_onPointerAxisSource(data unsafe.Pointer, pointer *C.struct_wl_pointer, source C.uint32_t) {
}

//export gio_onPointerAxisStop
func gio_onPointerAxisStop(data unsafe.Pointer, pointer *C.struct_wl_pointer, time, axis C.uint32_t) {
}

//export gio_onPointerAxisDiscrete
func gio_onPointerAxisDiscrete(data unsafe.Pointer, pointer *C.struct_wl_pointer, axis C.uint32_t, discrete C.int32_t) {
	w := winMap[pointer]
	switch axis {
	case C.WL_POINTER_AXIS_HORIZONTAL_SCROLL:
		w.discScroll.x += int(discrete)
	case C.WL_POINTER_AXIS_VERTICAL_SCROLL:
		w.discScroll.y += int(discrete)
	}
}

//export gio_onKeyboardKeymap
func gio_onKeyboardKeymap(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, format C.uint32_t, fd C.int32_t, size C.uint32_t) {
	conn.stopRepeat()
	defer syscall.Close(int(fd))
	if conn.xkbCompState != nil {
		C.xkb_compose_state_unref(conn.xkbCompState)
		conn.xkbCompState = nil
	}
	if conn.xkbCompTable != nil {
		C.xkb_compose_table_unref(conn.xkbCompTable)
		conn.xkbCompTable = nil
	}
	if conn.xkbState != nil {
		C.xkb_state_unref(conn.xkbState)
		conn.xkbState = nil
	}
	if conn.xkbMap != nil {
		C.xkb_keymap_unref(conn.xkbMap)
		conn.xkbMap = nil
	}
	if format != C.WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1 {
		return
	}
	if conn.xkb == nil {
		conn.xkb = C.xkb_context_new(C.XKB_CONTEXT_NO_FLAGS)
	}
	if conn.xkb == nil {
		return
	}
	if conn.xkbCompTable == nil {
		locale := os.Getenv("LC_ALL")
		if locale == "" {
			locale = os.Getenv("LC_CTYPE")
		}
		if locale == "" {
			locale = os.Getenv("LANG")
		}
		if locale == "" {
			locale = "C"
		}
		cloc := C.CString(locale)
		defer C.free(unsafe.Pointer(cloc))
		conn.xkbCompTable = C.xkb_compose_table_new_from_locale(conn.xkb, cloc, C.XKB_COMPOSE_COMPILE_NO_FLAGS)
		if conn.xkbCompTable != nil {
			conn.xkbCompState = C.xkb_compose_state_new(conn.xkbCompTable, C.XKB_COMPOSE_STATE_NO_FLAGS)
		}
	}
	mapData, err := syscall.Mmap(int(fd), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return
	}
	defer syscall.Munmap(mapData)
	conn.xkbMap = C.xkb_keymap_new_from_buffer(conn.xkb, (*C.char)(unsafe.Pointer(&mapData[0])), C.size_t(size-1), C.XKB_KEYMAP_FORMAT_TEXT_V1, C.XKB_KEYMAP_COMPILE_NO_FLAGS)
	if conn.xkbMap == nil {
		return
	}
	conn.xkbState = C.xkb_state_new(conn.xkbMap)
}

//export gio_onKeyboardEnter
func gio_onKeyboardEnter(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, serial C.uint32_t, surf *C.struct_wl_surface, keys *C.struct_wl_array) {
	conn.stopRepeat()
	w := winMap[surf]
	winMap[keyboard] = w
}

//export gio_onKeyboardLeave
func gio_onKeyboardLeave(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, serial C.uint32_t, surf *C.struct_wl_surface) {
	conn.stopRepeat()
}

//export gio_onKeyboardKey
func gio_onKeyboardKey(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, serial, timestamp, keyCode, state C.uint32_t) {
	conn.stopRepeat()
	w := winMap[keyboard]
	if state != C.WL_KEYBOARD_KEY_STATE_PRESSED || conn.xkbMap == nil || conn.xkbState == nil || conn.xkbCompState == nil {
		return
	}
	// According to the xkb_v1 spec: "to determine the xkb keycode, clients must add 8 to the key event keycode."
	keyCode += 8
	w.dispatchKey(keyCode)
	if conn.repeatRate > 0 && C.xkb_keymap_key_repeats(conn.xkbMap, C.xkb_keycode_t(keyCode)) == 1 {
		stop := make(chan struct{})
		conn.repeatStop = stop
		rate, delay := conn.repeatRate, conn.repeatDelay
		go func() {
			timer := time.NewTimer(delay)
			for {
				select {
				case <-timer.C:
				case <-stop:
					close(stop)
					return
				}
				w.dispatchKey(keyCode)
				delay = time.Second / time.Duration(rate)
				timer.Reset(delay)
			}
		}()
	}
}

//export gio_onFrameDone
func gio_onFrameDone(data unsafe.Pointer, callback *C.struct_wl_callback, t C.uint32_t) {
	C.wl_callback_destroy(callback)
	surf := (*C.struct_wl_surface)(data)
	w := winMap[surf]
	if w.lastFrameCallback == callback {
		w.lastFrameCallback = nil
		w.draw(false)
	}
}

func (w *window) loop() {
	dispfd := C.wl_display_get_fd(conn.disp)
	// Poll for events and notifications.
	pollfds := []syscall.PollFd{
		{Fd: int32(dispfd), Events: syscall.POLLIN | syscall.POLLERR},
		{Fd: int32(w.notRead), Events: syscall.POLLIN | syscall.POLLERR},
	}
	dispEvents := &pollfds[0].Revents
	// Plenty of room for a backlog of notifications.
	var buf = make([]byte, 100)
loop:
	for {
		C.wl_display_dispatch_pending(conn.disp)
		if ret := C.wl_display_flush(conn.disp); ret < 0 {
			break
		}
		if w.stage == StageDead {
			break
		}
		// Clear poll events.
		*dispEvents = 0
		if _, err := syscall.Ppoll(pollfds, nil, nil); err != nil && err != syscall.EINTR {
			panic(fmt.Errorf("ppoll failed: %v", err))
		}
		redraw := false
		// Clear notifications.
		for {
			_, err := syscall.Read(w.notRead, buf)
			if err == syscall.EAGAIN {
				break
			}
			if err != nil {
				panic(fmt.Errorf("read from notify pipe failed: %v", err))
			}
			redraw = true
		}
		// Handle events
		switch {
		case *dispEvents&syscall.POLLIN != 0:
			if ret := C.wl_display_dispatch(conn.disp); ret < 0 {
				break loop
			}
		case *dispEvents&(syscall.POLLERR|syscall.POLLHUP) != 0:
			break loop
		}
		if redraw {
			w.draw(false)
		}
	}
}

func (w *window) setAnimating(anim bool) {
	w.mu.Lock()
	w.animating = anim
	w.mu.Unlock()
	if anim {
		w.notify()
	}
}

// Wakeup wakes up the event loop through the notification pipe.
func (w *window) notify() {
	oneByte := make([]byte, 1)
	if _, err := syscall.Write(w.notWrite, oneByte); err != nil && err != syscall.EAGAIN {
		panic(fmt.Errorf("failed to write to pipe: %v", err))
	}
}

func (w *window) destroy() {
	if w.notWrite != 0 {
		syscall.Close(w.notWrite)
		w.notWrite = 0
	}
	if w.notRead != 0 {
		syscall.Close(w.notRead)
		w.notRead = 0
	}
	if w.topLvl != nil {
		delete(winMap, w.topLvl)
		C.xdg_toplevel_destroy(w.topLvl)
	}
	if w.surf != nil {
		delete(winMap, w.surf)
		C.wl_surface_destroy(w.surf)
	}
	if w.wmSurf != nil {
		delete(winMap, w.wmSurf)
		C.xdg_surface_destroy(w.wmSurf)
	}
	if w.decor != nil {
		C.zxdg_toplevel_decoration_v1_destroy(w.decor)
	}
}

func (w *window) dispatchKey(keyCode C.uint32_t) {
	if len(conn.utf8Buf) == 0 {
		conn.utf8Buf = make([]byte, 1)
	}
	sym := C.xkb_state_key_get_one_sym(conn.xkbState, C.xkb_keycode_t(keyCode))
	if n, ok := convertKeysym(sym); ok {
		cmd := key.Chord{Name: n}
		if C.xkb_state_mod_name_is_active(conn.xkbState, conn._XKB_MOD_NAME_CTRL, C.XKB_STATE_MODS_EFFECTIVE) == 1 {
			cmd.Modifiers |= key.ModCommand
		}
		w.w.event(cmd)
	}
	C.xkb_compose_state_feed(conn.xkbCompState, sym)
	var size C.int
	switch C.xkb_compose_state_get_status(conn.xkbCompState) {
	case C.XKB_COMPOSE_CANCELLED, C.XKB_COMPOSE_COMPOSING:
		return
	case C.XKB_COMPOSE_COMPOSED:
		size = C.xkb_compose_state_get_utf8(conn.xkbCompState, (*C.char)(unsafe.Pointer(&conn.utf8Buf[0])), C.size_t(len(conn.utf8Buf)))
		if int(size) >= len(conn.utf8Buf) {
			conn.utf8Buf = make([]byte, size+1)
			size = C.xkb_compose_state_get_utf8(conn.xkbCompState, (*C.char)(unsafe.Pointer(&conn.utf8Buf[0])), C.size_t(len(conn.utf8Buf)))
		}
		C.xkb_compose_state_reset(conn.xkbCompState)
	case C.XKB_COMPOSE_NOTHING:
		size = C.xkb_state_key_get_utf8(conn.xkbState, C.xkb_keycode_t(keyCode), (*C.char)(unsafe.Pointer(&conn.utf8Buf[0])), C.size_t(len(conn.utf8Buf)))
		if int(size) >= len(conn.utf8Buf) {
			conn.utf8Buf = make([]byte, size+1)
			size = C.xkb_state_key_get_utf8(conn.xkbState, C.xkb_keycode_t(keyCode), (*C.char)(unsafe.Pointer(&conn.utf8Buf[0])), C.size_t(len(conn.utf8Buf)))
		}
	}
	// Report only printable runes.
	str := conn.utf8Buf[:size]
	var n int
	for n < len(str) {
		r, s := utf8.DecodeRune(str)
		if unicode.IsPrint(r) {
			n += s
		} else {
			copy(str[n:], str[n+s:])
			str = str[:len(str)-s]
		}
	}
	if len(str) > 0 {
		w.w.event(key.Edit{Text: string(str)})
	}
}

//export gio_onKeyboardModifiers
func gio_onKeyboardModifiers(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, serial, depressed, latched, locked, group C.uint32_t) {
	conn.stopRepeat()
	if conn.xkbState == nil {
		return
	}
	xkbGrp := C.xkb_layout_index_t(group)
	C.xkb_state_update_mask(conn.xkbState, C.xkb_mod_mask_t(depressed), C.xkb_mod_mask_t(latched), C.xkb_mod_mask_t(locked), xkbGrp, xkbGrp, xkbGrp)
}

//export gio_onKeyboardRepeatInfo
func gio_onKeyboardRepeatInfo(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, rate, delay C.int32_t) {
	conn.repeatRate = int(rate)
	conn.repeatDelay = time.Duration(delay) * time.Millisecond
}

//export gio_onTextInputEnter
func gio_onTextInputEnter(data unsafe.Pointer, im *C.struct_zwp_text_input_v3, surf *C.struct_wl_surface) {
}

//export gio_onTextInputLeave
func gio_onTextInputLeave(data unsafe.Pointer, im *C.struct_zwp_text_input_v3, surf *C.struct_wl_surface) {
}

//export gio_onTextInputPreeditString
func gio_onTextInputPreeditString(data unsafe.Pointer, im *C.struct_zwp_text_input_v3, ctxt *C.char, begin, end C.int32_t) {
}

//export gio_onTextInputCommitString
func gio_onTextInputCommitString(data unsafe.Pointer, im *C.struct_zwp_text_input_v3, ctxt *C.char) {
}

//export gio_onTextInputDeleteSurroundingText
func gio_onTextInputDeleteSurroundingText(data unsafe.Pointer, im *C.struct_zwp_text_input_v3, before, after C.uint32_t) {
}

//export gio_onTextInputDone
func gio_onTextInputDone(data unsafe.Pointer, im *C.struct_zwp_text_input_v3, serial C.uint32_t) {
}

// ppmm returns the approximate pixels per millimeter for the output.
func (c *wlOutput) ppmm() (float32, error) {
	if c.physWidth == 0 || c.physHeight == 0 {
		return 0, errors.New("no physical size data for output")
	}
	// Because of https://gitlab.gnome.org/GNOME/mutter/issues/369, output dimensions might be undetectably swapped.
	// Instead, compute and return sqrt(px²/mm²).
	density := float32(math.Sqrt(float64(c.width*c.height) / float64(c.physWidth*c.physHeight)))
	return density, nil
}

func (w *window) flushScroll() {
	if w.scroll == (f32.Point{}) {
		return
	}
	// The Wayland reported scroll distance for
	// discrete scroll axis is only 10 pixels, where
	// 100 seems more appropriate.
	const discreteScale = 10
	if w.discScroll.x != 0 {
		w.scroll.X *= discreteScale
	}
	if w.discScroll.y != 0 {
		w.scroll.Y *= discreteScale
	}
	w.w.event(pointer.Event{
		Type:     pointer.Move,
		Source:   pointer.Mouse,
		Position: w.lastPos,
		Scroll:   w.scroll,
		Time:     w.scrollTime,
	})
	w.scroll = f32.Point{}
	w.discScroll.x = 0
	w.discScroll.y = 0
}

func (w *window) onPointerMotion(x, y C.wl_fixed_t, t C.uint32_t) {
	w.flushScroll()
	w.lastPos = f32.Point{X: fromFixed(x), Y: fromFixed(y)}
	w.w.event(pointer.Event{
		Type:     pointer.Move,
		Position: w.lastPos,
		Source:   pointer.Mouse,
		Time:     time.Duration(t) * time.Millisecond,
	})
}

func (w *window) updateOpaqueRegion() {
	reg := C.wl_compositor_create_region(conn.compositor)
	C.wl_region_add(reg, 0, 0, C.int32_t(w.width), C.int32_t(w.height))
	C.wl_surface_set_opaque_region(w.surf, reg)
	C.wl_region_destroy(reg)
}

func (w *window) updateOutputs() {
	scale := 1
	var found bool
	for _, conf := range outputConfig {
		for _, w2 := range conf.windows {
			if w2 == w {
				found = true
				if conf.scale > scale {
					scale = conf.scale
				}
			}
		}
	}
	w.mu.Lock()
	if found && scale != w.scale {
		w.scale = scale
		w.newScale = true
	}
	w.mu.Unlock()
	if !found {
		w.setStage(StageInvisible)
	} else {
		w.setStage(StageVisible)
		w.draw(true)
	}
}

func (w *window) config() (int, int, ui.Config) {
	width, height := w.width*w.scale, w.height*w.scale
	return width, height, ui.Config{
		PxPerDp: w.ppdp * float32(w.scale),
		PxPerSp: w.ppsp * float32(w.scale),
	}
}

func (w *window) draw(sync bool) {
	w.mu.Lock()
	animating := w.animating
	w.mu.Unlock()
	width, height, cfg := w.config()
	if cfg == (ui.Config{}) {
		return
	}
	if animating && w.lastFrameCallback == nil {
		w.lastFrameCallback = C.wl_surface_frame(w.surf)
		// Use the surface as listener data for gio_onFrameDone.
		C.gio_wl_callback_add_listener(w.lastFrameCallback, unsafe.Pointer(w.surf))
	}
	cfg.Now = time.Now()
	w.w.event(Draw{
		Size: image.Point{
			X: width,
			Y: height,
		},
		Config: &cfg,
		sync:   sync,
	})
}

func (w *window) setStage(s Stage) {
	if s == w.stage {
		return
	}
	w.stage = s
	w.w.event(ChangeStage{s})
}

func (w *window) display() unsafe.Pointer {
	return unsafe.Pointer(w.disp)
}

func (w *window) nativeWindow(visID int) (unsafe.Pointer, int, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.needAck {
		C.xdg_surface_ack_configure(w.wmSurf, w.serial)
		w.needAck = false
	}
	width, height, scale := w.width, w.height, w.scale
	if w.newScale {
		C.wl_surface_set_buffer_scale(w.surf, C.int32_t(scale))
		w.newScale = false
	}
	return unsafe.Pointer(w.surf), width * scale, height * scale
}

func (w *window) setTextInput(s key.TextInputState) {}

// detectFontScale reports current font scale, or 1.0
// if it fails.
func detectFontScale() float32 {
	// TODO: What about other window environments?
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "text-scaling-factor").Output()
	if err != nil {
		return 1.0
	}
	scale, err := strconv.ParseFloat(string(bytes.TrimSpace(out)), 32)
	if err != nil {
		return 1.0
	}
	return float32(scale)
}

func waylandConnect() error {
	c := new(wlConn)
	conn = c
	c.disp = C.wl_display_connect(nil)
	if c.disp == nil {
		c.destroy()
		return errors.New("wayland: wl_display_connect failed")
	}
	reg := C.wl_display_get_registry(c.disp)
	if reg == nil {
		c.destroy()
		return errors.New("wayland: wl_display_get_registry failed")
	}
	C.gio_wl_registry_add_listener(reg)
	// Get globals.
	C.wl_display_roundtrip(c.disp)
	// Get output configurations.
	C.wl_display_roundtrip(c.disp)
	if c.compositor == nil {
		c.destroy()
		return errors.New("wayland: no compositor available")
	}
	if c.wm == nil {
		c.destroy()
		return errors.New("wayland: no xdg_wm_base available")
	}
	if c.shm == nil {
		c.destroy()
		return errors.New("wayland: no wl_shm available")
	}
	if len(outputMap) == 0 {
		c.destroy()
		return errors.New("wayland: no outputs available")
	}
	c.cursorTheme = C.wl_cursor_theme_load(nil, 32, c.shm)
	if c.cursorTheme == nil {
		c.destroy()
		return errors.New("wayland: wl_cursor_theme_load failed")
	}
	cname := C.CString("left_ptr")
	defer C.free(unsafe.Pointer(cname))
	c.cursor = C.wl_cursor_theme_get_cursor(c.cursorTheme, cname)
	if c.cursor == nil {
		c.destroy()
		return errors.New("wayland: wl_cursor_theme_get_cursor failed")
	}
	c.cursorSurf = C.wl_compositor_create_surface(conn.compositor)
	if c.cursorSurf == nil {
		c.destroy()
		return errors.New("wayland: wl_compositor_create_surface failed")
	}
	c._XKB_MOD_NAME_CTRL = C.CString(C.XKB_MOD_NAME_CTRL)
	return nil
}

func (c *wlConn) stopRepeat() {
	if c.repeatStop == nil {
		return
	}
	c.repeatStop <- struct{}{}
	<-c.repeatStop
	c.repeatStop = nil
}

func (c *wlConn) destroy() {
	c.stopRepeat()
	if c._XKB_MOD_NAME_CTRL != nil {
		C.free(unsafe.Pointer(c._XKB_MOD_NAME_CTRL))
		c._XKB_MOD_NAME_CTRL = nil
	}
	if c.xkbCompState != nil {
		C.xkb_compose_state_unref(c.xkbCompState)
		c.xkbCompState = nil
	}
	if c.xkbCompTable != nil {
		C.xkb_compose_table_unref(c.xkbCompTable)
		c.xkbCompTable = nil
	}
	if c.xkbState != nil {
		C.xkb_state_unref(conn.xkbState)
	}
	if c.xkbMap != nil {
		C.xkb_keymap_unref(c.xkbMap)
	}
	if c.xkb != nil {
		C.xkb_context_unref(c.xkb)
	}
	if c.cursorSurf != nil {
		C.wl_surface_destroy(c.cursorSurf)
	}
	if c.cursorTheme != nil {
		C.wl_cursor_theme_destroy(c.cursorTheme)
	}
	if c.keyboard != nil {
		C.wl_keyboard_release(c.keyboard)
	}
	if c.pointer != nil {
		C.wl_pointer_release(c.pointer)
	}
	if c.touch != nil {
		C.wl_touch_release(c.touch)
	}
	if c.im != nil {
		C.zwp_text_input_v3_destroy(c.im)
	}
	if c.imm != nil {
		C.zwp_text_input_manager_v3_destroy(c.imm)
	}
	if c.seat != nil {
		C.wl_seat_release(c.seat)
	}
	if c.decor != nil {
		C.zxdg_decoration_manager_v1_destroy(c.decor)
	}
	if c.shm != nil {
		C.wl_shm_destroy(c.shm)
	}
	if c.compositor != nil {
		C.wl_compositor_destroy(c.compositor)
	}
	if c.wm != nil {
		C.xdg_wm_base_destroy(c.wm)
	}
	for _, output := range outputMap {
		C.wl_output_destroy(output)
	}
	if c.disp != nil {
		C.wl_display_disconnect(c.disp)
	}
}

// fromFixed converts a Wayland wl_fixed_t 23.8 number to float32.
func fromFixed(v C.wl_fixed_t) float32 {
	// Convert to float64 to avoid overflow.
	// From wayland-util.h.
	b := ((1023 + 44) << 52) + (1 << 51) + uint64(v)
	f := math.Float64frombits(b) - (3 << 43)
	return float32(f)
}

func convertKeysym(s C.xkb_keysym_t) (rune, bool) {
	if '0' <= s && s <= '9' || 'A' <= s && s <= 'Z' {
		return rune(s), true
	}
	if 'a' <= s && s <= 'z' {
		return rune(s - 0x20), true
	}
	var n rune
	switch s {
	case C.XKB_KEY_Escape:
		n = key.NameEscape
	case C.XKB_KEY_Left:
		n = key.NameLeftArrow
	case C.XKB_KEY_Right:
		n = key.NameRightArrow
	case C.XKB_KEY_Return:
		n = key.NameReturn
	case C.XKB_KEY_KP_Enter:
		n = key.NameEnter
	case C.XKB_KEY_Up:
		n = key.NameUpArrow
	case C.XKB_KEY_Down:
		n = key.NameDownArrow
	case C.XKB_KEY_Home:
		n = key.NameHome
	case C.XKB_KEY_End:
		n = key.NameEnd
	case C.XKB_KEY_BackSpace:
		n = key.NameDeleteBackward
	case C.XKB_KEY_Delete:
		n = key.NameDeleteForward
	case C.XKB_KEY_Page_Up:
		n = key.NamePageUp
	case C.XKB_KEY_Page_Down:
		n = key.NamePageDown
	default:
		return 0, false
	}
	return n, true
}
