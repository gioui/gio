// SPDX-License-Identifier: Unlicense OR MIT

// +build linux freebsd windows openbsd

package headless

import (
	"gioui.org/app/internal/egl"
)

func newGLContext() (context, error) {
	return egl.NewContext(egl.EGL_DEFAULT_DISPLAY)
}
