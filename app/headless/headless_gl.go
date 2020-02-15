// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"fmt"
	"runtime"

	"gioui.org/app/internal/glimpl"
	"gioui.org/app/internal/srgb"
	"gioui.org/gpu"
	"gioui.org/gpu/gl"
)

type glContext interface {
	Functions() *glimpl.Functions
	Backend() (gpu.Backend, error)
	MakeCurrent() error
	ReleaseCurrent()
	Release()
}

type glBackend struct {
	glContext
	srgb *srgb.FBO
}

func newContext(width, height int) (backend, error) {
	glctx, err := newGLContext()
	if err != nil {
		return nil, err
	}
	// Create the back buffer FBO after locking the thread and making
	// the context current.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := glctx.MakeCurrent(); err != nil {
		glctx.Release()
		return nil, err
	}
	defer glctx.ReleaseCurrent()
	fbo, err := srgb.New(glctx.Functions())
	if err != nil {
		glctx.Release()
		return nil, err
	}
	if err := fbo.Refresh(width, height); err != nil {
		fbo.Release()
		glctx.Release()
		return nil, err
	}
	return &glBackend{glctx, fbo}, nil
}

func (b *glBackend) Screenshot(width, height int, pixels []byte) error {
	if len(pixels) != width*height*4 {
		panic("unexpected RGBA size")
	}
	f := b.Functions()
	f.ReadPixels(0, 0, width, height, gl.RGBA, gl.UNSIGNED_BYTE, pixels)
	if glErr := f.GetError(); glErr != gl.NO_ERROR {
		return fmt.Errorf("glReadPixels failed: %d", glErr)
	}
	return nil
}

func (b *glBackend) Release() {
	if b.srgb != nil {
		b.srgb.Release()
		b.srgb = nil
	}
	b.glContext.Release()
}
