// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"errors"
	"strings"
	"syscall/js"
)

type Functions struct {
	Ctx                         js.Value
	ExtDisjointTimerQuery       js.Value
	ExtDisjointTimerQueryWebgl2 js.Value

	*FunctionCaller

	isWebGL2 bool
}

type (
	Context js.Value
	Query   js.Value
)

func NewFunctions(ctx Context, forceES bool) (*Functions, error) {
	f := &Functions{
		Ctx:            js.Value(ctx),
		FunctionCaller: NewFunctionCaller(ctx),
	}
	if err := f.Init(); err != nil {
		return nil, err
	}
	return f, nil
}

func (f *Functions) Init() error {
	webgl2Class := js.Global().Get("WebGL2RenderingContext")
	f.isWebGL2 = !webgl2Class.IsUndefined() && f.Ctx.InstanceOf(webgl2Class)
	if !f.isWebGL2 {
		f.ExtDisjointTimerQuery = f.getExtension("EXT_disjoint_timer_query")
		if f.getExtension("OES_texture_half_float").IsNull() && f.getExtension("OES_texture_float").IsNull() {
			return errors.New("gl: no support for neither OES_texture_half_float nor OES_texture_float")
		}
		if f.getExtension("EXT_sRGB").IsNull() {
			return errors.New("gl: EXT_sRGB not supported")
		}
	} else {
		// WebGL2 extensions.
		f.ExtDisjointTimerQueryWebgl2 = f.getExtension("EXT_disjoint_timer_query_webgl2")
		if f.getExtension("EXT_color_buffer_half_float").IsNull() && f.getExtension("EXT_color_buffer_float").IsNull() {
			return errors.New("gl: no support for neither EXT_color_buffer_half_float nor EXT_color_buffer_float")
		}
	}
	return nil
}

func (f *Functions) getExtension(name string) js.Value {
	return f.Ctx.Call("getExtension", name)
}
func (f *Functions) CreateQuery() Query {
	return Query(f.Ctx.Call("createQuery"))
}
func (f *Functions) BeginQuery(target Enum, query Query) {
	if !f.ExtDisjointTimerQueryWebgl2.IsNull() {
		f.Ctx.Call("beginQuery", int(target), js.Value(query))
	} else {
		f.ExtDisjointTimerQuery.Call("beginQueryEXT", int(target), js.Value(query))
	}
}
func (f *Functions) BufferData(target Enum, size int, usage Enum, data []byte) {
	if data == nil {
		f.FunctionCaller.BufferDataSize(target, size, usage)
	} else {
		f.FunctionCaller.BufferData(target, usage, data)
	}
}
func (f *Functions) DeleteQuery(query Query) {
	if !f.ExtDisjointTimerQueryWebgl2.IsNull() {
		f.Ctx.Call("deleteQuery", js.Value(query))
	} else {
		f.ExtDisjointTimerQuery.Call("deleteQueryEXT", js.Value(query))
	}
}
func (f *Functions) EndQuery(target Enum) {
	if !f.ExtDisjointTimerQueryWebgl2.IsNull() {
		f.Ctx.Call("endQuery", int(target))
	} else {
		f.ExtDisjointTimerQuery.Call("endQueryEXT", int(target))
	}
}
func (f *Functions) GetQueryObjectuiv(query Query, pname Enum) uint {
	if !f.ExtDisjointTimerQueryWebgl2.IsNull() {
		return uint(paramVal(f.Ctx.Call("getQueryParameter", js.Value(query), int(pname))))
	} else {
		return uint(paramVal(f.ExtDisjointTimerQuery.Call("getQueryObjectEXT", js.Value(query), int(pname))))
	}
}
func (f *Functions) InvalidateFramebuffer(target, attachment Enum) {
	if !f.isWebGL2 {
		// WebGL 1 doesn't have that function
		return
	}
	f.FunctionCaller.InvalidateFramebuffer(target, attachment)
}
func (f *Functions) GetError() Enum {
	// Avoid slow getError calls. See gio#179.
	return 0
}
func (f *Functions) GetFramebufferAttachmentParameteri(target, attachment, pname Enum) int {
	if !f.isWebGL2 && pname == FRAMEBUFFER_ATTACHMENT_COLOR_ENCODING {
		// FRAMEBUFFER_ATTACHMENT_COLOR_ENCODING is only available on WebGL 2
		return LINEAR
	}
	return f.FunctionCaller.GetFramebufferAttachmentParameteri(target, attachment, pname)
}
func (f *Functions) GetBinding(pname Enum) Object {
	obj := f.FunctionCaller.GetBinding(pname)
	if !obj.valid() {
		return Object{}
	}
	return obj
}
func (f *Functions) GetBindingi(pname Enum, idx int) Object {
	obj := f.FunctionCaller.GetBindingi(pname, idx)
	if !obj.valid() {
		return Object{}
	}
	return obj
}
func (f *Functions) GetInteger(pname Enum) int {
	if !f.isWebGL2 {
		switch pname {
		case PACK_ROW_LENGTH, UNPACK_ROW_LENGTH:
			return 0 // PACK_ROW_LENGTH and UNPACK_ROW_LENGTH is only available on WebGL 2
		}
	}
	return f.FunctionCaller.GetInteger(pname)
}
func (f *Functions) GetFloat(pname Enum) float32 {
	return f.FunctionCaller.GetFloat(pname)
}

func (f *Functions) GetInteger4(pname Enum) [4]int {
	return f.FunctionCaller.GetInteger4(pname)
}

func (f *Functions) GetFloat4(pname Enum) [4]float32 {
	return f.FunctionCaller.GetFloat4(pname)
}
func (f *Functions) GetString(pname Enum) string {
	switch pname {
	case EXTENSIONS:
		extsjs := f.Ctx.Call("getSupportedExtensions")
		var exts []string
		for i := 0; i < extsjs.Length(); i++ {
			exts = append(exts, "GL_"+extsjs.Index(i).String())
		}
		return strings.Join(exts, " ")
	default:
		return f.Ctx.Call("getParameter", int(pname)).String()
	}
}
func (f *Functions) GetVertexAttribBinding(index int, pname Enum) Object {
	obj := f.FunctionCaller.GetVertexAttribBinding(index, pname)
	if !obj.valid() {
		return Object{}
	}
	return obj
}

func (f *Functions) GetProgramInfoLog(p Program) string {
	return ""
}
func (f *Functions) GetShaderInfoLog(s Shader) string {
	return ""
}

func (f *FunctionCaller) CreateVertexArray() VertexArray {
	panic("not supported")
}
func (f *FunctionCaller) DeleteVertexArray(a VertexArray) {
	panic("not implemented")
}
func (f *FunctionCaller) DispatchCompute(x, y, z int) {
	panic("not implemented")
}
func (f *FunctionCaller) MemoryBarrier(barriers Enum) {
	panic("not implemented")
}
func (f *FunctionCaller) MapBufferRange(target Enum, offset, length int, access Enum) []byte {
	panic("not implemented")
}
func (f *FunctionCaller) UnmapBuffer(target Enum) bool {
	panic("not implemented")
}
func (f *FunctionCaller) BindImageTexture(unit int, t Texture, level int, layered bool, layer int, access, format Enum) {
	panic("not implemented")
}
func (f *FunctionCaller) BindVertexArray(a VertexArray) {
	panic("not supported")
}

func paramVal(v js.Value) int {
	switch v.Type() {
	case js.TypeBoolean:
		if b := v.Bool(); b {
			return 1
		} else {
			return 0
		}
	case js.TypeNumber:
		return v.Int()
	default:
		panic("unknown parameter type")
	}
}
