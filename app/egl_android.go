// SPDX-License-Identifier: Unlicense OR MIT

package app

/*
#include <android/native_window_jni.h>
#include <EGL/egl.h>
*/
import "C"

import (
	"unsafe"

	"gioui.org/internal/egl"
)

type androidContext struct {
	win *window
	*egl.Context
}

func (w *window) NewContext() (context, error) {
	ctx, err := egl.NewContext(nil)
	if err != nil {
		return nil, err
	}
	return &androidContext{win: w, Context: ctx}, nil
}

func (c *androidContext) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
}

func (c *androidContext) Refresh() error {
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

func (c *androidContext) Lock() error {
	return c.Context.MakeCurrent()
}

func (c *androidContext) Unlock() {
	c.Context.ReleaseCurrent()
}
