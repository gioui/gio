// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

package window

import (
	"gioui.org/app/internal/gl"
)

/*
#include <CoreFoundation/CoreFoundation.h>
#include <CoreGraphics/CoreGraphics.h>
#include <AppKit/AppKit.h>
#include <OpenGL/gl3.h>
#include "gl_macos.h"
*/
import "C"

type context struct {
	c    *gl.Functions
	ctx  C.CFTypeRef
	view C.CFTypeRef
}

func init() {
	viewFactory = func() C.CFTypeRef {
		return C.gio_createGLView()
	}
}

func newContext(w *window) (*context, error) {
	view := w.contextView()
	ctx := C.gio_contextForView(view)
	c := &context{
		ctx:  ctx,
		c:    new(gl.Functions),
		view: view,
	}
	return c, nil
}

func (c *context) Functions() *gl.Functions {
	return c.c
}

func (c *context) Release() {
	c.Lock()
	defer c.Unlock()
	C.gio_clearCurrentContext()
}

func (c *context) Present() error {
	// Assume the caller already locked the context.
	C.glFlush()
	return nil
}

func (c *context) Lock() {
	C.gio_lockContext(c.ctx)
}

func (c *context) Unlock() {
	C.gio_unlockContext(c.ctx)
}

func (c *context) MakeCurrent() error {
	c.Lock()
	defer c.Unlock()
	C.gio_makeCurrentContext(c.ctx)
	return nil
}

func (w *window) NewContext() (Context, error) {
	return newContext(w)
}
