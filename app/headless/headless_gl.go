// SPDX-License-Identifier: Unlicense OR MIT

package headless

func newContext() (context, error) {
	return newGLContext()
}
