// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

package window

import (
	"errors"
	"image"
	"runtime"
	"sync"
	"time"
	"unicode"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
)

/*
#cgo CFLAGS: -DGL_SILENCE_DEPRECATION -Werror -Wno-deprecated-declarations -fmodules -fobjc-arc -x objective-c


#include <AppKit/AppKit.h>
#include "os_macos.h"
*/
import "C"

func init() {
	// Darwin requires that UI operations happen on the main thread only.
	runtime.LockOSThread()
}

type window struct {
	view  C.CFTypeRef
	w     Callbacks
	stage system.Stage
	scale float32
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

func (w *window) ShowTextInput(show bool) {}

func (w *window) SetAnimating(anim bool) {
	var animb C.BOOL
	if anim {
		animb = 1
	}
	C.gio_setAnimating(w.view, animb)
}

func (w *window) setStage(stage system.Stage) {
	if stage == w.stage {
		return
	}
	w.stage = stage
	w.w.Event(system.StageEvent{Stage: stage})
}

// Use a top level func for onFrameCallback to avoid
// garbage from viewDo.
func onFrameCmd(views viewMap, view C.CFTypeRef) {
	// CVDisplayLink does not run on the main thread,
	// so we have to ignore requests to windows being
	// deleted.
	if w, exists := views[view]; exists {
		w.draw(false)
	}
}

//export gio_onFrameCallback
func gio_onFrameCallback(view C.CFTypeRef) {
	viewDo(view, onFrameCmd)
}

//export gio_onKeys
func gio_onKeys(view C.CFTypeRef, cstr *C.char, ti C.double, mods C.NSUInteger) {
	str := C.GoString(cstr)
	kmods := convertMods(mods)
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		for _, k := range str {
			if n, ok := convertKey(k); ok {
				w.w.Event(key.Event{
					Name:      n,
					Modifiers: kmods,
				})
			}
		}
	})
}

//export gio_onText
func gio_onText(view C.CFTypeRef, cstr *C.char) {
	str := C.GoString(cstr)
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.w.Event(key.EditEvent{Text: str})
	})
}

//export gio_onMouse
func gio_onMouse(view C.CFTypeRef, cdir C.int, cbtns C.NSUInteger, x, y, dx, dy C.CGFloat, ti C.double, mods C.NSUInteger) {
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
	var btns pointer.Buttons
	if cbtns&(1<<0) != 0 {
		btns |= pointer.ButtonLeft
	}
	if cbtns&(1<<1) != 0 {
		btns |= pointer.ButtonRight
	}
	if cbtns&(1<<2) != 0 {
		btns |= pointer.ButtonMiddle
	}
	t := time.Duration(float64(ti)*float64(time.Second) + .5)
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		x, y := float32(x)*w.scale, float32(y)*w.scale
		dx, dy := float32(dx)*w.scale, float32(dy)*w.scale
		w.w.Event(pointer.Event{
			Type:      typ,
			Source:    pointer.Mouse,
			Time:      t,
			Buttons:   btns,
			Position:  f32.Point{X: x, Y: y},
			Scroll:    f32.Point{X: dx, Y: dy},
			Modifiers: convertMods(mods),
		})
	})
}

//export gio_onDraw
func gio_onDraw(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		if w, exists := views[view]; exists {
			w.draw(true)
		}
	})
}

//export gio_onFocus
func gio_onFocus(view C.CFTypeRef, focus C.BOOL) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.w.Event(key.FocusEvent{Focus: focus == C.YES})
	})
}

func (w *window) draw(sync bool) {
	w.scale = float32(C.gio_getViewBackingScale(w.view))
	wf, hf := float32(C.gio_viewWidth(w.view)), float32(C.gio_viewHeight(w.view))
	if wf == 0 || hf == 0 {
		return
	}
	width := int(wf*w.scale + .5)
	height := int(hf*w.scale + .5)
	cfg := configFor(w.scale)
	cfg.now = time.Now()
	w.setStage(system.StageRunning)
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

func configFor(scale float32) config {
	return config{
		pxPerDp: scale,
		pxPerSp: scale,
	}
}

//export gio_onTerminate
func gio_onTerminate(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		delete(views, view)
		w.w.Event(system.DestroyEvent{})
	})
}

//export gio_onHide
func gio_onHide(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.setStage(system.StagePaused)
	})
}

//export gio_onShow
func gio_onShow(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		w := views[view]
		w.setStage(system.StageRunning)
	})
}

//export gio_onCreate
func gio_onCreate(view C.CFTypeRef) {
	viewDo(view, func(views viewMap, view C.CFTypeRef) {
		scale := float32(C.gio_getViewBackingScale(view))
		w := &window{
			view:  view,
			scale: scale,
		}
		wopts := <-mainWindow.out
		w.w = wopts.window
		w.w.SetDriver(w)
		views[view] = w
	})
}

func NewWindow(win Callbacks, opts *Options) error {
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
	// Window sizes is in unscaled screen coordinates, not device pixels.
	cfg := configFor(1.0)
	opts := wopts.opts
	w := cfg.Px(opts.Width)
	h := cfg.Px(opts.Height)
	w = int(float32(w))
	h = int(float32(h))
	title := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(title))
	C.gio_main(view, title, C.CGFloat(w), C.CGFloat(h))
}

func convertKey(k rune) (string, bool) {
	var n string
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
	case 0x3:
		n = key.NameEnter
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
	case C.NSF1FunctionKey:
		n = "F1"
	case C.NSF2FunctionKey:
		n = "F2"
	case C.NSF3FunctionKey:
		n = "F3"
	case C.NSF4FunctionKey:
		n = "F4"
	case C.NSF5FunctionKey:
		n = "F5"
	case C.NSF6FunctionKey:
		n = "F6"
	case C.NSF7FunctionKey:
		n = "F7"
	case C.NSF8FunctionKey:
		n = "F8"
	case C.NSF9FunctionKey:
		n = "F9"
	case C.NSF10FunctionKey:
		n = "F10"
	case C.NSF11FunctionKey:
		n = "F11"
	case C.NSF12FunctionKey:
		n = "F12"
	case 0x09, 0x19:
		n = key.NameTab
	case 0x20:
		n = "Space"
	default:
		k = unicode.ToUpper(k)
		if !unicode.IsPrint(k) {
			return "", false
		}
		n = string(k)
	}
	return n, true
}

func convertMods(mods C.NSUInteger) key.Modifiers {
	var kmods key.Modifiers
	if mods&C.NSAlternateKeyMask != 0 {
		kmods |= key.ModAlt
	}
	if mods&C.NSControlKeyMask != 0 {
		kmods |= key.ModCtrl
	}
	if mods&C.NSCommandKeyMask != 0 {
		kmods |= key.ModCommand
	}
	if mods&C.NSShiftKeyMask != 0 {
		kmods |= key.ModShift
	}
	return kmods
}
