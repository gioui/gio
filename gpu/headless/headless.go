// SPDX-License-Identifier: Unlicense OR MIT

// Package headless implements headless windows for rendering
// an operation list to an image.
package headless

import (
	"image"
	"image/color"
	"runtime"

	"gioui.org/gpu"
	"gioui.org/gpu/internal/driver"
	"gioui.org/op"
)

// Window is a headless window.
type Window struct {
	size   image.Point
	ctx    context
	dev    driver.Device
	gpu    gpu.GPU
	fboTex driver.Texture
	fbo    driver.Framebuffer
}

type context interface {
	API() gpu.API
	MakeCurrent() error
	ReleaseCurrent()
	Release()
}

// NewWindow creates a new headless window.
func NewWindow(width, height int) (*Window, error) {
	ctx, err := newContext()
	if err != nil {
		return nil, err
	}
	w := &Window{
		size: image.Point{X: width, Y: height},
		ctx:  ctx,
	}
	err = contextDo(ctx, func() error {
		api := ctx.API()
		dev, err := driver.NewDevice(api)
		if err != nil {
			return err
		}
		dev.Viewport(0, 0, width, height)
		fboTex, err := dev.NewTexture(
			driver.TextureFormatSRGBA,
			width, height,
			driver.FilterNearest, driver.FilterNearest,
			driver.BufferBindingFramebuffer,
		)
		if err != nil {
			return nil
		}
		fbo, err := dev.NewFramebuffer(fboTex)
		if err != nil {
			fboTex.Release()
			return err
		}
		gp, err := gpu.New(api)
		if err != nil {
			fbo.Release()
			fboTex.Release()
			return err
		}
		w.fboTex = fboTex
		w.fbo = fbo
		w.gpu = gp
		w.dev = dev
		return err
	})
	if err != nil {
		ctx.Release()
		return nil, err
	}
	return w, nil
}

// Release resources associated with the window.
func (w *Window) Release() {
	contextDo(w.ctx, func() error {
		if w.fbo != nil {
			w.fbo.Release()
			w.fbo = nil
		}
		if w.fboTex != nil {
			w.fboTex.Release()
			w.fboTex = nil
		}
		if w.gpu != nil {
			w.gpu.Release()
			w.gpu = nil
		}
		return nil
	})
	if w.ctx != nil {
		w.ctx.Release()
		w.ctx = nil
	}
}

// Frame replace the window content and state with the
// operation list.
func (w *Window) Frame(frame *op.Ops) error {
	return contextDo(w.ctx, func() error {
		w.gpu.Clear(color.NRGBA{})
		w.gpu.Collect(w.size, frame)
		return w.gpu.Frame(driver.RenderTarget(w.fbo))
	})
}

// Screenshot returns an image with the content of the window.
func (w *Window) Screenshot() (*image.RGBA, error) {
	var img *image.RGBA
	err := contextDo(w.ctx, func() error {
		var err error
		img, err = driver.DownloadImage(w.dev, w.fbo, image.Rectangle{Max: w.size})
		return err
	})
	if err != nil {
		return nil, err
	}
	return img, nil
}

func contextDo(ctx context, f func() error) error {
	errCh := make(chan error)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := ctx.MakeCurrent(); err != nil {
			errCh <- err
			return
		}
		err := f()
		ctx.ReleaseCurrent()
		errCh <- err
	}()
	return <-errCh
}
