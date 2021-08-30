// SPDX-License-Identifier: Unlicense OR MIT

//go:build (linux || freebsd) && !novulkan
// +build linux freebsd
// +build !novulkan

package app

import (
	"errors"
	"unsafe"

	"gioui.org/gpu"
	"gioui.org/internal/vk"
)

type vkContext struct {
	physDev    vk.PhysicalDevice
	inst       vk.Instance
	surf       vk.Surface
	dev        vk.Device
	queueFam   int
	queue      vk.Queue
	acquireSem vk.Semaphore
	presentSem vk.Semaphore

	swchain    vk.Swapchain
	imgs       []vk.Image
	views      []vk.ImageView
	fbos       []vk.Framebuffer
	format     vk.Format
	presentIdx int
}

func newVulkanContext(inst vk.Instance, surf vk.Surface) (*vkContext, error) {
	physDev, qFam, err := vk.ChoosePhysicalDevice(inst, surf)
	if err != nil {
		vk.DestroySurface(inst, surf)
		return nil, err
	}
	dev, err := vk.CreateDeviceAndQueue(physDev, qFam, "VK_KHR_swapchain")
	if err != nil {
		vk.DestroySurface(inst, surf)
		return nil, err
	}
	if err != nil {
		vk.DestroySurface(inst, surf)
		vk.DestroyDevice(dev)
		return nil, err
	}
	acquireSem, err := vk.CreateSemaphore(dev)
	if err != nil {
		vk.DestroySurface(inst, surf)
		vk.DestroyDevice(dev)
		return nil, err
	}
	presentSem, err := vk.CreateSemaphore(dev)
	if err != nil {
		vk.DestroySemaphore(dev, acquireSem)
		vk.DestroySurface(inst, surf)
		vk.DestroyDevice(dev)
		return nil, err
	}
	c := &vkContext{
		physDev:    physDev,
		inst:       inst,
		surf:       surf,
		dev:        dev,
		queueFam:   qFam,
		queue:      vk.GetDeviceQueue(dev, qFam, 0),
		acquireSem: acquireSem,
		presentSem: presentSem,
	}
	return c, nil
}

func (c *vkContext) RenderTarget() (gpu.RenderTarget, error) {
	vk.DeviceWaitIdle(c.dev)

	imgIdx, err := vk.AcquireNextImage(c.dev, c.swchain, c.acquireSem, 0)
	if err != nil {
		return nil, mapErr(err)
	}
	c.presentIdx = imgIdx
	return gpu.VulkanRenderTarget{
		WaitSem:     uint64(c.acquireSem),
		SignalSem:   uint64(c.presentSem),
		Framebuffer: uint64(c.fbos[imgIdx]),
		Image:       uint64(c.imgs[imgIdx]),
	}, nil
}

func (c *vkContext) api() gpu.API {
	return gpu.Vulkan{
		PhysDevice:  unsafe.Pointer(c.physDev),
		Device:      unsafe.Pointer(c.dev),
		Format:      int(c.format),
		QueueFamily: c.queueFam,
		QueueIndex:  0,
	}
}

func mapErr(err error) error {
	var vkErr vk.Error
	if !errors.As(err, &vkErr) {
		return err
	}
	switch {
	case vkErr == vk.SUBOPTIMAL_KHR:
		// Android reports VK_SUBOPTIMAL_KHR when presenting to a rotated
		// swapchain (preTransform != currentTransform). However, we don't
		// support transforming the output ourselves, so we'll live with it.
		return nil
	case vkErr.IsDeviceLost():
		return gpu.ErrDeviceLost
	}
	return err
}

func (c *vkContext) release() {
	vk.DeviceWaitIdle(c.dev)

	c.destroySurface()
	vk.DestroySemaphore(c.dev, c.acquireSem)
	vk.DestroySemaphore(c.dev, c.presentSem)
	vk.DestroyDevice(c.dev)
	*c = vkContext{}
}

func (c *vkContext) present() error {
	return mapErr(vk.PresentQueue(c.queue, c.swchain, c.presentSem, c.presentIdx))
}

func (c *vkContext) destroyImageViews() {
	for _, f := range c.fbos {
		vk.DestroyFramebuffer(c.dev, f)
	}
	c.fbos = nil
	for _, view := range c.views {
		vk.DestroyImageView(c.dev, view)
	}
	c.views = nil
}

func (c *vkContext) destroySurface() {
	vk.DeviceWaitIdle(c.dev)

	c.destroyImageViews()
	if c.swchain != 0 {
		vk.DestroySwapchain(c.dev, c.swchain)
		c.swchain = 0
	}
	if c.surf != 0 {
		vk.DestroySurface(c.inst, c.surf)
		c.surf = 0
	}
}

func (c *vkContext) setSurface(surf vk.Surface) {
	if c.surf != 0 {
		panic("another surface is active")
	}
	c.surf = surf
}

func (c *vkContext) refresh(width, height int) error {
	vk.DeviceWaitIdle(c.dev)

	c.destroyImageViews()
	swchain, imgs, format, err := vk.CreateSwapchain(c.physDev, c.dev, c.surf, width, height, c.swchain)
	if c.swchain != 0 {
		vk.DestroySwapchain(c.dev, c.swchain)
		c.swchain = 0
	}
	if err != nil {
		return mapErr(err)
	}
	c.swchain = swchain
	c.imgs = imgs
	c.format = format
	pass, err := vk.CreateRenderPass(
		c.dev,
		format,
		vk.ATTACHMENT_LOAD_OP_CLEAR,
		vk.IMAGE_LAYOUT_UNDEFINED,
		vk.IMAGE_LAYOUT_PRESENT_SRC_KHR,
		nil,
	)
	if err != nil {
		return mapErr(err)
	}
	defer vk.DestroyRenderPass(c.dev, pass)
	for _, img := range imgs {
		view, err := vk.CreateImageView(c.dev, img, format)
		if err != nil {
			return mapErr(err)
		}
		c.views = append(c.views, view)
		fbo, err := vk.CreateFramebuffer(c.dev, pass, view, width, height)
		if err != nil {
			return mapErr(err)
		}
		c.fbos = append(c.fbos, fbo)
	}
	return nil
}
