// SPDX-License-Identifier: Unlicense OR MIT

package gpu

// GPU accelerated path drawing using the algorithms from
// Pathfinder (https://github.com/servo/pathfinder).

import (
	"image"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/internal/path"
	gunsafe "gioui.org/internal/unsafe"
)

type pather struct {
	ctx Backend

	viewport image.Point

	stenciler *stenciler
	coverer   *coverer
}

type coverer struct {
	ctx         Backend
	prog        [2]*program
	texUniforms struct {
		vert struct {
			coverUniforms
			_ [8]byte // Padding to multiple of 16.
		}
	}
	colUniforms struct {
		vert struct {
			coverUniforms
			_ [8]byte // Padding to multiple of 16.
		}
		frag struct {
			colorUniforms
		}
	}
	layout InputLayout
}

type coverUniforms struct {
	z             float32
	_             float32 // Padding.
	scale         [2]float32
	offset        [2]float32
	uvCoverScale  [2]float32
	uvCoverOffset [2]float32
	uvScale       [2]float32
	uvOffset      [2]float32
}

type stenciler struct {
	ctx    Backend
	defFBO Framebuffer
	prog   struct {
		prog     *program
		uniforms struct {
			vert struct {
				scale      [2]float32
				offset     [2]float32
				pathOffset [2]float32
				_          [8]byte // Padding to multiple of 16.
			}
		}
		layout InputLayout
	}
	iprog struct {
		prog     *program
		uniforms struct {
			vert struct {
				uvScale  [2]float32
				uvOffset [2]float32
			}
		}
		layout InputLayout
	}
	fbos          fboSet
	intersections fboSet
	indexBuf      Buffer
}

type fboSet struct {
	fbos []stencilFBO
}

type stencilFBO struct {
	size image.Point
	fbo  Framebuffer
	tex  Texture
}

type pathData struct {
	ncurves int
	data    Buffer
}

const (
	// Number of path quads per draw batch.
	pathBatchSize = 10000
)

const (
	attribPathCorner = 0
	attribPathMaxY   = 1
	attribPathFrom   = 2
	attribPathCtrl   = 3
	attribPathTo     = 4
)

func newPather(ctx Backend) *pather {
	return &pather{
		ctx:       ctx,
		stenciler: newStenciler(ctx),
		coverer:   newCoverer(ctx),
	}
}

func newCoverer(ctx Backend) *coverer {
	c := &coverer{
		ctx: ctx,
	}
	prog, layout, err := createColorPrograms(ctx, shader_cover_vert, shader_cover_frag,
		[2]interface{}{&c.colUniforms.vert, &c.texUniforms.vert},
		[2]interface{}{&c.colUniforms.frag, nil},
	)
	if err != nil {
		panic(err)
	}
	c.prog = prog
	c.layout = layout
	return c
}

func newStenciler(ctx Backend) *stenciler {
	defFBO := ctx.DefaultFramebuffer()
	// Allocate a suitably large index buffer for drawing paths.
	indices := make([]uint16, pathBatchSize*6)
	for i := 0; i < pathBatchSize; i++ {
		i := uint16(i)
		indices[i*6+0] = i*4 + 0
		indices[i*6+1] = i*4 + 1
		indices[i*6+2] = i*4 + 2
		indices[i*6+3] = i*4 + 2
		indices[i*6+4] = i*4 + 1
		indices[i*6+5] = i*4 + 3
	}
	indexBuf, err := ctx.NewImmutableBuffer(BufferBindingIndices, gunsafe.BytesView(indices))
	if err != nil {
		panic(err)
	}
	progLayout, err := ctx.NewInputLayout(shader_stencil_vert, []InputDesc{
		{Type: DataTypeShort, Size: 2, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).CornerX))},
		{Type: DataTypeFloat, Size: 1, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).MaxY))},
		{Type: DataTypeFloat, Size: 2, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).FromX))},
		{Type: DataTypeFloat, Size: 2, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).CtrlX))},
		{Type: DataTypeFloat, Size: 2, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).ToX))},
	})
	if err != nil {
		panic(err)
	}
	iprogLayout, err := ctx.NewInputLayout(shader_intersect_vert, []InputDesc{
		{Type: DataTypeFloat, Size: 2, Offset: 0},
		{Type: DataTypeFloat, Size: 2, Offset: 4 * 2},
	})
	if err != nil {
		panic(err)
	}
	st := &stenciler{
		ctx:      ctx,
		defFBO:   defFBO,
		indexBuf: indexBuf,
	}
	prog, err := ctx.NewProgram(shader_stencil_vert, shader_stencil_frag)
	if err != nil {
		panic(err)
	}
	vertUniforms := newUniformBuffer(ctx, &st.prog.uniforms.vert)
	st.prog.prog = newProgram(prog, vertUniforms, nil)
	st.prog.layout = progLayout
	iprog, err := ctx.NewProgram(shader_intersect_vert, shader_intersect_frag)
	if err != nil {
		panic(err)
	}
	vertUniforms = newUniformBuffer(ctx, &st.iprog.uniforms.vert)
	st.iprog.prog = newProgram(iprog, vertUniforms, nil)
	st.iprog.layout = iprogLayout
	return st
}

func (s *fboSet) resize(ctx Backend, sizes []image.Point) {
	// Add fbos.
	for i := len(s.fbos); i < len(sizes); i++ {
		s.fbos = append(s.fbos, stencilFBO{})
	}
	// Resize fbos.
	for i, sz := range sizes {
		f := &s.fbos[i]
		// Resizing or recreating FBOs can introduce rendering stalls.
		// Avoid if the space waste is not too high.
		resize := sz.X > f.size.X || sz.Y > f.size.Y
		waste := float32(sz.X*sz.Y) / float32(f.size.X*f.size.Y)
		resize = resize || waste > 1.2
		if resize {
			if f.fbo != nil {
				f.fbo.Release()
				f.tex.Release()
			}
			tex, err := ctx.NewTexture(TextureFormatFloat, sz.X, sz.Y, FilterNearest, FilterNearest,
				BufferBindingTexture|BufferBindingFramebuffer)
			fbo, err := ctx.NewFramebuffer(tex)
			if err != nil {
				panic(err)
			}
			f.size = sz
			f.tex = tex
			f.fbo = fbo
		}
	}
	// Delete extra fbos.
	s.delete(ctx, len(sizes))
}

func (s *fboSet) invalidate(ctx Backend) {
	for _, f := range s.fbos {
		f.fbo.Invalidate()
	}
}

func (s *fboSet) delete(ctx Backend, idx int) {
	for i := idx; i < len(s.fbos); i++ {
		f := s.fbos[i]
		f.fbo.Release()
		f.tex.Release()
	}
	s.fbos = s.fbos[:idx]
}

func (s *stenciler) release() {
	s.fbos.delete(s.ctx, 0)
	s.prog.layout.Release()
	s.prog.prog.Release()
	s.iprog.layout.Release()
	s.iprog.prog.Release()
	s.indexBuf.Release()
}

func (p *pather) release() {
	p.stenciler.release()
	p.coverer.release()
}

func (c *coverer) release() {
	for _, p := range c.prog {
		p.Release()
	}
	c.layout.Release()
}

func buildPath(ctx Backend, p []byte) *pathData {
	buf, err := ctx.NewImmutableBuffer(BufferBindingVertices, p)
	if err != nil {
		panic(err)
	}
	return &pathData{
		ncurves: len(p) / path.VertStride,
		data:    buf,
	}
}

func (p *pathData) release() {
	p.data.Release()
}

func (p *pather) begin(sizes []image.Point) {
	p.stenciler.begin(sizes)
}

func (p *pather) end() {
	p.stenciler.end()
}

func (p *pather) stencilPath(bounds image.Rectangle, offset f32.Point, uv image.Point, data *pathData) {
	p.stenciler.stencilPath(bounds, offset, uv, data)
}

func (s *stenciler) beginIntersect(sizes []image.Point) {
	s.ctx.BlendFunc(BlendFactorDstColor, BlendFactorZero)
	// 8 bit coverage is enough, but OpenGL ES only supports single channel
	// floating point formats. Replace with GL_RGB+GL_UNSIGNED_BYTE if
	// no floating point support is available.
	s.intersections.resize(s.ctx, sizes)
	s.ctx.ClearColor(1.0, 0.0, 0.0, 0.0)
	s.iprog.prog.prog.Bind()
}

func (s *stenciler) endIntersect() {
	s.defFBO.Bind()
}

func (s *stenciler) invalidateFBO() {
	s.intersections.invalidate(s.ctx)
	s.fbos.invalidate(s.ctx)
	s.defFBO.Bind()
}

func (s *stenciler) cover(idx int) stencilFBO {
	return s.fbos.fbos[idx]
}

func (s *stenciler) begin(sizes []image.Point) {
	s.ctx.BlendFunc(BlendFactorOne, BlendFactorOne)
	s.fbos.resize(s.ctx, sizes)
	s.ctx.ClearColor(0.0, 0.0, 0.0, 0.0)
	s.prog.prog.prog.Bind()
	s.prog.layout.Bind()
	s.indexBuf.BindIndex()
}

func (s *stenciler) stencilPath(bounds image.Rectangle, offset f32.Point, uv image.Point, data *pathData) {
	s.ctx.Viewport(uv.X, uv.Y, bounds.Dx(), bounds.Dy())
	// Transform UI coordinates to OpenGL coordinates.
	texSize := f32.Point{X: float32(bounds.Dx()), Y: float32(bounds.Dy())}
	scale := f32.Point{X: 2 / texSize.X, Y: 2 / texSize.Y}
	orig := f32.Point{X: -1 - float32(bounds.Min.X)*2/texSize.X, Y: -1 - float32(bounds.Min.Y)*2/texSize.Y}
	s.prog.uniforms.vert.scale = [2]float32{scale.X, scale.Y}
	s.prog.uniforms.vert.offset = [2]float32{orig.X, orig.Y}
	s.prog.uniforms.vert.pathOffset = [2]float32{offset.X, offset.Y}
	s.prog.prog.UploadUniforms()
	// Draw in batches that fit in uint16 indices.
	start := 0
	nquads := data.ncurves / 4
	for start < nquads {
		batch := nquads - start
		if max := pathBatchSize; batch > max {
			batch = max
		}
		off := path.VertStride * start * 4
		data.data.BindVertex(path.VertStride, off)
		s.ctx.DrawElements(DrawModeTriangles, 0, batch*6)
		start += batch
	}
}

func (s *stenciler) end() {
	s.defFBO.Bind()
}

func (p *pather) cover(z float32, mat materialType, col [4]float32, scale, off, uvScale, uvOff, coverScale, coverOff f32.Point) {
	p.coverer.cover(z, mat, col, scale, off, uvScale, uvOff, coverScale, coverOff)
}

func (c *coverer) cover(z float32, mat materialType, col [4]float32, scale, off, uvScale, uvOff, coverScale, coverOff f32.Point) {
	p := c.prog[mat]
	p.prog.Bind()
	var uniforms *coverUniforms
	switch mat {
	case materialColor:
		c.colUniforms.frag.color = col
		uniforms = &c.colUniforms.vert.coverUniforms
	case materialTexture:
		c.texUniforms.vert.uvScale = [2]float32{uvScale.X, uvScale.Y}
		c.texUniforms.vert.uvOffset = [2]float32{uvOff.X, uvOff.Y}
		uniforms = &c.texUniforms.vert.coverUniforms
	}
	uniforms.z = z
	uniforms.scale = [2]float32{scale.X, scale.Y}
	uniforms.offset = [2]float32{off.X, off.Y}
	uniforms.uvCoverScale = [2]float32{coverScale.X, coverScale.Y}
	uniforms.uvCoverOffset = [2]float32{coverOff.X, coverOff.Y}
	p.UploadUniforms()
	c.ctx.DrawArrays(DrawModeTriangleStrip, 0, 4)
}
