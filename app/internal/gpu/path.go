// SPDX-License-Identifier: Unlicense OR MIT

package gpu

// GPU accelerated path drawing using the algorithms from
// Pathfinder (https://github.com/servo/pathfinder).

import (
	"image"
	"unsafe"

	"gioui.org/app/internal/gl"
	"gioui.org/f32"
	"gioui.org/internal/path"
)

type pather struct {
	ctx *context

	viewport image.Point

	stenciler *stenciler
	coverer   *coverer
}

type coverer struct {
	ctx  *context
	prog [2]gl.Program
	vars [2]struct {
		z                             gl.Uniform
		uScale, uOffset               gl.Uniform
		uUVScale, uUVOffset           gl.Uniform
		uCoverUVScale, uCoverUVOffset gl.Uniform
		uColor                        gl.Uniform
	}
}

type stenciler struct {
	ctx                *context
	defFBO             gl.Framebuffer
	indexBufQuads      int
	prog               gl.Program
	iprog              gl.Program
	fbos               fboSet
	intersections      fboSet
	uScale, uOffset    gl.Uniform
	uPathOffset        gl.Uniform
	uIntersectUVOffset gl.Uniform
	uIntersectUVScale  gl.Uniform
	indexBuf           gl.Buffer
}

type fboSet struct {
	fbos []stencilFBO
}

type stencilFBO struct {
	size image.Point
	fbo  gl.Framebuffer
	tex  gl.Texture
}

type pathData struct {
	ncurves int
	data    gl.Buffer
}

var (
	pathAttribs                = []string{"corner", "maxy", "from", "ctrl", "to"}
	attribPathCorner gl.Attrib = 0
	attribPathMaxY   gl.Attrib = 1
	attribPathFrom   gl.Attrib = 2
	attribPathCtrl   gl.Attrib = 3
	attribPathTo     gl.Attrib = 4

	intersectAttribs = []string{"pos", "uv"}
)

func newPather(ctx *context) *pather {
	return &pather{
		ctx:       ctx,
		stenciler: newStenciler(ctx),
		coverer:   newCoverer(ctx),
	}
}

func newCoverer(ctx *context) *coverer {
	prog, err := createColorPrograms(ctx, coverVSrc, coverFSrc)
	if err != nil {
		panic(err)
	}
	c := &coverer{
		ctx:  ctx,
		prog: prog,
	}
	for i, prog := range prog {
		ctx.UseProgram(prog)
		switch materialType(i) {
		case materialTexture:
			uTex := gl.GetUniformLocation(ctx.Functions, prog, "tex")
			ctx.Uniform1i(uTex, 0)
			c.vars[i].uUVScale = gl.GetUniformLocation(ctx.Functions, prog, "uvScale")
			c.vars[i].uUVOffset = gl.GetUniformLocation(ctx.Functions, prog, "uvOffset")
		case materialColor:
			c.vars[i].uColor = gl.GetUniformLocation(ctx.Functions, prog, "color")
		}
		uCover := gl.GetUniformLocation(ctx.Functions, prog, "cover")
		ctx.Uniform1i(uCover, 1)
		c.vars[i].z = gl.GetUniformLocation(ctx.Functions, prog, "z")
		c.vars[i].uScale = gl.GetUniformLocation(ctx.Functions, prog, "scale")
		c.vars[i].uOffset = gl.GetUniformLocation(ctx.Functions, prog, "offset")
		c.vars[i].uCoverUVScale = gl.GetUniformLocation(ctx.Functions, prog, "uvCoverScale")
		c.vars[i].uCoverUVOffset = gl.GetUniformLocation(ctx.Functions, prog, "uvCoverOffset")
	}
	return c
}

func newStenciler(ctx *context) *stenciler {
	defFBO := gl.Framebuffer(ctx.GetBinding(gl.FRAMEBUFFER_BINDING))
	prog, err := gl.CreateProgram(ctx.Functions, stencilVSrc, stencilFSrc, pathAttribs)
	if err != nil {
		panic(err)
	}
	ctx.UseProgram(prog)
	iprog, err := gl.CreateProgram(ctx.Functions, intersectVSrc, intersectFSrc, intersectAttribs)
	if err != nil {
		panic(err)
	}
	coverLoc := gl.GetUniformLocation(ctx.Functions, iprog, "cover")
	ctx.UseProgram(iprog)
	ctx.Uniform1i(coverLoc, 0)
	return &stenciler{
		ctx:                ctx,
		defFBO:             defFBO,
		prog:               prog,
		iprog:              iprog,
		uScale:             gl.GetUniformLocation(ctx.Functions, prog, "scale"),
		uOffset:            gl.GetUniformLocation(ctx.Functions, prog, "offset"),
		uPathOffset:        gl.GetUniformLocation(ctx.Functions, prog, "pathOffset"),
		uIntersectUVScale:  gl.GetUniformLocation(ctx.Functions, iprog, "uvScale"),
		uIntersectUVOffset: gl.GetUniformLocation(ctx.Functions, iprog, "uvOffset"),
		indexBuf:           ctx.CreateBuffer(),
	}
}

func (s *fboSet) resize(ctx *context, sizes []image.Point) {
	// Add fbos.
	for i := len(s.fbos); i < len(sizes); i++ {
		tex := ctx.CreateTexture()
		ctx.BindTexture(gl.TEXTURE_2D, tex)
		ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
		ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
		fbo := ctx.CreateFramebuffer()
		s.fbos = append(s.fbos, stencilFBO{
			fbo: fbo,
			tex: tex,
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
			ctx.BindTexture(gl.TEXTURE_2D, f.tex)
			tt := ctx.caps.floatTriple
			ctx.TexImage2D(gl.TEXTURE_2D, 0, tt.internalFormat, sz.X, sz.Y, tt.format, tt.typ, nil)
			ctx.BindFramebuffer(gl.FRAMEBUFFER, f.fbo)
			ctx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, f.tex, 0)
		}
	}
	// Delete extra fbos.
	s.delete(ctx, len(sizes))
}

func (s *fboSet) invalidate(ctx *context) {
	for _, f := range s.fbos {
		ctx.BindFramebuffer(gl.FRAMEBUFFER, f.fbo)
		ctx.InvalidateFramebuffer(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0)
	}
}

func (s *fboSet) delete(ctx *context, idx int) {
	for i := idx; i < len(s.fbos); i++ {
		f := s.fbos[i]
		ctx.DeleteFramebuffer(f.fbo)
		ctx.DeleteTexture(f.tex)
	}
	s.fbos = s.fbos[:idx]
}

func (s *stenciler) release() {
	s.fbos.delete(s.ctx, 0)
	s.ctx.DeleteProgram(s.prog)
	s.ctx.DeleteBuffer(s.indexBuf)
}

func (p *pather) release() {
	p.stenciler.release()
	p.coverer.release()
}

func (c *coverer) release() {
	for _, p := range c.prog {
		c.ctx.DeleteProgram(p)
	}
}

func buildPath(ctx *context, p []byte) *pathData {
	buf := ctx.CreateBuffer()
	ctx.BindBuffer(gl.ARRAY_BUFFER, buf)
	ctx.BufferData(gl.ARRAY_BUFFER, p, gl.STATIC_DRAW)
	return &pathData{
		ncurves: len(p) / path.VertStride,
		data:    buf,
	}
}

func (p *pathData) release(ctx *context) {
	ctx.DeleteBuffer(p.data)
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
	s.ctx.ActiveTexture(gl.TEXTURE1)
	s.ctx.BindTexture(gl.TEXTURE_2D, gl.Texture{})
	s.ctx.ActiveTexture(gl.TEXTURE0)
	s.ctx.BlendFunc(gl.DST_COLOR, gl.ZERO)
	// 8 bit coverage is enough, but OpenGL ES only supports single channel
	// floating point formats. Replace with GL_RGB+GL_UNSIGNED_BYTE if
	// no floating point support is available.
	s.intersections.resize(s.ctx, sizes)
	s.ctx.ClearColor(1.0, 0.0, 0.0, 0.0)
	s.ctx.UseProgram(s.iprog)
}

func (s *stenciler) endIntersect() {
	s.ctx.BindFramebuffer(gl.FRAMEBUFFER, s.defFBO)
}

func (s *stenciler) invalidateFBO() {
	s.intersections.invalidate(s.ctx)
	s.fbos.invalidate(s.ctx)
	s.ctx.BindFramebuffer(gl.FRAMEBUFFER, s.defFBO)
}

func (s *stenciler) cover(idx int) stencilFBO {
	return s.fbos.fbos[idx]
}

func (s *stenciler) begin(sizes []image.Point) {
	s.ctx.ActiveTexture(gl.TEXTURE1)
	s.ctx.BindTexture(gl.TEXTURE_2D, gl.Texture{})
	s.ctx.ActiveTexture(gl.TEXTURE0)
	s.ctx.BlendFunc(gl.ONE, gl.ONE)
	s.fbos.resize(s.ctx, sizes)
	s.ctx.ClearColor(0.0, 0.0, 0.0, 0.0)
	s.ctx.UseProgram(s.prog)
	s.ctx.EnableVertexAttribArray(attribPathCorner)
	s.ctx.EnableVertexAttribArray(attribPathMaxY)
	s.ctx.EnableVertexAttribArray(attribPathFrom)
	s.ctx.EnableVertexAttribArray(attribPathCtrl)
	s.ctx.EnableVertexAttribArray(attribPathTo)
	s.ctx.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, s.indexBuf)
}

func (s *stenciler) stencilPath(bounds image.Rectangle, offset f32.Point, uv image.Point, data *pathData) {
	s.ctx.BindBuffer(gl.ARRAY_BUFFER, data.data)
	s.ctx.Viewport(uv.X, uv.Y, bounds.Dx(), bounds.Dy())
	// Transform UI coordinates to OpenGL coordinates.
	texSize := f32.Point{X: float32(bounds.Dx()), Y: float32(bounds.Dy())}
	scale := f32.Point{X: 2 / texSize.X, Y: 2 / texSize.Y}
	orig := f32.Point{X: -1 - float32(bounds.Min.X)*2/texSize.X, Y: -1 - float32(bounds.Min.Y)*2/texSize.Y}
	s.ctx.Uniform2f(s.uScale, scale.X, scale.Y)
	s.ctx.Uniform2f(s.uOffset, orig.X, orig.Y)
	s.ctx.Uniform2f(s.uPathOffset, offset.X, offset.Y)
	// Draw in batches that fit in uint16 indices.
	start := 0
	nquads := data.ncurves / 4
	for start < nquads {
		batch := nquads - start
		if max := int(^uint16(0)) / 6; batch > max {
			batch = max
		}
		// Enlarge VBO if necessary.
		if batch > s.indexBufQuads {
			indices := make([]uint16, batch*6)
			for i := 0; i < batch; i++ {
				i := uint16(i)
				indices[i*6+0] = i*4 + 0
				indices[i*6+1] = i*4 + 1
				indices[i*6+2] = i*4 + 2
				indices[i*6+3] = i*4 + 2
				indices[i*6+4] = i*4 + 1
				indices[i*6+5] = i*4 + 3
			}
			s.ctx.BufferData(gl.ELEMENT_ARRAY_BUFFER, gl.BytesView(indices), gl.STATIC_DRAW)
			s.indexBufQuads = batch
		}
		off := path.VertStride * start * 4
		s.ctx.VertexAttribPointer(attribPathCorner, 2, gl.SHORT, false, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).CornerX)))
		s.ctx.VertexAttribPointer(attribPathMaxY, 1, gl.FLOAT, false, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).MaxY)))
		s.ctx.VertexAttribPointer(attribPathFrom, 2, gl.FLOAT, false, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).FromX)))
		s.ctx.VertexAttribPointer(attribPathCtrl, 2, gl.FLOAT, false, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).CtrlX)))
		s.ctx.VertexAttribPointer(attribPathTo, 2, gl.FLOAT, false, path.VertStride, off+int(unsafe.Offsetof((*(*path.Vertex)(nil)).ToX)))
		s.ctx.DrawElements(gl.TRIANGLES, batch*6, gl.UNSIGNED_SHORT, 0)
		start += batch
	}
}

func (s *stenciler) end() {
	s.ctx.DisableVertexAttribArray(attribPathCorner)
	s.ctx.DisableVertexAttribArray(attribPathMaxY)
	s.ctx.DisableVertexAttribArray(attribPathFrom)
	s.ctx.DisableVertexAttribArray(attribPathCtrl)
	s.ctx.DisableVertexAttribArray(attribPathTo)
	s.ctx.BindFramebuffer(gl.FRAMEBUFFER, s.defFBO)
}

func (p *pather) cover(z float32, mat materialType, col [4]float32, scale, off, uvScale, uvOff, coverScale, coverOff f32.Point) {
	p.coverer.cover(z, mat, col, scale, off, uvScale, uvOff, coverScale, coverOff)
}

func (c *coverer) cover(z float32, mat materialType, col [4]float32, scale, off, uvScale, uvOff, coverScale, coverOff f32.Point) {
	c.ctx.UseProgram(c.prog[mat])
	switch mat {
	case materialColor:
		c.ctx.Uniform4f(c.vars[mat].uColor, col[0], col[1], col[2], col[3])
	case materialTexture:
		c.ctx.Uniform2f(c.vars[mat].uUVScale, uvScale.X, uvScale.Y)
		c.ctx.Uniform2f(c.vars[mat].uUVOffset, uvOff.X, uvOff.Y)
	}
	c.ctx.Uniform1f(c.vars[mat].z, z)
	c.ctx.Uniform2f(c.vars[mat].uScale, scale.X, scale.Y)
	c.ctx.Uniform2f(c.vars[mat].uOffset, off.X, off.Y)
	c.ctx.Uniform2f(c.vars[mat].uCoverUVScale, coverScale.X, coverScale.Y)
	c.ctx.Uniform2f(c.vars[mat].uCoverUVOffset, coverOff.X, coverOff.Y)
	c.ctx.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
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
