// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,ios

package window

/*
#include <CoreFoundation/CoreFoundation.h>
#include <OpenGLES/ES2/gl.h>
#include <OpenGLES/ES2/glext.h>

__attribute__ ((visibility ("hidden"))) int gio_renderbufferStorage(CFTypeRef ctx, CFTypeRef layer, GLenum buffer);
__attribute__ ((visibility ("hidden"))) int gio_presentRenderbuffer(CFTypeRef ctx, GLenum buffer);
__attribute__ ((visibility ("hidden"))) int gio_makeCurrent(CFTypeRef ctx);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createContext(void);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createGLLayer(void);
*/
import "C"

import (
	"errors"
	"fmt"

	"gioui.org/gpu/backend"
	"gioui.org/gpu/gl"
	"gioui.org/internal/glimpl"
)

type context struct {
	owner                    *window
	c                        *glimpl.Functions
	ctx                      C.CFTypeRef
	layer                    C.CFTypeRef
	init                     bool
	frameBuffer              glimpl.Framebuffer
	colorBuffer, depthBuffer glimpl.Renderbuffer
}

func init() {
	layerFactory = func() uintptr {
		return uintptr(C.gio_createGLLayer())
	}
}

func newContext(w *window) (*context, error) {
	ctx := C.gio_createContext()
	if ctx == 0 {
		return nil, fmt.Errorf("failed to create EAGLContext")
	}
	c := &context{
		ctx:   ctx,
		owner: w,
		layer: C.CFTypeRef(w.contextLayer()),
		c:     new(glimpl.Functions),
	}
	return c, nil
}

func (c *context) Backend() (backend.Device, error) {
	return gl.NewBackend(c.c)
}

func (c *context) Release() {
	if c.ctx == 0 {
		return
	}
	C.gio_renderbufferStorage(c.ctx, 0, C.GLenum(glimpl.RENDERBUFFER))
	c.c.DeleteFramebuffer(c.frameBuffer)
	c.c.DeleteRenderbuffer(c.colorBuffer)
	c.c.DeleteRenderbuffer(c.depthBuffer)
	C.gio_makeCurrent(0)
	C.CFRelease(c.ctx)
	c.ctx = 0
}

func (c *context) Present() error {
	if c.layer == 0 {
		panic("context is not active")
	}
	// Discard depth buffer as recommended in
	// https://developer.apple.com/library/archive/documentation/3DDrawing/Conceptual/OpenGLES_ProgrammingGuide/WorkingwithEAGLContexts/WorkingwithEAGLContexts.html
	c.c.BindFramebuffer(glimpl.FRAMEBUFFER, c.frameBuffer)
	c.c.InvalidateFramebuffer(glimpl.FRAMEBUFFER, glimpl.DEPTH_ATTACHMENT)
	c.c.BindRenderbuffer(glimpl.RENDERBUFFER, c.colorBuffer)
	if C.gio_presentRenderbuffer(c.ctx, C.GLenum(glimpl.RENDERBUFFER)) == 0 {
		return errors.New("presentRenderBuffer failed")
	}
	return nil
}

func (c *context) Lock() {}

func (c *context) Unlock() {}

func (c *context) MakeCurrent() error {
	if C.gio_makeCurrent(c.ctx) == 0 {
		C.CFRelease(c.ctx)
		c.ctx = 0
		return errors.New("[EAGLContext setCurrentContext] failed")
	}
	if !c.init {
		c.init = true
		c.frameBuffer = c.c.CreateFramebuffer()
		c.colorBuffer = c.c.CreateRenderbuffer()
		c.depthBuffer = c.c.CreateRenderbuffer()
	}
	if !c.owner.isVisible() {
		// Make sure any in-flight GL commands are complete.
		c.c.Finish()
		return nil
	}
	currentRB := glimpl.Renderbuffer{uint(c.c.GetInteger(glimpl.RENDERBUFFER_BINDING))}
	c.c.BindRenderbuffer(glimpl.RENDERBUFFER, c.colorBuffer)
	if C.gio_renderbufferStorage(c.ctx, c.layer, C.GLenum(glimpl.RENDERBUFFER)) == 0 {
		return errors.New("renderbufferStorage failed")
	}
	w := c.c.GetRenderbufferParameteri(glimpl.RENDERBUFFER, glimpl.RENDERBUFFER_WIDTH)
	h := c.c.GetRenderbufferParameteri(glimpl.RENDERBUFFER, glimpl.RENDERBUFFER_HEIGHT)
	c.c.BindRenderbuffer(glimpl.RENDERBUFFER, c.depthBuffer)
	c.c.RenderbufferStorage(glimpl.RENDERBUFFER, glimpl.DEPTH_COMPONENT16, w, h)
	c.c.BindRenderbuffer(glimpl.RENDERBUFFER, currentRB)
	c.c.BindFramebuffer(glimpl.FRAMEBUFFER, c.frameBuffer)
	c.c.FramebufferRenderbuffer(glimpl.FRAMEBUFFER, glimpl.COLOR_ATTACHMENT0, glimpl.RENDERBUFFER, c.colorBuffer)
	c.c.FramebufferRenderbuffer(glimpl.FRAMEBUFFER, glimpl.DEPTH_ATTACHMENT, glimpl.RENDERBUFFER, c.depthBuffer)
	if st := c.c.CheckFramebufferStatus(glimpl.FRAMEBUFFER); st != glimpl.FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("framebuffer incomplete, status: %#x\n", st)
	}
	return nil
}

func (w *window) NewContext() (Context, error) {
	return newContext(w)
}
