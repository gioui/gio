// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"gioui.org/gpu"
	_ "gioui.org/internal/cocoainit"
)

/*
#cgo CFLAGS: -DGL_SILENCE_DEPRECATION -Werror -Wno-deprecated-declarations -fmodules -fobjc-arc -x objective-c

#include <CoreFoundation/CoreFoundation.h>

__attribute__ ((visibility ("hidden"))) CFTypeRef gio_headless_newContext(void);
__attribute__ ((visibility ("hidden"))) void gio_headless_clearCurrentContext(CFTypeRef ctxRef);
__attribute__ ((visibility ("hidden"))) void gio_headless_makeCurrentContext(CFTypeRef ctxRef);
*/
import "C"

type nsContext struct {
	ctx C.CFTypeRef
}

func newContext() (context, error) {
	ctx := C.gio_headless_newContext()
	return &nsContext{ctx: ctx}, nil
}

func (c *nsContext) API() gpu.API {
	return gpu.OpenGL{}
}

func (c *nsContext) MakeCurrent() error {
	C.gio_headless_makeCurrentContext(c.ctx)
	return nil
}

func (c *nsContext) ReleaseCurrent() {
	C.gio_headless_clearCurrentContext(c.ctx)
}

func (d *nsContext) Release() {
	if d.ctx != 0 {
		C.CFRelease(d.ctx)
		d.ctx = 0
	}
}
