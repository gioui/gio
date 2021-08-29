// SPDX-License-Identifier: Unlicense OR MIT

//go:build darwin && !ios && nometal
// +build darwin,!ios,nometal

package app

import (
	"errors"

	"gioui.org/gpu"
	"gioui.org/internal/gl"
)

/*
#include <CoreFoundation/CoreFoundation.h>
#include <CoreGraphics/CoreGraphics.h>
#include <AppKit/AppKit.h>

__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createGLContext(void);
__attribute__ ((visibility ("hidden"))) void gio_setContextView(CFTypeRef ctx, CFTypeRef view);
__attribute__ ((visibility ("hidden"))) void gio_makeCurrentContext(CFTypeRef ctx);
__attribute__ ((visibility ("hidden"))) void gio_updateContext(CFTypeRef ctx);
__attribute__ ((visibility ("hidden"))) void gio_flushContextBuffer(CFTypeRef ctx);
__attribute__ ((visibility ("hidden"))) void gio_clearCurrentContext(void);
__attribute__ ((visibility ("hidden"))) void gio_lockContext(CFTypeRef ctxRef);
__attribute__ ((visibility ("hidden"))) void gio_unlockContext(CFTypeRef ctxRef);
*/
import "C"

type glContext struct {
	c    *gl.Functions
	ctx  C.CFTypeRef
	view C.CFTypeRef
}

func newContext(w *window) (*glContext, error) {
	view := w.contextView()
	ctx := C.gio_createGLContext()
	if ctx == 0 {
		return nil, errors.New("gl: failed to create NSOpenGLContext")
	}
	C.gio_setContextView(ctx, view)
	c := &glContext{
		ctx:  ctx,
		view: view,
	}
	return c, nil
}

func (c *glContext) RenderTarget() gpu.RenderTarget {
	return gpu.OpenGLRenderTarget{}
}

func (c *glContext) API() gpu.API {
	return gpu.OpenGL{}
}

func (c *glContext) Release() {
	if c.ctx != 0 {
		C.gio_clearCurrentContext()
		C.CFRelease(c.ctx)
		c.ctx = 0
	}
}

func (c *glContext) Present() error {
	return nil
}

func (c *glContext) Lock() error {
	C.gio_lockContext(c.ctx)
	C.gio_makeCurrentContext(c.ctx)
	return nil
}

func (c *glContext) Unlock() {
	C.gio_clearCurrentContext()
	C.gio_unlockContext(c.ctx)
}

func (c *glContext) Refresh() error {
	c.Lock()
	defer c.Unlock()
	C.gio_updateContext(c.ctx)
	return nil
}

func (w *window) NewContext() (context, error) {
	return newContext(w)
}
