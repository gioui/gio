// SPDX-License-Identifier: Unlicense OR MIT

package d3d11

import (
	"fmt"
	"image"
	"unsafe"

	"golang.org/x/sys/windows"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/d3d11"
)

// This file implements the primitives that Gio's own renderer does not need,
// but custom rendering passes do: textures in arbitrary DXGI formats, block
// compressed uploads, depth buffers, culling and depth state, and index
// drawing. Everything runs on the device and immediate context the backend
// already owns, so a pass is ordered with respect to Gio's frame without any
// synchronization.

// RawTextureDesc describes a texture for NewRawTexture.
type RawTextureDesc struct {
	Width, Height int
	// MipLevels is 0 for a single level.
	MipLevels int
	// Format is a DXGI_FORMAT_* value.
	Format uint32
	// RenderTarget allocates a render target view for the texture.
	RenderTarget bool
	// Depth allocates a depth stencil view instead of a render target one.
	Depth bool
	// Sampled allocates a shader resource view and a sampler.
	Sampled bool
}

// RawTexture is a texture that a custom pass owns.
type RawTexture struct {
	backend *Backend
	tex     *d3d11.Texture2D
	rtv     *d3d11.RenderTargetView
	dsv     *d3d11.DepthStencilView
	srv     *d3d11.ShaderResourceView
	sampler *d3d11.SamplerState

	format    uint32
	width     int
	height    int
	mipLevels int
}

// RawBuffer is a buffer that a custom pass owns.
type RawBuffer struct {
	backend *Backend
	buf     *d3d11.Buffer
	size    int
	dynamic bool
}

// RawVertexInput describes one vertex attribute.
type RawVertexInput struct {
	// Semantic is the HLSL semantic, for example "POSITION" or "TEXCOORD".
	Semantic string
	// SemanticIndex disambiguates repeated semantics.
	SemanticIndex uint32
	// Format is a DXGI_FORMAT_* value describing the attribute.
	Format uint32
	// Offset is the byte offset of the attribute in the vertex.
	Offset int
}

// RawPipeline is a compiled vertex and pixel shader pair.
type RawPipeline struct {
	backend *Backend
	vs      *d3d11.VertexShader
	ps      *d3d11.PixelShader
	layout  *d3d11.InputLayout
	stride  int
}

// RawState is the rasterizer and depth state of a pass.
type RawState struct {
	// Cull is one of CULL_NONE, CULL_FRONT or CULL_BACK.
	Cull uint32
	// DepthTest enables depth testing against the bound depth buffer.
	DepthTest bool
	// DepthWrite enables depth writes.
	DepthWrite bool
}

// NewRawTexture creates a texture with an explicit DXGI format.
func (b *Backend) NewRawTexture(desc RawTextureDesc) (*RawTexture, error) {
	mipLevels := desc.MipLevels
	if mipLevels <= 0 {
		mipLevels = 1
	}
	bindFlags := uint32(0)
	if desc.Sampled {
		bindFlags |= d3d11.BIND_SHADER_RESOURCE
	}
	if desc.RenderTarget {
		bindFlags |= d3d11.BIND_RENDER_TARGET
	}
	if desc.Depth {
		bindFlags |= d3d11.BIND_DEPTH_STENCIL
	}
	if bindFlags == 0 {
		return nil, fmt.Errorf("d3d11: texture created with no bind flags")
	}
	tex, err := b.dev.CreateTexture2D(&d3d11.TEXTURE2D_DESC{
		Width:     uint32(desc.Width),
		Height:    uint32(desc.Height),
		MipLevels: uint32(mipLevels),
		ArraySize: 1,
		Format:    desc.Format,
		SampleDesc: d3d11.DXGI_SAMPLE_DESC{
			Count:   1,
			Quality: 0,
		},
		BindFlags: bindFlags,
	})
	if err != nil {
		return nil, err
	}
	t := &RawTexture{
		backend:   b,
		tex:       tex,
		format:    desc.Format,
		width:     desc.Width,
		height:    desc.Height,
		mipLevels: mipLevels,
	}
	res := (*d3d11.Resource)(unsafe.Pointer(tex))
	if desc.RenderTarget {
		if t.rtv, err = b.dev.CreateRenderTargetView(res); err != nil {
			t.Release()
			return nil, err
		}
	}
	if desc.Depth {
		if t.dsv, err = b.dev.CreateDepthStencilViewTEX2D(res, &d3d11.DEPTH_STENCIL_VIEW_DESC_TEX2D{
			Format:        desc.Format,
			ViewDimension: d3d11.DSV_DIMENSION_TEXTURE2D,
		}); err != nil {
			t.Release()
			return nil, err
		}
	}
	if desc.Sampled {
		if t.srv, err = b.dev.CreateShaderResourceView(res, unsafe.Pointer(&d3d11.SHADER_RESOURCE_VIEW_DESC_TEX2D{
			SHADER_RESOURCE_VIEW_DESC: d3d11.SHADER_RESOURCE_VIEW_DESC{
				Format:        desc.Format,
				ViewDimension: d3d11.SRV_DIMENSION_TEXTURE2D,
			},
			Texture2D: d3d11.TEX2D_SRV{
				MostDetailedMip: 0,
				MipLevels:       uint32(mipLevels),
			},
		})); err != nil {
			t.Release()
			return nil, err
		}
	}
	return t, nil
}

// MipSize returns the size in bytes of one mip level of the texture.
func (t *RawTexture) MipSize(level int) int {
	w, h := t.width>>uint(level), t.height>>uint(level)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if size, ok := d3d11.BlockSize(t.format, w, h); ok {
		return size
	}
	return w * h * 4
}

// UploadMip copies one mip level of raw, already encoded, data into the
// texture. For block compressed formats data holds the blocks as stored in
// the file, not decoded pixels.
func (t *RawTexture) UploadMip(level int, data []byte) error {
	if level < 0 || level >= t.mipLevels {
		return fmt.Errorf("d3d11: mip level %d out of range [0, %d)", level, t.mipLevels)
	}
	size := t.MipSize(level)
	if len(data) < size {
		return fmt.Errorf("d3d11: mip %d needs %d bytes, got %d", level, size, len(data))
	}
	res := (*d3d11.Resource)(unsafe.Pointer(t.tex))
	w, h := t.width>>uint(level), t.height>>uint(level)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	box := d3d11.BOX{
		Left:   0,
		Top:    0,
		Right:  uint32(w),
		Bottom: uint32(h),
		Front:  0,
		Back:   1,
	}
	t.backend.ctx.UpdateSubresourceMip(res, &box, uint32(level), uint32(t.MipRowSize(level)), uint32(size), data[:size])
	return nil
}

// Clear fills the render target with a color.
func (t *RawTexture) Clear(r, g, b, a float32) {
	if t.rtv == nil {
		return
	}
	color := [4]float32{r, g, b, a}
	t.backend.ctx.ClearRenderTargetView(t.rtv, &color)
}

// ClearDepth resets the depth buffer to value.
func (t *RawTexture) ClearDepth(value float32) {
	if t.dsv == nil {
		return
	}
	t.backend.ctx.ClearDepthStencilView(t.dsv, d3d11.CLEAR_DEPTH, value, 0)
}

// Release frees the texture and its views.
func (t *RawTexture) Release() {
	if t == nil {
		return
	}
	if t.rtv != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(t.rtv), t.rtv.Vtbl.Release)
	}
	if t.dsv != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(t.dsv), t.dsv.Vtbl.Release)
	}
	if t.srv != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(t.srv), t.srv.Vtbl.Release)
	}
	if t.sampler != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(t.sampler), t.sampler.Vtbl.Release)
	}
	if t.tex != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(t.tex), t.tex.Vtbl.Release)
	}
	*t = RawTexture{}
}

// NewRawBuffer creates a buffer with the given D3D11 bind flags. Buffers are
// always created for CPU writes: UpdateSubresource silently does nothing on
// buffers, so the contents go in through a mapping.
func (b *Backend) NewRawBuffer(bind uint32, size int, dynamic bool) (*RawBuffer, error) {
	if size <= 0 {
		return nil, fmt.Errorf("d3d11: buffer size must be positive, got %d", size)
	}
	buf, err := b.dev.CreateBuffer(&d3d11.BUFFER_DESC{
		ByteWidth:      uint32(size),
		Usage:          d3d11.USAGE_DYNAMIC,
		BindFlags:      bind,
		CPUAccessFlags: d3d11.CPU_ACCESS_WRITE,
	}, nil)
	if err != nil {
		return nil, err
	}
	return &RawBuffer{backend: b, buf: buf, size: size, dynamic: dynamic}, nil
}

// Upload copies data into the buffer.
func (b *RawBuffer) Upload(data []byte) error {
	if len(data) > b.size {
		return fmt.Errorf("d3d11: buffer holds %d bytes, got %d", b.size, len(data))
	}
	res := (*d3d11.Resource)(unsafe.Pointer(b.buf))
	mapped, err := b.backend.ctx.Map(res, 0, d3d11.MAP_WRITE_DISCARD, 0)
	if err != nil {
		return err
	}
	defer b.backend.ctx.Unmap(res, 0)
	// A mapped buffer has no row pitch of its own; its size is the size.
	row := int(mapped.RowPitch)
	if row <= 0 || row > b.size {
		row = b.size
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(mapped.PData)), row), data)
	return nil
}

// ReadBytes copies the buffer contents back to the CPU. It is meant for tests
// and diagnostics.
func (b *RawBuffer) ReadBytes() ([]byte, error) {
	staging, err := b.backend.dev.CreateBuffer(&d3d11.BUFFER_DESC{
		ByteWidth:      uint32(b.size),
		Usage:          d3d11.USAGE_STAGING,
		CPUAccessFlags: d3d11.CPU_ACCESS_READ,
	}, nil)
	if err != nil {
		return nil, err
	}
	defer d3d11.IUnknownRelease(unsafe.Pointer(staging), staging.Vtbl.Release)
	b.backend.ctx.CopySubresourceRegion(
		(*d3d11.Resource)(unsafe.Pointer(staging)), 0, 0, 0, 0,
		(*d3d11.Resource)(unsafe.Pointer(b.buf)), 0,
		&d3d11.BOX{Left: 0, Right: uint32(b.size), Top: 0, Bottom: 1, Front: 0, Back: 1},
	)
	mapped, err := b.backend.ctx.Map((*d3d11.Resource)(unsafe.Pointer(staging)), 0, d3d11.MAP_READ, 0)
	if err != nil {
		return nil, err
	}
	defer b.backend.ctx.Unmap((*d3d11.Resource)(unsafe.Pointer(staging)), 0)
	row := int(mapped.RowPitch)
	if row <= 0 || row > b.size {
		row = b.size
	}
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(mapped.PData)), row)...), nil
}

// Release frees the buffer.
func (b *RawBuffer) Release() {
	if b == nil {
		return
	}
	if b.buf != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(b.buf), b.buf.Vtbl.Release)
	}
	*b = RawBuffer{}
}

// NewRawPipeline compiles a vertex and pixel shader from DXBC bytecode.
func (b *Backend) NewRawPipeline(vs, ps []byte, inputs []RawVertexInput, stride int) (*RawPipeline, error) {
	vert, err := b.dev.CreateVertexShader(vs)
	if err != nil {
		return nil, err
	}
	frag, err := b.dev.CreatePixelShader(ps)
	if err != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(vert), vert.Vtbl.Release)
		return nil, err
	}
	descs := make([]d3d11.INPUT_ELEMENT_DESC, len(inputs))
	for i, in := range inputs {
		name, err := windows.BytePtrFromString(in.Semantic)
		if err != nil {
			d3d11.IUnknownRelease(unsafe.Pointer(vert), vert.Vtbl.Release)
			d3d11.IUnknownRelease(unsafe.Pointer(frag), frag.Vtbl.Release)
			return nil, err
		}
		descs[i] = d3d11.INPUT_ELEMENT_DESC{
			SemanticName:      name,
			SemanticIndex:     in.SemanticIndex,
			Format:            in.Format,
			AlignedByteOffset: uint32(in.Offset),
		}
	}
	layout, err := b.dev.CreateInputLayout(descs, vs)
	if err != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(vert), vert.Vtbl.Release)
		d3d11.IUnknownRelease(unsafe.Pointer(frag), frag.Vtbl.Release)
		return nil, err
	}
	return &RawPipeline{backend: b, vs: vert, ps: frag, layout: layout, stride: stride}, nil
}

// Release frees the pipeline.
func (p *RawPipeline) Release() {
	if p == nil {
		return
	}
	if p.layout != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(p.layout), p.layout.Vtbl.Release)
	}
	if p.vs != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(p.vs), p.vs.Vtbl.Release)
	}
	if p.ps != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(p.ps), p.ps.Vtbl.Release)
	}
	*p = RawPipeline{}
}

// BeginRawPass binds the render targets and sets the viewport. Clearing the
// depth buffer to one is implied when depth is bound.
func (b *Backend) BeginRawPass(color, depth *RawTexture, viewport image.Rectangle, clear bool, clearColor [4]float32) {
	var (
		rtv *d3d11.RenderTargetView
		dsv *d3d11.DepthStencilView
	)
	if color != nil {
		rtv = color.rtv
	}
	if depth != nil {
		dsv = depth.dsv
	}
	b.ctx.OMSetRenderTargets(rtv, dsv)
	b.rawViewport = d3d11.VIEWPORT{
		TopLeftX: float32(viewport.Min.X),
		TopLeftY: float32(viewport.Min.Y),
		Width:    float32(viewport.Dx()),
		Height:   float32(viewport.Dy()),
		MinDepth: 0,
		MaxDepth: 1,
	}
	b.ctx.RSSetViewports(&b.rawViewport)
	if clear {
		if rtv != nil {
			c := clearColor
			b.ctx.ClearRenderTargetView(rtv, &c)
		}
		if dsv != nil {
			b.ctx.ClearDepthStencilView(dsv, d3d11.CLEAR_DEPTH, 1, 0)
		}
	}
}

// SetRawState installs the rasterizer and depth state of a pass. The states
// are cached, so this is cheap to call every frame.
func (b *Backend) SetRawState(st RawState) {
	rs, ok := b.rawRasterizers[st.Cull]
	if !ok {
		var err error
		rs, err = b.dev.CreateRasterizerState(&d3d11.RASTERIZER_DESC{
			CullMode: st.Cull,
			FillMode: d3d11.FILL_SOLID,
		})
		if err != nil {
			rs = b.rasterizer
		} else {
			b.rawRasterizers[st.Cull] = rs
		}
	}
	b.ctx.RSSetState(rs)
	var key uint32
	if st.DepthTest {
		key |= 1
	}
	if st.DepthWrite {
		key |= 2
	}
	ds, ok := b.rawDepthStates[key]
	if !ok {
		desc := d3d11.DEPTH_STENCIL_DESC{
			DepthEnable:    boolToUint32(st.DepthTest),
			DepthWriteMask: boolToUint32(st.DepthWrite) * d3d11.DEPTH_WRITE_MASK_ALL,
			DepthFunc:      d3d11.DEPTH_COMPARISON_LESS_EQUAL,
		}
		var err error
		ds, err = b.dev.CreateDepthStencilState(&desc)
		if err != nil {
			ds = nil
		} else {
			b.rawDepthStates[key] = ds
		}
	}
	if ds != nil {
		b.ctx.OMSetDepthStencilState(ds, 0)
	}
}

// SetRawPipeline binds the shaders, input layout and primitive topology.
func (b *Backend) SetRawPipeline(p *RawPipeline) {
	b.rawStride = p.stride
	b.ctx.VSSetShader(p.vs)
	b.ctx.PSSetShader(p.ps)
	b.ctx.IASetInputLayout(p.layout)
	b.ctx.IASetPrimitiveTopology(d3d11.PRIMITIVE_TOPOLOGY_TRIANGLELIST)
}

// SetRawVertexBuffer binds the vertex buffer at the last bound pipeline's
// stride, starting at offset bytes into the buffer.
func (b *Backend) SetRawVertexBuffer(buf *RawBuffer, offset int) {
	b.ctx.IASetVertexBuffers(buf.buf, uint32(b.rawStride), uint32(offset))
}

// SetRawIndexBuffer binds 16 bit indices.
func (b *Backend) SetRawIndexBuffer(buf *RawBuffer) {
	b.ctx.IASetIndexBuffer(buf.buf, d3d11.DXGI_FORMAT_R16_UINT, 0)
}

// SetRawConstantBuffer binds a buffer to the first constant buffer slot of
// both shader stages.
func (b *Backend) SetRawConstantBuffer(buf *RawBuffer) {
	b.ctx.VSSetConstantBuffers(buf.buf)
	b.ctx.PSSetConstantBuffers(buf.buf)
}

// SetRawTexture binds a texture to a shader slot.
func (b *Backend) SetRawTexture(unit int, t *RawTexture) {
	if t == nil || t.srv == nil {
		return
	}
	if t.sampler == nil {
		sampler, err := b.dev.CreateSamplerState(&d3d11.SAMPLER_DESC{
			Filter:        d3d11.FILTER_MIN_MAG_LINEAR_MIP_POINT,
			AddressU:      d3d11.TEXTURE_ADDRESS_CLAMP,
			AddressV:      d3d11.TEXTURE_ADDRESS_CLAMP,
			AddressW:      d3d11.TEXTURE_ADDRESS_CLAMP,
			MaxAnisotropy: 1,
			MinLOD:        -3.4028235e+38,
			MaxLOD:        3.4028235e+38,
		})
		if err != nil {
			return
		}
		t.sampler = sampler
	}
	b.ctx.PSSetSamplers(uint32(unit), t.sampler)
	b.ctx.PSSetShaderResources(uint32(unit), t.srv)
}

// DrawRawIndexed draws count indices from the bound index buffer.
func (b *Backend) DrawRawIndexed(count int) {
	b.ctx.DrawIndexed(uint32(count), 0, 0)
}

// NewExternalTexture adopts a texture created by a custom pass so that Gio can
// sample it. Gio does not take ownership of the underlying resource.
func (b *Backend) NewExternalTexture(src driver.ExternalTexture) (driver.Texture, error) {
	raw, ok := src.Handle.(*RawTexture)
	if !ok {
		return nil, fmt.Errorf("d3d11: unsupported external texture handle %T", src.Handle)
	}
	tex := &Texture{
		backend:  b,
		format:   raw.format,
		tex:      raw.tex,
		bindings: driver.BufferBindingTexture,
		width:    raw.width,
		height:   raw.height,
		external: true,
	}
	// The texture must stay alive for as long as Gio samples it.
	d3d11.IUnknownAddRef(unsafe.Pointer(raw.tex), raw.tex.Vtbl.AddRef)
	if raw.srv != nil {
		d3d11.IUnknownAddRef(unsafe.Pointer(raw.srv), raw.srv.Vtbl.AddRef)
		tex.resView = raw.srv
	} else {
		srv, err := b.dev.CreateShaderResourceView((*d3d11.Resource)(unsafe.Pointer(raw.tex)), unsafe.Pointer(&d3d11.SHADER_RESOURCE_VIEW_DESC_TEX2D{
			SHADER_RESOURCE_VIEW_DESC: d3d11.SHADER_RESOURCE_VIEW_DESC{
				Format:        raw.format,
				ViewDimension: d3d11.SRV_DIMENSION_TEXTURE2D,
			},
			Texture2D: d3d11.TEX2D_SRV{
				MostDetailedMip: 0,
				MipLevels:       uint32(raw.mipLevels),
			},
		}))
		if err != nil {
			d3d11.IUnknownRelease(unsafe.Pointer(tex.tex), tex.tex.Vtbl.Release)
			return nil, err
		}
		tex.resView = srv
	}
	sampler, err := b.dev.CreateSamplerState(&d3d11.SAMPLER_DESC{
		Filter:        d3d11.FILTER_MIN_MAG_MIP_LINEAR,
		AddressU:      d3d11.TEXTURE_ADDRESS_CLAMP,
		AddressV:      d3d11.TEXTURE_ADDRESS_CLAMP,
		AddressW:      d3d11.TEXTURE_ADDRESS_CLAMP,
		MaxAnisotropy: 1,
		MinLOD:        -3.4028235e+38,
		MaxLOD:        3.4028235e+38,
	})
	if err != nil {
		d3d11.IUnknownRelease(unsafe.Pointer(tex.resView), tex.resView.Vtbl.Release)
		d3d11.IUnknownRelease(unsafe.Pointer(tex.tex), tex.tex.Vtbl.Release)
		return nil, err
	}
	tex.sampler = sampler
	return tex, nil
}

func boolToUint32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// Width returns the width of the texture in pixels.
func (t *RawTexture) Width() int { return t.width }

// Height returns the height of the texture in pixels.
func (t *RawTexture) Height() int { return t.height }

// Format returns the DXGI format of the texture.
func (t *RawTexture) Format() uint32 { return t.format }

// ReadPixels copies the top mip level of a texture back to the CPU. It is
// meant for tests and diagnostics, not for per frame use.
func (t *RawTexture) ReadPixels() ([]byte, error) {
	return t.ReadPixelsMip(0)
}

// ReadPixelsMip is ReadPixels for a named mip level, which is how a caller
// checks that a level was written where it said it was.
func (t *RawTexture) ReadPixelsMip(level int) ([]byte, error) {
	if level < 0 || level >= t.mipLevels {
		return nil, fmt.Errorf("d3d11: mip level %d out of range [0, %d)", level, t.mipLevels)
	}
	w := t.width >> uint(level)
	h := t.height >> uint(level)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	staging, err := t.backend.dev.CreateTexture2D(&d3d11.TEXTURE2D_DESC{
		Width:     uint32(w),
		Height:    uint32(h),
		MipLevels: 1,
		ArraySize: 1,
		Format:    t.format,
		SampleDesc: d3d11.DXGI_SAMPLE_DESC{
			Count:   1,
			Quality: 0,
		},
		Usage:          d3d11.USAGE_STAGING,
		CPUAccessFlags: d3d11.CPU_ACCESS_READ,
	})
	if err != nil {
		return nil, err
	}
	defer d3d11.IUnknownRelease(unsafe.Pointer(staging), staging.Vtbl.Release)
	t.backend.ctx.CopySubresourceRegion(
		(*d3d11.Resource)(unsafe.Pointer(staging)), 0, 0, 0, 0,
		(*d3d11.Resource)(unsafe.Pointer(t.tex)), uint32(level),
		&d3d11.BOX{
			Left:   0,
			Top:    0,
			Right:  uint32(w),
			Bottom: uint32(h),
			Front:  0,
			Back:   1,
		},
	)
	mapped, err := t.backend.ctx.Map((*d3d11.Resource)(unsafe.Pointer(staging)), 0, d3d11.MAP_READ, 0)
	if err != nil {
		return nil, err
	}
	defer t.backend.ctx.Unmap((*d3d11.Resource)(unsafe.Pointer(staging)), 0)
	rowBytes := t.MipRowSize(level)
	out := make([]byte, 0, rowBytes*h)
	for y := range h {
		row := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(mapped.PData)+uintptr(y)*uintptr(mapped.RowPitch))), rowBytes)
		out = append(out, row...)
	}
	return out, nil
}

// MipRowSize returns the number of bytes in one row of a mip level, which is
// what the copy and update row pitch arguments expect.
func (t *RawTexture) MipRowSize(level int) int {
	w, h := t.width>>uint(level), t.height>>uint(level)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	if row, ok := d3d11.BlockRowSize(t.format, w); ok {
		return row
	}
	return w * 4
}
