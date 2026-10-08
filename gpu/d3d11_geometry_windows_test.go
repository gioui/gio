// SPDX-License-Identifier: Unlicense OR MIT

package gpu_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/gpu"
	"gioui.org/gpu/headless"
)

// TestD3D11VertexMapping draws one triangle whose texture coordinates are
// known at every vertex and checks where they land. It pins the NDC to
// viewport mapping, which is the convention a custom pass has to get right for
// anything drawn through it to line up with Gio.
func TestD3D11VertexMapping(t *testing.T) {
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

	vs, err := d3d.CompileHLSL(testVertexShader, "main", "vs_5_0")
	if err != nil {
		t.Fatal(err)
	}
	// Write the texture coordinate out as colour, so the readback shows the
	// interpolation directly.
	ps, err := d3d.CompileHLSL(`
float4 main(float4 pos : SV_POSITION, float2 uv : TEXCOORD0) : SV_TARGET {
	return float4(uv, 0, 1);
}`, "main", "ps_5_0")
	if err != nil {
		t.Fatal(err)
	}
	pipeline, err := d3d.NewPipeline(gpu.D3D11PipelineDesc{
		Vertex:   vs,
		Fragment: ps,
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

	// One triangle: bottom left, bottom right, top centre of NDC, with the
	// texture coordinate running from the origin at the bottom left to one at
	// the apex.
	verts := appendFloats(nil,
		-1, -1, 0, 1, 0, 0,
		+1, -1, 0, 1, 1, 0,
		0, +1, 0, 1, 0.5, 1,
	)
	vbuf, err := d3d.NewBuffer(gpu.VertexBuffer, len(verts), false)
	if err != nil {
		t.Fatal(err)
	}
	defer vbuf.Release()
	if err := vbuf.Upload(verts); err != nil {
		t.Fatal(err)
	}
	ibuf, err := d3d.NewBuffer(gpu.IndexBuffer, 6, false)
	if err != nil {
		t.Fatal(err)
	}
	defer ibuf.Release()
	if err := ibuf.Upload([]byte{0, 0, 1, 0, 2, 0}); err != nil {
		t.Fatal(err)
	}

	// Clear to transparent black, so uncovered pixels are easy to tell apart.
	d3d.BeginPass(target, nil, image.Rect(0, 0, size, size), true, color.NRGBA{})
	d3d.SetPipeline(pipeline)
	d3d.SetVertexBuffer(vbuf, 0)
	d3d.SetIndexBuffer(ibuf)
	d3d.DrawIndexed(3)
	d3d.EndPass()

	px, err := target.ReadPixels()
	if err != nil {
		t.Fatal(err)
	}
	at := func(p image.Point) color.RGBA {
		o := (p.Y*size + p.X) * 4
		return color.RGBA{px[o], px[o+1], px[o+2], px[o+3]}
	}
	// The bottom edge of the viewport is the texture's v of zero, so the
	// triangle is upside down relative to the image: a custom pass drawing a
	// texture has to flip v itself.
	for _, tt := range []struct {
		name string
		at   image.Point
		want color.RGBA
	}{
		{"bottom left", image.Pt(0, size-1), color.RGBA{0, 0, 0, 0xff}},
		{"bottom right", image.Pt(size-1, size-1), color.RGBA{0xff, 0, 0, 0xff}},
		// Two rows down from the apex the v coordinate is 0.96, not one:
		// the pixel centre at 2.5 of 64 rows maps to that in NDC.
		{"apex", image.Pt(size/2, 2), color.RGBA{0x80, 0xf5, 0, 0xff}},
	} {
		if got := at(tt.at); !closeRGBA(got, tt.want, 2) {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
	// The top left corner is outside the triangle, so the clear shows through.
	if got := at(image.Pt(0, 0)); got.A != 0 {
		t.Errorf("top left should be uncovered, got %v", got)
	}
}

func closeRGBA(got, want color.RGBA, tol int) bool {
	delta := [4]int{
		int(got.R) - int(want.R),
		int(got.G) - int(want.G),
		int(got.B) - int(want.B),
		int(got.A) - int(want.A),
	}
	for _, d := range delta {
		if d < -tol || d > tol {
			return false
		}
	}
	return true
}
