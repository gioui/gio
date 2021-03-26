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
)

type compute struct {
	ctx driver.Device

	collector     collector
	enc           encoder
	texOps        []textureOp
	viewport      image.Point
	maxTextureDim int

	programs struct {
		elements   driver.Program
		tileAlloc  driver.Program
		pathCoarse driver.Program
		backdrop   driver.Program
		binning    driver.Program
		coarse     driver.Program
		kernel4    driver.Program
	}
	buffers struct {
		config driver.Buffer
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

		buffer sizedBuffer

		uniforms *materialUniforms
		uniBuf   driver.Buffer
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

	// The following fields hold scratch space to avoid garbage.
	zeroSlice []byte
	memHeader *memoryHeader
	conf      *config
}

type materialUniforms struct {
	scale [2]float32
	pos   [2]float32
}

type collector struct {
	profile      bool
	reader       ops.Reader
	states       []encoderState
	clear        bool
	clearColor   f32color.RGBA
	clipCache    []clipState
	clipCmdCache []clipCmd
	paintOps     []paintOp
}

type paintOp struct {
	clipStack []clipCmd
	state     encoderState
}

// clipCmd describes a clipping command ready to be used for the compute
// pipeline.
type clipCmd struct {
	// union of the bounds of the operations that are clipped.
	union    f32.Rectangle
	state    *clipState
	relTrans f32.Affine2D
}

type encoderState struct {
	t         f32.Affine2D
	relTrans  f32.Affine2D
	clip      *clipState
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

type clipState struct {
	bounds    f32.Rectangle
	absBounds f32.Rectangle
	pathVerts []byte
	parent    *clipState
	relTrans  f32.Affine2D
	stroke    clip.StrokeStyle
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
	img      imageOpData
	key      textureKey
	// offset is the integer offset, separated from key.transform to increase cache hit rate.
	off image.Point

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

type sizedBuffer struct {
	size   int
	buffer driver.Buffer
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
	maxDim := ctx.Caps().MaxTextureSize
	// Large atlas textures cause artifacts due to precision loss in
	// shaders.
	if cap := 8192; maxDim > cap {
		maxDim = cap
	}
	g := &compute{
		ctx:           ctx,
		maxTextureDim: maxDim,
		conf:          new(config),
		memHeader:     new(memoryHeader),
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
	g.materials.uniforms = new(materialUniforms)

	buf, err := ctx.NewBuffer(driver.BufferBindingUniforms, int(unsafe.Sizeof(*g.materials.uniforms)))
	if err != nil {
		g.Release()
		return nil, err
	}
	g.materials.uniBuf = buf
	g.materials.prog.SetVertexUniforms(buf)

	buf, err = ctx.NewBuffer(driver.BufferBindingShaderStorage, int(unsafe.Sizeof(config{})))
	if err != nil {
		g.Release()
		return nil, err
	}
	g.buffers.config = buf

	shaders := []struct {
		prog *driver.Program
		src  driver.ShaderSources
	}{
		{&g.programs.elements, shader_elements_comp},
		{&g.programs.tileAlloc, shader_tile_alloc_comp},
		{&g.programs.pathCoarse, shader_path_coarse_comp},
		{&g.programs.backdrop, shader_backdrop_comp},
		{&g.programs.binning, shader_binning_comp},
		{&g.programs.coarse, shader_coarse_comp},
		{&g.programs.kernel4, shader_kernel4_comp},
	}
	for _, shader := range shaders {
		p, err := ctx.NewComputeProgram(shader.src)
		if err != nil {
			g.Release()
			return nil, err
		}
		*shader.prog = p
	}
	return g, nil
}

func (g *compute) Collect(viewport image.Point, ops *op.Ops) {
	g.viewport = viewport
	g.collector.reset()
	g.enc.reset()
	g.texOps = g.texOps[:0]

	// Flip Y-axis.
	flipY := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(1, -1)).Offset(f32.Pt(0, float32(viewport.Y)))
	g.collector.collect(ops, flipY, viewport)
	g.collector.encode(viewport, &g.enc, &g.texOps)
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

	defFBO := g.ctx.BeginFrame(g.collector.clear, viewport)
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
	resize := false
	reclaimed := false
restart:
	for {
		for _, op := range g.texOps {
			if off, exists := m.offsets[op.key]; exists {
				g.enc.setFillImageOffset(op.sceneIdx, off.Sub(op.off))
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
			g.enc.setFillImageOffset(op.sceneIdx, offset.Sub(op.off))
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
		handle, err := g.ctx.NewTexture(driver.TextureFormatRGBA8, texSize, texSize,
			driver.FilterNearest, driver.FilterNearest,
			driver.BufferBindingShaderStorage|driver.BufferBindingFramebuffer)
		if err != nil {
			return fmt.Errorf("compute: failed to create material atlas: %v", err)
		}
		fbo, err := g.ctx.NewFramebuffer(handle, 0)
		if err != nil {
			handle.Release()
			return fmt.Errorf("compute: failed to create material framebuffer: %v", err)
		}
		m.tex = handle
		m.fbo = fbo
	}
	// Transform to clip space: [-1, -1] - [1, 1].
	g.materials.uniforms.scale = [2]float32{2 / float32(texSize), 2 / float32(texSize)}
	g.materials.uniforms.pos = [2]float32{-1, -1}
	g.materials.uniBuf.Upload(byteslice.Struct(g.materials.uniforms))
	vertexData := byteslice.Slice(m.quads)
	n := pow2Ceil(len(vertexData))
	m.buffer.ensureCapacity(g.ctx, driver.BufferBindingVertices, n)
	m.buffer.buffer.Upload(vertexData)
	g.ctx.BindTexture(0, g.images.tex)
	g.ctx.BindFramebuffer(m.fbo)
	g.ctx.Viewport(0, 0, texSize, texSize)
	if reclaimed {
		g.ctx.Clear(0, 0, 0, 0)
	}
	g.ctx.BindProgram(m.prog)
	g.ctx.BindVertexBuffer(m.buffer.buffer, int(unsafe.Sizeof(m.quads[0])), 0)
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
		for i, op := range g.texOps {
			if pos, exists := a.positions[op.img.handle]; exists {
				g.texOps[i].pos = pos
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
			g.texOps[i].pos = place.Pos
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
		a.tex.Upload(image.Pt(pos.X+size.X, pos.Y), rightPadding, g.zeros(rightPadding.X*rightPadding.Y*4), 0)
		bottomPadding := image.Pt(size.X, padding)
		a.tex.Upload(image.Pt(pos.X, pos.Y+size.Y), bottomPadding, g.zeros(bottomPadding.X*bottomPadding.Y*4), 0)
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

	enc := &g.enc
	// Pad scene with zeroes to avoid reading garbage in elements.comp.
	scenePadding := partitionSize - len(enc.scene)%partitionSize
	enc.scene = append(enc.scene, make([]scene.Command, scenePadding)...)

	realloced := false
	scene := byteslice.Slice(enc.scene)
	if s := len(scene); s > g.buffers.scene.size {
		realloced = true
		paddedCap := s * 11 / 10
		if err := g.buffers.scene.ensureCapacity(g.ctx, driver.BufferBindingShaderStorage, paddedCap); err != nil {
			return err
		}
	}
	g.buffers.scene.buffer.Upload(scene)

	w, h := tileDims.X*tileWidthPx, tileDims.Y*tileHeightPx
	if g.output.size.X != w || g.output.size.Y != h {
		if err := g.resizeOutput(image.Pt(w, h)); err != nil {
			return err
		}
	}
	g.ctx.BindImageTexture(kernel4OutputUnit, g.output.image, driver.AccessWrite, driver.TextureFormatRGBA8)
	if t := g.materials.tex; t != nil {
		g.ctx.BindImageTexture(kernel4AtlasUnit, t, driver.AccessRead, driver.TextureFormatRGBA8)
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
		if err := g.buffers.state.ensureCapacity(g.ctx, driver.BufferBindingShaderStorage, paddedCap); err != nil {
			return err
		}
	}

	g.buffers.config.Upload(byteslice.Struct(g.conf))

	minSize := int(unsafe.Sizeof(memoryHeader{})) + int(alloc)
	if minSize > g.buffers.memory.size {
		realloced = true
		// Add space for dynamic GPU allocations.
		const sizeBump = 4 * 1024 * 1024
		minSize += sizeBump
		if err := g.buffers.memory.ensureCapacity(g.ctx, driver.BufferBindingShaderStorage, minSize); err != nil {
			return err
		}
	}
	for {
		*g.memHeader = memoryHeader{
			mem_offset: alloc,
		}
		g.buffers.memory.buffer.Upload(byteslice.Struct(g.memHeader))
		g.buffers.state.buffer.Upload(g.zeros(clearSize))

		if realloced {
			realloced = false
			g.bindBuffers()
		}
		t := &g.timers
		g.ctx.MemoryBarrier()
		t.elements.begin()
		g.ctx.BindProgram(g.programs.elements)
		g.ctx.DispatchCompute(numPartitions, 1, 1)
		g.ctx.MemoryBarrier()
		t.elements.end()
		t.tileAlloc.begin()
		g.ctx.BindProgram(g.programs.tileAlloc)
		g.ctx.DispatchCompute((enc.npath+wgSize-1)/wgSize, 1, 1)
		g.ctx.MemoryBarrier()
		t.tileAlloc.end()
		t.pathCoarse.begin()
		g.ctx.BindProgram(g.programs.pathCoarse)
		g.ctx.DispatchCompute((enc.npathseg+31)/32, 1, 1)
		g.ctx.MemoryBarrier()
		t.pathCoarse.end()
		t.backdropBinning.begin()
		g.ctx.BindProgram(g.programs.backdrop)
		g.ctx.DispatchCompute((enc.npath+wgSize-1)/wgSize, 1, 1)
		// No barrier needed between backdrop and binning.
		g.ctx.BindProgram(g.programs.binning)
		g.ctx.DispatchCompute((enc.npath+wgSize-1)/wgSize, 1, 1)
		g.ctx.MemoryBarrier()
		t.backdropBinning.end()
		t.coarse.begin()
		g.ctx.BindProgram(g.programs.coarse)
		g.ctx.DispatchCompute(widthInBins, heightInBins, 1)
		g.ctx.MemoryBarrier()
		t.coarse.end()
		t.kernel4.begin()
		g.ctx.BindProgram(g.programs.kernel4)
		g.ctx.DispatchCompute(tileDims.X, tileDims.Y, 1)
		g.ctx.MemoryBarrier()
		t.kernel4.end()

		if err := g.buffers.memory.buffer.Download(byteslice.Struct(g.memHeader)); err != nil {
			if err == driver.ErrContentLost {
				continue
			}
			return err
		}
		switch errCode := g.memHeader.mem_error; errCode {
		case memNoError:
			return nil
		case memMallocFailed:
			// Resize memory and try again.
			realloced = true
			sz := g.buffers.memory.size * 15 / 10
			if err := g.buffers.memory.ensureCapacity(g.ctx, driver.BufferBindingShaderStorage, sz); err != nil {
				return err
			}
			continue
		default:
			return fmt.Errorf("compute: shader program failed with error %d", errCode)
		}
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
	img, err := g.ctx.NewTexture(driver.TextureFormatRGBA8, size.X, size.Y,
		driver.FilterNearest,
		driver.FilterNearest,
		driver.BufferBindingShaderStorage|driver.BufferBindingTexture)
	if err != nil {
		return err
	}
	g.output.image = img
	g.output.size = size
	return nil
}

func (g *compute) Release() {
	progs := []driver.Program{
		g.programs.elements,
		g.programs.tileAlloc,
		g.programs.pathCoarse,
		g.programs.backdrop,
		g.programs.binning,
		g.programs.coarse,
		g.programs.kernel4,
	}
	if p := g.output.blitProg; p != nil {
		p.Release()
	}
	for _, p := range progs {
		if p != nil {
			p.Release()
		}
	}
	g.buffers.scene.release()
	g.buffers.state.release()
	g.buffers.memory.release()
	if b := g.buffers.config; b != nil {
		b.Release()
	}
	if g.output.image != nil {
		g.output.image.Release()
	}
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
	g.materials.buffer.release()
	if b := g.materials.uniBuf; b != nil {
		b.Release()
	}
	if g.timers.t != nil {
		g.timers.t.release()
	}

	*g = compute{}
}

func (g *compute) bindBuffers() {
	bindStorageBuffers(g.programs.elements, g.buffers.memory.buffer, g.buffers.config, g.buffers.scene.buffer, g.buffers.state.buffer)
	bindStorageBuffers(g.programs.tileAlloc, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.pathCoarse, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.backdrop, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.binning, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.coarse, g.buffers.memory.buffer, g.buffers.config)
	bindStorageBuffers(g.programs.kernel4, g.buffers.memory.buffer, g.buffers.config)
}

func (b *sizedBuffer) release() {
	if b.buffer == nil {
		return
	}
	b.buffer.Release()
	*b = sizedBuffer{}
}

func (b *sizedBuffer) ensureCapacity(ctx driver.Device, binding driver.BufferBinding, size int) error {
	if b.size >= size {
		return nil
	}
	if b.buffer != nil {
		b.release()
	}
	buf, err := ctx.NewBuffer(binding, size)
	if err != nil {
		return err
	}
	b.buffer = buf
	b.size = size
	return nil
}

func bindStorageBuffers(prog driver.Program, buffers ...driver.Buffer) {
	for i, buf := range buffers {
		prog.SetStorageBuffer(i, buf)
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
	c.profile = false
	c.clipCache = c.clipCache[:0]
	c.clipCmdCache = c.clipCmdCache[:0]
	c.paintOps = c.paintOps[:0]
}

func (c *collector) addClip(state *encoderState, viewport, bounds f32.Rectangle, path []byte, stroke clip.StrokeStyle) {
	// Rectangle clip regions.
	if len(path) == 0 {
		transView := transformBounds(state.t.Invert(), viewport)
		// If the rectangular clip contains the viewport it can be discarded.
		if transView.In(bounds) {
			return
		}
		// If the rectangular clip region contains a previous path it can be discarded.
		p := state.clip
		t := state.relTrans.Invert()
		for p != nil {
			// rect is the parent bounds transformed relative to the rectangle.
			rect := transformBounds(t, p.bounds)
			if rect.In(bounds) {
				return
			}
			t = p.relTrans.Invert().Mul(t)
			p = p.parent
		}
	}

	absBounds := transformBounds(state.t, bounds).Bounds()
	c.clipCache = append(c.clipCache, clipState{
		parent:    state.clip,
		bounds:    bounds,
		absBounds: absBounds,
		relTrans:  state.relTrans,
		stroke:    stroke,
		pathVerts: path,
	})
	state.intersect = state.intersect.Intersect(absBounds)
	state.clip = &c.clipCache[len(c.clipCache)-1]
	state.relTrans = f32.Affine2D{}
}

func (c *collector) collect(root *op.Ops, trans f32.Affine2D, viewport image.Point) {
	fview := f32.Rectangle{Max: layout.FPt(viewport)}
	c.reader.Reset(root)
	state := encoderState{
		color:     color.NRGBA{A: 0xff},
		intersect: fview,
		t:         trans,
		relTrans:  trans,
	}
	r := &c.reader
	var (
		pathData []byte
		str      clip.StrokeStyle
	)
	c.save(opconst.InitialStateID, state)
	for encOp, ok := r.Decode(); ok; encOp, ok = r.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypeProfile:
			c.profile = true
		case opconst.TypeTransform:
			dop := ops.DecodeTransform(encOp.Data)
			state.t = state.t.Mul(dop)
			state.relTrans = state.relTrans.Mul(dop)
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
			c.addClip(&state, fview, op.bounds, pathData, str)
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
			paintState := state
			if paintState.matType == materialTexture {
				// Clip to the bounds of the image, to hide other images in the atlas.
				bounds := paintState.image.src.Bounds()
				c.addClip(&paintState, fview, layout.FRect(bounds), nil, clip.StrokeStyle{})
			}
			if paintState.intersect.Empty() {
				break
			}

			// If the paint is a uniform opaque color that takes up the whole
			// screen, it covers all previous paints and we can discard all
			// rendering commands recorded so far.
			if paintState.clip == nil && paintState.matType == materialColor && paintState.color.A == 255 {
				c.clearColor = f32color.LinearFromSRGB(paintState.color).Opaque()
				c.clear = true
				c.paintOps = c.paintOps[:0]
				break
			}

			// Flatten clip stack.
			p := paintState.clip
			startIdx := len(c.clipCmdCache)
			for p != nil {
				c.clipCmdCache = append(c.clipCmdCache, clipCmd{state: p, relTrans: p.relTrans})
				p = p.parent
			}
			clipStack := c.clipCmdCache[startIdx:]
			c.paintOps = append(c.paintOps, paintOp{
				clipStack: clipStack,
				state:     paintState,
			})
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
	for i := range c.paintOps {
		op := &c.paintOps[i]
		// For each clip, cull rectangular clip regions that contain its
		// (transformed) bounds. addClip already handled the converse case.
		// TODO: do better than O(n²) to efficiently deal with deep stacks.
		for i := 0; i < len(op.clipStack)-1; i++ {
			cl := op.clipStack[i]
			p := cl.state
			r := transformBounds(cl.relTrans, p.bounds)
			for j := i + 1; j < len(op.clipStack); j++ {
				cl2 := op.clipStack[j]
				p2 := cl2.state
				if len(p2.pathVerts) == 0 && r.In(p2.bounds) {
					op.clipStack = append(op.clipStack[:j], op.clipStack[j+1:]...)
					j--
					op.clipStack[j].relTrans = cl2.relTrans.Mul(op.clipStack[j].relTrans)
				}
				r = transformRect(cl2.relTrans, r)
			}
		}
	}
}

func (c *collector) encode(viewport image.Point, enc *encoder, texOps *[]textureOp) {
	fview := f32.Rectangle{Max: layout.FPt(viewport)}
	fillMode := scene.FillModeNonzero
	if c.clear {
		enc.rect(fview)
		enc.fillColor(f32color.NRGBAToRGBA(c.clearColor.SRGB()))
	}
	for _, op := range c.paintOps {
		// Fill in clip bounds, which the shaders expect to be the union
		// of all affected bounds.
		var union f32.Rectangle
		for i, cl := range op.clipStack {
			union = union.Union(cl.state.absBounds)
			op.clipStack[i].union = union
		}

		var inv f32.Affine2D
		for i := len(op.clipStack) - 1; i >= 0; i-- {
			cl := op.clipStack[i]
			if str := cl.state.stroke; str.Width > 0 {
				enc.fillMode(scene.FillModeStroke)
				enc.lineWidth(str.Width)
				fillMode = scene.FillModeStroke
			} else if fillMode != scene.FillModeNonzero {
				enc.fillMode(scene.FillModeNonzero)
				fillMode = scene.FillModeNonzero
			}
			enc.transform(cl.relTrans)
			inv = inv.Mul(cl.relTrans)
			if len(cl.state.pathVerts) == 0 {
				enc.rect(cl.state.bounds)
			} else {
				enc.encodePath(cl.state.pathVerts)
			}
			if i != 0 {
				enc.beginClip(cl.union)
			}
		}
		if op.state.clip == nil {
			// No clipping; fill the entire view.
			enc.rect(fview)
		}

		switch op.state.matType {
		case materialTexture:
			// Add fill command. Its offset is resolved and filled in renderMaterials.
			idx := enc.fillImage(0)
			sx, hx, ox, hy, sy, oy := op.state.t.Elems()
			// Separate integer offset from transformation. TextureOps that have identical transforms
			// except for their integer offsets can share a transformed image.
			intx, fracx := math.Modf(float64(ox))
			inty, fracy := math.Modf(float64(oy))
			t := f32.NewAffine2D(sx, hx, float32(fracx), hy, sy, float32(fracy))
			*texOps = append(*texOps, textureOp{
				sceneIdx: idx,
				img:      op.state.image,
				off:      image.Pt(int(intx), int(inty)),
				key: textureKey{
					transform: t,
					handle:    op.state.image.handle,
				},
			})
		case materialColor:
			enc.fillColor(f32color.NRGBAToRGBA(op.state.color))
		case materialLinearGradient:
			// TODO: implement.
			enc.fillColor(f32color.NRGBAToRGBA(op.state.color1))
		default:
			panic("not implemented")
		}
		enc.transform(inv.Invert())
		// Pop the clip stack, except the first entry used for fill.
		for i := 1; i < len(op.clipStack); i++ {
			cl := op.clipStack[i]
			enc.endClip(cl.union)
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

func transformRect(t f32.Affine2D, r rectangle) rectangle {
	var tr rectangle
	for i, c := range r {
		tr[i] = t.Transform(c)
	}
	return tr
}

func (r rectangle) In(b f32.Rectangle) bool {
	for _, c := range r {
		inside := b.Min.X <= c.X && c.X <= b.Max.X &&
			b.Min.Y <= c.Y && c.Y <= b.Max.Y
		if !inside {
			return false
		}
	}
	return true
}

func (r rectangle) Contains(b f32.Rectangle) bool {
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
