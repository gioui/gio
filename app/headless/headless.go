// SPDX-License-Identifier: Unlicense OR MIT

// Package headless implements headless windows for rendering
// an operation list to an image.
package headless

import (
	"fmt"
	"image"
	"runtime"

	"gioui.org/app/internal/gl"
	"gioui.org/app/internal/gpu"
	"gioui.org/op"
)

// Window is a headless window.
type Window struct {
	size image.Point
	ctx  context
	fbo  *gl.SRGBFBO
	gpu  *gpu.GPU
}

type context interface {
	Functions() *gl.Functions
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
		f := ctx.Functions()
		fbo, err := gl.NewSRGBFBO(f)
		if err != nil {
			ctx.Release()
			return err
		}
		if err := fbo.Refresh(width, height); err != nil {
			fbo.Release()
			ctx.Release()
			return err
		}
		gpu, err := gpu.New(f)
		if err != nil {
			fbo.Release()
			ctx.Release()
			return err
		}
		w.fbo = fbo
		w.gpu = gpu
		return err
	})
	if err != nil {
		return nil, err
	}
	return w, nil
}

// Release resources associated with the window.
func (w *Window) Release() {
	contextDo(w.ctx, func() error {
		if w.gpu != nil {
			w.gpu.Release()
			w.gpu = nil
		}
		if w.fbo != nil {
			w.fbo.Release()
			w.fbo = nil
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
		w.gpu.Collect(false, w.size, frame)
		w.gpu.Frame(false, w.size)
		w.gpu.EndFrame(false)
		return nil
	})
}

// Screenshot returns an image with the content of the window.
func (w *Window) Screenshot() (*image.RGBA, error) {
	img := image.NewRGBA(image.Rectangle{Max: w.size})
	if len(img.Pix) != w.size.X*w.size.Y*4 {
		panic("unexpected RGBA size")
	}
	contextDo(w.ctx, func() error {
		f := w.ctx.Functions()
		f.ReadPixels(0, 0, w.size.X, w.size.Y, gl.RGBA, gl.UNSIGNED_BYTE, img.Pix)
		if glErr := f.GetError(); glErr != gl.NO_ERROR {
			return fmt.Errorf("glReadPixels failed: %d", glErr)
		}
		return nil
	})
	// Flip image in y-direction. OpenGL's origin is in the lower
	// left corner.
	row := make([]uint8, img.Stride)
	for y := 0; y < w.size.Y/2; y++ {
		y1 := w.size.Y - y - 1
		dest := img.PixOffset(0, y1)
		src := img.PixOffset(0, y)
		copy(row, img.Pix[dest:])
		copy(img.Pix[dest:], img.Pix[src:src+len(row)])
		copy(img.Pix[src:], row)
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
