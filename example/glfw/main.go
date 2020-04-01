// SPDX-License-Identifier: Unlicense OR MIT

// package glfw doesn't build on OpenBSD and FreeBSD.
// +build !openbsd,!freebsd,!windows,!android,!ios,!js

package main

import (
	"image"
	"log"
	"math"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu"
	giogl "gioui.org/gpu/gl"
	"gioui.org/io/pointer"
	"gioui.org/io/router"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type glfwConfig struct {
	Scale float32
}

type goglFunctions struct {
}

func main() {
	// Required by the OpenGL threading model.
	runtime.LockOSThread()

	err := glfw.Init()
	if err != nil {
		log.Fatal(err)
	}
	defer glfw.Terminate()
	// Gio assumes a sRGB backbuffer.
	glfw.WindowHint(glfw.SRGBCapable, glfw.True)

	window, err := glfw.CreateWindow(800, 600, "Gio + GLFW", nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		log.Fatal(err)
	}
	// Enable sRGB.
	gl.Enable(gl.FRAMEBUFFER_SRGB)

	gofont.Register()
	f := new(goglFunctions)
	var queue router.Router
	gtx := layout.NewContext(&queue)
	th := material.NewTheme()
	backend, err := giogl.NewBackend(f)
	if err != nil {
		log.Fatal(err)
	}
	gpu, err := gpu.New(backend)
	if err != nil {
		log.Fatal(err)
	}

	registerCallbacks(window, &queue)
	for !window.ShouldClose() {
		glfw.PollEvents()
		scale := float32(1.0)
		if monitor := window.GetMonitor(); monitor != nil {
			scalex, _ := window.GetMonitor().GetContentScale()
			scale = scalex
		}
		width, height := window.GetSize()
		sz := image.Point{X: width, Y: height}
		gtx.Reset(&glfwConfig{scale}, sz)
		draw(gtx, th)
		gpu.Collect(sz, gtx.Ops)
		gpu.BeginFrame()
		queue.Frame(gtx.Ops)
		gpu.EndFrame()
		window.SwapBuffers()
	}
}

var button widget.Button

func draw(gtx *layout.Context, th *material.Theme) {
	layout.Center.Layout(gtx, func() {
		th.Button("Button").Layout(gtx, &button)
	})
}

func registerCallbacks(window *glfw.Window, q *router.Router) {
	var btns pointer.Buttons
	beginning := time.Now()
	var lastPos f32.Point
	window.SetCursorPosCallback(func(w *glfw.Window, xpos float64, ypos float64) {
		lastPos = f32.Point{X: float32(xpos), Y: float32(ypos)}
		q.Add(pointer.Event{
			Type:     pointer.Move,
			Position: lastPos,
			Source:   pointer.Mouse,
			Time:     time.Since(beginning),
			Buttons:  btns,
		})
	})
	window.SetMouseButtonCallback(func(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
		var btn pointer.Buttons
		switch button {
		case glfw.MouseButton1:
			btn = pointer.ButtonLeft
		case glfw.MouseButton2:
			btn = pointer.ButtonRight
		case glfw.MouseButton3:
			btn = pointer.ButtonMiddle
		}
		var typ pointer.Type
		switch action {
		case glfw.Release:
			typ = pointer.Release
			btns &^= btn
		case glfw.Press:
			typ = pointer.Press
			btns |= btn
		}
		q.Add(pointer.Event{
			Type:     typ,
			Source:   pointer.Mouse,
			Time:     time.Since(beginning),
			Position: lastPos,
			Buttons:  btns,
		})
	})
}

func (s *glfwConfig) Now() time.Time {
	return time.Now()
}

func (s *glfwConfig) Px(v unit.Value) int {
	scale := s.Scale
	if v.U == unit.UnitPx {
		scale = 1
	}
	return int(math.Round(float64(scale * v.V)))
}

func (f *goglFunctions) ActiveTexture(texture giogl.Enum) {
	gl.ActiveTexture(uint32(texture))
}

func (f *goglFunctions) AttachShader(p giogl.Program, s giogl.Shader) {
	gl.AttachShader(uint32(p.V), uint32(s.V))
}

func (f *goglFunctions) BeginQuery(target giogl.Enum, query giogl.Query) {
	gl.BeginQuery(uint32(target), uint32(query.V))
}

func (f *goglFunctions) BindAttribLocation(p giogl.Program, a giogl.Attrib, name string) {
	gl.BindAttribLocation(uint32(p.V), uint32(a), gl.Str(name+"\x00"))
}

func (f *goglFunctions) BindBuffer(target giogl.Enum, b giogl.Buffer) {
	gl.BindBuffer(uint32(target), uint32(b.V))
}

func (f *goglFunctions) BindBufferBase(target giogl.Enum, index int, b giogl.Buffer) {
	gl.BindBufferBase(uint32(target), uint32(index), uint32(b.V))
}

func (f *goglFunctions) BindFramebuffer(target giogl.Enum, fb giogl.Framebuffer) {
	gl.BindFramebuffer(uint32(target), uint32(fb.V))
}

func (f *goglFunctions) BindRenderbuffer(target giogl.Enum, rb giogl.Renderbuffer) {
	gl.BindRenderbuffer(uint32(target), uint32(rb.V))
}

func (f *goglFunctions) BindTexture(target giogl.Enum, t giogl.Texture) {
	gl.BindTexture(uint32(target), uint32(t.V))
}

func (f *goglFunctions) BlendEquation(mode giogl.Enum) {
	gl.BlendEquation(uint32(mode))
}

func (f *goglFunctions) BlendFunc(sfactor, dfactor giogl.Enum) {
	gl.BlendFunc(uint32(sfactor), uint32(dfactor))
}

func (f *goglFunctions) BufferData(target giogl.Enum, src []byte, usage giogl.Enum) {
	gl.BufferData(uint32(target), len(src), gl.Ptr(src), uint32(usage))
}

func (f *goglFunctions) CheckFramebufferStatus(target giogl.Enum) giogl.Enum {
	return giogl.Enum(gl.CheckFramebufferStatus(uint32(target)))
}

func (f *goglFunctions) Clear(mask giogl.Enum) {
	gl.Clear(uint32(mask))
}

func (f *goglFunctions) ClearColor(red, green, blue, alpha float32) {
	gl.ClearColor(red, green, blue, alpha)
}

func (f *goglFunctions) ClearDepthf(d float32) {
	gl.ClearDepthf(d)
}

func (f *goglFunctions) CompileShader(s giogl.Shader) {
	gl.CompileShader(uint32(s.V))
}

func (f *goglFunctions) CreateBuffer() giogl.Buffer {
	var buf uint32
	gl.GenBuffers(1, &buf)
	return giogl.Buffer{uint(buf)}
}

func (f *goglFunctions) CreateFramebuffer() giogl.Framebuffer {
	var fb uint32
	gl.GenFramebuffers(1, &fb)
	return giogl.Framebuffer{uint(fb)}
}

func (f *goglFunctions) CreateProgram() giogl.Program {
	return giogl.Program{uint(gl.CreateProgram())}
}

func (f *goglFunctions) CreateQuery() giogl.Query {
	var q uint32
	gl.GenQueries(1, &q)
	return giogl.Query{uint(q)}
}

func (f *goglFunctions) CreateRenderbuffer() giogl.Renderbuffer {
	var rb uint32
	gl.GenRenderbuffers(1, &rb)
	return giogl.Renderbuffer{uint(rb)}
}

func (f *goglFunctions) CreateShader(ty giogl.Enum) giogl.Shader {
	return giogl.Shader{uint(gl.CreateShader(uint32(ty)))}
}

func (f *goglFunctions) CreateTexture() giogl.Texture {
	var t uint32
	gl.GenTextures(1, &t)
	return giogl.Texture{uint(t)}
}

func (f *goglFunctions) DeleteBuffer(v giogl.Buffer) {
	buf := uint32(v.V)
	gl.DeleteBuffers(1, &buf)
}

func (f *goglFunctions) DeleteFramebuffer(v giogl.Framebuffer) {
	fb := uint32(v.V)
	gl.DeleteFramebuffers(1, &fb)
}

func (f *goglFunctions) DeleteProgram(p giogl.Program) {
	gl.DeleteProgram(uint32(p.V))
}

func (f *goglFunctions) DeleteQuery(query giogl.Query) {
	q := uint32(query.V)
	gl.DeleteQueries(1, &q)
}

func (f *goglFunctions) DeleteRenderbuffer(rb giogl.Renderbuffer) {
	r := uint32(rb.V)
	gl.DeleteRenderbuffers(1, &r)
}

func (f *goglFunctions) DeleteShader(s giogl.Shader) {
	gl.DeleteShader(uint32(s.V))
}

func (f *goglFunctions) DeleteTexture(v giogl.Texture) {
	t := uint32(v.V)
	gl.DeleteTextures(1, &t)
}

func (f *goglFunctions) DepthFunc(d giogl.Enum) {
	gl.DepthFunc(uint32(d))
}

func (f *goglFunctions) DepthMask(mask bool) {
	gl.DepthMask(mask)
}

func (f *goglFunctions) DisableVertexAttribArray(a giogl.Attrib) {
	gl.DisableVertexAttribArray(uint32(a))
}

func (f *goglFunctions) Disable(cap giogl.Enum) {
	gl.Disable(uint32(cap))
}

func (f *goglFunctions) DrawArrays(mode giogl.Enum, first, count int) {
	gl.DrawArrays(uint32(mode), int32(first), int32(count))
}

func (f *goglFunctions) DrawElements(mode giogl.Enum, count int, ty giogl.Enum, offset int) {
	gl.DrawElements(uint32(mode), int32(count), uint32(ty), unsafe.Pointer(uintptr(offset)))
}

func (f *goglFunctions) Enable(cap giogl.Enum) {
	gl.Enable(uint32(cap))
}

func (f *goglFunctions) EnableVertexAttribArray(a giogl.Attrib) {
	gl.EnableVertexAttribArray(uint32(a))
}

func (f *goglFunctions) EndQuery(target giogl.Enum) {
	gl.EndQuery(uint32(target))
}

func (f *goglFunctions) FramebufferRenderbuffer(target, attachment, renderbuffertarget giogl.Enum, renderbuffer giogl.Renderbuffer) {
	gl.FramebufferRenderbuffer(uint32(target), uint32(attachment), uint32(renderbuffertarget), uint32(renderbuffer.V))
}

func (f *goglFunctions) FramebufferTexture2D(target, attachment, texTarget giogl.Enum, t giogl.Texture, level int) {
	gl.FramebufferTexture2D(uint32(target), uint32(attachment), uint32(texTarget), uint32(t.V), int32(level))
}

func (f *goglFunctions) GetBinding(pname giogl.Enum) giogl.Object {
	var o int32
	gl.GetIntegerv(uint32(pname), &o)
	return giogl.Object{uint(o)}
}

func (f *goglFunctions) GetError() giogl.Enum {
	return giogl.Enum(gl.GetError())
}

func (f *goglFunctions) GetInteger(pname giogl.Enum) int {
	var p [100]int32
	gl.GetIntegerv(uint32(pname), &p[0])
	return int(p[0])
}

func (f *goglFunctions) GetProgrami(p giogl.Program, pname giogl.Enum) int {
	var params [100]int32
	gl.GetProgramiv(uint32(p.V), uint32(pname), &params[0])
	return int(params[0])
}

func (f *goglFunctions) GetProgramInfoLog(p giogl.Program) string {
	var logLength int32
	gl.GetProgramiv(uint32(p.V), gl.INFO_LOG_LENGTH, &logLength)
	log := strings.Repeat("\x00", int(logLength+1))
	gl.GetProgramInfoLog(uint32(p.V), logLength, nil, gl.Str(log))
	return log[:logLength]
}

func (f *goglFunctions) GetQueryObjectuiv(query giogl.Query, pname giogl.Enum) uint {
	var i uint32
	gl.GetQueryObjectuiv(uint32(query.V), uint32(pname), &i)
	return uint(i)
}

func (f *goglFunctions) GetShaderi(s giogl.Shader, pname giogl.Enum) int {
	var i int32
	gl.GetShaderiv(uint32(s.V), uint32(pname), &i)
	return int(i)
}

func (f *goglFunctions) GetShaderInfoLog(s giogl.Shader) string {
	var logLength int32
	gl.GetShaderiv(uint32(s.V), gl.INFO_LOG_LENGTH, &logLength)
	log := strings.Repeat("\x00", int(logLength+1))
	gl.GetShaderInfoLog(uint32(s.V), logLength, nil, gl.Str(log))
	return log[:logLength]
}

func (f *goglFunctions) GetString(pname giogl.Enum) string {
	switch {
	case pname == giogl.EXTENSIONS:
		// OpenGL 3 core profile doesn't support glGetString(GL_EXTENSIONS).
		// Use glGetStringi(GL_EXTENSIONS, <index>).
		var exts []string
		nexts := f.GetInteger(gl.NUM_EXTENSIONS)
		for i := 0; i < nexts; i++ {
			ext := gl.GetStringi(gl.EXTENSIONS, uint32(i))
			exts = append(exts, gl.GoStr(ext))
		}
		return strings.Join(exts, " ")
	default:
		return gl.GoStr(gl.GetString(uint32(pname)))
	}
}

func (f *goglFunctions) GetUniformBlockIndex(p giogl.Program, name string) uint {
	return uint(gl.GetUniformBlockIndex(uint32(p.V), gl.Str(name+"\x00")))
}

func (f *goglFunctions) GetUniformLocation(p giogl.Program, name string) giogl.Uniform {
	return giogl.Uniform{int(gl.GetUniformLocation(uint32(p.V), gl.Str(name+"\x00")))}
}

func (f *goglFunctions) InvalidateFramebuffer(target, attachment giogl.Enum) {
	// Doesn't exist in OpenGL Core.
}

func (f *goglFunctions) LinkProgram(p giogl.Program) {
	gl.LinkProgram(uint32(p.V))
}

func (f *goglFunctions) ReadPixels(x, y, width, height int, format, ty giogl.Enum, data []byte) {
	gl.ReadPixels(int32(x), int32(y), int32(width), int32(height), uint32(format), uint32(ty), unsafe.Pointer(&data[0]))
}

func (f *goglFunctions) RenderbufferStorage(target, internalformat giogl.Enum, width, height int) {
	gl.RenderbufferStorage(uint32(target), uint32(internalformat), int32(width), int32(height))
}

func (f *goglFunctions) ShaderSource(s giogl.Shader, src string) {
	csources, free := gl.Strs(src + "\x00")
	gl.ShaderSource(uint32(s.V), 1, csources, nil)
	free()
}

func (f *goglFunctions) TexImage2D(target giogl.Enum, level int, internalFormat int, width, height int, format, ty giogl.Enum, data []byte) {
	var p unsafe.Pointer
	if len(data) > 0 {
		p = unsafe.Pointer(&data[0])
	}
	gl.TexImage2D(uint32(target), int32(level), int32(internalFormat), int32(width), int32(height), 0, uint32(format), uint32(ty), p)
}

func (f *goglFunctions) TexParameteri(target, pname giogl.Enum, param int) {
	gl.TexParameteri(uint32(target), uint32(pname), int32(param))
}

func (f *goglFunctions) Uniform1f(dst giogl.Uniform, v float32) {
	gl.Uniform1f(int32(dst.V), v)
}

func (f *goglFunctions) UniformBlockBinding(p giogl.Program, uniformBlockIndex uint, uniformBlockBinding uint) {
	gl.UniformBlockBinding(uint32(p.V), uint32(uniformBlockIndex), uint32(uniformBlockBinding))
}

func (f *goglFunctions) Uniform1i(dst giogl.Uniform, v int) {
	gl.Uniform1i(int32(dst.V), int32(v))
}

func (f *goglFunctions) Uniform2f(dst giogl.Uniform, v0, v1 float32) {
	gl.Uniform2f(int32(dst.V), v0, v1)
}

func (f *goglFunctions) Uniform3f(dst giogl.Uniform, v0, v1, v2 float32) {
	gl.Uniform3f(int32(dst.V), v0, v1, v2)
}

func (f *goglFunctions) Uniform4f(dst giogl.Uniform, v0, v1, v2, v3 float32) {
	gl.Uniform4f(int32(dst.V), v0, v1, v2, v3)
}

func (f *goglFunctions) UseProgram(p giogl.Program) {
	gl.UseProgram(uint32(p.V))
}

func (f *goglFunctions) VertexAttribPointer(dst giogl.Attrib, size int, ty giogl.Enum, normalized bool, stride, offset int) {
	gl.VertexAttribPointer(uint32(dst), int32(size), uint32(ty), normalized, int32(stride), unsafe.Pointer(uintptr(offset)))
}

func (f *goglFunctions) Viewport(x, y, width, height int) {
	gl.Viewport(int32(x), int32(y), int32(width), int32(height))
}
