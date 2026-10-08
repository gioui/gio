// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"errors"

	"gioui.org/gpu"
)

// ErrNoGPUYet is returned by D3D11 before the window has drawn its first
// frame, which is also when Gio creates the device. It is not a failure: a
// caller that asks too early should ask again later.
var ErrNoGPUYet = errors.New("app: window has no GPU yet")

// D3D11 returns a handle for issuing custom Direct3D 11 passes against this
// window, or an error if its backend is not Direct3D 11.
//
// Passes draw on the same device and immediate context Gio uses, so they must
// be issued from the goroutine that calls Frame, and before it, to be ordered
// with Gio's own drawing. Textures created this way can be painted with
// paint.NewImageOpFromTexture without copying them to system memory.
func (w *Window) D3D11() (*gpu.D3D11, error) {
	if w.gpu == nil {
		return nil, ErrNoGPUYet
	}
	dev, ok := w.gpu.(gpu.D3D11Device)
	if !ok {
		return nil, errors.New("app: the window backend is not Direct3D 11")
	}
	return dev.D3D11()
}
