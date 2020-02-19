// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"errors"
	"fmt"
	"image"
	"strings"
	"time"

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
}

type gpuFramebuffer struct {
	funcs Functions
	obj   Framebuffer
}

type gpuBuffer struct {
	backend *Backend
	obj     Buffer
	typ     Enum
}

type gpuProgram struct {
	backend *Backend
	obj     Program
	nattr   int
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
		defFBO:      &gpuFramebuffer{funcs: f, obj: defFBO},
		funcs:       f,
		floatTriple: floatTriple,
		alphaTriple: alphaTripleFor(ver),
		srgbaTriple: srgbaTriple,
	}
	if hasExtension(exts, "GL_EXT_disjoint_timer_query_webgl2") || hasExtension(exts, "GL_EXT_disjoint_timer_query") {
		b.feats.Features |= gpu.FeatureTimers
	}
	b.feats.MaxTextureSize = f.GetInteger(MAX_TEXTURE_SIZE)
	return b, nil
}

func (b *Backend) BeginFrame() {
	// Assume GL state is reset.
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

func (b *Backend) NewFramebuffer() gpu.Framebuffer {
	fb := b.funcs.CreateFramebuffer()
	return &gpuFramebuffer{funcs: b.funcs, obj: fb}
}

func (b *Backend) NilTexture() gpu.Texture {
	return &gpuTexture{backend: b}
}

func (b *Backend) DefaultFramebuffer() gpu.Framebuffer {
	return b.defFBO
}

func (b *Backend) NewTexture(minFilter, magFilter gpu.TextureFilter) gpu.Texture {
	tex := &gpuTexture{backend: b, obj: b.funcs.CreateTexture()}
	tex.Bind(0)
	b.funcs.TexParameteri(TEXTURE_2D, TEXTURE_MAG_FILTER, toTexFilter(magFilter))
	b.funcs.TexParameteri(TEXTURE_2D, TEXTURE_MIN_FILTER, toTexFilter(minFilter))
	b.funcs.TexParameteri(TEXTURE_2D, TEXTURE_WRAP_S, CLAMP_TO_EDGE)
	b.funcs.TexParameteri(TEXTURE_2D, TEXTURE_WRAP_T, CLAMP_TO_EDGE)
	return tex
}

func (b *Backend) NewBuffer(typ gpu.BufferType, data []byte) gpu.Buffer {
	obj := b.funcs.CreateBuffer()
	var gltyp Enum
	switch typ {
	case gpu.BufferTypeVertices:
		gltyp = ARRAY_BUFFER
	case gpu.BufferTypeIndices:
		gltyp = ELEMENT_ARRAY_BUFFER
	default:
		panic("unsupported buffer type")
	}
	buf := &gpuBuffer{backend: b, obj: obj, typ: gltyp}
	b.funcs.BindBuffer(gltyp, obj)
	b.funcs.BufferData(gltyp, data, STATIC_DRAW)
	return buf
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
	b.setupVertexArrays()
	b.funcs.DrawElements(toGLDrawMode(mode), count, UNSIGNED_SHORT, off)
}

func (b *Backend) DrawArrays(mode gpu.DrawMode, off, count int) {
	b.setupVertexArrays()
	b.funcs.DrawArrays(toGLDrawMode(mode), off, count)
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
	return &gpuProgram{backend: b, obj: p, nattr: len(attr)}, nil
}

func (p *gpuProgram) Uniform1i(u gpu.Uniform, v int) {
	p.Bind()
	p.backend.funcs.Uniform1i(u.(Uniform), v)
}

func (p *gpuProgram) Uniform1f(u gpu.Uniform, v0 float32) {
	p.Bind()
	p.backend.funcs.Uniform1f(u.(Uniform), v0)
}

func (p *gpuProgram) Uniform2f(u gpu.Uniform, v0, v1 float32) {
	p.Bind()
	p.backend.funcs.Uniform2f(u.(Uniform), v0, v1)
}

func (p *gpuProgram) Uniform4f(u gpu.Uniform, v0, v1, v2, v3 float32) {
	p.Bind()
	p.backend.funcs.Uniform4f(u.(Uniform), v0, v1, v2, v3)
}

func (p *gpuProgram) Bind() {
	p.backend.useProgram(p)
	p.backend.enableVertexArrays(p.nattr)
}

func (p *gpuProgram) UniformFor(uniform string) gpu.Uniform {
	f := p.backend.funcs
	return GetUniformLocation(f, p.obj, uniform)
}

func (p *gpuProgram) Release() {
	p.backend.funcs.DeleteProgram(p.obj)
}

func (b *gpuBuffer) Release() {
	b.backend.funcs.DeleteBuffer(b.obj)
}

func (b *gpuBuffer) BindVertex(stride, offset int) {
	if b.typ != ARRAY_BUFFER {
		panic("not a vertex buffer")
	}
	b.backend.state.buffer = bufferBinding{buf: b, stride: stride, offset: offset}
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

func (b *gpuBuffer) BindIndex() {
	if b.typ != ELEMENT_ARRAY_BUFFER {
		panic("not an index buffer")
	}
	b.backend.funcs.BindBuffer(ELEMENT_ARRAY_BUFFER, b.obj)
}

func (f *gpuFramebuffer) IsComplete() error {
	if st := f.funcs.CheckFramebufferStatus(FRAMEBUFFER); st != FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("incomplete framebuffer, status = 0x%x, err = %d", st, f.funcs.GetError())
	}
	return nil
}

func (f *gpuFramebuffer) Bind() {
	f.funcs.BindFramebuffer(FRAMEBUFFER, f.obj)
}

func (f *gpuFramebuffer) Invalidate() {
	f.Bind()
	f.funcs.InvalidateFramebuffer(FRAMEBUFFER, COLOR_ATTACHMENT0)
}

func (f *gpuFramebuffer) Release() {
	f.funcs.DeleteFramebuffer(f.obj)
}

func (f *gpuFramebuffer) BindTexture(t gpu.Texture) {
	gltex := t.(*gpuTexture)
	f.Bind()
	f.funcs.FramebufferTexture2D(FRAMEBUFFER, COLOR_ATTACHMENT0, TEXTURE_2D, gltex.obj, 0)
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

func (t *gpuTexture) Bind(unit int) {
	t.backend.bindTexture(unit, t)
}

func (t *gpuTexture) Release() {
	t.backend.funcs.DeleteTexture(t.obj)
}

func (t *gpuTexture) Resize(format gpu.TextureFormat, width, height int) {
	t.Bind(0)
	tt := t.backend.floatTriple
	t.backend.funcs.TexImage2D(TEXTURE_2D, 0, tt.internalFormat, width, height, tt.format, tt.typ, nil)
}

func (t *gpuTexture) Upload(img *image.RGBA) {
	t.Bind(0)
	var pixels []byte
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if img.Stride != w*4 {
		panic("unsupported stride")
	}
	start := (b.Min.X + b.Min.Y*w) * 4
	end := (b.Max.X + (b.Max.Y-1)*w) * 4
	pixels = img.Pix[start:end]
	tt := t.backend.srgbaTriple
	t.backend.funcs.TexImage2D(TEXTURE_2D, 0, tt.internalFormat, w, h, tt.format, tt.typ, pixels)
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

func (l *gpuInputLayout) Bind() {
	l.backend.state.layout = l
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
