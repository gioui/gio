// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"errors"
	"strings"

	"gioui.org/app/internal/gl"
)

type context struct {
	caps caps
	*gl.Functions
}

type caps struct {
	EXT_disjoint_timer_query bool
	// floatTriple holds the settings for floating point
	// textures.
	floatTriple textureTriple
	// Single channel alpha textures.
	alphaTriple textureTriple
	srgbaTriple textureTriple
}

// textureTriple holds the type settings for
// a TexImage2D call.
type textureTriple struct {
	internalFormat int
	format         gl.Enum
	typ            gl.Enum
}

func newContext(glctx *gl.Functions) (*context, error) {
	ctx := &context{
		Functions: glctx,
	}
	exts := strings.Split(ctx.GetString(gl.EXTENSIONS), " ")
	glVer := ctx.GetString(gl.VERSION)
	ver, err := gl.ParseGLVersion(glVer)
	if err != nil {
		return nil, err
	}
	floatTriple, err := floatTripleFor(ctx, ver, exts)
	if err != nil {
		return nil, err
	}
	srgbaTriple, err := srgbaTripleFor(ver, exts)
	if err != nil {
		return nil, err
	}
	hasTimers := hasExtension(exts, "GL_EXT_disjoint_timer_query_webgl2") || hasExtension(exts, "GL_EXT_disjoint_timer_query")
	ctx.caps = caps{
		EXT_disjoint_timer_query: hasTimers,
		floatTriple:              floatTriple,
		alphaTriple:              alphaTripleFor(ver),
		srgbaTriple:              srgbaTriple,
	}
	return ctx, nil
}

// floatTripleFor determines the best texture triple for floating point FBOs.
func floatTripleFor(ctx *context, ver [2]int, exts []string) (textureTriple, error) {
	var triples []textureTriple
	if ver[0] >= 3 {
		triples = append(triples, textureTriple{gl.R16F, gl.Enum(gl.RED), gl.Enum(gl.HALF_FLOAT)})
	}
	if hasExtension(exts, "GL_OES_texture_half_float") && hasExtension(exts, "GL_EXT_color_buffer_half_float") {
		// Try single channel.
		triples = append(triples, textureTriple{gl.LUMINANCE, gl.Enum(gl.LUMINANCE), gl.Enum(gl.HALF_FLOAT_OES)})
		// Fallback to 4 channels.
		triples = append(triples, textureTriple{gl.RGBA, gl.Enum(gl.RGBA), gl.Enum(gl.HALF_FLOAT_OES)})
	}
	if hasExtension(exts, "GL_OES_texture_float") || hasExtension(exts, "GL_EXT_color_buffer_float") {
		triples = append(triples, textureTriple{gl.RGBA, gl.Enum(gl.RGBA), gl.Enum(gl.FLOAT)})
	}
	tex := ctx.CreateTexture()
	defer ctx.DeleteTexture(tex)
	ctx.BindTexture(gl.TEXTURE_2D, tex)
	ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	fbo := ctx.CreateFramebuffer()
	defer ctx.DeleteFramebuffer(fbo)
	defFBO := gl.Framebuffer(ctx.GetBinding(gl.FRAMEBUFFER_BINDING))
	ctx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	defer ctx.BindFramebuffer(gl.FRAMEBUFFER, defFBO)
	for _, tt := range triples {
		const size = 256
		ctx.TexImage2D(gl.TEXTURE_2D, 0, tt.internalFormat, size, size, tt.format, tt.typ, nil)
		ctx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0)
		if st := ctx.CheckFramebufferStatus(gl.FRAMEBUFFER); st == gl.FRAMEBUFFER_COMPLETE {
			return tt, nil
		}
	}
	return textureTriple{}, errors.New("floating point fbos not supported")
}

func srgbaTripleFor(ver [2]int, exts []string) (textureTriple, error) {
	switch {
	case ver[0] >= 3:
		return textureTriple{gl.SRGB8_ALPHA8, gl.Enum(gl.RGBA), gl.Enum(gl.UNSIGNED_BYTE)}, nil
	case hasExtension(exts, "GL_EXT_sRGB"):
		return textureTriple{gl.SRGB_ALPHA_EXT, gl.Enum(gl.SRGB_ALPHA_EXT), gl.Enum(gl.UNSIGNED_BYTE)}, nil
	default:
		return textureTriple{}, errors.New("no sRGB texture formats found")
	}
}

func alphaTripleFor(ver [2]int) textureTriple {
	intf, f := gl.R8, gl.Enum(gl.RED)
	if ver[0] < 3 {
		// R8, RED not supported on OpenGL ES 2.0.
		intf, f = gl.LUMINANCE, gl.Enum(gl.LUMINANCE)
	}
	return textureTriple{intf, f, gl.UNSIGNED_BYTE}
}

func hasExtension(exts []string, ext string) bool {
	for _, e := range exts {
		if ext == e {
			return true
		}
	}
	return false
}
