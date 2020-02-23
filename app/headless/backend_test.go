// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"math"
	"runtime"
	"testing"

	"gioui.org/gpu/backend"
	"gioui.org/internal/unsafe"
)

var dumpImages = flag.Bool("saveimages", false, "save test images")

var clearCol = color.RGBA{A: 0xff, R: 0xde, G: 0xad, B: 0xbe}

func TestFramebufferClear(t *testing.T) {
	b := newBackend(t)
	sz := image.Point{X: 800, Y: 600}
	fbo := setupFBO(t, b, sz)
	img := screenshot(t, fbo, sz)
	if got := img.RGBAAt(0, 0); got != clearCol {
		t.Errorf("got color %v, expected %v", got, clearCol)
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
	if got := img.RGBAAt(0, 0); got != clearCol {
		t.Errorf("got color %v, expected %v", got, clearCol)
	}
	// Just off the center to catch inverted triangles.
	cx, cy := 300, 400
	shaderCol := [4]float32{.25, .55, .75, 1.0}
	if got, exp := img.RGBAAt(cx, cy), tosRGB(shaderCol); got != exp {
		t.Errorf("got color %v, expected %v", got, exp)
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
	if got := img.RGBAAt(0, 0); got != clearCol {
		t.Errorf("got color %v, expected %v", got, clearCol)
	}
	cx, cy := 300, 400
	shaderCol := [4]float32{.25, .55, .75, 1.0}
	if got, exp := img.RGBAAt(cx, cy), tosRGB(shaderCol); got != exp {
		t.Errorf("got color %v, expected %v", got, exp)
	}
}

func TestFramebuffers(t *testing.T) {
	b := newBackend(t)
	sz := image.Point{X: 800, Y: 600}
	fbo1 := newFBO(t, b, sz)
	fbo2 := newFBO(t, b, sz)
	var (
		col1 = color.RGBA{R: 0xad, G: 0xbe, B: 0xef, A: 0xde}
		col2 = color.RGBA{R: 0xfe, G: 0xba, B: 0xbe, A: 0xca}
	)
	fcol1, fcol2 := fromsRGB(col1), fromsRGB(col2)
	b.ClearColor(fcol1[0], fcol1[1], fcol1[2], fcol1[3])
	b.BindFramebuffer(fbo1)
	b.Clear(backend.BufferAttachmentColor)
	b.ClearColor(fcol2[0], fcol2[1], fcol2[2], fcol2[3])
	b.BindFramebuffer(fbo2)
	b.Clear(backend.BufferAttachmentColor)
	img := screenshot(t, fbo1, sz)
	if got := img.RGBAAt(0, 0); got != col1 {
		t.Errorf("got color %v, expected %v", got, col1)
	}
	img = screenshot(t, fbo2, sz)
	if got := img.RGBAAt(0, 0); got != col2 {
		t.Errorf("got color %v, expected %v", got, col2)
	}
}

func setupFBO(t *testing.T, b backend.Device, size image.Point) backend.Framebuffer {
	fbo := newFBO(t, b, size)
	b.BindFramebuffer(fbo)
	b.Clear(backend.BufferAttachmentColor | backend.BufferAttachmentDepth)
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
	// ClearColor accepts linear RGBA colors, while 8-bit colors
	// are in the sRGB color space.
	col := fromsRGB(clearCol)
	b.ClearColor(col[0], col[1], col[2], col[3])
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
	flipImageY(img)
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

func tosRGB(col [4]float32) color.RGBA {
	for i := 0; i <= 2; i++ {
		c := col[i]
		// Use the formula from EXT_sRGB.
		switch {
		case c <= 0:
			c = 0
		case 0 < c && c < 0.0031308:
			c = 12.92 * c
		case 0.0031308 <= c && c < 1:
			c = 1.055*float32(math.Pow(float64(c), 0.41666)) - 0.055
		case c >= 1:
			c = 1
		}
		col[i] = c
	}
	return color.RGBA{R: uint8(col[0]*255 + .5), G: uint8(col[1]*255 + .5), B: uint8(col[2]*255 + .5), A: uint8(col[3]*255 + .5)}
}

func fromsRGB(col color.Color) [4]float32 {
	r, g, b, a := col.RGBA()
	color := [4]float32{float32(r) / 0xffff, float32(g) / 0xffff, float32(b) / 0xffff, float32(a) / 0xffff}
	for i := 0; i <= 2; i++ {
		c := color[i]
		// Use the formula from EXT_sRGB.
		if c <= 0.04045 {
			c = c / 12.92
		} else {
			c = float32(math.Pow(float64((c+0.055)/1.055), 2.4))
		}
		color[i] = c
	}
	return color
}
