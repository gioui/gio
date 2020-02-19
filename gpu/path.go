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
	ctx    Backend
	prog   [2]Program
	layout InputLayout
	vars   [2]struct {
		z                             Uniform
		uScale, uOffset               Uniform
		uUVScale, uUVOffset           Uniform
		uCoverUVScale, uCoverUVOffset Uniform
		uColor                        Uniform
	}
}

type stenciler struct {
	ctx                Backend
	defFBO             Framebuffer
	prog               Program
	progLayout         InputLayout
	iprog              Program
	iprogLayout        InputLayout
	fbos               fboSet
	intersections      fboSet
	uScale, uOffset    Uniform
	uPathOffset        Uniform
	uIntersectUVOffset Uniform
	uIntersectUVScale  Uniform
	indexBuf           Buffer
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
	prog, layout, err := createColorPrograms(ctx, shader_cover_vert, shader_cover_frag)
	if err != nil {
		panic(err)
	}
	c := &coverer{
		ctx:    ctx,
		prog:   prog,
		layout: layout,
	}
	for i, prog := range prog {
		switch materialType(i) {
		case materialTexture:
			uTex := prog.UniformFor("tex")
			prog.Uniform1i(uTex, 0)
			c.vars[i].uUVScale = prog.UniformFor("uniforms.uvScale")
			c.vars[i].uUVOffset = prog.UniformFor("uniforms.uvOffset")
		case materialColor:
			c.vars[i].uColor = prog.UniformFor("color.color")
		}
		uCover := prog.UniformFor("cover")
		prog.Uniform1i(uCover, 1)
		c.vars[i].z = prog.UniformFor("uniforms.z")
		c.vars[i].uScale = prog.UniformFor("uniforms.scale")
		c.vars[i].uOffset = prog.UniformFor("uniforms.offset")
		c.vars[i].uCoverUVScale = prog.UniformFor("uniforms.uvCoverScale")
		c.vars[i].uCoverUVOffset = prog.UniformFor("uniforms.uvCoverOffset")
	}
	return c
}

func newStenciler(ctx Backend) *stenciler {
	defFBO := ctx.DefaultFramebuffer()
	prog, err := ctx.NewProgram(shader_stencil_vert, shader_stencil_frag)
	if err != nil {
		panic(err)
	}
	iprog, err := ctx.NewProgram(shader_intersect_vert, shader_intersect_frag)
	if err != nil {
		panic(err)
	}
	coverLoc := iprog.UniformFor("cover")
	iprog.Uniform1i(coverLoc, 0)
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
	indexBuf := ctx.NewBuffer(BufferTypeIndices, gunsafe.BytesView(indices))
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
	return &stenciler{
		ctx:                ctx,
		defFBO:             defFBO,
		prog:               prog,
		progLayout:         progLayout,
		iprog:              iprog,
		iprogLayout:        iprogLayout,
		uScale:             prog.UniformFor("uniforms.scale"),
		uOffset:            prog.UniformFor("uniforms.offset"),
		uPathOffset:        prog.UniformFor("uniforms.pathOffset"),
		uIntersectUVScale:  iprog.UniformFor("uvparams.scale"),
		uIntersectUVOffset: iprog.UniformFor("uvparams.offset"),
		indexBuf:           indexBuf,
	}
}

func (s *fboSet) resize(ctx Backend, sizes []image.Point) {
	// Add fbos.
	for i := len(s.fbos); i < len(sizes); i++ {
		s.fbos = append(s.fbos, stencilFBO{
			fbo: ctx.NewFramebuffer(),
			tex: ctx.NewTexture(FilterNearest, FilterNearest),
		})
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
			f.size = sz
			f.tex.Resize(TextureFormatFloat, sz.X, sz.Y)
			f.fbo.BindTexture(f.tex)
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
	s.progLayout.Release()
	s.prog.Release()
	s.iprogLayout.Release()
	s.iprog.Release()
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
	buf := ctx.NewBuffer(BufferTypeData, p)
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
	s.ctx.NilTexture().Bind(1)
	s.ctx.BlendFunc(BlendFactorDstColor, BlendFactorZero)
	// 8 bit coverage is enough, but OpenGL ES only supports single channel
	// floating point formats. Replace with GL_RGB+GL_UNSIGNED_BYTE if
	// no floating point support is available.
	s.intersections.resize(s.ctx, sizes)
	s.ctx.ClearColor(1.0, 0.0, 0.0, 0.0)
	s.iprog.Bind()
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
	s.ctx.NilTexture().Bind(1)
	s.ctx.BlendFunc(BlendFactorOne, BlendFactorOne)
	s.fbos.resize(s.ctx, sizes)
	s.ctx.ClearColor(0.0, 0.0, 0.0, 0.0)
	s.prog.Bind()
	s.progLayout.Bind()
	s.indexBuf.BindIndex()
}

func (s *stenciler) stencilPath(bounds image.Rectangle, offset f32.Point, uv image.Point, data *pathData) {
	s.ctx.Viewport(uv.X, uv.Y, bounds.Dx(), bounds.Dy())
	// Transform UI coordinates to OpenGL coordinates.
	texSize := f32.Point{X: float32(bounds.Dx()), Y: float32(bounds.Dy())}
	scale := f32.Point{X: 2 / texSize.X, Y: 2 / texSize.Y}
	orig := f32.Point{X: -1 - float32(bounds.Min.X)*2/texSize.X, Y: -1 - float32(bounds.Min.Y)*2/texSize.Y}
	s.prog.Uniform2f(s.uScale, scale.X, scale.Y)
	s.prog.Uniform2f(s.uOffset, orig.X, orig.Y)
	s.prog.Uniform2f(s.uPathOffset, offset.X, offset.Y)
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
	p.Bind()
	switch mat {
	case materialColor:
		p.Uniform4f(c.vars[mat].uColor, col[0], col[1], col[2], col[3])
	case materialTexture:
		p.Uniform2f(c.vars[mat].uUVScale, uvScale.X, uvScale.Y)
		p.Uniform2f(c.vars[mat].uUVOffset, uvOff.X, uvOff.Y)
	}
	p.Uniform1f(c.vars[mat].z, z)
	p.Uniform2f(c.vars[mat].uScale, scale.X, scale.Y)
	p.Uniform2f(c.vars[mat].uOffset, off.X, off.Y)
	p.Uniform2f(c.vars[mat].uCoverUVScale, coverScale.X, coverScale.Y)
	p.Uniform2f(c.vars[mat].uCoverUVOffset, coverOff.X, coverOff.Y)
	c.ctx.DrawArrays(DrawModeTriangleStrip, 0, 4)
}
