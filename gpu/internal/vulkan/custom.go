// SPDX-License-Identifier: Unlicense OR MIT

//go:build windows

package vulkan

import (
	"fmt"
	"image"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/vk"
)

// A custom pass records work on the same device and queue Gio uses. Render
// passes submit and wait before returning so Gio can sample their result;
// queued compute may share that render submission when its output is consumed
// by the immediately following custom render pass.
//
// Everything here runs on the goroutine that lays out and submits Gio's
// frames, never inside a Gio frame.

// Custom owns the render passes, the descriptor pool and the pass state of
// custom rendering against one backend. One Custom serves one fixed pair of
// color and depth formats — the offscreen target the 3D viewport draws into.
type Custom struct {
	backend     *Backend
	colorFormat vk.Format
	depthFormat vk.Format
	// passClear clears its attachments on every begin; passLoad keeps what
	// is already in the color target. Both are compatible with every
	// pipeline and framebuffer built here, because compatibility ignores
	// load ops.
	passClear vk.RenderPass
	passLoad  vk.RenderPass

	// pool hands out descriptor sets to pipelines. When it runs out a new
	// one replaces it and the old is destroyed at the next frame boundary,
	// which is after every command buffer that used its sets has finished.
	pool     vk.DescriptorPool
	poolSets []vk.DescriptorSet

	// The pass being recorded and the state bound in it.
	active        bool
	computeQueued bool
	color         *CustomTexture
	depth         *CustomTexture
	pipe          *CustomPipeline
	ubo           *CustomBuffer
	tex           *CustomTexture
	storage       [customStorageUnits]*CustomBuffer
}

const customStorageUnits = 8

// NewCustom prepares a custom pass set that renders into colorFormat with an
// optional depthFormat, and fails when the device cannot do either.
func (b *Backend) NewCustom(colorFormat, depthFormat vk.Format) (*Custom, error) {
	props := vk.GetPhysicalDeviceFormatProperties(b.physDev, colorFormat)
	const needColor = vk.FORMAT_FEATURE_COLOR_ATTACHMENT_BIT | vk.FORMAT_FEATURE_SAMPLED_IMAGE_BIT
	if props&needColor != needColor {
		return nil, fmt.Errorf("vulkan: format %d cannot be a sampled color attachment", colorFormat)
	}
	if depthFormat != 0 {
		props := vk.GetPhysicalDeviceFormatProperties(b.physDev, depthFormat)
		if props&vk.FORMAT_FEATURE_DEPTH_STENCIL_ATTACHMENT_BIT == 0 {
			return nil, fmt.Errorf("vulkan: format %d cannot be a depth attachment", depthFormat)
		}
	}
	newPass := func(load vk.AttachmentLoadOp) (vk.RenderPass, error) {
		return vk.CreateRenderPassEx(b.dev, vk.RenderPassDesc{
			ColorFormat: colorFormat,
			DepthFormat: depthFormat,
			LoadOp:      load,
			// The pass never transitions the color attachment: the
			// caller barriers in and out around it, so what a barrier
			// leaves is what the pass finds and keeps.
			InitialLayout: vk.IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
			FinalLayout:   vk.IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
		})
	}
	clearPass, err := newPass(vk.ATTACHMENT_LOAD_OP_CLEAR)
	if err != nil {
		return nil, mapErr(err)
	}
	loadPass, err := newPass(vk.ATTACHMENT_LOAD_OP_LOAD)
	if err != nil {
		vk.DestroyRenderPass(b.dev, clearPass)
		return nil, mapErr(err)
	}
	return &Custom{
		backend:     b,
		colorFormat: colorFormat,
		depthFormat: depthFormat,
		passClear:   clearPass,
		passLoad:    loadPass,
	}, nil
}

// Release frees the render passes and any descriptor pool the custom pass set
// accumulated. Textures, buffers and pipelines are owned by whoever created
// them and must be released separately.
func (c *Custom) Release() {
	if c == nil {
		return
	}
	b := c.backend
	if b != nil && b.dev != nil {
		clearPass, loadPass, pool := c.passClear, c.passLoad, c.pool
		b.deferFunc(func(d vk.Device) {
			if clearPass != 0 {
				vk.DestroyRenderPass(d, clearPass)
			}
			if loadPass != 0 {
				vk.DestroyRenderPass(d, loadPass)
			}
			if pool != 0 {
				vk.DestroyDescriptorPool(d, pool)
			}
		})
	}
	c.passClear, c.passLoad, c.pool = 0, 0, 0
	c.poolSets = nil
	c.active = false
	c.computeQueued = false
	if c.backend != nil {
		c.backend.customActive = false
	}
	c.color, c.depth, c.pipe, c.ubo, c.tex = nil, nil, nil, nil, nil
}

// submitAndWait ends the command buffer being recorded, submits it on Gio's
// queue and waits for it. Custom passes call it when they are done: nothing
// they produce may be read before it returns.
func (b *Backend) submitAndWait() error {
	if b.cmdPool.current == nil {
		return nil
	}
	b.submitCmdBuf(b.fence)
	if err := vk.WaitForFences(b.dev, b.fence); err != nil {
		return err
	}
	if err := vk.ResetFences(b.dev, b.fence); err != nil {
		return err
	}
	return nil
}

// CustomTextureDesc describes a texture a custom pass owns.
type CustomTextureDesc struct {
	Width, Height int
	// MipLevels is 1 for a texture without a chain. Levels are uploaded
	// explicitly with UploadMip; nothing is generated.
	MipLevels int
	Format    vk.Format
	// Sampled makes the texture readable in a shader and gives it a
	// sampler; the descriptor set a draw binds requires one.
	Sampled bool
	// RenderTarget allows drawing into the texture.
	RenderTarget bool
	// Depth makes it a depth buffer: depth aspect, depth usage, no sampler.
	Depth bool
}

// CustomTexture is a texture a custom pass owns. Its layout is tracked so
// every transition it needs is an explicit barrier, both inside the pass and
// around it.
type CustomTexture struct {
	backend *Backend
	img     vk.Image
	mem     vk.DeviceMemory
	view    vk.ImageView
	sampler vk.Sampler
	format  vk.Format
	aspect  uint32
	width   int
	height  int
	mipmaps int
	depth   bool

	// fbo is the framebuffer of the last pass this texture was a color
	// target of, remembered for the next one. fboDepth is the depth view
	// it was built with, so a new depth companion rebuilds it.
	fbo       vk.Framebuffer
	fboDepth  vk.ImageView
	fboWidth  int
	fboHeight int

	layout vk.ImageLayout
	scope  struct {
		stage  vk.PipelineStageFlags
		access vk.AccessFlags
	}
	// uploading marks a mip chain that has been copied but not yet moved
	// to the layout a sampler reads.
	uploading bool

	// Gio can adopt a rendered texture to paint it. extRefs counts the
	// adoptions still alive, released says the owner let go, and the
	// image dies when both have happened — in whichever order.
	extRefs  int
	released bool
}

// NewCustomTexture creates a texture.
func (b *Backend) NewCustomTexture(desc CustomTextureDesc) (*CustomTexture, error) {
	if desc.Width < 1 || desc.Height < 1 {
		return nil, fmt.Errorf("vulkan: texture is %dx%d", desc.Width, desc.Height)
	}
	mips := desc.MipLevels
	if mips < 1 {
		mips = 1
	}
	usage := vk.IMAGE_USAGE_TRANSFER_DST_BIT
	aspect := vk.IMAGE_ASPECT_COLOR_BIT
	if !desc.Depth {
		usage |= vk.IMAGE_USAGE_TRANSFER_SRC_BIT
	}
	if desc.Sampled {
		usage |= vk.IMAGE_USAGE_SAMPLED_BIT
	}
	if desc.RenderTarget {
		usage |= vk.IMAGE_USAGE_COLOR_ATTACHMENT_BIT
	}
	if desc.Depth {
		usage |= vk.IMAGE_USAGE_DEPTH_STENCIL_ATTACHMENT_BIT
		aspect = vk.IMAGE_ASPECT_DEPTH_BIT
	}
	img, mem, err := vk.CreateImage(b.physDev, b.dev, desc.Format, desc.Width, desc.Height, mips, usage)
	if err != nil {
		return nil, mapErr(err)
	}
	view, err := vk.CreateImageViewEx(b.dev, img, desc.Format, aspect)
	if err != nil {
		vk.DestroyImage(b.dev, img)
		vk.FreeMemory(b.dev, mem)
		return nil, mapErr(err)
	}
	var sampler vk.Sampler
	if desc.Sampled && !desc.Depth {
		sampler, err = vk.CreateSampler(b.dev, vk.FILTER_LINEAR, vk.FILTER_LINEAR, vk.SAMPLER_MIPMAP_MODE_LINEAR)
		if err != nil {
			vk.DestroyImageView(b.dev, view)
			vk.DestroyImage(b.dev, img)
			vk.FreeMemory(b.dev, mem)
			return nil, mapErr(err)
		}
	}
	return &CustomTexture{
		backend: b,
		img:     img,
		mem:     mem,
		view:    view,
		sampler: sampler,
		format:  desc.Format,
		aspect:  aspect,
		width:   desc.Width,
		height:  desc.Height,
		mipmaps: mips,
		depth:   desc.Depth,
		layout:  vk.IMAGE_LAYOUT_UNDEFINED,
	}, nil
}

// Size returns the size of the top mip level in pixels.
func (t *CustomTexture) Size() image.Point {
	return image.Pt(t.width, t.height)
}

// imageBarrier moves the whole image to layout and records the barrier that
// orders it after whatever the texture was used for before. The first barrier
// of a fresh image starts from UNDEFINED with no source scope.
func (t *CustomTexture) imageBarrier(cmd vk.CommandBuffer, layout vk.ImageLayout, stage vk.PipelineStageFlags, access vk.AccessFlags) {
	srcStage := t.scope.stage
	if srcStage == 0 && t.layout == layout {
		t.scope.stage = stage
		t.scope.access = access
		return
	}
	if srcStage == 0 {
		srcStage = vk.PIPELINE_STAGE_TOP_OF_PIPE_BIT
	}
	b := vk.BuildImageMemoryBarrierEx(
		t.img,
		t.scope.access, access,
		t.layout, layout,
		0, vk.REMAINING_MIP_LEVELS,
		t.aspect,
	)
	vk.CmdPipelineBarrier(cmd, srcStage, stage, vk.DEPENDENCY_BY_REGION_BIT, nil, nil, []vk.ImageMemoryBarrier{b})
	t.layout = layout
	t.scope.stage = stage
	t.scope.access = access
}

// blockBytes is the size of one compressed texel block, or zero when the
// format stores four bytes a pixel.
func blockBytes(format vk.Format) int {
	switch format {
	case vk.FORMAT_BC1_RGBA_SRGB:
		return 8
	case vk.FORMAT_BC2_SRGB, vk.FORMAT_BC3_SRGB:
		return 16
	}
	return 0
}

// mipDims is the size of one mip level, never below one pixel.
func (t *CustomTexture) mipDims(level int) (int, int) {
	w, h := t.width>>uint(level), t.height>>uint(level)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

// MipSize is the number of bytes one mip level occupies. A compressed level
// counts blocks the way the .rsh files store them: a level never drops below
// one block.
func (t *CustomTexture) MipSize(level int) int {
	w, h := t.mipDims(level)
	if block := blockBytes(t.format); block > 0 {
		return ((w + 3) / 4) * ((h + 3) / 4) * block
	}
	return w * h * 4
}

// uploadAllowed reports whether a transfer may be recorded right now. Both
// a custom pass and a Gio frame have a command buffer mid recording, and
// recording a transfer into it — then submitting it — ends the frame or the
// pass early and leaves vkCmdEndRenderPass without a recording buffer to
// end. Validation reports that as an invalid command buffer; a driver
// without validation crashes instead, so the mistake is an error here.
func (b *Backend) uploadAllowed() error {
	if b.customActive {
		return fmt.Errorf("vulkan: texture upload while a custom pass is active")
	}
	if b.cmdPool.current != nil {
		return fmt.Errorf("vulkan: texture upload while a command buffer is in flight")
	}
	return nil
}

// UploadMip copies raw, already encoded data into one mip level. For a block
// compressed texture the data is the blocks as the file stores them. The
// submission that carries the copy also waits, so a caller may upload the
// levels one at a time; call UploadDone once the chain is complete.
func (t *CustomTexture) UploadMip(level int, data []byte) error {
	if t.depth {
		return fmt.Errorf("vulkan: cannot upload into a depth texture")
	}
	if err := t.backend.uploadAllowed(); err != nil {
		return err
	}
	if level < 0 || level >= t.mipmaps {
		return fmt.Errorf("vulkan: mip level %d out of range [0, %d)", level, t.mipmaps)
	}
	w, h := t.mipDims(level)
	size := t.MipSize(level)
	if len(data) < size {
		return fmt.Errorf("vulkan: mip %d needs %d bytes, got %d", level, size, len(data))
	}
	b := t.backend
	stage, mem, off := b.stagingBuffer(size)
	copy(mem, data[:size])
	cmd := b.ensureCmdBuf()
	t.imageBarrier(cmd, vk.IMAGE_LAYOUT_TRANSFER_DST_OPTIMAL, vk.PIPELINE_STAGE_TRANSFER_BIT, vk.ACCESS_TRANSFER_WRITE_BIT)
	// BufferRowLength 0 means tightly packed rows, which is how a chain
	// split out of a file is laid out for every level but the top ones —
	// and at the image edge, where a compressed level is smaller than a
	// block, the copy covers the partial block that is there.
	op := vk.BuildBufferImageCopyMip(off, 0, 0, 0, w, h, level)
	vk.CmdCopyBufferToImage(cmd, stage.buf, t.img, t.layout, op)
	t.uploading = true
	return b.submitAndWait()
}

// UploadDone moves an uploaded mip chain to the layout a sampler reads. A
// texture that skips it cannot be bound to a draw, which is the check that
// catches the mistake.
func (t *CustomTexture) UploadDone() error {
	if !t.uploading {
		return nil
	}
	b := t.backend
	if err := b.uploadAllowed(); err != nil {
		return err
	}
	cmd := b.ensureCmdBuf()
	t.imageBarrier(cmd, vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, vk.PIPELINE_STAGE_FRAGMENT_SHADER_BIT, vk.ACCESS_SHADER_READ_BIT)
	t.uploading = false
	return b.submitAndWait()
}

// ReadPixels copies the texture back to the CPU as tightly packed RGBA rows,
// for tests and diagnostics. The image ends the call in the layout it started
// in, so a texture already adopted by Gio can still be painted afterwards.
func (t *CustomTexture) ReadPixels() ([]byte, error) {
	if t.depth {
		return nil, fmt.Errorf("vulkan: cannot read a depth texture back")
	}
	b := t.backend
	if err := b.uploadAllowed(); err != nil {
		return nil, err
	}
	out := make([]byte, t.width*t.height*4)
	stage, mem, off := b.stagingBuffer(len(out))
	cmd := b.ensureCmdBuf()
	t.imageBarrier(cmd, vk.IMAGE_LAYOUT_TRANSFER_SRC_OPTIMAL, vk.PIPELINE_STAGE_TRANSFER_BIT, vk.ACCESS_TRANSFER_READ_BIT)
	region := vk.BuildBufferImageCopyMip(off, t.width, 0, 0, t.width, t.height, 0)
	vk.CmdCopyImageToBuffer(cmd, t.img, t.layout, stage.buf, []vk.BufferImageCopy{region})
	t.imageBarrier(cmd, vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL, vk.PIPELINE_STAGE_FRAGMENT_SHADER_BIT, vk.ACCESS_SHADER_READ_BIT)
	stage.scope.stage = vk.PIPELINE_STAGE_TRANSFER_BIT
	stage.scope.access = vk.ACCESS_TRANSFER_WRITE_BIT
	stage.barrier(cmd, vk.PIPELINE_STAGE_HOST_BIT, vk.ACCESS_HOST_READ_BIT)
	if err := b.submitAndWait(); err != nil {
		return nil, err
	}
	copy(out, mem)
	return out, nil
}

// Release gives the texture up. While Gio still paints an adopted copy of it
// the image stays alive, and dies with the last copy that lets go.
func (t *CustomTexture) Release() {
	if t == nil || t.released {
		return
	}
	t.released = true
	t.destroyIfDone()
}

// unref is what an adopted copy calls when Gio drops it.
func (t *CustomTexture) unref() {
	if t.extRefs > 0 {
		t.extRefs--
	}
	t.destroyIfDone()
}

func (t *CustomTexture) destroyIfDone() {
	if !t.released || t.extRefs > 0 || t.img == 0 {
		return
	}
	b := t.backend
	old := *t
	*t = CustomTexture{}
	if b == nil || b.dev == nil {
		return
	}
	b.deferFunc(func(d vk.Device) {
		if old.fbo != 0 {
			vk.DestroyFramebuffer(d, old.fbo)
		}
		if old.sampler != 0 {
			vk.DestroySampler(d, old.sampler)
		}
		vk.DestroyImageView(d, old.view)
		vk.DestroyImage(d, old.img)
		vk.FreeMemory(d, old.mem)
	})
}

// CustomBuffer is a host visible buffer a custom pass reads from or writes
// into. It stays mapped for as long as it lives, so uploading is a copy and
// a fence the caller already waited for, not a barrier of its own.
type CustomBuffer struct {
	backend *Backend
	buf     vk.Buffer
	mem     vk.DeviceMemory
	mapped  []byte
	size    int
	scope   struct {
		stage  vk.PipelineStageFlags
		access vk.AccessFlags
	}
}

// NewCustomBuffer creates a buffer of size bytes in host visible memory.
func (b *Backend) NewCustomBuffer(usage vk.BufferUsageFlags, size int) (*CustomBuffer, error) {
	if size < 1 {
		size = 1
	}
	buf, mem, err := vk.CreateBuffer(b.physDev, b.dev, size, usage,
		vk.MEMORY_PROPERTY_HOST_VISIBLE_BIT|vk.MEMORY_PROPERTY_HOST_COHERENT_BIT)
	if err != nil {
		return nil, mapErr(err)
	}
	mapped, err := vk.MapMemory(b.dev, mem, 0, size)
	if err != nil {
		vk.DestroyBuffer(b.dev, buf)
		vk.FreeMemory(b.dev, mem)
		return nil, mapErr(err)
	}
	return &CustomBuffer{backend: b, buf: buf, mem: mem, mapped: mapped, size: size}, nil
}

// Upload copies data into the buffer.
func (c *CustomBuffer) Upload(data []byte) error {
	if len(data) > c.size {
		return fmt.Errorf("vulkan: buffer holds %d bytes, got %d", c.size, len(data))
	}
	copy(c.mapped, data)
	c.scope.stage = vk.PIPELINE_STAGE_HOST_BIT
	c.scope.access = vk.ACCESS_HOST_WRITE_BIT
	return nil
}

func (c *CustomBuffer) barrier(cmd vk.CommandBuffer, stage vk.PipelineStageFlags, access vk.AccessFlags) {
	srcStage := c.scope.stage
	if srcStage == 0 {
		srcStage = vk.PIPELINE_STAGE_TOP_OF_PIPE_BIT
	}
	if c.scope.stage == stage && c.scope.access == access {
		return
	}
	b := vk.BuildBufferMemoryBarrier(c.buf, c.scope.access, access)
	vk.CmdPipelineBarrier(cmd, srcStage, stage, 0, nil, []vk.BufferMemoryBarrier{b}, nil)
	c.scope.stage = stage
	c.scope.access = access
}

// Release frees the buffer.
func (c *CustomBuffer) Release() {
	if c == nil || c.buf == 0 {
		return
	}
	b := c.backend
	old := *c
	*c = CustomBuffer{}
	if b == nil || b.dev == nil {
		return
	}
	b.deferFunc(func(d vk.Device) {
		vk.UnmapMemory(d, old.mem)
		vk.DestroyBuffer(d, old.buf)
		vk.FreeMemory(d, old.mem)
	})
}

// CustomVertexAttr is one vertex attribute of a custom pipeline.
type CustomVertexAttr struct {
	Location int
	Format   vk.Format
	Offset   int
}

// CustomPipelineDesc describes a shader pair and the vertex layout it expects.
type CustomPipelineDesc struct {
	// Vertex and Fragment are SPIR-V words.
	Vertex, Fragment string
	Attrs            []CustomVertexAttr
	// Stride is the size of one vertex in bytes.
	Stride int
	// Cull selects which windings are discarded.
	Cull vk.CullMode
	// DepthTest and DepthWrite apply when the pass has a depth attachment.
	DepthTest  bool
	DepthWrite bool
	// Blend enables the colour blend state, and the two factors are what it
	// is: BLEND_FACTOR_ONE with ONE_MINUS_SRC_ALPHA for premultiplied
	// "over", and ONE with ONE for additive. A pipeline with Blend off
	// replaces what is under it, which is what a frame of a model wants.
	Blend     bool
	SrcFactor vk.BlendFactor
	DstFactor vk.BlendFactor
	// PushSize is the size of the push constant block the shaders declare,
	// or zero when they declare none.
	PushSize int
}

// CustomPipeline is a compiled shader pair owned by the caller, with the
// descriptor set layout its shaders read uniforms and a texture through.
type CustomPipeline struct {
	c         *Custom
	pipe      vk.Pipeline
	layout    vk.PipelineLayout
	setLayout vk.DescriptorSetLayout
	pushSize  int
	// sets caches one descriptor set per uniform buffer and texture pair,
	// because a set's contents are read when the draw executes, not when
	// it is recorded: updating one set between two draws of a frame would
	// give both draws the second texture.
	sets map[descKey]vk.DescriptorSet
}

// CustomComputeDesc describes a compute shader and the storage buffers it
// reads or writes.
type CustomComputeDesc struct {
	Compute  string
	Storage  []int
	PushSize int
}

// CustomComputePipeline is a compiled compute shader owned by the caller.
type CustomComputePipeline struct {
	c         *Custom
	pipe      vk.Pipeline
	layout    vk.PipelineLayout
	setLayout vk.DescriptorSetLayout
	pushSize  int
	storage   []int
	sets      map[storageDescKey]vk.DescriptorSet
}

type storageDescKey struct {
	bufs [customStorageUnits]*CustomBuffer
}

// NewComputePipeline compiles a compute pipeline that reads and writes custom
// storage buffers on the same device and queue as the render pass.
func (c *Custom) NewComputePipeline(desc CustomComputeDesc) (*CustomComputePipeline, error) {
	b := c.backend
	mod, err := vk.CreateShaderModule(b.dev, desc.Compute)
	if err != nil {
		return nil, mapErr(err)
	}
	defer vk.DestroyShaderModule(b.dev, mod)

	binds := make([]vk.DescriptorSetLayoutBinding, len(desc.Storage))
	for i, binding := range desc.Storage {
		if binding < 0 || binding >= customStorageUnits {
			return nil, fmt.Errorf("vulkan: storage binding %d out of range [0, %d)", binding, customStorageUnits)
		}
		binds[i] = vk.DescriptorSetLayoutBinding{
			Binding:        binding,
			DescriptorType: vk.DESCRIPTOR_TYPE_STORAGE_BUFFER,
			StageFlags:     vk.SHADER_STAGE_COMPUTE_BIT,
		}
	}
	setLayout, err := vk.CreateDescriptorSetLayout(b.dev, binds)
	if err != nil {
		return nil, mapErr(err)
	}
	var ranges []vk.PushConstantRange
	if desc.PushSize > 0 {
		ranges = append(ranges, vk.BuildPushConstantRange(vk.SHADER_STAGE_COMPUTE_BIT, 0, desc.PushSize))
	}
	layout, err := vk.CreatePipelineLayout(b.dev, ranges, []vk.DescriptorSetLayout{setLayout})
	if err != nil {
		vk.DestroyDescriptorSetLayout(b.dev, setLayout)
		return nil, mapErr(err)
	}
	pipe, err := vk.CreateComputePipeline(b.dev, mod, layout)
	if err != nil {
		vk.DestroyPipelineLayout(b.dev, layout)
		vk.DestroyDescriptorSetLayout(b.dev, setLayout)
		return nil, mapErr(err)
	}
	return &CustomComputePipeline{
		c:         c,
		pipe:      pipe,
		layout:    layout,
		setLayout: setLayout,
		pushSize:  desc.PushSize,
		storage:   append([]int(nil), desc.Storage...),
		sets:      make(map[storageDescKey]vk.DescriptorSet),
	}, nil
}

// Release frees the compute pipeline and its layouts. Descriptor sets live in
// the pass set's pool and go with it.
func (p *CustomComputePipeline) Release() {
	if p == nil || p.pipe == 0 {
		return
	}
	old := *p
	*p = CustomComputePipeline{}
	b := old.c.backend
	if b == nil || b.dev == nil {
		return
	}
	b.deferFunc(func(d vk.Device) {
		vk.DestroyPipeline(d, old.pipe)
		vk.DestroyPipelineLayout(d, old.layout)
		vk.DestroyDescriptorSetLayout(d, old.setLayout)
	})
}

type descKey struct {
	ubo *CustomBuffer
	tex *CustomTexture
}

// NewPipeline compiles a pipeline against the pass set's formats.
func (c *Custom) NewPipeline(desc CustomPipelineDesc) (*CustomPipeline, error) {
	b := c.backend
	vmod, err := vk.CreateShaderModule(b.dev, desc.Vertex)
	if err != nil {
		return nil, mapErr(err)
	}
	defer vk.DestroyShaderModule(b.dev, vmod)
	fmod, err := vk.CreateShaderModule(b.dev, desc.Fragment)
	if err != nil {
		return nil, mapErr(err)
	}
	defer vk.DestroyShaderModule(b.dev, fmod)

	setLayout, err := vk.CreateDescriptorSetLayout(b.dev, []vk.DescriptorSetLayoutBinding{
		{
			Binding:        0,
			DescriptorType: vk.DESCRIPTOR_TYPE_UNIFORM_BUFFER,
			StageFlags:     vk.SHADER_STAGE_VERTEX_BIT | vk.SHADER_STAGE_FRAGMENT_BIT,
		},
		{
			Binding:        1,
			DescriptorType: vk.DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER,
			StageFlags:     vk.SHADER_STAGE_FRAGMENT_BIT,
		},
	})
	if err != nil {
		return nil, mapErr(err)
	}
	var ranges []vk.PushConstantRange
	if desc.PushSize > 0 {
		ranges = append(ranges, vk.BuildPushConstantRange(
			vk.SHADER_STAGE_VERTEX_BIT|vk.SHADER_STAGE_FRAGMENT_BIT, 0, desc.PushSize))
	}
	layout, err := vk.CreatePipelineLayout(b.dev, ranges, []vk.DescriptorSetLayout{setLayout})
	if err != nil {
		vk.DestroyDescriptorSetLayout(b.dev, setLayout)
		return nil, mapErr(err)
	}
	binds := []vk.VertexInputBindingDescription{{Binding: 0, Stride: desc.Stride}}
	attrs := make([]vk.VertexInputAttributeDescription, len(desc.Attrs))
	for i, a := range desc.Attrs {
		attrs[i] = vk.VertexInputAttributeDescription{
			Location: a.Location,
			Binding:  0,
			Format:   a.Format,
			Offset:   a.Offset,
		}
	}
	// A pass with a depth attachment requires depth-stencil state on every
	// pipeline that renders into it, whether or not this pipeline tests.
	var depth *vk.DepthState
	if c.depthFormat != 0 {
		depth = &vk.DepthState{Test: desc.DepthTest, Write: desc.DepthWrite}
	}
	pipe, err := vk.CreateGraphicsPipelineEx(b.dev, c.passClear, vk.PipelineDesc{
		Vertex:    vmod,
		Fragment:  fmod,
		Topology:  vk.PRIMITIVE_TOPOLOGY_TRIANGLE_LIST,
		Bindings:  binds,
		Attrs:     attrs,
		Layout:    layout,
		Cull:      desc.Cull,
		Depth:     depth,
		Blend:     desc.Blend,
		SrcFactor: desc.SrcFactor,
		DstFactor: desc.DstFactor,
	})
	if err != nil {
		vk.DestroyPipelineLayout(b.dev, layout)
		vk.DestroyDescriptorSetLayout(b.dev, setLayout)
		return nil, mapErr(err)
	}
	return &CustomPipeline{
		c:         c,
		pipe:      pipe,
		layout:    layout,
		setLayout: setLayout,
		pushSize:  desc.PushSize,
		sets:      make(map[descKey]vk.DescriptorSet),
	}, nil
}

// Release frees the pipeline and its layouts. Descriptor sets live in the
// pass set's pool and go with it.
func (p *CustomPipeline) Release() {
	if p == nil || p.pipe == 0 {
		return
	}
	old := *p
	*p = CustomPipeline{}
	b := old.c.backend
	if b == nil || b.dev == nil {
		return
	}
	b.deferFunc(func(d vk.Device) {
		vk.DestroyPipeline(d, old.pipe)
		vk.DestroyPipelineLayout(d, old.layout)
		vk.DestroyDescriptorSetLayout(d, old.setLayout)
	})
}

// allocSet hands out a descriptor set from the pass set's pool, growing to a
// new pool when the current one is spent. The set being replaced is still
// referenced by command buffers that have not run yet, so it is destroyed at
// the next frame boundary rather than now.
func (c *Custom) allocSet(layout vk.DescriptorSetLayout) (vk.DescriptorSet, error) {
	if len(c.poolSets) == 0 {
		if c.pool != 0 {
			old := c.pool
			c.pool = 0
			c.backend.deferFunc(func(d vk.Device) {
				vk.DestroyDescriptorPool(d, old)
			})
		}
		const poolSize = 64
		sizes := []vk.DescriptorPoolSize{
			vk.BuildDescriptorPoolSize(vk.DESCRIPTOR_TYPE_UNIFORM_BUFFER, poolSize),
			vk.BuildDescriptorPoolSize(vk.DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, poolSize),
			vk.BuildDescriptorPoolSize(vk.DESCRIPTOR_TYPE_STORAGE_BUFFER, poolSize),
		}
		pool, err := vk.CreateDescriptorPool(c.backend.dev, poolSize, sizes)
		if err != nil {
			return 0, mapErr(err)
		}
		sets, err := vk.AllocateDescriptorSets(c.backend.dev, pool, layout, poolSize)
		if err != nil {
			vk.DestroyDescriptorPool(c.backend.dev, pool)
			return 0, mapErr(err)
		}
		c.pool = pool
		c.poolSets = sets
	}
	set := c.poolSets[0]
	c.poolSets = c.poolSets[1:]
	return set, nil
}

func (p *CustomComputePipeline) descSet(storage [customStorageUnits]*CustomBuffer) (vk.DescriptorSet, error) {
	key := storageDescKey{bufs: storage}
	if set, ok := p.sets[key]; ok {
		return set, nil
	}
	set, err := p.c.allocSet(p.setLayout)
	if err != nil {
		return 0, err
	}
	dev := p.c.backend.dev
	for _, binding := range p.storage {
		buf := storage[binding]
		if buf == nil || buf.buf == 0 {
			return 0, fmt.Errorf("vulkan: no storage buffer at binding %d", binding)
		}
		vk.UpdateDescriptorSet(dev, vk.BuildWriteDescriptorSetBuffer(set, binding, vk.DESCRIPTOR_TYPE_STORAGE_BUFFER, buf.buf))
	}
	p.sets[key] = set
	return set, nil
}

// descSet returns the set that reads uniforms from ubo and samples tex,
// creating it the first time the pair is drawn with.
func (p *CustomPipeline) descSet(ubo *CustomBuffer, tex *CustomTexture) (vk.DescriptorSet, error) {
	key := descKey{ubo: ubo, tex: tex}
	if set, ok := p.sets[key]; ok {
		return set, nil
	}
	if tex.layout != vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL {
		return 0, fmt.Errorf("vulkan: texture is in layout %d, want shader read; a mip chain needs UploadDone", tex.layout)
	}
	if tex.sampler == 0 {
		return 0, fmt.Errorf("vulkan: texture has no sampler, it was not created as sampled")
	}
	set, err := p.c.allocSet(p.setLayout)
	if err != nil {
		return 0, err
	}
	dev := p.c.backend.dev
	vk.UpdateDescriptorSet(dev, vk.BuildWriteDescriptorSetBuffer(set, 0, vk.DESCRIPTOR_TYPE_UNIFORM_BUFFER, ubo.buf))
	vk.UpdateDescriptorSet(dev, vk.BuildWriteDescriptorSetImage(set, 1, vk.DESCRIPTOR_TYPE_COMBINED_IMAGE_SAMPLER, tex.sampler, tex.view, vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL))
	p.sets[key] = set
	return set, nil
}

// framebufferFor returns the framebuffer a pass over this color and depth
// pair uses, building it the first time and when either attachment changed.
func (c *Custom) framebufferFor(color, depth *CustomTexture) (vk.Framebuffer, error) {
	views := []vk.ImageView{color.view}
	if c.depthFormat != 0 {
		if depth == nil {
			return 0, fmt.Errorf("vulkan: the custom pass has a depth attachment, but no depth texture was bound")
		}
		views = append(views, depth.view)
	}
	var depthView vk.ImageView
	if depth != nil {
		depthView = depth.view
	}
	if color.fbo != 0 && color.fboDepth == depthView && color.fboWidth == color.width && color.fboHeight == color.height {
		return color.fbo, nil
	}
	if color.fbo != 0 {
		old := color.fbo
		color.fbo = 0
		c.backend.deferFunc(func(d vk.Device) {
			vk.DestroyFramebuffer(d, old)
		})
	}
	fbo, err := vk.CreateFramebufferEx(c.backend.dev, c.passClear, views, color.width, color.height)
	if err != nil {
		return 0, mapErr(err)
	}
	color.fbo = fbo
	color.fboDepth = depthView
	color.fboWidth, color.fboHeight = color.width, color.height
	return fbo, nil
}

// BeginPass starts a pass that draws into target, clearing it to clearColor
// when clear is set and clearing the depth attachment to one. Call EndPass
// when the draws are done.
func (c *Custom) BeginPass(target, depth *CustomTexture, viewport image.Rectangle, clear bool, clearColor [4]float32) error {
	if target == nil {
		return fmt.Errorf("vulkan: no color target")
	}
	if c.active {
		return fmt.Errorf("vulkan: a custom pass is already active")
	}
	if c.backend.cmdPool.current != nil && !c.computeQueued {
		return fmt.Errorf("vulkan: a custom pass cannot run inside a Gio frame")
	}
	fbo, err := c.framebufferFor(target, depth)
	if err != nil {
		return err
	}
	cmd := c.backend.ensureCmdBuf()
	target.imageBarrier(cmd,
		vk.IMAGE_LAYOUT_COLOR_ATTACHMENT_OPTIMAL,
		vk.PIPELINE_STAGE_COLOR_ATTACHMENT_OUTPUT_BIT,
		vk.ACCESS_COLOR_ATTACHMENT_READ_BIT|vk.ACCESS_COLOR_ATTACHMENT_WRITE_BIT,
	)
	if depth != nil {
		depth.imageBarrier(cmd,
			vk.IMAGE_LAYOUT_DEPTH_STENCIL_ATTACHMENT_OPTIMAL,
			vk.PIPELINE_STAGE_EARLY_FRAGMENT_TESTS_BIT|vk.PIPELINE_STAGE_LATE_FRAGMENT_TESTS_BIT,
			vk.ACCESS_DEPTH_STENCIL_ATTACHMENT_READ_BIT|vk.ACCESS_DEPTH_STENCIL_ATTACHMENT_WRITE_BIT,
		)
	}
	pass := c.passClear
	if !clear {
		pass = c.passLoad
	}
	w, h := target.width, target.height
	vk.CmdBeginRenderPassEx(cmd, pass, fbo, w, h, clearColor, c.depthFormat != 0, 1.0)
	// The viewport is dynamic state and applies to the draws that follow.
	vk.CmdSetViewport(cmd, 0, vk.BuildViewport(
		float32(viewport.Min.X), float32(viewport.Min.Y),
		float32(viewport.Dx()), float32(viewport.Dy()),
	))
	c.active = true
	c.computeQueued = false
	c.backend.customActive = true
	c.color = target
	c.depth = depth
	return nil
}

// EndPass ends the pass, hands the color target back to the layout samplers
// read it in, and submits everything the pass recorded. When it returns the
// frame is finished on the GPU.
func (c *Custom) EndPass() error {
	if !c.active {
		return nil
	}
	cmd := c.backend.cmdPool.current
	vk.CmdEndRenderPass(cmd)
	if c.color != nil {
		c.color.imageBarrier(cmd,
			vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
			vk.PIPELINE_STAGE_FRAGMENT_SHADER_BIT,
			vk.ACCESS_SHADER_READ_BIT,
		)
	}
	c.active = false
	c.backend.customActive = false
	c.color = nil
	c.depth = nil
	return c.backend.submitAndWait()
}

func (c *Custom) cmdBuf() (vk.CommandBuffer, error) {
	if !c.active || c.backend.cmdPool.current == nil {
		return nil, fmt.Errorf("vulkan: no active custom pass")
	}
	return c.backend.cmdPool.current, nil
}

// SetPipeline binds a pipeline.
func (c *Custom) SetPipeline(p *CustomPipeline) error {
	cmd, err := c.cmdBuf()
	if err != nil {
		return err
	}
	if p == nil || p.pipe == 0 {
		return fmt.Errorf("vulkan: no pipeline")
	}
	c.pipe = p
	vk.CmdBindPipeline(cmd, vk.PIPELINE_BIND_POINT_GRAPHICS, p.pipe)
	return nil
}

// PrepareVertexBuffer records any required barrier before a render pass
// begins, such as compute shader writes becoming vertex input reads.
func (c *Custom) PrepareVertexBuffer(b *CustomBuffer) error {
	if c.active {
		return fmt.Errorf("vulkan: vertex buffer preparation during a custom pass")
	}
	if b == nil || b.buf == 0 {
		return fmt.Errorf("vulkan: no vertex buffer")
	}
	if c.backend.cmdPool.current == nil {
		return fmt.Errorf("vulkan: vertex buffer preparation requires queued commands")
	}
	cmd := c.backend.cmdPool.current
	b.barrier(cmd, vk.PIPELINE_STAGE_VERTEX_INPUT_BIT, vk.ACCESS_VERTEX_ATTRIBUTE_READ_BIT)
	return nil
}

// SetVertexBuffer binds a vertex buffer at offset.
func (c *Custom) SetVertexBuffer(b *CustomBuffer, offset int) error {
	cmd, err := c.cmdBuf()
	if err != nil {
		return err
	}
	if b == nil || b.buf == 0 {
		return fmt.Errorf("vulkan: no vertex buffer")
	}
	b.barrier(cmd, vk.PIPELINE_STAGE_VERTEX_INPUT_BIT, vk.ACCESS_VERTEX_ATTRIBUTE_READ_BIT)
	vk.CmdBindVertexBuffers(cmd, 0, []vk.Buffer{b.buf}, []vk.DeviceSize{vk.DeviceSize(offset)})
	return nil
}

// SetIndexBuffer binds the index buffer draws read their indices from.
func (c *Custom) SetIndexBuffer(b *CustomBuffer) error {
	cmd, err := c.cmdBuf()
	if err != nil {
		return err
	}
	if b == nil || b.buf == 0 {
		return fmt.Errorf("vulkan: no index buffer")
	}
	b.barrier(cmd, vk.PIPELINE_STAGE_VERTEX_INPUT_BIT, vk.ACCESS_INDEX_READ_BIT)
	vk.CmdBindIndexBuffer(cmd, b.buf, 0, vk.INDEX_TYPE_UINT16)
	return nil
}

// SetUniforms selects the buffer the frame's uniforms are read from. The
// draws that follow bind it through their descriptor set.
func (c *Custom) SetUniforms(b *CustomBuffer) error {
	if b == nil || b.buf == 0 {
		return fmt.Errorf("vulkan: no uniform buffer")
	}
	c.ubo = b
	return nil
}

// SetTexture selects the texture the draws that follow sample.
func (c *Custom) SetTexture(t *CustomTexture) error {
	if t == nil {
		return fmt.Errorf("vulkan: no texture")
	}
	c.tex = t
	return nil
}

// SetStorageBuffer selects a storage buffer for a later compute dispatch.
func (c *Custom) SetStorageBuffer(binding int, b *CustomBuffer) error {
	if binding < 0 || binding >= customStorageUnits {
		return fmt.Errorf("vulkan: storage binding %d out of range [0, %d)", binding, customStorageUnits)
	}
	if b == nil || b.buf == 0 {
		return fmt.Errorf("vulkan: no storage buffer")
	}
	c.storage[binding] = b
	return nil
}

// DispatchCompute runs a compute shader outside a render pass and waits for it
// before returning, so the next render pass can consume its output.
// DispatchCompute runs a compute shader outside the render pass and waits for it
// before returning, so subsequent draws can consume its output.
func (c *Custom) DispatchCompute(p *CustomComputePipeline, push []byte, x, y, z int) error {
	if err := c.dispatchCompute(p, push, x, y, z); err != nil {
		return err
	}
	return c.backend.submitAndWait()
}

// DispatchComputeQueued records compute work to be consumed by the next custom
// render pass, allowing both stages to share one queue submission and fence.
func (c *Custom) DispatchComputeQueued(p *CustomComputePipeline, push []byte, x, y, z int) error {
	if c.active {
		return fmt.Errorf("vulkan: compute dispatch while a custom pass is active")
	}
	if c.backend.cmdPool.current != nil && !c.computeQueued {
		return fmt.Errorf("vulkan: another command buffer is already being recorded")
	}
	if err := c.dispatchCompute(p, push, x, y, z); err != nil {
		return err
	}
	c.computeQueued = true
	return nil
}

func (c *Custom) dispatchCompute(p *CustomComputePipeline, push []byte, x, y, z int) error {
	if c.active {
		return fmt.Errorf("vulkan: compute dispatch while a custom pass is active")
	}
	if p == nil || p.pipe == 0 {
		return fmt.Errorf("vulkan: no pipeline")
	}
	if len(push)%4 != 0 {
		return fmt.Errorf("vulkan: push constants are %d bytes, want a multiple of 4", len(push))
	}
	if p.pushSize > 0 && len(push) > p.pushSize {
		return fmt.Errorf("vulkan: push constants are %d bytes, the pipeline declared %d", len(push), p.pushSize)
	}
	if x < 1 || y < 1 || z < 1 {
		return fmt.Errorf("vulkan: compute dispatch is %dx%dx%d", x, y, z)
	}
	for _, binding := range p.storage {
		buf := c.storage[binding]
		if buf == nil || buf.buf == 0 {
			return fmt.Errorf("vulkan: no storage buffer at binding %d", binding)
		}
	}
	set, err := p.descSet(c.storage)
	if err != nil {
		return err
	}
	cmd := c.backend.ensureCmdBuf()
	vk.CmdBindPipeline(cmd, vk.PIPELINE_BIND_POINT_COMPUTE, p.pipe)
	if len(push) > 0 {
		vk.CmdPushConstants(cmd, p.layout, vk.SHADER_STAGE_COMPUTE_BIT, 0, push)
	}
	for _, binding := range p.storage {
		buf := c.storage[binding]
		buf.barrier(cmd, vk.PIPELINE_STAGE_COMPUTE_SHADER_BIT, vk.ACCESS_SHADER_READ_BIT|vk.ACCESS_SHADER_WRITE_BIT)
	}
	vk.CmdBindDescriptorSets(cmd, vk.PIPELINE_BIND_POINT_COMPUTE, p.layout, 0, []vk.DescriptorSet{set})
	vk.CmdDispatch(cmd, x, y, z)
	for _, binding := range p.storage {
		buf := c.storage[binding]
		buf.scope.stage = vk.PIPELINE_STAGE_COMPUTE_SHADER_BIT
		buf.scope.access = vk.ACCESS_SHADER_READ_BIT | vk.ACCESS_SHADER_WRITE_BIT
	}
	return nil
}

func (c *Custom) PushDraw(data []byte) error {
	cmd, err := c.cmdBuf()
	if err != nil {
		return err
	}
	if c.pipe == nil || c.pipe.pipe == 0 {
		return fmt.Errorf("vulkan: no pipeline")
	}
	if len(data) == 0 || len(data)%4 != 0 {
		return fmt.Errorf("vulkan: push constants are %d bytes, want a multiple of 4", len(data))
	}
	if c.pipe.pushSize > 0 && len(data) > c.pipe.pushSize {
		return fmt.Errorf("vulkan: push constants are %d bytes, the pipeline declared %d", len(data), c.pipe.pushSize)
	}
	vk.CmdPushConstants(cmd, c.pipe.layout,
		vk.SHADER_STAGE_VERTEX_BIT|vk.SHADER_STAGE_FRAGMENT_BIT, 0, data)
	return nil
}

// DrawIndexed draws count indices from the bound index buffer with the
// currently selected pipeline, uniforms and texture.
func (c *Custom) DrawIndexed(count int) error {
	cmd, err := c.cmdBuf()
	if err != nil {
		return err
	}
	if c.pipe == nil || c.pipe.pipe == 0 {
		return fmt.Errorf("vulkan: no pipeline")
	}
	if c.ubo == nil {
		return fmt.Errorf("vulkan: no uniform buffer")
	}
	if c.tex == nil {
		return fmt.Errorf("vulkan: no texture")
	}
	set, err := c.pipe.descSet(c.ubo, c.tex)
	if err != nil {
		return err
	}
	vk.CmdBindDescriptorSets(cmd, vk.PIPELINE_BIND_POINT_GRAPHICS, c.pipe.layout, 0, []vk.DescriptorSet{set})
	vk.CmdDrawIndexed(cmd, count, 1, 0, 0, 0)
	return nil
}

// Draw draws count vertices straight out of the bound vertex buffer, with no
// index buffer: what a batch of quads or lines wants, where the order they are
// written in is already the order they are drawn in.
func (c *Custom) Draw(count int) error {
	cmd, err := c.cmdBuf()
	if err != nil {
		return err
	}
	if c.pipe == nil || c.pipe.pipe == 0 {
		return fmt.Errorf("vulkan: no pipeline")
	}
	if c.ubo == nil {
		return fmt.Errorf("vulkan: no uniform buffer")
	}
	if c.tex == nil {
		return fmt.Errorf("vulkan: no texture")
	}
	set, err := c.pipe.descSet(c.ubo, c.tex)
	if err != nil {
		return err
	}
	vk.CmdBindDescriptorSets(cmd, vk.PIPELINE_BIND_POINT_GRAPHICS, c.pipe.layout, 0, []vk.DescriptorSet{set})
	vk.CmdDraw(cmd, count, 1, 0, 0)
	return nil
}

// NewExternalTexture adopts a texture a custom pass rendered so that Gio can
// paint it where it is, without a copy. Gio does not own the image: when it
// lets go, the release is handed back to the texture that made it, which
// keeps the image alive until then.
func (b *Backend) NewExternalTexture(src driver.ExternalTexture) (driver.Texture, error) {
	raw, ok := src.Handle.(*CustomTexture)
	if !ok {
		return nil, fmt.Errorf("vulkan: unsupported external texture handle %T", src.Handle)
	}
	if raw.released || raw.img == 0 {
		return nil, fmt.Errorf("vulkan: cannot adopt a released texture")
	}
	raw.extRefs++
	return &Texture{
		backend:   b,
		img:       raw.img,
		view:      raw.view,
		sampler:   raw.sampler,
		format:    raw.format,
		width:     raw.width,
		height:    raw.height,
		layout:    vk.IMAGE_LAYOUT_SHADER_READ_ONLY_OPTIMAL,
		adopted:   true,
		onRelease: raw.unref,
	}, nil
}
