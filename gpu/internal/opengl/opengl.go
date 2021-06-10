// SPDX-License-Identifier: Unlicense OR MIT

package opengl

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"time"
	"unsafe"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/gl"
)

// Backend implements driver.Device.
type Backend struct {
	funcs *gl.Functions

	clear      bool
	glstate    glState
	state      state
	savedState glState

	glver [2]int
	gles  bool
	ubo   bool
	feats driver.Caps
	// floatTriple holds the settings for floating point
	// textures.
	floatTriple textureTriple
	// Single channel alpha textures.
	alphaTriple textureTriple
	srgbaTriple textureTriple

	sRGBFBO *SRGBFBO

	// vertArray is bound during a frame. We don't need it, but
	// core desktop OpenGL profile 3.3 requires some array bound.
	vertArray gl.VertexArray
}

// State tracking.
type glState struct {
	drawFBO     gl.Framebuffer
	readFBO     gl.Framebuffer
	renderBuf   gl.Renderbuffer
	vertAttribs [5]struct {
		obj        gl.Buffer
		enabled    bool
		size       int
		typ        gl.Enum
		normalized bool
		stride     int
		offset     uintptr
	}
	prog     gl.Program
	texUnits struct {
		active gl.Enum
		binds  [2]gl.Texture
	}
	arrayBuf  gl.Buffer
	elemBuf   gl.Buffer
	uniBuf    gl.Buffer
	uniBufs   [2]gl.Buffer
	storeBuf  gl.Buffer
	storeBufs [4]gl.Buffer
	vertArray gl.VertexArray
	depthMask bool
	depthFunc gl.Enum
	srgb      bool
	blend     struct {
		enable         bool
		srcRGB, dstRGB gl.Enum
		srcA, dstA     gl.Enum
	}
	depthTest  bool
	clearColor [4]float32
	clearDepth float32
	viewport   [4]int
}

type state struct {
	prog   *gpuProgram
	layout *gpuInputLayout
	buffer bufferBinding
}

type bufferBinding struct {
	obj    gl.Buffer
	offset int
	stride int
}

type gpuTimer struct {
	funcs *gl.Functions
	obj   gl.Query
}

type gpuTexture struct {
	backend *Backend
	obj     gl.Texture
	triple  textureTriple
	width   int
	height  int
}

type gpuFramebuffer struct {
	backend  *Backend
	obj      gl.Framebuffer
	hasDepth bool
	depthBuf gl.Renderbuffer
	foreign  bool
}

type gpuBuffer struct {
	backend   *Backend
	hasBuffer bool
	obj       gl.Buffer
	typ       driver.BufferBinding
	size      int
	immutable bool
	version   int
	// For emulation of uniform buffers.
	data []byte
}

type gpuProgram struct {
	backend      *Backend
	obj          gl.Program
	vertUniforms uniformsTracker
	fragUniforms uniformsTracker
	storage      [storageBindings]*gpuBuffer
}

type uniformsTracker struct {
	locs    []uniformLocation
	size    int
	buf     *gpuBuffer
	version int
}

type uniformLocation struct {
	uniform gl.Uniform
	offset  int
	typ     driver.DataType
	size    int
}

type gpuInputLayout struct {
	inputs []driver.InputLocation
	layout []driver.InputDesc
}

// textureTriple holds the type settings for
// a TexImage2D call.
type textureTriple struct {
	internalFormat gl.Enum
	format         gl.Enum
	typ            gl.Enum
}

const (
	storageBindings = 32
)

func init() {
	driver.NewOpenGLDevice = newOpenGLDevice
}

func newOpenGLDevice(api driver.OpenGL) (driver.Device, error) {
	f, err := gl.NewFunctions(api.Context, api.ES)
	if err != nil {
		return nil, err
	}
	exts := strings.Split(f.GetString(gl.EXTENSIONS), " ")
	glVer := f.GetString(gl.VERSION)
	ver, gles, err := gl.ParseGLVersion(glVer)
	if err != nil {
		return nil, err
	}
	floatTriple, ffboErr := floatTripleFor(f, ver, exts)
	srgbaTriple, err := srgbaTripleFor(ver, exts)
	if err != nil {
		return nil, err
	}
	gles30 := gles && ver[0] >= 3
	gles31 := gles && (ver[0] > 3 || (ver[0] == 3 && ver[1] >= 1))
	gl40 := !gles && ver[0] >= 4
	b := &Backend{
		glver:       ver,
		gles:        gles,
		ubo:         gles30 || gl40,
		funcs:       f,
		floatTriple: floatTriple,
		alphaTriple: alphaTripleFor(ver),
		srgbaTriple: srgbaTriple,
	}
	b.feats.BottomLeftOrigin = true
	if ffboErr == nil {
		b.feats.Features |= driver.FeatureFloatRenderTargets
	}
	if gles31 {
		b.feats.Features |= driver.FeatureCompute
	}
	if hasExtension(exts, "GL_EXT_disjoint_timer_query_webgl2") || hasExtension(exts, "GL_EXT_disjoint_timer_query") {
		b.feats.Features |= driver.FeatureTimers
	}
	b.feats.MaxTextureSize = f.GetInteger(gl.MAX_TEXTURE_SIZE)
	return b, nil
}

func (b *Backend) BeginFrame(clear bool, viewport image.Point) driver.Framebuffer {
	b.clear = clear
	b.glstate = b.queryState()
	b.savedState = b.glstate
	b.state = state{}
	renderFBO := b.glstate.drawFBO
	if b.gles {
		// If the output framebuffer is not in the sRGB colorspace already, emulate it.
		var fbEncoding int
		if !renderFBO.Valid() {
			fbEncoding = b.funcs.GetFramebufferAttachmentParameteri(gl.FRAMEBUFFER, gl.BACK, gl.FRAMEBUFFER_ATTACHMENT_COLOR_ENCODING)
		} else {
			fbEncoding = b.funcs.GetFramebufferAttachmentParameteri(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.FRAMEBUFFER_ATTACHMENT_COLOR_ENCODING)
		}
		if fbEncoding == gl.LINEAR && viewport != (image.Point{}) {
			if b.sRGBFBO == nil {
				sfbo, err := NewSRGBFBO(b.funcs, &b.glstate)
				if err != nil {
					panic(err)
				}
				b.sRGBFBO = sfbo
			}
			if err := b.sRGBFBO.Refresh(viewport); err != nil {
				panic(err)
			}
			renderFBO = b.sRGBFBO.Framebuffer()
		}
	} else {
		b.glstate.set(b.funcs, gl.FRAMEBUFFER_SRGB, true)
		if !b.vertArray.Valid() {
			b.vertArray = b.funcs.CreateVertexArray()
		}
		b.glstate.bindVertexArray(b.funcs, b.vertArray)
	}
	b.glstate.bindFramebuffer(b.funcs, gl.FRAMEBUFFER, renderFBO)
	if b.sRGBFBO != nil && !clear {
		b.Clear(0, 0, 0, 0)
	}
	return &gpuFramebuffer{backend: b, obj: renderFBO, foreign: true}
}

func (b *Backend) EndFrame() {
	if b.sRGBFBO != nil {
		b.glstate.bindFramebuffer(b.funcs, gl.FRAMEBUFFER, b.savedState.drawFBO)
		if b.clear {
			b.SetBlend(false)
		} else {
			b.BlendFunc(driver.BlendFactorOne, driver.BlendFactorOneMinusSrcAlpha)
			b.SetBlend(true)
		}
		b.sRGBFBO.Blit()
	}
	b.restoreState(b.savedState)
	// For single-buffered framebuffers such as on macOS.
	b.funcs.Flush()
}

func (b *Backend) queryState() glState {
	s := glState{
		prog:       gl.Program(b.funcs.GetBinding(gl.CURRENT_PROGRAM)),
		arrayBuf:   gl.Buffer(b.funcs.GetBinding(gl.ARRAY_BUFFER_BINDING)),
		elemBuf:    gl.Buffer(b.funcs.GetBinding(gl.ELEMENT_ARRAY_BUFFER_BINDING)),
		drawFBO:    gl.Framebuffer(b.funcs.GetBinding(gl.FRAMEBUFFER_BINDING)),
		depthMask:  b.funcs.GetInteger(gl.DEPTH_WRITEMASK) != gl.FALSE,
		depthTest:  b.funcs.IsEnabled(gl.DEPTH_TEST),
		depthFunc:  gl.Enum(b.funcs.GetInteger(gl.DEPTH_FUNC)),
		clearDepth: b.funcs.GetFloat(gl.DEPTH_CLEAR_VALUE),
		clearColor: b.funcs.GetFloat4(gl.COLOR_CLEAR_VALUE),
		viewport:   b.funcs.GetInteger4(gl.VIEWPORT),
	}
	s.blend.enable = b.funcs.IsEnabled(gl.BLEND)
	s.blend.srcRGB = gl.Enum(b.funcs.GetInteger(gl.BLEND_SRC_RGB))
	s.blend.dstRGB = gl.Enum(b.funcs.GetInteger(gl.BLEND_DST_RGB))
	s.blend.srcA = gl.Enum(b.funcs.GetInteger(gl.BLEND_SRC_ALPHA))
	s.blend.dstA = gl.Enum(b.funcs.GetInteger(gl.BLEND_DST_ALPHA))
	s.texUnits.active = gl.Enum(b.funcs.GetInteger(gl.ACTIVE_TEXTURE))
	if !b.gles {
		s.srgb = b.funcs.IsEnabled(gl.FRAMEBUFFER_SRGB)
	}
	if !b.gles || b.glver[0] >= 3 {
		s.vertArray = gl.VertexArray(b.funcs.GetBinding(gl.VERTEX_ARRAY_BINDING))
		s.readFBO = gl.Framebuffer(b.funcs.GetBinding(gl.READ_FRAMEBUFFER_BINDING))
		s.uniBuf = gl.Buffer(b.funcs.GetBinding(gl.UNIFORM_BUFFER_BINDING))
		for i := range s.uniBufs {
			s.uniBufs[i] = gl.Buffer(b.funcs.GetBindingi(gl.UNIFORM_BUFFER_BINDING, i))
		}
	}
	if b.gles && (b.glver[0] > 3 || (b.glver[0] == 3 && b.glver[1] >= 1)) {
		s.storeBuf = gl.Buffer(b.funcs.GetBinding(gl.SHADER_STORAGE_BUFFER_BINDING))
		for i := range s.storeBufs {
			s.storeBufs[i] = gl.Buffer(b.funcs.GetBindingi(gl.SHADER_STORAGE_BUFFER_BINDING, i))
		}
	}
	for i := range s.texUnits.binds {
		s.activeTexture(b.funcs, gl.TEXTURE0+gl.Enum(i))
		s.texUnits.binds[i] = gl.Texture(b.funcs.GetBinding(gl.TEXTURE_BINDING_2D))
	}
	for i := range s.vertAttribs {
		a := &s.vertAttribs[i]
		a.enabled = b.funcs.GetVertexAttrib(i, gl.VERTEX_ATTRIB_ARRAY_ENABLED) != gl.FALSE
		a.obj = gl.Buffer(b.funcs.GetVertexAttribBinding(i, gl.VERTEX_ATTRIB_ARRAY_ENABLED))
		a.size = b.funcs.GetVertexAttrib(i, gl.VERTEX_ATTRIB_ARRAY_SIZE)
		a.typ = gl.Enum(b.funcs.GetVertexAttrib(i, gl.VERTEX_ATTRIB_ARRAY_TYPE))
		a.normalized = b.funcs.GetVertexAttrib(i, gl.VERTEX_ATTRIB_ARRAY_NORMALIZED) != gl.FALSE
		a.stride = b.funcs.GetVertexAttrib(i, gl.VERTEX_ATTRIB_ARRAY_STRIDE)
		a.offset = b.funcs.GetVertexAttribPointer(i, gl.VERTEX_ATTRIB_ARRAY_POINTER)
	}
	return s
}

func (b *Backend) restoreState(dst glState) {
	src := b.glstate
	f := b.funcs
	for i, unit := range dst.texUnits.binds {
		src.bindTexture(f, i, unit)
	}
	src.activeTexture(f, dst.texUnits.active)
	src.bindFramebuffer(f, gl.FRAMEBUFFER, dst.drawFBO)
	src.bindFramebuffer(f, gl.READ_FRAMEBUFFER, dst.readFBO)
	src.set(f, gl.BLEND, dst.blend.enable)
	bf := dst.blend
	src.setBlendFuncSeparate(f, bf.srcRGB, bf.dstRGB, bf.srcA, bf.dstA)
	src.set(f, gl.DEPTH_TEST, dst.depthTest)
	src.setDepthFunc(f, dst.depthFunc)
	src.set(f, gl.FRAMEBUFFER_SRGB, dst.srgb)
	src.bindVertexArray(f, dst.vertArray)
	src.useProgram(f, dst.prog)
	src.bindBuffer(f, gl.ELEMENT_ARRAY_BUFFER, dst.elemBuf)
	for i, b := range dst.uniBufs {
		src.bindBufferBase(f, gl.UNIFORM_BUFFER, i, b)
	}
	src.bindBuffer(f, gl.UNIFORM_BUFFER, dst.uniBuf)
	for i, b := range dst.storeBufs {
		src.bindBufferBase(f, gl.SHADER_STORAGE_BUFFER, i, b)
	}
	src.bindBuffer(f, gl.SHADER_STORAGE_BUFFER, dst.storeBuf)
	src.setDepthMask(f, dst.depthMask)
	src.setClearDepth(f, dst.clearDepth)
	col := dst.clearColor
	src.setClearColor(f, col[0], col[1], col[2], col[3])
	for i, attr := range dst.vertAttribs {
		src.setVertexAttribArray(f, i, attr.enabled)
		src.vertexAttribPointer(f, attr.obj, i, attr.size, attr.typ, attr.normalized, attr.stride, int(attr.offset))
	}
	src.bindBuffer(f, gl.ARRAY_BUFFER, dst.arrayBuf)
	v := dst.viewport
	src.setViewport(f, v[0], v[1], v[2], v[3])
}

func (s *glState) setVertexAttribArray(f *gl.Functions, idx int, enabled bool) {
	a := &s.vertAttribs[idx]
	if enabled != a.enabled {
		if enabled {
			f.EnableVertexAttribArray(gl.Attrib(idx))
		} else {
			f.DisableVertexAttribArray(gl.Attrib(idx))
		}
		a.enabled = enabled
	}
}

func (s *glState) vertexAttribPointer(f *gl.Functions, buf gl.Buffer, idx, size int, typ gl.Enum, normalized bool, stride, offset int) {
	s.bindBuffer(f, gl.ARRAY_BUFFER, buf)
	a := &s.vertAttribs[idx]
	a.obj = buf
	a.size = size
	a.typ = typ
	a.normalized = normalized
	a.stride = stride
	a.offset = uintptr(offset)
	f.VertexAttribPointer(gl.Attrib(idx), a.size, a.typ, a.normalized, a.stride, int(a.offset))
}

func (s *glState) activeTexture(f *gl.Functions, unit gl.Enum) {
	if unit != s.texUnits.active {
		f.ActiveTexture(unit)
		s.texUnits.active = unit
	}
}

func (s *glState) bindRenderbuffer(f *gl.Functions, target gl.Enum, r gl.Renderbuffer) {
	if !r.Equal(s.renderBuf) {
		f.BindRenderbuffer(gl.RENDERBUFFER, r)
		s.renderBuf = r
	}
}

func (s *glState) bindTexture(f *gl.Functions, unit int, t gl.Texture) {
	s.activeTexture(f, gl.TEXTURE0+gl.Enum(unit))
	if !t.Equal(s.texUnits.binds[unit]) {
		f.BindTexture(gl.TEXTURE_2D, t)
		s.texUnits.binds[unit] = t
	}
}

func (s *glState) bindVertexArray(f *gl.Functions, a gl.VertexArray) {
	if !a.Equal(s.vertArray) {
		f.BindVertexArray(a)
		s.vertArray = a
	}
}

func (s *glState) deleteRenderbuffer(f *gl.Functions, r gl.Renderbuffer) {
	f.DeleteRenderbuffer(r)
	if r.Equal(s.renderBuf) {
		s.renderBuf = gl.Renderbuffer{}
	}
}

func (s *glState) deleteFramebuffer(f *gl.Functions, fbo gl.Framebuffer) {
	f.DeleteFramebuffer(fbo)
	if fbo.Equal(s.drawFBO) {
		s.drawFBO = gl.Framebuffer{}
	}
	if fbo.Equal(s.readFBO) {
		s.readFBO = gl.Framebuffer{}
	}
}

func (s *glState) deleteBuffer(f *gl.Functions, b gl.Buffer) {
	f.DeleteBuffer(b)
	if b.Equal(s.arrayBuf) {
		s.arrayBuf = gl.Buffer{}
	}
	if b.Equal(s.elemBuf) {
		s.elemBuf = gl.Buffer{}
	}
	if b.Equal(s.uniBuf) {
		s.uniBuf = gl.Buffer{}
	}
	if b.Equal(s.storeBuf) {
		s.uniBuf = gl.Buffer{}
	}
	for i, b2 := range s.storeBufs {
		if b.Equal(b2) {
			s.storeBufs[i] = gl.Buffer{}
		}
	}
	for i, b2 := range s.uniBufs {
		if b.Equal(b2) {
			s.uniBufs[i] = gl.Buffer{}
		}
	}
}

func (s *glState) deleteProgram(f *gl.Functions, p gl.Program) {
	f.DeleteProgram(p)
	if p.Equal(s.prog) {
		s.prog = gl.Program{}
	}
}

func (s *glState) deleteVertexArray(f *gl.Functions, a gl.VertexArray) {
	f.DeleteVertexArray(a)
	if a.Equal(s.vertArray) {
		s.vertArray = gl.VertexArray{}
	}
}

func (s *glState) deleteTexture(f *gl.Functions, t gl.Texture) {
	f.DeleteTexture(t)
	binds := &s.texUnits.binds
	for i, obj := range binds {
		if t.Equal(obj) {
			binds[i] = gl.Texture{}
		}
	}
}

func (s *glState) useProgram(f *gl.Functions, p gl.Program) {
	if !p.Equal(s.prog) {
		f.UseProgram(p)
		s.prog = p
	}
}

func (s *glState) bindFramebuffer(f *gl.Functions, target gl.Enum, fbo gl.Framebuffer) {
	switch target {
	case gl.FRAMEBUFFER:
		if fbo.Equal(s.drawFBO) && fbo.Equal(s.readFBO) {
			return
		}
		s.drawFBO = fbo
		s.readFBO = fbo
	case gl.READ_FRAMEBUFFER:
		if fbo.Equal(s.readFBO) {
			return
		}
		s.readFBO = fbo
	case gl.DRAW_FRAMEBUFFER:
		if fbo.Equal(s.drawFBO) {
			return
		}
		s.drawFBO = fbo
	default:
		panic("unknown target")
	}
	f.BindFramebuffer(target, fbo)
}

func (s *glState) bindBufferBase(f *gl.Functions, target gl.Enum, idx int, buf gl.Buffer) {
	switch target {
	case gl.UNIFORM_BUFFER:
		if buf.Equal(s.uniBuf) && buf.Equal(s.uniBufs[idx]) {
			return
		}
		s.uniBuf = buf
		s.uniBufs[idx] = buf
	case gl.SHADER_STORAGE_BUFFER:
		if buf.Equal(s.storeBuf) && buf.Equal(s.storeBufs[idx]) {
			return
		}
		s.storeBuf = buf
		s.storeBufs[idx] = buf
	default:
		panic("unknown buffer target")
	}
	f.BindBufferBase(target, idx, buf)
}

func (s *glState) bindBuffer(f *gl.Functions, target gl.Enum, buf gl.Buffer) {
	switch target {
	case gl.ARRAY_BUFFER:
		if buf.Equal(s.arrayBuf) {
			return
		}
		s.arrayBuf = buf
	case gl.ELEMENT_ARRAY_BUFFER:
		if buf.Equal(s.elemBuf) {
			return
		}
		s.elemBuf = buf
	case gl.UNIFORM_BUFFER:
		if buf.Equal(s.uniBuf) {
			return
		}
		s.uniBuf = buf
	case gl.SHADER_STORAGE_BUFFER:
		if buf.Equal(s.storeBuf) {
			return
		}
		s.storeBuf = buf
	default:
		panic("unknown buffer target")
	}
	f.BindBuffer(target, buf)
}

func (s *glState) setClearDepth(f *gl.Functions, d float32) {
	if d != s.clearDepth {
		f.ClearDepthf(d)
		s.clearDepth = d
	}
}

func (s *glState) setClearColor(f *gl.Functions, r, g, b, a float32) {
	col := [4]float32{r, g, b, a}
	if col != s.clearColor {
		f.ClearColor(r, g, b, a)
		s.clearColor = col
	}
}

func (s *glState) setViewport(f *gl.Functions, x, y, width, height int) {
	view := [4]int{x, y, width, height}
	if view != s.viewport {
		f.Viewport(x, y, width, height)
		s.viewport = view
	}
}

func (s *glState) setDepthFunc(f *gl.Functions, df gl.Enum) {
	if df != s.depthFunc {
		f.DepthFunc(df)
		s.depthFunc = df
	}
}

func (s *glState) setBlendFuncSeparate(f *gl.Functions, srcRGB, dstRGB, srcA, dstA gl.Enum) {
	if srcRGB != s.blend.srcRGB || dstRGB != s.blend.dstRGB || srcA != s.blend.srcA || dstA != s.blend.dstA {
		s.blend.srcRGB = srcRGB
		s.blend.dstRGB = dstRGB
		s.blend.srcA = srcA
		s.blend.dstA = dstA
		f.BlendFuncSeparate(srcA, dstA, srcA, dstA)
	}
}

func (s *glState) setDepthMask(f *gl.Functions, enable bool) {
	if enable != s.depthMask {
		f.DepthMask(enable)
		s.depthMask = enable
	}
}

func (s *glState) set(f *gl.Functions, target gl.Enum, enable bool) {
	switch target {
	case gl.FRAMEBUFFER_SRGB:
		if s.srgb == enable {
			return
		}
		s.srgb = enable
	case gl.BLEND:
		if enable == s.blend.enable {
			return
		}
		s.blend.enable = enable
	case gl.DEPTH_TEST:
		if enable == s.depthTest {
			return
		}
		s.depthTest = enable
	default:
		panic("unknown enable")
	}
	if enable {
		f.Enable(target)
	} else {
		f.Disable(target)
	}
}

func (b *Backend) Caps() driver.Caps {
	return b.feats
}

func (b *Backend) NewTimer() driver.Timer {
	return &gpuTimer{
		funcs: b.funcs,
		obj:   b.funcs.CreateQuery(),
	}
}

func (b *Backend) IsTimeContinuous() bool {
	return b.funcs.GetInteger(gl.GPU_DISJOINT_EXT) == gl.FALSE
}

func (b *Backend) NewFramebuffer(tex driver.Texture, depthBits int) (driver.Framebuffer, error) {
	glErr(b.funcs)
	gltex := tex.(*gpuTexture)
	fb := b.funcs.CreateFramebuffer()
	fbo := &gpuFramebuffer{backend: b, obj: fb}
	b.BindFramebuffer(fbo)
	if err := glErr(b.funcs); err != nil {
		fbo.Release()
		return nil, err
	}
	b.funcs.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, gltex.obj, 0)
	if depthBits > 0 {
		size := gl.Enum(gl.DEPTH_COMPONENT16)
		switch {
		case depthBits > 24:
			size = gl.DEPTH_COMPONENT32F
		case depthBits > 16:
			size = gl.DEPTH_COMPONENT24
		}
		depthBuf := b.funcs.CreateRenderbuffer()
		b.glstate.bindRenderbuffer(b.funcs, gl.RENDERBUFFER, depthBuf)
		b.funcs.RenderbufferStorage(gl.RENDERBUFFER, size, gltex.width, gltex.height)
		b.funcs.FramebufferRenderbuffer(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.RENDERBUFFER, depthBuf)
		fbo.depthBuf = depthBuf
		fbo.hasDepth = true
		if err := glErr(b.funcs); err != nil {
			fbo.Release()
			return nil, err
		}
	}
	if st := b.funcs.CheckFramebufferStatus(gl.FRAMEBUFFER); st != gl.FRAMEBUFFER_COMPLETE {
		fbo.Release()
		return nil, fmt.Errorf("incomplete framebuffer, status = 0x%x, err = %d", st, b.funcs.GetError())
	}
	return fbo, nil
}

func (b *Backend) NewTexture(format driver.TextureFormat, width, height int, minFilter, magFilter driver.TextureFilter, binding driver.BufferBinding) (driver.Texture, error) {
	glErr(b.funcs)
	tex := &gpuTexture{backend: b, obj: b.funcs.CreateTexture(), width: width, height: height}
	switch format {
	case driver.TextureFormatFloat:
		tex.triple = b.floatTriple
	case driver.TextureFormatSRGB:
		tex.triple = b.srgbaTriple
	case driver.TextureFormatRGBA8:
		tex.triple = textureTriple{gl.RGBA8, gl.RGBA, gl.UNSIGNED_BYTE}
	default:
		return nil, errors.New("unsupported texture format")
	}
	b.BindTexture(0, tex)
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, toTexFilter(magFilter))
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, toTexFilter(minFilter))
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	b.funcs.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	if b.gles && b.glver[0] >= 3 {
		// Immutable textures are required for BindImageTexture, and can't hurt otherwise.
		b.funcs.TexStorage2D(gl.TEXTURE_2D, 1, tex.triple.internalFormat, width, height)
	} else {
		b.funcs.TexImage2D(gl.TEXTURE_2D, 0, tex.triple.internalFormat, width, height, tex.triple.format, tex.triple.typ)
	}
	if err := glErr(b.funcs); err != nil {
		tex.Release()
		return nil, err
	}
	return tex, nil
}

func (b *Backend) NewBuffer(typ driver.BufferBinding, size int) (driver.Buffer, error) {
	glErr(b.funcs)
	buf := &gpuBuffer{backend: b, typ: typ, size: size}
	if typ&driver.BufferBindingUniforms != 0 {
		if typ != driver.BufferBindingUniforms {
			return nil, errors.New("uniforms buffers cannot be bound as anything else")
		}
		if !b.ubo {
			// GLES 2 doesn't support uniform buffers.
			buf.data = make([]byte, size)
		}
	}
	if typ&^driver.BufferBindingUniforms != 0 || b.ubo {
		buf.hasBuffer = true
		buf.obj = b.funcs.CreateBuffer()
		if err := glErr(b.funcs); err != nil {
			buf.Release()
			return nil, err
		}
		firstBinding := firstBufferType(typ)
		b.glstate.bindBuffer(b.funcs, firstBinding, buf.obj)
		b.funcs.BufferData(firstBinding, size, gl.DYNAMIC_DRAW)
	}
	return buf, nil
}

func (b *Backend) NewImmutableBuffer(typ driver.BufferBinding, data []byte) (driver.Buffer, error) {
	glErr(b.funcs)
	obj := b.funcs.CreateBuffer()
	buf := &gpuBuffer{backend: b, obj: obj, typ: typ, size: len(data), hasBuffer: true}
	firstBinding := firstBufferType(typ)
	b.glstate.bindBuffer(b.funcs, firstBinding, buf.obj)
	b.funcs.BufferData(firstBinding, len(data), gl.STATIC_DRAW)
	buf.Upload(data)
	buf.immutable = true
	if err := glErr(b.funcs); err != nil {
		buf.Release()
		return nil, err
	}
	return buf, nil
}

func glErr(f *gl.Functions) error {
	if st := f.GetError(); st != gl.NO_ERROR {
		return fmt.Errorf("glGetError: %#x", st)
	}
	return nil
}

func (b *Backend) Release() {
	if b.sRGBFBO != nil {
		b.sRGBFBO.Release()
	}
	if b.vertArray.Valid() {
		b.glstate.deleteVertexArray(b.funcs, b.vertArray)
	}
	*b = Backend{}
}

func (b *Backend) MemoryBarrier() {
	b.funcs.MemoryBarrier(gl.ALL_BARRIER_BITS)
}

func (b *Backend) DispatchCompute(x, y, z int) {
	if p := b.state.prog; p != nil {
		for binding, buf := range p.storage {
			if buf != nil {
				b.glstate.bindBufferBase(b.funcs, gl.SHADER_STORAGE_BUFFER, binding, buf.obj)
			}
		}
	}
	b.funcs.DispatchCompute(x, y, z)
}

func (b *Backend) BindImageTexture(unit int, tex driver.Texture, access driver.AccessBits, f driver.TextureFormat) {
	t := tex.(*gpuTexture)
	var acc gl.Enum
	switch access {
	case driver.AccessWrite:
		acc = gl.WRITE_ONLY
	case driver.AccessRead:
		acc = gl.READ_ONLY
	default:
		panic("unsupported access bits")
	}
	var format gl.Enum
	switch f {
	case driver.TextureFormatRGBA8:
		format = gl.RGBA8
	default:
		panic("unsupported format")
	}
	b.funcs.BindImageTexture(unit, t.obj, 0, false, 0, acc, format)
}

func (b *Backend) useProgram(p *gpuProgram) {
	b.glstate.useProgram(b.funcs, p.obj)
	b.state.prog = p
}

func (b *Backend) SetDepthTest(enable bool) {
	b.glstate.set(b.funcs, gl.DEPTH_TEST, enable)
}

func (b *Backend) BlendFunc(sfactor, dfactor driver.BlendFactor) {
	src, dst := toGLBlendFactor(sfactor), toGLBlendFactor(dfactor)
	b.glstate.setBlendFuncSeparate(b.funcs, src, dst, src, dst)
}

func toGLBlendFactor(f driver.BlendFactor) gl.Enum {
	switch f {
	case driver.BlendFactorOne:
		return gl.ONE
	case driver.BlendFactorOneMinusSrcAlpha:
		return gl.ONE_MINUS_SRC_ALPHA
	case driver.BlendFactorZero:
		return gl.ZERO
	case driver.BlendFactorDstColor:
		return gl.DST_COLOR
	default:
		panic("unsupported blend factor")
	}
}

func (b *Backend) DepthMask(mask bool) {
	b.glstate.setDepthMask(b.funcs, mask)
}

func (b *Backend) SetBlend(enable bool) {
	b.glstate.set(b.funcs, gl.BLEND, enable)
}

func (b *Backend) DrawElements(mode driver.DrawMode, off, count int) {
	b.prepareDraw()
	// off is in 16-bit indices, but DrawElements take a byte offset.
	byteOff := off * 2
	b.funcs.DrawElements(toGLDrawMode(mode), count, gl.UNSIGNED_SHORT, byteOff)
}

func (b *Backend) DrawArrays(mode driver.DrawMode, off, count int) {
	b.prepareDraw()
	b.funcs.DrawArrays(toGLDrawMode(mode), off, count)
}

func (b *Backend) prepareDraw() {
	p := b.state.prog
	if p == nil {
		return
	}
	b.setupVertexArrays()
	p.updateUniforms()
}

func toGLDrawMode(mode driver.DrawMode) gl.Enum {
	switch mode {
	case driver.DrawModeTriangleStrip:
		return gl.TRIANGLE_STRIP
	case driver.DrawModeTriangles:
		return gl.TRIANGLES
	default:
		panic("unsupported draw mode")
	}
}

func (b *Backend) Viewport(x, y, width, height int) {
	b.glstate.setViewport(b.funcs, x, y, width, height)
}

func (b *Backend) Clear(colR, colG, colB, colA float32) {
	b.glstate.setClearColor(b.funcs, colR, colG, colB, colA)
	b.funcs.Clear(gl.COLOR_BUFFER_BIT)
}

func (b *Backend) ClearDepth(d float32) {
	b.glstate.setClearDepth(b.funcs, d)
	b.funcs.Clear(gl.DEPTH_BUFFER_BIT)
}

func (b *Backend) DepthFunc(f driver.DepthFunc) {
	var glfunc gl.Enum
	switch f {
	case driver.DepthFuncGreater:
		glfunc = gl.GREATER
	case driver.DepthFuncGreaterEqual:
		glfunc = gl.GEQUAL
	default:
		panic("unsupported depth func")
	}
	b.glstate.setDepthFunc(b.funcs, glfunc)
}

func (b *Backend) NewInputLayout(vs driver.ShaderSources, layout []driver.InputDesc) (driver.InputLayout, error) {
	if len(vs.Inputs) != len(layout) {
		return nil, fmt.Errorf("NewInputLayout: got %d inputs, expected %d", len(layout), len(vs.Inputs))
	}
	for i, inp := range vs.Inputs {
		if exp, got := inp.Size, layout[i].Size; exp != got {
			return nil, fmt.Errorf("NewInputLayout: data size mismatch for %q: got %d expected %d", inp.Name, got, exp)
		}
	}
	return &gpuInputLayout{
		inputs: vs.Inputs,
		layout: layout,
	}, nil
}

func (b *Backend) NewComputeProgram(src driver.ShaderSources) (driver.Program, error) {
	p, err := gl.CreateComputeProgram(b.funcs, src.GLSL310ES)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", src.Name, err)
	}
	gpuProg := &gpuProgram{
		backend: b,
		obj:     p,
	}
	return gpuProg, nil
}

func (b *Backend) NewProgram(vertShader, fragShader driver.ShaderSources) (driver.Program, error) {
	attr := make([]string, len(vertShader.Inputs))
	for _, inp := range vertShader.Inputs {
		attr[inp.Location] = inp.Name
	}
	vsrc, fsrc := vertShader.GLSL100ES, fragShader.GLSL100ES
	if b.glver[0] >= 3 {
		// OpenGL (ES) 3.0.
		switch {
		case b.gles:
			vsrc, fsrc = vertShader.GLSL300ES, fragShader.GLSL300ES
		case b.glver[0] >= 4 || b.glver[1] >= 2:
			// OpenGL 3.2 Core only accepts glsl 1.50 or newer.
			vsrc, fsrc = vertShader.GLSL150, fragShader.GLSL150
		default:
			vsrc, fsrc = vertShader.GLSL130, fragShader.GLSL130
		}
	}
	p, err := gl.CreateProgram(b.funcs, vsrc, fsrc, attr)
	if err != nil {
		return nil, err
	}
	gpuProg := &gpuProgram{
		backend: b,
		obj:     p,
	}
	b.BindProgram(gpuProg)
	// Bind texture uniforms.
	for _, tex := range vertShader.Textures {
		u := b.funcs.GetUniformLocation(p, tex.Name)
		if u.Valid() {
			b.funcs.Uniform1i(u, tex.Binding)
		}
	}
	for _, tex := range fragShader.Textures {
		u := b.funcs.GetUniformLocation(p, tex.Name)
		if u.Valid() {
			b.funcs.Uniform1i(u, tex.Binding)
		}
	}
	if b.ubo {
		for _, block := range vertShader.Uniforms.Blocks {
			blockIdx := b.funcs.GetUniformBlockIndex(p, block.Name)
			if blockIdx != gl.INVALID_INDEX {
				b.funcs.UniformBlockBinding(p, blockIdx, uint(block.Binding))
			}
		}
		// To match Direct3D 11 with separate vertex and fragment
		// shader uniform buffers, offset all fragment blocks to be
		// located after the vertex blocks.
		off := len(vertShader.Uniforms.Blocks)
		for _, block := range fragShader.Uniforms.Blocks {
			blockIdx := b.funcs.GetUniformBlockIndex(p, block.Name)
			if blockIdx != gl.INVALID_INDEX {
				b.funcs.UniformBlockBinding(p, blockIdx, uint(block.Binding+off))
			}
		}
	} else {
		gpuProg.vertUniforms.setup(b.funcs, p, vertShader.Uniforms.Size, vertShader.Uniforms.Locations)
		gpuProg.fragUniforms.setup(b.funcs, p, fragShader.Uniforms.Size, fragShader.Uniforms.Locations)
	}
	return gpuProg, nil
}

func lookupUniform(funcs *gl.Functions, p gl.Program, loc driver.UniformLocation) uniformLocation {
	u := funcs.GetUniformLocation(p, loc.Name)
	if !u.Valid() {
		panic(fmt.Errorf("uniform %q not found", loc.Name))
	}
	return uniformLocation{uniform: u, offset: loc.Offset, typ: loc.Type, size: loc.Size}
}

func (p *gpuProgram) SetStorageBuffer(binding int, buffer driver.Buffer) {
	buf := buffer.(*gpuBuffer)
	if buf.typ&driver.BufferBindingShaderStorage == 0 {
		panic("not a shader storage buffer")
	}
	p.storage[binding] = buf
}

func (p *gpuProgram) SetVertexUniforms(buffer driver.Buffer) {
	p.vertUniforms.setBuffer(buffer)
}

func (p *gpuProgram) SetFragmentUniforms(buffer driver.Buffer) {
	p.fragUniforms.setBuffer(buffer)
}

func (p *gpuProgram) updateUniforms() {
	f := p.backend.funcs
	if p.backend.ubo {
		if b := p.vertUniforms.buf; b != nil {
			p.backend.glstate.bindBufferBase(f, gl.UNIFORM_BUFFER, 0, b.obj)
		}
		if b := p.fragUniforms.buf; b != nil {
			p.backend.glstate.bindBufferBase(f, gl.UNIFORM_BUFFER, 1, b.obj)
		}
	} else {
		p.vertUniforms.update(f)
		p.fragUniforms.update(f)
	}
}

func (b *Backend) BindProgram(prog driver.Program) {
	p := prog.(*gpuProgram)
	b.useProgram(p)
}

func (p *gpuProgram) Release() {
	p.backend.glstate.deleteProgram(p.backend.funcs, p.obj)
}

func (u *uniformsTracker) setup(funcs *gl.Functions, p gl.Program, uniformSize int, uniforms []driver.UniformLocation) {
	u.locs = make([]uniformLocation, len(uniforms))
	for i, uniform := range uniforms {
		u.locs[i] = lookupUniform(funcs, p, uniform)
	}
	u.size = uniformSize
}

func (u *uniformsTracker) setBuffer(buffer driver.Buffer) {
	buf := buffer.(*gpuBuffer)
	if buf.typ&driver.BufferBindingUniforms == 0 {
		panic("not a uniform buffer")
	}
	if buf.size < u.size {
		panic(fmt.Errorf("uniform buffer too small, got %d need %d", buf.size, u.size))
	}
	u.buf = buf
	// Force update.
	u.version = buf.version - 1
}

func (p *uniformsTracker) update(funcs *gl.Functions) {
	b := p.buf
	if b == nil || b.version == p.version {
		return
	}
	p.version = b.version
	data := b.data
	for _, u := range p.locs {
		data := data[u.offset:]
		switch {
		case u.typ == driver.DataTypeFloat && u.size == 1:
			data := data[:4]
			v := *(*[1]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform1f(u.uniform, v[0])
		case u.typ == driver.DataTypeFloat && u.size == 2:
			data := data[:8]
			v := *(*[2]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform2f(u.uniform, v[0], v[1])
		case u.typ == driver.DataTypeFloat && u.size == 3:
			data := data[:12]
			v := *(*[3]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform3f(u.uniform, v[0], v[1], v[2])
		case u.typ == driver.DataTypeFloat && u.size == 4:
			data := data[:16]
			v := *(*[4]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform4f(u.uniform, v[0], v[1], v[2], v[3])
		default:
			panic("unsupported uniform data type or size")
		}
	}
}

func (b *gpuBuffer) Upload(data []byte) {
	if b.immutable {
		panic("immutable buffer")
	}
	if len(data) > b.size {
		panic("buffer size overflow")
	}
	b.version++
	copy(b.data, data)
	if b.hasBuffer {
		firstBinding := firstBufferType(b.typ)
		b.backend.glstate.bindBuffer(b.backend.funcs, firstBinding, b.obj)
		if len(data) == b.size {
			// the iOS GL implementation doesn't recognize when BufferSubData
			// clears the entire buffer. Tell it and avoid GPU stalls.
			// See also https://github.com/godotengine/godot/issues/23956.
			b.backend.funcs.BufferData(firstBinding, b.size, gl.DYNAMIC_DRAW)
		}
		b.backend.funcs.BufferSubData(firstBinding, 0, data)
	}
}

func (b *gpuBuffer) Download(data []byte) error {
	if len(data) > b.size {
		panic("buffer size overflow")
	}
	if !b.hasBuffer {
		copy(data, b.data)
		return nil
	}
	firstBinding := firstBufferType(b.typ)
	b.backend.glstate.bindBuffer(b.backend.funcs, firstBinding, b.obj)
	bufferMap := b.backend.funcs.MapBufferRange(firstBinding, 0, len(data), gl.MAP_READ_BIT)
	if bufferMap == nil {
		return fmt.Errorf("MapBufferRange: error %#x", b.backend.funcs.GetError())
	}
	copy(data, bufferMap)
	if !b.backend.funcs.UnmapBuffer(firstBinding) {
		return driver.ErrContentLost
	}
	return nil
}

func (b *gpuBuffer) Release() {
	if b.hasBuffer {
		b.backend.glstate.deleteBuffer(b.backend.funcs, b.obj)
		b.hasBuffer = false
	}
}

func (b *Backend) BindVertexBuffer(buf driver.Buffer, stride, offset int) {
	gbuf := buf.(*gpuBuffer)
	if gbuf.typ&driver.BufferBindingVertices == 0 {
		panic("not a vertex buffer")
	}
	b.state.buffer = bufferBinding{obj: gbuf.obj, stride: stride, offset: offset}
}

func (b *Backend) setupVertexArrays() {
	layout := b.state.layout
	if layout == nil {
		return
	}
	const max = len(b.glstate.vertAttribs)
	var enabled [max]bool
	buf := b.state.buffer
	for i, inp := range layout.inputs {
		l := layout.layout[i]
		var gltyp gl.Enum
		switch l.Type {
		case driver.DataTypeFloat:
			gltyp = gl.FLOAT
		case driver.DataTypeShort:
			gltyp = gl.SHORT
		default:
			panic("unsupported data type")
		}
		enabled[inp.Location] = true
		b.glstate.vertexAttribPointer(b.funcs, buf.obj, inp.Location, l.Size, gltyp, false, buf.stride, buf.offset+l.Offset)
	}
	for i := 0; i < max; i++ {
		b.glstate.setVertexAttribArray(b.funcs, i, enabled[i])
	}
}

func (b *Backend) BindIndexBuffer(buf driver.Buffer) {
	gbuf := buf.(*gpuBuffer)
	if gbuf.typ&driver.BufferBindingIndices == 0 {
		panic("not an index buffer")
	}
	b.glstate.bindBuffer(b.funcs, gl.ELEMENT_ARRAY_BUFFER, gbuf.obj)
}

func (b *Backend) BlitFramebuffer(dst, src driver.Framebuffer, srect, drect image.Rectangle) {
	b.glstate.bindFramebuffer(b.funcs, gl.DRAW_FRAMEBUFFER, dst.(*gpuFramebuffer).obj)
	b.glstate.bindFramebuffer(b.funcs, gl.READ_FRAMEBUFFER, src.(*gpuFramebuffer).obj)
	b.funcs.BlitFramebuffer(
		srect.Min.X, srect.Min.Y, srect.Max.X, srect.Max.Y,
		drect.Min.X, drect.Min.Y, drect.Max.X, drect.Max.Y,
		gl.COLOR_BUFFER_BIT|gl.DEPTH_BUFFER_BIT|gl.STENCIL_BUFFER_BIT,
		gl.NEAREST)
}

func (f *gpuFramebuffer) ReadPixels(src image.Rectangle, pixels []byte) error {
	glErr(f.backend.funcs)
	f.backend.BindFramebuffer(f)
	if len(pixels) < src.Dx()*src.Dy()*4 {
		return errors.New("unexpected RGBA size")
	}
	f.backend.funcs.ReadPixels(src.Min.X, src.Min.Y, src.Dx(), src.Dy(), gl.RGBA, gl.UNSIGNED_BYTE, pixels)
	return glErr(f.backend.funcs)
}

func (b *Backend) BindFramebuffer(fbo driver.Framebuffer) {
	b.glstate.bindFramebuffer(b.funcs, gl.FRAMEBUFFER, fbo.(*gpuFramebuffer).obj)
}

func (f *gpuFramebuffer) Invalidate() {
	f.backend.BindFramebuffer(f)
	f.backend.funcs.InvalidateFramebuffer(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0)
}

func (f *gpuFramebuffer) Release() {
	if f.foreign {
		panic("framebuffer not created by NewFramebuffer")
	}
	f.backend.glstate.deleteFramebuffer(f.backend.funcs, f.obj)
	if f.hasDepth {
		f.backend.glstate.deleteRenderbuffer(f.backend.funcs, f.depthBuf)
	}
}

func toTexFilter(f driver.TextureFilter) int {
	switch f {
	case driver.FilterNearest:
		return gl.NEAREST
	case driver.FilterLinear:
		return gl.LINEAR
	default:
		panic("unsupported texture filter")
	}
}

func (b *Backend) BindTexture(unit int, t driver.Texture) {
	b.glstate.bindTexture(b.funcs, unit, t.(*gpuTexture).obj)
}

func (t *gpuTexture) Release() {
	t.backend.glstate.deleteTexture(t.backend.funcs, t.obj)
}

func (t *gpuTexture) Upload(offset, size image.Point, pixels []byte) {
	if min := size.X * size.Y * 4; min > len(pixels) {
		panic(fmt.Errorf("size %d larger than data %d", min, len(pixels)))
	}
	t.backend.BindTexture(0, t)
	t.backend.funcs.TexSubImage2D(gl.TEXTURE_2D, 0, offset.X, offset.Y, size.X, size.Y, t.triple.format, t.triple.typ, pixels)
}

func (t *gpuTimer) Begin() {
	t.funcs.BeginQuery(gl.TIME_ELAPSED_EXT, t.obj)
}

func (t *gpuTimer) End() {
	t.funcs.EndQuery(gl.TIME_ELAPSED_EXT)
}

func (t *gpuTimer) ready() bool {
	return t.funcs.GetQueryObjectuiv(t.obj, gl.QUERY_RESULT_AVAILABLE) == gl.TRUE
}

func (t *gpuTimer) Release() {
	t.funcs.DeleteQuery(t.obj)
}

func (t *gpuTimer) Duration() (time.Duration, bool) {
	if !t.ready() {
		return 0, false
	}
	nanos := t.funcs.GetQueryObjectuiv(t.obj, gl.QUERY_RESULT)
	return time.Duration(nanos), true
}

func (b *Backend) BindInputLayout(l driver.InputLayout) {
	b.state.layout = l.(*gpuInputLayout)
}

func (l *gpuInputLayout) Release() {}

// floatTripleFor determines the best texture triple for floating point FBOs.
func floatTripleFor(f *gl.Functions, ver [2]int, exts []string) (textureTriple, error) {
	var triples []textureTriple
	if ver[0] >= 3 {
		triples = append(triples, textureTriple{gl.R16F, gl.Enum(gl.RED), gl.Enum(gl.HALF_FLOAT)})
	}
	// According to the OES_texture_half_float specification, EXT_color_buffer_half_float is needed to
	// render to FBOs. However, the Safari WebGL1 implementation does support half-float FBOs but does not
	// report EXT_color_buffer_half_float support. The triples are verified below, so it doesn't matter if we're
	// wrong.
	if hasExtension(exts, "GL_OES_texture_half_float") || hasExtension(exts, "GL_EXT_color_buffer_half_float") {
		// Try single channel.
		triples = append(triples, textureTriple{gl.LUMINANCE, gl.Enum(gl.LUMINANCE), gl.Enum(gl.HALF_FLOAT_OES)})
		// Fallback to 4 channels.
		triples = append(triples, textureTriple{gl.RGBA, gl.Enum(gl.RGBA), gl.Enum(gl.HALF_FLOAT_OES)})
	}
	if hasExtension(exts, "GL_OES_texture_float") || hasExtension(exts, "GL_EXT_color_buffer_float") {
		triples = append(triples, textureTriple{gl.RGBA, gl.Enum(gl.RGBA), gl.Enum(gl.FLOAT)})
	}
	tex := f.CreateTexture()
	defer f.DeleteTexture(tex)
	defTex := gl.Texture(f.GetBinding(gl.TEXTURE_BINDING_2D))
	defer f.BindTexture(gl.TEXTURE_2D, defTex)
	f.BindTexture(gl.TEXTURE_2D, tex)
	f.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	f.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	f.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	f.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	fbo := f.CreateFramebuffer()
	defer f.DeleteFramebuffer(fbo)
	defFBO := gl.Framebuffer(f.GetBinding(gl.FRAMEBUFFER_BINDING))
	f.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	defer f.BindFramebuffer(gl.FRAMEBUFFER, defFBO)
	var attempts []string
	for _, tt := range triples {
		const size = 256
		f.TexImage2D(gl.TEXTURE_2D, 0, tt.internalFormat, size, size, tt.format, tt.typ)
		f.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
		st := f.CheckFramebufferStatus(gl.FRAMEBUFFER)
		if st == gl.FRAMEBUFFER_COMPLETE {
			return tt, nil
		}
		attempts = append(attempts, fmt.Sprintf("(0x%x, 0x%x, 0x%x): 0x%x", tt.internalFormat, tt.format, tt.typ, st))
	}
	return textureTriple{}, fmt.Errorf("floating point fbos not supported (attempted %s)", attempts)
}

func srgbaTripleFor(ver [2]int, exts []string) (textureTriple, error) {
	switch {
	case ver[0] >= 3:
		return textureTriple{gl.SRGB8_ALPHA8, gl.Enum(gl.RGBA), gl.Enum(gl.UNSIGNED_BYTE)}, nil
	case hasExtension(exts, "GL_EXT_sRGB"):
		return textureTriple{gl.SRGB_ALPHA_EXT, gl.Enum(gl.SRGB_ALPHA_EXT), gl.Enum(gl.UNSIGNED_BYTE)}, nil
	default:
		return textureTriple{}, errors.New("no sRGB texture formats found")
	}
}

func alphaTripleFor(ver [2]int) textureTriple {
	intf, f := gl.Enum(gl.R8), gl.Enum(gl.RED)
	if ver[0] < 3 {
		// R8, RED not supported on OpenGL ES 2.0.
		intf, f = gl.LUMINANCE, gl.Enum(gl.LUMINANCE)
	}
	return textureTriple{intf, f, gl.UNSIGNED_BYTE}
}

func hasExtension(exts []string, ext string) bool {
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func firstBufferType(typ driver.BufferBinding) gl.Enum {
	switch {
	case typ&driver.BufferBindingIndices != 0:
		return gl.ELEMENT_ARRAY_BUFFER
	case typ&driver.BufferBindingVertices != 0:
		return gl.ARRAY_BUFFER
	case typ&driver.BufferBindingUniforms != 0:
		return gl.UNIFORM_BUFFER
	case typ&driver.BufferBindingShaderStorage != 0:
		return gl.SHADER_STORAGE_BUFFER
	default:
		panic("unsupported buffer type")
	}
}
