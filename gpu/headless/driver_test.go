// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"runtime"
	"testing"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/byteslice"
	"gioui.org/internal/f32color"
	"gioui.org/shader"
	"gioui.org/shader/gio"
)

var dumpImages = flag.Bool("saveimages", false, "save test images")

var clearCol = color.NRGBA{A: 0xff, R: 0xde, G: 0xad, B: 0xbe}
var clearColExpect = f32color.NRGBAToRGBA(clearCol)

func TestFramebufferClear(t *testing.T) {
	b := newDriver(t)
	sz := image.Point{X: 800, Y: 600}
	fbo := newFBO(t, b, sz)
	d := driver.LoadDesc{
		// ClearColor accepts linear RGBA colors, while 8-bit colors
		// are in the sRGB color space.
		ClearColor: f32color.LinearFromSRGB(clearCol),
		Action:     driver.LoadActionClear,
	}
	b.BeginRenderPass(fbo, d)
	b.EndRenderPass()
	img := screenshot(t, b, fbo, sz)
	if got := img.RGBAAt(0, 0); got != clearColExpect {
		t.Errorf("got color %v, expected %v", got, clearColExpect)
	}
}

func TestInputShader(t *testing.T) {
	b := newDriver(t)
	sz := image.Point{X: 800, Y: 600}
	vsh, fsh, err := newShaders(b, gio.Shader_input_vert, gio.Shader_simple_frag)
	if err != nil {
		t.Fatal(err)
	}
	defer vsh.Release()
	defer fsh.Release()
	layout := driver.VertexLayout{
		Inputs: []driver.InputDesc{
			{
				Type:   shader.DataTypeFloat,
				Size:   4,
				Offset: 0,
			},
		},
		Stride: 4 * 4,
	}
	fbo := newFBO(t, b, sz)
	pipe, err := b.NewPipeline(driver.PipelineDesc{
		VertexShader:   vsh,
		FragmentShader: fsh,
		VertexLayout:   layout,
		PixelFormat:    driver.TextureFormatSRGBA,
		Topology:       driver.TopologyTriangles,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Release()
	buf, err := b.NewImmutableBuffer(driver.BufferBindingVertices,
		byteslice.Slice([]float32{
			0, -.5, .5, 1,
			-.5, +.5, .5, 1,
			.5, +.5, .5, 1,
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer buf.Release()
	d := driver.LoadDesc{
		ClearColor: f32color.LinearFromSRGB(clearCol),
		Action:     driver.LoadActionClear,
	}
	b.BeginRenderPass(fbo, d)
	b.Viewport(0, 0, sz.X, sz.Y)
	b.BindPipeline(pipe)
	b.BindVertexBuffer(buf, 0)
	b.DrawArrays(0, 3)
	b.EndRenderPass()
	img := screenshot(t, b, fbo, sz)
	if got := img.RGBAAt(0, 0); got != clearColExpect {
		t.Errorf("got color %v, expected %v", got, clearColExpect)
	}
	cx, cy := 300, 400
	shaderCol := f32color.RGBA{R: .25, G: .55, B: .75, A: 1.0}
	if got, exp := img.RGBAAt(cx, cy), shaderCol.SRGB(); got != f32color.NRGBAToRGBA(exp) {
		t.Errorf("got color %v, expected %v", got, f32color.NRGBAToRGBA(exp))
	}
}

func newShaders(ctx driver.Device, vsrc, fsrc shader.Sources) (vert driver.VertexShader, frag driver.FragmentShader, err error) {
	vert, err = ctx.NewVertexShader(vsrc)
	if err != nil {
		return
	}
	frag, err = ctx.NewFragmentShader(fsrc)
	if err != nil {
		vert.Release()
	}
	return
}

func TestFramebuffers(t *testing.T) {
	b := newDriver(t)
	sz := image.Point{X: 800, Y: 600}
	var (
		col1 = color.NRGBA{R: 0xac, G: 0xbd, B: 0xef, A: 0xde}
		col2 = color.NRGBA{R: 0xfe, G: 0xbb, B: 0xbe, A: 0xca}
	)
	fbo1 := newFBO(t, b, sz)
	fbo2 := newFBO(t, b, sz)
	fcol1, fcol2 := f32color.LinearFromSRGB(col1), f32color.LinearFromSRGB(col2)
	d := driver.LoadDesc{Action: driver.LoadActionClear}
	d.ClearColor = fcol1
	b.BeginRenderPass(fbo1, d)
	b.EndRenderPass()
	d.ClearColor = fcol2
	b.BeginRenderPass(fbo2, d)
	b.EndRenderPass()
	img := screenshot(t, b, fbo1, sz)
	if got := img.RGBAAt(0, 0); got != f32color.NRGBAToRGBA(col1) {
		t.Errorf("got color %v, expected %v", got, f32color.NRGBAToRGBA(col1))
	}
	img = screenshot(t, b, fbo2, sz)
	if got := img.RGBAAt(0, 0); got != f32color.NRGBAToRGBA(col2) {
		t.Errorf("got color %v, expected %v", got, f32color.NRGBAToRGBA(col2))
	}
}

func newFBO(t *testing.T, b driver.Device, size image.Point) driver.Texture {
	fboTex, err := b.NewTexture(
		driver.TextureFormatSRGBA,
		size.X, size.Y,
		driver.FilterNearest, driver.FilterNearest,
		driver.BufferBindingFramebuffer,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fboTex.Release()
	})
	return fboTex
}

func newDriver(t *testing.T) driver.Device {
	ctx, err := newContext()
	if err != nil {
		t.Skipf("no context available: %v", err)
	}
	if err := ctx.MakeCurrent(); err != nil {
		t.Fatal(err)
	}
	b, err := driver.NewDevice(ctx.API())
	if err != nil {
		t.Fatal(err)
	}
	b.BeginFrame(nil, true, image.Pt(1, 1))
	t.Cleanup(func() {
		b.EndFrame()
		b.Release()
		ctx.ReleaseCurrent()
		runtime.UnlockOSThread()
		ctx.Release()
	})
	return b
}

func screenshot(t *testing.T, d driver.Device, fbo driver.Texture, size image.Point) *image.RGBA {
	img := image.NewRGBA(image.Rectangle{Max: size})
	err := driver.DownloadImage(d, fbo, img)
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
	return os.WriteFile(file, buf.Bytes(), 0666)
}
