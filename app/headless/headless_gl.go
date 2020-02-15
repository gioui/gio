// SPDX-License-Identifier: Unlicense OR MIT

// +build !windows

package headless

func newContext() (context, error) {
	return newGLContext()
}
