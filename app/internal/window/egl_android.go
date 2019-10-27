// SPDX-License-Identifier: Unlicense OR MIT

package window

/*
#include <EGL/egl.h>
*/
import "C"
import "gioui.org/app/internal/gl"

func (w *window) eglDestroy() {
}

func (w *window) eglDisplay() _EGLNativeDisplayType {
	return nil
}

func (w *window) eglWindow(visID int) (_EGLNativeWindowType, int, int, error) {
	win, width, height := w.nativeWindow(visID)
	return _EGLNativeWindowType(win), width, height, nil
}

func (w *window) NewContext() (gl.Context, error) {
	return newContext(w)
}

func (w *window) needVSync() bool { return false }
