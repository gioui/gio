// SPDX-License-Identifier: Unlicense OR MIT

package wm

import (
	"errors"
	"syscall/js"

	"gioui.org/gpu"
	"gioui.org/internal/gl"
)

type context struct {
	ctx js.Value
	cnv js.Value
}

func newContext(w *window) (*context, error) {
	args := map[string]interface{}{
		// Enable low latency rendering.
		// See https://developers.google.com/web/updates/2019/05/desynchronized.
		"desynchronized":        true,
		"preserveDrawingBuffer": true,
	}
	ctx := w.cnv.Call("getContext", "webgl2", args)
	if ctx.IsNull() {
		ctx = w.cnv.Call("getContext", "webgl", args)
	}
	if ctx.IsNull() {
		return nil, errors.New("app: webgl is not supported")
	}
	c := &context{
		ctx: ctx,
		cnv: w.cnv,
	}
	return c, nil
}

func (c *context) RenderTarget() gpu.RenderTarget {
	return gpu.OpenGLRenderTarget{}
}

func (c *context) API() gpu.API {
	return gpu.OpenGL{Context: gl.Context(c.ctx)}
}

func (c *context) Release() {
}

func (c *context) Present() error {
	if c.ctx.Call("isContextLost").Bool() {
		return errors.New("context lost")
	}
	return nil
}

func (c *context) Lock() error {
	return nil
}

func (c *context) Unlock() {}

func (c *context) Refresh() error {
	return nil
}

func (w *window) NewContext() (Context, error) {
	return newContext(w)
}
