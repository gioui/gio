// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"gioui.org/app/internal/d3d11"
	"gioui.org/gpu/backend"
)

type d3d11Context struct {
	*d3d11.Device
}

func newContext() (context, error) {
	dev, err := d3d11.NewDevice()
	if err != nil {
		return nil, err
	}
	return &d3d11Context{Device: dev}, nil
}

func (c *d3d11Context) Backend() (backend.Device, error) {
	backend, err := d3d11.NewBackend(c.Device)
	if err != nil {
		return nil, err
	}
	return backend, nil
}

func (c *d3d11Context) MakeCurrent() error {
	return nil
}

func (c *d3d11Context) ReleaseCurrent() {
}

func (c *d3d11Context) Release() {
	c.Device.Release()
	c.Device = nil
}
