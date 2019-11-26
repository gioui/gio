// SPDX-License-Identifier: Unlicense OR MIT

package window

/*
#cgo CFLAGS: -Werror
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

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type window struct {
	callbacks Callbacks

	view C.jobject

	dpi       int
	fontScale float32
	insets    system.Insets

	stage   system.Stage
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
	mRegisterFragment              C.jmethodID
}

var dataDirChan = make(chan string, 1)

var theJVM *C.JavaVM

var views = make(map[C.jlong]*window)

var mainWindow = newWindowRendezvous()

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

//export runGoMain
func runGoMain(env *C.JNIEnv, class C.jclass, jdataDir C.jbyteArray, context C.jobject) {
	dirBytes := C.gio_jni_GetByteArrayElements(env, jdataDir)
	if dirBytes == nil {
		panic("runGoMain: GetByteArrayElements failed")
	}
	n := C.gio_jni_GetArrayLength(env, jdataDir)
	dataDir := C.GoStringN((*C.char)(unsafe.Pointer(dirBytes)), n)
	dataDirChan <- dataDir
	C.gio_jni_ReleaseByteArrayElements(env, jdataDir, dirBytes)
	context = C.gio_jni_NewGlobalRef(env, context)

	PlatformHandle = &Handle{
		JVM:     uintptr(unsafe.Pointer(theJVM)),
		Context: uintptr(context),
	}
	runMain()
}

func GetDataDir() string {
	return <-dataDirChan
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
		mRegisterFragment:              jniGetMethodID(env, class, "registerFragment", "(Ljava/lang/String;)V"),
	}
	wopts := <-mainWindow.out
	w.callbacks = wopts.window
	w.callbacks.SetDriver(w)
	handle := C.jlong(view)
	views[handle] = w
	w.loadConfig(env, class)
	w.setStage(system.StagePaused)
	return handle
}

//export onDestroyView
func onDestroyView(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	w.callbacks.SetDriver(nil)
	delete(views, handle)
	C.gio_jni_DeleteGlobalRef(env, w.view)
	w.view = 0
}

//export onStopView
func onStopView(env *C.JNIEnv, class C.jclass, handle C.jlong) {
	w := views[handle]
	w.started = false
	w.setStage(system.StagePaused)
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
	w.setStage(system.StagePaused)
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
	if w.stage >= system.StageRunning {
		w.draw(true)
	}
}

//export onFrameCallback
func onFrameCallback(env *C.JNIEnv, class C.jclass, view C.jlong, nanos C.jlong) {
	w, exist := views[view]
	if !exist {
		return
	}
	if w.stage < system.StageRunning {
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

//export onBack
func onBack(env *C.JNIEnv, class C.jclass, view C.jlong) C.jboolean {
	w := views[view]
	ev := &system.CommandEvent{Type: system.CommandBack}
	w.callbacks.Event(ev)
	if ev.Cancel {
		return C.JNI_TRUE
	}
	return C.JNI_FALSE
}

//export onFocusChange
func onFocusChange(env *C.JNIEnv, class C.jclass, view C.jlong, focus C.jboolean) {
	w := views[view]
	w.callbacks.Event(key.FocusEvent{Focus: focus == C.JNI_TRUE})
}

//export onWindowInsets
func onWindowInsets(env *C.JNIEnv, class C.jclass, view C.jlong, top, right, bottom, left C.jint) {
	w := views[view]
	w.insets = system.Insets{
		Top:    unit.Px(float32(top)),
		Right:  unit.Px(float32(right)),
		Bottom: unit.Px(float32(bottom)),
		Left:   unit.Px(float32(left)),
	}
	if w.stage >= system.StageRunning {
		w.draw(true)
	}
}

func (w *window) setVisible() {
	win := w.aNativeWindow()
	width, height := C.ANativeWindow_getWidth(win), C.ANativeWindow_getHeight(win)
	if width == 0 || height == 0 {
		return
	}
	w.setStage(system.StageRunning)
	w.draw(true)
}

func (w *window) setStage(stage system.Stage) {
	if stage == w.stage {
		return
	}
	w.stage = stage
	w.callbacks.Event(system.StageEvent{stage})
}

func (w *window) nativeWindow(visID int) (*C.ANativeWindow, int, int) {
	win := w.aNativeWindow()
	var width, height int
	if win != nil {
		if C.ANativeWindow_setBuffersGeometry(win, 0, 0, C.int32_t(visID)) != 0 {
			panic(errors.New("ANativeWindow_setBuffersGeometry failed"))
		}
		w, h := C.ANativeWindow_getWidth(win), C.ANativeWindow_getHeight(win)
		width, height = int(w), int(h)
	}
	return win, width, height
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

func (w *window) SetAnimating(anim bool) {
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
	const inchPrDp = 1.0 / 160
	ppdp := float32(w.dpi) * inchPrDp
	w.callbacks.Event(FrameEvent{
		FrameEvent: system.FrameEvent{
			Size: image.Point{
				X: int(width),
				Y: int(height),
			},
			Insets: w.insets,
			Config: &config{
				pxPerDp: ppdp,
				pxPerSp: w.fontScale * ppdp,
				now:     time.Now(),
			},
		},
		Sync: sync,
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

func convertKeyCode(code C.jint) (string, bool) {
	var n string
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
		return "", false
	}
	return n, true
}

//export onKeyEvent
func onKeyEvent(env *C.JNIEnv, class C.jclass, handle C.jlong, keyCode, r C.jint, t C.jlong) {
	w := views[handle]
	if n, ok := convertKeyCode(keyCode); ok {
		w.callbacks.Event(key.Event{Name: n})
	}
	if r != 0 {
		w.callbacks.Event(key.EditEvent{Text: string(rune(r))})
	}
}

//export onTouchEvent
func onTouchEvent(env *C.JNIEnv, class C.jclass, handle C.jlong, action, pointerID, tool C.jint, x, y C.jfloat, jbtns C.jint, t C.jlong) {
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
	var btns pointer.Buttons
	if jbtns&C.AMOTION_EVENT_BUTTON_PRIMARY != 0 {
		btns |= pointer.ButtonLeft
	}
	if jbtns&C.AMOTION_EVENT_BUTTON_SECONDARY != 0 {
		btns |= pointer.ButtonRight
	}
	if jbtns&C.AMOTION_EVENT_BUTTON_TERTIARY != 0 {
		btns |= pointer.ButtonMiddle
	}
	switch tool {
	case C.AMOTION_EVENT_TOOL_TYPE_FINGER:
		src = pointer.Touch
	case C.AMOTION_EVENT_TOOL_TYPE_MOUSE:
		src = pointer.Mouse
	default:
		return
	}
	w.callbacks.Event(pointer.Event{
		Type:      typ,
		Source:    src,
		Buttons:   btns,
		PointerID: pointer.ID(pointerID),
		Time:      time.Duration(t) * time.Millisecond,
		Position:  f32.Point{X: float32(x), Y: float32(y)},
	})
}

func (w *window) ShowTextInput(show bool) {
	if w.view == 0 {
		return
	}
	runInJVM(func(env *C.JNIEnv) {
		if show {
			C.gio_jni_CallVoidMethod(env, w.view, w.mshowTextInput)
		} else {
			C.gio_jni_CallVoidMethod(env, w.view, w.mhideTextInput)
		}
	})
}

func (w *window) RegisterFragment(del string) {
	runInJVM(func(env *C.JNIEnv) {
		cdel := C.CString(del)
		defer C.free(unsafe.Pointer(cdel))
		C.gio_jni_RegisterFragment(env, w.view, w.mRegisterFragment, cdel)
	})
}

func Main() {
}

func NewWindow(window Callbacks, opts *Options) error {
	mainWindow.in <- windowAndOptions{window, opts}
	return <-mainWindow.errs
}
