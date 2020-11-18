// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"runtime"
	"testing"

	"gioui.org/gpu/backend"
	"gioui.org/internal/f32color"
	"gioui.org/internal/unsafe"
)

var dumpImages = flag.Bool("saveimages", false, "save test images")

var clearCol = color.NRGBA{A: 0xff, R: 0xde, G: 0xad, B: 0xbe}
var clearColExpect = f32color.NRGBAToRGBA(clearCol)

func TestFramebufferClear(t *testing.T) {
	b := newBackend(t)
	sz := image.Point{X: 800, Y: 600}
	fbo := setupFBO(t, b, sz)
	img := screenshot(t, fbo, sz)
	if got := img.RGBAAt(0, 0); got != clearColExpect {
		t.Errorf("got color %v, expected %v", got, clearColExpect)
	}
}

func TestSimpleShader(t *testing.T) {
	b := newBackend(t)
	sz := image.Point{X: 800, Y: 600}
	fbo := setupFBO(t, b, sz)
	p, err := b.NewProgram(shader_simple_vert, shader_simple_frag)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	b.BindProgram(p)
	b.DrawArrays(backend.DrawModeTriangles, 0, 3)
	img := screenshot(t, fbo, sz)
	if got := img.RGBAAt(0, 0); got != clearColExpect {
		t.Errorf("got color %v, expected %v", got, clearColExpect)
	}
	// Just off the center to catch inverted triangles.
	cx, cy := 300, 400
	shaderCol := f32color.RGBA{R: .25, G: .55, B: .75, A: 1.0}
	if got, exp := img.RGBAAt(cx, cy), shaderCol.SRGB(); got != f32color.NRGBAToRGBA(exp) {
		t.Errorf("got color %v, expected %v", got, f32color.NRGBAToRGBA(exp))
	}
}

func TestInputShader(t *testing.T) {
	b := newBackend(t)
	sz := image.Point{X: 800, Y: 600}
	fbo := setupFBO(t, b, sz)
	p, err := b.NewProgram(shader_input_vert, shader_simple_frag)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release()
	b.BindProgram(p)
	buf, err := b.NewImmutableBuffer(backend.BufferBindingVertices,
		unsafe.BytesView([]float32{
			0, .5, .5, 1,
			-.5, -.5, .5, 1,
			.5, -.5, .5, 1,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer buf.Release()
	b.BindVertexBuffer(buf, 4*4, 0)
	layout, err := b.NewInputLayout(shader_input_vert, []backend.InputDesc{
		{
			Type:   backend.DataTypeFloat,
			Size:   4,
			Offset: 0,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer layout.Release()
	b.BindInputLayout(layout)
	b.DrawArrays(backend.DrawModeTriangles, 0, 3)
	img := screenshot(t, fbo, sz)
	if got := img.RGBAAt(0, 0); got != clearColExpect {
		t.Errorf("got color %v, expected %v", got, clearColExpect)
	}
	cx, cy := 300, 400
	shaderCol := f32color.RGBA{R: .25, G: .55, B: .75, A: 1.0}
	if got, exp := img.RGBAAt(cx, cy), shaderCol.SRGB(); got != f32color.NRGBAToRGBA(exp) {
		t.Errorf("got color %v, expected %v", got, f32color.NRGBAToRGBA(exp))
	}
}

func TestFramebuffers(t *testing.T) {
	b := newBackend(t)
	sz := image.Point{X: 800, Y: 600}
	fbo1 := newFBO(t, b, sz)
	fbo2 := newFBO(t, b, sz)
	var (
		col1 = color.NRGBA{R: 0xad, G: 0xbe, B: 0xef, A: 0xde}
		col2 = color.NRGBA{R: 0xfe, G: 0xba, B: 0xbe, A: 0xca}
	)
	fcol1, fcol2 := f32color.LinearFromSRGB(col1), f32color.LinearFromSRGB(col2)
	b.BindFramebuffer(fbo1)
	b.Clear(fcol1.Float32())
	b.BindFramebuffer(fbo2)
	b.Clear(fcol2.Float32())
	img := screenshot(t, fbo1, sz)
	if got := img.RGBAAt(0, 0); got != f32color.NRGBAToRGBA(col1) {
		t.Errorf("got color %v, expected %v", got, f32color.NRGBAToRGBA(col1))
	}
	img = screenshot(t, fbo2, sz)
	if got := img.RGBAAt(0, 0); got != f32color.NRGBAToRGBA(col2) {
		t.Errorf("got color %v, expected %v", got, f32color.NRGBAToRGBA(col2))
	}
}

func setupFBO(t *testing.T, b backend.Device, size image.Point) backend.Framebuffer {
	fbo := newFBO(t, b, size)
	b.BindFramebuffer(fbo)
	// ClearColor accepts linear RGBA colors, while 8-bit colors
	// are in the sRGB color space.
	col := f32color.LinearFromSRGB(clearCol)
	b.Clear(col.Float32())
	b.ClearDepth(0.0)
	b.Viewport(0, 0, size.X, size.Y)
	return fbo
}

func newFBO(t *testing.T, b backend.Device, size image.Point) backend.Framebuffer {
	fboTex, err := b.NewTexture(
		backend.TextureFormatSRGB,
		size.X, size.Y,
		backend.FilterNearest, backend.FilterNearest,
		backend.BufferBindingFramebuffer,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fboTex.Release()
	})
	const depthBits = 16
	fbo, err := b.NewFramebuffer(fboTex, depthBits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fbo.Release()
	})
	return fbo
}

func newBackend(t *testing.T) backend.Device {
	ctx, err := newContext()
	if err != nil {
		t.Skipf("no context available: %v", err)
	}
	runtime.LockOSThread()
	if err := ctx.MakeCurrent(); err != nil {
		t.Fatal(err)
	}
	b, err := ctx.Backend()
	if err != nil {
		t.Fatal(err)
	}
	b.BeginFrame()
	t.Cleanup(func() {
		b.EndFrame()
		ctx.ReleaseCurrent()
		runtime.UnlockOSThread()
		ctx.Release()
	})
	return b
}

func screenshot(t *testing.T, fbo backend.Framebuffer, size image.Point) *image.RGBA {
	img := image.NewRGBA(image.Rectangle{Max: size})
	err := fbo.ReadPixels(
		image.Rectangle{
			Max: image.Point{X: size.X, Y: size.Y},
		}, img.Pix)
	if err != nil {
		t.Fatal(err)
	}
	if *dumpImages {
		if err := saveImage(t.Name()+".png", img); err != nil {
			t.Error(err)
		}
	}
	return img
}

func saveImage(file string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return ioutil.WriteFile(file, buf.Bytes(), 0666)
}
