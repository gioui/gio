// SPDX-License-Identifier: Unlicense OR MIT

//go:build !noopengl

package app

/*
#include <android/native_window_jni.h>
#include <EGL/egl.h>
*/
import "C"

import (
	"fmt"
	"unsafe"

	"gioui.org/internal/egl"
)

type androidContext struct {
	win     *window
	eglSurf egl.NativeWindowType
	*egl.Context
}

func init() {
	newAndroidGLESContext = func(w *window) (context, error) {
		ctx, err := egl.NewContext(nil)
		if err != nil {
			return nil, err
		}
		return &androidContext{win: w, Context: ctx}, nil
	}
}

func (c *androidContext) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
}

func (c *androidContext) Refresh() error {
	c.Context.ReleaseSurface()
	if err := c.win.setVisual(c.Context.VisualID()); err != nil {
		return err
	}
	win, _, _ := c.win.nativeWindow()
	c.eglSurf = egl.NativeWindowType(unsafe.Pointer(win))
	return nil
}

func (c *androidContext) Lock() error {
	// The Android emulator creates a broken surface if it is not
	// created on the same thread as the context is made current.
	if c.eglSurf != nil {
		if err := c.Context.CreateSurface(c.eglSurf); err != nil {
			return err
		}
		if format := C.ANativeWindow_getFormat(c.win.win); format >= 0x100 && format <= 0x1ff {
			// Some drivers turn sRGB surfaces into vendor buffer formats that
			// SurfaceFlinger treats as opaque, even when their pixels have alpha.
			// Retry with a standard native format and let Gio emulate sRGB.
			c.Context.ReleaseSurface()
			c.Context.DisableSRGB()
			if err := c.win.setVisual(c.Context.VisualID()); err != nil {
				return err
			}
			if err := c.Context.CreateSurface(c.eglSurf); err != nil {
				return err
			}
			if format := C.ANativeWindow_getFormat(c.win.win); format >= 0x100 && format <= 0x1ff {
				c.eglSurf = nil
				return fmt.Errorf("Android EGL surface uses opaque native buffer format %#x", format)
			}
		}
		c.eglSurf = nil
	}
	return c.Context.MakeCurrent()
}

func (c *androidContext) Unlock() {
	c.Context.ReleaseCurrent()
}
