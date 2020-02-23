// SPDX-License-Identifier: Unlicense OR MIT

package headless

func newContext() (backend, error) {
	return newGLContext()
}
