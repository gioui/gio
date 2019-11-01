// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"os"
	"unsafe"

	syscall "golang.org/x/sys/windows"

	"gioui.org/app/internal/gl"
)

type (
	_EGLint               int32
	_EGLDisplay           uintptr
	_EGLConfig            uintptr
	_EGLContext           uintptr
	_EGLSurface           uintptr
	_EGLNativeDisplayType uintptr
	_EGLNativeWindowType  uintptr
)

var (
	libEGL                  = syscall.NewLazyDLL("libEGL.dll")
	_eglChooseConfig        = libEGL.NewProc("eglChooseConfig")
	_eglCreateContext       = libEGL.NewProc("eglCreateContext")
	_eglCreateWindowSurface = libEGL.NewProc("eglCreateWindowSurface")
	_eglDestroyContext      = libEGL.NewProc("eglDestroyContext")
	_eglDestroySurface      = libEGL.NewProc("eglDestroySurface")
	_eglGetConfigAttrib     = libEGL.NewProc("eglGetConfigAttrib")
	_eglGetDisplay          = libEGL.NewProc("eglGetDisplay")
	_eglGetError            = libEGL.NewProc("eglGetError")
	_eglInitialize          = libEGL.NewProc("eglInitialize")
	_eglMakeCurrent         = libEGL.NewProc("eglMakeCurrent")
	_eglReleaseThread       = libEGL.NewProc("eglReleaseThread")
	_eglSwapInterval        = libEGL.NewProc("eglSwapInterval")
	_eglSwapBuffers         = libEGL.NewProc("eglSwapBuffers")
	_eglTerminate           = libEGL.NewProc("eglTerminate")
	_eglQueryString         = libEGL.NewProc("eglQueryString")
)

func init() {
	mustLoadDLL(libEGL, "libEGL.dll")
	mustLoadDLL(gl.LibGLESv2, "libGLESv2.dll")
	// d3dcompiler_47.dll is needed internally for shader compilation to function.
	mustLoadDLL(syscall.NewLazyDLL("d3dcompiler_47.dll"), "d3dcompiler_47.dll")
}

func mustLoadDLL(dll *syscall.LazyDLL, name string) {
	loadErr := dll.Load()
	if loadErr == nil {
		return
	}
	pmsg := syscall.StringToUTF16Ptr("Failed to load " + name + ". Gio requires the ANGLE OpenGL ES driver to run. A prebuilt version can be downloaded from https://gioui.org/doc/install.")
	ptitle := syscall.StringToUTF16Ptr("Error")
	syscall.MessageBox(0 /* HWND */, pmsg, ptitle, syscall.MB_ICONERROR|syscall.MB_SYSTEMMODAL)
	os.Exit(1)
}

func eglChooseConfig(disp _EGLDisplay, attribs []_EGLint) (_EGLConfig, bool) {
	var cfg _EGLConfig
	var ncfg _EGLint
	a := &attribs[0]
	r, _, _ := _eglChooseConfig.Call(uintptr(disp), uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(&cfg)), 1, uintptr(unsafe.Pointer(&ncfg)))
	issue34474KeepAlive(a)
	return cfg, r != 0
}

func eglCreateContext(disp _EGLDisplay, cfg _EGLConfig, shareCtx _EGLContext, attribs []_EGLint) _EGLContext {
	a := &attribs[0]
	c, _, _ := _eglCreateContext.Call(uintptr(disp), uintptr(cfg), uintptr(shareCtx), uintptr(unsafe.Pointer(a)))
	issue34474KeepAlive(a)
	return _EGLContext(c)
}

func eglCreateWindowSurface(disp _EGLDisplay, cfg _EGLConfig, win _EGLNativeWindowType, attribs []_EGLint) _EGLSurface {
	a := &attribs[0]
	s, _, _ := _eglCreateWindowSurface.Call(uintptr(disp), uintptr(cfg), uintptr(win), uintptr(unsafe.Pointer(a)))
	issue34474KeepAlive(a)
	return _EGLSurface(s)
}

func eglDestroySurface(disp _EGLDisplay, surf _EGLSurface) bool {
	r, _, _ := _eglDestroySurface.Call(uintptr(disp), uintptr(surf))
	return r != 0
}

func eglDestroyContext(disp _EGLDisplay, ctx _EGLContext) bool {
	r, _, _ := _eglDestroyContext.Call(uintptr(disp), uintptr(ctx))
	return r != 0
}

func eglGetConfigAttrib(disp _EGLDisplay, cfg _EGLConfig, attr _EGLint) (_EGLint, bool) {
	var val uintptr
	r, _, _ := _eglGetConfigAttrib.Call(uintptr(disp), uintptr(cfg), uintptr(attr), uintptr(unsafe.Pointer(&val)))
	return _EGLint(val), r != 0
}

func eglGetDisplay(disp _EGLNativeDisplayType) _EGLDisplay {
	d, _, _ := _eglGetDisplay.Call(uintptr(disp))
	return _EGLDisplay(d)
}

func eglGetError() _EGLint {
	e, _, _ := _eglGetError.Call()
	return _EGLint(e)
}

func eglInitialize(disp _EGLDisplay) (_EGLint, _EGLint, bool) {
	var maj, min uintptr
	r, _, _ := _eglInitialize.Call(uintptr(disp), uintptr(unsafe.Pointer(&maj)), uintptr(unsafe.Pointer(&min)))
	return _EGLint(maj), _EGLint(min), r != 0
}

func eglMakeCurrent(disp _EGLDisplay, draw, read _EGLSurface, ctx _EGLContext) bool {
	r, _, _ := _eglMakeCurrent.Call(uintptr(disp), uintptr(draw), uintptr(read), uintptr(ctx))
	return r != 0
}

func eglReleaseThread() bool {
	r, _, _ := _eglReleaseThread.Call()
	return r != 0
}

func eglSwapInterval(disp _EGLDisplay, interval _EGLint) bool {
	r, _, _ := _eglSwapInterval.Call(uintptr(disp), uintptr(interval))
	return r != 0
}

func eglSwapBuffers(disp _EGLDisplay, surf _EGLSurface) bool {
	r, _, _ := _eglSwapBuffers.Call(uintptr(disp), uintptr(surf))
	return r != 0
}

func eglTerminate(disp _EGLDisplay) bool {
	r, _, _ := _eglTerminate.Call(uintptr(disp))
	return r != 0
}

func eglQueryString(disp _EGLDisplay, name _EGLint) string {
	r, _, _ := _eglQueryString.Call(uintptr(disp), uintptr(name))
	return gl.GoString(gl.SliceOf(r))
}

func (w *window) eglDestroy() {
}

func (w *window) eglDisplay() _EGLNativeDisplayType {
	return _EGLNativeDisplayType(w.HDC())
}

func (w *window) eglWindow(visID int) (_EGLNativeWindowType, int, int, error) {
	hwnd, width, height := w.HWND()
	return _EGLNativeWindowType(hwnd), width, height, nil
}

func (w *window) NewContext() (gl.Context, error) {
	return newContext(w)
}

func (w *window) needVSync() bool { return true }
