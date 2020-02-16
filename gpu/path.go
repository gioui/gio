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
	ctx  Backend
	prog [2]Program
	vars [2]struct {
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
	iprog              Program
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

var (
	pathAttribs      = []string{"corner", "maxy", "from", "ctrl", "to"}
	intersectAttribs = []string{"pos", "uv"}
)

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
	prog, err := createColorPrograms(ctx, coverVSrc, coverFSrc)
	if err != nil {
		panic(err)
	}
	c := &coverer{
		ctx:  ctx,
		prog: prog,
	}
	for i, prog := range prog {
		switch materialType(i) {
		case materialTexture:
			uTex := prog.UniformFor("tex")
			prog.Uniform1i(uTex, 0)
			c.vars[i].uUVScale = prog.UniformFor("uvScale")
			c.vars[i].uUVOffset = prog.UniformFor("uvOffset")
		case materialColor:
			c.vars[i].uColor = prog.UniformFor("color")
		}
		uCover := prog.UniformFor("cover")
		prog.Uniform1i(uCover, 1)
		c.vars[i].z = prog.UniformFor("z")
		c.vars[i].uScale = prog.UniformFor("scale")
		c.vars[i].uOffset = prog.UniformFor("offset")
		c.vars[i].uCoverUVScale = prog.UniformFor("uvCoverScale")
		c.vars[i].uCoverUVOffset = prog.UniformFor("uvCoverOffset")
	}
	return c
}

func newStenciler(ctx Backend) *stenciler {
	defFBO := ctx.DefaultFramebuffer()
	prog, err := ctx.NewProgram(stencilVSrc, stencilFSrc, pathAttribs)
	if err != nil {
		panic(err)
	}
	iprog, err := ctx.NewProgram(intersectVSrc, intersectFSrc, intersectAttribs)
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
	return &stenciler{
		ctx:                ctx,
		defFBO:             defFBO,
		prog:               prog,
		iprog:              iprog,
		uScale:             prog.UniformFor("scale"),
		uOffset:            prog.UniformFor("offset"),
		uPathOffset:        prog.UniformFor("pathOffset"),
		uIntersectUVScale:  iprog.UniformFor("uvScale"),
		uIntersectUVOffset: iprog.UniformFor("uvOffset"),
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
	s.prog.Release()
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
	s.indexBuf.Bind()
}

func (s *stenciler) stencilPath(bounds image.Rectangle, offset f32.Point, uv image.Point, data *pathData) {
	data.data.Bind()
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
		s.ctx.SetupVertexArray(attribPathCorner, 2, DataTypeShort, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).CornerX)))
		s.ctx.SetupVertexArray(attribPathMaxY, 1, DataTypeFloat, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).MaxY)))
		s.ctx.SetupVertexArray(attribPathFrom, 2, DataTypeFloat, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).FromX)))
		s.ctx.SetupVertexArray(attribPathCtrl, 2, DataTypeFloat, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).CtrlX)))
		s.ctx.SetupVertexArray(attribPathTo, 2, DataTypeFloat, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).ToX)))
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

const stencilVSrc = `
#version 100

precision highp float;

uniform vec2 scale;
uniform vec2 offset;
uniform vec2 pathOffset;

attribute vec2 corner;
attribute float maxy;
attribute vec2 from;
attribute vec2 ctrl;
attribute vec2 to;

varying vec2 vFrom;
varying vec2 vCtrl;
varying vec2 vTo;

void main() {
	// Add a one pixel overlap so curve quads cover their
	// entire curves. Could use conservative rasterization
	// if available.
	vec2 from = from + pathOffset;
	vec2 ctrl = ctrl + pathOffset;
	vec2 to = to + pathOffset;
	float maxy = maxy + pathOffset.y;
	vec2 pos;
	if (corner.x > 0.0) {
		// East.
		pos.x = max(max(from.x, ctrl.x), to.x)+1.0;
	} else {
		// West.
		pos.x = min(min(from.x, ctrl.x), to.x)-1.0;
	}
	if (corner.y > 0.0) {
		// North.
		pos.y = maxy + 1.0;
	} else {
		// South.
		pos.y = min(min(from.y, ctrl.y), to.y) - 1.0;
	}
	vFrom = from-pos;
	vCtrl = ctrl-pos;
	vTo = to-pos;
    pos *= scale;
    pos += offset;
    gl_Position = vec4(pos, 1, 1);
}
`

const stencilFSrc = `
#version 100

precision mediump float;

varying vec2 vFrom;
varying vec2 vCtrl;
varying vec2 vTo;

uniform sampler2D areaLUT;

void main() {
	float dx = vTo.x - vFrom.x;
	// Sort from and to in increasing order so the root below
	// is always the positive square root, if any.
	// We need the direction of the curve below, so this can't be
	// done from the vertex shader.
	bool increasing = vTo.x >= vFrom.x;
	vec2 left = increasing ? vFrom : vTo;
	vec2 right = increasing ? vTo : vFrom;

	// The signed horizontal extent of the fragment.
	vec2 extent = clamp(vec2(vFrom.x, vTo.x), -0.5, 0.5);
	// Find the t where the curve crosses the middle of the
	// extent, x₀.
	// Given the Bézier curve with x coordinates P₀, P₁, P₂
	// where P₀ is at the origin, its x coordinate in t
	// is given by:
	//
	// x(t) = 2(1-t)tP₁ + t²P₂
	// 
	// Rearranging:
	//
	// x(t) = (P₂ - 2P₁)t² + 2P₁t
	//
	// Setting x(t) = x₀ and using Muller's quadratic formula ("Citardauq")
	// for robustnesss,
	//
	// t = 2x₀/(2P₁±√(4P₁²+4(P₂-2P₁)x₀))
	//
	// which simplifies to
	//
	// t = x₀/(P₁±√(P₁²+(P₂-2P₁)x₀))
	//
	// Setting v = P₂-P₁,
	//
	// t = x₀/(P₁±√(P₁²+(v-P₁)x₀))
	//
	// t lie in [0; 1]; P₂ ≥ P₁ and P₁ ≥ 0 since we split curves where
	// the control point lies before the start point or after the end point.
	// It can then be shown that only the positive square root is valid.
	float midx = mix(extent.x, extent.y, 0.5);
	float x0 = midx - left.x;
	vec2 p1 = vCtrl - left;
	vec2 v = right - vCtrl;
	float t = x0/(p1.x+sqrt(p1.x*p1.x+(v.x-p1.x)*x0));
	// Find y(t) on the curve.
	float y = mix(mix(left.y, vCtrl.y, t), mix(vCtrl.y, right.y, t), t);
	// And the slope.
	vec2 d_half = mix(p1, v, t);
	float dy = d_half.y/d_half.x;
	// Together, y and dy form a line approximation.

	// Compute the fragment area above the line.
	// The area is symmetric around dy = 0. Scale slope with extent width.
	float width = extent.y - extent.x;
	dy = abs(dy*width);

	vec4 sides = vec4(dy*+0.5 + y, dy*-0.5 + y, (+0.5-y)/dy, (-0.5-y)/dy);
	sides = clamp(sides+0.5, 0.0, 1.0);

	float area = 0.5*(sides.z - sides.z*sides.y + 1.0 - sides.x+sides.x*sides.w);
	area *= width;

	// Work around issue #13.
	if (width == 0.0)
		area = 0.0;

	gl_FragColor.r = area;
}
`

const coverVSrc = `
#version 100

precision highp float;

uniform float z;
uniform vec2 scale;
uniform vec2 offset;
uniform vec2 uvScale;
uniform vec2 uvOffset;
uniform vec2 uvCoverScale;
uniform vec2 uvCoverOffset;

attribute vec2 pos;

varying vec2 vCoverUV;

attribute vec2 uv;
varying vec2 vUV;

void main() {
    gl_Position = vec4(pos*scale + offset, z, 1);
	vUV = uv*uvScale + uvOffset;
	vCoverUV = uv*uvCoverScale+uvCoverOffset;
}
`

const coverFSrc = `
#version 100

precision mediump float;

// Use high precision to be pixel accurate for
// large cover atlases.
varying highp vec2 vCoverUV;
uniform sampler2D cover;
varying vec2 vUV;

HEADER

void main() {
    gl_FragColor = GET_COLOR;
	float cover = abs(texture2D(cover, vCoverUV).r);
	gl_FragColor *= cover;
}
`

const intersectVSrc = `
#version 100

precision highp float;

attribute vec2 pos;
attribute vec2 uv;

uniform vec2 uvScale;
uniform vec2 uvOffset;

varying vec2 vUV;

void main() {
	vec2 p = pos;
	p.y = -p.y;
	gl_Position = vec4(p, 0, 1);
	vUV = uv*uvScale + uvOffset;
}
`

const intersectFSrc = `
#version 100

precision mediump float;

// Use high precision to be pixel accurate for
// large cover atlases.
varying highp vec2 vUV;
uniform sampler2D cover;

void main() {
	float cover = abs(texture2D(cover, vUV).r);
    gl_FragColor.r = cover;
}
`
