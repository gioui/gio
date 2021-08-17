// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/maphash"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"math"
	"math/bits"
	"runtime"
	"sort"
	"time"
	"unsafe"

	"gioui.org/cpu"
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
	"gioui.org/shader"
	"gioui.org/shader/gio"
	"gioui.org/shader/piet"
)

type compute struct {
	ctx driver.Device

	collector     collector
	enc           encoder
	texOps        []textureOp
	viewport      image.Point
	maxTextureDim int
	srgb          bool

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
		blitProg driver.Program
		layout   driver.InputLayout

		buffer sizedBuffer

		uniforms *copyUniforms
		uniBuf   driver.Buffer

		layerVertices []layerVertex
		layerAtlases  []*layerAtlas
		packer        packer

		descriptors *piet.Kernel4DescriptorSetLayout
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

		vert struct {
			uniforms *materialVertUniforms
			buf      driver.Buffer
		}

		frag struct {
			buf driver.Buffer
		}

		// CPU fields
		cpuTex cpu.ImageDescriptor
		// regions track new materials in tex, so they can be transferred to cpuTex.
		regions []image.Rectangle
		scratch []byte
	}
	timers struct {
		profile string
		t       *timers
		compact *timer
		render  *timer
		blit    *timer
	}

	// CPU fallback fields.
	useCPU     bool
	dispatcher *dispatcher

	// The following fields hold scratch space to avoid garbage.
	zeroSlice []byte
	memHeader *memoryHeader
	conf      *config
}

type layer struct {
	rect     image.Rectangle
	place    layerPlace
	newPlace layerPlace
	ops      []paintOp
}

type layerPlace struct {
	atlas *layerAtlas
	pos   image.Point
}

type layerAtlas struct {
	// image is the layer atlas texture. Note that it is in RGBA format,
	// but contains data in sRGB. See blitLayers for more detail.
	image    driver.Texture
	fbo      driver.Framebuffer
	cpuImage cpu.ImageDescriptor
	size     image.Point
	layers   int
}

type copyUniforms struct {
	scale   [2]float32
	pos     [2]float32
	uvScale [2]float32
	_       [8]byte // Pad to 16 bytes.
}

type materialVertUniforms struct {
	scale [2]float32
	pos   [2]float32
}

type materialFragUniforms struct {
	emulateSRGB float32
	_           [12]byte // Pad to 16 bytes
}

type collector struct {
	hasher     maphash.Hash
	profile    bool
	reader     ops.Reader
	states     []encoderState
	clear      bool
	clearColor f32color.RGBA
	clipStates []clipState
	order      []hashIndex
	prevFrame  opsCollector
	frame      opsCollector
}

type hashIndex struct {
	index int
	hash  uint64
}

type opsCollector struct {
	paths    []byte
	clipCmds []clipCmd
	ops      []paintOp
	layers   []layer
}

type paintOp struct {
	clipStack []clipCmd
	offset    image.Point
	state     paintKey
	intersect f32.Rectangle
	hash      uint64
	layer     int
}

// clipCmd describes a clipping command ready to be used for the compute
// pipeline.
type clipCmd struct {
	// union of the bounds of the operations that are clipped.
	union     f32.Rectangle
	state     clipKey
	path      []byte
	pathKey   ops.Key
	absBounds f32.Rectangle
}

type encoderState struct {
	relTrans  f32.Affine2D
	clip      *clipState
	intersect f32.Rectangle

	paintKey
}

// clipKey completely describes a clip operation (along with its path) and is appropriate
// for hashing and equality checks.
type clipKey struct {
	bounds   f32.Rectangle
	stroke   clip.StrokeStyle
	relTrans f32.Affine2D
	pathHash uint64
}

// paintKey completely defines a paint operation. It is suitable for hashing and
// equality checks.
type paintKey struct {
	t       f32.Affine2D
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
	absBounds f32.Rectangle
	parent    *clipState
	path      []byte
	pathKey   ops.Key

	clipKey
}

type layerVertex struct {
	posX, posY float32
	u, v       float32
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
	g := &compute{
		ctx:           ctx,
		maxTextureDim: maxDim,
		srgb:          caps.Features.Has(driver.FeatureSRGB),
		conf:          new(config),
		memHeader:     new(memoryHeader),
	}
	shaders := []struct {
		prog *computeProgram
		src  shader.Sources
		info *cpu.ProgramInfo
	}{
		{&g.programs.elements, piet.Shader_elements_comp, piet.ElementsProgramInfo},
		{&g.programs.tileAlloc, piet.Shader_tile_alloc_comp, piet.Tile_allocProgramInfo},
		{&g.programs.pathCoarse, piet.Shader_path_coarse_comp, piet.Path_coarseProgramInfo},
		{&g.programs.backdrop, piet.Shader_backdrop_comp, piet.BackdropProgramInfo},
		{&g.programs.binning, piet.Shader_binning_comp, piet.BinningProgramInfo},
		{&g.programs.coarse, piet.Shader_coarse_comp, piet.CoarseProgramInfo},
		{&g.programs.kernel4, piet.Shader_kernel4_comp, piet.Kernel4ProgramInfo},
	}
	if !caps.Features.Has(driver.FeatureCompute) {
		if !cpu.Supported {
			return nil, errors.New("gpu: missing support for compute programs")
		}
		g.useCPU = true
	}
	if g.useCPU {
		g.dispatcher = newDispatcher(runtime.NumCPU())
	}

	// Large enough for reasonable fill sizes, yet still spannable by the compute programs.
	g.output.packer.maxDim = 4096
	blitProg, err := ctx.NewProgram(gio.Shader_copy_vert, gio.Shader_copy_frag)
	if err != nil {
		g.Release()
		return nil, err
	}
	g.output.blitProg = blitProg
	progLayout, err := ctx.NewInputLayout(gio.Shader_copy_vert, []shader.InputDesc{
		{Type: shader.DataTypeFloat, Size: 2, Offset: 0},
		{Type: shader.DataTypeFloat, Size: 2, Offset: 4 * 2},
	})
	if err != nil {
		g.Release()
		return nil, err
	}
	g.output.layout = progLayout
	g.output.uniforms = new(copyUniforms)

	buf, err := ctx.NewBuffer(driver.BufferBindingUniforms, int(unsafe.Sizeof(*g.output.uniforms)))
	if err != nil {
		g.Release()
		return nil, err
	}
	g.output.uniBuf = buf
	g.output.blitProg.SetVertexUniforms(buf)

	materialProg, err := ctx.NewProgram(gio.Shader_material_vert, gio.Shader_material_frag)
	if err != nil {
		g.Release()
		return nil, err
	}
	g.materials.prog = materialProg
	progLayout, err = ctx.NewInputLayout(gio.Shader_material_vert, []shader.InputDesc{
		{Type: shader.DataTypeFloat, Size: 2, Offset: 0},
		{Type: shader.DataTypeFloat, Size: 2, Offset: 4 * 2},
	})
	if err != nil {
		g.Release()
		return nil, err
	}
	g.materials.layout = progLayout
	g.materials.vert.uniforms = new(materialVertUniforms)

	buf, err = ctx.NewBuffer(driver.BufferBindingUniforms, int(unsafe.Sizeof(*g.materials.vert.uniforms)))
	if err != nil {
		g.Release()
		return nil, err
	}
	g.materials.vert.buf = buf
	g.materials.prog.SetVertexUniforms(buf)
	var emulateSRGB materialFragUniforms
	if !g.srgb {
		emulateSRGB.emulateSRGB = 1.0
	}
	buf, err = ctx.NewBuffer(driver.BufferBindingUniforms, int(unsafe.Sizeof(emulateSRGB)))
	if err != nil {
		g.Release()
		return nil, err
	}
	buf.Upload(byteslice.Struct(&emulateSRGB))
	g.materials.frag.buf = buf
	g.materials.prog.SetFragmentUniforms(buf)

	for _, shader := range shaders {
		if !g.useCPU {
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
			desc := new(piet.ElementsDescriptorSetLayout)
			g.programs.elements.descriptors = unsafe.Pointer(desc)
			g.programs.elements.buffers = []*cpu.BufferDescriptor{desc.Binding0(), desc.Binding1(), desc.Binding2(), desc.Binding3()}
		}
		{
			desc := new(piet.Tile_allocDescriptorSetLayout)
			g.programs.tileAlloc.descriptors = unsafe.Pointer(desc)
			g.programs.tileAlloc.buffers = []*cpu.BufferDescriptor{desc.Binding0(), desc.Binding1()}
		}
		{
			desc := new(piet.Path_coarseDescriptorSetLayout)
			g.programs.pathCoarse.descriptors = unsafe.Pointer(desc)
			g.programs.pathCoarse.buffers = []*cpu.BufferDescriptor{desc.Binding0(), desc.Binding1()}
		}
		{
			desc := new(piet.BackdropDescriptorSetLayout)
			g.programs.backdrop.descriptors = unsafe.Pointer(desc)
			g.programs.backdrop.buffers = []*cpu.BufferDescriptor{desc.Binding0(), desc.Binding1()}
		}
		{
			desc := new(piet.BinningDescriptorSetLayout)
			g.programs.binning.descriptors = unsafe.Pointer(desc)
			g.programs.binning.buffers = []*cpu.BufferDescriptor{desc.Binding0(), desc.Binding1()}
		}
		{
			desc := new(piet.CoarseDescriptorSetLayout)
			g.programs.coarse.descriptors = unsafe.Pointer(desc)
			g.programs.coarse.buffers = []*cpu.BufferDescriptor{desc.Binding0(), desc.Binding1()}
		}
		{
			desc := new(piet.Kernel4DescriptorSetLayout)
			g.programs.kernel4.descriptors = unsafe.Pointer(desc)
			g.programs.kernel4.buffers = []*cpu.BufferDescriptor{desc.Binding0(), desc.Binding1()}
			g.output.descriptors = desc
		}
	}
	return g, nil
}

func (g *compute) Collect(viewport image.Point, ops *op.Ops) {
	g.viewport = viewport
	g.collector.reset()
	for i := range g.output.layerAtlases {
		g.output.layerAtlases[i].layers = 0
	}

	g.collector.collect(ops, viewport)
	g.collector.layer(viewport)
}

func (g *compute) Clear(col color.NRGBA) {
	g.collector.clear = true
	g.collector.clearColor = f32color.LinearFromSRGB(col)
}

func (g *compute) Frame(target RenderTarget) error {
	viewport := g.viewport
	defFBO := g.ctx.BeginFrame(target, g.collector.clear, viewport)
	defer g.ctx.EndFrame()

	t := &g.timers
	if g.collector.profile && t.t == nil && g.ctx.Caps().Features.Has(driver.FeatureTimers) {
		t.t = newTimers(g.ctx)
		t.compact = t.t.newTimer()
		t.render = t.t.newTimer()
		t.blit = t.t.newTimer()
	}

	g.ctx.BindFramebuffer(defFBO)
	if g.collector.clear {
		g.collector.clear = false
		g.ctx.Clear(g.collector.clearColor.Float32())
	}

	t.compact.begin()
	if err := g.compactLayers(); err != nil {
		return err
	}
	t.compact.end()
	t.render.begin()
	if err := g.renderLayers(viewport); err != nil {
		return err
	}
	t.render.end()
	g.ctx.BindFramebuffer(defFBO)
	t.blit.begin()
	g.blitLayers(viewport)
	t.blit.end()
	if g.collector.profile && t.t.ready() {
		com, ren, blit := t.compact.Elapsed, t.render.Elapsed, t.blit.Elapsed
		ft := com + ren + blit
		q := 100 * time.Microsecond
		ft = ft.Round(q)
		com, ren, blit = com.Round(q), ren.Round(q), blit.Round(q)
		t.profile = fmt.Sprintf("ft:%7s com: %7s ren:%7s blit:%7s", ft, com, ren, blit)
	}
	return nil
}

func (g *compute) dumpAtlases() {
	for i, a := range g.output.layerAtlases {
		dump, err := driver.DownloadImage(g.ctx, a.fbo, image.Rectangle{Max: a.size})
		if err != nil {
			panic(err)
		}
		nrgba := image.NewNRGBA(dump.Bounds())
		bnd := dump.Bounds()
		for x := bnd.Min.X; x < bnd.Max.X; x++ {
			for y := bnd.Min.Y; y < bnd.Max.Y; y++ {
				nrgba.SetNRGBA(x, y, f32color.RGBAToNRGBA(dump.RGBAAt(x, y)))
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, nrgba); err != nil {
			panic(err)
		}
		if err := ioutil.WriteFile(fmt.Sprintf("dump-%d.png", i), buf.Bytes(), 0600); err != nil {
			panic(err)
		}
	}
}

func (g *compute) Profile() string {
	return g.timers.profile
}

func (g *compute) compactLayers() error {
	layers := g.collector.frame.layers
	for len(layers) > 0 {
		var atlas *layerAtlas
		addedLayers := false
		end := 0
		for end < len(layers) {
			l := &layers[end]
			if l.place.atlas == nil {
				end++
				continue
			}
			l.newPlace = l.place
			if atlas == nil {
				atlas = g.newAtlas()
				g.output.packer.clear()
				g.output.packer.newPage()
			}
			size := l.rect.Size()
			place, fits := g.output.packer.tryAdd(size.Add(image.Pt(1, 1)))
			if !fits {
				if !addedLayers {
					panic(fmt.Errorf("compute: internal error: empty atlas no longer fits layer (layer: %v)", size))
				}
				break
			}
			addedLayers = true
			l.newPlace = layerPlace{
				atlas: atlas,
				pos:   place.Pos,
			}
			atlas.layers++
			end++
		}
		if !addedLayers {
			layers = layers[end:]
			continue
		}
		outputSize := g.output.packer.sizes[0]
		atlas.ensureSize(g.useCPU, g.ctx, outputSize)
		for i, l := range layers[:end] {
			if l.newPlace == l.place {
				continue
			}
			src := l.place.atlas.fbo
			dst := atlas.fbo
			sz := l.rect.Size()
			sr := image.Rectangle{Min: l.place.pos, Max: l.place.pos.Add(sz)}
			dr := image.Rectangle{Min: l.newPlace.pos, Max: l.newPlace.pos.Add(sz)}
			g.ctx.BlitFramebuffer(dst, src, sr, dr)
			l.place.atlas.layers--
			if l.place.atlas.layers == 0 {
				l.place.atlas.fbo.Invalidate()
			}
			layers[i].place = l.newPlace
		}
		layers = layers[end:]
	}
	return nil
}

func (g *compute) renderLayers(viewport image.Point) error {
	layers := g.collector.frame.layers
	for len(layers) > 0 {
		var atlas *layerAtlas
		addedLayers := false
		for len(layers) > 0 {
			l := &layers[0]
			if a := l.place.atlas; a != nil {
				a.layers++
				layers = layers[1:]
				continue
			}
			if atlas == nil {
				atlas = g.newAtlas()
				g.output.packer.clear()
				g.output.packer.newPage()
				g.enc.reset()
				g.texOps = g.texOps[:0]
			}
			// Position onto atlas; pad to avoid overlap.
			size := l.rect.Size()
			place, fits := g.output.packer.tryAdd(size.Add(image.Pt(1, 1)))
			if !fits {
				if !addedLayers {
					// The maximum compute output is either smaller than the window, or an operation
					// in the layer wasn't clipped to the window.
					panic(fmt.Errorf("compute: internal error: layer larger than maximum compute output (viewport: %v, layer: %v)", viewport, size))
				}
				break
			}
			addedLayers = true
			l.place = layerPlace{
				atlas: atlas,
				pos:   place.Pos,
			}
			atlas.layers++
			encodeLayer(*l, place.Pos, viewport, &g.enc, &g.texOps)
			layers = layers[1:]
		}
		if !addedLayers {
			break
		}
		if err := g.uploadImages(); err != nil {
			return err
		}
		if err := g.renderMaterials(); err != nil {
			return err
		}
		outputSize := g.output.packer.sizes[0]
		tileDims := image.Point{
			X: (outputSize.X + tileWidthPx - 1) / tileWidthPx,
			Y: (outputSize.Y + tileHeightPx - 1) / tileHeightPx,
		}
		w, h := tileDims.X*tileWidthPx, tileDims.Y*tileHeightPx
		if err := atlas.ensureSize(g.useCPU, g.ctx, image.Pt(w, h)); err != nil {
			return err
		}
		if err := g.render(atlas.image, atlas.cpuImage, tileDims, atlas.size.X*4); err != nil {
			return err
		}
	}
	return nil
}

func (g *compute) newAtlas() *layerAtlas {
	// Look for empty atlas to re-use.
	for _, a := range g.output.layerAtlases {
		if a.layers == 0 {
			return a
		}
	}
	a := new(layerAtlas)
	g.output.layerAtlases = append(g.output.layerAtlases, a)
	return a
}

func (g *compute) blitLayers(viewport image.Point) {
	if len(g.collector.frame.layers) == 0 {
		return
	}
	layers := g.collector.frame.layers
	g.ctx.BlendFunc(driver.BlendFactorOne, driver.BlendFactorOneMinusSrcAlpha)
	g.ctx.SetBlend(true)
	defer g.ctx.SetBlend(false)
	g.ctx.Viewport(0, 0, viewport.X, viewport.Y)
	g.ctx.BindProgram(g.output.blitProg)
	g.ctx.BindInputLayout(g.output.layout)
	for len(layers) > 0 {
		g.output.layerVertices = g.output.layerVertices[:0]
		atlas := layers[0].place.atlas
		for len(layers) > 0 {
			l := layers[0]
			if l.place.atlas != atlas {
				break
			}
			placef := layout.FPt(l.place.pos)
			sizef := layout.FPt(l.rect.Size())
			quad := [4]layerVertex{
				{posX: float32(l.rect.Min.X), posY: float32(l.rect.Min.Y), u: placef.X, v: placef.Y},
				{posX: float32(l.rect.Max.X), posY: float32(l.rect.Min.Y), u: placef.X + sizef.X, v: placef.Y},
				{posX: float32(l.rect.Max.X), posY: float32(l.rect.Max.Y), u: placef.X + sizef.X, v: placef.Y + sizef.Y},
				{posX: float32(l.rect.Min.X), posY: float32(l.rect.Max.Y), u: placef.X, v: placef.Y + sizef.Y},
			}
			g.output.layerVertices = append(g.output.layerVertices, quad[0], quad[1], quad[3], quad[3], quad[2], quad[1])
			layers = layers[1:]
		}

		// Transform positions to clip space: [-1, -1] - [1, 1], and texture
		// coordinates to texture space: [0, 0] - [1, 1].
		clip := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(2/float32(viewport.X), 2/float32(viewport.Y))).Offset(f32.Pt(-1, -1))
		// Flip y-axis to match framebuffer output space.
		flipY := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(1, -1)).Offset(f32.Pt(0, float32(viewport.Y)))
		clip = clip.Mul(flipY)
		sx, _, ox, _, sy, oy := clip.Elems()
		g.output.uniforms.scale = [2]float32{sx, sy}
		g.output.uniforms.pos = [2]float32{ox, oy}
		g.output.uniforms.uvScale = [2]float32{1 / float32(atlas.size.X), 1 / float32(atlas.size.Y)}
		g.output.uniBuf.Upload(byteslice.Struct(g.output.uniforms))
		vertexData := byteslice.Slice(g.output.layerVertices)
		g.output.buffer.ensureCapacity(false, g.ctx, driver.BufferBindingVertices, len(vertexData))
		g.output.buffer.buffer.Upload(vertexData)
		g.ctx.BindVertexBuffer(g.output.buffer.buffer, int(unsafe.Sizeof(g.output.layerVertices[0])), 0)
		g.ctx.BindTexture(0, atlas.image)
		g.ctx.DrawArrays(driver.DrawModeTriangles, 0, len(g.output.layerVertices))
	}
}

func (g *compute) renderMaterials() error {
	m := &g.materials
	m.quads = m.quads[:0]
	m.regions = m.regions[:0]
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
		fbo, err := g.ctx.NewFramebuffer(handle)
		if err != nil {
			handle.Release()
			return fmt.Errorf("compute: failed to create material framebuffer: %v", err)
		}
		m.tex = handle
		m.fbo = fbo
		if g.useCPU {
			m.cpuTex = cpu.NewImageRGBA(texSize, texSize)
		}
	}
	// Transform to clip space: [-1, -1] - [1, 1].
	g.materials.vert.uniforms.scale = [2]float32{2 / float32(texSize), 2 / float32(texSize)}
	g.materials.vert.uniforms.pos = [2]float32{-1, -1}
	g.materials.vert.buf.Upload(byteslice.Struct(g.materials.vert.uniforms))
	vertexData := byteslice.Slice(m.quads)
	n := pow2Ceil(len(vertexData))
	m.buffer.ensureCapacity(false, g.ctx, driver.BufferBindingVertices, n)
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
		format := driver.TextureFormatSRGBA
		if !g.srgb {
			format = driver.TextureFormatRGBA8
		}
		handle, err := g.ctx.NewTexture(format, sz, sz, driver.FilterLinear, driver.FilterLinear, driver.BufferBindingTexture)
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

func (g *compute) render(dst driver.Texture, cpuDst cpu.ImageDescriptor, tileDims image.Point, stride int) error {
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
		if err := g.buffers.scene.ensureCapacity(g.useCPU, g.ctx, driver.BufferBindingShaderStorage, paddedCap); err != nil {
			return err
		}
	}
	g.buffers.scene.upload(scene)

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
		if err := g.buffers.state.ensureCapacity(g.useCPU, g.ctx, driver.BufferBindingShaderStorage, paddedCap); err != nil {
			return err
		}
	}

	confData := byteslice.Struct(g.conf)
	g.buffers.config.ensureCapacity(g.useCPU, g.ctx, driver.BufferBindingShaderStorage, len(confData))
	g.buffers.config.upload(confData)

	minSize := int(unsafe.Sizeof(memoryHeader{})) + int(alloc)
	if minSize > g.buffers.memory.size {
		realloced = true
		// Add space for dynamic GPU allocations.
		const sizeBump = 4 * 1024 * 1024
		minSize += sizeBump
		if err := g.buffers.memory.ensureCapacity(g.useCPU, g.ctx, driver.BufferBindingShaderStorage, minSize); err != nil {
			return err
		}
	}

	if !g.useCPU {
		g.ctx.BindImageTexture(kernel4OutputUnit, dst, driver.AccessWrite, driver.TextureFormatRGBA8)
		if t := g.materials.tex; t != nil {
			g.ctx.BindImageTexture(kernel4AtlasUnit, t, driver.AccessRead, driver.TextureFormatRGBA8)
		}
	} else {
		*g.output.descriptors.Binding2() = cpuDst
		*g.output.descriptors.Binding3() = g.materials.cpuTex
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
		g.memoryBarrier()
		g.dispatch(g.programs.elements, numPartitions, 1, 1)
		g.memoryBarrier()
		g.dispatch(g.programs.tileAlloc, (enc.npath+wgSize-1)/wgSize, 1, 1)
		g.memoryBarrier()
		g.dispatch(g.programs.pathCoarse, (enc.npathseg+31)/32, 1, 1)
		g.memoryBarrier()
		g.dispatch(g.programs.backdrop, (enc.npath+wgSize-1)/wgSize, 1, 1)
		// No barrier needed between backdrop and binning.
		g.dispatch(g.programs.binning, (enc.npath+wgSize-1)/wgSize, 1, 1)
		g.memoryBarrier()
		g.dispatch(g.programs.coarse, widthInBins, heightInBins, 1)
		g.memoryBarrier()
		g.downloadMaterials()
		g.dispatch(g.programs.kernel4, tileDims.X, tileDims.Y, 1)
		g.memoryBarrier()
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
				w, h := tileDims.X*tileWidthPx, tileDims.Y*tileHeightPx
				dst.Upload(image.Pt(0, 0), image.Pt(w, h), cpuDst.Data(), stride)
			}
			return nil
		case memMallocFailed:
			// Resize memory and try again.
			realloced = true
			sz := g.buffers.memory.size * 15 / 10
			if err := g.buffers.memory.ensureCapacity(g.useCPU, g.ctx, driver.BufferBindingShaderStorage, sz); err != nil {
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

func (a *layerAtlas) ensureSize(useCPU bool, ctx driver.Device, size image.Point) error {
	if a.size.X >= size.X && a.size.Y >= size.Y {
		return nil
	}
	size.X, size.Y = pow2Ceil(size.X), pow2Ceil(size.Y)
	if a.fbo != nil {
		a.fbo.Release()
		a.fbo = nil
	}
	if a.image != nil {
		a.image.Release()
		a.image = nil
	}
	a.cpuImage.Free()

	img, err := ctx.NewTexture(driver.TextureFormatRGBA8, size.X, size.Y,
		driver.FilterNearest,
		driver.FilterNearest,
		driver.BufferBindingShaderStorage|driver.BufferBindingTexture|driver.BufferBindingFramebuffer)
	if err != nil {
		return err
	}
	fbo, err := ctx.NewFramebuffer(img)
	if err != nil {
		img.Release()
		return err
	}
	a.fbo = fbo
	a.image = img
	if useCPU {
		a.cpuImage = cpu.NewImageRGBA(size.X, size.Y)
	}
	a.size = size
	return nil
}

func (g *compute) Release() {
	if g.useCPU {
		g.dispatcher.Stop()
	}
	type resource interface {
		Release()
	}
	res := []resource{
		&g.programs.elements,
		&g.programs.tileAlloc,
		&g.programs.pathCoarse,
		&g.programs.backdrop,
		&g.programs.binning,
		&g.programs.coarse,
		&g.programs.kernel4,
		g.output.blitProg,
		&g.output.buffer,
		g.output.uniBuf,
		&g.buffers.scene,
		&g.buffers.state,
		&g.buffers.memory,
		&g.buffers.config,
		g.images.tex,
		g.materials.layout,
		g.materials.prog,
		g.materials.fbo,
		g.materials.tex,
		&g.materials.buffer,
		g.materials.vert.buf,
		g.materials.frag.buf,
		g.timers.t,
	}
	g.materials.cpuTex.Free()
	for _, r := range res {
		if r != nil {
			r.Release()
		}
	}
	for _, a := range g.output.layerAtlases {
		if a.fbo != nil {
			a.fbo.Release()
		}
		if a.image != nil {
			a.image.Release()
		}
		a.cpuImage.Free()
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

func (b *sizedBuffer) Release() {
	if b.buffer == nil {
		return
	}
	b.cpuBuf.Free()
	*b = sizedBuffer{}
}

func (b *sizedBuffer) ensureCapacity(useCPU bool, ctx driver.Device, binding driver.BufferBinding, size int) error {
	if b.size >= size {
		return nil
	}
	if b.buffer != nil {
		b.Release()
	}
	b.cpuBuf.Free()
	if !useCPU {
		buf, err := ctx.NewBuffer(binding, size)
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
	if e.scene[index].Op() != scene.OpFillImage {
		panic("invalid fill image command")
	}
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
	c.prevFrame, c.frame = c.frame, c.prevFrame
	c.profile = false
	c.clipStates = c.clipStates[:0]
	c.frame.reset()
}

func (c *opsCollector) reset() {
	c.paths = c.paths[:0]
	c.clipCmds = c.clipCmds[:0]
	c.ops = c.ops[:0]
	c.layers = c.layers[:0]
}

func (c *collector) addClip(state *encoderState, viewport, bounds f32.Rectangle, path []byte, key ops.Key, hash uint64, stroke clip.StrokeStyle) {
	// Rectangle clip regions.
	if len(path) == 0 {
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
	c.clipStates = append(c.clipStates, clipState{
		parent:    state.clip,
		absBounds: absBounds,
		path:      path,
		pathKey:   key,
		clipKey: clipKey{
			bounds:   bounds,
			relTrans: state.relTrans,
			stroke:   stroke,
			pathHash: hash,
		},
	})
	state.intersect = state.intersect.Intersect(absBounds)
	state.clip = &c.clipStates[len(c.clipStates)-1]
	state.relTrans = f32.Affine2D{}
}

func (c *collector) collect(root *op.Ops, viewport image.Point) {
	fview := f32.Rectangle{Max: layout.FPt(viewport)}
	c.reader.Reset(root)
	state := encoderState{
		intersect: fview,
		paintKey: paintKey{
			color: color.NRGBA{A: 0xff},
		},
	}
	r := &c.reader
	var (
		pathData struct {
			data []byte
			key  ops.Key
			hash uint64
		}
		str clip.StrokeStyle
	)
	c.save(opconst.InitialStateID, state)
	c.addClip(&state, fview, fview, nil, ops.Key{}, 0, clip.StrokeStyle{})
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
			hash := bo.Uint64(encOp.Data[1:])
			encOp, ok = r.Decode()
			if !ok {
				panic("unexpected end of path operation")
			}
			pathData.data = encOp.Data[opconst.TypeAuxLen:]
			pathData.key = encOp.Key
			pathData.hash = hash
		case opconst.TypeClip:
			var op clipOp
			op.decode(encOp.Data)
			c.addClip(&state, fview, op.bounds, pathData.data, pathData.key, pathData.hash, str)
			pathData.data = nil
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
				c.addClip(&paintState, fview, layout.FRect(bounds), nil, ops.Key{}, 0, clip.StrokeStyle{})
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
				c.frame.reset()
				break
			}

			// Flatten clip stack.
			p := paintState.clip
			startIdx := len(c.frame.clipCmds)
			for p != nil {
				idx := len(c.frame.paths)
				c.frame.paths = append(c.frame.paths, make([]byte, len(p.path))...)
				path := c.frame.paths[idx:]
				copy(path, p.path)
				c.frame.clipCmds = append(c.frame.clipCmds, clipCmd{
					state:     p.clipKey,
					path:      path,
					pathKey:   p.pathKey,
					absBounds: p.absBounds,
				})
				p = p.parent
			}
			clipStack := c.frame.clipCmds[startIdx:]
			c.frame.ops = append(c.frame.ops, paintOp{
				clipStack: clipStack,
				state:     paintState.paintKey,
				intersect: paintState.intersect,
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
	for i := range c.frame.ops {
		op := &c.frame.ops[i]
		// For each clip, cull rectangular clip regions that contain its
		// (transformed) bounds. addClip already handled the converse case.
		// TODO: do better than O(n²) to efficiently deal with deep stacks.
		for j := 0; j < len(op.clipStack)-1; j++ {
			cl := op.clipStack[j]
			p := cl.state
			r := transformBounds(p.relTrans, p.bounds)
			for k := j + 1; k < len(op.clipStack); k++ {
				cl2 := op.clipStack[k]
				p2 := cl2.state
				if len(cl2.path) == 0 && r.In(cl2.state.bounds) {
					op.clipStack = append(op.clipStack[:k], op.clipStack[k+1:]...)
					k--
					op.clipStack[k].state.relTrans = p2.relTrans.Mul(op.clipStack[k].state.relTrans)
				}
				r = transformRect(p2.relTrans, r)
			}
		}
		// Separate the integer offset from the first transform. Two ops that differ
		// only in integer offsets may share backing storage.
		if len(op.clipStack) > 0 {
			c := &op.clipStack[len(op.clipStack)-1]
			t := c.state.relTrans
			t, off := separateTransform(t)
			c.state.relTrans = t
			op.offset = off
			op.state.t = op.state.t.Offset(layout.FPt(off.Mul(-1)))
		}
		op.hash = c.hashOp(*op)
	}
}

func (c *collector) hashOp(op paintOp) uint64 {
	c.hasher.Reset()
	for _, cl := range op.clipStack {
		k := cl.state
		keyBytes := (*[unsafe.Sizeof(k)]byte)(unsafe.Pointer(unsafe.Pointer(&k)))
		c.hasher.Write(keyBytes[:])
	}
	k := op.state
	keyBytes := (*[unsafe.Sizeof(k)]byte)(unsafe.Pointer(unsafe.Pointer(&k)))
	c.hasher.Write(keyBytes[:])
	return c.hasher.Sum64()
}

func (c *collector) layer(viewport image.Point) {
	// Sort ops from previous frames by hash.
	prevOps := c.prevFrame.ops
	c.order = c.order[:0]
	for i, op := range prevOps {
		c.order = append(c.order, hashIndex{
			index: i,
			hash:  op.hash,
		})
	}
	sort.Slice(c.order, func(i, j int) bool {
		return c.order[i].hash < c.order[j].hash
	})
	addLayer := func(l layer) {
		for i, op := range l.ops {
			l.rect = l.rect.Union(boundRectF(op.intersect))
			l.ops[i].layer = len(c.frame.layers)
		}
		c.frame.layers = append(c.frame.layers, l)
		if l.place.atlas != nil {
			l.place.atlas.layers++
		}
	}
	ops := c.frame.ops
	idx := 0
	for idx < len(ops) {
		op := ops[idx]
		// Search for longest matching op sequence.
		// start is the earliest index of a match.
		start := searchOp(c.order, op.hash)
		layerOps, layerIdx := longestLayer(prevOps, c.order[start:], ops[idx:])
		if len(layerOps) == 0 {
			idx++
			continue
		}
		if unmatched := ops[:idx]; len(unmatched) > 0 {
			// Flush layer of unmatched ops.
			addLayer(layer{ops: unmatched})
			ops = ops[idx:]
			idx = 0
		}
		l := c.prevFrame.layers[layerIdx]
		var place layerPlace
		if len(l.ops) == len(layerOps) {
			place = l.place
		}
		addLayer(layer{ops: layerOps, place: place})
		ops = ops[len(layerOps):]
	}
	if len(ops) > 0 {
		addLayer(layer{ops: ops})
	}
}

func longestLayer(prev []paintOp, order []hashIndex, ops []paintOp) ([]paintOp, int) {
	longest := 0
	longestIdx := -1
outer:
	for len(order) > 0 {
		first := order[0]
		order = order[1:]
		match := prev[first.index:]
		// Potential match found. Now find longest matching sequence.
		end := 0
		layer := match[0].layer
		off := match[0].offset.Sub(ops[0].offset)
		for end < len(match) && end < len(ops) {
			m := match[end]
			o := ops[end]
			// End on layer boundaries.
			if m.layer != layer {
				break
			}
			// End layer when the next op doesn't match.
			if m.hash != o.hash {
				if end == 0 {
					// Hashes are sorted so if the first op doesn't match, no
					// more matches are possible.
					break outer
				}
				break
			}
			if !opEqual(off, m, o) {
				break
			}
			end++
		}
		if end > longest {
			longest = end
			longestIdx = layer
		}
	}
	return ops[:longest], longestIdx
}

func searchOp(order []hashIndex, hash uint64) int {
	lo, hi := 0, len(order)
	for lo < hi {
		mid := (lo + hi) / 2
		if order[mid].hash < hash {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

func opEqual(off image.Point, o1 paintOp, o2 paintOp) bool {
	if len(o1.clipStack) != len(o2.clipStack) {
		return false
	}
	if o1.state != o2.state {
		return false
	}
	if o1.offset.Sub(o2.offset) != off {
		return false
	}
	for i, cl1 := range o1.clipStack {
		cl2 := o2.clipStack[i]
		if len(cl1.path) != len(cl2.path) {
			return false
		}
		if cl1.state != cl2.state {
			return false
		}
		if cl1.pathKey != cl2.pathKey && !bytes.Equal(cl1.path, cl2.path) {
			return false
		}
	}
	return true
}

func encodeLayer(l layer, pos image.Point, viewport image.Point, enc *encoder, texOps *[]textureOp) {
	off := pos.Sub(l.rect.Min)
	offf := layout.FPt(off)

	enc.transform(f32.Affine2D{}.Offset(offf))
	for _, op := range l.ops {
		encodeOp(viewport, offf, enc, texOps, op)
	}
	enc.transform(f32.Affine2D{}.Offset(offf.Mul(-1)))
}

func encodeOp(viewport image.Point, absOff f32.Point, enc *encoder, texOps *[]textureOp, op paintOp) {
	// Fill in clip bounds, which the shaders expect to be the union
	// of all affected bounds.
	var union f32.Rectangle
	for i, cl := range op.clipStack {
		union = union.Union(cl.absBounds)
		op.clipStack[i].union = union
	}

	fillMode := scene.FillModeNonzero
	opOff := layout.FPt(op.offset)
	inv := f32.Affine2D{}.Offset(opOff)
	enc.transform(inv)
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
		enc.transform(cl.state.relTrans)
		inv = inv.Mul(cl.state.relTrans)
		if len(cl.path) == 0 {
			enc.rect(cl.state.bounds)
		} else {
			enc.encodePath(cl.path)
		}
		if i != 0 {
			enc.beginClip(cl.union.Add(absOff))
		}
	}
	if len(op.clipStack) == 0 {
		// No clipping; fill the entire view.
		enc.rect(f32.Rectangle{Max: layout.FPt(viewport)})
	}

	switch op.state.matType {
	case materialTexture:
		// Add fill command. Its offset is resolved and filled in renderMaterials.
		idx := enc.fillImage(0)
		// Separate integer offset from transformation. TextureOps that have identical transforms
		// except for their integer offsets can share a transformed image.
		t := op.state.t.Offset(absOff.Add(opOff))
		t, off := separateTransform(t)
		*texOps = append(*texOps, textureOp{
			sceneIdx: idx,
			img:      op.state.image,
			off:      off,
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
		enc.endClip(cl.union.Add(absOff))
	}
	if fillMode != scene.FillModeNonzero {
		enc.fillMode(scene.FillModeNonzero)
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

func separateTransform(t f32.Affine2D) (f32.Affine2D, image.Point) {
	sx, hx, ox, hy, sy, oy := t.Elems()
	intx, fracx := math.Modf(float64(ox))
	inty, fracy := math.Modf(float64(oy))
	t = f32.NewAffine2D(sx, hx, float32(fracx), hy, sy, float32(fracy))
	return t, image.Pt(int(intx), int(inty))
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
