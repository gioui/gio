// SPDX-License-Identifier: Unlicense OR MIT

// +build android windows

package app

type eglWindow struct {
	w _EGLNativeWindowType
}

func newEGLWindow(w _EGLNativeWindowType, width, height int) (*eglWindow, error) {
	return &eglWindow{w}, nil
}

func (w *eglWindow) window() _EGLNativeWindowType {
	return w.w
}

func (w *eglWindow) resize(width, height int) {}
func (w *eglWindow) destroy()                 {}
