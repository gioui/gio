// SPDX-License-Identifier: Unlicense OR MIT

package app

/*
#cgo LDFLAGS: -landroid

#include <android/native_window_jni.h>
#include <android/configuration.h>
#include <android/keycodes.h>
#include <android/input.h>
#include <stdlib.h>
#include "os_android.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
	"unsafe"

	"gioui.org/ui/f32"
	"gioui.org/ui/key"
	"gioui.org/ui/pointer"
	"gioui.org/ui"
)

type window struct {
	*Window

	view C.jobject

	dpi       int
	fontScale float32

	stage   Stage
	started bool

	mu        sync.Mutex
	win       *C.ANativeWindow
	animating bool

	mgetDensity                    C.jmethodID
	mgetFontScale                  C.jmethodID
	mshowTextInput                 C.jmethodID
	mhideTextInput                 C.jmethodID
	mpostFrameCallback             C.jmethodID
	mpostFrameCallbackOnMainThread C.jmethodID
}

var theJVM *C.JavaVM

var windows = make(chan *Window)

var views = make(map[C.jlong]*window)

func jniGetMethodID(env *C.JNIEnv, class C.jclass, method, sig string) C.jmethodID {
	m := C.CString(method)
	defer C.free(unsafe.Pointer(m))
	s := C.CString(sig)
	defer C.free(unsafe.Pointer(s))
	return C.gio_jni_GetMethodID(env, class, m, s)
}

func jniGetStaticMethodID(env *C.JNIEnv, class C.jclass, method, sig string) C.jmethodID {
	m := C.CString(method)
	defer C.free(unsafe.Pointer(m))
	s := C.CString(sig)
	defer C.free(unsafe.Pointer(s))
	return C.gio_jni_GetStaticMethodID(env, class, m, s)
}

//export setJVM
func setJVM(vm *C.JavaVM) {
	theJVM = vm
}

//export onCreateView
func onCreateView(env *C.JNIEnv, class C.jclass, view C.jobject) C.jlong {
	view = C.gio_jni_NewGlobalRef(env, view)
	w := &window{
		view:                           view,
		mgetDensity:                    jniGetMethodID(env, class, "getDensity", "()I"),
		mgetFontScale:                  jniGetMethodID(env, class, "getFontScale", "()F"),
		mshowTextInput:                 jniGetMethodID(env, class, "showTextInput", "()V"),
		mhideTextInput:                 jniGetMethodID(env, class, "hideTextInput", "()V"),
		mpostFrameCallback:             jniGetMethodID(env, class, "postFrameCallback", "()V"),
		mpostFrameCallbackOnMainThread: jniGetMethodID(env, class, "postFrameCallbackOnMainThread", "()V"),
	}
	ow := newWindow(w)
	w.Window = ow
	handle := C.jlong(view)
	views[handle] = w
	w.loadConfig(env, class)
	windows <- ow
	w.setStage(StageInvisible)
	return handle
}

//export onDestroyView
func onDestroyView(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	delete(views, handle)
	w.setStage(StageDead)
	C.gio_jni_DeleteGlobalRef(env, w.view)
	w.view = 0
}

//export onStopView
func onStopView(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	w.started = false
	w.setStage(StageInvisible)
}

//export onStartView
func onStartView(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	w.started = true
	if w.aNativeWindow() != nil {
		w.setVisible()
	}
}

//export onSurfaceDestroyed
func onSurfaceDestroyed(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	w.mu.Lock()
	w.win = nil
	w.mu.Unlock()
	w.setStage(StageInvisible)
}

//export onSurfaceChanged
func onSurfaceChanged(env *C.JNIEnv, class C.jclass, handle C.jlong, surf C.jobject) {
	w := views[handle]
	w.mu.Lock()
	w.win = C.ANativeWindow_fromSurface(env, surf)
	w.mu.Unlock()
	if w.started {
		w.setVisible()
	}
}

//export onLowMemory
func onLowMemory() {
	runtime.GC()
	debug.FreeOSMemory()
}

//export onConfigurationChanged
func onConfigurationChanged(env *C.JNIEnv, class C.jclass, view C.jlong) {
	w := views[view]
	w.loadConfig(env, class)
	if w.stage >= StageVisible {
		w.draw(true)
	}
}

//export onFrameCallback
func onFrameCallback(env *C.JNIEnv, class C.jclass, view C.jlong, nanos C.jlong) {
	w, exist := views[view]
	if !exist {
		return
	}
	if w.stage < StageVisible {
		return
	}
	w.mu.Lock()
	anim := w.animating
	w.mu.Unlock()
	if anim {
		runInJVM(func(env *C.JNIEnv) {
			C.gio_jni_CallVoidMethod(env, w.view, w.mpostFrameCallback)
		})
		w.draw(false)
	}
}

func (w *window) setVisible() {
	win := w.aNativeWindow()
	width, height := C.ANativeWindow_getWidth(win), C.ANativeWindow_getHeight(win)
	if width == 0 || height == 0 {
		return
	}
	w.setStage(StageVisible)
	w.draw(true)
}

func (w *window) setStage(stage Stage) {
	if stage == w.stage {
		return
	}
	w.stage = stage
	w.event(ChangeStage{stage})
}

func (w *window) display() unsafe.Pointer {
	return nil
}

func (w *window) nativeWindow(visID int) (unsafe.Pointer, int, int) {
	win := w.aNativeWindow()
	var width, height int
	if win != nil {
		if C.ANativeWindow_setBuffersGeometry(win, 0, 0, C.int32_t(visID)) != 0 {
			panic(errors.New("ANativeWindow_setBuffersGeometry failed"))
		}
		w, h := C.ANativeWindow_getWidth(win), C.ANativeWindow_getHeight(win)
		width, height = int(w), int(h)
	}
	return unsafe.Pointer(win), width, height
}

func (w *window) aNativeWindow() *C.ANativeWindow {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.win
}

func (w *window) loadConfig(env *C.JNIEnv, class C.jclass) {
	dpi := int(C.gio_jni_CallIntMethod(env, w.view, w.mgetDensity))
	w.fontScale = float32(C.gio_jni_CallFloatMethod(env, w.view, w.mgetFontScale))
	switch dpi {
	case C.ACONFIGURATION_DENSITY_NONE,
		C.ACONFIGURATION_DENSITY_DEFAULT,
		C.ACONFIGURATION_DENSITY_ANY:
		// Assume standard density.
		w.dpi = C.ACONFIGURATION_DENSITY_MEDIUM
	default:
		w.dpi = int(dpi)
	}
}

func (w *window) setAnimating(anim bool) {
	w.mu.Lock()
	w.animating = anim
	w.mu.Unlock()
	if anim {
		runInJVM(func(env *C.JNIEnv) {
			C.gio_jni_CallVoidMethod(env, w.view, w.mpostFrameCallbackOnMainThread)
		})
	}
}

func (w *window) draw(sync bool) {
	win := w.aNativeWindow()
	width, height := C.ANativeWindow_getWidth(win), C.ANativeWindow_getHeight(win)
	if width == 0 || height == 0 {
		return
	}
	ppdp := float32(w.dpi) * inchPrDp
	w.event(Draw{
		Size: image.Point{
			X: int(width),
			Y: int(height),
		},
		Config: &ui.Config{
			PxPerDp: ppdp,
			PxPerSp: w.fontScale * ppdp,
			Now:     time.Now(),
		},
		sync: sync,
	})
}

type keyMapper func(devId, keyCode C.int32_t) rune

func runInJVM(f func(env *C.JNIEnv)) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var env *C.JNIEnv
	var detach bool
	if res := C.gio_jni_GetEnv(theJVM, &env, C.JNI_VERSION_1_6); res != C.JNI_OK {
		if res != C.JNI_EDETACHED {
			panic(fmt.Errorf("JNI GetEnv failed with error %d", res))
		}
		if C.gio_jni_AttachCurrentThread(theJVM, &env, nil) != C.JNI_OK {
			panic(errors.New("runInJVM: AttachCurrentThread failed"))
		}
		detach = true
	}

	if detach {
		defer func() {
			C.gio_jni_DetachCurrentThread(theJVM)
		}()
	}
	f(env)
}

func convertKeyCode(code C.jint) (rune, bool) {
	var n rune
	switch code {
	case C.AKEYCODE_DPAD_UP:
		n = key.NameUpArrow
	case C.AKEYCODE_DPAD_DOWN:
		n = key.NameDownArrow
	case C.AKEYCODE_DPAD_LEFT:
		n = key.NameLeftArrow
	case C.AKEYCODE_DPAD_RIGHT:
		n = key.NameRightArrow
	case C.AKEYCODE_FORWARD_DEL:
		n = key.NameDeleteForward
	case C.AKEYCODE_DEL:
		n = key.NameDeleteBackward
	default:
		return 0, false
	}
	return n, true
}

//export onKeyEvent
func onKeyEvent(env *C.JNIEnv, class C.jclass, handle C.jlong, keyCode, r C.jint, t C.jlong) {
	w := views[handle]
	if n, ok := convertKeyCode(keyCode); ok {
		w.event(key.Chord{Name: n})
	}
	if r != 0 {
		w.event(key.Edit{Text: string(rune(r))})
	}
}

//export onTouchEvent
func onTouchEvent(env *C.JNIEnv, class C.jclass, handle C.jlong, action, pointerID, tool C.jint, x, y C.jfloat, t C.jlong) {
	w := views[handle]
	var typ pointer.Type
	switch action {
	case C.AMOTION_EVENT_ACTION_DOWN, C.AMOTION_EVENT_ACTION_POINTER_DOWN:
		typ = pointer.Press
	case C.AMOTION_EVENT_ACTION_UP, C.AMOTION_EVENT_ACTION_POINTER_UP:
		typ = pointer.Release
	case C.AMOTION_EVENT_ACTION_CANCEL:
		typ = pointer.Cancel
	case C.AMOTION_EVENT_ACTION_MOVE:
		typ = pointer.Move
	default:
		return
	}
	var src pointer.Source
	switch tool {
	case C.AMOTION_EVENT_TOOL_TYPE_FINGER:
		src = pointer.Touch
	case C.AMOTION_EVENT_TOOL_TYPE_MOUSE:
		src = pointer.Mouse
	default:
		return
	}
	w.event(pointer.Event{
		Type:      typ,
		Source:    src,
		PointerID: pointer.ID(pointerID),
		Time:      time.Duration(t) * time.Millisecond,
		Position:  f32.Point{X: float32(x), Y: float32(y)},
	})
}

func (w *window) setTextInput(s key.TextInputState) {
	if w.view == 0 {
		return
	}
	switch s {
	case key.TextInputOpen:
		runInJVM(func(env *C.JNIEnv) {
			C.gio_jni_CallVoidMethod(env, w.view, w.mshowTextInput)
		})
	case key.TextInputClosed:
		runInJVM(func(env *C.JNIEnv) {
			C.gio_jni_CallVoidMethod(env, w.view, w.mhideTextInput)
		})
	}
}

func Main() {
	// Android runs in c-shared mode where is never reached.
	panic("unreachable")
}

func createWindow(opts *WindowOptions) (*Window, error) {
	return <-windows, nil
}
