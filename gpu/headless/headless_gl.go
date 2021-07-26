// SPDX-License-Identifier: Unlicense OR MIT

//go:build !windows
// +build !windows

package headless

func newContext() (context, error) {
	return newGLContext()
}
