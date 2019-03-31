// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

package app

/*
#cgo CFLAGS: -DGL_SILENCE_DEPRECATION -Werror -fmodules -fobjc-arc -x objective-c

#include <AppKit/AppKit.h>
#include "os_macos.h"
*/
import "C"
import (
	"errors"
	"image"
	"runtime"
	"time"
	"unsafe"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/key"
	"gioui.org/ui/pointer"
)

func init() {
	// Darwin requires that UI operations happen on the main thread only.
	runtime.LockOSThread()
}

type window struct {
	view  C.CFTypeRef
	w     *Window
	stage Stage
}

type windowError struct {
	window *Window
	err    error
}

var windowOpts = make(chan *WindowOptions)

var windows = make(chan windowError)

var viewFactory func() uintptr

var views = make(map[C.CFTypeRef]*window)

func (w *window) contextView() C.CFTypeRef {
	return w.view
}

func (w *window) setTextInput(s key.TextInputState) {}

func (w *window) setAnimating(anim bool) {
	var animb C.BOOL
	if anim {
		animb = 1
	}
	C.gio_setAnimating(w.view, animb)
}

func (w *window) setStage(stage Stage) {
	if stage == w.stage {
		return
	}
	w.stage = stage
	w.w.event(ChangeStage{stage})
}

//export gio_onFrameCallback
func gio_onFrameCallback(view C.CFTypeRef) {
	w := views[view]
	w.draw(false)
}

//export gio_onKeys
func gio_onKeys(view C.CFTypeRef, cstr *C.char, ti C.double, mods C.NSUInteger) {
	str := C.GoString(cstr)
	var kmods key.Modifiers
	if mods&C.NSEventModifierFlagCommand != 0 {
		kmods |= key.ModCommand
	}
	w := views[view]
	for _, k := range str {
		if n, ok := convertKey(k); ok {
			w.w.event(key.Chord{Name: n, Modifiers: kmods})
		}
	}
}

//export gio_onText
func gio_onText(view C.CFTypeRef, cstr *C.char) {
	str := C.GoString(cstr)
	w := views[view]
	w.w.event(key.Edit{Text: str})
}

//export gio_onMouse
func gio_onMouse(view C.CFTypeRef, cdir C.int, x, y, dx, dy C.CGFloat, ti C.double) {
	var typ pointer.Type
	switch cdir {
	case C.GIO_MOUSE_MOVE:
		typ = pointer.Move
	case C.GIO_MOUSE_UP:
		typ = pointer.Release
	case C.GIO_MOUSE_DOWN:
		typ = pointer.Press
	default:
		panic("invalid direction")
	}
	t := time.Duration(float64(ti)*float64(time.Second) + .5)
	w := views[view]
	w.w.event(pointer.Event{
		Type:     typ,
		Source:   pointer.Mouse,
		Time:     t,
		Position: f32.Point{X: float32(x), Y: float32(y)},
		Scroll:   f32.Point{X: float32(dx), Y: float32(dy)},
	})
}

//export gio_onDraw
func gio_onDraw(view C.CFTypeRef) {
	w := views[view]
	w.draw(true)
}

func (w *window) draw(sync bool) {
	width, height := int(C.gio_viewWidth(w.view)+.5), int(C.gio_viewHeight(w.view)+.5)
	if width == 0 || height == 0 {
		return
	}
	cfg := getConfig()
	cfg.Now = time.Now()
	w.setStage(StageVisible)
	w.w.event(Draw{
		Size: image.Point{
			X: width,
			Y: height,
		},
		Config: &cfg,
		sync:   sync,
	})
}

func getConfig() ui.Config {
	ppdp := float32(C.gio_getPixelsPerDP())
	ppdp *= monitorScale
	if ppdp < minDensity {
		ppdp = minDensity
	}
	return ui.Config{
		PxPerDp: ppdp,
		PxPerSp: ppdp,
	}
}

//export gio_onTerminate
func gio_onTerminate(view C.CFTypeRef) {
	w := views[view]
	delete(views, view)
	w.setStage(StageDead)
}

//export gio_onHide
func gio_onHide(view C.CFTypeRef) {
	w := views[view]
	w.setStage(StageInvisible)
}

//export gio_onShow
func gio_onShow(view C.CFTypeRef) {
	w := views[view]
	w.setStage(StageVisible)
}

//export gio_onCreate
func gio_onCreate(view C.CFTypeRef) {
	w := &window{
		view: view,
	}
	ow := newWindow(w)
	w.w = ow
	views[view] = w
	windows <- windowError{window: ow}
}

func createWindow(opts *WindowOptions) (*Window, error) {
	windowOpts <- opts
	werr := <-windows
	return werr.window, werr.err
}

func Main() {
	view := C.CFTypeRef(viewFactory())
	if view == 0 {
		windows <- windowError{err: errors.New("CreateWindow: failed to create view")}
		return
	}
	cfg := getConfig()
	opts := <-windowOpts
	w := cfg.Pixels(opts.Width)
	h := cfg.Pixels(opts.Height)
	title := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(title))
	C.gio_main(view, title, C.CGFloat(w), C.CGFloat(h))
}

func convertKey(k rune) (rune, bool) {
	if '0' <= k && k <= '9' || 'A' <= k && k <= 'Z' {
		return k, true
	}
	if 'a' <= k && k <= 'z' {
		return k - 0x20, true
	}
	var n rune
	switch k {
	case 0x1b:
		n = key.NameEscape
	case C.NSLeftArrowFunctionKey:
		n = key.NameLeftArrow
	case C.NSRightArrowFunctionKey:
		n = key.NameRightArrow
	case C.NSUpArrowFunctionKey:
		n = key.NameUpArrow
	case C.NSDownArrowFunctionKey:
		n = key.NameDownArrow
	case 0xd:
		n = key.NameReturn
	case C.NSHomeFunctionKey:
		n = key.NameHome
	case C.NSEndFunctionKey:
		n = key.NameEnd
	case 0x7f:
		n = key.NameDeleteBackward
	case C.NSDeleteFunctionKey:
		n = key.NameDeleteForward
	case C.NSPageUpFunctionKey:
		n = key.NamePageUp
	case C.NSPageDownFunctionKey:
		n = key.NamePageDown
	default:
		return 0, false
	}
	return n, true
}
