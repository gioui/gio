// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"errors"

	"gioui.org/gpu"
)

// Vulkan returns a handle for issuing custom Vulkan passes against this
// window, or an error if its backend is not Vulkan.
//
// Passes draw on the same device and queue Gio uses, so they must be issued
// from the goroutine that calls Frame, and before it. A pass submits its
// commands and waits for them, so the texture it produces is finished by the
// time Frame samples it. Textures created this way can be painted with
// paint.NewImageOpFromTexture without copying them to system memory.
func (w *Window) Vulkan() (*gpu.VulkanPass, error) {
	if w.gpu == nil {
		return nil, ErrNoGPUYet
	}
	dev, ok := w.gpu.(gpu.VulkanDevice)
	if !ok {
		return nil, errors.New("app: the window backend is not Vulkan")
	}
	return dev.Vulkan()
}
