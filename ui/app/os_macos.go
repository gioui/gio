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
	"sync"
	"time"
	"unsafe"

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

type viewCmd struct {
	view C.CFTypeRef
	f    viewFunc
}

type viewFunc func(views viewMap, view C.CFTypeRef)

type viewMap map[C.CFTypeRef]*window

var (
	viewOnce sync.Once
	viewCmds = make(chan viewCmd)
	viewAcks = make(chan struct{})
)

var mainWindow = newWindowRendezvous()

var viewFactory func() C.CFTypeRef

func viewDo(view C.CFTypeRef, f viewFunc) {
	viewOnce.Do(func() {
		go runViewCmdLoop()
	})
	viewCmds <- viewCmd{view, f}
	<-viewAcks
}

func runViewCmdLoop() {
	views := make(viewMap)
	for {
		select {
		case cmd := <-viewCmds:
			cmd.f(views, cmd.view)
			viewAcks <- struct{}{}
		}
	}
}

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
	w.w.event(StageEvent{stage})
}

// Use a top level func for onFrameCallback to avoid
// garbage from viewDo.
func onFrameCmd(views viewMap, view C.CFTypeRef) {
	w := views[view]
	w.draw(false)
}

//export gio_onFrameCallback
func gio_onFrameCallback(view C.CFTypeRef) {
	viewDo(view, onFrameCmd)
}

//export gio_onKeys
func gio_onKeys(view C.CFTypeRef, cstr *C.char, ti C.double, mods C.NSUInteger) {
	str := C.GoString(cstr)
	var kmods key.Modifiers
	if mods&C.NSEventModifierFlagCommand != 0 {
		kmods |= key.ModCommand
	}
	if mods&C.NSEventModifierFlagShift != 0 {
		kmods |= key.ModShift
	}
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		for _, k := range str {
			if n, ok := convertKey(k); ok {
				w.w.event(key.ChordEvent{Name: n, Modifiers: kmods})
			}
		}
	})
}

//export gio_onText
func gio_onText(view C.CFTypeRef, cstr *C.char) {
	str := C.GoString(cstr)
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.w.event(key.EditEvent{Text: str})
	})
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
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.w.event(pointer.Event{
			Type:     typ,
			Source:   pointer.Mouse,
			Time:     t,
			Position: f32.Point{X: float32(x), Y: float32(y)},
			Scroll:   f32.Point{X: float32(dx), Y: float32(dy)},
		})
	})
}

//export gio_onDraw
func gio_onDraw(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.draw(true)
	})
}

//export gio_onFocus
func gio_onFocus(view C.CFTypeRef, focus C.BOOL) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.w.event(key.FocusEvent{Focus: focus == C.YES})
	})
}

func (w *window) draw(sync bool) {
	width, height := int(C.gio_viewWidth(w.view)+.5), int(C.gio_viewHeight(w.view)+.5)
	if width == 0 || height == 0 {
		return
	}
	cfg := getConfig()
	cfg.now = time.Now()
	w.setStage(StageRunning)
	w.w.event(DrawEvent{
		Size: image.Point{
			X: width,
			Y: height,
		},
		Config: cfg,
		sync:   sync,
	})
}

func getConfig() Config {
	ppdp := float32(C.gio_getPixelsPerDP())
	ppdp *= monitorScale
	if ppdp < minDensity {
		ppdp = minDensity
	}
	return Config{
		pxPerDp: ppdp,
		pxPerSp: ppdp,
	}
}

//export gio_onTerminate
func gio_onTerminate(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		delete(views, view)
		w.w.event(DestroyEvent{})
	})
}

//export gio_onHide
func gio_onHide(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.setStage(StagePaused)
	})
}

//export gio_onShow
func gio_onShow(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.setStage(StageRunning)
	})
}

//export gio_onCreate
func gio_onCreate(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := &window{
			view: view,
		}
		wopts := <-mainWindow.out
		w.w = wopts.window
		w.w.setDriver(w)
		views[view] = w
	})
}

func createWindow(win *Window, opts *WindowOptions) error {
	mainWindow.in <- windowAndOptions{win, opts}
	return <-mainWindow.errs
}

func Main() {
	wopts := <-mainWindow.out
	view := viewFactory()
	if view == 0 {
		// TODO: return this error from CreateWindow.
		panic(errors.New("CreateWindow: failed to create view"))
	}
	cfg := getConfig()
	opts := wopts.opts
	w := cfg.Px(opts.Width)
	h := cfg.Px(opts.Height)
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
