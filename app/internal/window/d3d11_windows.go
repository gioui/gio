// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"gioui.org/app/internal/d3d11"
	"gioui.org/gpu/backend"
)

type d3d11Context struct {
	win           *window
	swchain       *d3d11.SwapChain
	fbo           *d3d11.Framebuffer
	dev           *d3d11.Device
	width, height int
}

func init() {
	backends = append(backends, gpuAPI{
		priority: 1,
		initializer: func(w *window) (Context, error) {
			hwnd, _, _ := w.HWND()
			dev, err := d3d11.NewDevice()
			if err != nil {
				return nil, err
			}
			swchain, err := dev.CreateSwapChain(hwnd)
			if err != nil {
				dev.Release()
				return nil, err
			}
			return &d3d11Context{win: w, dev: dev, swchain: swchain}, nil
		},
	})
}

func (c *d3d11Context) Backend() (backend.Device, error) {
	return d3d11.NewBackend(c.dev)
}

func (c *d3d11Context) Present() error {
	err := c.swchain.Present()
	if err == nil {
		return nil
	}
	if err, ok := err.(d3d11.ErrorCode); ok {
		switch err.Code {
		case d3d11.DXGI_STATUS_OCCLUDED:
			// Ignore
			return nil
		case d3d11.DXGI_ERROR_DEVICE_RESET, d3d11.DXGI_ERROR_DEVICE_REMOVED, d3d11.D3DDDIERR_DEVICEREMOVED:
			return ErrDeviceLost
		}
	}
	return err
}

func (c *d3d11Context) MakeCurrent() error {
	_, width, height := c.win.HWND()
	if c.fbo != nil && width == c.width && height == c.height {
		c.dev.BindFramebuffer(c.fbo)
		return nil
	}
	if c.fbo != nil {
		c.fbo.Release()
		c.fbo = nil
	}
	if err := c.swchain.Resize(); err != nil {
		return err
	}
	c.width = width
	c.height = height
	fbo, err := c.swchain.Framebuffer(c.dev)
	if err != nil {
		return err
	}
	c.fbo = fbo
	c.dev.BindFramebuffer(c.fbo)
	return nil
}

func (c *d3d11Context) Lock() {}

func (c *d3d11Context) Unlock() {}

func (c *d3d11Context) Release() {
	if c.fbo != nil {
		c.fbo.Release()
	}
	if c.swchain != nil {
		c.swchain.Release()
	}
	if c.dev != nil {
		c.dev.Release()
	}
	c.fbo = nil
	c.swchain = nil
	c.dev = nil
}
