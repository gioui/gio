// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nox11

package window

import "gioui.org/app/internal/gl"

func (w *x11Window) NewContext() (gl.Context, error) {
	return newContext(w)
}

func (w *x11Window) eglDestroy() {
	w.destroy()
}

func (w *x11Window) eglDisplay() _EGLNativeDisplayType {
	return _EGLNativeDisplayType(w.display())
}

func (w *x11Window) eglWindow(visID int) (_EGLNativeWindowType, int, int, error) {
	return _EGLNativeWindowType(uintptr(w.xw)), w.width, w.height, nil
}

func (w *x11Window) needVSync() bool { return true }
