package gpu

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/byteslice"
	"gioui.org/shader"
)

type Call struct {
	F    any
	Refs []any
	Args []uint32
}

type Quad struct {
	Dr      image.Rectangle
	Src     Call
	Pos     image.Point
	Mask    Call
	MaskOff image.Point
	Op      draw.Op
}

type gpu2 struct {
	ctx       driver.Device
	quadVerts struct {
		buf  driver.Buffer
		size int
	}
	viewport driver.Buffer
	pipeline driver.Pipeline
}

func newGPU2(ctx driver.Device) (*gpu2, error) {
	g := &gpu2{ctx: ctx}
	if err := g.init(); err != nil {
		g.Release()
		return nil, err
	}
	return g, nil
}

func (g *gpu2) init() (err error) {
	var res []func()
	defer func() {
		if err != nil {
			for _, r := range res {
				r()
			}
		}
	}()
	const (
		header = `
			struct VertexIn {
				float2 pos [[attribute(0)]];
				uint color [[attribute(1)]];
			};
			struct VertexOut {
			    float4 position [[position]];
			    half4 color;
			};`
		vsrc = header + `
		vertex VertexOut main0(uint vertexID [[vertex_id]],
							 constant float2 &halfInvViewport[[buffer(0)]],
                             VertexIn v [[stage_in]])  {
		    VertexOut out;
		    float2 pos = v.pos*halfInvViewport;
		    pos = float2(pos.x-1., 1.-pos.y);
		    out.position = float4(pos, 0.0, 1.0);
		    out.color = metal::unpack_unorm4x8_srgb_to_half(v.color);
		    return out;
		}`
		fsrc = header + `
		fragment half4 main0(VertexOut in [[stage_in]]) {
		    return in.color;
		}`
	)
	blend := driver.BlendDesc{
		SrcFactor: driver.BlendFactorOne,
		DstFactor: driver.BlendFactorOneMinusSrcAlpha,
	}
	vsh, err := g.ctx.CompileVertexShader(vsrc, []shader.InputLocation{
		{
			Location: 0,
			Type:     shader.DataTypeInt16,
			Size:     2,
		},
		{
			Location: 1,
			Type:     shader.DataTypeUInt32,
			Size:     1,
		},
	})
	if err != nil {
		return err
	}
	defer vsh.Release()
	fsh, err := g.ctx.CompileFragmentShader(fsrc)
	if err != nil {
		return err
	}
	defer fsh.Release()
	layout := driver.VertexLayout{
		Inputs: []driver.InputDesc{
			{Type: shader.DataTypeInt16, Size: 2, Offset: 0},
			{Type: shader.DataTypeUInt32, Size: 1, Offset: 2 * 2},
		},
		Stride: 2*2 + 1*4,
	}
	pipe, err := g.ctx.NewPipeline(driver.PipelineDesc{
		VertexShader:   vsh,
		FragmentShader: fsh,
		BlendDesc:      blend,
		VertexLayout:   layout,
		PixelFormat:    driver.TextureFormatOutput,
		Topology:       driver.TopologyTriangles,
	})
	if err != nil {
		return err
	}
	res = append(res, pipe.Release)
	viewport, err := g.ctx.NewBuffer(driver.BufferBindingUniforms, 4*2)
	if err != nil {
		return err
	}
	res = append(res, viewport.Release)
	g.pipeline = pipe
	g.viewport = viewport
	return nil
}

func (g *gpu2) Frame(viewport image.Point, quads []Quad) error {
	var verts []byte
	fmt.Println("FRAME")
	for i, q := range quads {
		fmt.Printf("%+v\n", q)
		r := q.Dr
		col := color.RGBA{R: 0x20 + uint8(i)*20, G: uint8(i), B: 0xff - uint8(i), A: 0xcc}
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Min.X))
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Min.Y))
		verts = append(verts, col.R, col.G, col.B, col.A)
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Min.X))
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Max.Y))
		verts = append(verts, col.R, col.G, col.B, col.A)
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Max.X))
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Min.Y))
		verts = append(verts, col.R, col.G, col.B, col.A)
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Max.X))
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Min.Y))
		verts = append(verts, col.R, col.G, col.B, col.A)
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Min.X))
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Max.Y))
		verts = append(verts, col.R, col.G, col.B, col.A)
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Max.X))
		verts = binary.NativeEndian.AppendUint16(verts, uint16(r.Max.Y))
		verts = append(verts, col.R, col.G, col.B, col.A)
	}
	if len(verts) == 0 {
		return nil
	}
	if g.quadVerts.size < len(verts) {
		if g.quadVerts.buf != nil {
			g.quadVerts.buf.Release()
		}
		quadVerts, err := g.ctx.NewBuffer(driver.BufferBindingVertices, len(verts))
		g.quadVerts.buf = quadVerts
		if err != nil {
			return err
		}
		g.quadVerts.size = len(verts)
	}
	g.quadVerts.buf.Upload(verts)
	g.viewport.Upload(byteslice.Slice([]float32{
		2. / float32(viewport.X), 2. / float32(viewport.Y),
	}))
	g.ctx.BindPipeline(g.pipeline)
	g.ctx.BindUniforms(g.viewport)
	g.ctx.BindVertexBuffer(g.quadVerts.buf, 0)
	g.ctx.DrawArrays(0, 6*len(quads))
	return nil
}

func (g *gpu2) Release() {
	if g.quadVerts.buf != nil {
		g.quadVerts.buf.Release()
	}
	if g.pipeline != nil {
		g.pipeline.Release()
	}
	*g = gpu2{}
}
