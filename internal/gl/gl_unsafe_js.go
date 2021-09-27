//+build unsafe
// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"syscall/js"
)

type FunctionCaller struct {}

func NewFunctionCaller(ctx Context) *FunctionCaller {
	js.Global().Call("setUnsafeGL", js.Value(ctx))
	return &FunctionCaller{}
}

//go:noescape
func asmActiveTexture(t Enum) 

func (f *FunctionCaller) ActiveTexture(t Enum) {
	asmActiveTexture(t)
}

//go:noescape
func asmAttachShader(p Program, s Shader) 

func (f *FunctionCaller) AttachShader(p Program, s Shader) {
	asmAttachShader(p, s)
}

//go:noescape
func asmBindAttribLocation(p Program, a Attrib, name string) 

func (f *FunctionCaller) BindAttribLocation(p Program, a Attrib, name string) {
	asmBindAttribLocation(p, a, name)
}

//go:noescape
func asmBindBuffer(target Enum, b Buffer) 

func (f *FunctionCaller) BindBuffer(target Enum, b Buffer) {
	asmBindBuffer(target, b)
}

//go:noescape
func asmBindBufferBase(target Enum, index int, b Buffer) 

func (f *FunctionCaller) BindBufferBase(target Enum, index int, b Buffer) {
	asmBindBufferBase(target, index, b)
}

//go:noescape
func asmBindFramebuffer(target Enum, fb Framebuffer) 

func (f *FunctionCaller) BindFramebuffer(target Enum, fb Framebuffer) {
	asmBindFramebuffer(target, fb)
}

//go:noescape
func asmBindRenderbuffer(target Enum, rb Renderbuffer) 

func (f *FunctionCaller) BindRenderbuffer(target Enum, rb Renderbuffer) {
	asmBindRenderbuffer(target, rb)
}

//go:noescape
func asmBindTexture(target Enum, t Texture) 

func (f *FunctionCaller) BindTexture(target Enum, t Texture) {
	asmBindTexture(target, t)
}

//go:noescape
func asmBlendEquation(mode Enum) 

func (f *FunctionCaller) BlendEquation(mode Enum) {
	asmBlendEquation(mode)
}

//go:noescape
func asmBlendFuncSeparate(srcRGB Enum, dstRGB Enum, srcA Enum, dstA Enum) 

func (f *FunctionCaller) BlendFuncSeparate(srcRGB, dstRGB, srcA, dstA Enum) {
	asmBlendFuncSeparate(srcRGB, dstRGB, srcA, dstA)
}

//go:noescape
func asmBufferData(target Enum, data []byte, usage Enum) 

func (f *FunctionCaller) BufferData(target Enum, usage Enum, data []byte) {
	asmBufferData(target, data, usage)
}

//go:noescape
func asmBufferDataSize(target Enum, size int, usage Enum) 

func (f *FunctionCaller) BufferDataSize(target Enum, size int, usage Enum) {
	asmBufferDataSize(target, size, usage)
}

//go:noescape
func asmBufferSubData(target Enum, offset int, src []byte) 

func (f *FunctionCaller) BufferSubData(target Enum, offset int, src []byte) {
	asmBufferSubData(target, offset, src)
}

//go:noescape
func asmCheckFramebufferStatus(target Enum) Enum

func (f *FunctionCaller) CheckFramebufferStatus(target Enum) Enum {
	return asmCheckFramebufferStatus(target)
}

//go:noescape
func asmClear(mask Enum) 

func (f *FunctionCaller) Clear(mask Enum) {
	asmClear(mask)
}

//go:noescape
func asmClearColor(red float64, green float64, blue float64, alpha float64) 

func (f *FunctionCaller) ClearColor(red, green, blue, alpha float32) {
	asmClearColor(float64(red), float64(green), float64(blue), float64(alpha))
}

//go:noescape
func asmClearDepthf(d float64) 

func (f *FunctionCaller) ClearDepthf(d float32) {
	asmClearDepthf(float64(d))
}

//go:noescape
func asmCompileShader(s Shader) 

func (f *FunctionCaller) CompileShader(s Shader) {
	asmCompileShader(s)
}

//go:noescape
func asmCopyTexSubImage2D(target Enum, level int, xoffset int, yoffset int, x int, y int, width int, height int) 

func (f *FunctionCaller) CopyTexSubImage2D(target Enum, level, xoffset, yoffset, x, y, width, height int) {
	asmCopyTexSubImage2D(target, level, xoffset, yoffset, x, y, width, height)
}

//go:noescape
func asmCreateBuffer() Buffer

func (f *FunctionCaller) CreateBuffer() Buffer {
	return asmCreateBuffer()
}

//go:noescape
func asmCreateFramebuffer() Framebuffer

func (f *FunctionCaller) CreateFramebuffer() Framebuffer {
	return asmCreateFramebuffer()
}

//go:noescape
func asmCreateProgram() Program

func (f *FunctionCaller) CreateProgram() Program {
	return asmCreateProgram()
}

//go:noescape
func asmCreateRenderbuffer() Renderbuffer

func (f *FunctionCaller) CreateRenderbuffer() Renderbuffer {
	return asmCreateRenderbuffer()
}

//go:noescape
func asmCreateShader(ty Enum) Shader

func (f *FunctionCaller) CreateShader(ty Enum) Shader {
	return asmCreateShader(ty)
}

//go:noescape
func asmCreateTexture() Texture

func (f *FunctionCaller) CreateTexture() Texture {
	return asmCreateTexture()
}

//go:noescape
func asmDeleteBuffer(v Buffer) 

func (f *FunctionCaller) DeleteBuffer(v Buffer) {
	asmDeleteBuffer(v)
}

//go:noescape
func asmDeleteFramebuffer(v Framebuffer) 

func (f *FunctionCaller) DeleteFramebuffer(v Framebuffer) {
	asmDeleteFramebuffer(v)
}

//go:noescape
func asmDeleteProgram(p Program) 

func (f *FunctionCaller) DeleteProgram(p Program) {
	asmDeleteProgram(p)
}

//go:noescape
func asmDeleteShader(s Shader) 

func (f *FunctionCaller) DeleteShader(s Shader) {
	asmDeleteShader(s)
}

//go:noescape
func asmDeleteRenderbuffer(v Renderbuffer) 

func (f *FunctionCaller) DeleteRenderbuffer(v Renderbuffer) {
	asmDeleteRenderbuffer(v)
}

//go:noescape
func asmDeleteTexture(v Texture) 

func (f *FunctionCaller) DeleteTexture(v Texture) {
	asmDeleteTexture(v)
}

//go:noescape
func asmDepthFunc(fn Enum) 

func (f *FunctionCaller) DepthFunc(fn Enum) {
	asmDepthFunc(fn)
}

//go:noescape
func asmDepthMask(mask bool) 

func (f *FunctionCaller) DepthMask(mask bool) {
	asmDepthMask(mask)
}

//go:noescape
func asmDisableVertexAttribArray(a Attrib) 

func (f *FunctionCaller) DisableVertexAttribArray(a Attrib) {
	asmDisableVertexAttribArray(a)
}

//go:noescape
func asmDisable(cap Enum) 

func (f *FunctionCaller) Disable(cap Enum) {
	asmDisable(cap)
}

//go:noescape
func asmDrawArrays(mode Enum, first int, count int) 

func (f *FunctionCaller) DrawArrays(mode Enum, first, count int) {
	asmDrawArrays(mode, first, count)
}

//go:noescape
func asmDrawElements(mode Enum, count int, ty Enum, offset int) 

func (f *FunctionCaller) DrawElements(mode Enum, count int, ty Enum, offset int) {
	asmDrawElements(mode, count, ty, offset)
}

//go:noescape
func asmEnable(cap Enum) 

func (f *FunctionCaller) Enable(cap Enum) {
	asmEnable(cap)
}

//go:noescape
func asmEnableVertexAttribArray(a Attrib) 

func (f *FunctionCaller) EnableVertexAttribArray(a Attrib) {
	asmEnableVertexAttribArray(a)
}

//go:noescape
func asmFinish() 

func (f *FunctionCaller) Finish() {
	asmFinish()
}

//go:noescape
func asmFlush() 

func (f *FunctionCaller) Flush() {
	asmFlush()
}

//go:noescape
func asmFramebufferRenderbuffer(target Enum, attachment Enum, renderbuffertarget Enum, renderbuffer Renderbuffer) 

func (f *FunctionCaller) FramebufferRenderbuffer(target, attachment, renderbuffertarget Enum, renderbuffer Renderbuffer) {
	asmFramebufferRenderbuffer(target, attachment, renderbuffertarget, renderbuffer)
}

//go:noescape
func asmFramebufferTexture2D(target Enum, attachment Enum, texTarget Enum, t Texture, level int) 

func (f *FunctionCaller) FramebufferTexture2D(target, attachment, texTarget Enum, t Texture, level int) {
	asmFramebufferTexture2D(target, attachment, texTarget, t, level)
}

//go:noescape
func asmGetRenderbufferParameteri(pname Enum) int

func (f *FunctionCaller) GetRenderbufferParameteri(target, pname Enum) int {
	return asmGetRenderbufferParameteri(pname)
}

//go:noescape
func asmGetFramebufferAttachmentParameteri(target Enum, attachment Enum, pname Enum) int

func (f *FunctionCaller) GetFramebufferAttachmentParameteri(target, attachment, pname Enum) int {
	return asmGetFramebufferAttachmentParameteri(target, attachment, pname)
}

//go:noescape
func asmGetBinding(pname Enum) Object

func (f *FunctionCaller) GetBinding(pname Enum) Object {
	return asmGetBinding(pname)
}

//go:noescape
func asmGetBindingi(pname Enum, idx int) Object

func (f *FunctionCaller) GetBindingi(pname Enum, idx int) Object {
	return asmGetBindingi(pname, idx)
}

//go:noescape
func asmGetInteger(pname Enum) int

func (f *FunctionCaller) GetInteger(pname Enum) int {
	return asmGetInteger(pname)
}

//go:noescape
func asmGetFloat(pname Enum) float32

func (f *FunctionCaller) GetFloat(pname Enum) float32 {
	return asmGetFloat(pname)
}

//go:noescape
func asmGetInteger4(pname Enum) [4]int

func (f *FunctionCaller) GetInteger4(pname Enum) [4]int {
	return asmGetInteger4(pname)
}

//go:noescape
func asmGetFloat4(pname Enum) [4]float32

func (f *FunctionCaller) GetFloat4(pname Enum) [4]float32 {
	return asmGetFloat4(pname)
}

//go:noescape
func asmGetProgrami(p Program, pname Enum) int

func (f *FunctionCaller) GetProgrami(p Program, pname Enum) int {
	return asmGetProgrami(p, pname)
}

//go:noescape
func asmGetShaderi(s Shader, pname Enum) int

func (f *FunctionCaller) GetShaderi(s Shader, pname Enum) int {
	return asmGetShaderi(s, pname)
}

//go:noescape
func asmGetUniformBlockIndex(p Program, name string) uint

func (f *FunctionCaller) GetUniformBlockIndex(p Program, name string) uint {
	return asmGetUniformBlockIndex(p, name)
}

//go:noescape
func asmGetUniformLocation(p Program, name string) Uniform

func (f *FunctionCaller) GetUniformLocation(p Program, name string) Uniform {
	return asmGetUniformLocation(p, name)
}

//go:noescape
func asmGetVertexAttrib(index int, pname Enum) int

func (f *FunctionCaller) GetVertexAttrib(index int, pname Enum) int {
	return asmGetVertexAttrib(index, pname)
}

//go:noescape
func asmGetVertexAttribBinding(index int, pname Enum) Object

func (f *FunctionCaller) GetVertexAttribBinding(index int, pname Enum) Object {
	return asmGetVertexAttribBinding(index, pname)
}

//go:noescape
func asmGetVertexAttribPointer(index int, pname Enum) uintptr

func (f *FunctionCaller) GetVertexAttribPointer(index int, pname Enum) uintptr {
	return asmGetVertexAttribPointer(index, pname)
}

//go:noescape
func asmInvalidateFramebuffer(target Enum, attachment Enum) 

func (f *FunctionCaller) InvalidateFramebuffer(target, attachment Enum) {
	asmInvalidateFramebuffer(target, attachment)
}

//go:noescape
func asmIsEnabled(cap Enum) bool

func (f *FunctionCaller) IsEnabled(cap Enum) bool {
	return asmIsEnabled(cap)
}

//go:noescape
func asmLinkProgram(p Program) 

func (f *FunctionCaller) LinkProgram(p Program) {
	asmLinkProgram(p)
}

//go:noescape
func asmPixelStorei(pname Enum, param int) 

func (f *FunctionCaller) PixelStorei(pname Enum, param int) {
	asmPixelStorei(pname, param)
}

//go:noescape
func asmRenderbufferStorage(target Enum, internalformat Enum, width int, height int) 

func (f *FunctionCaller) RenderbufferStorage(target, internalformat Enum, width, height int) {
	asmRenderbufferStorage(target, internalformat, width, height)
}

//go:noescape
func asmReadPixels(x int, y int, width int, height int, format Enum, ty Enum, data []byte) 

func (f *FunctionCaller) ReadPixels(x, y, width, height int, format, ty Enum, data []byte) {
	asmReadPixels(x, y, width, height, format, ty, data)
}

//go:noescape
func asmScissor(x int32, y int32, width int32, height int32) 

func (f *FunctionCaller) Scissor(x, y, width, height int32) {
	asmScissor(x, y, width, height)
}

//go:noescape
func asmShaderSource(s Shader, src string) 

func (f *FunctionCaller) ShaderSource(s Shader, src string) {
	asmShaderSource(s, src)
}

//go:noescape
func asmTexImage2D(target Enum, level int, internalFormat Enum, width int, height int, format Enum, ty Enum) 

func (f *FunctionCaller) TexImage2D(target Enum, level int, internalFormat Enum, width, height int, format, ty Enum) {
	asmTexImage2D(target, level, internalFormat, width, height, format, ty)
}

//go:noescape
func asmTexStorage2D(target Enum, levels int, internalFormat Enum, width int, height int) 

func (f *FunctionCaller) TexStorage2D(target Enum, levels int, internalFormat Enum, width, height int) {
	asmTexStorage2D(target, levels, internalFormat, width, height)
}

//go:noescape
func asmTexSubImage2D(target Enum, level int, x int, y int, width int, height int, format Enum, ty Enum, data []byte) 

func (f *FunctionCaller) TexSubImage2D(target Enum, level int, x, y, width, height int, format, ty Enum, data []byte) {
	asmTexSubImage2D(target, level, x, y, width, height, format, ty, data)
}

//go:noescape
func asmTexParameteri(target Enum, pname Enum, param int) 

func (f *FunctionCaller) TexParameteri(target, pname Enum, param int) {
	asmTexParameteri(target, pname, param)
}

//go:noescape
func asmUniformBlockBinding(p Program, uniformBlockIndex uint, uniformBlockBinding uint) 

func (f *FunctionCaller) UniformBlockBinding(p Program, uniformBlockIndex uint, uniformBlockBinding uint) {
	asmUniformBlockBinding(p, uniformBlockIndex, uniformBlockBinding)
}

//go:noescape
func asmUniform1f(dst Uniform, v float64) 

func (f *FunctionCaller) Uniform1f(dst Uniform, v float32) {
	asmUniform1f(dst, float64(v))
}

//go:noescape
func asmUniform1i(dst Uniform, v int) 

func (f *FunctionCaller) Uniform1i(dst Uniform, v int) {
	asmUniform1i(dst, v)
}

//go:noescape
func asmUniform2f(dst Uniform, v0 float64, v1 float64) 

func (f *FunctionCaller) Uniform2f(dst Uniform, v0, v1 float32) {
	asmUniform2f(dst, float64(v0), float64(v1))
}

//go:noescape
func asmUniform3f(dst Uniform, v0 float64, v1 float64, v2 float64) 

func (f *FunctionCaller) Uniform3f(dst Uniform, v0, v1, v2 float32) {
	asmUniform3f(dst, float64(v0), float64(v1), float64(v2))
}

//go:noescape
func asmUniform4f(dst Uniform, v0 float64, v1 float64, v2 float64, v3 float64) 

func (f *FunctionCaller) Uniform4f(dst Uniform, v0, v1, v2, v3 float32) {
	asmUniform4f(dst, float64(v0), float64(v1), float64(v2), float64(v3))
}

//go:noescape
func asmUseProgram(p Program) 

func (f *FunctionCaller) UseProgram(p Program) {
	asmUseProgram(p)
}

//go:noescape
func asmVertexAttribPointer(dst Attrib, size int, ty Enum, normalized bool, stride int, offset int) 

func (f *FunctionCaller) VertexAttribPointer(dst Attrib, size int, ty Enum, normalized bool, stride, offset int) {
	asmVertexAttribPointer(dst, size, ty, normalized, stride, offset)
}

//go:noescape
func asmViewport(x int, y int, width int, height int) 

func (f *FunctionCaller) Viewport(x, y, width, height int) {
	asmViewport(x, y, width, height)
}
