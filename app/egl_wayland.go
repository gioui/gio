// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android

package app

import (
	"errors"
	"unsafe"
)

/*
#cgo LDFLAGS: -lwayland-egl
#cgo CFLAGS: -DWL_EGL_PLATFORM

#include <wayland-client.h>
#include <wayland-egl.h>
#include <EGL/egl.h>
*/
import "C"

type (
	_EGLNativeDisplayType = C.EGLNativeDisplayType
	_EGLNativeWindowType  = C.EGLNativeWindowType
)

type eglWindow struct {
	w *C.struct_wl_egl_window
}

func newEGLWindow(w _EGLNativeWindowType, width, height int) (*eglWindow, error) {
	surf := (*C.struct_wl_surface)(unsafe.Pointer(w))
	win := C.wl_egl_window_create(surf, C.int(width), C.int(height))
	if win == nil {
		return nil, errors.New("wl_egl_create_window failed")
	}
	return &eglWindow{win}, nil
}

func (w *eglWindow) window() _EGLNativeWindowType {
	return w.w
}

func (w *eglWindow) resize(width, height int) {
	C.wl_egl_window_resize(w.w, C.int(width), C.int(height), 0, 0)
}

func (w *eglWindow) destroy() {
	C.wl_egl_window_destroy(w.w)
}

func eglGetDisplay(disp _EGLNativeDisplayType) _EGLDisplay {
	return C.eglGetDisplay(disp)
}

func eglCreateWindowSurface(disp _EGLDisplay, conf _EGLConfig, win _EGLNativeWindowType, attribs []_EGLint) _EGLSurface {
	eglSurf := C.eglCreateWindowSurface(disp, conf, win, &attribs[0])
	return eglSurf
}
