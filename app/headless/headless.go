// SPDX-License-Identifier: Unlicense OR MIT

// Package headless implements headless windows for rendering
// an operation list to an image.
package headless

import (
	"image"
	"runtime"

	"gioui.org/gpu"
	"gioui.org/gpu/backend"
	"gioui.org/op"
)

// Window is a headless window.
type Window struct {
	size    image.Point
	ctx     context
	backend backend.Device
	gpu     *gpu.GPU
	fboTex  backend.Texture
	fbo     backend.Framebuffer
}

type context interface {
	Backend() (backend.Device, error)
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
		dev, err := ctx.Backend()
		if err != nil {
			return err
		}
		dev.Viewport(0, 0, width, height)
		fboTex, err := dev.NewTexture(
			backend.TextureFormatSRGB,
			width, height,
			backend.FilterNearest, backend.FilterNearest,
			backend.BufferBindingFramebuffer,
		)
		if err != nil {
			return nil
		}
		const depthBits = 16
		fbo, err := dev.NewFramebuffer(fboTex, depthBits)
		if err != nil {
			fboTex.Release()
			return err
		}
		dev.BindFramebuffer(fbo)
		gp, err := gpu.New(dev)
		if err != nil {
			fbo.Release()
			fboTex.Release()
			return err
		}
		w.fboTex = fboTex
		w.fbo = fbo
		w.gpu = gp
		w.backend = dev
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
		if w.ctx != nil {
			w.ctx.Release()
			w.ctx = nil
		}
		return nil
	})
}

// Frame replace the window content and state with the
// operation list.
func (w *Window) Frame(frame *op.Ops) {
	contextDo(w.ctx, func() error {
		w.gpu.Collect(w.size, frame)
		w.gpu.BeginFrame()
		w.gpu.EndFrame()
		return nil
	})
}

// Screenshot returns an image with the content of the window.
func (w *Window) Screenshot() (*image.RGBA, error) {
	img := image.NewRGBA(image.Rectangle{Max: w.size})
	err := contextDo(w.ctx, func() error {
		return w.fbo.ReadPixels(
			image.Rectangle{
				Max: image.Point{X: w.size.X, Y: w.size.Y},
			}, img.Pix)
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
		defer ctx.ReleaseCurrent()
		errCh <- f()
	}()
	return <-errCh
}
