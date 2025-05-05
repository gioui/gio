// SPDX-License-Identifier: Unlicense OR MIT

package egl

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	syscall "golang.org/x/sys/windows"
)

type (
	_EGLint           int32
	_EGLDisplay       uintptr
	_EGLConfig        uintptr
	_EGLContext       uintptr
	_EGLSurface       uintptr
	NativeDisplayType uintptr
	NativeWindowType  uintptr
)

var (
	libEGL                  = syscall.DLL{}
	_eglChooseConfig        *syscall.Proc
	_eglCreateContext       *syscall.Proc
	_eglCreateWindowSurface *syscall.Proc
	_eglDestroyContext      *syscall.Proc
	_eglDestroySurface      *syscall.Proc
	_eglGetConfigAttrib     *syscall.Proc
	_eglGetDisplay          *syscall.Proc
	_eglGetError            *syscall.Proc
	_eglInitialize          *syscall.Proc
	_eglMakeCurrent         *syscall.Proc
	_eglReleaseThread       *syscall.Proc
	_eglSwapInterval        *syscall.Proc
	_eglSwapBuffers         *syscall.Proc
	_eglTerminate           *syscall.Proc
	_eglQueryString         *syscall.Proc
	_eglWaitClient          *syscall.Proc
)

var loadOnce sync.Once

func loadEGL() error {
	var err error
	loadOnce.Do(func() {
		err = loadDLLs()
	})
	return err
}

func loadDLLs() error {
	if err := loadDLL(&libEGL, "libEGL.dll"); err != nil {
		return err
	}

	procs := map[string]**syscall.Proc{
		"eglChooseConfig":        &_eglChooseConfig,
		"eglCreateContext":       &_eglCreateContext,
		"eglCreateWindowSurface": &_eglCreateWindowSurface,
		"eglDestroyContext":      &_eglDestroyContext,
		"eglDestroySurface":      &_eglDestroySurface,
		"eglGetConfigAttrib":     &_eglGetConfigAttrib,
		"eglGetDisplay":          &_eglGetDisplay,
		"eglGetError":            &_eglGetError,
		"eglInitialize":          &_eglInitialize,
		"eglMakeCurrent":         &_eglMakeCurrent,
		"eglReleaseThread":       &_eglReleaseThread,
		"eglSwapInterval":        &_eglSwapInterval,
		"eglSwapBuffers":         &_eglSwapBuffers,
		"eglTerminate":           &_eglTerminate,
		"eglQueryString":         &_eglQueryString,
		"eglWaitClient":          &_eglWaitClient,
	}
	for name, proc := range procs {
		p, err := libEGL.FindProc(name)
		if err != nil {
			return fmt.Errorf("failed to locate %s in %s: %w", name, libEGL.Name, err)
		}
		*proc = p
	}
	return nil
}

func loadDLL(dll *syscall.DLL, name string) error {
	handle, err := syscall.LoadLibraryEx(name, 0, syscall.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	if err != nil {
		return fmt.Errorf("egl: failed to load %s: %v", name, err)
	}
	dll.Handle = handle
	dll.Name = name
	return nil
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

func eglCreateWindowSurface(disp _EGLDisplay, cfg _EGLConfig, win NativeWindowType, attribs []_EGLint) _EGLSurface {
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

func eglGetDisplay(disp NativeDisplayType) _EGLDisplay {
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
	return syscall.BytePtrToString((*byte)(unsafe.Pointer(r)))
}

func eglWaitClient() bool {
	r, _, _ := _eglWaitClient.Call()
	return r != 0
}

// issue34474KeepAlive calls runtime.KeepAlive as a
// workaround for golang.org/issue/34474.
func issue34474KeepAlive(v any) {
	runtime.KeepAlive(v)
}
