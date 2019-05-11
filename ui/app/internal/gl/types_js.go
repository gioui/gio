package gl

import "syscall/js"

type (
	Buffer       js.Value
	Framebuffer  js.Value
	Program      js.Value
	Renderbuffer js.Value
	Shader       js.Value
	Texture      js.Value
	Query        js.Value
	Uniform      js.Value
	Object       js.Value
)

func (u Uniform) Valid() bool {
	return js.Value(u) != js.Null()
}
