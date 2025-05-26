// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func loadGLESv2Procs() error {
	dllName := "libGLESv2.dll"
	handle, err := windows.LoadLibraryEx(dllName, 0, windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	if err != nil {
		return fmt.Errorf("gl: failed to load %s: %v", dllName, err)
	}
	gles := windows.DLL{Handle: handle, Name: dllName}
	// d3dcompiler_47.dll is needed internally for shader compilation to function.
	dllName = "d3dcompiler_47.dll"
	_, err = windows.LoadLibraryEx(dllName, 0, windows.LOAD_LIBRARY_SEARCH_DEFAULT_DIRS)
	if err != nil {
		return fmt.Errorf("gl: failed to load %s: %v", dllName, err)
	}
	procs := map[string]**windows.Proc{
		"glActiveTexture":                       &_glActiveTexture,
		"glAttachShader":                        &_glAttachShader,
		"glBeginQuery":                          &_glBeginQuery,
		"glBindAttribLocation":                  &_glBindAttribLocation,
		"glBindBuffer":                          &_glBindBuffer,
		"glBindBufferBase":                      &_glBindBufferBase,
		"glBindFramebuffer":                     &_glBindFramebuffer,
		"glBindRenderbuffer":                    &_glBindRenderbuffer,
		"glBindTexture":                         &_glBindTexture,
		"glBindVertexArray":                     &_glBindVertexArray,
		"glBlendEquation":                       &_glBlendEquation,
		"glBlendFuncSeparate":                   &_glBlendFuncSeparate,
		"glBufferData":                          &_glBufferData,
		"glBufferSubData":                       &_glBufferSubData,
		"glCheckFramebufferStatus":              &_glCheckFramebufferStatus,
		"glClear":                               &_glClear,
		"glClearColor":                          &_glClearColor,
		"glClearDepthf":                         &_glClearDepthf,
		"glDeleteQueries":                       &_glDeleteQueries,
		"glDeleteVertexArrays":                  &_glDeleteVertexArrays,
		"glCompileShader":                       &_glCompileShader,
		"glCopyTexSubImage2D":                   &_glCopyTexSubImage2D,
		"glGenerateMipmap":                      &_glGenerateMipmap,
		"glGenBuffers":                          &_glGenBuffers,
		"glGenFramebuffers":                     &_glGenFramebuffers,
		"glGenVertexArrays":                     &_glGenVertexArrays,
		"glGetUniformBlockIndex":                &_glGetUniformBlockIndex,
		"glCreateProgram":                       &_glCreateProgram,
		"glGenRenderbuffers":                    &_glGenRenderbuffers,
		"glCreateShader":                        &_glCreateShader,
		"glGenTextures":                         &_glGenTextures,
		"glDeleteBuffers":                       &_glDeleteBuffers,
		"glDeleteFramebuffers":                  &_glDeleteFramebuffers,
		"glDeleteProgram":                       &_glDeleteProgram,
		"glDeleteShader":                        &_glDeleteShader,
		"glDeleteRenderbuffers":                 &_glDeleteRenderbuffers,
		"glDeleteTextures":                      &_glDeleteTextures,
		"glDepthFunc":                           &_glDepthFunc,
		"glDepthMask":                           &_glDepthMask,
		"glDisableVertexAttribArray":            &_glDisableVertexAttribArray,
		"glDisable":                             &_glDisable,
		"glDrawArrays":                          &_glDrawArrays,
		"glDrawElements":                        &_glDrawElements,
		"glEnable":                              &_glEnable,
		"glEnableVertexAttribArray":             &_glEnableVertexAttribArray,
		"glEndQuery":                            &_glEndQuery,
		"glFinish":                              &_glFinish,
		"glFlush":                               &_glFlush,
		"glFramebufferRenderbuffer":             &_glFramebufferRenderbuffer,
		"glFramebufferTexture2D":                &_glFramebufferTexture2D,
		"glGenQueries":                          &_glGenQueries,
		"glGetError":                            &_glGetError,
		"glGetRenderbufferParameteriv":          &_glGetRenderbufferParameteriv,
		"glGetFloatv":                           &_glGetFloatv,
		"glGetFramebufferAttachmentParameteriv": &_glGetFramebufferAttachmentParameteriv,
		"glGetIntegerv":                         &_glGetIntegerv,
		"glGetIntegeri_v":                       &_glGetIntegeri_v,
		"glGetProgramiv":                        &_glGetProgramiv,
		"glGetProgramInfoLog":                   &_glGetProgramInfoLog,
		"glGetQueryObjectuiv":                   &_glGetQueryObjectuiv,
		"glGetShaderiv":                         &_glGetShaderiv,
		"glGetShaderInfoLog":                    &_glGetShaderInfoLog,
		"glGetString":                           &_glGetString,
		"glGetUniformLocation":                  &_glGetUniformLocation,
		"glGetVertexAttribiv":                   &_glGetVertexAttribiv,
		"glGetVertexAttribPointerv":             &_glGetVertexAttribPointerv,
		"glInvalidateFramebuffer":               &_glInvalidateFramebuffer,
		"glIsEnabled":                           &_glIsEnabled,
		"glLinkProgram":                         &_glLinkProgram,
		"glPixelStorei":                         &_glPixelStorei,
		"glReadPixels":                          &_glReadPixels,
		"glRenderbufferStorage":                 &_glRenderbufferStorage,
		"glScissor":                             &_glScissor,
		"glShaderSource":                        &_glShaderSource,
		"glTexImage2D":                          &_glTexImage2D,
		"glTexStorage2D":                        &_glTexStorage2D,
		"glTexSubImage2D":                       &_glTexSubImage2D,
		"glTexParameteri":                       &_glTexParameteri,
		"glUniformBlockBinding":                 &_glUniformBlockBinding,
		"glUniform1f":                           &_glUniform1f,
		"glUniform1i":                           &_glUniform1i,
		"glUniform2f":                           &_glUniform2f,
		"glUniform3f":                           &_glUniform3f,
		"glUniform4f":                           &_glUniform4f,
		"glUseProgram":                          &_glUseProgram,
		"glVertexAttribPointer":                 &_glVertexAttribPointer,
		"glViewport":                            &_glViewport,
	}
	for name, proc := range procs {
		p, err := gles.FindProc(name)
		if err != nil {
			return fmt.Errorf("failed to locate %s in %s: %w", name, gles.Name, err)
		}
		*proc = p
	}
	return nil
}

var (
	glInitOnce                             sync.Once
	_glActiveTexture                       *windows.Proc
	_glAttachShader                        *windows.Proc
	_glBeginQuery                          *windows.Proc
	_glBindAttribLocation                  *windows.Proc
	_glBindBuffer                          *windows.Proc
	_glBindBufferBase                      *windows.Proc
	_glBindFramebuffer                     *windows.Proc
	_glBindRenderbuffer                    *windows.Proc
	_glBindTexture                         *windows.Proc
	_glBindVertexArray                     *windows.Proc
	_glBlendEquation                       *windows.Proc
	_glBlendFuncSeparate                   *windows.Proc
	_glBufferData                          *windows.Proc
	_glBufferSubData                       *windows.Proc
	_glCheckFramebufferStatus              *windows.Proc
	_glClear                               *windows.Proc
	_glClearColor                          *windows.Proc
	_glClearDepthf                         *windows.Proc
	_glDeleteQueries                       *windows.Proc
	_glDeleteVertexArrays                  *windows.Proc
	_glCompileShader                       *windows.Proc
	_glCopyTexSubImage2D                   *windows.Proc
	_glGenerateMipmap                      *windows.Proc
	_glGenBuffers                          *windows.Proc
	_glGenFramebuffers                     *windows.Proc
	_glGenVertexArrays                     *windows.Proc
	_glGetUniformBlockIndex                *windows.Proc
	_glCreateProgram                       *windows.Proc
	_glGenRenderbuffers                    *windows.Proc
	_glCreateShader                        *windows.Proc
	_glGenTextures                         *windows.Proc
	_glDeleteBuffers                       *windows.Proc
	_glDeleteFramebuffers                  *windows.Proc
	_glDeleteProgram                       *windows.Proc
	_glDeleteShader                        *windows.Proc
	_glDeleteRenderbuffers                 *windows.Proc
	_glDeleteTextures                      *windows.Proc
	_glDepthFunc                           *windows.Proc
	_glDepthMask                           *windows.Proc
	_glDisableVertexAttribArray            *windows.Proc
	_glDisable                             *windows.Proc
	_glDrawArrays                          *windows.Proc
	_glDrawElements                        *windows.Proc
	_glEnable                              *windows.Proc
	_glEnableVertexAttribArray             *windows.Proc
	_glEndQuery                            *windows.Proc
	_glFinish                              *windows.Proc
	_glFlush                               *windows.Proc
	_glFramebufferRenderbuffer             *windows.Proc
	_glFramebufferTexture2D                *windows.Proc
	_glGenQueries                          *windows.Proc
	_glGetError                            *windows.Proc
	_glGetRenderbufferParameteriv          *windows.Proc
	_glGetFloatv                           *windows.Proc
	_glGetFramebufferAttachmentParameteriv *windows.Proc
	_glGetIntegerv                         *windows.Proc
	_glGetIntegeri_v                       *windows.Proc
	_glGetProgramiv                        *windows.Proc
	_glGetProgramInfoLog                   *windows.Proc
	_glGetQueryObjectuiv                   *windows.Proc
	_glGetShaderiv                         *windows.Proc
	_glGetShaderInfoLog                    *windows.Proc
	_glGetString                           *windows.Proc
	_glGetUniformLocation                  *windows.Proc
	_glGetVertexAttribiv                   *windows.Proc
	_glGetVertexAttribPointerv             *windows.Proc
	_glInvalidateFramebuffer               *windows.Proc
	_glIsEnabled                           *windows.Proc
	_glLinkProgram                         *windows.Proc
	_glPixelStorei                         *windows.Proc
	_glReadPixels                          *windows.Proc
	_glRenderbufferStorage                 *windows.Proc
	_glScissor                             *windows.Proc
	_glShaderSource                        *windows.Proc
	_glTexImage2D                          *windows.Proc
	_glTexStorage2D                        *windows.Proc
	_glTexSubImage2D                       *windows.Proc
	_glTexParameteri                       *windows.Proc
	_glUniformBlockBinding                 *windows.Proc
	_glUniform1f                           *windows.Proc
	_glUniform1i                           *windows.Proc
	_glUniform2f                           *windows.Proc
	_glUniform3f                           *windows.Proc
	_glUniform4f                           *windows.Proc
	_glUseProgram                          *windows.Proc
	_glVertexAttribPointer                 *windows.Proc
	_glViewport                            *windows.Proc
)

type Functions struct {
	// Query caches.
	int32s   [100]int32
	float32s [100]float32
	uintptrs [100]uintptr
}

type Context any

func NewFunctions(ctx Context, forceES bool) (*Functions, error) {
	if ctx != nil {
		panic("non-nil context")
	}
	var err error
	glInitOnce.Do(func() {
		err = loadGLESv2Procs()
	})
	return new(Functions), err
}

func (c *Functions) ActiveTexture(t Enum) {
	syscall.Syscall(_glActiveTexture.Addr(), 1, uintptr(t), 0, 0)
}

func (c *Functions) AttachShader(p Program, s Shader) {
	syscall.Syscall(_glAttachShader.Addr(), 2, uintptr(p.V), uintptr(s.V), 0)
}

func (f *Functions) BeginQuery(target Enum, query Query) {
	syscall.Syscall(_glBeginQuery.Addr(), 2, uintptr(target), uintptr(query.V), 0)
}

func (c *Functions) BindAttribLocation(p Program, a Attrib, name string) {
	cname := cString(name)
	c0 := &cname[0]
	syscall.Syscall(_glBindAttribLocation.Addr(), 3, uintptr(p.V), uintptr(a), uintptr(unsafe.Pointer(c0)))
	issue34474KeepAlive(c)
}

func (c *Functions) BindBuffer(target Enum, b Buffer) {
	syscall.Syscall(_glBindBuffer.Addr(), 2, uintptr(target), uintptr(b.V), 0)
}

func (c *Functions) BindBufferBase(target Enum, index int, b Buffer) {
	syscall.Syscall(_glBindBufferBase.Addr(), 3, uintptr(target), uintptr(index), uintptr(b.V))
}

func (c *Functions) BindFramebuffer(target Enum, fb Framebuffer) {
	syscall.Syscall(_glBindFramebuffer.Addr(), 2, uintptr(target), uintptr(fb.V), 0)
}

func (c *Functions) BindRenderbuffer(target Enum, rb Renderbuffer) {
	syscall.Syscall(_glBindRenderbuffer.Addr(), 2, uintptr(target), uintptr(rb.V), 0)
}

func (f *Functions) BindImageTexture(unit int, t Texture, level int, layered bool, layer int, access, format Enum) {
	panic("not implemented")
}

func (c *Functions) BindTexture(target Enum, t Texture) {
	syscall.Syscall(_glBindTexture.Addr(), 2, uintptr(target), uintptr(t.V), 0)
}

func (c *Functions) BindVertexArray(a VertexArray) {
	syscall.Syscall(_glBindVertexArray.Addr(), 1, uintptr(a.V), 0, 0)
}

func (c *Functions) BlendEquation(mode Enum) {
	syscall.Syscall(_glBlendEquation.Addr(), 1, uintptr(mode), 0, 0)
}

func (c *Functions) BlendFuncSeparate(srcRGB, dstRGB, srcA, dstA Enum) {
	syscall.Syscall6(_glBlendFuncSeparate.Addr(), 4, uintptr(srcRGB), uintptr(dstRGB), uintptr(srcA), uintptr(dstA), 0, 0)
}

func (c *Functions) BufferData(target Enum, size int, usage Enum, data []byte) {
	var p unsafe.Pointer
	if len(data) > 0 {
		p = unsafe.Pointer(&data[0])
	}
	syscall.Syscall6(_glBufferData.Addr(), 4, uintptr(target), uintptr(size), uintptr(p), uintptr(usage), 0, 0)
}

func (f *Functions) BufferSubData(target Enum, offset int, src []byte) {
	if n := len(src); n > 0 {
		s0 := &src[0]
		syscall.Syscall6(_glBufferSubData.Addr(), 4, uintptr(target), uintptr(offset), uintptr(n), uintptr(unsafe.Pointer(s0)), 0, 0)
		issue34474KeepAlive(s0)
	}
}

func (c *Functions) CheckFramebufferStatus(target Enum) Enum {
	s, _, _ := syscall.Syscall(_glCheckFramebufferStatus.Addr(), 1, uintptr(target), 0, 0)
	return Enum(s)
}

func (c *Functions) Clear(mask Enum) {
	syscall.Syscall(_glClear.Addr(), 1, uintptr(mask), 0, 0)
}

func (c *Functions) ClearColor(red, green, blue, alpha float32) {
	syscall.Syscall6(_glClearColor.Addr(), 4, uintptr(math.Float32bits(red)), uintptr(math.Float32bits(green)), uintptr(math.Float32bits(blue)), uintptr(math.Float32bits(alpha)), 0, 0)
}

func (c *Functions) ClearDepthf(d float32) {
	syscall.Syscall(_glClearDepthf.Addr(), 1, uintptr(math.Float32bits(d)), 0, 0)
}

func (c *Functions) CompileShader(s Shader) {
	syscall.Syscall(_glCompileShader.Addr(), 1, uintptr(s.V), 0, 0)
}

func (f *Functions) CopyTexSubImage2D(target Enum, level, xoffset, yoffset, x, y, width, height int) {
	syscall.Syscall9(_glCopyTexSubImage2D.Addr(), 8, uintptr(target), uintptr(level), uintptr(xoffset), uintptr(yoffset), uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0)
}

func (f *Functions) GenerateMipmap(target Enum) {
	syscall.Syscall(_glGenerateMipmap.Addr(), 1, uintptr(target), 0, 0)
}

func (c *Functions) CreateBuffer() Buffer {
	var buf uintptr
	syscall.Syscall(_glGenBuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&buf)), 0)
	return Buffer{uint(buf)}
}

func (c *Functions) CreateFramebuffer() Framebuffer {
	var fb uintptr
	syscall.Syscall(_glGenFramebuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&fb)), 0)
	return Framebuffer{uint(fb)}
}

func (c *Functions) CreateProgram() Program {
	p, _, _ := syscall.Syscall(_glCreateProgram.Addr(), 0, 0, 0, 0)
	return Program{uint(p)}
}

func (f *Functions) CreateQuery() Query {
	var q uintptr
	syscall.Syscall(_glGenQueries.Addr(), 2, 1, uintptr(unsafe.Pointer(&q)), 0)
	return Query{uint(q)}
}

func (c *Functions) CreateRenderbuffer() Renderbuffer {
	var rb uintptr
	syscall.Syscall(_glGenRenderbuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&rb)), 0)
	return Renderbuffer{uint(rb)}
}

func (c *Functions) CreateShader(ty Enum) Shader {
	s, _, _ := syscall.Syscall(_glCreateShader.Addr(), 1, uintptr(ty), 0, 0)
	return Shader{uint(s)}
}

func (c *Functions) CreateTexture() Texture {
	var t uintptr
	syscall.Syscall(_glGenTextures.Addr(), 2, 1, uintptr(unsafe.Pointer(&t)), 0)
	return Texture{uint(t)}
}

func (c *Functions) CreateVertexArray() VertexArray {
	var t uintptr
	syscall.Syscall(_glGenVertexArrays.Addr(), 2, 1, uintptr(unsafe.Pointer(&t)), 0)
	return VertexArray{uint(t)}
}

func (c *Functions) DeleteBuffer(v Buffer) {
	syscall.Syscall(_glDeleteBuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&v)), 0)
}

func (c *Functions) DeleteFramebuffer(v Framebuffer) {
	syscall.Syscall(_glDeleteFramebuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&v.V)), 0)
}

func (c *Functions) DeleteProgram(p Program) {
	syscall.Syscall(_glDeleteProgram.Addr(), 1, uintptr(p.V), 0, 0)
}

func (f *Functions) DeleteQuery(query Query) {
	syscall.Syscall(_glDeleteQueries.Addr(), 2, 1, uintptr(unsafe.Pointer(&query.V)), 0)
}

func (c *Functions) DeleteShader(s Shader) {
	syscall.Syscall(_glDeleteShader.Addr(), 1, uintptr(s.V), 0, 0)
}

func (c *Functions) DeleteRenderbuffer(v Renderbuffer) {
	syscall.Syscall(_glDeleteRenderbuffers.Addr(), 2, 1, uintptr(unsafe.Pointer(&v.V)), 0)
}

func (c *Functions) DeleteTexture(v Texture) {
	syscall.Syscall(_glDeleteTextures.Addr(), 2, 1, uintptr(unsafe.Pointer(&v.V)), 0)
}

func (f *Functions) DeleteVertexArray(array VertexArray) {
	syscall.Syscall(_glDeleteVertexArrays.Addr(), 2, 1, uintptr(unsafe.Pointer(&array.V)), 0)
}

func (c *Functions) DepthFunc(f Enum) {
	syscall.Syscall(_glDepthFunc.Addr(), 1, uintptr(f), 0, 0)
}

func (c *Functions) DepthMask(mask bool) {
	var m uintptr
	if mask {
		m = 1
	}
	syscall.Syscall(_glDepthMask.Addr(), 1, m, 0, 0)
}

func (c *Functions) DisableVertexAttribArray(a Attrib) {
	syscall.Syscall(_glDisableVertexAttribArray.Addr(), 1, uintptr(a), 0, 0)
}

func (c *Functions) Disable(cap Enum) {
	syscall.Syscall(_glDisable.Addr(), 1, uintptr(cap), 0, 0)
}

func (c *Functions) DrawArrays(mode Enum, first, count int) {
	syscall.Syscall(_glDrawArrays.Addr(), 3, uintptr(mode), uintptr(first), uintptr(count))
}

func (c *Functions) DrawElements(mode Enum, count int, ty Enum, offset int) {
	syscall.Syscall6(_glDrawElements.Addr(), 4, uintptr(mode), uintptr(count), uintptr(ty), uintptr(offset), 0, 0)
}

func (f *Functions) DispatchCompute(x, y, z int) {
	panic("not implemented")
}

func (c *Functions) Enable(cap Enum) {
	syscall.Syscall(_glEnable.Addr(), 1, uintptr(cap), 0, 0)
}

func (c *Functions) EnableVertexAttribArray(a Attrib) {
	syscall.Syscall(_glEnableVertexAttribArray.Addr(), 1, uintptr(a), 0, 0)
}

func (f *Functions) EndQuery(target Enum) {
	syscall.Syscall(_glEndQuery.Addr(), 1, uintptr(target), 0, 0)
}

func (c *Functions) Finish() {
	syscall.Syscall(_glFinish.Addr(), 0, 0, 0, 0)
}

func (c *Functions) Flush() {
	syscall.Syscall(_glFlush.Addr(), 0, 0, 0, 0)
}

func (c *Functions) FramebufferRenderbuffer(target, attachment, renderbuffertarget Enum, renderbuffer Renderbuffer) {
	syscall.Syscall6(_glFramebufferRenderbuffer.Addr(), 4, uintptr(target), uintptr(attachment), uintptr(renderbuffertarget), uintptr(renderbuffer.V), 0, 0)
}

func (c *Functions) FramebufferTexture2D(target, attachment, texTarget Enum, t Texture, level int) {
	syscall.Syscall6(_glFramebufferTexture2D.Addr(), 5, uintptr(target), uintptr(attachment), uintptr(texTarget), uintptr(t.V), uintptr(level), 0)
}

func (f *Functions) GetUniformBlockIndex(p Program, name string) uint {
	cname := cString(name)
	c0 := &cname[0]
	u, _, _ := syscall.Syscall(_glGetUniformBlockIndex.Addr(), 2, uintptr(p.V), uintptr(unsafe.Pointer(c0)), 0)
	issue34474KeepAlive(c0)
	return uint(u)
}

func (c *Functions) GetBinding(pname Enum) Object {
	return Object{uint(c.GetInteger(pname))}
}

func (c *Functions) GetBindingi(pname Enum, idx int) Object {
	return Object{uint(c.GetIntegeri(pname, idx))}
}

func (c *Functions) GetError() Enum {
	e, _, _ := syscall.Syscall(_glGetError.Addr(), 0, 0, 0, 0)
	return Enum(e)
}

func (c *Functions) GetRenderbufferParameteri(target, pname Enum) int {
	syscall.Syscall(_glGetRenderbufferParameteriv.Addr(), 3, uintptr(target), uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])))
	return int(c.int32s[0])
}

func (c *Functions) GetFramebufferAttachmentParameteri(target, attachment, pname Enum) int {
	syscall.Syscall6(_glGetFramebufferAttachmentParameteriv.Addr(), 4, uintptr(target), uintptr(attachment), uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])), 0, 0)
	return int(c.int32s[0])
}

func (c *Functions) GetInteger4(pname Enum) [4]int {
	syscall.Syscall(_glGetIntegerv.Addr(), 2, uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])), 0)
	var r [4]int
	for i := range r {
		r[i] = int(c.int32s[i])
	}
	return r
}

func (c *Functions) GetInteger(pname Enum) int {
	syscall.Syscall(_glGetIntegerv.Addr(), 2, uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])), 0)
	return int(c.int32s[0])
}

func (c *Functions) GetIntegeri(pname Enum, idx int) int {
	syscall.Syscall(_glGetIntegeri_v.Addr(), 3, uintptr(pname), uintptr(idx), uintptr(unsafe.Pointer(&c.int32s[0])))
	return int(c.int32s[0])
}

func (c *Functions) GetFloat(pname Enum) float32 {
	syscall.Syscall(_glGetFloatv.Addr(), 2, uintptr(pname), uintptr(unsafe.Pointer(&c.float32s[0])), 0)
	return c.float32s[0]
}

func (c *Functions) GetFloat4(pname Enum) [4]float32 {
	syscall.Syscall(_glGetFloatv.Addr(), 2, uintptr(pname), uintptr(unsafe.Pointer(&c.float32s[0])), 0)
	var r [4]float32
	copy(r[:], c.float32s[:])
	return r
}

func (c *Functions) GetProgrami(p Program, pname Enum) int {
	syscall.Syscall(_glGetProgramiv.Addr(), 3, uintptr(p.V), uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])))
	return int(c.int32s[0])
}

func (c *Functions) GetProgramInfoLog(p Program) string {
	n := c.GetProgrami(p, INFO_LOG_LENGTH)
	if n == 0 {
		return ""
	}
	buf := make([]byte, n)
	syscall.Syscall6(_glGetProgramInfoLog.Addr(), 4, uintptr(p.V), uintptr(len(buf)), 0, uintptr(unsafe.Pointer(&buf[0])), 0, 0)
	return string(buf)
}

func (c *Functions) GetQueryObjectuiv(query Query, pname Enum) uint {
	syscall.Syscall(_glGetQueryObjectuiv.Addr(), 3, uintptr(query.V), uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])))
	return uint(c.int32s[0])
}

func (c *Functions) GetShaderi(s Shader, pname Enum) int {
	syscall.Syscall(_glGetShaderiv.Addr(), 3, uintptr(s.V), uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])))
	return int(c.int32s[0])
}

func (c *Functions) GetShaderInfoLog(s Shader) string {
	n := c.GetShaderi(s, INFO_LOG_LENGTH)
	buf := make([]byte, n)
	syscall.Syscall6(_glGetShaderInfoLog.Addr(), 4, uintptr(s.V), uintptr(len(buf)), 0, uintptr(unsafe.Pointer(&buf[0])), 0, 0)
	return string(buf)
}

func (c *Functions) GetString(pname Enum) string {
	s, _, _ := syscall.Syscall(_glGetString.Addr(), 1, uintptr(pname), 0, 0)
	return windows.BytePtrToString((*byte)(unsafe.Pointer(s)))
}

func (c *Functions) GetUniformLocation(p Program, name string) Uniform {
	cname := cString(name)
	c0 := &cname[0]
	u, _, _ := syscall.Syscall(_glGetUniformLocation.Addr(), 2, uintptr(p.V), uintptr(unsafe.Pointer(c0)), 0)
	issue34474KeepAlive(c0)
	return Uniform{int(u)}
}

func (c *Functions) GetVertexAttrib(index int, pname Enum) int {
	syscall.Syscall(_glGetVertexAttribiv.Addr(), 3, uintptr(index), uintptr(pname), uintptr(unsafe.Pointer(&c.int32s[0])))
	return int(c.int32s[0])
}

func (c *Functions) GetVertexAttribBinding(index int, pname Enum) Object {
	return Object{uint(c.GetVertexAttrib(index, pname))}
}

func (c *Functions) GetVertexAttribPointer(index int, pname Enum) uintptr {
	syscall.Syscall(_glGetVertexAttribPointerv.Addr(), 3, uintptr(index), uintptr(pname), uintptr(unsafe.Pointer(&c.uintptrs[0])))
	return c.uintptrs[0]
}

func (c *Functions) InvalidateFramebuffer(target, attachment Enum) {
	addr := _glInvalidateFramebuffer.Addr()
	if addr == 0 {
		// InvalidateFramebuffer is just a hint. Skip it if not supported.
		return
	}
	syscall.Syscall(addr, 3, uintptr(target), 1, uintptr(unsafe.Pointer(&attachment)))
}

func (f *Functions) IsEnabled(cap Enum) bool {
	u, _, _ := syscall.Syscall(_glIsEnabled.Addr(), 1, uintptr(cap), 0, 0)
	return u == TRUE
}

func (c *Functions) LinkProgram(p Program) {
	syscall.Syscall(_glLinkProgram.Addr(), 1, uintptr(p.V), 0, 0)
}

func (c *Functions) PixelStorei(pname Enum, param int) {
	syscall.Syscall(_glPixelStorei.Addr(), 2, uintptr(pname), uintptr(param), 0)
}

func (f *Functions) MemoryBarrier(barriers Enum) {
	panic("not implemented")
}

func (f *Functions) MapBufferRange(target Enum, offset, length int, access Enum) []byte {
	panic("not implemented")
}

func (f *Functions) ReadPixels(x, y, width, height int, format, ty Enum, data []byte) {
	d0 := &data[0]
	syscall.Syscall9(_glReadPixels.Addr(), 7, uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(format), uintptr(ty), uintptr(unsafe.Pointer(d0)), 0, 0)
	issue34474KeepAlive(d0)
}

func (c *Functions) RenderbufferStorage(target, internalformat Enum, width, height int) {
	syscall.Syscall6(_glRenderbufferStorage.Addr(), 4, uintptr(target), uintptr(internalformat), uintptr(width), uintptr(height), 0, 0)
}

func (c *Functions) Scissor(x, y, width, height int32) {
	syscall.Syscall6(_glScissor.Addr(), 4, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0)
}

func (c *Functions) ShaderSource(s Shader, src string) {
	var n uintptr = uintptr(len(src))
	psrc := &src
	syscall.Syscall6(_glShaderSource.Addr(), 4, uintptr(s.V), 1, uintptr(unsafe.Pointer(psrc)), uintptr(unsafe.Pointer(&n)), 0, 0)
	issue34474KeepAlive(psrc)
}

func (f *Functions) TexImage2D(target Enum, level int, internalFormat Enum, width int, height int, format Enum, ty Enum) {
	syscall.Syscall9(_glTexImage2D.Addr(), 9, uintptr(target), uintptr(level), uintptr(internalFormat), uintptr(width), uintptr(height), 0, uintptr(format), uintptr(ty), 0)
}

func (f *Functions) TexStorage2D(target Enum, levels int, internalFormat Enum, width, height int) {
	syscall.Syscall6(_glTexStorage2D.Addr(), 5, uintptr(target), uintptr(levels), uintptr(internalFormat), uintptr(width), uintptr(height), 0)
}

func (c *Functions) TexSubImage2D(target Enum, level int, x, y, width, height int, format, ty Enum, data []byte) {
	d0 := &data[0]
	syscall.Syscall9(_glTexSubImage2D.Addr(), 9, uintptr(target), uintptr(level), uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(format), uintptr(ty), uintptr(unsafe.Pointer(d0)))
	issue34474KeepAlive(d0)
}

func (c *Functions) TexParameteri(target, pname Enum, param int) {
	syscall.Syscall(_glTexParameteri.Addr(), 3, uintptr(target), uintptr(pname), uintptr(param))
}

func (f *Functions) UniformBlockBinding(p Program, uniformBlockIndex uint, uniformBlockBinding uint) {
	syscall.Syscall(_glUniformBlockBinding.Addr(), 3, uintptr(p.V), uintptr(uniformBlockIndex), uintptr(uniformBlockBinding))
}

func (c *Functions) Uniform1f(dst Uniform, v float32) {
	syscall.Syscall(_glUniform1f.Addr(), 2, uintptr(dst.V), uintptr(math.Float32bits(v)), 0)
}

func (c *Functions) Uniform1i(dst Uniform, v int) {
	syscall.Syscall(_glUniform1i.Addr(), 2, uintptr(dst.V), uintptr(v), 0)
}

func (c *Functions) Uniform2f(dst Uniform, v0, v1 float32) {
	syscall.Syscall(_glUniform2f.Addr(), 3, uintptr(dst.V), uintptr(math.Float32bits(v0)), uintptr(math.Float32bits(v1)))
}

func (c *Functions) Uniform3f(dst Uniform, v0, v1, v2 float32) {
	syscall.Syscall6(_glUniform3f.Addr(), 4, uintptr(dst.V), uintptr(math.Float32bits(v0)), uintptr(math.Float32bits(v1)), uintptr(math.Float32bits(v2)), 0, 0)
}

func (c *Functions) Uniform4f(dst Uniform, v0, v1, v2, v3 float32) {
	syscall.Syscall6(_glUniform4f.Addr(), 5, uintptr(dst.V), uintptr(math.Float32bits(v0)), uintptr(math.Float32bits(v1)), uintptr(math.Float32bits(v2)), uintptr(math.Float32bits(v3)), 0)
}

func (c *Functions) UseProgram(p Program) {
	syscall.Syscall(_glUseProgram.Addr(), 1, uintptr(p.V), 0, 0)
}

func (f *Functions) UnmapBuffer(target Enum) bool {
	panic("not implemented")
}

func (c *Functions) VertexAttribPointer(dst Attrib, size int, ty Enum, normalized bool, stride, offset int) {
	var norm uintptr
	if normalized {
		norm = 1
	}
	syscall.Syscall6(_glVertexAttribPointer.Addr(), 6, uintptr(dst), uintptr(size), uintptr(ty), norm, uintptr(stride), uintptr(offset))
}

func (c *Functions) Viewport(x, y, width, height int) {
	syscall.Syscall6(_glViewport.Addr(), 4, uintptr(x), uintptr(y), uintptr(width), uintptr(height), 0, 0)
}

func cString(s string) []byte {
	b := make([]byte, len(s)+1)
	copy(b, s)
	return b
}

// issue34474KeepAlive calls runtime.KeepAlive as a
// workaround for golang.org/issue/34474.
func issue34474KeepAlive(v any) {
	runtime.KeepAlive(v)
}
