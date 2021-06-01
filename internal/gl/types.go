// +build !js

package gl

type (
	Buffer       struct{ V uint }
	Framebuffer  struct{ V uint }
	Program      struct{ V uint }
	Renderbuffer struct{ V uint }
	Shader       struct{ V uint }
	Texture      struct{ V uint }
	Query        struct{ V uint }
	Uniform      struct{ V int }
	VertexArray  struct{ V uint }
	Object       struct{ V uint }
)

func (u Framebuffer) Valid() bool {
	return u.V != 0
}

func (u Uniform) Valid() bool {
	return u.V != -1
}

func (p Program) Valid() bool {
	return p.V != 0
}

func (s Shader) Valid() bool {
	return s.V != 0
}

func (a VertexArray) Valid() bool {
	return a.V != 0
}
