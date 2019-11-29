// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"unsafe"

	"gioui.org/app/internal/egl"
)

type context struct {
	win *window
	*egl.Context
}

func (w *window) NewContext() (Context, error) {
	disp := egl.NativeDisplayType(unsafe.Pointer(w.HDC()))
	ctx, err := egl.NewContext(disp)
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

func (c *context) MakeCurrent() error {
	c.Context.ReleaseSurface()
	win, width, height := c.win.HWND()
	eglSurf := egl.NativeWindowType(win)
	if err := c.Context.CreateSurface(eglSurf, width, height); err != nil {
		return err
	}
	if err := c.Context.MakeCurrent(); err != nil {
		return err
	}
	c.Context.EnableVSync(true)
	return nil
}

func (c *context) Lock() {}

func (c *context) Unlock() {}
