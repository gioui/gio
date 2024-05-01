// SPDX-License-Identifier: Unlicense OR MIT

//go:build !noopengl

package app

import (
	"gioui.org/internal/egl"
)

type glContext struct {
	win *window
	*egl.Context
}

func init() {
	drivers = append(drivers, gpuAPI{
		priority: 2,
		initializer: func(w *window) (context, error) {
			disp := egl.NativeDisplayType(w.HDC())
			ctx, err := egl.NewContext(disp)
			if err != nil {
				return nil, err
			}
			win, _, _ := w.HWND()
			eglSurf := egl.NativeWindowType(win)
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
			return &glContext{win: w, Context: ctx}, nil
		},
	})
}

func (c *glContext) Release() {
	if c.Context != nil {
		c.Context.Release()
		c.Context = nil
	}
}

func (c *glContext) Refresh() error {
	return nil
}

func (c *glContext) Lock() error {
	return c.Context.MakeCurrent()
}

func (c *glContext) Unlock() {
	c.Context.ReleaseCurrent()
}
