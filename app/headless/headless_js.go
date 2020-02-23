// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"errors"
	"syscall/js"

	"gioui.org/app/internal/glimpl"
	"gioui.org/gpu/backend"
	"gioui.org/gpu/gl"
)

type jsContext struct {
	ctx js.Value
	f   *glimpl.Functions
}

func newGLContext() (context, error) {
	version := 2
	doc := js.Global().Get("document")
	cnv := doc.Call("createElement", "canvas")
	ctx := cnv.Call("getContext", "webgl2")
	if ctx.IsNull() {
		version = 1
		ctx = cnv.Call("getContext", "webgl")
	}
	if ctx.IsNull() {
		return nil, errors.New("headless: webgl is not supported")
	}
	f := &glimpl.Functions{Ctx: ctx}
	if err := f.Init(version); err != nil {
		return nil, err
	}
	c := &jsContext{
		ctx: ctx,
		f:   f,
	}
	return c, nil
}

func (c *jsContext) Backend() (backend.Device, error) {
	return gl.NewBackend(c.f)
}

func (c *jsContext) Functions() *glimpl.Functions {
	return c.f
}

func (c *jsContext) Release() {
}

func (c *jsContext) ReleaseCurrent() {
}

func (c *jsContext) MakeCurrent() error {
	return nil
}
