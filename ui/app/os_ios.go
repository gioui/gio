// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,ios

package app

/*
#cgo CFLAGS: -fmodules -fobjc-arc -x objective-c

#include <CoreGraphics/CoreGraphics.h>
#include <UIKit/UIKit.h>
#include <stdint.h>
#include "os_ios.h"

*/
import "C"

import (
	"image"
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/key"
	"gioui.org/ui/pointer"
)

type window struct {
	view C.CFTypeRef
	w    *Window

	layer   C.CFTypeRef
	visible atomic.Value

	pointerMap []C.CFTypeRef
}

var mainWindow = newWindowRendezvous()

var layerFactory func() uintptr

var views = make(map[C.CFTypeRef]*window)

func init() {
	// Darwin requires UI operations happen on the main thread only.
	runtime.LockOSThread()
}

//export onCreate
func onCreate(view C.CFTypeRef) {
	w := &window{
		view: view,
	}
	wopts := <-mainWindow.out
	w.w = wopts.window
	w.w.setDriver(w)
	w.visible.Store(false)
	w.layer = C.CFTypeRef(layerFactory())
	C.gio_addLayerToView(view, w.layer)
	views[view] = w
	w.w.event(StageEvent{StagePaused})
}

//export onDraw
func onDraw(view C.CFTypeRef, dpi, sdpi, width, height C.CGFloat, sync C.int, top, right, bottom, left C.CGFloat) {
	if width == 0 || height == 0 {
		return
	}
	w := views[view]
	wasVisible := w.isVisible()
	w.visible.Store(true)
	C.gio_updateView(view, w.layer)
	if !wasVisible {
		w.w.event(StageEvent{StageRunning})
	}
	isSync := false
	if sync != 0 {
		isSync = true
	}
	w.w.event(DrawEvent{
		Size: image.Point{
			X: int(width + .5),
			Y: int(height + .5),
		},
		Insets: Insets{
			Top:    ui.Px(float32(top)),
			Right:  ui.Px(float32(right)),
			Bottom: ui.Px(float32(bottom)),
			Left:   ui.Px(float32(left)),
		},
		Config: Config{
			pxPerDp: float32(dpi) * inchPrDp,
			pxPerSp: float32(sdpi) * inchPrDp,
			now:     time.Now(),
		},
		sync: isSync,
	})
}

//export onStop
func onStop(view C.CFTypeRef) {
	w := views[view]
	w.visible.Store(false)
	w.w.event(StageEvent{StagePaused})
}

//export onDestroy
func onDestroy(view C.CFTypeRef) {
	w := views[view]
	delete(views, view)
	w.w.event(StageEvent{StageDead})
	C.gio_removeLayer(w.layer)
	C.CFRelease(w.layer)
	w.layer = 0
	w.view = 0
}

//export onFocus
func onFocus(view C.CFTypeRef, focus int) {
	w := views[view]
	w.w.event(key.FocusEvent{Focus: focus != 0})
}

//export onLowMemory
func onLowMemory() {
	runtime.GC()
	debug.FreeOSMemory()
}

//export onUpArrow
func onUpArrow(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameUpArrow)
}

//export onDownArrow
func onDownArrow(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameDownArrow)
}

//export onLeftArrow
func onLeftArrow(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameLeftArrow)
}

//export onRightArrow
func onRightArrow(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameRightArrow)
}

//export onDeleteBackward
func onDeleteBackward(view C.CFTypeRef) {
	views[view].onKeyCommand(key.NameDeleteBackward)
}

//export onText
func onText(view C.CFTypeRef, str *C.char) {
	w := views[view]
	w.w.event(key.EditEvent{
		Text: C.GoString(str),
	})
}

//export onTouch
func onTouch(last C.int, view, touchRef C.CFTypeRef, phase C.NSInteger, x, y C.CGFloat, ti C.double) {
	var typ pointer.Type
	switch phase {
	case C.UITouchPhaseBegan:
		typ = pointer.Press
	case C.UITouchPhaseMoved:
		typ = pointer.Move
	case C.UITouchPhaseEnded:
		typ = pointer.Release
	case C.UITouchPhaseCancelled:
		typ = pointer.Cancel
	default:
		return
	}
	w := views[view]
	t := time.Duration(float64(ti) * float64(time.Second))
	p := f32.Point{X: float32(x), Y: float32(y)}
	w.w.event(pointer.Event{
		Type:      typ,
		Source:    pointer.Touch,
		PointerID: w.lookupTouch(last != 0, touchRef),
		Position:  p,
		Time:      t,
	})
}

func (w *window) setAnimating(anim bool) {
	if w.view == 0 {
		return
	}
	var animi C.int
	if anim {
		animi = 1
	}
	C.gio_setAnimating(w.view, animi)
}

func (w *window) onKeyCommand(name rune) {
	w.w.event(key.ChordEvent{
		Name: name,
	})
}

// lookupTouch maps an UITouch pointer value to an index. If
// last is set, the map is cleared.
func (w *window) lookupTouch(last bool, touch C.CFTypeRef) pointer.ID {
	id := -1
	for i, ref := range w.pointerMap {
		if ref == touch {
			id = i
			break
		}
	}
	if id == -1 {
		id = len(w.pointerMap)
		w.pointerMap = append(w.pointerMap, touch)
	}
	if last {
		w.pointerMap = w.pointerMap[:0]
	}
	return pointer.ID(id)
}

func (w *window) contextLayer() uintptr {
	return uintptr(w.layer)
}

func (w *window) isVisible() bool {
	return w.visible.Load().(bool)
}

func (w *window) setTextInput(s key.TextInputState) {
	if w.view == 0 {
		return
	}
	switch s {
	case key.TextInputOpen:
		C.gio_showTextInput(w.view)
	case key.TextInputClose:
		C.gio_hideTextInput(w.view)
	}
}

func createWindow(win *Window, opts *WindowOptions) error {
	mainWindow.in <- windowAndOptions{win, opts}
	return <-mainWindow.errs
}

func Main() {
	// iOS runs in c-archive mode, so this is never reached.
	panic("call to Main from outside main")
}
