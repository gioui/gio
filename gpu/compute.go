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
	"gioui.org/gpu/backend"
	"gioui.org/internal/f32color"
	gunsafe "gioui.org/internal/unsafe"
	"gioui.org/layout"
	"gioui.org/op"
)

type compute struct {
	ctx backend.Device
	enc encoder

	drawOps       drawOps
	cache         *resourceCache
	maxTextureDim int

	defFBO   backend.Framebuffer
	programs struct {
		elements   backend.Program
		tileAlloc  backend.Program
		pathCoarse backend.Program
		backdrop   backend.Program
		binning    backend.Program
		coarse     backend.Program
		kernel4    backend.Program
	}
	buffers struct {
		config backend.Buffer
		scene  sizedBuffer
		state  sizedBuffer
		memory sizedBuffer
	}
	output struct {
		size image.Point
		// image is the output texture. Note that it is in RGBA format,
		// but contains data in sRGB. See blitOutput for more detail.
		image    backend.Texture
		blitProg backend.Program
	}
	// images contains ImageOp images packed into a texture atlas.
	images struct {
		packer packer
		// positions maps imageOpData.handles to positions inside tex.
		positions map[interface{}]image.Point
		tex       backend.Texture
	}
	// materials contains the pre-processed materials (transformed images for
	// now, gradients etc. later) packed in a texture atlas. The atlas is used
	// as source in kernel4.
	materials struct {
		prog   backend.Program
		layout backend.InputLayout

		packer packer

		texSize image.Point
		tex     backend.Texture
		fbo     backend.Framebuffer
		quads   []materialVertex

		bufSize int
		buffer  backend.Buffer
	}
	timers struct {
		profile         string
		t               *timers
		elements        *timer
		tileAlloc       *timer
		pathCoarse      *timer
		backdropBinning *timer
		coarse          *timer
		kernel4         *timer
	}

	// The following fields hold scratch space to avoid garbage.
	zeroSlice []byte
	memHeader *memoryHeader
	conf      *config
}

// materialVertex describes a vertex of a quad used to render a transformed
// material.
type materialVertex struct {
	posX, posY float32
	u, v       float32
}

type encoder struct {
	scene    []sceneElem
	npath    int
	npathseg int
}

type encodeState struct {
	trans f32.Affine2D
	clip  f32.Rectangle
}

type sizedBuffer struct {
	size   int
	buffer backend.Buffer
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
}

type sceneElem [sceneElemSize / 4]uint32

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

// GPU structure sizes and constants.
const (
	tileWidthPx       = 32
	tileHeightPx      = 32
	ptclInitialAlloc  = 1024
	kernel4OutputUnit = 2
	kernel4AtlasUnit  = 3

	pathSize      = 12
	binSize       = 8
	pathsegSize   = 44
	annoSize      = 28
	stateSize     = 60
	stateStride   = 4 + 2*stateSize
	sceneElemSize = 36
)

// GPU commands from scene.h
const (
	elemNop = iota
	elemStrokeLine
	elemFillLine
	elemStrokeQuad
	elemFillQuad
	elemStrokeCubic
	elemFillCubic
	elemStroke
	elemFill
	elemLineWidth
	elemTransform
	elemBeginClip
	elemEndClip
	elemFillImage
)

const (
	flagEndPath = 16 // FLAG_END_PATH from elements.comp
)

// mem.h constants.
const (
	memNoError      = 0 // NO_ERROR
	memMallocFailed = 1 // ERR_MALLOC_FAILED
)

func newCompute(ctx backend.Device) (*compute, error) {
	maxDim := ctx.Caps().MaxTextureSize
	// Large atlas textures cause artifacts due to precision loss in
	// shaders.
	if cap := 8192; maxDim > cap {
		maxDim = cap
	}
	g := &compute{
		ctx:           ctx,
		defFBO:        ctx.CurrentFramebuffer(),
		cache:         newResourceCache(),
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
	progLayout, err := ctx.NewInputLayout(shader_material_vert, []backend.InputDesc{
		{Type: backend.DataTypeFloat, Size: 2, Offset: 0},
		{Type: backend.DataTypeFloat, Size: 2, Offset: 4 * 2},
	})
	if err != nil {
		g.Release()
		return nil, err
	}
	g.materials.layout = progLayout

	g.drawOps.pathCache = newOpCache()
	g.drawOps.retainPathData = true

	buf, err := ctx.NewBuffer(backend.BufferBindingShaderStorage, int(unsafe.Sizeof(config{})))
	if err != nil {
		g.Release()
		return nil, err
	}
	g.buffers.config = buf

	shaders := []struct {
		prog *backend.Program
		src  backend.ShaderSources
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
	g.drawOps.reset(g.cache, viewport)
	g.drawOps.collect(g.ctx, g.cache, ops, viewport)
	for _, img := range g.drawOps.allImageOps {
		expandPathOp(img.path, img.clip)
	}
	if g.drawOps.profile && g.timers.t == nil && g.ctx.Caps().Features.Has(backend.FeatureTimers) {
		t := &g.timers
		t.t = newTimers(g.ctx)
		t.elements = g.timers.t.newTimer()
		t.tileAlloc = g.timers.t.newTimer()
		t.pathCoarse = g.timers.t.newTimer()
		t.backdropBinning = g.timers.t.newTimer()
		t.coarse = g.timers.t.newTimer()
		t.kernel4 = g.timers.t.newTimer()
	}
}

func (g *compute) Clear(col color.NRGBA) {
	g.drawOps.clear = true
	g.drawOps.clearColor = f32color.LinearFromSRGB(col)
}

func (g *compute) Frame() error {
	viewport := g.drawOps.viewport
	tileDims := image.Point{
		X: (viewport.X + tileWidthPx - 1) / tileWidthPx,
		Y: (viewport.Y + tileHeightPx - 1) / tileHeightPx,
	}

	g.ctx.BeginFrame()
	defer g.ctx.EndFrame()

	if err := g.uploadImages(g.drawOps.allImageOps); err != nil {
		return err
	}
	if err := g.encode(viewport); err != nil {
		return err
	}
	if err := g.renderMaterials(); err != nil {
		return err
	}
	if err := g.render(tileDims); err != nil {
		return err
	}
	g.blitOutput(viewport)
	g.cache.frame()
	g.drawOps.pathCache.frame()
	t := &g.timers
	if g.drawOps.profile && t.t.ready() {
		et, tat, pct, bbt := t.elements.Elapsed, t.tileAlloc.Elapsed, t.pathCoarse.Elapsed, t.backdropBinning.Elapsed
		ct, k4t := t.coarse.Elapsed, t.kernel4.Elapsed
		ft := et + tat + pct + bbt + ct + k4t
		q := 100 * time.Microsecond
		ft = ft.Round(q)
		et, tat, pct, bbt = et.Round(q), tat.Round(q), pct.Round(q), bbt.Round(q)
		ct, k4t = ct.Round(q), k4t.Round(q)
		t.profile = fmt.Sprintf("ft:%7s et:%7s tat:%7s pct:%7s bbt:%7s ct:%7s k4t:%7s", ft, et, tat, pct, bbt, ct, k4t)
	}
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
	g.ctx.BindFramebuffer(g.defFBO)
	g.ctx.Viewport(0, 0, viewport.X, viewport.Y)
	g.ctx.BindTexture(0, g.output.image)
	g.ctx.BindProgram(g.output.blitProg)
	g.ctx.DrawArrays(backend.DrawModeTriangleStrip, 0, 4)
}

func (g *compute) encode(viewport image.Point) error {
	g.materials.packer.maxDim = g.maxTextureDim
	g.materials.packer.clear()
	g.materials.packer.newPage()
	g.materials.quads = g.materials.quads[:0]
	g.enc.reset()

	// Flip Y-axis.
	flipY := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(1, -1)).Offset(f32.Pt(0, float32(viewport.Y)))
	g.enc.transform(flipY)
	if g.drawOps.clear {
		g.drawOps.clear = false
		g.enc.rect(f32.Rectangle{Max: layout.FPt(viewport)}, false)
		g.enc.fill(f32color.NRGBAToRGBA(g.drawOps.clearColor.SRGB()))
	}
	return g.encodeOps(flipY, viewport, g.drawOps.allImageOps)
}

func (g *compute) uploadImages(ops []imageOp) error {
	// padding is the number of pixels added to the right and below
	// images, to avoid atlas filtering artifacts.
	const padding = 1

	a := &g.images
	var uploads map[interface{}]*image.RGBA
	resize := false
	reclaimed := false
restart:
	for {
		for _, op := range ops {
			switch m := op.material; m.material {
			case materialTexture:
				if _, exists := a.positions[m.data.handle]; exists {
					continue
				}
				size := m.data.src.Bounds().Size()
				size.X += padding
				size.Y += padding
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
					g.images.positions = make(map[interface{}]image.Point)
				}
				a.positions[m.data.handle] = place.Pos
				if uploads == nil {
					uploads = make(map[interface{}]*image.RGBA)
				}
				uploads[m.data.handle] = m.data.src
			}
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
		handle, err := g.ctx.NewTexture(backend.TextureFormatSRGB, sz, sz, backend.FilterLinear, backend.FilterLinear, backend.BufferBindingTexture)
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
		backend.UploadImage(a.tex, pos, img)
		rightPadding := image.Pt(padding, size.Y)
		a.tex.Upload(image.Pt(pos.X+size.X, pos.Y), rightPadding, g.zeros(rightPadding.X*rightPadding.Y*4))
		bottomPadding := image.Pt(size.X, padding)
		a.tex.Upload(image.Pt(pos.X, pos.Y+size.Y), bottomPadding, g.zeros(bottomPadding.X*bottomPadding.Y*4))
	}
	return nil
}

func (g *compute) renderMaterials() error {
	m := &g.materials
	outSize := g.materials.packer.sizes[0]
	if outSize == (image.Point{}) {
		return nil
	}
	if outSize.X > m.texSize.X || outSize.Y > m.texSize.Y {
		if m.fbo != nil {
			m.fbo.Release()
			m.fbo = nil
		}
		if m.tex != nil {
			m.tex.Release()
			m.tex = nil
		}
		// Round to nearest power of 2 while we're doing an expensive recreation anyway.
		sz := image.Pt(pow2Ceil(outSize.X), pow2Ceil(outSize.Y))
		m.texSize = sz
		handle, err := g.ctx.NewTexture(backend.TextureFormatRGBA8, sz.X, sz.Y, backend.FilterNearest, backend.FilterNearest, backend.BufferBindingShaderStorage|backend.BufferBindingFramebuffer)
		if err != nil {
			return fmt.Errorf("compute: failed to create material atlas: %v", err)
		}
		m.tex = handle
		fbo, err := g.ctx.NewFramebuffer(handle, 0)
		if err != nil {
			return fmt.Errorf("compute: failed to create material framebuffer: %v", err)
		}
		m.fbo = fbo
	}
	// TODO: move to shaders.
	// Transform to clip space: [-1, -1] - [1, 1].
	clip := f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(2/float32(m.texSize.X), 2/float32(m.texSize.Y))).Offset(f32.Pt(-1, -1))
	for i, v := range m.quads {
		p := clip.Transform(f32.Pt(v.posX, v.posY))
		m.quads[i].posX = p.X
		m.quads[i].posY = p.Y
	}
	vertexData := gunsafe.BytesView(m.quads)
	if len(vertexData) > m.bufSize {
		if m.buffer != nil {
			m.buffer.Release()
			m.buffer = nil
		}
		// Ditto.
		n := pow2Ceil(len(vertexData))
		buf, err := g.ctx.NewBuffer(backend.BufferBindingVertices, n)
		if err != nil {
			return err
		}
		m.bufSize = n
		m.buffer = buf
	}
	m.buffer.Upload(vertexData)
	g.ctx.BindTexture(0, g.images.tex)
	g.ctx.BindFramebuffer(m.fbo)
	g.ctx.Viewport(0, 0, m.texSize.X, m.texSize.Y)
	g.ctx.Clear(0, 0, 0, 0)
	g.ctx.BindProgram(m.prog)
	g.ctx.BindVertexBuffer(m.buffer, int(unsafe.Sizeof(m.quads[0])), 0)
	g.ctx.BindInputLayout(m.layout)
	g.ctx.DrawArrays(backend.DrawModeTriangles, 0, len(m.quads))
	return nil
}

func pow2Ceil(v int) int {
	exp := bits.Len(uint(v))
	if bits.OnesCount(uint(v)) == 1 {
		exp--
	}
	return 1 << exp
}

// addMaterialQuad appends a render of an image to materials and returns the pixel offset
// that maps the material texture to the correct position in the rendered image.
func (g *compute) addMaterialQuad(M f32.Affine2D, img imageOpData) (image.Point, error) {
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
	size := bounds.Size()
	// A material is clipped to avoid drawing outside its bounds inside the atlas. However,
	// imprecision in the clipping may cause a single pixel overflow. Be safe.
	size = size.Add(image.Pt(1, 1))
	place, fits := g.materials.packer.tryAdd(size)
	if !fits {
		return image.Point{}, errors.New("compute: no space left in image atlas")
	}
	// Position quad to match place.
	offset := place.Pos.Sub(bounds.Min)
	offsetf := layout.FPt(offset)
	q0 = q0.Add(offsetf)
	q1 = q1.Add(offsetf)
	q2 = q2.Add(offsetf)
	q3 = q3.Add(offsetf)
	uvPos, ok := g.images.positions[img.handle]
	if !ok {
		panic("compute: internal error: image not placed")
	}
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
	// Draw quad as two triangles.
	g.materials.quads = append(g.materials.quads, quad[0], quad[1], quad[3], quad[3], quad[1], quad[2])
	return offset, nil
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

func (g *compute) encodeOps(trans f32.Affine2D, viewport image.Point, ops []imageOp) error {
	for _, op := range ops {
		bounds := layout.FRect(op.clip)
		// clip is the union of all drawing affected by the clipping
		// operation. TODO: tighten.
		clip := f32.Rect(0, 0, float32(viewport.X), float32(viewport.Y))
		nclips := g.encodeClipStack(clip, bounds, op.path)
		m := op.material
		switch m.material {
		case materialTexture:
			t := trans.Mul(m.trans)
			off, err := g.addMaterialQuad(t, m.data)
			if err != nil {
				return err
			}

			g.enc.fillImage(0, off)
		case materialColor:
			g.enc.fill(f32color.NRGBAToRGBA(op.material.color.SRGB()))
		case materialLinearGradient:
			// TODO: implement.
			g.enc.fill(f32color.NRGBAToRGBA(op.material.color1.SRGB()))
		default:
			panic("not implemented")
		}
		// Pop the clip stack.
		for i := 0; i < nclips; i++ {
			g.enc.endClip(clip)
		}
	}
	return nil
}

// encodeClips encodes a stack of clip paths and return the stack depth.
func (g *compute) encodeClipStack(clip, bounds f32.Rectangle, p *pathOp) int {
	nclips := 0
	if p != nil && p.parent != nil {
		nclips += g.encodeClipStack(clip, bounds, p.parent)
		g.enc.beginClip(clip)
		nclips += 1
	}
	if p != nil && p.path {
		pathData, _ := g.drawOps.pathCache.get(p.pathKey)
		g.enc.transform(f32.Affine2D{}.Offset(p.off))
		g.enc.append(pathData.computePath)
		g.enc.transform(f32.Affine2D{}.Offset(p.off.Mul(-1)))
	} else {
		g.enc.rect(bounds, false)
	}
	return nclips
}

// encodePath takes a Path encoded with quadSplitter and encode it for elements.comp.
// This is certainly wasteful, but minimizes implementation differences to the old
// renderer.
func encodePath(p []byte) encoder {
	var enc encoder
	var (
		prevTo  f32.Point
		hasPrev bool
	)
	for len(p) > 0 {
		// p contains quadratic curves encoded in vertex structs.
		vertex := p[:vertStride]
		// We only need some of the values. This code undoes vertex.encode.
		from := f32.Pt(
			math.Float32frombits(bo.Uint32(vertex[8:])),
			math.Float32frombits(bo.Uint32(vertex[12:])),
		)
		ctrl := f32.Pt(
			math.Float32frombits(bo.Uint32(vertex[16:])),
			math.Float32frombits(bo.Uint32(vertex[20:])),
		)
		to := f32.Pt(
			math.Float32frombits(bo.Uint32(vertex[24:])),
			math.Float32frombits(bo.Uint32(vertex[28:])),
		)
		if hasPrev && from != prevTo {
			enc.scene[len(enc.scene)-1][0] = (flagEndPath << 16) | enc.scene[len(enc.scene)-1][0]
		}
		hasPrev = true
		prevTo = to
		enc.quad(from, ctrl, to, false)

		// The vertex is duplicated 4 times, one for each corner of quads drawn
		// by the old renderer.
		p = p[vertStride*4:]
	}
	if hasPrev {
		enc.scene[len(enc.scene)-1][0] = (flagEndPath << 16) | enc.scene[len(enc.scene)-1][0]
	}
	return enc
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

	// Pad scene with zeroes to avoid reading garbage in elements.comp.
	scenePadding := partitionSize - len(g.enc.scene)%partitionSize
	g.enc.scene = append(g.enc.scene, make([]sceneElem, scenePadding)...)

	realloced := false
	if s := len(g.enc.scene) * sceneElemSize; s > g.buffers.scene.size {
		realloced = true
		paddedCap := s * 11 / 10
		if err := g.buffers.scene.ensureCapacity(g.ctx, paddedCap); err != nil {
			return err
		}
	}
	g.buffers.scene.buffer.Upload(gunsafe.BytesView(g.enc.scene))

	w, h := tileDims.X*tileWidthPx, tileDims.Y*tileHeightPx
	if g.output.size.X < w || g.output.size.Y < h {
		if err := g.resizeOutput(image.Pt(w, h)); err != nil {
			return err
		}
	}
	g.ctx.BindImageTexture(kernel4OutputUnit, g.output.image, backend.AccessWrite, backend.TextureFormatRGBA8)
	if t := g.materials.tex; t != nil {
		g.ctx.BindImageTexture(kernel4AtlasUnit, t, backend.AccessRead, backend.TextureFormatRGBA8)
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
		n_elements:      uint32(g.enc.npath),
		n_pathseg:       uint32(g.enc.npathseg),
		width_in_tiles:  uint32(tileDims.X),
		height_in_tiles: uint32(tileDims.Y),
		tile_alloc:      malloc(g.enc.npath * pathSize),
		bin_alloc:       malloc(round(g.enc.npath, wgSize) * binSize),
		ptcl_alloc:      malloc(tileDims.X * tileDims.Y * ptclInitialAlloc),
		pathseg_alloc:   malloc(g.enc.npathseg * pathsegSize),
		anno_alloc:      malloc(g.enc.npath * annoSize),
	}

	numPartitions := (g.enc.numElements() + 127) / 128
	// clearSize is the atomic partition counter plus flag and 2 states per partition.
	clearSize := 4 + numPartitions*stateStride
	if clearSize > g.buffers.state.size {
		realloced = true
		paddedCap := clearSize * 11 / 10
		if err := g.buffers.state.ensureCapacity(g.ctx, paddedCap); err != nil {
			return err
		}
	}

	g.buffers.config.Upload(gunsafe.StructView(g.conf))

	minSize := int(unsafe.Sizeof(memoryHeader{})) + int(alloc)
	if minSize > g.buffers.memory.size {
		realloced = true
		// Add space for dynamic GPU allocations.
		const sizeBump = 4 * 1024 * 1024
		minSize += sizeBump
		if err := g.buffers.memory.ensureCapacity(g.ctx, minSize); err != nil {
			return err
		}
	}
	for {
		*g.memHeader = memoryHeader{
			mem_offset: alloc,
		}
		g.buffers.memory.buffer.Upload(gunsafe.StructView(g.memHeader))
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
		g.ctx.DispatchCompute((g.enc.npath+wgSize-1)/wgSize, 1, 1)
		g.ctx.MemoryBarrier()
		t.tileAlloc.end()
		t.pathCoarse.begin()
		g.ctx.BindProgram(g.programs.pathCoarse)
		g.ctx.DispatchCompute((g.enc.npathseg+31)/32, 1, 1)
		g.ctx.MemoryBarrier()
		t.pathCoarse.end()
		t.backdropBinning.begin()
		g.ctx.BindProgram(g.programs.backdrop)
		g.ctx.DispatchCompute((g.enc.npath+wgSize-1)/wgSize, 1, 1)
		// No barrier needed between backdrop and binning.
		g.ctx.BindProgram(g.programs.binning)
		g.ctx.DispatchCompute((g.enc.npath+wgSize-1)/wgSize, 1, 1)
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

		if err := g.buffers.memory.buffer.Download(gunsafe.StructView(g.memHeader)); err != nil {
			return err
		}
		switch errCode := g.memHeader.mem_error; errCode {
		case memNoError:
			return nil
		case memMallocFailed:
			// Resize memory and try again.
			realloced = true
			sz := g.buffers.memory.size * 15 / 10
			if err := g.buffers.memory.ensureCapacity(g.ctx, sz); err != nil {
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
	img, err := g.ctx.NewTexture(backend.TextureFormatRGBA8, size.X, size.Y,
		backend.FilterNearest,
		backend.FilterNearest,
		backend.BufferBindingShaderStorage|backend.BufferBindingTexture)
	if err != nil {
		return err
	}
	g.output.image = img
	g.output.size = size
	return nil
}

func (g *compute) Release() {
	if g.drawOps.pathCache != nil {
		g.drawOps.pathCache.release()
	}
	if g.cache != nil {
		g.cache.release()
	}
	progs := []backend.Program{
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
	if g.materials.buffer != nil {
		g.materials.buffer.Release()
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

func (b *sizedBuffer) ensureCapacity(ctx backend.Device, size int) error {
	if b.size >= size {
		return nil
	}
	if b.buffer != nil {
		b.release()
	}
	buf, err := ctx.NewBuffer(backend.BufferBindingShaderStorage, size)
	if err != nil {
		return err
	}
	b.buffer = buf
	b.size = size
	return nil
}

func bindStorageBuffers(prog backend.Program, buffers ...backend.Buffer) {
	for i, buf := range buffers {
		prog.SetStorageBuffer(i, buf)
	}
}

var bo = binary.LittleEndian

func (e *encoder) reset() {
	e.scene = e.scene[:0]
	e.npath = 0
	e.npathseg = 0
}

func (e *encoder) numElements() int {
	return len(e.scene)
}

func (e *encoder) append(e2 encoder) {
	e.scene = append(e.scene, e2.scene...)
	e.npath += e2.npath
	e.npathseg += e2.npathseg
}

func (e *encoder) transform(m f32.Affine2D) {
	sx, hx, ox, hy, sy, oy := m.Elems()
	e.scene = append(e.scene, sceneElem{
		0: elemTransform,
		1: math.Float32bits(sx),
		2: math.Float32bits(hy),
		3: math.Float32bits(hx),
		4: math.Float32bits(sy),
		5: math.Float32bits(ox),
		6: math.Float32bits(oy),
	})
}

func (e *encoder) lineWidth(width float32) {
	e.scene = append(e.scene, sceneElem{
		0: elemLineWidth,
		1: math.Float32bits(width),
	})
}

func (e *encoder) stroke(col color.RGBA) {
	e.scene = append(e.scene, sceneElem{
		0: elemStroke,
		1: uint32(col.R)<<24 | uint32(col.G)<<16 | uint32(col.B)<<8 | uint32(col.A),
	})
	e.npath++
}

func (e *encoder) beginClip(bbox f32.Rectangle) {
	e.scene = append(e.scene, sceneElem{
		0: elemBeginClip,
		1: math.Float32bits(bbox.Min.X),
		2: math.Float32bits(bbox.Min.Y),
		3: math.Float32bits(bbox.Max.X),
		4: math.Float32bits(bbox.Max.Y),
	})
	e.npath++
}

func (e *encoder) endClip(bbox f32.Rectangle) {
	e.scene = append(e.scene, sceneElem{
		0: elemEndClip,
		1: math.Float32bits(bbox.Min.X),
		2: math.Float32bits(bbox.Min.Y),
		3: math.Float32bits(bbox.Max.X),
		4: math.Float32bits(bbox.Max.Y),
	})
	e.npath++
}

func (e *encoder) rect(r f32.Rectangle, stroke bool) {
	// Rectangle corners, clock-wise.
	c0, c1, c2, c3 := r.Min, f32.Pt(r.Min.X, r.Max.Y), r.Max, f32.Pt(r.Max.X, r.Min.Y)
	e.line(c0, c1, stroke, 0)
	e.line(c1, c2, stroke, 0)
	e.line(c2, c3, stroke, 0)
	e.line(c3, c0, stroke, flagEndPath)
}

func (e *encoder) fill(col color.RGBA) {
	e.scene = append(e.scene, sceneElem{
		0: elemFill,
		1: uint32(col.R)<<24 | uint32(col.G)<<16 | uint32(col.B)<<8 | uint32(col.A),
	})
	e.npath++
}

func (e *encoder) fillImage(index int, offset image.Point) {
	x := int16(offset.X)
	y := int16(offset.Y)
	e.scene = append(e.scene, sceneElem{
		0: elemFillImage,
		1: uint32(index),
		2: uint32(uint16(x)) | uint32(uint16(y))<<16,
	})
	e.npath++
}

func (e *encoder) line(start, end f32.Point, stroke bool, flags uint32) {
	tag := uint32(elemFillLine)
	if stroke {
		tag = elemStrokeLine
	}
	e.scene = append(e.scene, sceneElem{
		0: flags<<16 | tag,
		1: math.Float32bits(start.X),
		2: math.Float32bits(start.Y),
		3: math.Float32bits(end.X),
		4: math.Float32bits(end.Y),
	})
	e.npathseg++
}

func (e *encoder) quad(start, ctrl, end f32.Point, stroke bool) {
	tag := uint32(elemFillQuad)
	if stroke {
		tag = elemStrokeQuad
	}
	e.scene = append(e.scene, sceneElem{
		0: tag,
		1: math.Float32bits(start.X),
		2: math.Float32bits(start.Y),
		3: math.Float32bits(ctrl.X),
		4: math.Float32bits(ctrl.Y),
		5: math.Float32bits(end.X),
		6: math.Float32bits(end.Y),
	})
	e.npathseg++
}
