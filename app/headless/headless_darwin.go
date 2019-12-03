// SPDX-License-Identifier: Unlicense OR MIT

package headless

import "gioui.org/app/internal/gl"

/*
#cgo CFLAGS: -DGL_SILENCE_DEPRECATION -Werror -Wno-deprecated-declarations -fmodules -fobjc-arc -x objective-c

#include <CoreFoundation/CoreFoundation.h>
#include "headless_darwin.h"
*/
import "C"

type nsContext struct {
	c        *gl.Functions
	ctx      C.CFTypeRef
	prepared bool
}

func newContext() (context, error) {
	ctx := C.gio_headless_newContext()
	return &nsContext{ctx: ctx, c: new(gl.Functions)}, nil
}

func (c *nsContext) MakeCurrent() error {
	C.gio_headless_makeCurrentContext(c.ctx)
	if !c.prepared {
		C.gio_headless_prepareContext(c.ctx)
		c.prepared = true
	}
	return nil
}

func (c *nsContext) ReleaseCurrent() {
	C.gio_headless_clearCurrentContext(c.ctx)
}

func (c *nsContext) Functions() *gl.Functions {
	return c.c
}

func (d *nsContext) Release() {
	if d.ctx != 0 {
		C.gio_headless_releaseContext(d.ctx)
		d.ctx = 0
	}
}
