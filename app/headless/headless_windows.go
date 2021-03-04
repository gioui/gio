// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"unsafe"

	"gioui.org/gpu"
	"gioui.org/app/internal/d3d11"
)

type d3d11Context struct {
	dev *d3d11.Device
}

func newContext() (context, error) {
	dev, err := d3d11.NewDevice()
	if err != nil {
		return nil, err
	}
	return &d3d11Context{dev: dev}, nil
}

func (c *d3d11Context) API() gpu.API {
	return gpu.Direct3D11{Device: unsafe.Pointer(c.dev.Handle)}
}

func (c *d3d11Context) MakeCurrent() error {
	return nil
}

func (c *d3d11Context) ReleaseCurrent() {
}

func (c *d3d11Context) Release() {
	c.dev.Release()
	c.dev = nil
}
