// SPDX-License-Identifier: Unlicense OR MIT

//go:build ((linux && !android) || freebsd || openbsd) && !nox11 && !noopengl
// +build linux,!android freebsd openbsd
// +build !nox11
// +build !noopengl

package app

import (
	"unsafe"

	"gioui.org/internal/egl"
)

type x11Context struct {
	win *x11Window
	*egl.Context
}

func init() {
	newX11EGLContext = func(w *x11Window) (context, error) {
		disp := egl.NativeDisplayType(unsafe.Pointer(w.display()))
		ctx, err := egl.NewContext(disp)
		if err != nil {
			return nil, err
		}
		win, _, _ := w.window()
		eglSurf := egl.NativeWindowType(uintptr(win))
		if err := ctx.CreateSurface(eglSurf); err != nil {
			ctx.Release()
			return nil, err
		}
		if err := ctx.MakeCurrent(); err != nil {
			ctx.Release()
			return nil, err
		}
		defer ctx.ReleaseCurrent()
		ctx.EnableVSync(true)
		return &x11Context{win: w, Context: ctx}, nil
	}
}

func (c *x11Context) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
}

func (c *x11Context) Refresh() error {
	return nil
}

func (c *x11Context) Lock() error {
	return c.Context.MakeCurrent()
}

func (c *x11Context) Unlock() {
	c.Context.ReleaseCurrent()
}
