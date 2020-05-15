// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nowayland freebsd

package window

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"math"
	"os/exec"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"gioui.org/app/internal/xkb"
	"gioui.org/f32"
	"gioui.org/internal/fling"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	syscall "golang.org/x/sys/unix"
)

// Use wayland-scanner to generate glue code for the xdg-shell and xdg-decoration extensions.
//go:generate wayland-scanner client-header /usr/share/wayland-protocols/stable/xdg-shell/xdg-shell.xml wayland_xdg_shell.h
//go:generate wayland-scanner private-code /usr/share/wayland-protocols/stable/xdg-shell/xdg-shell.xml wayland_xdg_shell.c

//go:generate wayland-scanner client-header /usr/share/wayland-protocols/unstable/text-input/text-input-unstable-v3.xml wayland_text_input.h
//go:generate wayland-scanner private-code /usr/share/wayland-protocols/unstable/text-input/text-input-unstable-v3.xml wayland_text_input.c

//go:generate wayland-scanner client-header /usr/share/wayland-protocols/unstable/xdg-decoration/xdg-decoration-unstable-v1.xml wayland_xdg_decoration.h
//go:generate wayland-scanner private-code /usr/share/wayland-protocols/unstable/xdg-decoration/xdg-decoration-unstable-v1.xml wayland_xdg_decoration.c

//go:generate sed -i "1s;^;// +build linux,!android,!nowayland freebsd\\n\\n;" wayland_xdg_shell.c
//go:generate sed -i "1s;^;// +build linux,!android,!nowayland freebsd\\n\\n;" wayland_xdg_decoration.c
//go:generate sed -i "1s;^;// +build linux,!android,!nowayland freebsd\\n\\n;" wayland_text_input.c

/*
#cgo linux pkg-config: wayland-client wayland-cursor
#cgo freebsd openbsd LDFLAGS: -lwayland-client -lwayland-cursor
#cgo freebsd CFLAGS: -I/usr/local/include
#cgo freebsd LDFLAGS: -L/usr/local/lib

#include <stdlib.h>
#include <wayland-client.h>
#include <wayland-cursor.h>
#include "wayland_text_input.h"
#include "wayland_xdg_shell.h"
#include "wayland_xdg_decoration.h"
#include "os_wayland.h"
*/
import "C"

type wlDisplay struct {
	disp         *C.struct_wl_display
	reg          *C.struct_wl_registry
	compositor   *C.struct_wl_compositor
	wm           *C.struct_xdg_wm_base
	imm          *C.struct_zwp_text_input_manager_v3
	shm          *C.struct_wl_shm
	decor        *C.struct_zxdg_decoration_manager_v1
	seat         *wlSeat
	xkb          *xkb.Context
	outputMap    map[C.uint32_t]*C.struct_wl_output
	outputConfig map[*C.struct_wl_output]*wlOutput

	// Notification pipe fds.
	notify struct {
		read, write int
	}

	repeat repeatState
}

type wlSeat struct {
	disp     *wlDisplay
	seat     *C.struct_wl_seat
	name     C.uint32_t
	pointer  *C.struct_wl_pointer
	touch    *C.struct_wl_touch
	keyboard *C.struct_wl_keyboard
	im       *C.struct_zwp_text_input_v3

	pointerFocus  *window
	keyboardFocus *window
	touchFoci     map[C.int32_t]*window
}

type repeatState struct {
	rate  int
	delay time.Duration

	key   uint32
	win   Callbacks
	stopC chan struct{}

	start time.Duration
	last  time.Duration
	mu    sync.Mutex
	now   time.Duration
}

type window struct {
	w          Callbacks
	disp       *wlDisplay
	surf       *C.struct_wl_surface
	wmSurf     *C.struct_xdg_surface
	topLvl     *C.struct_xdg_toplevel
	decor      *C.struct_zxdg_toplevel_decoration_v1
	ppdp, ppsp float32
	scroll     struct {
		time  time.Duration
		steps image.Point
		dist  f32.Point
	}
	pointerBtns pointer.Buttons
	lastPos     f32.Point
	lastTouch   f32.Point

	cursor struct {
		theme  *C.struct_wl_cursor_theme
		cursor *C.struct_wl_cursor
		surf   *C.struct_wl_surface
	}

	fling struct {
		yExtrapolation fling.Extrapolation
		xExtrapolation fling.Extrapolation
		anim           fling.Animation
		start          bool
		dir            f32.Point
	}

	stage             system.Stage
	dead              bool
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

type poller struct {
	pollfds [2]syscall.PollFd
	// buf is scratch space for draining the notification pipe.
	buf [100]byte
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

// callbackMap maps Wayland native handles to corresponding Go
// references. It is necessary because the the Wayland client API
// forces the use of callbacks and storing pointers to Go values
// in C is forbidden.
var callbackMap sync.Map

func init() {
	wlDriver = newWLWindow
}

func newWLWindow(window Callbacks, opts *Options) error {
	d, err := newWLDisplay()
	if err != nil {
		return err
	}
	w, err := d.createNativeWindow(opts)
	if err != nil {
		d.destroy()
		return err
	}
	// Increment window counter.
	windowCounter <- +1
	w.w = window
	go func() {
		defer func() {
			// Decrement window counter.
			windowCounter <- -1
		}()
		defer d.destroy()
		defer w.destroy()
		w.w.SetDriver(w)
		if err := w.loop(); err != nil {
			panic(err)
		}
	}()
	return nil
}

func (d *wlDisplay) createNativeWindow(opts *Options) (*window, error) {
	var scale int
	for _, conf := range d.outputConfig {
		if s := conf.scale; s > scale {
			scale = s
		}
	}
	ppdp := detectUIScale()

	w := &window{
		disp:     d,
		scale:    scale,
		newScale: scale != 1,
		ppdp:     ppdp,
		ppsp:     ppdp,
	}
	w.surf = C.wl_compositor_create_surface(d.compositor)
	if w.surf == nil {
		w.destroy()
		return nil, errors.New("wayland: wl_compositor_create_surface failed")
	}
	callbackStore(unsafe.Pointer(w.surf), w)
	w.wmSurf = C.xdg_wm_base_get_xdg_surface(d.wm, w.surf)
	if w.wmSurf == nil {
		w.destroy()
		return nil, errors.New("wayland: xdg_wm_base_get_xdg_surface failed")
	}
	w.topLvl = C.xdg_surface_get_toplevel(w.wmSurf)
	if w.topLvl == nil {
		w.destroy()
		return nil, errors.New("wayland: xdg_surface_get_toplevel failed")
	}
	w.cursor.theme = C.wl_cursor_theme_load(nil, 32, d.shm)
	if w.cursor.theme == nil {
		w.destroy()
		return nil, errors.New("wayland: wl_cursor_theme_load failed")
	}
	cname := C.CString("left_ptr")
	defer C.free(unsafe.Pointer(cname))
	w.cursor.cursor = C.wl_cursor_theme_get_cursor(w.cursor.theme, cname)
	if w.cursor.cursor == nil {
		w.destroy()
		return nil, errors.New("wayland: wl_cursor_theme_get_cursor failed")
	}
	w.cursor.surf = C.wl_compositor_create_surface(d.compositor)
	if w.cursor.surf == nil {
		w.destroy()
		return nil, errors.New("wayland: wl_compositor_create_surface failed")
	}
	C.gio_xdg_wm_base_add_listener(d.wm, unsafe.Pointer(w.surf))
	C.gio_wl_surface_add_listener(w.surf, unsafe.Pointer(w.surf))
	C.gio_xdg_surface_add_listener(w.wmSurf, unsafe.Pointer(w.surf))
	C.gio_xdg_toplevel_add_listener(w.topLvl, unsafe.Pointer(w.surf))
	title := C.CString(opts.Title)
	C.xdg_toplevel_set_title(w.topLvl, title)
	C.free(unsafe.Pointer(title))

	_, _, cfg := w.config()
	w.width = cfg.Px(opts.Width)
	w.height = cfg.Px(opts.Height)
	if d.decor != nil {
		// Request server side decorations.
		w.decor = C.zxdg_decoration_manager_v1_get_toplevel_decoration(d.decor, w.topLvl)
		C.zxdg_toplevel_decoration_v1_set_mode(w.decor, C.ZXDG_TOPLEVEL_DECORATION_V1_MODE_SERVER_SIDE)
	}
	w.updateOpaqueRegion()
	C.wl_surface_commit(w.surf)
	return w, nil
}

func callbackDelete(k unsafe.Pointer) {
	callbackMap.Delete(k)
}

func callbackStore(k unsafe.Pointer, v interface{}) {
	callbackMap.Store(k, v)
}

func callbackLoad(k unsafe.Pointer) interface{} {
	v, exists := callbackMap.Load(k)
	if !exists {
		panic("missing callback entry")
	}
	return v
}

//export gio_onSeatCapabilities
func gio_onSeatCapabilities(data unsafe.Pointer, seat *C.struct_wl_seat, caps C.uint32_t) {
	s := callbackLoad(data).(*wlSeat)
	s.updateCaps(caps)
}

func (s *wlSeat) destroy() {
	if s.im != nil {
		C.zwp_text_input_v3_destroy(s.im)
		s.im = nil
	}
	if s.pointer != nil {
		C.wl_pointer_release(s.pointer)
	}
	if s.touch != nil {
		C.wl_touch_release(s.touch)
	}
	if s.keyboard != nil {
		C.wl_keyboard_release(s.keyboard)
	}
	if s.seat != nil {
		callbackDelete(unsafe.Pointer(s.seat))
		C.wl_seat_release(s.seat)
	}
}

func (s *wlSeat) updateCaps(caps C.uint32_t) {
	if s.im == nil && s.disp.imm != nil {
		s.im = C.zwp_text_input_manager_v3_get_text_input(s.disp.imm, s.seat)
		C.gio_zwp_text_input_v3_add_listener(s.im, unsafe.Pointer(s.seat))
	}
	switch {
	case s.pointer == nil && caps&C.WL_SEAT_CAPABILITY_POINTER != 0:
		s.pointer = C.wl_seat_get_pointer(s.seat)
		C.gio_wl_pointer_add_listener(s.pointer, unsafe.Pointer(s.seat))
	case s.pointer != nil && caps&C.WL_SEAT_CAPABILITY_POINTER == 0:
		C.wl_pointer_release(s.pointer)
		s.pointer = nil
	}
	switch {
	case s.touch == nil && caps&C.WL_SEAT_CAPABILITY_TOUCH != 0:
		s.touch = C.wl_seat_get_touch(s.seat)
		C.gio_wl_touch_add_listener(s.touch, unsafe.Pointer(s.seat))
	case s.touch != nil && caps&C.WL_SEAT_CAPABILITY_TOUCH == 0:
		C.wl_touch_release(s.touch)
		s.touch = nil
	}
	switch {
	case s.keyboard == nil && caps&C.WL_SEAT_CAPABILITY_KEYBOARD != 0:
		s.keyboard = C.wl_seat_get_keyboard(s.seat)
		C.gio_wl_keyboard_add_listener(s.keyboard, unsafe.Pointer(s.seat))
	case s.keyboard != nil && caps&C.WL_SEAT_CAPABILITY_KEYBOARD == 0:
		C.wl_keyboard_release(s.keyboard)
		s.keyboard = nil
	}
}

//export gio_onSeatName
func gio_onSeatName(data unsafe.Pointer, seat *C.struct_wl_seat, name *C.char) {
}

//export gio_onXdgSurfaceConfigure
func gio_onXdgSurfaceConfigure(data unsafe.Pointer, wmSurf *C.struct_xdg_surface, serial C.uint32_t) {
	w := callbackLoad(data).(*window)
	w.mu.Lock()
	w.serial = serial
	w.needAck = true
	w.mu.Unlock()
	w.setStage(system.StageRunning)
	w.draw(true)
}

//export gio_onToplevelClose
func gio_onToplevelClose(data unsafe.Pointer, topLvl *C.struct_xdg_toplevel) {
	w := callbackLoad(data).(*window)
	w.dead = true
}

//export gio_onToplevelConfigure
func gio_onToplevelConfigure(data unsafe.Pointer, topLvl *C.struct_xdg_toplevel, width, height C.int32_t, states *C.struct_wl_array) {
	w := callbackLoad(data).(*window)
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
	d := callbackLoad(data).(*wlDisplay)
	c := d.outputConfig[output]
	c.width = int(width)
	c.height = int(height)
}

//export gio_onOutputGeometry
func gio_onOutputGeometry(data unsafe.Pointer, output *C.struct_wl_output, x, y, physWidth, physHeight, subpixel C.int32_t, make, model *C.char, transform C.int32_t) {
	d := callbackLoad(data).(*wlDisplay)
	c := d.outputConfig[output]
	c.transform = transform
	c.physWidth = int(physWidth)
	c.physHeight = int(physHeight)
}

//export gio_onOutputScale
func gio_onOutputScale(data unsafe.Pointer, output *C.struct_wl_output, scale C.int32_t) {
	d := callbackLoad(data).(*wlDisplay)
	c := d.outputConfig[output]
	c.scale = int(scale)
}

//export gio_onOutputDone
func gio_onOutputDone(data unsafe.Pointer, output *C.struct_wl_output) {
	d := callbackLoad(data).(*wlDisplay)
	conf := d.outputConfig[output]
	for _, w := range conf.windows {
		w.draw(true)
	}
}

//export gio_onSurfaceEnter
func gio_onSurfaceEnter(data unsafe.Pointer, surf *C.struct_wl_surface, output *C.struct_wl_output) {
	w := callbackLoad(data).(*window)
	conf := w.disp.outputConfig[output]
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
	w := callbackLoad(data).(*window)
	conf := w.disp.outputConfig[output]
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
	d := callbackLoad(data).(*wlDisplay)
	switch C.GoString(cintf) {
	case "wl_compositor":
		d.compositor = (*C.struct_wl_compositor)(C.wl_registry_bind(reg, name, &C.wl_compositor_interface, 3))
	case "wl_output":
		output := (*C.struct_wl_output)(C.wl_registry_bind(reg, name, &C.wl_output_interface, 2))
		C.gio_wl_output_add_listener(output, unsafe.Pointer(d.disp))
		d.outputMap[name] = output
		d.outputConfig[output] = new(wlOutput)
	case "wl_seat":
		if d.seat == nil {
			s := (*C.struct_wl_seat)(C.wl_registry_bind(reg, name, &C.wl_seat_interface, 5))
			d.seat = &wlSeat{
				disp: d,
				name: name,
				seat: s,
			}
			callbackStore(unsafe.Pointer(s), d.seat)
			C.gio_wl_seat_add_listener(d.seat.seat, unsafe.Pointer(d.seat.seat))
		}
	case "wl_shm":
		d.shm = (*C.struct_wl_shm)(C.wl_registry_bind(reg, name, &C.wl_shm_interface, 1))
	case "xdg_wm_base":
		d.wm = (*C.struct_xdg_wm_base)(C.wl_registry_bind(reg, name, &C.xdg_wm_base_interface, 1))
	case "zxdg_decoration_manager_v1":
		d.decor = (*C.struct_zxdg_decoration_manager_v1)(C.wl_registry_bind(reg, name, &C.zxdg_decoration_manager_v1_interface, 1))
		// TODO: Implement and test text-input support.
		/*case "zwp_text_input_manager_v3":
		d.imm = (*C.struct_zwp_text_input_manager_v3)(C.wl_registry_bind(reg, name, &C.zwp_text_input_manager_v3_interface, 1))*/
	}
}

//export gio_onRegistryGlobalRemove
func gio_onRegistryGlobalRemove(data unsafe.Pointer, reg *C.struct_wl_registry, name C.uint32_t) {
	d := callbackLoad(data).(*wlDisplay)
	if s := d.seat; s != nil && name == s.name {
		s.destroy()
		d.seat = nil
	}
	if output, exists := d.outputMap[name]; exists {
		C.wl_output_destroy(output)
		delete(d.outputMap, name)
		delete(d.outputConfig, output)
	}
}

//export gio_onTouchDown
func gio_onTouchDown(data unsafe.Pointer, touch *C.struct_wl_touch, serial, t C.uint32_t, surf *C.struct_wl_surface, id C.int32_t, x, y C.wl_fixed_t) {
	s := callbackLoad(data).(*wlSeat)
	w := callbackLoad(unsafe.Pointer(surf)).(*window)
	s.touchFoci[id] = w
	w.lastTouch = f32.Point{
		X: fromFixed(x) * float32(w.scale),
		Y: fromFixed(y) * float32(w.scale),
	}
	w.w.Event(pointer.Event{
		Type:      pointer.Press,
		Source:    pointer.Touch,
		Position:  w.lastTouch,
		PointerID: pointer.ID(id),
		Time:      time.Duration(t) * time.Millisecond,
	})
}

//export gio_onTouchUp
func gio_onTouchUp(data unsafe.Pointer, touch *C.struct_wl_touch, serial, t C.uint32_t, id C.int32_t) {
	s := callbackLoad(data).(*wlSeat)
	w := s.touchFoci[id]
	delete(s.touchFoci, id)
	w.w.Event(pointer.Event{
		Type:      pointer.Release,
		Source:    pointer.Touch,
		Position:  w.lastTouch,
		PointerID: pointer.ID(id),
		Time:      time.Duration(t) * time.Millisecond,
	})
}

//export gio_onTouchMotion
func gio_onTouchMotion(data unsafe.Pointer, touch *C.struct_wl_touch, t C.uint32_t, id C.int32_t, x, y C.wl_fixed_t) {
	s := callbackLoad(data).(*wlSeat)
	w := s.touchFoci[id]
	w.lastTouch = f32.Point{
		X: fromFixed(x) * float32(w.scale),
		Y: fromFixed(y) * float32(w.scale),
	}
	w.w.Event(pointer.Event{
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
	s := callbackLoad(data).(*wlSeat)
	for id, w := range s.touchFoci {
		delete(s.touchFoci, id)
		w.w.Event(pointer.Event{
			Type:   pointer.Cancel,
			Source: pointer.Touch,
		})
	}
}

//export gio_onPointerEnter
func gio_onPointerEnter(data unsafe.Pointer, pointer *C.struct_wl_pointer, serial C.uint32_t, surf *C.struct_wl_surface, x, y C.wl_fixed_t) {
	s := callbackLoad(data).(*wlSeat)
	w := callbackLoad(unsafe.Pointer(surf)).(*window)
	s.pointerFocus = w
	// Get images[0].
	img := *w.cursor.cursor.images
	buf := C.wl_cursor_image_get_buffer(img)
	if buf == nil {
		return
	}
	C.wl_pointer_set_cursor(pointer, serial, w.cursor.surf, C.int32_t(img.hotspot_x), C.int32_t(img.hotspot_y))
	C.wl_surface_attach(w.cursor.surf, buf, 0, 0)
	C.wl_surface_damage(w.cursor.surf, 0, 0, C.int32_t(img.width), C.int32_t(img.height))
	C.wl_surface_commit(w.cursor.surf)
	w.lastPos = f32.Point{X: fromFixed(x), Y: fromFixed(y)}
}

//export gio_onPointerLeave
func gio_onPointerLeave(data unsafe.Pointer, p *C.struct_wl_pointer, serial C.uint32_t, surface *C.struct_wl_surface) {
}

//export gio_onPointerMotion
func gio_onPointerMotion(data unsafe.Pointer, p *C.struct_wl_pointer, t C.uint32_t, x, y C.wl_fixed_t) {
	s := callbackLoad(data).(*wlSeat)
	w := s.pointerFocus
	w.resetFling()
	w.onPointerMotion(x, y, t)
}

//export gio_onPointerButton
func gio_onPointerButton(data unsafe.Pointer, p *C.struct_wl_pointer, serial, t, wbtn, state C.uint32_t) {
	s := callbackLoad(data).(*wlSeat)
	w := s.pointerFocus
	// From linux-event-codes.h.
	const (
		BTN_LEFT   = 0x110
		BTN_RIGHT  = 0x111
		BTN_MIDDLE = 0x112
	)
	var btn pointer.Buttons
	switch wbtn {
	case BTN_LEFT:
		btn = pointer.ButtonLeft
	case BTN_RIGHT:
		btn = pointer.ButtonRight
	case BTN_MIDDLE:
		btn = pointer.ButtonMiddle
	default:
		return
	}
	var typ pointer.Type
	switch state {
	case 0:
		w.pointerBtns &^= btn
		typ = pointer.Release
	case 1:
		w.pointerBtns |= btn
		typ = pointer.Press
	}
	w.flushScroll()
	w.resetFling()
	w.w.Event(pointer.Event{
		Type:     typ,
		Source:   pointer.Mouse,
		Buttons:  w.pointerBtns,
		Position: w.lastPos,
		Time:     time.Duration(t) * time.Millisecond,
	})
}

//export gio_onPointerAxis
func gio_onPointerAxis(data unsafe.Pointer, p *C.struct_wl_pointer, t, axis C.uint32_t, value C.wl_fixed_t) {
	s := callbackLoad(data).(*wlSeat)
	w := s.pointerFocus
	v := fromFixed(value)
	w.resetFling()
	if w.scroll.dist == (f32.Point{}) {
		w.scroll.time = time.Duration(t) * time.Millisecond
	}
	switch axis {
	case C.WL_POINTER_AXIS_HORIZONTAL_SCROLL:
		w.scroll.dist.X += v
	case C.WL_POINTER_AXIS_VERTICAL_SCROLL:
		w.scroll.dist.Y += v
	}
}

//export gio_onPointerFrame
func gio_onPointerFrame(data unsafe.Pointer, p *C.struct_wl_pointer) {
	s := callbackLoad(data).(*wlSeat)
	w := s.pointerFocus
	w.flushScroll()
	w.flushFling()
}

func (w *window) flushFling() {
	if !w.fling.start {
		return
	}
	w.fling.start = false
	estx, esty := w.fling.xExtrapolation.Estimate(), w.fling.yExtrapolation.Estimate()
	w.fling.xExtrapolation = fling.Extrapolation{}
	w.fling.yExtrapolation = fling.Extrapolation{}
	vel := float32(math.Sqrt(float64(estx.Velocity*estx.Velocity + esty.Velocity*esty.Velocity)))
	_, _, c := w.config()
	if !w.fling.anim.Start(&c, time.Now(), vel) {
		return
	}
	invDist := 1 / vel
	w.fling.dir.X = estx.Velocity * invDist
	w.fling.dir.Y = esty.Velocity * invDist
	// Wake up the window loop.
	w.disp.wakeup()
}

//export gio_onPointerAxisSource
func gio_onPointerAxisSource(data unsafe.Pointer, pointer *C.struct_wl_pointer, source C.uint32_t) {
}

//export gio_onPointerAxisStop
func gio_onPointerAxisStop(data unsafe.Pointer, p *C.struct_wl_pointer, t, axis C.uint32_t) {
	s := callbackLoad(data).(*wlSeat)
	w := s.pointerFocus
	w.fling.start = true
}

//export gio_onPointerAxisDiscrete
func gio_onPointerAxisDiscrete(data unsafe.Pointer, p *C.struct_wl_pointer, axis C.uint32_t, discrete C.int32_t) {
	s := callbackLoad(data).(*wlSeat)
	w := s.pointerFocus
	w.resetFling()
	switch axis {
	case C.WL_POINTER_AXIS_HORIZONTAL_SCROLL:
		w.scroll.steps.X += int(discrete)
	case C.WL_POINTER_AXIS_VERTICAL_SCROLL:
		w.scroll.steps.Y += int(discrete)
	}
}

func (w *window) resetFling() {
	w.fling.start = false
	w.fling.anim = fling.Animation{}
}

//export gio_onKeyboardKeymap
func gio_onKeyboardKeymap(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, format C.uint32_t, fd C.int32_t, size C.uint32_t) {
	defer syscall.Close(int(fd))
	s := callbackLoad(data).(*wlSeat)
	s.disp.repeat.Stop(0)
	s.disp.xkb.DestroyKeymapState()
	if format != C.WL_KEYBOARD_KEYMAP_FORMAT_XKB_V1 {
		return
	}
	if err := s.disp.xkb.LoadKeymap(int(format), int(fd), int(size)); err != nil {
		// TODO: Do better.
		panic(err)
	}
}

//export gio_onKeyboardEnter
func gio_onKeyboardEnter(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, serial C.uint32_t, surf *C.struct_wl_surface, keys *C.struct_wl_array) {
	s := callbackLoad(data).(*wlSeat)
	w := callbackLoad(unsafe.Pointer(surf)).(*window)
	s.keyboardFocus = w
	s.disp.repeat.Stop(0)
	w.w.Event(key.FocusEvent{Focus: true})
}

//export gio_onKeyboardLeave
func gio_onKeyboardLeave(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, serial C.uint32_t, surf *C.struct_wl_surface) {
	s := callbackLoad(data).(*wlSeat)
	s.disp.repeat.Stop(0)
	w := s.keyboardFocus
	w.w.Event(key.FocusEvent{Focus: false})
}

//export gio_onKeyboardKey
func gio_onKeyboardKey(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, serial, timestamp, keyCode, state C.uint32_t) {
	s := callbackLoad(data).(*wlSeat)
	w := s.keyboardFocus
	t := time.Duration(timestamp) * time.Millisecond
	s.disp.repeat.Stop(t)
	w.resetFling()
	if state != C.WL_KEYBOARD_KEY_STATE_PRESSED {
		return
	}
	kc := mapXKBKeycode(uint32(keyCode))
	for _, e := range w.disp.xkb.DispatchKey(kc) {
		w.w.Event(e)
	}
	if w.disp.xkb.IsRepeatKey(kc) {
		w.disp.repeat.Start(w, kc, t)
	}
}

func mapXKBKeycode(keyCode uint32) uint32 {
	// According to the xkb_v1 spec: "to determine the xkb keycode, clients must add 8 to the key event keycode."
	return keyCode + 8
}

func (r *repeatState) Start(w *window, keyCode uint32, t time.Duration) {
	if r.rate <= 0 {
		return
	}
	stopC := make(chan struct{})
	r.start = t
	r.last = 0
	r.now = 0
	r.stopC = stopC
	r.key = keyCode
	r.win = w.w
	rate, delay := r.rate, r.delay
	go func() {
		timer := time.NewTimer(delay)
		for {
			select {
			case <-timer.C:
			case <-stopC:
				close(stopC)
				return
			}
			r.Advance(delay)
			w.disp.wakeup()
			delay = time.Second / time.Duration(rate)
			timer.Reset(delay)
		}
	}()
}

func (r *repeatState) Stop(t time.Duration) {
	if r.stopC == nil {
		return
	}
	r.stopC <- struct{}{}
	<-r.stopC
	r.stopC = nil
	t -= r.start
	if r.now > t {
		r.now = t
	}
}

func (r *repeatState) Advance(dt time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.now += dt
}

func (r *repeatState) Repeat(d *wlDisplay) {
	if r.rate <= 0 {
		return
	}
	r.mu.Lock()
	now := r.now
	r.mu.Unlock()
	for {
		var delay time.Duration
		if r.last < r.delay {
			delay = r.delay
		} else {
			delay = time.Second / time.Duration(r.rate)
		}
		if r.last+delay > now {
			break
		}
		for _, e := range d.xkb.DispatchKey(r.key) {
			r.win.Event(e)
		}
		r.last += delay
	}
}

//export gio_onFrameDone
func gio_onFrameDone(data unsafe.Pointer, callback *C.struct_wl_callback, t C.uint32_t) {
	C.wl_callback_destroy(callback)
	w := callbackLoad(data).(*window)
	if w.lastFrameCallback == callback {
		w.lastFrameCallback = nil
		w.draw(false)
	}
}

func (w *window) loop() error {
	var p poller
	for {
		notified, err := w.disp.dispatch(&p)
		if err != nil {
			return err
		}
		if w.dead {
			w.w.Event(system.DestroyEvent{})
			break
		}
		if notified {
			w.draw(false)
		}
	}
	return nil
}

func (d *wlDisplay) dispatch(p *poller) (bool, error) {
	dispfd := C.wl_display_get_fd(d.disp)
	// Poll for events and notifications.
	pollfds := append(p.pollfds[:0],
		syscall.PollFd{Fd: int32(dispfd), Events: syscall.POLLIN | syscall.POLLERR},
		syscall.PollFd{Fd: int32(d.notify.read), Events: syscall.POLLIN | syscall.POLLERR},
	)
	dispFd := &pollfds[0]
	if ret, err := C.wl_display_flush(d.disp); ret < 0 {
		if err != syscall.EAGAIN {
			return false, fmt.Errorf("wayland: wl_display_flush failed: %v", err)
		}
		// EAGAIN means the output buffer was full. Poll for
		// POLLOUT to know when we can write again.
		dispFd.Events |= syscall.POLLOUT
	}
	if _, err := syscall.Poll(pollfds, -1); err != nil && err != syscall.EINTR {
		return false, fmt.Errorf("wayland: poll failed: %v", err)
	}
	notified := false
	// Clear notifications.
	for {
		_, err := syscall.Read(d.notify.read, p.buf[:])
		if err == syscall.EAGAIN {
			break
		}
		if err != nil {
			return false, fmt.Errorf("wayland: read from notify pipe failed: %v", err)
		}
		notified = true
	}
	// Handle events
	switch {
	case dispFd.Revents&syscall.POLLIN != 0:
		if ret, err := C.wl_display_dispatch(d.disp); ret < 0 {
			return false, fmt.Errorf("wayland: wl_display_dispatch failed: %v", err)
		}
	case dispFd.Revents&(syscall.POLLERR|syscall.POLLHUP) != 0:
		return false, errors.New("wayland: display file descriptor gone")
	}
	d.repeat.Repeat(d)
	return notified, nil
}

func (w *window) SetAnimating(anim bool) {
	w.mu.Lock()
	w.animating = anim
	animating := w.isAnimating()
	w.mu.Unlock()
	if animating {
		w.disp.wakeup()
	}
}

// Wakeup wakes up the event loop through the notification pipe.
func (d *wlDisplay) wakeup() {
	oneByte := make([]byte, 1)
	if _, err := syscall.Write(d.notify.write, oneByte); err != nil && err != syscall.EAGAIN {
		panic(fmt.Errorf("failed to write to pipe: %v", err))
	}
}

func (w *window) destroy() {
	if w.cursor.surf != nil {
		C.wl_surface_destroy(w.cursor.surf)
	}
	if w.cursor.theme != nil {
		C.wl_cursor_theme_destroy(w.cursor.theme)
	}
	if w.topLvl != nil {
		C.xdg_toplevel_destroy(w.topLvl)
	}
	if w.surf != nil {
		C.wl_surface_destroy(w.surf)
	}
	if w.wmSurf != nil {
		C.xdg_surface_destroy(w.wmSurf)
	}
	if w.decor != nil {
		C.zxdg_toplevel_decoration_v1_destroy(w.decor)
	}
	callbackDelete(unsafe.Pointer(w.surf))
}

//export gio_onKeyboardModifiers
func gio_onKeyboardModifiers(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, serial, depressed, latched, locked, group C.uint32_t) {
	s := callbackLoad(data).(*wlSeat)
	d := s.disp
	d.repeat.Stop(0)
	if d.xkb == nil {
		return
	}
	d.xkb.UpdateMask(uint32(depressed), uint32(latched), uint32(locked), uint32(group), uint32(group), uint32(group))
}

//export gio_onKeyboardRepeatInfo
func gio_onKeyboardRepeatInfo(data unsafe.Pointer, keyboard *C.struct_wl_keyboard, rate, delay C.int32_t) {
	s := callbackLoad(data).(*wlSeat)
	d := s.disp
	d.repeat.Stop(0)
	d.repeat.rate = int(rate)
	d.repeat.delay = time.Duration(delay) * time.Millisecond
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

func (w *window) flushScroll() {
	var fling f32.Point
	if w.fling.anim.Active() {
		dist := float32(w.fling.anim.Tick(time.Now()))
		fling = w.fling.dir.Mul(dist)
	}
	// The Wayland reported scroll distance for
	// discrete scroll axes is only 10 pixels, where
	// 100 seems more appropriate.
	const discreteScale = 10
	if w.scroll.steps.X != 0 {
		w.scroll.dist.X *= discreteScale
	}
	if w.scroll.steps.Y != 0 {
		w.scroll.dist.Y *= discreteScale
	}
	total := w.scroll.dist.Add(fling)
	if total == (f32.Point{}) {
		return
	}
	w.w.Event(pointer.Event{
		Type:     pointer.Move,
		Source:   pointer.Mouse,
		Buttons:  w.pointerBtns,
		Position: w.lastPos,
		Scroll:   total,
		Time:     w.scroll.time,
	})
	if w.scroll.steps == (image.Point{}) {
		w.fling.xExtrapolation.SampleDelta(w.scroll.time, -w.scroll.dist.X)
		w.fling.yExtrapolation.SampleDelta(w.scroll.time, -w.scroll.dist.Y)
	}
	w.scroll.dist = f32.Point{}
	w.scroll.steps = image.Point{}
}

func (w *window) onPointerMotion(x, y C.wl_fixed_t, t C.uint32_t) {
	w.flushScroll()
	w.lastPos = f32.Point{
		X: fromFixed(x) * float32(w.scale),
		Y: fromFixed(y) * float32(w.scale),
	}
	w.w.Event(pointer.Event{
		Type:     pointer.Move,
		Position: w.lastPos,
		Buttons:  w.pointerBtns,
		Source:   pointer.Mouse,
		Time:     time.Duration(t) * time.Millisecond,
	})
}

func (w *window) updateOpaqueRegion() {
	reg := C.wl_compositor_create_region(w.disp.compositor)
	C.wl_region_add(reg, 0, 0, C.int32_t(w.width), C.int32_t(w.height))
	C.wl_surface_set_opaque_region(w.surf, reg)
	C.wl_region_destroy(reg)
}

func (w *window) updateOutputs() {
	scale := 1
	var found bool
	for _, conf := range w.disp.outputConfig {
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
		w.setStage(system.StagePaused)
	} else {
		w.setStage(system.StageRunning)
		w.draw(true)
	}
}

func (w *window) config() (int, int, config) {
	width, height := w.width*w.scale, w.height*w.scale
	return width, height, config{
		pxPerDp: w.ppdp * float32(w.scale),
		pxPerSp: w.ppsp * float32(w.scale),
	}
}

func (w *window) isAnimating() bool {
	return w.animating || w.fling.anim.Active()
}

func (w *window) draw(sync bool) {
	w.flushScroll()
	w.mu.Lock()
	animating := w.isAnimating()
	dead := w.dead
	w.mu.Unlock()
	if dead || (!animating && !sync) {
		return
	}
	width, height, cfg := w.config()
	if cfg == (config{}) {
		return
	}
	if animating && w.lastFrameCallback == nil {
		w.lastFrameCallback = C.wl_surface_frame(w.surf)
		// Use the surface as listener data for gio_onFrameDone.
		C.gio_wl_callback_add_listener(w.lastFrameCallback, unsafe.Pointer(w.surf))
	}
	cfg.now = time.Now()
	w.w.Event(FrameEvent{
		FrameEvent: system.FrameEvent{
			Size: image.Point{
				X: width,
				Y: height,
			},
			Config: &cfg,
		},
		Sync: sync,
	})
}

func (w *window) setStage(s system.Stage) {
	if s == w.stage {
		return
	}
	w.stage = s
	w.w.Event(system.StageEvent{s})
}

func (w *window) display() *C.struct_wl_display {
	return w.disp.disp
}

func (w *window) surface() (*C.struct_wl_surface, int, int) {
	if w.needAck {
		C.xdg_surface_ack_configure(w.wmSurf, w.serial)
		w.needAck = false
	}
	width, height, scale := w.width, w.height, w.scale
	if w.newScale {
		C.wl_surface_set_buffer_scale(w.surf, C.int32_t(scale))
		w.newScale = false
	}
	return w.surf, width * scale, height * scale
}

func (w *window) ShowTextInput(show bool) {}

// detectUIScale reports the system UI scale, or 1.0 if it fails.
func detectUIScale() float32 {
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

func newWLDisplay() (*wlDisplay, error) {
	d := &wlDisplay{
		outputMap:    make(map[C.uint32_t]*C.struct_wl_output),
		outputConfig: make(map[*C.struct_wl_output]*wlOutput),
	}
	pipe := make([]int, 2)
	if err := syscall.Pipe2(pipe, syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		return nil, fmt.Errorf("wayland: failed to create pipe: %v", err)
	}
	d.notify.read = pipe[0]
	d.notify.write = pipe[1]
	xkb, err := xkb.New()
	if err != nil {
		d.destroy()
		return nil, fmt.Errorf("wayland: %v", err)
	}
	d.xkb = xkb
	d.disp, err = C.wl_display_connect(nil)
	if d.disp == nil {
		d.destroy()
		return nil, fmt.Errorf("wayland: wl_display_connect failed: %v", err)
	}
	callbackMap.Store(unsafe.Pointer(d.disp), d)
	d.reg = C.wl_display_get_registry(d.disp)
	if d.reg == nil {
		d.destroy()
		return nil, errors.New("wayland: wl_display_get_registry failed")
	}
	C.gio_wl_registry_add_listener(d.reg, unsafe.Pointer(d.disp))
	// Wait for the server to register all its globals to the
	// registry listener (gio_onRegistryGlobal).
	C.wl_display_roundtrip(d.disp)
	// Configuration listeners are added to outputs by gio_onRegistryGlobal.
	// We need another roundtrip to get the initial output configurations
	// through the gio_onOutput* callbacks.
	C.wl_display_roundtrip(d.disp)
	if d.compositor == nil {
		d.destroy()
		return nil, errors.New("wayland: no compositor available")
	}
	if d.wm == nil {
		d.destroy()
		return nil, errors.New("wayland: no xdg_wm_base available")
	}
	if d.shm == nil {
		d.destroy()
		return nil, errors.New("wayland: no wl_shm available")
	}
	if len(d.outputMap) == 0 {
		d.destroy()
		return nil, errors.New("wayland: no outputs available")
	}
	return d, nil
}

func (d *wlDisplay) destroy() {
	if d.notify.write != 0 {
		syscall.Close(d.notify.write)
		d.notify.write = 0
	}
	if d.notify.read != 0 {
		syscall.Close(d.notify.read)
		d.notify.read = 0
	}
	d.repeat.Stop(0)
	if d.xkb != nil {
		d.xkb.Destroy()
		d.xkb = nil
	}
	if d.seat != nil {
		d.seat.destroy()
		d.seat = nil
	}
	if d.imm != nil {
		C.zwp_text_input_manager_v3_destroy(d.imm)
	}
	if d.decor != nil {
		C.zxdg_decoration_manager_v1_destroy(d.decor)
	}
	if d.shm != nil {
		C.wl_shm_destroy(d.shm)
	}
	if d.compositor != nil {
		C.wl_compositor_destroy(d.compositor)
	}
	if d.wm != nil {
		C.xdg_wm_base_destroy(d.wm)
	}
	for _, output := range d.outputMap {
		C.wl_output_destroy(output)
	}
	if d.reg != nil {
		C.wl_registry_destroy(d.reg)
	}
	if d.disp != nil {
		C.wl_display_disconnect(d.disp)
		callbackDelete(unsafe.Pointer(d.disp))
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
