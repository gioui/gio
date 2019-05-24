// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

package app

import (
	"gioui.org/ui/app/internal/gl"
)

/*
#cgo CFLAGS: -DGL_SILENCE_DEPRECATION -fmodules -fobjc-arc -x objective-c

#include <CoreFoundation/CoreFoundation.h>
#include <CoreGraphics/CoreGraphics.h>
#include <AppKit/AppKit.h>
#include <OpenGL/gl3.h>
#include "gl_macos.h"
*/
import "C"

type context struct {
	c   *gl.Functions
	ctx C.CFTypeRef
}

func init() {
	viewFactory = func() uintptr {
		return uintptr(C.gio_createGLView())
	}
}

func newContext(w *window) (*context, error) {
	ctx := C.gio_contextForView(w.contextView())
	c := &context{
		ctx: ctx,
		c:   new(gl.Functions),
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
	C.CFRelease(c.ctx)
	c.ctx = 0
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
