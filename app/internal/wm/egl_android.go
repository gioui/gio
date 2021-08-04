// SPDX-License-Identifier: Unlicense OR MIT

package wm

/*
#include <android/native_window_jni.h>
#include <EGL/egl.h>
*/
import "C"

import (
	"unsafe"

	"gioui.org/internal/egl"
)

type context struct {
	win *window
	*egl.Context
}

func (w *window) NewContext() (Context, error) {
	ctx, err := egl.NewContext(nil)
	if err != nil {
		return nil, err
	}
	return &context{win: w, Context: ctx}, nil
}

func (c *context) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
}

func (c *context) Refresh() error {
	c.Context.ReleaseSurface()
	var (
		win           *C.ANativeWindow
		width, height int
	)
	win, width, height = c.win.nativeWindow(c.Context.VisualID())
	if win == nil {
		return nil
	}
	eglSurf := egl.NativeWindowType(unsafe.Pointer(win))
	return c.Context.CreateSurface(eglSurf, width, height)
}

func (c *context) Lock() error {
	return c.Context.MakeCurrent()
}

func (c *context) Unlock() {
	c.Context.ReleaseCurrent()
}
