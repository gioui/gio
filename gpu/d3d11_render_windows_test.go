// SPDX-License-Identifier: Unlicense OR MIT

package gpu_test

import (
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"testing"

	"gioui.org/gpu"
	"gioui.org/gpu/headless"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

const testVertexShader = `
struct VSIn {
	float4 pos : POSITION;
	float2 uv   : TEXCOORD0;
};

struct VSOut {
	float4 pos : SV_POSITION;
	float2 uv  : TEXCOORD0;
};

VSOut main(VSIn v) {
	VSOut res;
	res.pos = v.pos;
	res.uv  = v.uv;
	return res;
}
`

const testPixelShader = `
Texture2D    tex0  : register(t0);
SamplerState samp0 : register(s0);

float4 main(float4 pos : SV_POSITION, float2 uv : TEXCOORD0) : SV_TARGET {
	return tex0.Sample(samp0, uv);
}
`

// quadrantTexture builds an n by n texture of four solid quadrants, red, green,
// blue and white, laid out the way a texture atlas would be.
func quadrantTexture(n int) []byte {
	px := make([]byte, n*n*4)
	half := n / 2
	for y := range n {
		for x := range n {
			var c [4]byte
			switch {
			case x < half && y < half:
				c = [4]byte{0xff, 0, 0, 0xff}
			case x >= half && y < half:
				c = [4]byte{0, 0xff, 0, 0xff}
			case x < half:
				c = [4]byte{0, 0, 0xff, 0xff}
			default:
				c = [4]byte{0xff, 0xff, 0xff, 0xff}
			}
			copy(px[(y*n+x)*4:], c[:])
		}
	}
	return px
}

func appendFloats(dst []byte, f ...float32) []byte {
	for _, v := range f {
		dst = binary.LittleEndian.AppendUint32(dst, math.Float32bits(v))
	}
	return dst
}

// TestD3D11PassPaintsWithoutCopy renders a textured quad with a custom
// Direct3D 11 pass, paints the result through Gio's normal op stream and
// checks the pixels survived. It exercises shader compilation at runtime,
// offscreen rendering, and handing a GPU resident texture to Gio.
func TestD3D11PassPaintsWithoutCopy(t *testing.T) {
	const size = 64
	win, err := headless.NewWindow(size, size)
	if err != nil {
		t.Skipf("no GPU backend available: %v", err)
	}
	defer win.Release()

	d3d, err := win.D3D11()
	if err != nil {
		t.Skipf("Direct3D 11 is not the backend: %v", err)
	}

	// A source texture, uploaded once and then only sampled.
	src, err := d3d.NewTexture(gpu.D3D11TextureDesc{
		Width:   size,
		Height:  size,
		Format:  gpu.FormatR8G8B8A8UNormSRGB,
		Sampled: true,
	})
	if err != nil {
		t.Fatalf("NewTexture: %v", err)
	}
	defer src.Release()
	if got, want := src.MipSize(0), size*size*4; got != want {
		t.Fatalf("MipSize(0) = %d, want %d", got, want)
	}
	if err := src.UploadMip(0, quadrantTexture(size)); err != nil {
		t.Fatalf("UploadMip: %v", err)
	}
	if err := src.UploadMip(1, nil); err == nil {
		t.Fatal("uploading to a mip level outside the texture should fail")
	}

	// The pass renders into a texture Gio can paint directly.
	target, err := d3d.NewTexture(gpu.D3D11TextureDesc{
		Width:        size,
		Height:       size,
		Format:       gpu.FormatR8G8B8A8UNormSRGB,
		RenderTarget: true,
		Sampled:      true,
	})
	if err != nil {
		t.Fatalf("NewTexture: %v", err)
	}
	defer target.Release()

	vertex, err := d3d.CompileHLSL(testVertexShader, "main", "vs_5_0")
	if err != nil {
		t.Fatalf("CompileHLSL: %v", err)
	}
	fragment, err := d3d.CompileHLSL(testPixelShader, "main", "ps_5_0")
	if err != nil {
		t.Fatalf("CompileHLSL: %v", err)
	}
	pipeline, err := d3d.NewPipeline(gpu.D3D11PipelineDesc{
		Vertex:   vertex,
		Fragment: fragment,
		Stride:   24, // float4 position + float2 uv
		Inputs: []gpu.D3D11Input{
			{Semantic: "POSITION", Format: gpu.VertexFormatFloat32x4, Offset: 0},
			{Semantic: "TEXCOORD", Format: gpu.VertexFormatFloat32x2, Offset: 16},
		},
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	defer pipeline.Release()

	quad := appendFloats(nil,
		-1, -1, 0, 1, 0, 1, // bottom left
		+1, -1, 0, 1, 1, 1, // bottom right
		-1, +1, 0, 1, 0, 0, // top left
		+1, +1, 0, 1, 1, 0, // top right
	)
	vbuf, err := d3d.NewBuffer(gpu.VertexBuffer, len(quad), false)
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	defer vbuf.Release()
	if err := vbuf.Upload(quad); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	// Two triangles: bottom left, bottom right, top left and bottom right,
	// top left, top right. The winding is counter clockwise in NDC.
	indices := []byte{0, 0, 1, 0, 2, 0, 1, 0, 2, 0, 3, 0}
	ibuf, err := d3d.NewBuffer(gpu.IndexBuffer, len(indices), false)
	if err != nil {
		t.Fatalf("NewBuffer: %v", err)
	}
	defer ibuf.Release()
	if err := ibuf.Upload(indices); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	d3d.BeginPass(target, nil, image.Rect(0, 0, size, size), true, color.NRGBA{})
	d3d.SetPipeline(pipeline)
	d3d.SetTexture(0, src)
	d3d.SetVertexBuffer(vbuf, 0)
	d3d.SetIndexBuffer(ibuf)
	d3d.DrawIndexed(6)
	d3d.EndPass()

	// The quad covers the whole viewport, so its four corners carry the four
	// quadrants of the source texture.
	quadrants := []struct {
		name string
		at   image.Point
		want color.RGBA
	}{
		{"top left", image.Pt(size/8, size/8), color.RGBA{0xff, 0, 0, 0xff}},
		{"top right", image.Pt(size-size/8, size/8), color.RGBA{0, 0xff, 0, 0xff}},
		{"bottom left", image.Pt(size/8, size-size/8), color.RGBA{0, 0, 0xff, 0xff}},
		{"bottom right", image.Pt(size-size/8, size-size/8), color.RGBA{0xff, 0xff, 0xff, 0xff}},
	}

	// Check the pass output on its own, before Gio is involved at all.
	px, err := target.ReadPixels()
	if err != nil {
		t.Fatalf("ReadPixels: %v", err)
	}
	for _, tt := range quadrants {
		o := (tt.at.Y*size + tt.at.X) * 4
		got := color.RGBA{px[o], px[o+1], px[o+2], px[o+3]}
		if got != tt.want {
			t.Errorf("pass %s: got %v, want %v", tt.name, got, tt.want)
		}
	}

	// Paint the pass output as an ordinary image, at one to one scale.
	var ops op.Ops
	clip.Rect{Max: image.Pt(size, size)}.Push(&ops)
	paint.NewImageOpFromTexture(target).Add(&ops)
	paint.PaintOp{}.Add(&ops)
	if err := win.Frame(&ops); err != nil {
		t.Fatalf("Frame: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if err := win.Screenshot(img); err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	for _, tt := range quadrants {
		got := img.RGBAAt(tt.at.X, tt.at.Y)
		if got != tt.want {
			t.Errorf("painted %s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

// bc1Solid returns n by n pixels worth of BC1 data for a single RGB565
// colour, repeated over every four by four block.
func bc1Solid(n int, color565 uint16) []byte {
	block := []byte{
		byte(color565), byte(color565 >> 8),
		byte(color565), byte(color565 >> 8),
		0, 0, 0, 0, // every texel picks the first endpoint
	}
	blocks := (n / 4) * (n / 4)
	out := make([]byte, 0, blocks*len(block))
	for range blocks {
		out = append(out, block...)
	}
	return out
}

// TestD3D11BlockCompressedTexture uploads the block compressed data a game
// texture archive holds and samples it without ever decoding it on the CPU.
func TestD3D11BlockCompressedTexture(t *testing.T) {
	const size = 64
	win, err := headless.NewWindow(size, size)
	if err != nil {
		t.Skipf("no GPU backend available: %v", err)
	}
	defer win.Release()
	d3d, err := win.D3D11()
	if err != nil {
		t.Skipf("Direct3D 11 is not the backend: %v", err)
	}

	// 0x7C00 is 15/31 red and 32/63 green in RGB565.
	const color565 = uint16(0x7c00)
	for _, tt := range []struct {
		name   string
		format gpu.D3D11Format
		want   color.RGBA
	}{
		// Read as stored, straight into the render target.
		{"unorm", gpu.FormatBC1UNorm, color.RGBA{0x7b, 0x82, 0, 0xff}},
		// Read through the sRGB decode, so the values land linear in the
		// target.
		{"srgb", gpu.FormatBC1UNormSRGB, color.RGBA{0x33, 0x39, 0, 0xff}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tex, err := d3d.NewTexture(gpu.D3D11TextureDesc{
				Width:     size,
				Height:    size,
				MipLevels: 4,
				Format:    tt.format,
				Sampled:   true,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer tex.Release()
			// Four levels of a 64 by 64 BC1 texture: 256, 64, 16 and 4
			// blocks of eight bytes.
			if got, want := tex.MipSize(0), 256*8; got != want {
				t.Errorf("MipSize(0) = %d, want %d", got, want)
			}
			if got, want := tex.MipSize(3), 4*8; got != want {
				t.Errorf("MipSize(3) = %d, want %d", got, want)
			}
			for level := range 4 {
				n := size >> level
				if n < 4 {
					break
				}
				if err := tex.UploadMip(level, bc1Solid(n, color565)); err != nil {
					t.Fatalf("UploadMip(%d): %v", level, err)
				}
			}

			if got := sampleCentre(t, d3d, tex); !closeRGBA(got, tt.want, 2) {
				t.Errorf("sampled %v, want %v", got, tt.want)
			}
		})
	}
}

// sampleCentre draws a full viewport quad over tex and returns the colour in
// the middle of the result.
func sampleCentre(t *testing.T, d3d *gpu.D3D11, tex *gpu.D3D11Texture) color.RGBA {
	t.Helper()
	const size = 64
	target, err := d3d.NewTexture(gpu.D3D11TextureDesc{
		Width:        size,
		Height:       size,
		Format:       gpu.FormatR8G8B8A8UNorm,
		RenderTarget: true,
		Sampled:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer target.Release()

	vertex, err := d3d.CompileHLSL(testVertexShader, "main", "vs_5_0")
	if err != nil {
		t.Fatal(err)
	}
	fragment, err := d3d.CompileHLSL(testPixelShader, "main", "ps_5_0")
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := d3d.NewPipeline(gpu.D3D11PipelineDesc{
		Vertex:   vertex,
		Fragment: fragment,
		Stride:   24,
		Inputs: []gpu.D3D11Input{
			{Semantic: "POSITION", Format: gpu.VertexFormatFloat32x4, Offset: 0},
			{Semantic: "TEXCOORD", Format: gpu.VertexFormatFloat32x2, Offset: 16},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pipeline.Release()

	quad := appendFloats(nil,
		-1, -1, 0, 1, 0, 1,
		+1, -1, 0, 1, 1, 1,
		-1, +1, 0, 1, 0, 0,
		+1, +1, 0, 1, 1, 0,
	)
	vbuf, err := d3d.NewBuffer(gpu.VertexBuffer, len(quad), false)
	if err != nil {
		t.Fatal(err)
	}
	defer vbuf.Release()
	if err := vbuf.Upload(quad); err != nil {
		t.Fatal(err)
	}
	ibuf, err := d3d.NewBuffer(gpu.IndexBuffer, 12, false)
	if err != nil {
		t.Fatal(err)
	}
	defer ibuf.Release()
	if err := ibuf.Upload([]byte{0, 0, 1, 0, 2, 0, 1, 0, 2, 0, 3, 0}); err != nil {
		t.Fatal(err)
	}

	d3d.BeginPass(target, nil, image.Rect(0, 0, size, size), true, color.NRGBA{})
	d3d.SetPipeline(pipeline)
	d3d.SetTexture(0, tex)
	d3d.SetVertexBuffer(vbuf, 0)
	d3d.SetIndexBuffer(ibuf)
	d3d.DrawIndexed(6)
	d3d.EndPass()

	px, err := target.ReadPixels()
	if err != nil {
		t.Fatal(err)
	}
	o := ((size/2)*size + size/2) * 4
	return color.RGBA{px[o], px[o+1], px[o+2], px[o+3]}
}
