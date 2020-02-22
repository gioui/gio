// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"time"
	"unsafe"

	"gioui.org/gpu"
)

// Backend implements gpu.Backend.
type Backend struct {
	funcs  Functions
	defFBO *gpuFramebuffer

	state glstate

	feats gpu.Caps
	// floatTriple holds the settings for floating point
	// textures.
	floatTriple textureTriple
	// Single channel alpha textures.
	alphaTriple textureTriple
	srgbaTriple textureTriple
}

// State tracking.
type glstate struct {
	// nattr is the current number of enabled vertex arrays.
	nattr    int
	prog     *gpuProgram
	texUnits [2]*gpuTexture
	layout   *gpuInputLayout
	buffer   bufferBinding
}

type bufferBinding struct {
	buf    *gpuBuffer
	offset int
	stride int
}

type gpuTimer struct {
	funcs Functions
	obj   Query
}

type gpuTexture struct {
	backend *Backend
	obj     Texture
	triple  textureTriple
	width   int
	height  int
}

type gpuFramebuffer struct {
	backend *Backend
	obj      Framebuffer
	hasDepth bool
	depthBuf Renderbuffer
}

type gpuBuffer struct {
	backend   *Backend
	obj       Buffer
	typ       gpu.BufferBinding
	size      int
	immutable bool
	version   int
	// For emulation of uniform buffers.
	data []byte
}

type gpuProgram struct {
	backend      *Backend
	obj          Program
	nattr        int
	vertUniforms uniformsTracker
	fragUniforms uniformsTracker
}

type uniformsTracker struct {
	locs    []uniformLocation
	size    int
	buf     *gpuBuffer
	version int
}

type uniformLocation struct {
	uniform Uniform
	offset  int
	typ     gpu.DataType
	size    int
}

type gpuInputLayout struct {
	backend *Backend
	inputs  []gpu.InputLocation
	layout  []gpu.InputDesc
}

// textureTriple holds the type settings for
// a TexImage2D call.
type textureTriple struct {
	internalFormat int
	format         Enum
	typ            Enum
}

func NewBackend(f Functions) (*Backend, error) {
	exts := strings.Split(f.GetString(EXTENSIONS), " ")
	glVer := f.GetString(VERSION)
	ver, err := ParseGLVersion(glVer)
	if err != nil {
		return nil, err
	}
	floatTriple, err := floatTripleFor(f, ver, exts)
	if err != nil {
		return nil, err
	}
	srgbaTriple, err := srgbaTripleFor(ver, exts)
	if err != nil {
		return nil, err
	}
	defFBO := Framebuffer(f.GetBinding(FRAMEBUFFER_BINDING))
	b := &Backend{
		funcs:       f,
		floatTriple: floatTriple,
		alphaTriple: alphaTripleFor(ver),
		srgbaTriple: srgbaTriple,
	}
	b.defFBO = &gpuFramebuffer{backend: b, obj: defFBO}
	if hasExtension(exts, "GL_EXT_disjoint_timer_query_webgl2") || hasExtension(exts, "GL_EXT_disjoint_timer_query") {
		b.feats.Features |= gpu.FeatureTimers
	}
	b.feats.MaxTextureSize = f.GetInteger(MAX_TEXTURE_SIZE)
	return b, nil
}

func (b *Backend) BeginFrame() {
	// Assume GL state is reset between frames.
	b.state = glstate{}
}

func (b *Backend) EndFrame() {
	b.funcs.ActiveTexture(TEXTURE0)
}

func (b *Backend) Caps() gpu.Caps {
	return b.feats
}

func (b *Backend) NewTimer() gpu.Timer {
	return &gpuTimer{
		funcs: b.funcs,
		obj:   b.funcs.CreateQuery(),
	}
}

func (b *Backend) IsTimeContinuous() bool {
	return b.funcs.GetInteger(GPU_DISJOINT_EXT) == FALSE
}

func (b *Backend) NewFramebuffer(tex gpu.Texture, depthBits int) (gpu.Framebuffer, error) {
	glErr(b.funcs)
	gltex := tex.(*gpuTexture)
	fb := b.funcs.CreateFramebuffer()
	fbo := &gpuFramebuffer{backend: b, obj: fb}
	b.BindFramebuffer(fbo)
	if err := glErr(b.funcs); err != nil {
		fbo.Release()
		return nil, err
	}
	b.funcs.FramebufferTexture2D(FRAMEBUFFER, COLOR_ATTACHMENT0, TEXTURE_2D, gltex.obj, 0)
	if depthBits > 0 {
		size := Enum(DEPTH_COMPONENT16)
		switch {
		case depthBits > 24:
			size = DEPTH_COMPONENT32F
		case depthBits > 16:
			size = DEPTH_COMPONENT24
		}
		depthBuf := b.funcs.CreateRenderbuffer()
		b.funcs.BindRenderbuffer(RENDERBUFFER, depthBuf)
		b.funcs.RenderbufferStorage(RENDERBUFFER, size, gltex.width, gltex.height)
		fbo.depthBuf = depthBuf
		fbo.hasDepth = true
		if err := glErr(b.funcs); err != nil {
			fbo.Release()
			return nil, err
		}
	}
	if st := b.funcs.CheckFramebufferStatus(FRAMEBUFFER); st != FRAMEBUFFER_COMPLETE {
		fbo.Release()
		return nil, fmt.Errorf("incomplete framebuffer, status = 0x%x, err = %d", st, b.funcs.GetError())
	}
	return fbo, nil
}

func (b *Backend) DefaultFramebuffer() gpu.Framebuffer {
	return b.defFBO
}

func (b *Backend) NewTexture(format gpu.TextureFormat, width, height int, minFilter, magFilter gpu.TextureFilter, binding gpu.BufferBinding) (gpu.Texture, error) {
	glErr(b.funcs)
	tex := &gpuTexture{backend: b, obj: b.funcs.CreateTexture(), width: width, height: height}
	switch format {
	case gpu.TextureFormatFloat:
		tex.triple = b.floatTriple
	case gpu.TextureFormatSRGB:
		tex.triple = b.srgbaTriple
	default:
		return nil, errors.New("unsupported texture format")
	}
	b.BindTexture(0, tex)
	b.funcs.TexParameteri(TEXTURE_2D, TEXTURE_MAG_FILTER, toTexFilter(magFilter))
	b.funcs.TexParameteri(TEXTURE_2D, TEXTURE_MIN_FILTER, toTexFilter(minFilter))
	b.funcs.TexParameteri(TEXTURE_2D, TEXTURE_WRAP_S, CLAMP_TO_EDGE)
	b.funcs.TexParameteri(TEXTURE_2D, TEXTURE_WRAP_T, CLAMP_TO_EDGE)
	b.funcs.TexImage2D(TEXTURE_2D, 0, tex.triple.internalFormat, width, height, tex.triple.format, tex.triple.typ, nil)
	if err := glErr(b.funcs); err != nil {
		tex.Release()
		return nil, err
	}
	return tex, nil
}

func (b *Backend) NewBuffer(typ gpu.BufferBinding, size int) (gpu.Buffer, error) {
	glErr(b.funcs)
	buf := &gpuBuffer{backend: b, typ: typ, size: size}
	if typ&gpu.BufferBindingUniforms != 0 {
		if typ != gpu.BufferBindingUniforms {
			return nil, errors.New("uniforms buffers cannot be bound as anything else")
		}
		// GLES 2 doesn't support uniform buffers.
		buf.data = make([]byte, size)
	}
	if typ&^gpu.BufferBindingUniforms != 0 {
		buf.obj = b.funcs.CreateBuffer()
		if err := glErr(b.funcs); err != nil {
			buf.Release()
			return nil, err
		}
	}
	return buf, nil
}

func (b *Backend) NewImmutableBuffer(typ gpu.BufferBinding, data []byte) (gpu.Buffer, error) {
	glErr(b.funcs)
	obj := b.funcs.CreateBuffer()
	buf := &gpuBuffer{backend: b, obj: obj, typ: typ, size: len(data)}
	buf.Upload(data)
	buf.immutable = true
	if err := glErr(b.funcs); err != nil {
		buf.Release()
		return nil, err
	}
	return buf, nil
}

func glErr(f Functions) error {
	if st := f.GetError(); st != NO_ERROR {
		return fmt.Errorf("glGetError: %#x", st)
	}
	return nil
}

func (b *Backend) bindTexture(unit int, t *gpuTexture) {
	if b.state.texUnits[unit] != t {
		b.funcs.ActiveTexture(TEXTURE0 + Enum(unit))
		b.funcs.BindTexture(TEXTURE_2D, t.obj)
		b.state.texUnits[unit] = t
	}
}

func (b *Backend) useProgram(p *gpuProgram) {
	if b.state.prog != p {
		p.backend.funcs.UseProgram(p.obj)
		b.state.prog = p
	}
}

func (b *Backend) enableVertexArrays(n int) {
	// Enable needed arrays.
	for i := b.state.nattr; i < n; i++ {
		b.funcs.EnableVertexAttribArray(Attrib(i))
	}
	// Disable extra arrays.
	for i := n; i < b.state.nattr; i++ {
		b.funcs.DisableVertexAttribArray(Attrib(i))
	}
	b.state.nattr = n
}

func (b *Backend) SetDepthTest(enable bool) {
	if enable {
		b.funcs.Enable(DEPTH_TEST)
	} else {
		b.funcs.Disable(DEPTH_TEST)
	}
}

func (b *Backend) BlendFunc(sfactor, dfactor gpu.BlendFactor) {
	b.funcs.BlendFunc(toGLBlendFactor(sfactor), toGLBlendFactor(dfactor))
}

func toGLBlendFactor(f gpu.BlendFactor) Enum {
	switch f {
	case gpu.BlendFactorOne:
		return ONE
	case gpu.BlendFactorOneMinusSrcAlpha:
		return ONE_MINUS_SRC_ALPHA
	case gpu.BlendFactorZero:
		return ZERO
	case gpu.BlendFactorDstColor:
		return DST_COLOR
	default:
		panic("unsupported blend factor")
	}
}

func (b *Backend) DepthMask(mask bool) {
	b.funcs.DepthMask(mask)
}

func (b *Backend) SetBlend(enable bool) {
	if enable {
		b.funcs.Enable(BLEND)
	} else {
		b.funcs.Disable(BLEND)
	}
}

func (b *Backend) DrawElements(mode gpu.DrawMode, off, count int) {
	b.prepareDraw()
	// off is in 16-bit indices, but DrawElements take a byte offset.
	byteOff := off * 2
	b.funcs.DrawElements(toGLDrawMode(mode), count, UNSIGNED_SHORT, byteOff)
}

func (b *Backend) DrawArrays(mode gpu.DrawMode, off, count int) {
	b.prepareDraw()
	b.funcs.DrawArrays(toGLDrawMode(mode), off, count)
}

func (b *Backend) prepareDraw() {
	b.setupVertexArrays()
	if p := b.state.prog; p != nil {
		p.updateUniforms()
	}
}

func toGLDrawMode(mode gpu.DrawMode) Enum {
	switch mode {
	case gpu.DrawModeTriangleStrip:
		return TRIANGLE_STRIP
	case gpu.DrawModeTriangles:
		return TRIANGLES
	default:
		panic("unsupported draw mode")
	}
}

func (b *Backend) Viewport(x, y, width, height int) {
	b.funcs.Viewport(x, y, width, height)
}

func (b *Backend) Clear(attachments gpu.BufferAttachments) {
	var mask Enum
	if attachments&gpu.BufferAttachmentColor != 0 {
		mask |= COLOR_BUFFER_BIT
	}
	if attachments&gpu.BufferAttachmentDepth != 0 {
		mask |= DEPTH_BUFFER_BIT
	}
	b.funcs.Clear(mask)
}

func (b *Backend) ClearDepth(d float32) {
	b.funcs.ClearDepthf(d)
}

func (b *Backend) ClearColor(colR, colG, colB, colA float32) {
	b.funcs.ClearColor(colR, colG, colB, colA)
}

func (b *Backend) DepthFunc(f gpu.DepthFunc) {
	var glfunc Enum
	switch f {
	case gpu.DepthFuncGreater:
		glfunc = GREATER
	default:
		panic("unsupported depth func")
	}
	b.funcs.DepthFunc(glfunc)
}

func (b *Backend) NewInputLayout(vs gpu.ShaderSources, layout []gpu.InputDesc) (gpu.InputLayout, error) {
	if len(vs.Inputs) != len(layout) {
		return nil, fmt.Errorf("NewInputLayout: got %d inputs, expected %d", len(layout), len(vs.Inputs))
	}
	for i, inp := range vs.Inputs {
		if exp, got := inp.Size, layout[i].Size; exp != got {
			return nil, fmt.Errorf("NewInputLayout: data size mismatch for %q: got %d expected %d", inp.Name, got, exp)
		}
	}
	return &gpuInputLayout{
		backend: b,
		inputs:  vs.Inputs,
		layout:  layout,
	}, nil
}

func (b *Backend) NewProgram(vssrc, fssrc gpu.ShaderSources) (gpu.Program, error) {
	attr := make([]string, len(vssrc.Inputs))
	for _, inp := range vssrc.Inputs {
		attr[inp.Location] = inp.Name
	}
	p, err := CreateProgram(b.funcs, vssrc.GLES2, fssrc.GLES2, attr)
	if err != nil {
		return nil, err
	}
	gpuProg := &gpuProgram{
		backend: b,
		obj:     p,
		nattr:   len(attr),
	}
	b.BindProgram(gpuProg)
	// Bind texture uniforms.
	for _, tex := range vssrc.Textures {
		u := b.funcs.GetUniformLocation(p, tex.Name)
		if u.valid() {
			b.funcs.Uniform1i(u, tex.Binding)
		}
	}
	for _, tex := range fssrc.Textures {
		u := b.funcs.GetUniformLocation(p, tex.Name)
		if u.valid() {
			b.funcs.Uniform1i(u, tex.Binding)
		}
	}
	gpuProg.vertUniforms.setup(b.funcs, p, vssrc.UniformSize, vssrc.Uniforms)
	gpuProg.fragUniforms.setup(b.funcs, p, fssrc.UniformSize, fssrc.Uniforms)
	return gpuProg, nil
}

func lookupUniform(funcs Functions, p Program, loc gpu.UniformLocation) uniformLocation {
	u := GetUniformLocation(funcs, p, loc.Name)
	return uniformLocation{uniform: u, offset: loc.Offset, typ: loc.Type, size: loc.Size}
}

func (p *gpuProgram) SetVertexUniforms(buffer gpu.Buffer) {
	p.vertUniforms.setBuffer(buffer)
}

func (p *gpuProgram) SetFragmentUniforms(buffer gpu.Buffer) {
	p.fragUniforms.setBuffer(buffer)
}

func (p *gpuProgram) updateUniforms() {
	p.vertUniforms.update(p.backend.funcs)
	p.fragUniforms.update(p.backend.funcs)
}

func (b *Backend) BindProgram(prog gpu.Program) {
	p := prog.(*gpuProgram)
	b.useProgram(p)
	b.enableVertexArrays(p.nattr)
}

func (p *gpuProgram) Release() {
	p.backend.funcs.DeleteProgram(p.obj)
}

func (u *uniformsTracker) setup(funcs Functions, p Program, uniformSize int, uniforms []gpu.UniformLocation) {
	u.locs = make([]uniformLocation, len(uniforms))
	for i, uniform := range uniforms {
		u.locs[i] = lookupUniform(funcs, p, uniform)
	}
	u.size = uniformSize
}

func (u *uniformsTracker) setBuffer(buffer gpu.Buffer) {
	buf := buffer.(*gpuBuffer)
	if buf.typ&gpu.BufferBindingUniforms == 0 {
		panic("not a uniform buffer")
	}
	if buf.size < u.size {
		panic(fmt.Errorf("uniform buffer too small, got %d need %d", buf.size, u.size))
	}
	u.buf = buf
	// Force update.
	u.version = buf.version - 1
}

func (p *uniformsTracker) update(funcs Functions) {
	b := p.buf
	if b == nil || b.version == p.version {
		return
	}
	p.version = b.version
	data := b.data
	for _, u := range p.locs {
		data := data[u.offset:]
		switch {
		case u.typ == gpu.DataTypeFloat && u.size == 1:
			data := data[:4]
			v := *(*[1]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform1f(u.uniform, v[0])
		case u.typ == gpu.DataTypeFloat && u.size == 2:
			data := data[:8]
			v := *(*[2]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform2f(u.uniform, v[0], v[1])
		case u.typ == gpu.DataTypeFloat && u.size == 3:
			data := data[:12]
			v := *(*[3]float32)(unsafe.Pointer(&data[0]))
			funcs.Uniform3f(u.uniform, v[0], v[1], v[2])
		case u.typ == gpu.DataTypeFloat && u.size == 4:
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
	if b.typ&gpu.BufferBindingUniforms != 0 {
		copy(b.data, data)
	}
	if b.typ&^gpu.BufferBindingUniforms != 0 {
		firstBinding := firstBufferType(b.typ)
		b.backend.funcs.BindBuffer(firstBinding, b.obj)
		b.backend.funcs.BufferData(firstBinding, data, STATIC_DRAW)
	}
}

func (b *gpuBuffer) Release() {
	if b.typ&^gpu.BufferBindingUniforms != 0 {
		b.backend.funcs.DeleteBuffer(b.obj)
	}
}

func (b *Backend) BindVertexBuffer(buf gpu.Buffer, stride, offset int) {
	gbuf := buf.(*gpuBuffer)
	if gbuf.typ&gpu.BufferBindingVertices == 0 {
		panic("not a vertex buffer")
	}
	b.state.buffer = bufferBinding{buf: gbuf, stride: stride, offset: offset}
}

func (b *Backend) setupVertexArrays() {
	layout := b.state.layout
	if layout == nil {
		panic("no input layout is current")
	}
	buf := b.state.buffer
	b.funcs.BindBuffer(ARRAY_BUFFER, buf.buf.obj)
	for i, inp := range layout.inputs {
		l := layout.layout[i]
		var gltyp Enum
		switch l.Type {
		case gpu.DataTypeFloat:
			gltyp = FLOAT
		case gpu.DataTypeShort:
			gltyp = SHORT
		default:
			panic("unsupported data type")
		}
		b.funcs.VertexAttribPointer(Attrib(inp.Location), l.Size, gltyp, false, buf.stride, buf.offset+l.Offset)
	}
}

func (b *Backend) BindIndexBuffer(buf gpu.Buffer) {
	gbuf := buf.(*gpuBuffer)
	if gbuf.typ&gpu.BufferBindingIndices == 0 {
		panic("not an index buffer")
	}
	b.funcs.BindBuffer(ELEMENT_ARRAY_BUFFER, gbuf.obj)
}

func (f *gpuFramebuffer) ReadPixels(src image.Rectangle, pixels []byte) error {
	glErr(f.backend.funcs)
	f.backend.BindFramebuffer(f)
	if len(pixels) < src.Dx()*src.Dy() {
		return errors.New("unexpected RGBA size")
	}
	f.backend.funcs.ReadPixels(src.Min.X, src.Min.Y, src.Dx(), src.Dy(), RGBA, UNSIGNED_BYTE, pixels)
	return glErr(f.backend.funcs)
}

func (b *Backend) BindFramebuffer(fbo gpu.Framebuffer) {
	b.funcs.BindFramebuffer(FRAMEBUFFER, fbo.(*gpuFramebuffer).obj)
}

func (f *gpuFramebuffer) Invalidate() {
	f.backend.BindFramebuffer(f)
	f.backend.funcs.InvalidateFramebuffer(FRAMEBUFFER, COLOR_ATTACHMENT0)
}

func (f *gpuFramebuffer) Release() {
	f.backend.funcs.DeleteFramebuffer(f.obj)
	if f.hasDepth {
		f.backend.funcs.DeleteRenderbuffer(f.depthBuf)
	}
}

func toTexFilter(f gpu.TextureFilter) int {
	switch f {
	case gpu.FilterNearest:
		return NEAREST
	case gpu.FilterLinear:
		return LINEAR
	default:
		panic("unsupported texture filter")
	}
}

func (b *Backend) BindTexture(unit int, t gpu.Texture) {
	b.bindTexture(unit, t.(*gpuTexture))
}

func (t *gpuTexture) Release() {
	t.backend.funcs.DeleteTexture(t.obj)
}

func (t *gpuTexture) Upload(img *image.RGBA) {
	t.backend.BindTexture(0, t)
	var pixels []byte
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if img.Stride != w*4 {
		panic("unsupported stride")
	}
	start := (b.Min.X + b.Min.Y*w) * 4
	end := (b.Max.X + (b.Max.Y-1)*w) * 4
	pixels = img.Pix[start:end]
	t.backend.funcs.TexImage2D(TEXTURE_2D, 0, t.triple.internalFormat, w, h, t.triple.format, t.triple.typ, pixels)
}

func (t *gpuTimer) Begin() {
	t.funcs.BeginQuery(TIME_ELAPSED_EXT, t.obj)
}

func (t *gpuTimer) End() {
	t.funcs.EndQuery(TIME_ELAPSED_EXT)
}

func (t *gpuTimer) ready() bool {
	return t.funcs.GetQueryObjectuiv(t.obj, QUERY_RESULT_AVAILABLE) == TRUE
}

func (t *gpuTimer) Release() {
	t.funcs.DeleteQuery(t.obj)
}

func (t *gpuTimer) Duration() (time.Duration, bool) {
	if !t.ready() {
		return 0, false
	}
	nanos := t.funcs.GetQueryObjectuiv(t.obj, QUERY_RESULT)
	return time.Duration(nanos), true
}

func (b *Backend) BindInputLayout(l gpu.InputLayout) {
	b.state.layout = l.(*gpuInputLayout)
}

func (l *gpuInputLayout) Release() {}

// floatTripleFor determines the best texture triple for floating point FBOs.
func floatTripleFor(f Functions, ver [2]int, exts []string) (textureTriple, error) {
	var triples []textureTriple
	if ver[0] >= 3 {
		triples = append(triples, textureTriple{R16F, Enum(RED), Enum(HALF_FLOAT)})
	}
	if hasExtension(exts, "GL_OES_texture_half_float") && hasExtension(exts, "GL_EXT_color_buffer_half_float") {
		// Try single channel.
		triples = append(triples, textureTriple{LUMINANCE, Enum(LUMINANCE), Enum(HALF_FLOAT_OES)})
		// Fallback to 4 channels.
		triples = append(triples, textureTriple{RGBA, Enum(RGBA), Enum(HALF_FLOAT_OES)})
	}
	if hasExtension(exts, "GL_OES_texture_float") || hasExtension(exts, "GL_EXT_color_buffer_float") {
		triples = append(triples, textureTriple{RGBA, Enum(RGBA), Enum(FLOAT)})
	}
	tex := f.CreateTexture()
	defer f.DeleteTexture(tex)
	f.BindTexture(TEXTURE_2D, tex)
	f.TexParameteri(TEXTURE_2D, TEXTURE_WRAP_S, CLAMP_TO_EDGE)
	f.TexParameteri(TEXTURE_2D, TEXTURE_WRAP_T, CLAMP_TO_EDGE)
	f.TexParameteri(TEXTURE_2D, TEXTURE_MAG_FILTER, NEAREST)
	f.TexParameteri(TEXTURE_2D, TEXTURE_MIN_FILTER, NEAREST)
	fbo := f.CreateFramebuffer()
	defer f.DeleteFramebuffer(fbo)
	defFBO := Framebuffer(f.GetBinding(FRAMEBUFFER_BINDING))
	f.BindFramebuffer(FRAMEBUFFER, fbo)
	defer f.BindFramebuffer(FRAMEBUFFER, defFBO)
	var attempts []string
	for _, tt := range triples {
		const size = 256
		f.TexImage2D(TEXTURE_2D, 0, tt.internalFormat, size, size, tt.format, tt.typ, nil)
		f.FramebufferTexture2D(FRAMEBUFFER, COLOR_ATTACHMENT0, TEXTURE_2D, tex, 0)
		st := f.CheckFramebufferStatus(FRAMEBUFFER)
		if st == FRAMEBUFFER_COMPLETE {
			return tt, nil
		}
		attempts = append(attempts, fmt.Sprintf("(0x%x, 0x%x, 0x%x): 0x%x", tt.internalFormat, tt.format, tt.typ, st))
	}
	return textureTriple{}, fmt.Errorf("floating point fbos not supported (attempted %s)", attempts)
}

func srgbaTripleFor(ver [2]int, exts []string) (textureTriple, error) {
	switch {
	case ver[0] >= 3:
		return textureTriple{SRGB8_ALPHA8, Enum(RGBA), Enum(UNSIGNED_BYTE)}, nil
	case hasExtension(exts, "GL_EXT_sRGB"):
		return textureTriple{SRGB_ALPHA_EXT, Enum(SRGB_ALPHA_EXT), Enum(UNSIGNED_BYTE)}, nil
	default:
		return textureTriple{}, errors.New("no sRGB texture formats found")
	}
}

func alphaTripleFor(ver [2]int) textureTriple {
	intf, f := R8, Enum(RED)
	if ver[0] < 3 {
		// R8, RED not supported on OpenGL ES 2.0.
		intf, f = LUMINANCE, Enum(LUMINANCE)
	}
	return textureTriple{intf, f, UNSIGNED_BYTE}
}

func hasExtension(exts []string, ext string) bool {
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}

func firstBufferType(typ gpu.BufferBinding) Enum {
	switch {
	case typ&gpu.BufferBindingIndices != 0:
		return ELEMENT_ARRAY_BUFFER
	case typ&gpu.BufferBindingVertices != 0:
		return ARRAY_BUFFER
	case typ&gpu.BufferBindingUniforms != 0:
		return UNIFORM_BUFFER
	default:
		panic("unsupported buffer type")
	}
}
