package app

import (
	"errors"
	"syscall/js"

	"gioui.org/ui/app/internal/gl"
)

type context struct {
	ctx     js.Value
	cnv     js.Value
	f       *gl.Functions
	srgbFBO *gl.SRGBFBO
}

func newContext(w *window) (*context, error) {
	args := map[string]interface{}{
		// Enable low latency rendering.
		// See https://developers.google.com/web/updates/2019/05/desynchronized.
		"desynchronized":        true,
		"preserveDrawingBuffer": true,
	}
	ctx := w.cnv.Call("getContext", "webgl2", args)
	if ctx == js.Null() {
		ctx = w.cnv.Call("getContext", "webgl", args)
	}
	if ctx == js.Null() {
		return nil, errors.New("app: webgl is not supported")
	}
	f := &gl.Functions{Ctx: ctx}
	f.Init()
	c := &context{
		ctx: ctx,
		cnv: w.cnv,
		f:   f,
	}
	return c, nil
}

func (c *context) Functions() *gl.Functions {
	return c.f
}

func (c *context) Release() {
	if c.srgbFBO != nil {
		c.srgbFBO.Release()
		c.srgbFBO = nil
	}
}

func (c *context) Present() error {
	if c.srgbFBO != nil {
		c.srgbFBO.Blit()
	}
	if c.srgbFBO != nil {
		c.srgbFBO.AfterPresent()
	}
	if c.ctx.Call("isContextLost").Bool() {
		return errors.New("context lost")
	}
	return nil
}

func (c *context) MakeCurrent() error {
	if c.srgbFBO == nil {
		var err error
		c.srgbFBO, err = gl.NewSRGBFBO(c.f)
		if err != nil {
			c.Release()
			c.srgbFBO = nil
			return err
		}
	}
	w, h := c.cnv.Get("width").Int(), c.cnv.Get("height").Int()
	if err := c.srgbFBO.Refresh(w, h); err != nil {
		c.Release()
		return err
	}
	return nil
}
