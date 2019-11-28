// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nox11 freebsd

package window

import (
	"gioui.org/app/internal/egl"
	"gioui.org/app/internal/gl"
)

func (w *x11Window) NewContext() (gl.Context, error) {
	return egl.NewContext(w)
}

func (w *x11Window) EGLDestroy() {
}

func (w *x11Window) EGLDisplay() egl.NativeDisplayType {
	return egl.NativeDisplayType(w.display())
}

func (w *x11Window) EGLWindow(visID int) (egl.NativeWindowType, int, int, error) {
	return egl.NativeWindowType(uintptr(w.xw)), w.width, w.height, nil
}

func (w *x11Window) NeedVSync() bool { return true }
