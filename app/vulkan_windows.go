// SPDX-License-Identifier: Unlicense OR MIT

//go:build windows

package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"gioui.org/app/internal/windows"
	"gioui.org/gpu"
	"gioui.org/internal/vk"
)

// win32VkContext presents to a VK_KHR_win32_surface surface created from the
// window HWND. The shared vkContext in vulkan.go owns the device, swapchain
// and frame synchronization; this type only binds it to the Win32 window.
type win32VkContext struct {
	win    *window
	inst   vk.Instance
	surf   vk.Surface
	ctx    *vkContext
	width  int
	height int
}

// vulkanSelected reports whether the process asked for the Vulkan backend
// through DOW_GPU_API (set by the DoW Mod Studio settings dialog before the
// window is created). The environment is read when a GPU context is created,
// not at package init, because the settings dialog sets it later during
// startup.
func vulkanSelected() bool {
	return strings.EqualFold(os.Getenv("DOW_GPU_API"), "vulkan")
}

func init() {
	// The driver sorts by priority at context creation time, but the
	// DOW_GPU_API environment variable is only set once the application has
	// started. So register with the top priority and gate on the variable in
	// the initializer: when Vulkan is not selected the initializer fails
	// immediately and the next driver (Direct3D) is tried; when it is
	// selected, Vulkan runs first and Direct3D stays as the fallback.
	drivers = append(drivers, gpuAPI{
		priority: 0,
		initializer: func(w *window) (context, error) {
			if !vulkanSelected() {
				return nil, errors.New("vulkan: backend not selected (DOW_GPU_API)")
			}
			c, err := newWin32VkContext(w)
			if err != nil {
				log.Printf("3D preview: Vulkan initialization failed, falling back to the next driver: %v", err)
				return nil, err
			}
			return c, nil
		},
	})
}

func newWin32VkContext(w *window) (*win32VkContext, error) {
	inst, err := vk.CreateInstance("VK_KHR_surface", "VK_KHR_win32_surface")
	if err != nil {
		return nil, fmt.Errorf("vulkan: %w", err)
	}
	hwnd, width, height := w.HWND()
	hinstance, err := windows.GetModuleHandle()
	if err != nil {
		vk.DestroyInstance(inst)
		return nil, err
	}
	surf, err := vk.CreateWin32Surface(inst, uintptr(hinstance), uintptr(hwnd))
	if err != nil {
		vk.DestroyInstance(inst)
		return nil, err
	}
	vctx, err := newVulkanContext(inst, surf)
	if err != nil {
		vk.DestroySurface(inst, surf)
		vk.DestroyInstance(inst)
		return nil, err
	}
	log.Printf("3D preview: Vulkan context created for a %dx%d window (device: %q, DOW_GPU_API=%q)",
		width, height, vk.DeviceName(vctx.physDev), os.Getenv("DOW_GPU_API"))
	return &win32VkContext{
		win:    w,
		inst:   inst,
		surf:   surf,
		ctx:    vctx,
		width:  width,
		height: height,
	}, nil
}

func (c *win32VkContext) RenderTarget() (gpu.RenderTarget, error) {
	return c.ctx.RenderTarget()
}

func (c *win32VkContext) API() gpu.API {
	return c.ctx.api()
}

func (c *win32VkContext) Release() {
	c.ctx.release()
	vk.DestroySurface(c.inst, c.surf)
	vk.DestroyInstance(c.inst)
	*c = win32VkContext{}
}

func (c *win32VkContext) Present() error {
	return c.ctx.present()
}

func (c *win32VkContext) Lock() error {
	return nil
}

func (c *win32VkContext) Unlock() {}

func (c *win32VkContext) Refresh() error {
	_, width, height := c.win.HWND()
	// A minimized or zero sized window has no surface to render to; skip the
	// frame until it is restored.
	if width < 1 || height < 1 {
		return errOutOfDate
	}
	// Like the Direct3D backend, only recreate the swapchain when the window
	// actually changed size. WM_PAINT drives sync frames and fires often.
	if c.ctx.swchain != 0 && width == c.width && height == c.height {
		return nil
	}
	if err := c.ctx.refresh(c.surf, width, height); err != nil {
		return err
	}
	c.width = width
	c.height = height
	return nil
}
