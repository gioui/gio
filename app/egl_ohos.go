// SPDX-License-Identifier: Unlicense OR MIT

//go:build (gio_ohos || openharmony) && !noopengl

package app

/*
#include "ohos_glue.h"
*/
import "C"

import (
	"unsafe"

	"gioui.org/internal/egl"
)

// ohosContext implements Gio's context interface for a HarmonyOS
// OHNativeWindow obtained from an XComponent.  Refresh only records the
// window; the EGL surface is created in Lock on the render thread, exactly
// like the Android backend (docs/44 §4.1).
type ohosContext struct {
	win     *window
	eglSurf egl.NativeWindowType
	*egl.Context
}

var newOHOSGLESContext func(w *window) (context, error)

func init() {
	newOHOSGLESContext = func(w *window) (context, error) {
		ctx, err := egl.NewContext(nil)
		if err != nil {
			return nil, err
		}
		return &ohosContext{win: w, Context: ctx}, nil
	}
}

func (c *ohosContext) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
}

func (c *ohosContext) Refresh() error {
	if c.Context == nil {
		return errOutOfDate
	}
	c.Context.ReleaseSurface()
	win, size := c.win.pump.surfaceInfo()
	if win == 0 || size.X <= 0 || size.Y <= 0 {
		return errOutOfDate
	}
	// Fixed RGBA8888 + explicit geometry: some XComponent surfaces come up
	// with a format EGL rejects (docs/45 WP-3 risk box).
	C.gio_ohos_set_buffer_format(unsafe.Pointer(win), 12 /* NATIVEBUFFER_PIXEL_FMT_RGBA_8888 */)
	C.gio_ohos_set_buffer_geometry(unsafe.Pointer(win), C.int(size.X), C.int(size.Y))
	c.eglSurf = egl.NativeWindowType(win)
	return nil
}

func (c *ohosContext) Lock() error {
	if c.eglSurf != 0 {
		if err := c.Context.CreateSurface(c.eglSurf); err != nil {
			return err
		}
		c.eglSurf = 0
	}
	return c.Context.MakeCurrent()
}

func (c *ohosContext) Unlock() {
	c.Context.ReleaseCurrent()
}
