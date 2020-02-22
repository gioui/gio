// SPDX-License-Identifier: Unlicense OR MIT

package headless

func newContext(width, height int) (backend, error) {
	return newGLContext()
}
