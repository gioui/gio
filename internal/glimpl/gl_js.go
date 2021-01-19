// SPDX-License-Identifier: Unlicense OR MIT

package glimpl

import (
	"errors"
	"strings"
	"syscall/js"
	"unsafe"
)

type Functions struct {
	Ctx                            value
	ExtDisjointTimerQuery          value
	ExtDisjointTimerQueryWebgl2    value
	isInvalidateFramebufferEnabled bool
	InvalidateFrameBufferSlice     []uint32
}

type Context js.Value

func NewFunctions(ctx Context) (*Functions, error) {
	f := &Functions{
		Ctx:                        value{ref: uint64(js.Global().Call("GlimpContext", js.Value(ctx)).Int())},
		InvalidateFrameBufferSlice: make([]uint32, 1),
	}
	if err := f.Init(); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Functions) Init() error {
	_defaultBuffer = make([]byte, 4096)
	buffer(_defaultBuffer)

	c := f.Ctx.Get(_glConstructor)
	if c.Get(_glName).String() != "WebGL2RenderingContext" {
		f.ExtDisjointTimerQuery = f.getExtension("EXT_disjoint_timer_query")
		if f.getExtension("OES_texture_half_float").ref == 0 && f.getExtension("OES_texture_float").ref == 0 {
			return errors.New("gl: no support for neither OES_texture_half_float nor OES_texture_float")
		}
		if f.getExtension("EXT_sRGB").ref == 0 {
			return errors.New("gl: EXT_sRGB not supported")
		}
	} else {
		// WebGL2 extensions.
		f.ExtDisjointTimerQueryWebgl2 = f.getExtension("EXT_disjoint_timer_query_webgl2")
		if f.getExtension("EXT_color_buffer_half_float").ref == 0 && f.getExtension("EXT_color_buffer_float").ref == 0 {
			return errors.New("gl: no support for neither EXT_color_buffer_half_float nor EXT_color_buffer_float")
		}
	}
	f.isInvalidateFramebufferEnabled = f.Ctx.Get(_glInvalidateFramebuffer).ref != 0

	free(c.ref)
	return nil
}

func (f *Functions) getExtension(name string) value {
	return f.Ctx.Call(_glGetExtension, uintptr(unsafe.Pointer(&name)))
}
func (f *Functions) ActiveTexture(t Enum) {
	f.Ctx.Call(_glActiveTexture, uintptr(unsafe.Pointer(&t)))
}
func (f *Functions) AttachShader(p Program, s Shader) {
	f.Ctx.Call(_glAttachShader, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&s)))
}
func (f *Functions) BeginQuery(target Enum, query Query) {
	if f.ExtDisjointTimerQueryWebgl2.ref != 0 {
		f.Ctx.Call(_glBeginQuery, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&query)))
	} else {
		f.ExtDisjointTimerQuery.Call(_glBeginQueryEXT, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&query)))
	}
}
func (f *Functions) BindAttribLocation(p Program, a Attrib, name string) {
	f.Ctx.Call(_glBindAttribLocation, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&a)), uintptr(unsafe.Pointer(&name)))
}
func (f *Functions) BindBuffer(target Enum, b Buffer) {
	f.Ctx.Call(_glBindBuffer, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&b)))
}
func (f *Functions) BindBufferBase(target Enum, index int, b Buffer) {
	f.Ctx.Call(_glBindBufferBase, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&index)), uintptr(unsafe.Pointer(&b)))
}
func (f *Functions) BindFramebuffer(target Enum, fb Framebuffer) {
	f.Ctx.Call(_glBindFramebuffer, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&fb)))
}
func (f *Functions) BindRenderbuffer(target Enum, rb Renderbuffer) {
	f.Ctx.Call(_glBindRenderbuffer, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&rb)))
}
func (f *Functions) BindTexture(target Enum, t Texture) {
	f.Ctx.Call(_glBindTexture, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&t)))
}
func (f *Functions) BlendEquation(mode Enum) {
	f.Ctx.Call(_glBlendEquation, uintptr(unsafe.Pointer(&mode)))
}
func (f *Functions) BlendFunc(sfactor, dfactor Enum) {
	f.Ctx.Call(_glBlendFunc, uintptr(unsafe.Pointer(&sfactor)), uintptr(unsafe.Pointer(&dfactor)))
}
func (f *Functions) BufferData(target Enum, src []byte, usage Enum) {
	f.Ctx.Call(_glBufferData, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&src)), uintptr(unsafe.Pointer(&usage)))
}
func (f *Functions) CheckFramebufferStatus(target Enum) Enum {
	return Enum(f.Ctx.Call(_glCheckFramebufferStatus, uintptr(unsafe.Pointer(&target))).Int())
}
func (f *Functions) Clear(mask Enum) {
	f.Ctx.Call(_glClear, uintptr(unsafe.Pointer(&mask)))
}
func (f *Functions) ClearColor(red, green, blue, alpha float32) {
	f.Ctx.Call(_glClearColor, uintptr(unsafe.Pointer(&red)), uintptr(unsafe.Pointer(&green)), uintptr(unsafe.Pointer(&blue)), uintptr(unsafe.Pointer(&alpha)))
}
func (f *Functions) ClearDepthf(d float32) {
	f.Ctx.Call(_glClearDepth, uintptr(unsafe.Pointer(&d)))
}
func (f *Functions) CompileShader(s Shader) {
	f.Ctx.Call(_glCompileShader, uintptr(unsafe.Pointer(&s)))
}
func (f *Functions) CreateBuffer() Buffer {
	return Buffer(f.Ctx.Call(_glCreateBuffer))
}
func (f *Functions) CreateFramebuffer() Framebuffer {
	return Framebuffer(f.Ctx.Call(_glCreateFramebuffer))
}
func (f *Functions) CreateProgram() Program {
	return Program(f.Ctx.Call(_glCreateProgram))
}
func (f *Functions) CreateQuery() Query {
	return Query(f.Ctx.Call(_glCreateQuery))
}
func (f *Functions) CreateRenderbuffer() Renderbuffer {
	return Renderbuffer(f.Ctx.Call(_glCreateRenderbuffer))
}
func (f *Functions) CreateShader(ty Enum) Shader {
	return Shader(f.Ctx.Call(_glCreateShader, uintptr(unsafe.Pointer(&ty))))
}
func (f *Functions) CreateTexture() Texture {
	return Texture(f.Ctx.Call(_glCreateTexture))
}
func (f *Functions) DeleteBuffer(v Buffer) {
	f.Ctx.Call(_glDeleteBuffer, uintptr(unsafe.Pointer(&v)))
	free(v.ref)
}
func (f *Functions) DeleteFramebuffer(v Framebuffer) {
	f.Ctx.Call(_glDeleteFramebuffer, uintptr(unsafe.Pointer(&v)))
	free(v.ref)
}
func (f *Functions) DeleteProgram(p Program) {
	f.Ctx.Call(_glDeleteProgram, uintptr(unsafe.Pointer(&p)))
	free(p.ref)
}
func (f *Functions) DeleteQuery(query Query) {
	if f.ExtDisjointTimerQueryWebgl2.ref != 0 {
		f.Ctx.Call(_glDeleteQuery, uintptr(unsafe.Pointer(&query)))
	} else {
		f.ExtDisjointTimerQuery.Call(_glDeleteQueryEXT, uintptr(unsafe.Pointer(&query)))
	}
}
func (f *Functions) DeleteShader(s Shader) {
	f.Ctx.Call(_glDeleteShader, uintptr(unsafe.Pointer(&s)))
	free(s.ref)
}
func (f *Functions) DeleteRenderbuffer(v Renderbuffer) {
	f.Ctx.Call(_glDeleteRenderbuffer, uintptr(unsafe.Pointer(&v)))
	free(v.ref)
}
func (f *Functions) DeleteTexture(v Texture) {
	f.Ctx.Call(_glDeleteTexture, uintptr(unsafe.Pointer(&v)))
	free(v.ref)
}
func (f *Functions) DepthFunc(fn Enum) {
	f.Ctx.Call(_glDepthFunc, uintptr(unsafe.Pointer(&fn)))
}
func (f *Functions) DepthMask(mask bool) {
	f.Ctx.Call(_glDepthMask, uintptr(unsafe.Pointer(&mask)))
}
func (f *Functions) DisableVertexAttribArray(a Attrib) {
	f.Ctx.Call(_glDisableVertexAttribArray, uintptr(unsafe.Pointer(&a)))
}
func (f *Functions) Disable(cap Enum) {
	f.Ctx.Call(_glDisable, uintptr(unsafe.Pointer(&cap)))
}
func (f *Functions) DrawArrays(mode Enum, first, count int) {
	f.Ctx.Call(_glDrawArrays, uintptr(unsafe.Pointer(&mode)), uintptr(unsafe.Pointer(&first)), uintptr(unsafe.Pointer(&count)))
}
func (f *Functions) DrawElements(mode Enum, count int, ty Enum, offset int) {
	f.Ctx.Call(_glDrawElements, uintptr(unsafe.Pointer(&mode)), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&ty)), uintptr(unsafe.Pointer(&offset)))
}
func (f *Functions) Enable(cap Enum) {
	f.Ctx.Call(_glEnable, uintptr(unsafe.Pointer(&cap)))
}
func (f *Functions) EnableVertexAttribArray(a Attrib) {
	f.Ctx.Call(_glEnableVertexAttribArray, uintptr(unsafe.Pointer(&a)))
}
func (f *Functions) EndQuery(target Enum) {
	if f.ExtDisjointTimerQueryWebgl2.ref != 0 {
		f.Ctx.Call(_glEndQuery, uintptr(unsafe.Pointer(&target)))
	} else {
		f.ExtDisjointTimerQuery.Call(_glEndQueryEXT, uintptr(unsafe.Pointer(&target)))
	}
}
func (f *Functions) Finish() {
	f.Ctx.Call(_glFinish)
}
func (f *Functions) FramebufferRenderbuffer(target, attachment, renderbuffertarget Enum, renderbuffer Renderbuffer) {
	f.Ctx.Call(_glFramebufferRenderbuffer, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&attachment)), uintptr(unsafe.Pointer(&renderbuffertarget)), uintptr(unsafe.Pointer(&renderbuffer)))
}
func (f *Functions) FramebufferTexture2D(target, attachment, texTarget Enum, t Texture, level int) {
	f.Ctx.Call(_glFramebufferTexture2D, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&attachment)), uintptr(unsafe.Pointer(&texTarget)), uintptr(unsafe.Pointer(&t)), uintptr(unsafe.Pointer(&level)))
}
func (f *Functions) GetError() Enum {
	// Avoid slow getError calls. See gio#179.
	return 0
}
func (f *Functions) GetRenderbufferParameteri(_, pname Enum) int {
	return f.Ctx.Call(_glGetRenderbufferParameteri, uintptr(unsafe.Pointer(&pname))).Int()
}
func (f *Functions) GetFramebufferAttachmentParameteri(target, attachment, pname Enum) int {
	return f.Ctx.Call(_glGetFramebufferAttachmentParameter, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&attachment)), uintptr(unsafe.Pointer(&pname))).Int()
}
func (f *Functions) GetBinding(pname Enum) Object {
	return Object(f.Ctx.Call(_glGetParameterv, uintptr(unsafe.Pointer(&pname))))
}
func (f *Functions) GetInteger(pname Enum) int {
	return f.Ctx.Call(_glGetParameteri, uintptr(unsafe.Pointer(&pname))).Int()
}
func (f *Functions) GetProgrami(p Program, pname Enum) int {
	return f.Ctx.Call(_glGetProgramParameter, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&pname))).Int()
}
func (f *Functions) GetProgramInfoLog(p Program) string {
	return f.Ctx.Call(_glGetProgramInfoLog, uintptr(unsafe.Pointer(&p))).String()
}
func (f *Functions) GetQueryObjectuiv(query Query, pname Enum) uint {
	if f.ExtDisjointTimerQueryWebgl2.ref != 0 {
		return uint(f.Ctx.Call(_glGetQueryParameter, uintptr(unsafe.Pointer(&query)), uintptr(unsafe.Pointer(&pname))).Int())
	} else {
		return uint(f.ExtDisjointTimerQuery.Call(_glGetQueryParameterEXT, uintptr(unsafe.Pointer(&query)), uintptr(unsafe.Pointer(&pname))).Int())
	}
}
func (f *Functions) GetShaderi(s Shader, pname Enum) int {
	return f.Ctx.Call(_glGetShaderParameter, uintptr(unsafe.Pointer(&s)), uintptr(unsafe.Pointer(&pname))).Int()
}
func (f *Functions) GetShaderInfoLog(s Shader) string {
	return f.Ctx.Call(_glGetShaderInfoLog, uintptr(unsafe.Pointer(&s))).String()
}
func (f *Functions) GetString(pname Enum) string {
	switch pname {
	case EXTENSIONS:
		exts := strings.Split(f.Ctx.Call(_glGetSupportedExtensions).String(), ",")
		for i, s := range exts {
			// GL_ prefix is mandatory
			exts[i] = "GL_" + s
		}
		return strings.Join(exts, " ")
	default:
		return f.Ctx.Call(_glGetParameter, uintptr(unsafe.Pointer(&pname))).String()
	}
}
func (f *Functions) GetUniformBlockIndex(p Program, name string) uint {
	return uint(f.Ctx.Call(_glGetUniformBlockIndex, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&name))).Int())
}
func (f *Functions) GetUniformLocation(p Program, name string) Uniform {
	return Uniform(f.Ctx.Call(_glGetUniformLocation, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&name))))
}
func (f *Functions) InvalidateFramebuffer(target, attachment Enum) {
	if f.isInvalidateFramebufferEnabled {
		f.InvalidateFrameBufferSlice[0] = uint32(attachment)
		f.Ctx.Call(_glInvalidateFramebuffer, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&f.InvalidateFrameBufferSlice)))
	}
}
func (f *Functions) LinkProgram(p Program) {
	f.Ctx.Call(_glLinkProgram, uintptr(unsafe.Pointer(&p)))
}
func (f *Functions) PixelStorei(pname Enum, param int32) {
	f.Ctx.Call(_glPixelStorei, uintptr(unsafe.Pointer(&pname)), uintptr(unsafe.Pointer(&param)))
}
func (f *Functions) RenderbufferStorage(target, internalFormat Enum, width, height int) {
	f.Ctx.Call(_glRenderbufferStorage, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&internalFormat)), uintptr(unsafe.Pointer(&width)), uintptr(unsafe.Pointer(&height)))
}
func (f *Functions) ReadPixels(x, y, width, height int, format, ty Enum, data []byte) {
	f.Ctx.Call(_glReadPixels, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)), uintptr(unsafe.Pointer(&width)), uintptr(unsafe.Pointer(&height)), uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&ty)), uintptr(unsafe.Pointer(&data)))
}
func (f *Functions) Scissor(x, y, width, height int32) {
	f.Ctx.Call(_glScissor, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)), uintptr(unsafe.Pointer(&width)), uintptr(unsafe.Pointer(&height)))
}
func (f *Functions) ShaderSource(s Shader, src string) {
	f.Ctx.Call(_glShaderSource, uintptr(unsafe.Pointer(&s)), uintptr(unsafe.Pointer(&src)))
}
func (f *Functions) TexImage2D(target Enum, level int, internalFormat int, width, height int, format, ty Enum, data []byte) {
	f.Ctx.Call(_glTexImage2D, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&level)), uintptr(unsafe.Pointer(&internalFormat)), uintptr(unsafe.Pointer(&width)), uintptr(unsafe.Pointer(&height)), uintptr(unsafe.Pointer(&_zero)), uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&ty)), uintptr(unsafe.Pointer(&data)))
}
func (f *Functions) TexSubImage2D(target Enum, level int, x, y, width, height int, format, ty Enum, data []byte) {
	f.Ctx.Call(_glTexSubImage2D, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&level)), uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)), uintptr(unsafe.Pointer(&width)), uintptr(unsafe.Pointer(&height)), uintptr(unsafe.Pointer(&format)), uintptr(unsafe.Pointer(&ty)), uintptr(unsafe.Pointer(&data)))
}
func (f *Functions) TexParameteri(target, pname Enum, param int) {
	f.Ctx.Call(_glTexParameteri, uintptr(unsafe.Pointer(&target)), uintptr(unsafe.Pointer(&pname)), uintptr(unsafe.Pointer(&param)))
}
func (f *Functions) UniformBlockBinding(p Program, uniformBlockIndex uint, uniformBlockBinding uint) {
	f.Ctx.Call(_glUniformBlockBinding, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&uniformBlockIndex)), uintptr(unsafe.Pointer(&uniformBlockBinding)))
}
func (f *Functions) Uniform1f(dst Uniform, v0 float32) {
	f.Ctx.Call(_glUniform1f, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&v0)))
}
func (f *Functions) Uniform1i(dst Uniform, v int) {
	f.Ctx.Call(_glUniform1i, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&v)))
}
func (f *Functions) Uniform2f(dst Uniform, v0, v1 float32) {
	f.Ctx.Call(_glUniform2f, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&v0)), uintptr(unsafe.Pointer(&v1)))
}
func (f *Functions) Uniform3f(dst Uniform, v0, v1, v2 float32) {
	f.Ctx.Call(_glUniform3f, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&v0)), uintptr(unsafe.Pointer(&v1)), uintptr(unsafe.Pointer(&v2)))
}
func (f *Functions) Uniform4f(dst Uniform, v0, v1, v2, v3 float32) {
	f.Ctx.Call(_glUniform4f, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&v0)), uintptr(unsafe.Pointer(&v1)), uintptr(unsafe.Pointer(&v2)), uintptr(unsafe.Pointer(&v3)))
}
func (f *Functions) UseProgram(p Program) {
	f.Ctx.Call(_glUseProgram, uintptr(unsafe.Pointer(&p)))
}
func (f *Functions) VertexAttribPointer(dst Attrib, size int, ty Enum, normalized bool, stride, offset int) {
	f.Ctx.Call(_glVertexAttribPointer, uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&size)), uintptr(unsafe.Pointer(&ty)), uintptr(unsafe.Pointer(&normalized)), uintptr(unsafe.Pointer(&stride)), uintptr(unsafe.Pointer(&offset)))
}
func (f *Functions) Viewport(x, y, width, height int) {
	f.Ctx.Call(_glViewPort, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)), uintptr(unsafe.Pointer(&width)), uintptr(unsafe.Pointer(&height)))
}

type jsType byte

const (
	_glTypeNull jsType = iota | 0x10
	_glTypeByte
	_glTypeInt32
	_glTypeInt64
	_glTypeFloat32
	_glTypeFloat64
	_glTypeBoolean
	_glTypeString

	_glTypeSlice jsType = iota | 0x30
	_glTypeSlice32

	_glTypeValue jsType = iota | 0x40
)

type proc struct {
	ref    uint64
	output jsType
}

var (
	_glConstructor = newProc(_glTypeValue, "constructor")
	_glName        = newProc(_glTypeString, "name")

	_glGetExtension                      = newProc(_glTypeValue, "getExtension", _glTypeString)
	_glActiveTexture                     = newProc(_glTypeNull, "activeTexture", _glTypeInt64)
	_glAttachShader                      = newProc(_glTypeNull, "attachShader", _glTypeValue, _glTypeValue)
	_glBindAttribLocation                = newProc(_glTypeNull, "bindAttribLocation", _glTypeValue, _glTypeInt64, _glTypeString)
	_glBindBuffer                        = newProc(_glTypeNull, "bindBuffer", _glTypeInt64, _glTypeValue)
	_glBindBufferBase                    = newProc(_glTypeNull, "bindBufferBase", _glTypeInt64, _glTypeInt64, _glTypeValue)
	_glBindFramebuffer                   = newProc(_glTypeNull, "bindFramebuffer", _glTypeInt64, _glTypeValue)
	_glBindRenderbuffer                  = newProc(_glTypeNull, "bindRenderbuffer", _glTypeInt64, _glTypeValue)
	_glBindTexture                       = newProc(_glTypeNull, "bindTexture", _glTypeInt64, _glTypeValue)
	_glBlendEquation                     = newProc(_glTypeNull, "blendEquation", _glTypeInt64)
	_glBlendFunc                         = newProc(_glTypeNull, "blendFunc", _glTypeInt64, _glTypeInt64)
	_glBufferData                        = newProc(_glTypeNull, "bufferData", _glTypeInt64, _glTypeSlice, _glTypeInt64)
	_glCheckFramebufferStatus            = newProc(_glTypeInt64, "checkFramebufferStatus", _glTypeInt64)
	_glClear                             = newProc(_glTypeNull, "clear", _glTypeInt64)
	_glClearColor                        = newProc(_glTypeNull, "clearColor", _glTypeFloat32, _glTypeFloat32, _glTypeFloat32, _glTypeFloat32)
	_glClearDepth                        = newProc(_glTypeNull, "clearDepth", _glTypeFloat32)
	_glCompileShader                     = newProc(_glTypeNull, "compileShader", _glTypeValue)
	_glCreateBuffer                      = newProc(_glTypeValue, "createBuffer")
	_glCreateFramebuffer                 = newProc(_glTypeValue, "createFramebuffer")
	_glCreateProgram                     = newProc(_glTypeValue, "createProgram")
	_glCreateQuery                       = newProc(_glTypeValue, "createQuery")
	_glCreateRenderbuffer                = newProc(_glTypeValue, "createRenderbuffer")
	_glCreateShader                      = newProc(_glTypeValue, "createShader", _glTypeInt64)
	_glCreateTexture                     = newProc(_glTypeValue, "createTexture")
	_glDeleteBuffer                      = newProc(_glTypeNull, "deleteBuffer", _glTypeValue)
	_glDeleteFramebuffer                 = newProc(_glTypeNull, "deleteFramebuffer", _glTypeValue)
	_glDeleteProgram                     = newProc(_glTypeNull, "deleteProgram", _glTypeValue)
	_glDeleteShader                      = newProc(_glTypeNull, "deleteShader", _glTypeValue)
	_glDeleteRenderbuffer                = newProc(_glTypeNull, "deleteRenderbuffer", _glTypeValue)
	_glDeleteTexture                     = newProc(_glTypeNull, "deleteTexture", _glTypeValue)
	_glDepthFunc                         = newProc(_glTypeNull, "depthFunc", _glTypeInt64)
	_glDepthMask                         = newProc(_glTypeNull, "depthMask", _glTypeBoolean)
	_glDisableVertexAttribArray          = newProc(_glTypeNull, "disableVertexAttribArray", _glTypeInt64)
	_glDrawArrays                        = newProc(_glTypeNull, "drawArrays", _glTypeInt64, _glTypeInt64, _glTypeInt64)
	_glDrawElements                      = newProc(_glTypeNull, "drawElements", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64)
	_glDisable                           = newProc(_glTypeNull, "disable", _glTypeInt64)
	_glEnable                            = newProc(_glTypeNull, "enable", _glTypeInt64)
	_glEnableVertexAttribArray           = newProc(_glTypeNull, "enableVertexAttribArray", _glTypeInt64)
	_glFinish                            = newProc(_glTypeNull, "finish")
	_glFramebufferRenderbuffer           = newProc(_glTypeNull, "framebufferRenderbuffer", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeValue)
	_glFramebufferTexture2D              = newProc(_glTypeNull, "framebufferTexture2D", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeValue, _glTypeInt64)
	_glGetError                          = newProc(_glTypeInt64, "getError")
	_glGetRenderbufferParameteri         = newProc(_glTypeInt64, "getRenderbufferParameteri", _glTypeInt64)
	_glGetFramebufferAttachmentParameter = newProc(_glTypeInt64, "getFramebufferAttachmentParameter", _glTypeInt64, _glTypeInt64, _glTypeInt64)
	_glGetParameterv                     = newProc(_glTypeValue, "getParameter", _glTypeInt64)
	_glGetParameteri                     = newProc(_glTypeInt64, "getParameter", _glTypeInt64)
	_glGetProgramParameter               = newProc(_glTypeInt64, "getProgramParameter", _glTypeValue, _glTypeInt64)
	_glGetProgramInfoLog                 = newProc(_glTypeString, "getProgramInfoLog", _glTypeValue)
	_glGetShaderParameter                = newProc(_glTypeInt64, "getShaderParameter", _glTypeValue, _glTypeInt64)
	_glGetShaderInfoLog                  = newProc(_glTypeString, "getShaderInfoLog", _glTypeValue)
	_glGetParameter                      = newProc(_glTypeString, "getParameter", _glTypeInt64)
	_glGetSupportedExtensions            = newProc(_glTypeString, "getSupportedExtensions")
	_glGetUniformBlockIndex              = newProc(_glTypeInt64, "getUniformBlockIndex", _glTypeValue, _glTypeString)
	_glGetUniformLocation                = newProc(_glTypeValue, "getUniformLocation", _glTypeValue, _glTypeString)
	_glInvalidateFramebuffer             = newProc(_glTypeNull, "invalidateFramebuffer", _glTypeInt64, _glTypeSlice32)
	_glLinkProgram                       = newProc(_glTypeNull, "linkProgram", _glTypeValue)
	_glPixelStorei                       = newProc(_glTypeNull, "pixelStorei", _glTypeInt64, _glTypeInt64)
	_glRenderbufferStorage               = newProc(_glTypeNull, "renderbufferStorage", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64)
	_glReadPixels                        = newProc(_glTypeNull, "readPixels", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeSlice)
	_glScissor                           = newProc(_glTypeNull, "scissor", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64)
	_glShaderSource                      = newProc(_glTypeNull, "shaderSource", _glTypeValue, _glTypeString)
	_glTexImage2D                        = newProc(_glTypeNull, "texImage2D", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeSlice)
	_glTexSubImage2D                     = newProc(_glTypeNull, "texSubImage2D", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeSlice)
	_glTexParameteri                     = newProc(_glTypeNull, "texParameteri", _glTypeInt64, _glTypeInt64, _glTypeInt64)
	_glUniformBlockBinding               = newProc(_glTypeNull, "uniformBlockBinding", _glTypeValue, _glTypeInt64, _glTypeInt64)
	_glUniform1i                         = newProc(_glTypeNull, "uniform1i", _glTypeValue, _glTypeInt64)
	_glUniform1f                         = newProc(_glTypeNull, "uniform1f", _glTypeValue, _glTypeFloat32)
	_glUniform2f                         = newProc(_glTypeNull, "uniform2f", _glTypeValue, _glTypeFloat32, _glTypeFloat32)
	_glUniform3f                         = newProc(_glTypeNull, "uniform3f", _glTypeValue, _glTypeFloat32, _glTypeFloat32, _glTypeFloat32)
	_glUniform4f                         = newProc(_glTypeNull, "uniform4f", _glTypeValue, _glTypeFloat32, _glTypeFloat32, _glTypeFloat32, _glTypeFloat32)
	_glUseProgram                        = newProc(_glTypeNull, "useProgram", _glTypeValue)
	_glVertexAttribPointer               = newProc(_glTypeNull, "vertexAttribPointer", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeBoolean, _glTypeInt64, _glTypeInt64)
	_glViewPort                          = newProc(_glTypeNull, "viewport", _glTypeInt64, _glTypeInt64, _glTypeInt64, _glTypeInt64)
	_glBeginQuery                        = newProc(_glTypeNull, "beginQuery", _glTypeInt64, _glTypeValue)
	_glBeginQueryEXT                     = newProc(_glTypeNull, "beginQueryEXT", _glTypeInt64, _glTypeValue)
	_glDeleteQuery                       = newProc(_glTypeNull, "deleteQuery", _glTypeValue)
	_glDeleteQueryEXT                    = newProc(_glTypeNull, "deleteQueryEXT", _glTypeValue)
	_glEndQuery                          = newProc(_glTypeNull, "endQuery", _glTypeInt64)
	_glEndQueryEXT                       = newProc(_glTypeNull, "endQueryEXT", _glTypeInt64)
	_glGetQueryParameter                 = newProc(_glTypeInt64, "getQueryParameter", _glTypeValue, _glTypeInt64)
	_glGetQueryParameterEXT              = newProc(_glTypeInt64, "getQueryObjectEXT", _glTypeValue, _glTypeInt64)
)

var (
	// _defaultBuffer is used to store the string, it's read by value.String function.
	_defaultBuffer    []byte
	_defaultUndefined value
	_zero             int
)

func newProc(output jsType, function string, input ...jsType) proc {
	return proc{ref: newMethod(output, function, input), output: output}
}

type value struct {
	// ref is the reference based on the output jsType.
	// _glTypeString the ref is the length of the string.
	// _glTypeInt64 the ref is the integer-value.
	// _glTypeValue the ref is the index of the JS-object in the `values` map.
	ref uint64
	// Do not change the order, that matters to JS. Ref must be the first!
}

func (v value) Call(id proc, args ...uintptr) value {
	ref := call(v.ref, id.ref, args)
	if ref == 0 {
		return _defaultUndefined
	}

	return value{ref: uint64(ref)}
}

func (v value) Get(id proc) value {
	return value{ref: get(v.ref, id.ref)}
}

// Int returns the ref as int.
// Make sure that you are using _glTypeInt64 to avoid surprises.
func (v value) Int() int {
	return int(v.ref)
}

// String returns the string.
// It must be called IMMEDIATELY after the Call, otherwise
// you can get trash or information from another call
// It's not thread safe. But WebGL seems to be not thread-safe too, anyway.
func (v value) String() string {
	return string(_defaultBuffer[:v.ref])
}

func newMethod(output jsType, function string, input []jsType) uint64
func call(ref uint64, method uint64, args []uintptr) uint64
func get(ref uint64, method uint64) uint64
func free(ref uint64) uint64
func buffer([]byte)
