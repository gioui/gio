// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"math/bits"
	"runtime"
	"time"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/byteslice"
	"gioui.org/internal/f32color"
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/internal/scene"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	cpu "git.sr.ht/~eliasnaur/compute"
)

type compute struct {
	ctx driver.Device

	collector     collector
	viewport      image.Point
	maxTextureDim int

	programs struct {
		elements   computeProgram
		tileAlloc  computeProgram
		pathCoarse computeProgram
		backdrop   computeProgram
		binning    computeProgram
		coarse     computeProgram
		kernel4    computeProgram
	}
	buffers struct {
		config sizedBuffer
		scene  sizedBuffer
		state  sizedBuffer
		memory sizedBuffer
	}
	output struct {
		size image.Point
		// image is the output texture. Note that it is in RGBA format,
		// but contains data in sRGB. See blitOutput for more detail.
		image    driver.Texture
		blitProg driver.Program

		cpuImage    cpu.ImageDescriptor
		descriptors *cpu.Kernel4DescriptorSetLayout
	}
	// images contains ImageOp images packed into a texture atlas.
	images struct {
		packer packer
		// positions maps imageOpData.handles to positions inside tex.
		positions map[interface{}]image.Point
		tex       driver.Texture
	}
	// materials contains the pre-processed materials (transformed images for
	// now, gradients etc. later) packed in a texture atlas. The atlas is used
	// as source in kernel4.
	materials struct {
		// offsets maps texture ops to the offsets to put in their FillImage commands.
		offsets map[textureKey]image.Point

		prog   driver.Program
		layout driver.InputLayout

		packer packer

		tex   driver.Texture
		fbo   driver.Framebuffer
		quads []materialVertex

		bufSize int
		buffer  driver.Buffer

		// CPU fields
		cpuTex cpu.ImageDescriptor
		// regions track new materials in tex, so they can be transferred to cpuTex.
		regions []image.Rectangle
		scratch []byte
	}
	timers struct {
		profile         string
		t               *timers
		materials       *timer
		elements        *timer
		tileAlloc       *timer
		pathCoarse      *timer
		backdropBinning *timer
		coarse          *timer
		kernel4         *timer
		blit            *timer
	}

	// CPU fallback fields.
	useCPU     bool
	dispatcher *dispatcher

	// The following fields hold scratch space to avoid garbage.
	zeroSlice []byte
	memHeader *memoryHeader
	conf      *config
}

type collector struct {
	enc        encoder
	profile    bool
	reader     ops.Reader
	states     []encoderState
	clear      bool
	clearColor f32color.RGBA
	pathCache  []pathState
	texOps     []textureOp
	clipStack  []clipCmd
}

// clipCmd describes a clipping command ready to be used for the compute
// pipeline.
type clipCmd struct {
	// union of the bounds of the operations that are clipped.
	union f32.Rectangle
	path  *pathState
}

type encoderState struct {
	t         f32.Affine2D
	path      *pathState
	intersect f32.Rectangle

	matType materialType
	// Current paint.ImageOp
	image imageOpData
	// Current paint.ColorOp, if any.
	color color.NRGBA

	// Current paint.LinearGradientOp.
	stop1  f32.Point
	stop2  f32.Point
	color1 color.NRGBA
	color2 color.NRGBA
}

type pathState struct {
	bounds f32.Rectangle
	// intersect is the intersection of the bounds of this path and its parents.
	intersect f32.Rectangle
	pathVerts []byte
	parent    *pathState
	trans     f32.Affine2D
	stroke    clip.StrokeStyle
	rect      rectangle
	isRect    bool
}

// materialVertex describes a vertex of a quad used to render a transformed
// material.
type materialVertex struct {
	posX, posY float32
	u, v       float32
}

// textureKey identifies textureOp.
type textureKey struct {
	handle    interface{}
	transform f32.Affine2D
}

// textureOp represents an paintOp that requires texture space.
type textureOp struct {
	// sceneIdx is the index in the scene that contains the fill image command
	// that corresponds to the operation.
	sceneIdx int
	key      textureKey
	img      imageOpData

	// pos is the position of the untransformed image in the images texture.
	pos image.Point
}

type encoder struct {
	scene    []scene.Command
	npath    int
	npathseg int
	ntrans   int
}

type encodeState struct {
	trans f32.Affine2D
	clip  f32.Rectangle
}

// sizedBuffer holds a GPU buffer, or its equivalent CPU memory.
type sizedBuffer struct {
	size   int
	buffer driver.Buffer
	// cpuBuf is initialized when useCPU is true.
	cpuBuf cpu.BufferDescriptor
}

// computeProgram holds a compute program, or its equivalent CPU implementation.
type computeProgram struct {
	prog driver.Program

	// CPU fields.
	progInfo    *cpu.ProgramInfo
	descriptors unsafe.Pointer
	buffers     []*cpu.BufferDescriptor
}

// config matches Config in setup.h
type config struct {
	n_elements      uint32 // paths
	n_pathseg       uint32
	width_in_tiles  uint32
	height_in_tiles uint32
	tile_alloc      memAlloc
	bin_alloc       memAlloc
	ptcl_alloc      memAlloc
	pathseg_alloc   memAlloc
	anno_alloc      memAlloc
	trans_alloc     memAlloc
}

// memAlloc matches Alloc in mem.h
type memAlloc struct {
	offset uint32
	//size   uint32
}

// memoryHeader matches the header of Memory in mem.h.
type memoryHeader struct {
	mem_offset uint32
	mem_error  uint32
}

// rect is a oriented rectangle.
type rectangle [4]f32.Point

// GPU structure sizes and constants.
const (
	tileWidthPx       = 32
	tileHeightPx      = 32
	ptclInitialAlloc  = 1024
	kernel4OutputUnit = 2
	kernel4AtlasUnit  = 3

	pathSize    = 12
	binSize     = 8
	pathsegSize = 52
	annoSize    = 32
	transSize   = 24
	stateSize   = 60
	stateStride = 4 + 2*stateSize
)

// mem.h constants.
const (
	memNoError      = 0 // NO_ERROR
	memMallocFailed = 1 // ERR_MALLOC_FAILED
)

func newCompute(ctx driver.Device) (*compute, error) {
	caps := ctx.Caps()
	maxDim := caps.MaxTextureSize
	// Large atlas textures cause artifacts due to precision loss in
	// shaders.
	if cap := 8192; maxDim > cap {
		maxDim = cap
	}
	useCPU := supportsCPUCompute && !caps.Features.Has(driver.FeatureCompute)
	useCPU = true
	g := &compute{
		ctx:           ctx,
		maxTextureDim: maxDim,
		conf:          new(config),
		memHeader:     new(memoryHeader),
		useCPU:        useCPU,
	}
	if useCPU {
		g.dispatcher = newDispatcher(runtime.NumCPU())
	}

	blitProg, err := ctx.NewProgram(shader_copy_vert, shader_copy_frag)
	if err != nil {
		g.Release()
		return nil, err
	}
	g.output.blitProg = blitProg

	materialProg, err := ctx.NewProgram(shader_material_vert, shader_material_frag)
	if err != nil {
		g.Release()
		return nil, err
	}
	g.materials.prog = materialProg
	progLayout, err := ctx.NewInputLayout(shader_material_vert, []driver.InputDesc{
		{Type: driver.DataTypeFloat, Size: 2, Offset: 0},
		{Type: driver.DataTypeFloat, Size: 2, Offset: 4 * 2},
	})
	if err != nil {
		g.Release()
		return nil, err
	}
	g.materials.layout = progLayout

	shaders := []struct {
		prog *computeProgram
		src  driver.ShaderSources
		info *cpu.ProgramInfo
	}{
		{&g.programs.elements, shader_elements_comp, cpu.ElementsProgramInfo},
		{&g.programs.tileAlloc, shader_tile_alloc_comp, cpu.Tile_allocProgramInfo},
		{&g.programs.pathCoarse, shader_path_coarse_comp, cpu.Path_coarseProgramInfo},
		{&g.programs.backdrop, shader_backdrop_comp, cpu.BackdropProgramInfo},
		{&g.programs.binning, shader_binning_comp, cpu.BinningProgramInfo},
		{&g.programs.coarse, shader_coarse_comp, cpu.CoarseProgramInfo},
		{&g.programs.kernel4, shader_kernel4_comp, cpu.Kernel4ProgramInfo},
	}
	for _, shader := range shaders {
		if !useCPU {
			p, err := ctx.NewComputeProgram(shader.src)
			if err != nil {
				g.Release()
				return nil, err
			}
			shader.prog.prog = p
		} else {
			shader.prog.progInfo = shader.info
		}
	}
	if g.useCPU {
		{
			desc := new(cpu.ElementsDescriptorSetLayout)
			g.programs.elements.descriptors = unsafe.Pointer(desc)
			g.programs.elements.buffers = []*cpu.BufferDescriptor{&desc.Binding0, &desc.Binding1, &desc.Binding2, &desc.Binding3}
		}
		{
			desc := new(cpu.Tile_allocDescriptorSetLayout)
			g.programs.tileAlloc.descriptors = unsafe.Pointer(desc)
			g.programs.tileAlloc.buffers = []*cpu.BufferDescriptor{&desc.Binding0, &desc.Binding1}
		}
		{
			desc := new(cpu.Path_coarseDescriptorSetLayout)
			g.programs.pathCoarse.descriptors = unsafe.Pointer(desc)
			g.programs.pathCoarse.buffers = []*cpu.BufferDescriptor{&desc.Binding0, &desc.Binding1}
		}
		{
			desc := new(cpu.BackdropDescriptorSetLayout)
			g.programs.backdrop.descriptors = unsafe.Pointer(desc)
			g.programs.backdrop.buffers = []*cpu.BufferDescriptor{&desc.Binding0, &desc.Binding1}
		}
		{
			desc := new(cpu.BinningDescriptorSetLayout)
			g.programs.binning.descriptors = unsafe.Pointer(desc)
			g.programs.binning.buffers = []*cpu.BufferDescriptor{&desc.Binding0, &desc.Binding1}
		}
		{
			desc := new(cpu.CoarseDescriptorSetLayout)
			g.programs.coarse.descriptors = unsafe.Pointer(desc)
			g.programs.coarse.buffers = []*cpu.BufferDescriptor{&desc.Binding0, &desc.Binding1}
		}
		{
			desc := new(cpu.Kernel4DescriptorSetLayout)
			g.programs.kernel4.descriptors = unsafe.Pointer(desc)
			g.programs.kernel4.buffers = []*cpu.BufferDescriptor{&desc.Binding0, &desc.Binding1}
			g.output.descriptors = desc
		}
	}
	return g, nil
}

func (g *compute) Collect(viewport image.Point, ops *op.Ops) {
	g.viewport = viewport
	g.collector.reset()

	// Flip Y-axis.
	flipY := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(1, -1)).Offset(f32.Pt(0, float32(viewport.Y)))
	g.collector.collect(g.ctx, ops, flipY, viewport)
}

func (g *compute) Clear(col color.NRGBA) {
	g.collector.clear = true
	g.collector.clearColor = f32color.LinearFromSRGB(col)
}

func (g *compute) Frame() error {
	viewport := g.viewport
	tileDims := image.Point{
		X: (viewport.X + tileWidthPx - 1) / tileWidthPx,
		Y: (viewport.Y + tileHeightPx - 1) / tileHeightPx,
	}

	defFBO := g.ctx.BeginFrame()
	defer g.ctx.EndFrame()

	if g.collector.profile && g.timers.t == nil && g.ctx.Caps().Features.Has(driver.FeatureTimers) {
		t := &g.timers
		t.t = newTimers(g.ctx)
		t.materials = g.timers.t.newTimer()
		t.elements = g.timers.t.newTimer()
		t.tileAlloc = g.timers.t.newTimer()
		t.pathCoarse = g.timers.t.newTimer()
		t.backdropBinning = g.timers.t.newTimer()
		t.coarse = g.timers.t.newTimer()
		t.kernel4 = g.timers.t.newTimer()
		t.blit = g.timers.t.newTimer()
	}

	mat := g.timers.materials
	mat.begin()
	if err := g.uploadImages(); err != nil {
		return err
	}
	if err := g.renderMaterials(); err != nil {
		return err
	}
	mat.end()
	if err := g.render(tileDims); err != nil {
		return err
	}
	g.ctx.BindFramebuffer(defFBO)
	g.blitOutput(viewport)
	t := &g.timers
	if g.collector.profile && t.t.ready() {
		mat := t.materials.Elapsed
		et, tat, pct, bbt := t.elements.Elapsed, t.tileAlloc.Elapsed, t.pathCoarse.Elapsed, t.backdropBinning.Elapsed
		ct, k4t := t.coarse.Elapsed, t.kernel4.Elapsed
		blit := t.blit.Elapsed
		ft := mat + et + tat + pct + bbt + ct + k4t + blit
		q := 100 * time.Microsecond
		ft = ft.Round(q)
		mat = mat.Round(q)
		et, tat, pct, bbt = et.Round(q), tat.Round(q), pct.Round(q), bbt.Round(q)
		ct, k4t = ct.Round(q), k4t.Round(q)
		blit = blit.Round(q)
		t.profile = fmt.Sprintf("ft:%7s mat: %7s et:%7s tat:%7s pct:%7s bbt:%7s ct:%7s k4t:%7s blit:%7s", ft, mat, et, tat, pct, bbt, ct, k4t, blit)
	}
	g.collector.clear = false
	return nil
}

func (g *compute) Profile() string {
	return g.timers.profile
}

// blitOutput copies the compute render output to the output FBO. We need to
// copy because compute shaders can only write to textures, not FBOs. Compute
// shader can only write to RGBA textures, but since we actually render in sRGB
// format we can't use glBlitFramebuffer, because it does sRGB conversion.
func (g *compute) blitOutput(viewport image.Point) {
	t := g.timers.blit
	t.begin()
	if !g.collector.clear {
		g.ctx.BlendFunc(driver.BlendFactorOne, driver.BlendFactorOneMinusSrcAlpha)
		g.ctx.SetBlend(true)
		defer g.ctx.SetBlend(false)
	}
	g.ctx.Viewport(0, 0, viewport.X, viewport.Y)
	g.ctx.BindTexture(0, g.output.image)
	g.ctx.BindProgram(g.output.blitProg)
	g.ctx.DrawArrays(driver.DrawModeTriangleStrip, 0, 4)
	t.end()
}

func (g *compute) renderMaterials() error {
	m := &g.materials
	m.quads = m.quads[:0]
	m.regions = m.regions[:0]
	resize := false
	reclaimed := false
restart:
	for {
		for _, op := range g.collector.texOps {
			if off, exists := m.offsets[op.key]; exists {
				g.collector.enc.setFillImageOffset(op.sceneIdx, off)
				continue
			}
			quad, bounds := g.materialQuad(op.key.transform, op.img, op.pos)

			// A material is clipped to avoid drawing outside its bounds inside the atlas. However,
			// imprecision in the clipping may cause a single pixel overflow. Be safe.
			size := bounds.Size().Add(image.Pt(1, 1))
			place, fits := m.packer.tryAdd(size)
			if !fits {
				m.offsets = nil
				m.quads = m.quads[:0]
				m.packer.clear()
				if !reclaimed {
					// Some images may no longer be in use, try again
					// after clearing existing maps.
					reclaimed = true
				} else {
					m.packer.maxDim += 256
					resize = true
					if m.packer.maxDim > g.maxTextureDim {
						return errors.New("compute: no space left in material atlas")
					}
				}
				m.packer.newPage()
				continue restart
			}
			// Position quad to match place.
			offset := place.Pos.Sub(bounds.Min)
			offsetf := layout.FPt(offset)
			for i := range quad {
				quad[i].posX += offsetf.X
				quad[i].posY += offsetf.Y
			}
			// Draw quad as two triangles.
			m.quads = append(m.quads, quad[0], quad[1], quad[3], quad[3], quad[1], quad[2])
			if m.offsets == nil {
				m.offsets = make(map[textureKey]image.Point)
			}
			m.offsets[op.key] = offset
			g.collector.enc.setFillImageOffset(op.sceneIdx, offset)
			m.regions = append(m.regions, image.Rectangle{
				Min: place.Pos,
				Max: place.Pos.Add(size),
			})
		}
		break
	}
	if len(m.quads) == 0 {
		return nil
	}
	texSize := m.packer.maxDim
	if resize {
		if m.fbo != nil {
			m.fbo.Release()
			m.fbo = nil
		}
		if m.tex != nil {
			m.tex.Release()
			m.tex = nil
		}
		m.cpuTex.Free()
		handle, err := g.ctx.NewTexture(driver.TextureFormatRGBA8, texSize, texSize,
			driver.FilterNearest, driver.FilterNearest,
			driver.BufferBindingShaderStorage|driver.BufferBindingFramebuffer)
		if err != nil {
			return fmt.Errorf("compute: failed to create material atlas: %v", err)
		}
		m.tex = handle
		fbo, err := g.ctx.NewFramebuffer(handle, 0)
		if err != nil {
			return fmt.Errorf("compute: failed to create material framebuffer: %v", err)
		}
		m.fbo = fbo
		if g.useCPU {
			m.cpuTex = cpu.NewImageRGBA(texSize, texSize)
		}
	}
	// TODO: move to shaders.
	// Transform to clip space: [-1, -1] - [1, 1].
	clip := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(2/float32(texSize), 2/float32(texSize))).Offset(f32.Pt(-1, -1))
	for i, v := range m.quads {
		p := clip.Transform(f32.Pt(v.posX, v.posY))
		m.quads[i].posX = p.X
		m.quads[i].posY = p.Y
	}
	vertexData := byteslice.Slice(m.quads)
	if len(vertexData) > m.bufSize {
		if m.buffer != nil {
			m.buffer.Release()
			m.buffer = nil
		}
		n := pow2Ceil(len(vertexData))
		buf, err := g.ctx.NewBuffer(driver.BufferBindingVertices, n)
		if err != nil {
			return err
		}
		m.bufSize = n
		m.buffer = buf
	}
	m.buffer.Upload(vertexData)
	g.ctx.BindTexture(0, g.images.tex)
	g.ctx.BindFramebuffer(m.fbo)
	g.ctx.Viewport(0, 0, texSize, texSize)
	if reclaimed {
		g.ctx.Clear(0, 0, 0, 0)
	}
	g.ctx.BindProgram(m.prog)
	g.ctx.BindVertexBuffer(m.buffer, int(unsafe.Sizeof(m.quads[0])), 0)
	g.ctx.BindInputLayout(m.layout)
	g.ctx.DrawArrays(driver.DrawModeTriangles, 0, len(m.quads))
	return nil
}

func (g *compute) uploadImages() error {
	// padding is the number of pixels added to the right and below
	// images, to avoid atlas filtering artifacts.
	const padding = 1

	a := &g.images
	var uploads map[interface{}]*image.RGBA
	resize := false
	reclaimed := false
restart:
	for {
		for i, op := range g.collector.texOps {
			if pos, exists := a.positions[op.img.handle]; exists {
				g.collector.texOps[i].pos = pos
				continue
			}
			size := op.img.src.Bounds().Size().Add(image.Pt(padding, padding))
			place, fits := a.packer.tryAdd(size)
			if !fits {
				a.positions = nil
				uploads = nil
				a.packer.clear()
				if !reclaimed {
					// Some images may no longer be in use, try again
					// after clearing existing maps.
					reclaimed = true
				} else {
					a.packer.maxDim += 256
					resize = true
					if a.packer.maxDim > g.maxTextureDim {
						return errors.New("compute: no space left in image atlas")
					}
				}
				a.packer.newPage()
				continue restart
			}
			if a.positions == nil {
				a.positions = make(map[interface{}]image.Point)
			}
			a.positions[op.img.handle] = place.Pos
			g.collector.texOps[i].pos = place.Pos
			if uploads == nil {
				uploads = make(map[interface{}]*image.RGBA)
			}
			uploads[op.img.handle] = op.img.src
		}
		break
	}
	if len(uploads) == 0 {
		return nil
	}
	if resize {
		if a.tex != nil {
			a.tex.Release()
			a.tex = nil
		}
		sz := a.packer.maxDim
		handle, err := g.ctx.NewTexture(driver.TextureFormatSRGB, sz, sz, driver.FilterLinear, driver.FilterLinear, driver.BufferBindingTexture)
		if err != nil {
			return fmt.Errorf("compute: failed to create image atlas: %v", err)
		}
		a.tex = handle
	}
	for h, img := range uploads {
		pos, ok := a.positions[h]
		if !ok {
			panic("compute: internal error: image not placed")
		}
		size := img.Bounds().Size()
		driver.UploadImage(a.tex, pos, img)
		rightPadding := image.Pt(padding, size.Y)
		a.tex.Upload(image.Pt(pos.X+size.X, pos.Y), rightPadding, g.zeros(rightPadding.X*rightPadding.Y*4))
		bottomPadding := image.Pt(size.X, padding)
		a.tex.Upload(image.Pt(pos.X, pos.Y+size.Y), bottomPadding, g.zeros(bottomPadding.X*bottomPadding.Y*4))
	}
	return nil
}

func pow2Ceil(v int) int {
	exp := bits.Len(uint(v))
	if bits.OnesCount(uint(v)) == 1 {
		exp--
	}
	return 1 << exp
}

// materialQuad constructs a quad that represents the transformed image. It returns the quad
// and its bounds.
func (g *compute) materialQuad(M f32.Affine2D, img imageOpData, uvPos image.Point) ([4]materialVertex, image.Rectangle) {
	imgSize := layout.FPt(img.src.Bounds().Size())
	sx, hx, ox, hy, sy, oy := M.Elems()
	transOff := f32.Pt(ox, oy)
	// The 4 corners of the image rectangle transformed by M, excluding its offset, are:
	//
	// q0: M * (0, 0)   q3: M * (w, 0)
	// q1: M * (0, h)   q2: M * (w, h)
	//
	// Note that q0 = M*0 = 0, q2 = q1 + q3.
	q0 := f32.Pt(0, 0)
	q1 := f32.Pt(hx*imgSize.Y, sy*imgSize.Y)
	q3 := f32.Pt(sx*imgSize.X, hy*imgSize.X)
	q2 := q1.Add(q3)
	q0 = q0.Add(transOff)
	q1 = q1.Add(transOff)
	q2 = q2.Add(transOff)
	q3 = q3.Add(transOff)

	boundsf := f32.Rectangle{
		Min: min(min(q0, q1), min(q2, q3)),
		Max: max(max(q0, q1), max(q2, q3)),
	}

	bounds := boundRectF(boundsf)
	uvPosf := layout.FPt(uvPos)
	atlasScale := 1 / float32(g.images.packer.maxDim)
	uvBounds := f32.Rectangle{
		Min: uvPosf.Mul(atlasScale),
		Max: uvPosf.Add(imgSize).Mul(atlasScale),
	}
	quad := [4]materialVertex{
		{posX: q0.X, posY: q0.Y, u: uvBounds.Min.X, v: uvBounds.Min.Y},
		{posX: q1.X, posY: q1.Y, u: uvBounds.Min.X, v: uvBounds.Max.Y},
		{posX: q2.X, posY: q2.Y, u: uvBounds.Max.X, v: uvBounds.Max.Y},
		{posX: q3.X, posY: q3.Y, u: uvBounds.Max.X, v: uvBounds.Min.Y},
	}
	return quad, bounds
}

func max(p1, p2 f32.Point) f32.Point {
	p := p1
	if p2.X > p.X {
		p.X = p2.X
	}
	if p2.Y > p.Y {
		p.Y = p2.Y
	}
	return p
}

func min(p1, p2 f32.Point) f32.Point {
	p := p1
	if p2.X < p.X {
		p.X = p2.X
	}
	if p2.Y < p.Y {
		p.Y = p2.Y
	}
	return p
}

// buildClipStack constructs the stack of clip commands given the initial path.
func (c *collector) buildClipStack(clip f32.Rectangle, p *pathState) {
	if p := p.parent; p != nil {
		c.buildClipStack(clip.Union(p.bounds), p)
	}
	c.clipStack = append(c.clipStack, clipCmd{union: clip, path: p})
}

func (enc *encoder) encodePath(verts []byte) {
	for len(verts) >= scene.CommandSize+4 {
		cmd := ops.DecodeCommand(verts[4:])
		enc.scene = append(enc.scene, cmd)
		enc.npathseg++
		verts = verts[scene.CommandSize+4:]
	}
}

func (g *compute) render(tileDims image.Point) error {
	const (
		// wgSize is the largest and most common workgroup size.
		wgSize = 128
		// PARTITION_SIZE from elements.comp
		partitionSize = 32 * 4
	)
	widthInBins := (tileDims.X + 15) / 16
	heightInBins := (tileDims.Y + 7) / 8
	if widthInBins*heightInBins > wgSize {
		return fmt.Errorf("gpu: output too large (%dx%d)", tileDims.X*tileWidthPx, tileDims.Y*tileHeightPx)
	}

	enc := &g.collector.enc
	// Pad scene with zeroes to avoid reading garbage in elements.comp.
	scenePadding := partitionSize - len(enc.scene)%partitionSize
	enc.scene = append(enc.scene, make([]scene.Command, scenePadding)...)

	realloced := false
	scene := byteslice.Slice(enc.scene)
	if s := len(scene); s > g.buffers.scene.size {
		realloced = true
		paddedCap := s * 11 / 10
		if err := g.buffers.scene.ensureCapacity(g, paddedCap); err != nil {
			return err
		}
	}
	g.buffers.scene.upload(scene)

	w, h := tileDims.X*tileWidthPx, tileDims.Y*tileHeightPx
	if g.output.size.X != w || g.output.size.Y != h {
		if err := g.resizeOutput(image.Pt(w, h)); err != nil {
			return err
		}
	}
	// alloc is the number of allocated bytes for static buffers.
	var alloc uint32
	round := func(v, quantum int) int {
		return (v + quantum - 1) &^ (quantum - 1)
	}
	malloc := func(size int) memAlloc {
		size = round(size, 4)
		offset := alloc
		alloc += uint32(size)
		return memAlloc{offset /*, uint32(size)*/}
	}

	*g.conf = config{
		n_elements:      uint32(enc.npath),
		n_pathseg:       uint32(enc.npathseg),
		width_in_tiles:  uint32(tileDims.X),
		height_in_tiles: uint32(tileDims.Y),
		tile_alloc:      malloc(enc.npath * pathSize),
		bin_alloc:       malloc(round(enc.npath, wgSize) * binSize),
		ptcl_alloc:      malloc(tileDims.X * tileDims.Y * ptclInitialAlloc),
		pathseg_alloc:   malloc(enc.npathseg * pathsegSize),
		anno_alloc:      malloc(enc.npath * annoSize),
		trans_alloc:     malloc(enc.ntrans * transSize),
	}

	numPartitions := (enc.numElements() + 127) / 128
	// clearSize is the atomic partition counter plus flag and 2 states per partition.
	clearSize := 4 + numPartitions*stateStride
	if clearSize > g.buffers.state.size {
		realloced = true
		paddedCap := clearSize * 11 / 10
		if err := g.buffers.state.ensureCapacity(g, paddedCap); err != nil {
			return err
		}
	}

	confData := byteslice.Struct(g.conf)
	g.buffers.config.ensureCapacity(g, len(confData))
	g.buffers.config.upload(confData)

	minSize := int(unsafe.Sizeof(memoryHeader{})) + int(alloc)
	if minSize > g.buffers.memory.size {
		realloced = true
		// Add space for dynamic GPU allocations.
		const sizeBump = 4 * 1024 * 1024
		minSize += sizeBump
		if err := g.buffers.memory.ensureCapacity(g, minSize); err != nil {
			return err
		}
	}

	if !g.useCPU {
		g.ctx.BindImageTexture(kernel4OutputUnit, g.output.image, driver.AccessWrite, driver.TextureFormatRGBA8)
		if t := g.materials.tex; t != nil {
			g.ctx.BindImageTexture(kernel4AtlasUnit, t, driver.AccessRead, driver.TextureFormatRGBA8)
		}
	} else {
		g.output.descriptors.Binding2 = g.output.cpuImage
		g.output.descriptors.Binding3 = g.materials.cpuTex
	}

	for {
		*g.memHeader = memoryHeader{
			mem_offset: alloc,
		}
		g.buffers.memory.upload(byteslice.Struct(g.memHeader))
		g.buffers.state.upload(g.zeros(clearSize))

		if realloced {
			realloced = false
			g.bindBuffers()
		}

		t := &g.timers
		g.memoryBarrier()
		t.elements.begin()
		g.dispatch(g.programs.elements, numPartitions, 1, 1)
		g.memoryBarrier()
		t.elements.end()
		t.tileAlloc.begin()
		g.dispatch(g.programs.tileAlloc, (enc.npath+wgSize-1)/wgSize, 1, 1)
		g.memoryBarrier()
		t.tileAlloc.end()
		t.pathCoarse.begin()
		g.dispatch(g.programs.pathCoarse, (enc.npathseg+31)/32, 1, 1)
		g.memoryBarrier()
		t.pathCoarse.end()
		t.backdropBinning.begin()
		g.dispatch(g.programs.backdrop, (enc.npath+wgSize-1)/wgSize, 1, 1)
		// No barrier needed between backdrop and binning.
		g.dispatch(g.programs.binning, (enc.npath+wgSize-1)/wgSize, 1, 1)
		g.memoryBarrier()
		t.backdropBinning.end()
		t.coarse.begin()
		g.dispatch(g.programs.coarse, widthInBins, heightInBins, 1)
		g.memoryBarrier()
		t.coarse.end()
		g.downloadMaterials()
		t.kernel4.begin()
		g.dispatch(g.programs.kernel4, tileDims.X, tileDims.Y, 1)
		g.memoryBarrier()
		t.kernel4.end()
		if g.useCPU {
			g.dispatcher.Sync()
		}

		if err := g.buffers.memory.download(byteslice.Struct(g.memHeader)); err != nil {
			if err == driver.ErrContentLost {
				continue
			}
			return err
		}
		switch errCode := g.memHeader.mem_error; errCode {
		case memNoError:
			if g.useCPU {
				g.output.image.Upload(image.Pt(0, 0), image.Pt(w, h), g.output.cpuImage.Data())
			}
			return nil
		case memMallocFailed:
			// Resize memory and try again.
			realloced = true
			sz := g.buffers.memory.size * 15 / 10
			if err := g.buffers.memory.ensureCapacity(g, sz); err != nil {
				return err
			}
			continue
		default:
			return fmt.Errorf("compute: shader program failed with error %d", errCode)
		}
	}
}

func (g *compute) downloadMaterials() {
	m := &g.materials
	if !g.useCPU || len(m.regions) == 0 {
		return
	}
	copyFBO := m.fbo
	data := m.cpuTex.Data()
	for _, r := range m.regions {
		dims := r.Size()
		if n := dims.X * dims.Y * 4; n > len(m.scratch) {
			m.scratch = make([]byte, n)
		}
		copyFBO.ReadPixels(r, m.scratch)
		stride := m.packer.maxDim * 4
		col := r.Min.X * 4
		row := stride * r.Min.Y
		off := col + row
		w := dims.X * 4
		for y := 0; y < dims.Y; y++ {
			copy(data[off:off+w], m.scratch[y*dims.X*4:])
			off += stride
		}
	}
}

func (g *compute) memoryBarrier() {
	if !g.useCPU {
		g.ctx.MemoryBarrier()
	} else {
		g.dispatcher.Barrier()
	}
}

func (g *compute) dispatch(p computeProgram, x, y, z int) {
	if !g.useCPU {
		g.ctx.BindProgram(p.prog)
		g.ctx.DispatchCompute(x, y, z)
	} else {
		g.dispatcher.Dispatch(p.progInfo, p.descriptors, x, y, z)
	}
}

// zeros returns a byte slice with size bytes of zeros.
func (g *compute) zeros(size int) []byte {
	if cap(g.zeroSlice) < size {
		g.zeroSlice = append(g.zeroSlice, make([]byte, size)...)
	}
	return g.zeroSlice[:size]
}

func (g *compute) resizeOutput(size image.Point) error {
	if g.output.image != nil {
		g.output.image.Release()
		g.output.image = nil
	}
	g.output.cpuImage.Free()

	img, err := g.ctx.NewTexture(driver.TextureFormatRGBA8, size.X, size.Y,
		driver.FilterNearest,
		driver.FilterNearest,
		driver.BufferBindingShaderStorage|driver.BufferBindingTexture)
	if err != nil {
		return err
	}
	g.output.image = img
	if g.useCPU {
		g.output.cpuImage = cpu.NewImageRGBA(size.X, size.Y)
	}
	g.output.size = size
	return nil
}

func (g *compute) Release() {
	if g.useCPU {
		g.dispatcher.Stop()
	}
	g.programs.elements.Release()
	g.programs.tileAlloc.Release()
	g.programs.pathCoarse.Release()
	g.programs.backdrop.Release()
	g.programs.binning.Release()
	g.programs.coarse.Release()
	g.programs.kernel4.Release()
	if p := g.output.blitProg; p != nil {
		p.Release()
	}
	g.buffers.scene.release()
	g.buffers.state.release()
	g.buffers.memory.release()
	g.buffers.config.release()
	if g.output.image != nil {
		g.output.image.Release()
	}
	g.output.cpuImage.Free()
	if g.images.tex != nil {
		g.images.tex.Release()
	}
	if g.materials.layout != nil {
		g.materials.layout.Release()
	}
	if g.materials.prog != nil {
		g.materials.prog.Release()
	}
	if g.materials.fbo != nil {
		g.materials.fbo.Release()
	}
	if g.materials.tex != nil {
		g.materials.tex.Release()
	}
	g.materials.cpuTex.Free()
	if g.materials.buffer != nil {
		g.materials.buffer.Release()
	}
	if g.timers.t != nil {
		g.timers.t.release()
	}

	*g = compute{}
}

func (g *compute) bindBuffers() {
	g.bindStorageBuffers(g.programs.elements, g.buffers.memory, g.buffers.config, g.buffers.scene, g.buffers.state)
	g.bindStorageBuffers(g.programs.tileAlloc, g.buffers.memory, g.buffers.config)
	g.bindStorageBuffers(g.programs.pathCoarse, g.buffers.memory, g.buffers.config)
	g.bindStorageBuffers(g.programs.backdrop, g.buffers.memory, g.buffers.config)
	g.bindStorageBuffers(g.programs.binning, g.buffers.memory, g.buffers.config)
	g.bindStorageBuffers(g.programs.coarse, g.buffers.memory, g.buffers.config)
	g.bindStorageBuffers(g.programs.kernel4, g.buffers.memory, g.buffers.config)
}

func (p *computeProgram) Release() {
	if p.prog != nil {
		p.prog.Release()
	}
	*p = computeProgram{}
}

func (b *sizedBuffer) release() {
	if b.buffer != nil {
		b.buffer.Release()
	}
	b.cpuBuf.Free()
	*b = sizedBuffer{}
}

func (b *sizedBuffer) ensureCapacity(g *compute, size int) error {
	if b.size >= size {
		return nil
	}
	if b.buffer != nil {
		b.release()
	}
	b.cpuBuf.Free()
	if !g.useCPU {
		buf, err := g.ctx.NewBuffer(driver.BufferBindingShaderStorage, size)
		if err != nil {
			return err
		}
		b.buffer = buf
	} else {
		b.cpuBuf = cpu.NewBuffer(size)
	}
	b.size = size
	return nil
}

func (b *sizedBuffer) download(data []byte) error {
	if b.buffer != nil {
		return b.buffer.Download(data)
	} else {
		copy(data, b.cpuBuf.Data())
		return nil
	}
}

func (b *sizedBuffer) upload(data []byte) {
	if b.buffer != nil {
		b.buffer.Upload(data)
	} else {
		copy(b.cpuBuf.Data(), data)
	}
}

func (g *compute) bindStorageBuffers(prog computeProgram, buffers ...sizedBuffer) {
	for i, buf := range buffers {
		if !g.useCPU {
			prog.prog.SetStorageBuffer(i, buf.buffer)
		} else {
			*prog.buffers[i] = buf.cpuBuf
		}
	}
}

var bo = binary.LittleEndian

func (e *encoder) reset() {
	e.scene = e.scene[:0]
	e.npath = 0
	e.npathseg = 0
	e.ntrans = 0
}

func (e *encoder) numElements() int {
	return len(e.scene)
}

func (e *encoder) append(e2 encoder) {
	e.scene = append(e.scene, e2.scene...)
	e.npath += e2.npath
	e.npathseg += e2.npathseg
	e.ntrans += e2.ntrans
}

func (e *encoder) transform(m f32.Affine2D) {
	e.scene = append(e.scene, scene.Transform(m))
	e.ntrans++
}

func (e *encoder) lineWidth(width float32) {
	e.scene = append(e.scene, scene.SetLineWidth(width))
}

func (e *encoder) fillMode(mode scene.FillMode) {
	e.scene = append(e.scene, scene.SetFillMode(mode))
}

func (e *encoder) beginClip(bbox f32.Rectangle) {
	e.scene = append(e.scene, scene.BeginClip(bbox))
	e.npath++
}

func (e *encoder) endClip(bbox f32.Rectangle) {
	e.scene = append(e.scene, scene.EndClip(bbox))
	e.npath++
}

func (e *encoder) rect(r f32.Rectangle) {
	// Rectangle corners, clock-wise.
	c0, c1, c2, c3 := r.Min, f32.Pt(r.Min.X, r.Max.Y), r.Max, f32.Pt(r.Max.X, r.Min.Y)
	e.line(c0, c1)
	e.line(c1, c2)
	e.line(c2, c3)
	e.line(c3, c0)
}

func (e *encoder) fillColor(col color.RGBA) {
	e.scene = append(e.scene, scene.FillColor(col))
	e.npath++
}

func (e *encoder) setFillImageOffset(index int, offset image.Point) {
	x := int16(offset.X)
	y := int16(offset.Y)
	e.scene[index][2] = uint32(uint16(x)) | uint32(uint16(y))<<16
}

func (e *encoder) fillImage(index int) int {
	idx := len(e.scene)
	e.scene = append(e.scene, scene.FillImage(index))
	e.npath++
	return idx
}

func (e *encoder) line(start, end f32.Point) {
	e.scene = append(e.scene, scene.Line(start, end))
	e.npathseg++
}

func (e *encoder) quad(start, ctrl, end f32.Point) {
	e.scene = append(e.scene, scene.Quad(start, ctrl, end))
	e.npathseg++
}

func (c *collector) reset() {
	c.enc.reset()
	c.profile = false
	c.pathCache = c.pathCache[:0]
	c.texOps = c.texOps[:0]
}

func (c *collector) addClip(state *encoderState, bounds f32.Rectangle, rect bool, path []byte, stroke clip.StrokeStyle) {
	r := transformBounds(state.t, bounds)
	bounds = r.Bounds()
	state.intersect = state.intersect.Intersect(bounds)
	if rect {
		// If any previous clip paths are fully contained in this rectangle
		// clip, it can be skipped.
		p := state.path
		for p != nil {
			if p.rect.In(r) {
				//log.Printf("%v containing %v: %v bounds: %v bsize: %v intersect: %v isize: %v", r, p.rect, p.rect.In(r), bounds, bounds.Size(), state.intersect, state.intersect.Size())
				return
			}
			p = p.parent
		}
		/* // Cull parent clip rects that are redundant given this rectangle.
		p = state.path
		var prev *pathState
		for p != nil {
			if p.isRect && p.rect.Contains(r) {
				log.Println("skipping2", p.rect)
				if p == state.path {
					state.path = p.parent
				} else {
					prev.parent = p.parent
				}
			} else {
				prev = p
			}
			p = p.parent
		}*/
	}
	c.pathCache = append(c.pathCache, pathState{
		parent:    state.path,
		bounds:    bounds,
		intersect: state.intersect,
		trans:     state.t,
		stroke:    stroke,
		pathVerts: path,
		rect:      r,
		isRect:    rect,
	})
	state.path = &c.pathCache[len(c.pathCache)-1]
}

func (c *collector) collect(ctx driver.Device, root *op.Ops, trans f32.Affine2D, viewport image.Point) {
	fview := f32.Rectangle{Max: layout.FPt(viewport)}
	if c.clear {
		c.enc.rect(fview)
		c.enc.fillColor(f32color.NRGBAToRGBA(c.clearColor.SRGB()))
	}
	c.reader.Reset(root)
	state := encoderState{
		color:     color.NRGBA{A: 0xff},
		intersect: fview,
		t:         trans,
	}
	// Clip to the viewport.
	c.addClip(&state, fview, true, nil, clip.StrokeStyle{})
	r := &c.reader
	var (
		pathData []byte
		str      clip.StrokeStyle
		fillMode = scene.FillModeNonzero
	)
	c.save(opconst.InitialStateID, state)
	for encOp, ok := r.Decode(); ok; encOp, ok = r.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypeProfile:
			c.profile = true
		case opconst.TypeTransform:
			dop := ops.DecodeTransform(encOp.Data)
			state.t = state.t.Mul(dop)
		case opconst.TypeStroke:
			str = decodeStrokeOp(encOp.Data)
		case opconst.TypePath:
			encOp, ok = r.Decode()
			if !ok {
				panic("unexpected end of path operation")
			}
			pathData = encOp.Data[opconst.TypeAuxLen:]

		case opconst.TypeClip:
			var op clipOp
			op.decode(encOp.Data)
			c.addClip(&state, op.bounds, op.rect, pathData, str)
			pathData = nil
			str = clip.StrokeStyle{}
		case opconst.TypeColor:
			state.matType = materialColor
			state.color = decodeColorOp(encOp.Data)
		case opconst.TypeLinearGradient:
			state.matType = materialLinearGradient
			op := decodeLinearGradientOp(encOp.Data)
			state.stop1 = op.stop1
			state.stop2 = op.stop2
			state.color1 = op.color1
			state.color2 = op.color2
		case opconst.TypeImage:
			state.matType = materialTexture
			state.image = decodeImageOp(encOp.Data, encOp.Refs)
		case opconst.TypePaint:
			if state.matType == materialTexture {
				// Clip to the bounds of the image, to hide other images in the atlas.
				bounds := state.image.src.Bounds()
				c.addClip(&state, layout.FRect(bounds), true, nil, clip.StrokeStyle{})
			}
			if state.intersect.Empty() {
				break
			}
			/*opaque := false
			switch state.matType {
			case materialColor:
				opaque = state.color.A == 1.0
			case materialLinearGradient:
				opaque = state.color1.A == 1.0 && state.color2.A == 1.0
			}

			if bounds == (image.Rectangle{Max: c.viewport}) && state.rect && mat.opaque && (mat.material == materialColor) {
				// The image is a uniform opaque color and takes up the whole screen.
				// Scrap images up to and including this image and set clear color.
				c.paintOps = c.paintOps[:0]
				c.clearColor = mat.color.Opaque()
				c.clear = true
				continue
			}*/

			c.clipStack = c.clipStack[:0]
			c.buildClipStack(state.path.bounds, state.path)

			//log.Println("clip stack", c.clipStack)

			var inv f32.Affine2D
			for i, cl := range c.clipStack {
				if str := cl.path.stroke; str.Width > 0 {
					c.enc.fillMode(scene.FillModeStroke)
					c.enc.lineWidth(str.Width)
					fillMode = scene.FillModeStroke
				} else if fillMode != scene.FillModeNonzero {
					c.enc.fillMode(scene.FillModeNonzero)
					fillMode = scene.FillModeNonzero
				}
				c.enc.transform(inv)
				if cl.path.isRect {
					r := cl.path.rect
					c.enc.line(r[0], r[1])
					c.enc.line(r[1], r[2])
					c.enc.line(r[2], r[3])
					c.enc.line(r[3], r[0])
					inv = f32.Affine2D{}
				} else {
					c.enc.transform(cl.path.trans)
					inv = cl.path.trans.Invert()
					c.enc.encodePath(cl.path.pathVerts)
				}
				if i != len(c.clipStack)-1 {
					c.enc.beginClip(cl.union)
				}
			}
			//log.Println("draw", state.matType, "inter", state.intersect)
			switch state.matType {
			case materialTexture:
				// Add fill command. Its offset is resolved and filled in renderMaterials.
				idx := c.enc.fillImage(0)
				c.texOps = append(c.texOps, textureOp{
					sceneIdx: idx,
					img:      state.image,
					key: textureKey{
						transform: state.t,
						handle:    state.image.handle,
					},
				})
			case materialColor:
				c.enc.fillColor(f32color.NRGBAToRGBA(state.color))
			case materialLinearGradient:
				// TODO: implement.
				c.enc.fillColor(f32color.NRGBAToRGBA(state.color1))
			default:
				panic("not implemented")
			}
			c.enc.transform(inv)
			// Pop the clip stack, except the last entry used for fill.
			for i := len(c.clipStack) - 2; i >= 0; i-- {
				cl := c.clipStack[i]
				c.enc.endClip(cl.union)
			}
		case opconst.TypeSave:
			id := ops.DecodeSave(encOp.Data)
			c.save(id, state)
		case opconst.TypeLoad:
			id, mask := ops.DecodeLoad(encOp.Data)
			s := c.states[id]
			if mask&opconst.TransformState != 0 {
				state.t = s.t
			}
			if mask&^opconst.TransformState != 0 {
				state = s
			}
		}
	}
}

func (c *collector) save(id int, state encoderState) {
	if extra := id - len(c.states) + 1; extra > 0 {
		c.states = append(c.states, make([]encoderState, extra)...)
	}
	c.states[id] = state
}

func transformBounds(t f32.Affine2D, bounds f32.Rectangle) rectangle {
	return rectangle{
		t.Transform(bounds.Min), t.Transform(f32.Pt(bounds.Max.X, bounds.Min.Y)),
		t.Transform(bounds.Max), t.Transform(f32.Pt(bounds.Min.X, bounds.Max.Y)),
	}
}

// In reports whether r is in r2.
func (r rectangle) In(r2 rectangle) bool {
	// For every corner in r, compute the signed dot product with each edge in r2.
	// If all products have the same sign (or zero), the point is inside r2 (or on
	// one of the edges).
	// If all corners are inside r2, r must also be inside r2.
	edges := [4]f32.Point{r2[1].Sub(r2[0]), r2[2].Sub(r2[1]), r2[3].Sub(r2[2]), r2[0].Sub(r2[3])}
	for _, p := range r {
		var sign float32
		for i, e := range edges {
			v := p.Sub(r2[i])
			dot := v.X*e.X + v.Y*e.Y
			if dot == 0 {
				// Count points on the edge as inside.
				continue
			}
			if sign != 0 && (dot > 0) != (sign > 0) {
				return false
			}
			sign = dot
		}
	}
	return true
}

func (r rectangle) Bounds() f32.Rectangle {
	bounds := f32.Rectangle{
		Min: f32.Pt(math.MaxFloat32, math.MaxFloat32),
		Max: f32.Pt(-math.MaxFloat32, -math.MaxFloat32),
	}
	for _, c := range r {
		if c.X < bounds.Min.X {
			bounds.Min.X = c.X
		}
		if c.Y < bounds.Min.Y {
			bounds.Min.Y = c.Y
		}
		if c.X > bounds.Max.X {
			bounds.Max.X = c.X
		}
		if c.Y > bounds.Max.Y {
			bounds.Max.Y = c.Y
		}
	}
	return bounds
}
