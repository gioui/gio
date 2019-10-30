// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nowayland freebsd

package window

import (
	"errors"
	"sync"
	"unsafe"

	"gioui.org/app/internal/gl"
)

/*
#cgo LDFLAGS: -lwayland-egl

#include <EGL/egl.h>
#include <wayland-client.h>
#include <wayland-egl.h>
*/
import "C"

var eglWindows struct {
	mu      sync.Mutex
	windows map[*C.struct_wl_surface]*C.struct_wl_egl_window
}

func (w *window) eglDestroy() {
	surf, _, _ := w.surface()
	if surf == nil {
		return
	}
	eglWindows.mu.Lock()
	defer eglWindows.mu.Unlock()
	if eglWin, ok := eglWindows.windows[surf]; ok {
		C.wl_egl_window_destroy(eglWin)
		delete(eglWindows.windows, surf)
	}
}

func (w *window) eglDisplay() _EGLNativeDisplayType {
	return _EGLNativeDisplayType(unsafe.Pointer(w.display()))
}

func (w *window) eglWindow(visID int) (_EGLNativeWindowType, int, int, error) {
	surf, width, height := w.surface()
	if surf == nil {
		return nilEGLNativeWindowType, 0, 0, errors.New("wayland: no surface")
	}
	eglWindows.mu.Lock()
	defer eglWindows.mu.Unlock()
	eglWin, ok := eglWindows.windows[surf]
	if !ok {
		if eglWindows.windows == nil {
			eglWindows.windows = make(map[*C.struct_wl_surface]*C.struct_wl_egl_window)
		}
		eglWin = C.wl_egl_window_create(surf, C.int(width), C.int(height))
		if eglWin == nil {
			return nilEGLNativeWindowType, 0, 0, errors.New("wayland: wl_egl_create_window failed")
		}
		eglWindows.windows[surf] = eglWin
	}
	C.wl_egl_window_resize(eglWin, C.int(width), C.int(height), 0, 0)
	return _EGLNativeWindowType(uintptr(unsafe.Pointer(eglWin))), width, height, nil
}

func (w *window) NewContext() (gl.Context, error) {
	return newContext(w)
}

func (w *window) needVSync() bool { return false }
