// SPDX-License-Identifier: Unlicense OR MIT

//go:build linux || freebsd || openbsd
// +build linux freebsd openbsd

package headless

import (
	"gioui.org/internal/egl"
)

func newContext() (context, error) {
	return egl.NewContext(egl.EGL_DEFAULT_DISPLAY)
}
