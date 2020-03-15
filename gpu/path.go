// SPDX-License-Identifier: Unlicense OR MIT

package gpu

// GPU accelerated path drawing using the algorithms from
// Pathfinder (https://github.com/servo/pathfinder).

import (
	"image"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/gpu/backend"
	"gioui.org/internal/f32color"
	"gioui.org/internal/path"
	gunsafe "gioui.org/internal/unsafe"
)

type pather struct {
	ctx backend.Device

	viewport image.Point

	stenciler *stenciler
	coverer   *coverer
}

type coverer struct {
	ctx         backend.Device
	prog        [2]*program
	texUniforms *coverTexUniforms
	colUniforms *coverColUniforms
	layout      backend.InputLayout
}

type coverTexUniforms struct {
	vert struct {
		coverUniforms
		_ [12]byte // Padding to multiple of 16.
	}
}

type coverColUniforms struct {
	vert struct {
		coverUniforms
		_ [12]byte // Padding to multiple of 16.
	}
	frag struct {
		colorUniforms
	}
}

type coverUniforms struct {
	transform        [4]float32
	uvCoverTransform [4]float32
	uvTransform      [4]float32
	z                float32
}

type stenciler struct {
	ctx  backend.Device
	prog struct {
		prog     *program
		uniforms *stencilUniforms
		layout   backend.InputLayout
	}
	iprog struct {
		prog     *program
		uniforms *intersectUniforms
		layout   backend.InputLayout
	}
	fbos          fboSet
	intersections fboSet
	indexBuf      backend.Buffer
}

type stencilUniforms struct {
	vert struct {
		transform  [4]float32
		pathOffset [2]float32
		_          [8]byte // Padding to multiple of 16.
	}
}

type intersectUniforms struct {
	vert struct {
		uvTransform    [4]float32
		subUVTransform [4]float32
	}
}

type fboSet struct {
	fbos []stencilFBO
}

type stencilFBO struct {
	size image.Point
	fbo  backend.Framebuffer
	tex  backend.Texture
}

type pathData struct {
	ncurves int
	data    backend.Buffer
}

const (
	// Number of path quads per draw batch.
	pathBatchSize = 10000
)

func newPather(ctx backend.Device) *pather {
	return &pather{
		ctx:       ctx,
		stenciler: newStenciler(ctx),
		coverer:   newCoverer(ctx),
	}
}

func newCoverer(ctx backend.Device) *coverer {
	c := &coverer{
		ctx: ctx,
	}
	c.colUniforms = new(coverColUniforms)
	c.texUniforms = new(coverTexUniforms)
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

func newStenciler(ctx backend.Device) *stenciler {
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
	indexBuf, err := ctx.NewImmutableBuffer(backend.BufferBindingIndices, gunsafe.BytesView(indices))
	if err != nil {
		panic(err)
	}
	progLayout, err := ctx.NewInputLayout(shader_stencil_vert, []backend.InputDesc{
		{Type: backend.DataTypeFloat, Size: 1, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).Corner))},
		{Type: backend.DataTypeFloat, Size: 1, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).MaxY))},
		{Type: backend.DataTypeFloat, Size: 2, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).FromX))},
		{Type: backend.DataTypeFloat, Size: 2, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).CtrlX))},
		{Type: backend.DataTypeFloat, Size: 2, Offset: int(unsafe.Offsetof((*(*path.Vertex)(nil)).ToX))},
	})
	if err != nil {
		panic(err)
	}
	iprogLayout, err := ctx.NewInputLayout(shader_intersect_vert, []backend.InputDesc{
		{Type: backend.DataTypeFloat, Size: 2, Offset: 0},
		{Type: backend.DataTypeFloat, Size: 2, Offset: 4 * 2},
	})
	if err != nil {
		panic(err)
	}
	st := &stenciler{
		ctx:      ctx,
		indexBuf: indexBuf,
	}
	prog, err := ctx.NewProgram(shader_stencil_vert, shader_stencil_frag)
	if err != nil {
		panic(err)
	}
	st.prog.uniforms = new(stencilUniforms)
	vertUniforms := newUniformBuffer(ctx, &st.prog.uniforms.vert)
	st.prog.prog = newProgram(prog, vertUniforms, nil)
	st.prog.layout = progLayout
	iprog, err := ctx.NewProgram(shader_intersect_vert, shader_intersect_frag)
	if err != nil {
		panic(err)
	}
	st.iprog.uniforms = new(intersectUniforms)
	vertUniforms = newUniformBuffer(ctx, &st.iprog.uniforms.vert)
	st.iprog.prog = newProgram(iprog, vertUniforms, nil)
	st.iprog.layout = iprogLayout
	return st
}

func (s *fboSet) resize(ctx backend.Device, sizes []image.Point) {
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
			tex, err := ctx.NewTexture(backend.TextureFormatFloat, sz.X, sz.Y, backend.FilterNearest, backend.FilterNearest,
				backend.BufferBindingTexture|backend.BufferBindingFramebuffer)
			if err != nil {
				panic(err)
			}
			fbo, err := ctx.NewFramebuffer(tex, 0)
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

func (s *fboSet) invalidate(ctx backend.Device) {
	for _, f := range s.fbos {
		f.fbo.Invalidate()
	}
}

func (s *fboSet) delete(ctx backend.Device, idx int) {
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

func buildPath(ctx backend.Device, p []byte) *pathData {
	buf, err := ctx.NewImmutableBuffer(backend.BufferBindingVertices, p)
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

func (p *pather) stencilPath(bounds image.Rectangle, offset f32.Point, uv image.Point, data *pathData) {
	p.stenciler.stencilPath(bounds, offset, uv, data)
}

func (s *stenciler) beginIntersect(sizes []image.Point) {
	s.ctx.BlendFunc(backend.BlendFactorDstColor, backend.BlendFactorZero)
	// 8 bit coverage is enough, but OpenGL ES only supports single channel
	// floating point formats. Replace with GL_RGB+GL_UNSIGNED_BYTE if
	// no floating point support is available.
	s.intersections.resize(s.ctx, sizes)
	s.ctx.BindProgram(s.iprog.prog.prog)
}

func (s *stenciler) invalidateFBO() {
	s.intersections.invalidate(s.ctx)
	s.fbos.invalidate(s.ctx)
}

func (s *stenciler) cover(idx int) stencilFBO {
	return s.fbos.fbos[idx]
}

func (s *stenciler) begin(sizes []image.Point) {
	s.ctx.BlendFunc(backend.BlendFactorOne, backend.BlendFactorOne)
	s.fbos.resize(s.ctx, sizes)
	s.ctx.BindProgram(s.prog.prog.prog)
	s.ctx.BindInputLayout(s.prog.layout)
	s.ctx.BindIndexBuffer(s.indexBuf)
}

func (s *stenciler) stencilPath(bounds image.Rectangle, offset f32.Point, uv image.Point, data *pathData) {
	s.ctx.Viewport(uv.X, uv.Y, bounds.Dx(), bounds.Dy())
	// Transform UI coordinates to OpenGL coordinates.
	texSize := f32.Point{X: float32(bounds.Dx()), Y: float32(bounds.Dy())}
	scale := f32.Point{X: 2 / texSize.X, Y: 2 / texSize.Y}
	orig := f32.Point{X: -1 - float32(bounds.Min.X)*2/texSize.X, Y: -1 - float32(bounds.Min.Y)*2/texSize.Y}
	s.prog.uniforms.vert.transform = [4]float32{scale.X, scale.Y, orig.X, orig.Y}
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
		s.ctx.BindVertexBuffer(data.data, path.VertStride, off)
		s.ctx.DrawElements(backend.DrawModeTriangles, 0, batch*6)
		start += batch
	}
}

func (p *pather) cover(z float32, mat materialType, col f32color.RGBA, scale, off, uvScale, uvOff, coverScale, coverOff f32.Point) {
	p.coverer.cover(z, mat, col, scale, off, uvScale, uvOff, coverScale, coverOff)
}

func (c *coverer) cover(z float32, mat materialType, col f32color.RGBA, scale, off, uvScale, uvOff, coverScale, coverOff f32.Point) {
	p := c.prog[mat]
	c.ctx.BindProgram(p.prog)
	var uniforms *coverUniforms
	switch mat {
	case materialColor:
		c.colUniforms.frag.color = col
		uniforms = &c.colUniforms.vert.coverUniforms
	case materialTexture:
		c.texUniforms.vert.uvTransform = [4]float32{uvScale.X, uvScale.Y, uvOff.X, uvOff.Y}
		uniforms = &c.texUniforms.vert.coverUniforms
	}
	uniforms.z = z
	uniforms.transform = [4]float32{scale.X, scale.Y, off.X, off.Y}
	uniforms.uvCoverTransform = [4]float32{coverScale.X, coverScale.Y, coverOff.X, coverOff.Y}
	p.UploadUniforms()
	c.ctx.DrawArrays(backend.DrawModeTriangleStrip, 0, 4)
}
