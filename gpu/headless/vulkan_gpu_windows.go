// SPDX-License-Identifier: Unlicense OR MIT

//go:build windows

package headless

import (
	"errors"

	"gioui.org/gpu"
)

// Vulkan returns a handle for issuing custom Vulkan passes against this
// window, or an error if its backend is not Vulkan.
func (w *Window) Vulkan() (*gpu.VulkanPass, error) {
	dev, ok := w.gpu.(gpu.VulkanDevice)
	if !ok {
		return nil, errors.New("headless: the backend is not Vulkan")
	}
	var v *gpu.VulkanPass
	err := contextDo(w.ctx, func() error {
		var err error
		v, err = dev.Vulkan()
		return err
	})
	return v, err
}
