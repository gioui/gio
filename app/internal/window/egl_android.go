// SPDX-License-Identifier: Unlicense OR MIT

package window

/*
#include <EGL/egl.h>
*/
import "C"

import (
	"unsafe"

	"gioui.org/app/internal/egl"
	"gioui.org/app/internal/gl"
)

func (w *window) EGLDestroy() {
}

func (w *window) EGLDisplay() egl.NativeDisplayType {
	return nil
}

func (w *window) EGLWindow(visID int) (egl.NativeWindowType, int, int, error) {
	win, width, height := w.nativeWindow(visID)
	return egl.NativeWindowType(unsafe.Pointer(win)), width, height, nil
}

func (w *window) NewContext() (gl.Context, error) {
	return egl.NewContext(w)
}

func (w *window) NeedVSync() bool { return false }
