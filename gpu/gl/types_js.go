// SPDX-License-Identifier: Unlicense OR MIT

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

func (p Program) Valid() bool {
	return !js.Value(p).IsUndefined() && !js.Value(p).IsNull()
}

func (s Shader) Valid() bool {
	return !js.Value(s).IsUndefined() && !js.Value(s).IsNull()
}

func (u Uniform) Valid() bool {
	return !js.Value(u).IsUndefined() && !js.Value(u).IsNull()
}

func (t Texture) Valid() bool {
	return !js.Value(t).IsUndefined() && !js.Value(t).IsNull()
}

func (t Texture) Equal(t2 Texture) bool {
	return js.Value(t).Equal(js.Value(t2))
}
