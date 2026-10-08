// SPDX-License-Identifier: Unlicense OR MIT

//go:build windows

package gpu

import (
	"errors"
	"fmt"
	"image"
	"image/color"

	backend "gioui.org/gpu/internal/vulkan"
	"gioui.org/internal/vk"
	"gioui.org/op/paint"
)

// VulkanFormat is a VkFormat value. The numbers are fixed by the Vulkan
// specification, so they are written out the way the Direct3D formats are.
type VulkanFormat = uint32

const (
	VulkanFormatR8G8B8A8SRGB VulkanFormat = 43  // VK_FORMAT_R8G8B8A8_SRGB
	VulkanFormatD32Float     VulkanFormat = 126 // VK_FORMAT_D32_SFLOAT
	VulkanFormatBC1SRGB      VulkanFormat = 134 // VK_FORMAT_BC1_RGBA_SRGB_BLOCK
	VulkanFormatBC2SRGB      VulkanFormat = 136 // VK_FORMAT_BC2_SRGB_BLOCK
	VulkanFormatBC3SRGB      VulkanFormat = 138 // VK_FORMAT_BC3_SRGB_BLOCK

	// The vertex formats a skinned mesh uses.
	VulkanFormatFloat32   VulkanFormat = 100 // VK_FORMAT_R32_SFLOAT
	VulkanFormatFloat32x2 VulkanFormat = 103 // VK_FORMAT_R32G32_SFLOAT
	VulkanFormatFloat32x3 VulkanFormat = 106 // VK_FORMAT_R32G32B32_SFLOAT
	VulkanFormatFloat32x4 VulkanFormat = 109 // VK_FORMAT_R32G32B32A32_SFLOAT
)

// VulkanPass is a handle on the Vulkan backend of a GPU. It draws on the same
// device and queue Gio itself uses, and every pass it records is submitted
// and waited for before it returns, so a texture it produces is complete by
// the time Gio paints it — and can be painted with
// paint.NewImageOpFromTexture without a copy.
type VulkanPass struct {
	b *backend.Backend
	// c is the pass set for the offscreen target's formats, created the
	// first time something needs it.
	c *backend.Custom
}

// Vulkan returns a handle for issuing custom Vulkan passes. It reports an
// error if the GPU is not backed by Vulkan.
func (g *gpu) Vulkan() (*VulkanPass, error) {
	b, ok := g.ctx.(*backend.Backend)
	if !ok {
		return nil, errors.New("gpu: the current backend is not Vulkan")
	}
	return &VulkanPass{b: b}, nil
}

// custom builds the pass set on first use: one pair of render passes for an
// sRGB color target with a 32 bit depth buffer, which is what the offscreen
// target of the 3D viewport is.
func (v *VulkanPass) custom() (*backend.Custom, error) {
	if v.c == nil {
		c, err := v.b.NewCustom(
			vk.Format(VulkanFormatR8G8B8A8SRGB),
			vk.Format(VulkanFormatD32Float),
		)
		if err != nil {
			return nil, err
		}
		v.c = c
	}
	return v.c, nil
}

// Release frees the pass set. Textures, buffers and pipelines are owned by
// the caller and must be released separately.
func (v *VulkanPass) Release() {
	if v == nil || v.c == nil {
		return
	}
	v.c.Release()
	v.c = nil
}

// VulkanTextureDesc describes a texture.
type VulkanTextureDesc struct {
	Width, Height int
	// MipLevels is 1 for a texture without a chain. Levels are uploaded
	// explicitly with UploadMip; none are generated.
	MipLevels int
	Format    VulkanFormat
	// RenderTarget allows drawing into the texture.
	RenderTarget bool
	// Depth makes the texture a depth buffer.
	Depth bool
	// Sampled allows reading the texture in a shader.
	Sampled bool
}

// VulkanTexture is a texture owned by the caller.
type VulkanTexture struct {
	t *backend.CustomTexture
}

// VulkanTexture is a valid paint source: Gio samples it where it is, with no
// copy in between.
var _ paint.Texture = (*VulkanTexture)(nil)

// NewTexture creates a texture.
func (v *VulkanPass) NewTexture(desc VulkanTextureDesc) (*VulkanTexture, error) {
	t, err := v.b.NewCustomTexture(backend.CustomTextureDesc{
		Width:        desc.Width,
		Height:       desc.Height,
		MipLevels:    desc.MipLevels,
		Format:       vk.Format(desc.Format),
		Sampled:      desc.Sampled,
		RenderTarget: desc.RenderTarget,
		Depth:        desc.Depth,
	})
	if err != nil {
		return nil, err
	}
	return &VulkanTexture{t: t}, nil
}

// UploadMip copies raw, already encoded data into one mip level. For block
// compressed formats the data must be the blocks as stored in the file.
func (t *VulkanTexture) UploadMip(level int, data []byte) error {
	if t == nil || t.t == nil {
		return errors.New("gpu: upload into a released texture")
	}
	return t.t.UploadMip(level, data)
}

// UploadDone finishes an upload: it moves the mip chain to the layout a
// sampler reads it in. A texture that skips it cannot be bound to a draw.
func (t *VulkanTexture) UploadDone() error {
	if t == nil || t.t == nil {
		return nil
	}
	return t.t.UploadDone()
}

// Bounds returns the size of the texture in pixels.
func (t *VulkanTexture) Bounds() image.Rectangle {
	if t == nil || t.t == nil {
		return image.Rectangle{}
	}
	return image.Rectangle{Max: t.t.Size()}
}

// ReadPixels copies the texture back to the CPU as tightly packed rows. It
// is meant for tests and diagnostics.
func (t *VulkanTexture) ReadPixels() ([]byte, error) {
	if t == nil || t.t == nil {
		return nil, errors.New("gpu: readback of a released texture")
	}
	return t.t.ReadPixels()
}

// Release frees the texture and its views. Painting a released texture
// panics. A copy Gio adopted stays valid until Gio lets go of it.
func (t *VulkanTexture) Release() {
	if t == nil || t.t == nil {
		return
	}
	t.t.Release()
	t.t = nil
}

// GPUTexture returns the resource Gio should sample.
func (t *VulkanTexture) GPUTexture() any { return t.t }

// ColorModel implements image.Image. The colour model is never used: the
// pixels of a texture are not readable from the CPU.
func (t *VulkanTexture) ColorModel() color.Model { return color.NRGBAModel }

// At implements image.Image. The pixels of a texture are not readable from
// the CPU, so this exists only to satisfy the interface.
func (t *VulkanTexture) At(x, y int) color.Color { return color.NRGBA{} }

// VulkanBuffer is a buffer owned by the caller. Buffers live in host visible
// memory, so an upload is a plain copy into memory the GPU reads directly,
// and the frame that writes a buffer every frame needs no synchronisation of
// its own — the pass that reads it has already finished when the next write
// happens.
type VulkanBuffer struct {
	b *backend.CustomBuffer
}

// NewBuffer creates a buffer of size bytes. The dynamic flag is accepted for
// symmetry with the Direct3D API and ignored: host visible memory is already
// what a dynamic buffer wants.
func (v *VulkanPass) NewBuffer(kind D3D11BufferKind, size int, dynamic bool) (*VulkanBuffer, error) {
	var usage vk.BufferUsageFlags
	switch kind {
	case VertexBuffer:
		usage = vk.BUFFER_USAGE_VERTEX_BUFFER_BIT
	case IndexBuffer:
		usage = vk.BUFFER_USAGE_INDEX_BUFFER_BIT
	case ConstantBuffer:
		usage = vk.BUFFER_USAGE_UNIFORM_BUFFER_BIT
	default:
		return nil, fmt.Errorf("gpu: unknown buffer kind %d", kind)
	}
	b, err := v.b.NewCustomBuffer(usage, size)
	if err != nil {
		return nil, err
	}
	return &VulkanBuffer{b: b}, nil
}

// NewStorageBuffer creates a buffer a compute shader may read and write.
func (v *VulkanPass) NewStorageBuffer(size int) (*VulkanBuffer, error) {
	b, err := v.b.NewCustomBuffer(vk.BUFFER_USAGE_STORAGE_BUFFER_BIT, size)
	if err != nil {
		return nil, err
	}
	return &VulkanBuffer{b: b}, nil
}

// NewVertexStorageBuffer creates a buffer a compute shader may write and a
// graphics pipeline may later read as vertices.
func (v *VulkanPass) NewVertexStorageBuffer(size int) (*VulkanBuffer, error) {
	b, err := v.b.NewCustomBuffer(vk.BUFFER_USAGE_VERTEX_BUFFER_BIT|vk.BUFFER_USAGE_STORAGE_BUFFER_BIT, size)
	if err != nil {
		return nil, err
	}
	return &VulkanBuffer{b: b}, nil
}

// Upload copies data into the buffer.
func (b *VulkanBuffer) Upload(data []byte) error {
	if b == nil || b.b == nil {
		return errors.New("gpu: upload into a released buffer")
	}
	return b.b.Upload(data)
}

// Release frees the buffer.
func (b *VulkanBuffer) Release() {
	if b == nil || b.b == nil {
		return
	}
	b.b.Release()
	b.b = nil
}

// VulkanInput describes one vertex attribute of a pipeline.
type VulkanInput struct {
	Location int
	Format   VulkanFormat
	// Offset is the byte offset of the attribute within a vertex.
	Offset int
}

// VulkanBlendMode is how a pipeline's fragments meet what is already there.
type VulkanBlendMode int

const (
	// VulkanBlendReplace writes the fragment over the target, ignoring
	// what was there: what a frame of geometry wants.
	VulkanBlendReplace VulkanBlendMode = iota
	// VulkanBlendPremultiplied lays a premultiplied fragment over the
	// target, which is what a sprite or a particle is.
	VulkanBlendPremultiplied
	// VulkanBlendAdd brightens the target by the fragment, which is what
	// a glow is.
	VulkanBlendAdd
)

// VulkanPipelineDesc describes a shader pair and the vertex layout it
// expects.
type VulkanPipelineDesc struct {
	// Vertex and Fragment are SPIR-V words, for example the compiled form
	// of a GLSL shader.
	Vertex, Fragment []byte
	// Inputs describes the interleaved vertex attributes.
	Inputs []VulkanInput
	// Stride is the size of one vertex in bytes.
	Stride int
	// Cull selects which windings to discard.
	Cull D3D11CullMode
	// DepthTest tests fragments against the bound depth buffer.
	DepthTest bool
	// DepthWrite writes the depth of drawn fragments.
	DepthWrite bool
	// Blend says how a fragment meets what is already in the target.
	Blend VulkanBlendMode
	// PushSize is the size of the push constant block the shaders
	// declare, or zero when they declare none.
	PushSize int
}

// VulkanPipeline is a compiled shader pair owned by the caller.
type VulkanPipeline struct {
	p *backend.CustomPipeline
}

// VulkanComputePipelineDesc describes a compute shader and the storage buffers
// it reads or writes.
type VulkanComputePipelineDesc struct {
	Compute  []byte
	Storage  []int
	PushSize int
}

// VulkanComputePipeline is a compiled compute shader owned by the caller.
type VulkanComputePipeline struct {
	p *backend.CustomComputePipeline
}

// NewPipeline compiles a pipeline against the offscreen target's formats.
//
// The winding convention deserves a note: front faces are declared clockwise,
// which is what Direct3D calls front, and a projection whose Y the caller
// flips into Vulkan's downward Y reproduces the on screen triangles of a
// Direct3D pipeline exactly — so a mesh culled to look right on Direct3D
// looks the same here.
func (v *VulkanPass) NewPipeline(desc VulkanPipelineDesc) (*VulkanPipeline, error) {
	c, err := v.custom()
	if err != nil {
		return nil, err
	}
	attrs := make([]backend.CustomVertexAttr, len(desc.Inputs))
	for i, in := range desc.Inputs {
		attrs[i] = backend.CustomVertexAttr{
			Location: in.Location,
			Format:   vk.Format(in.Format),
			Offset:   in.Offset,
		}
	}
	cull := vk.CullMode(vk.CULL_MODE_NONE)
	switch desc.Cull {
	case CullFront:
		cull = vk.CULL_MODE_FRONT
	case CullBack:
		cull = vk.CULL_MODE_BACK
	}
	src, dst := vk.BLEND_FACTOR_ONE, vk.BLEND_FACTOR_ZERO
	switch desc.Blend {
	case VulkanBlendPremultiplied:
		src, dst = vk.BLEND_FACTOR_ONE, vk.BLEND_FACTOR_ONE_MINUS_SRC_ALPHA
	case VulkanBlendAdd:
		src, dst = vk.BLEND_FACTOR_ONE, vk.BLEND_FACTOR_ONE
	}
	p, err := c.NewPipeline(backend.CustomPipelineDesc{
		Vertex:     string(desc.Vertex),
		Fragment:   string(desc.Fragment),
		Attrs:      attrs,
		Stride:     desc.Stride,
		Cull:       cull,
		DepthTest:  desc.DepthTest,
		DepthWrite: desc.DepthWrite,
		Blend:      desc.Blend != VulkanBlendReplace,
		SrcFactor:  src,
		DstFactor:  dst,
		PushSize:   desc.PushSize,
	})
	if err != nil {
		return nil, err
	}
	return &VulkanPipeline{p: p}, nil
}

// Release frees the pipeline.
func (p *VulkanPipeline) Release() {
	if p == nil || p.p == nil {
		return
	}
	p.p.Release()
	p.p = nil
}

// NewComputePipeline compiles a compute pipeline.
func (v *VulkanPass) NewComputePipeline(desc VulkanComputePipelineDesc) (*VulkanComputePipeline, error) {
	c, err := v.custom()
	if err != nil {
		return nil, err
	}
	p, err := c.NewComputePipeline(backend.CustomComputeDesc{
		Compute:  string(desc.Compute),
		Storage:  desc.Storage,
		PushSize: desc.PushSize,
	})
	if err != nil {
		return nil, err
	}
	return &VulkanComputePipeline{p: p}, nil
}

// Release frees the compute pipeline.
func (p *VulkanComputePipeline) Release() {
	if p == nil || p.p == nil {
		return
	}
	p.p.Release()
	p.p = nil
}

// BeginPass binds the render targets and clears them when clear is set: the
// color target to clearColor and the depth buffer to one. The clear colour
// is a linear value, which for an sRGB target is encoded on the way in.
// Call EndPass when done; EndPass submits and waits, so the frame is
// finished on the GPU by then.
func (v *VulkanPass) BeginPass(target, depth *VulkanTexture, viewport image.Rectangle, clear bool, clearColor [4]float32) error {
	c, err := v.custom()
	if err != nil {
		return err
	}
	return c.BeginPass(rawVulkanTexture(target), rawVulkanTexture(depth), viewport, clear, clearColor)
}

// EndPass ends the pass, hands the color target back to the layout samplers
// read it in, submits everything the pass recorded and waits for it.
func (v *VulkanPass) EndPass() error {
	if v == nil || v.c == nil {
		return nil
	}
	return v.c.EndPass()
}

// SetPipeline binds a pipeline.
func (v *VulkanPass) SetPipeline(p *VulkanPipeline) error {
	if v == nil || v.c == nil || p == nil || p.p == nil {
		return errors.New("gpu: no pipeline")
	}
	return v.c.SetPipeline(p.p)
}

// PrepareVertexBuffer records a compute-write to vertex-read barrier before
// the next render pass. It is useful when a buffer was written by queued
// compute work that will be consumed by that pass.
func (v *VulkanPass) PrepareVertexBuffer(b *VulkanBuffer) error {
	c, err := v.custom()
	if err != nil {
		return err
	}
	if b == nil || b.b == nil {
		return errors.New("gpu: no vertex buffer")
	}
	return c.PrepareVertexBuffer(b.b)
}

// SetVertexBuffer binds the vertex buffer, starting at offset bytes.
func (v *VulkanPass) SetVertexBuffer(b *VulkanBuffer, offset int) error {
	if v == nil || v.c == nil || b == nil || b.b == nil {
		return errors.New("gpu: no vertex buffer")
	}
	return v.c.SetVertexBuffer(b.b, offset)
}

// SetIndexBuffer binds the index buffer draws read their indices from.
func (v *VulkanPass) SetIndexBuffer(b *VulkanBuffer) error {
	if v == nil || v.c == nil || b == nil || b.b == nil {
		return errors.New("gpu: no index buffer")
	}
	return v.c.SetIndexBuffer(b.b)
}

// SetUniforms selects the buffer the frame's uniforms are read from.
func (v *VulkanPass) SetUniforms(b *VulkanBuffer) error {
	if v == nil || v.c == nil || b == nil || b.b == nil {
		return errors.New("gpu: no uniform buffer")
	}
	return v.c.SetUniforms(b.b)
}

// SetTexture binds a texture to the sampler slot. Only one sampler is
// declared, so the unit is accepted for symmetry and ignored.
func (v *VulkanPass) SetTexture(unit int, t *VulkanTexture) error {
	if v == nil || v.c == nil || t == nil || t.t == nil {
		return errors.New("gpu: no texture")
	}
	return v.c.SetTexture(t.t)
}

// SetDrawConstants pushes the per draw constants of the bound pipeline —
// the shade term, the tint, the mapping — over the ones already there. The
// values stay in effect until the next push, so a run of draws that share
// them pushes once.
func (v *VulkanPass) SetDrawConstants(data []byte) error {
	if v == nil || v.c == nil {
		return errors.New("gpu: no active pass")
	}
	return v.c.PushDraw(data)
}

// SetStorageBuffer binds a buffer for a later compute dispatch.
func (v *VulkanPass) SetStorageBuffer(binding int, b *VulkanBuffer) error {
	c, err := v.custom()
	if err != nil {
		return err
	}
	if b == nil || b.b == nil {
		return errors.New("gpu: no storage buffer")
	}
	return c.SetStorageBuffer(binding, b.b)
}

// DispatchCompute runs a compute shader outside the render pass and waits for
// it before returning, so subsequent draws can consume its output.
func (v *VulkanPass) DispatchCompute(p *VulkanComputePipeline, push []byte, x, y, z int) error {
	c, err := v.custom()
	if err != nil {
		return err
	}
	if p == nil || p.p == nil {
		return errors.New("gpu: no compute pipeline")
	}
	return c.DispatchCompute(p.p, push, x, y, z)
}

// DispatchComputeQueued records compute work to be consumed by the next custom
// render pass, so compute and graphics share one submission and fence.
func (v *VulkanPass) DispatchComputeQueued(p *VulkanComputePipeline, push []byte, x, y, z int) error {
	c, err := v.custom()
	if err != nil {
		return err
	}
	if p == nil || p.p == nil {
		return errors.New("gpu: no compute pipeline")
	}
	return c.DispatchComputeQueued(p.p, push, x, y, z)
}

// DrawIndexed draws count indices from the bound index buffer.
func (v *VulkanPass) DrawIndexed(count int) error {
	if v == nil || v.c == nil {
		return errors.New("gpu: no active pass")
	}
	return v.c.DrawIndexed(count)
}

// Draw draws count vertices straight out of the bound vertex buffer, with no
// index buffer: what a batch of quads or lines wants.
func (v *VulkanPass) Draw(count int) error {
	if v == nil || v.c == nil {
		return errors.New("gpu: no active pass")
	}
	return v.c.Draw(count)
}

func rawVulkanTexture(t *VulkanTexture) *backend.CustomTexture {
	if t == nil {
		return nil
	}
	return t.t
}

// VulkanDevice is implemented by a GPU whose backend is Vulkan. It is used
// to reach the custom drawing API from an app.Window, which owns the GPU.
type VulkanDevice interface {
	Vulkan() (*VulkanPass, error)
}
