// SPDX-License-Identifier: Unlicense OR MIT

//go:build ((linux && !android) || freebsd) && !nowayland && !noopengl
// +build linux,!android freebsd
// +build !nowayland
// +build !noopengl

package app

import (
	"errors"
	"unsafe"

	"gioui.org/internal/egl"
)

/*
#cgo linux pkg-config: egl wayland-egl
#cgo freebsd openbsd LDFLAGS: -lwayland-egl
#cgo CFLAGS: -DEGL_NO_X11

#include <EGL/egl.h>
#include <wayland-client.h>
#include <wayland-egl.h>
*/
import "C"

type wlContext struct {
	win *window
	*egl.Context
	eglWin *C.struct_wl_egl_window
}

func init() {
	newWaylandEGLContext = func(w *window) (context, error) {
		disp := egl.NativeDisplayType(unsafe.Pointer(w.display()))
		ctx, err := egl.NewContext(disp)
		if err != nil {
			return nil, err
		}

		surf, width, height := w.surface()
		if surf == nil {
			return nil, errors.New("wayland: no surface")
		}
		eglWin := C.wl_egl_window_create(surf, C.int(width), C.int(height))
		if eglWin == nil {
			return nil, errors.New("wayland: wl_egl_window_create failed")
		}
		eglSurf := egl.NativeWindowType(uintptr(unsafe.Pointer(eglWin)))
		if err := ctx.CreateSurface(eglSurf); err != nil {
			return nil, err
		}
		// We're in charge of the frame callbacks, don't let eglSwapBuffers
		// wait for callbacks that may never arrive.
		ctx.EnableVSync(false)

		return &wlContext{Context: ctx, win: w, eglWin: eglWin}, nil
	}
}

func (c *wlContext) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
	if c.eglWin != nil {
		C.wl_egl_window_destroy(c.eglWin)
		c.eglWin = nil
	}
}

func (c *wlContext) Refresh() error {
	surf, width, height := c.win.surface()
	if surf == nil {
		return errors.New("wayland: no surface")
	}
	C.wl_egl_window_resize(c.eglWin, C.int(width), C.int(height), 0, 0)
	return nil
}

func (c *wlContext) Lock() error {
	return c.Context.MakeCurrent()
}

func (c *wlContext) Unlock() {
	c.Context.ReleaseCurrent()
}
