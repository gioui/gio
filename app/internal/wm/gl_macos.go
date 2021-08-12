// SPDX-License-Identifier: Unlicense OR MIT

//go:build darwin && !ios
// +build darwin,!ios

package wm

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

type context struct {
	c    *gl.Functions
	ctx  C.CFTypeRef
	view C.CFTypeRef
}

func newContext(w *window) (*context, error) {
	view := w.contextView()
	ctx := C.gio_createGLContext()
	if ctx == 0 {
		return nil, errors.New("gl: failed to create NSOpenGLContext")
	}
	C.gio_setContextView(ctx, view)
	c := &context{
		ctx:  ctx,
		view: view,
	}
	return c, nil
}

func (c *context) RenderTarget() gpu.RenderTarget {
	return gpu.OpenGLRenderTarget{}
}

func (c *context) API() gpu.API {
	return gpu.OpenGL{}
}

func (c *context) Release() {
	if c.ctx != 0 {
		C.gio_clearCurrentContext()
		C.CFRelease(c.ctx)
		c.ctx = 0
	}
}

func (c *context) Present() error {
	return nil
}

func (c *context) Lock() error {
	C.gio_lockContext(c.ctx)
	C.gio_makeCurrentContext(c.ctx)
	return nil
}

func (c *context) Unlock() {
	C.gio_clearCurrentContext()
	C.gio_unlockContext(c.ctx)
}

func (c *context) Refresh() error {
	c.Lock()
	defer c.Unlock()
	C.gio_updateContext(c.ctx)
	return nil
}

func (w *window) NewContext() (Context, error) {
	return newContext(w)
}
