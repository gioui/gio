// SPDX-License-Identifier: Unlicense OR MIT

package app

/*
#include <EGL/egl.h>
*/
import "C"

type (
	_EGLNativeDisplayType = C.EGLNativeDisplayType
	_EGLNativeWindowType  = C.EGLNativeWindowType
)

func eglGetDisplay(disp _EGLNativeDisplayType) _EGLDisplay {
	return C.eglGetDisplay(disp)
}

func eglCreateWindowSurface(disp _EGLDisplay, conf _EGLConfig, win _EGLNativeWindowType, attribs []_EGLint) _EGLSurface {
	eglSurf := C.eglCreateWindowSurface(disp, conf, win, &attribs[0])
	return eglSurf
}
