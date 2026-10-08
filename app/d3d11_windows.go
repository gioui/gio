// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"fmt"
	"log"
	"unsafe"

	"gioui.org/gpu"
	"gioui.org/internal/d3d11"
)

type d3d11Context struct {
	win *window
	dev *d3d11.Device
	ctx *d3d11.DeviceContext

	swchain       *d3d11.IDXGISwapChain
	renderTarget  *d3d11.RenderTargetView
	backBuffer    *d3d11.IUnknown
	debugQueue    *d3d11.InfoQueue
	width, height int
}

// debugDirectX turns on the D3D11 validation layer, which is the only thing
// that says why a draw call was discarded. It needs the Graphics Tools
// optional feature; when it is missing the device falls back silently.
const debugDirectX = true

func init() {
	drivers = append(drivers, gpuAPI{
		priority: 1,
		initializer: func(w *window) (context, error) {
			hwnd, _, _ := w.HWND()
			var flags uint32
			if debugDirectX {
				flags |= d3d11.CREATE_DEVICE_DEBUG
			}
			dev, ctx, _, err := d3d11.CreateDevice(
				d3d11.DRIVER_TYPE_HARDWARE,
				flags,
			)
			if err != nil && flags != 0 {
				// The debug layer is an optional Windows feature; without it
				// the device has to be created the ordinary way.
				log.Printf("3D preview: d3d11 debug layer unavailable (%v), continuing without it", err)
				dev, ctx, _, err = d3d11.CreateDevice(d3d11.DRIVER_TYPE_HARDWARE, 0)
			}
			if err != nil {
				return nil, fmt.Errorf("NewContext: %v", err)
			}
			c := &d3d11Context{win: w, dev: dev, ctx: ctx}
			if q, qerr := dev.InfoQueue(); qerr == nil {
				q.SetMessageCountLimit(512)
				c.debugQueue = q
			}
			swchain, err := d3d11.CreateSwapChain(dev, hwnd)
			if err != nil {
				d3d11.IUnknownRelease(unsafe.Pointer(ctx), ctx.Vtbl.Release)
				d3d11.IUnknownRelease(unsafe.Pointer(dev), dev.Vtbl.Release)
				return nil, err
			}
			c.swchain = swchain
			return c, nil
		},
	})
}

func (c *d3d11Context) API() gpu.API {
	return gpu.Direct3D11{Device: unsafe.Pointer(c.dev)}
}

func (c *d3d11Context) RenderTarget() (gpu.RenderTarget, error) {
	return gpu.Direct3D11RenderTarget{
		RenderTarget: unsafe.Pointer(c.renderTarget),
	}, nil
}

func (c *d3d11Context) Present() error {
	return wrapErr(c.swchain.Present(1, 0))
}

func wrapErr(err error) error {
	if err, ok := err.(d3d11.ErrorCode); ok {
		switch err.Code {
		case d3d11.DXGI_STATUS_OCCLUDED:
			// Ignore
			return nil
		case d3d11.DXGI_ERROR_DEVICE_RESET, d3d11.DXGI_ERROR_DEVICE_REMOVED, d3d11.D3DDDIERR_DEVICEREMOVED:
			return gpu.ErrDeviceLost
		}
	}
	return err
}

func (c *d3d11Context) Refresh() error {
	var width, height int
	_, width, height = c.win.HWND()
	if c.renderTarget != nil && width == c.width && height == c.height {
		return nil
	}
	c.releaseFBO()
	// The immediate context holds a reference to the render target bound to
	// the output merger, and DXGI refuses to resize the swap chain while the
	// back buffer is referenced. Frames leave the window render target bound,
	// so unbind before resizing or ResizeBuffers fails with
	// DXGI_ERROR_INVALID_CALL and the window closes.
	c.ctx.OMSetRenderTargets(nil, nil)
	if err := c.swchain.ResizeBuffers(0, 0, 0, d3d11.DXGI_FORMAT_UNKNOWN, 0); err != nil {
		log.Printf("3D preview: ResizeBuffers to %dx%d: %v", width, height, err)
		return wrapErr(err)
	}
	c.width = width
	c.height = height

	backBuffer, err := c.swchain.GetBuffer(0, &d3d11.IID_Texture2D)
	if err != nil {
		return err
	}
	texture := (*d3d11.Resource)(unsafe.Pointer(backBuffer))
	renderTarget, err := c.dev.CreateRenderTargetView(texture)
	if err != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(backBuffer), backBuffer.Vtbl.Release)
		return err
	}
	// The texture is kept for ProbeBackbuffer: a readback of what is really
	// in the buffer is the only way to tell draws that landed from draws
	// the driver discarded.
	c.backBuffer = backBuffer
	c.renderTarget = renderTarget
	log.Printf("3D preview: backbuffer RTV %x %dx%d",
		uintptr(unsafe.Pointer(renderTarget)), width, height)
	return nil
}

// ProbeBackbuffer copies the back buffer to the CPU and reports what it
// actually holds: a handful of exact pixels and how many colours a coarse
// grid of the frame has. A frame whose grid has one colour is a frame where
// nothing was drawn, whatever the draw counters say.
func (c *d3d11Context) ProbeBackbuffer() (string, error) {
	if c.backBuffer == nil || c.width < 1 || c.height < 1 {
		return "no backbuffer", nil
	}
	staging, err := c.dev.CreateTexture2D(&d3d11.TEXTURE2D_DESC{
		Width:     uint32(c.width),
		Height:    uint32(c.height),
		MipLevels: 1,
		ArraySize: 1,
		Format:    d3d11.DXGI_FORMAT_R8G8B8A8_UNORM_SRGB,
		SampleDesc: d3d11.DXGI_SAMPLE_DESC{
			Count: 1,
		},
		Usage:          d3d11.USAGE_STAGING,
		CPUAccessFlags: d3d11.CPU_ACCESS_READ,
	})
	if err != nil {
		return "", fmt.Errorf("staging: %w", err)
	}
	defer d3d11.IUnknownRelease(unsafe.Pointer(staging), staging.Vtbl.Release)
	box := d3d11.BOX{Left: 0, Top: 0, Right: uint32(c.width), Bottom: uint32(c.height), Front: 0, Back: 1}
	c.ctx.CopySubresourceRegion(
		(*d3d11.Resource)(unsafe.Pointer(staging)), 0, 0, 0, 0,
		(*d3d11.Resource)(unsafe.Pointer(c.backBuffer)), 0,
		&box,
	)
	mapped, err := c.ctx.Map((*d3d11.Resource)(unsafe.Pointer(staging)), 0, d3d11.MAP_READ, 0)
	if err != nil {
		return "", fmt.Errorf("map: %w", err)
	}
	defer c.ctx.Unmap((*d3d11.Resource)(unsafe.Pointer(staging)), 0)
	pitch := int(mapped.RowPitch)
	pixel := func(x, y int) (int, int, int) {
		o := uintptr(mapped.PData) + uintptr(y*pitch) + uintptr(x*4)
		p := unsafe.Slice((*byte)(unsafe.Pointer(o)), 4)
		return int(p[0]), int(p[1]), int(p[2])
	}
	colors := map[[3]int]bool{}
	bright, total := 0, 0
	stepX, stepY := c.width/40, c.height/24
	if stepX < 1 {
		stepX = 1
	}
	if stepY < 1 {
		stepY = 1
	}
	for y := 0; y < c.height; y += stepY {
		for x := 0; x < c.width; x += stepX {
			pr, pg, pb := pixel(x, y)
			colors[[3]int{pr, pg, pb}] = true
			if pr+pg+pb > 180 {
				bright++
			}
			total++
		}
	}
	a, b, d := pixel(8, 8)
	e, f, g := pixel(c.width/2, c.height/2)
	h, i, j := pixel(c.width-8, c.height-8)
	return fmt.Sprintf("%dx%d gridcolors=%d bright=%d/%d corner=%d,%d,%d mid=%d,%d,%d last=%d,%d,%d",
		c.width, c.height, len(colors), bright, total, a, b, d, e, f, g, h, i, j), nil
}

func (c *d3d11Context) Lock() error {
	c.ctx.OMSetRenderTargets(c.renderTarget, nil)
	return nil
}

func (c *d3d11Context) Unlock() {}

func (c *d3d11Context) Release() {
	c.releaseFBO()
	if c.swchain != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(c.swchain), c.swchain.Vtbl.Release)
	}
	if c.ctx != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(c.ctx), c.ctx.Vtbl.Release)
	}
	if c.dev != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(c.dev), c.dev.Vtbl.Release)
	}
	*c = d3d11Context{}
	if debugDirectX {
		d3d11.ReportLiveObjects()
	}
}

func (c *d3d11Context) releaseFBO() {
	if c.renderTarget != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(c.renderTarget), c.renderTarget.Vtbl.Release)
		c.renderTarget = nil
	}
	if c.backBuffer != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(c.backBuffer), c.backBuffer.Vtbl.Release)
		c.backBuffer = nil
	}
}

// DrainDebugMessages reads everything the validation layer recorded since the
// last drain and empties the queue.
func (c *d3d11Context) DrainDebugMessages() []string {
	if c.debugQueue == nil {
		return nil
	}
	n := c.debugQueue.NumStoredMessages()
	if n == 0 {
		return nil
	}
	if n > 32 {
		n = 32
	}
	out := make([]string, 0, n)
	for i := uint64(0); i < n; i++ {
		if s, ok := c.debugQueue.GetMessage(i); ok {
			out = append(out, s)
		}
	}
	c.debugQueue.ClearStoredMessages()
	return out
}
