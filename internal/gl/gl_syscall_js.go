//go:build !unsafe
// +build !unsafe

// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"syscall/js"
)

type FunctionCaller struct {
	Ctx js.Value

	// Cached reference to the Uint8Array JS type.
	uint8Array js.Value

	// Cached JS arrays.
	arrayBuf js.Value
	int32Buf js.Value

	isWebGL2 bool
}

func NewFunctionCaller(ctx Context) *FunctionCaller {
	return &FunctionCaller{
		Ctx:        js.Value(ctx),
		uint8Array: js.Global().Get("Uint8Array"),
	}
}

func (f *FunctionCaller) ActiveTexture(t Enum) {
	f.Ctx.Call("activeTexture", int(t))
}
func (f *FunctionCaller) AttachShader(p Program, s Shader) {
	f.Ctx.Call("attachShader", js.Value(p), js.Value(s))
}
func (f *FunctionCaller) BindAttribLocation(p Program, a Attrib, name string) {
	f.Ctx.Call("bindAttribLocation", js.Value(p), int(a), name)
}
func (f *FunctionCaller) BindBuffer(target Enum, b Buffer) {
	f.Ctx.Call("bindBuffer", int(target), js.Value(b))
}
func (f *FunctionCaller) BindBufferBase(target Enum, index int, b Buffer) {
	f.Ctx.Call("bindBufferBase", int(target), index, js.Value(b))
}
func (f *FunctionCaller) BindFramebuffer(target Enum, fb Framebuffer) {
	f.Ctx.Call("bindFramebuffer", int(target), js.Value(fb))
}
func (f *FunctionCaller) BindRenderbuffer(target Enum, rb Renderbuffer) {
	f.Ctx.Call("bindRenderbuffer", int(target), js.Value(rb))
}
func (f *FunctionCaller) BindTexture(target Enum, t Texture) {
	f.Ctx.Call("bindTexture", int(target), js.Value(t))
}
func (f *FunctionCaller) BlendEquation(mode Enum) {
	f.Ctx.Call("blendEquation", int(mode))
}
func (f *FunctionCaller) BlendFuncSeparate(srcRGB, dstRGB, srcA, dstA Enum) {
	f.Ctx.Call("blendFunc", int(srcRGB), int(dstRGB), int(srcA), int(dstA))
}
func (f *FunctionCaller) BufferData(target Enum, usage Enum, data []byte) {
	f.Ctx.Call("bufferData", int(target), f.byteArrayOf(data), int(usage))
}
func (f *FunctionCaller) BufferDataSize(target Enum, size int, usage Enum) {
	f.Ctx.Call("bufferData", int(target), size, int(usage))
}
func (f *FunctionCaller) BufferSubData(target Enum, offset int, src []byte) {
	f.Ctx.Call("bufferSubData", int(target), offset, f.byteArrayOf(src))
}
func (f *FunctionCaller) CheckFramebufferStatus(target Enum) Enum {
	return Enum(f.Ctx.Call("checkFramebufferStatus", int(target)).Int())
}
func (f *FunctionCaller) Clear(mask Enum) {
	f.Ctx.Call("clear", int(mask))
}
func (f *FunctionCaller) ClearColor(red, green, blue, alpha float32) {
	f.Ctx.Call("clearColor", red, green, blue, alpha)
}
func (f *FunctionCaller) ClearDepthf(d float32) {
	f.Ctx.Call("clearDepth", d)
}
func (f *FunctionCaller) CompileShader(s Shader) {
	f.Ctx.Call("compileShader", js.Value(s))
}
func (f *FunctionCaller) CopyTexSubImage2D(target Enum, level, xoffset, yoffset, x, y, width, height int) {
	f.Ctx.Call("copyTexSubImage2D", int(target), level, xoffset, yoffset, x, y, width, height)
}
func (f *FunctionCaller) CreateBuffer() Buffer {
	return Buffer(f.Ctx.Call("createBuffer"))
}
func (f *FunctionCaller) CreateFramebuffer() Framebuffer {
	return Framebuffer(f.Ctx.Call("createFramebuffer"))
}
func (f *FunctionCaller) CreateProgram() Program {
	return Program(f.Ctx.Call("createProgram"))
}
func (f *FunctionCaller) CreateRenderbuffer() Renderbuffer {
	return Renderbuffer(f.Ctx.Call("createRenderbuffer"))
}
func (f *FunctionCaller) CreateShader(ty Enum) Shader {
	return Shader(f.Ctx.Call("createShader", int(ty)))
}
func (f *FunctionCaller) CreateTexture() Texture {
	return Texture(f.Ctx.Call("createTexture"))
}
func (f *FunctionCaller) DeleteBuffer(v Buffer) {
	f.Ctx.Call("deleteBuffer", js.Value(v))
}
func (f *FunctionCaller) DeleteFramebuffer(v Framebuffer) {
	f.Ctx.Call("deleteFramebuffer", js.Value(v))
}
func (f *FunctionCaller) DeleteProgram(p Program) {
	f.Ctx.Call("deleteProgram", js.Value(p))
}
func (f *FunctionCaller) DeleteShader(s Shader) {
	f.Ctx.Call("deleteShader", js.Value(s))
}
func (f *FunctionCaller) DeleteRenderbuffer(v Renderbuffer) {
	f.Ctx.Call("deleteRenderbuffer", js.Value(v))
}
func (f *FunctionCaller) DeleteTexture(v Texture) {
	f.Ctx.Call("deleteTexture", js.Value(v))
}
func (f *FunctionCaller) DepthFunc(fn Enum) {
	f.Ctx.Call("depthFunc", int(fn))
}
func (f *FunctionCaller) DepthMask(mask bool) {
	f.Ctx.Call("depthMask", mask)
}
func (f *FunctionCaller) DisableVertexAttribArray(a Attrib) {
	f.Ctx.Call("disableVertexAttribArray", int(a))
}
func (f *FunctionCaller) Disable(cap Enum) {
	f.Ctx.Call("disable", int(cap))
}
func (f *FunctionCaller) DrawArrays(mode Enum, first, count int) {
	f.Ctx.Call("drawArrays", int(mode), first, count)
}
func (f *FunctionCaller) DrawElements(mode Enum, count int, ty Enum, offset int) {
	f.Ctx.Call("drawElements", int(mode), count, int(ty), offset)
}
func (f *FunctionCaller) Enable(cap Enum) {
	f.Ctx.Call("enable", int(cap))
}
func (f *FunctionCaller) EnableVertexAttribArray(a Attrib) {
	f.Ctx.Call("enableVertexAttribArray", int(a))
}
func (f *FunctionCaller) Finish() {
	f.Ctx.Call("finish")
}
func (f *FunctionCaller) Flush() {
	f.Ctx.Call("flush")
}
func (f *FunctionCaller) FramebufferRenderbuffer(target, attachment, renderbuffertarget Enum, renderbuffer Renderbuffer) {
	f.Ctx.Call("framebufferRenderbuffer", int(target), int(attachment), int(renderbuffertarget), js.Value(renderbuffer))
}
func (f *FunctionCaller) FramebufferTexture2D(target, attachment, texTarget Enum, t Texture, level int) {
	f.Ctx.Call("framebufferTexture2D", int(target), int(attachment), int(texTarget), js.Value(t), level)
}
func (f *FunctionCaller) GetRenderbufferParameteri(target, pname Enum) int {
	return paramVal(f.Ctx.Call("getRenderbufferParameteri", int(pname)))
}
func (f *FunctionCaller) GetFramebufferAttachmentParameteri(target, attachment, pname Enum) int {
	return paramVal(f.Ctx.Call("getFramebufferAttachmentParameter", int(target), int(attachment), int(pname)))
}
func (f *FunctionCaller) GetBinding(pname Enum) Object {
	return Object(f.Ctx.Call("getParameter", int(pname)))
}
func (f *FunctionCaller) GetBindingi(pname Enum, idx int) Object {
	return Object(f.Ctx.Call("getIndexedParameter", int(pname), idx))
}
func (f *FunctionCaller) GetInteger(pname Enum) int {
	return paramVal(f.Ctx.Call("getParameter", int(pname)))
}
func (f *FunctionCaller) GetFloat(pname Enum) float32 {
	return float32(f.Ctx.Call("getParameter", int(pname)).Float())
}
func (f *FunctionCaller) GetInteger4(pname Enum) [4]int {
	arr := f.Ctx.Call("getParameter", int(pname))
	var res [4]int
	for i := range res {
		res[i] = arr.Index(i).Int()
	}
	return res
}
func (f *FunctionCaller) GetFloat4(pname Enum) [4]float32 {
	arr := f.Ctx.Call("getParameter", int(pname))
	var res [4]float32
	for i := range res {
		res[i] = float32(arr.Index(i).Float())
	}
	return res
}
func (f *FunctionCaller) GetProgrami(p Program, pname Enum) int {
	return paramVal(f.Ctx.Call("getProgramParameter", js.Value(p), int(pname)))
}
func (f *FunctionCaller) GetShaderi(s Shader, pname Enum) int {
	return paramVal(f.Ctx.Call("getShaderParameter", js.Value(s), int(pname)))
}
func (f *FunctionCaller) GetUniformBlockIndex(p Program, name string) uint {
	return uint(paramVal(f.Ctx.Call("getUniformBlockIndex", js.Value(p), name)))
}
func (f *FunctionCaller) GetUniformLocation(p Program, name string) Uniform {
	return Uniform(f.Ctx.Call("getUniformLocation", js.Value(p), name))
}
func (f *FunctionCaller) GetVertexAttrib(index int, pname Enum) int {
	return paramVal(f.Ctx.Call("getVertexAttrib", index, int(pname)))
}
func (f *FunctionCaller) GetVertexAttribBinding(index int, pname Enum) Object {
	return Object(f.Ctx.Call("getVertexAttrib", index, int(pname)))
}
func (f *FunctionCaller) GetVertexAttribPointer(index int, pname Enum) uintptr {
	return uintptr(f.Ctx.Call("getVertexAttribOffset", index, int(pname)).Int())
}
func (f *FunctionCaller) InvalidateFramebuffer(target, attachment Enum) {
	fn := f.Ctx.Get("invalidateFramebuffer")
	if !fn.IsUndefined() {
		if f.int32Buf.IsUndefined() {
			f.int32Buf = js.Global().Get("Int32Array").New(1)
		}
		f.int32Buf.SetIndex(0, int32(attachment))
		f.Ctx.Call("invalidateFramebuffer", int(target), f.int32Buf)
	}
}
func (f *FunctionCaller) IsEnabled(cap Enum) bool {
	return f.Ctx.Call("isEnabled", int(cap)).Truthy()
}
func (f *FunctionCaller) LinkProgram(p Program) {
	f.Ctx.Call("linkProgram", js.Value(p))
}
func (f *FunctionCaller) PixelStorei(pname Enum, param int) {
	f.Ctx.Call("pixelStorei", int(pname), param)
}
func (f *FunctionCaller) RenderbufferStorage(target, internalformat Enum, width, height int) {
	f.Ctx.Call("renderbufferStorage", int(target), int(internalformat), width, height)
}
func (f *FunctionCaller) ReadPixels(x, y, width, height int, format, ty Enum, data []byte) {
	ba := f.byteArrayOf(data)
	f.Ctx.Call("readPixels", x, y, width, height, int(format), int(ty), ba)
	js.CopyBytesToGo(data, ba)
}
func (f *FunctionCaller) Scissor(x, y, width, height int32) {
	f.Ctx.Call("scissor", x, y, width, height)
}
func (f *FunctionCaller) ShaderSource(s Shader, src string) {
	f.Ctx.Call("shaderSource", js.Value(s), src)
}
func (f *FunctionCaller) TexImage2D(target Enum, level int, internalFormat Enum, width, height int, format, ty Enum) {
	f.Ctx.Call("texImage2D", int(target), int(level), int(internalFormat), int(width), int(height), 0, int(format), int(ty), nil)
}
func (f *FunctionCaller) TexStorage2D(target Enum, levels int, internalFormat Enum, width, height int) {
	f.Ctx.Call("texStorage2D", int(target), levels, int(internalFormat), width, height)
}
func (f *FunctionCaller) TexSubImage2D(target Enum, level int, x, y, width, height int, format, ty Enum, data []byte) {
	f.Ctx.Call("texSubImage2D", int(target), level, x, y, width, height, int(format), int(ty), f.byteArrayOf(data))
}
func (f *FunctionCaller) TexParameteri(target, pname Enum, param int) {
	f.Ctx.Call("texParameteri", int(target), int(pname), int(param))
}
func (f *FunctionCaller) UniformBlockBinding(p Program, uniformBlockIndex uint, uniformBlockBinding uint) {
	f.Ctx.Call("uniformBlockBinding", js.Value(p), int(uniformBlockIndex), int(uniformBlockBinding))
}
func (f *FunctionCaller) Uniform1f(dst Uniform, v float32) {
	f.Ctx.Call("uniform1f", js.Value(dst), v)
}
func (f *FunctionCaller) Uniform1i(dst Uniform, v int) {
	f.Ctx.Call("uniform1i", js.Value(dst), v)
}
func (f *FunctionCaller) Uniform2f(dst Uniform, v0, v1 float32) {
	f.Ctx.Call("uniform2f", js.Value(dst), v0, v1)
}
func (f *FunctionCaller) Uniform3f(dst Uniform, v0, v1, v2 float32) {
	f.Ctx.Call("uniform3f", js.Value(dst), v0, v1, v2)
}
func (f *FunctionCaller) Uniform4f(dst Uniform, v0, v1, v2, v3 float32) {
	f.Ctx.Call("uniform4f", js.Value(dst), v0, v1, v2, v3)
}
func (f *FunctionCaller) UseProgram(p Program) {
	f.Ctx.Call("useProgram", js.Value(p))
}
func (f *FunctionCaller) VertexAttribPointer(dst Attrib, size int, ty Enum, normalized bool, stride, offset int) {
	f.Ctx.Call("vertexAttribPointer", int(dst), size, int(ty), normalized, stride, offset)
}
func (f *FunctionCaller) Viewport(x, y, width, height int) {
	f.Ctx.Call("viewport", x, y, width, height)
}

func (f *FunctionCaller) byteArrayOf(data []byte) js.Value {
	if len(data) == 0 {
		return js.Null()
	}
	f.resizeByteBuffer(len(data))
	ba := f.uint8Array.New(f.arrayBuf, int(0), int(len(data)))
	js.CopyBytesToJS(ba, data)
	return ba
}

func (f *FunctionCaller) resizeByteBuffer(n int) {
	if n == 0 {
		return
	}
	if !f.arrayBuf.IsUndefined() && f.arrayBuf.Length() >= n {
		return
	}
	f.arrayBuf = js.Global().Get("ArrayBuffer").New(n)
}
