// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"gioui.org/app/internal/egl"
	"gioui.org/app/internal/gl"
)

func (w *window) EGLDestroy() {
}

func (w *window) EGLDisplay() egl.NativeDisplayType {
	return egl.NativeDisplayType(w.HDC())
}

func (w *window) EGLWindow(visID int) (egl.NativeWindowType, int, int, error) {
	hwnd, width, height := w.HWND()
	return egl.NativeWindowType(hwnd), width, height, nil
}

func (w *window) NewContext() (gl.Context, error) {
	return egl.NewContext(w)
}

func (w *window) NeedVSync() bool { return true }
