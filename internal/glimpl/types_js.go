// SPDX-License-Identifier: Unlicense OR MIT

package glimpl

type (
	Buffer       value
	Framebuffer  value
	Program      value
	Renderbuffer value
	Shader       value
	Texture      value
	Query        value
	Uniform      value
	Object       value
)

func (p *Program) Valid() bool {
	return p.ref != 0
}

func (s *Shader) Valid() bool {
	return s.ref != 0
}

func (u *Uniform) Valid() bool {
	return u.ref != 0
}
