// SPDX-License-Identifier: Unlicense OR MIT

// +build linux windows freebsd

package egl

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	"gioui.org/app/internal/gl"
)

type Context struct {
	c             *gl.Functions
	driver        Driver
	eglCtx        *eglContext
	eglWin        NativeWindowType
	eglSurf       _EGLSurface
	width, height int
	// For sRGB emulation.
	srgbFBO *gl.SRGBFBO
}

type Driver interface {
	EGLDisplay() NativeDisplayType
	EGLDestroy()
}

type WindowDriver interface {
	EGLWindow(visID int) (NativeWindowType, int, int, error)
	NeedVSync() bool
}

type eglContext struct {
	disp        _EGLDisplay
	config      _EGLConfig
	ctx         _EGLContext
	visualID    int
	srgb        bool
	surfaceless bool
}

var (
	nilEGLDisplay          _EGLDisplay
	nilEGLSurface          _EGLSurface
	nilEGLContext          _EGLContext
	nilEGLConfig           _EGLConfig
	nilEGLNativeWindowType NativeWindowType
)

const (
	_EGL_ALPHA_SIZE             = 0x3021
	_EGL_BLUE_SIZE              = 0x3022
	_EGL_CONFIG_CAVEAT          = 0x3027
	_EGL_CONTEXT_CLIENT_VERSION = 0x3098
	_EGL_DEPTH_SIZE             = 0x3025
	_EGL_GL_COLORSPACE_KHR      = 0x309d
	_EGL_GL_COLORSPACE_SRGB_KHR = 0x3089
	_EGL_GREEN_SIZE             = 0x3023
	_EGL_EXTENSIONS             = 0x3055
	_EGL_NATIVE_VISUAL_ID       = 0x302e
	_EGL_NONE                   = 0x3038
	_EGL_OPENGL_ES2_BIT         = 0x4
	_EGL_RED_SIZE               = 0x3024
	_EGL_RENDERABLE_TYPE        = 0x3040
	_EGL_SURFACE_TYPE           = 0x3033
	_EGL_WINDOW_BIT             = 0x4
)

func (c *Context) Release() {
	if c.srgbFBO != nil {
		c.srgbFBO.Release()
		c.srgbFBO = nil
	}
	c.destroySurface()
	c.eglWin = nilEGLNativeWindowType
	if c.eglCtx != nil {
		eglDestroyContext(c.eglCtx.disp, c.eglCtx.ctx)
		eglTerminate(c.eglCtx.disp)
		eglReleaseThread()
		c.eglCtx = nil
	}
	if c.driver != nil {
		c.driver.EGLDestroy()
		c.driver = nil
	}
}

func (c *Context) Present() error {
	if c.eglWin == nilEGLNativeWindowType {
		panic("context is not active")
	}
	if c.srgbFBO != nil {
		c.srgbFBO.Blit()
	}
	if !eglSwapBuffers(c.eglCtx.disp, c.eglSurf) {
		return fmt.Errorf("eglSwapBuffers failed (%x)", eglGetError())
	}
	if c.srgbFBO != nil {
		c.srgbFBO.AfterPresent()
	}
	return nil
}

func NewContext(d Driver) (*Context, error) {
	eglCtx, err := createContext(d.EGLDisplay())
	if err != nil {
		return nil, err
	}
	if _, windowed := d.(WindowDriver); !windowed && !eglCtx.surfaceless {
		eglDestroyContext(eglCtx.disp, eglCtx.ctx)
		return nil, errors.New("EGL_KHR_surfaceless_context not supported")
	}
	c := &Context{
		driver: d,
		eglCtx: eglCtx,
		c:      new(gl.Functions),
	}
	return c, nil
}

func (c *Context) Functions() *gl.Functions {
	return c.c
}

func (c *Context) Lock() {}

func (c *Context) Unlock() {}

func (c *Context) destroySurface() {
	if c.eglSurf == nilEGLSurface {
		return
	}
	// Make sure any in-flight GL commands are complete.
	c.c.Finish()
	eglMakeCurrent(c.eglCtx.disp, nilEGLSurface, nilEGLSurface, nilEGLContext)
	eglDestroySurface(c.eglCtx.disp, c.eglSurf)
	c.eglSurf = nilEGLSurface
}

func (c *Context) MakeCurrent() error {
	wdriver, ok := c.driver.(WindowDriver)
	if !ok {
		if !eglMakeCurrent(c.eglCtx.disp, nilEGLSurface, nilEGLSurface, c.eglCtx.ctx) {
			return fmt.Errorf("eglMakeCurrent error 0x%x", eglGetError())
		}
		return nil
	}
	win, width, height, err := wdriver.EGLWindow(int(c.eglCtx.visualID))
	if err != nil {
		return err
	}
	if c.eglWin == win && width == c.width && height == c.height {
		return nil
	}
	c.width, c.height = width, height
	if win == nilEGLNativeWindowType && c.srgbFBO != nil {
		c.srgbFBO.Release()
		c.srgbFBO = nil
	}
	c.destroySurface()
	c.eglWin = win
	if c.eglWin == nilEGLNativeWindowType {
		return nil
	}
	eglSurf, err := createSurface(c.eglCtx, win)
	c.eglSurf = eglSurf
	if err != nil {
		c.eglWin = nilEGLNativeWindowType
		return err
	}
	if !eglMakeCurrent(c.eglCtx.disp, eglSurf, eglSurf, c.eglCtx.ctx) {
		return fmt.Errorf("eglMakeCurrent error 0x%x", eglGetError())
	}
	// eglSwapInterval 1 leads to erratic frame rates and unnecessary blocking.
	// We rely on platform specific frame rate limiting instead, except on Windows
	// and X11 where eglSwapInterval is all there is.
	if wdriver.NeedVSync() {
		eglSwapInterval(c.eglCtx.disp, 1)
	} else {
		eglSwapInterval(c.eglCtx.disp, 0)
	}
	if c.eglCtx.srgb {
		return nil
	}
	if c.srgbFBO == nil {
		var err error
		c.srgbFBO, err = gl.NewSRGBFBO(c.c)
		if err != nil {
			c.Release()
			return err
		}
	}
	if err := c.srgbFBO.Refresh(c.width, c.height); err != nil {
		c.Release()
		return err
	}
	return nil
}

func hasExtension(exts []string, ext string) bool {
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func createContext(disp NativeDisplayType) (*eglContext, error) {
	eglDisp := eglGetDisplay(disp)
	if eglDisp == nilEGLDisplay {
		return nil, fmt.Errorf("eglGetDisplay(_EGL_DEFAULT_DISPLAY) failed: 0x%x", eglGetError())
	}
	major, minor, ret := eglInitialize(eglDisp)
	if !ret {
		return nil, fmt.Errorf("eglInitialize failed: 0x%x", eglGetError())
	}
	// sRGB framebuffer support on EGL 1.5 or if EGL_KHR_gl_colorspace is supported.
	exts := strings.Split(eglQueryString(eglDisp, _EGL_EXTENSIONS), " ")
	srgb := major > 1 || minor >= 5 || hasExtension(exts, "EGL_KHR_gl_colorspace")
	attribs := []_EGLint{
		_EGL_RENDERABLE_TYPE, _EGL_OPENGL_ES2_BIT,
		_EGL_SURFACE_TYPE, _EGL_WINDOW_BIT,
		_EGL_BLUE_SIZE, 8,
		_EGL_GREEN_SIZE, 8,
		_EGL_RED_SIZE, 8,
		_EGL_CONFIG_CAVEAT, _EGL_NONE,
	}
	if srgb {
		if runtime.GOOS == "linux" {
			// Some Mesa drivers crash if an sRGB framebuffer is requested without alpha.
			// https://bugs.freedesktop.org/show_bug.cgi?id=107782.
			attribs = append(attribs, _EGL_ALPHA_SIZE, 1)
		}
		// Only request a depth buffer if we're going to render directly to the framebuffer.
		attribs = append(attribs, _EGL_DEPTH_SIZE, 16)
	}
	attribs = append(attribs, _EGL_NONE)
	eglCfg, ret := eglChooseConfig(eglDisp, attribs)
	if !ret {
		return nil, fmt.Errorf("eglChooseConfig failed: 0x%x", eglGetError())
	}
	if eglCfg == nilEGLConfig {
		return nil, errors.New("eglChooseConfig returned 0 configs")
	}
	visID, ret := eglGetConfigAttrib(eglDisp, eglCfg, _EGL_NATIVE_VISUAL_ID)
	if !ret {
		return nil, errors.New("newContext: eglGetConfigAttrib for _EGL_NATIVE_VISUAL_ID failed")
	}
	ctxAttribs := []_EGLint{
		_EGL_CONTEXT_CLIENT_VERSION, 3,
		_EGL_NONE,
	}
	eglCtx := eglCreateContext(eglDisp, eglCfg, nilEGLContext, ctxAttribs)
	if eglCtx == nilEGLContext {
		// Fall back to OpenGL ES 2 and rely on extensions.
		ctxAttribs := []_EGLint{
			_EGL_CONTEXT_CLIENT_VERSION, 2,
			_EGL_NONE,
		}
		eglCtx = eglCreateContext(eglDisp, eglCfg, nilEGLContext, ctxAttribs)
		if eglCtx == nilEGLContext {
			return nil, fmt.Errorf("eglCreateContext failed: 0x%x", eglGetError())
		}
	}
	return &eglContext{
		disp:        eglDisp,
		config:      _EGLConfig(eglCfg),
		ctx:         _EGLContext(eglCtx),
		visualID:    int(visID),
		srgb:        srgb,
		surfaceless: hasExtension(exts, "EGL_KHR_surfaceless_context"),
	}, nil
}

func createSurface(eglCtx *eglContext, win NativeWindowType) (_EGLSurface, error) {
	var surfAttribs []_EGLint
	if eglCtx.srgb {
		surfAttribs = append(surfAttribs, _EGL_GL_COLORSPACE_KHR, _EGL_GL_COLORSPACE_SRGB_KHR)
	}
	surfAttribs = append(surfAttribs, _EGL_NONE)
	eglSurf := eglCreateWindowSurface(eglCtx.disp, eglCtx.config, win, surfAttribs)
	if eglSurf == nilEGLSurface && eglCtx.srgb {
		// Try again without sRGB
		eglCtx.srgb = false
		surfAttribs = []_EGLint{_EGL_NONE}
		eglSurf = eglCreateWindowSurface(eglCtx.disp, eglCtx.config, win, surfAttribs)
	}
	if eglSurf == nilEGLSurface {
		return nilEGLSurface, fmt.Errorf("newContext: eglCreateWindowSurface failed 0x%x (sRGB=%v)", eglGetError(), eglCtx.srgb)
	}
	return eglSurf, nil
}
