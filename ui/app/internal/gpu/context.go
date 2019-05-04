package gpu

import (
	"errors"
	"strings"

	"gioui.org/ui/app/internal/gl"
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

func newContext(glctx gl.Context) (*context, error) {
	ctx := &context{
		Functions: glctx.Functions(),
	}
	exts := ctx.GetString(gl.EXTENSIONS)
	glVer := ctx.GetString(gl.VERSION)
	ver, err := gl.ParseGLVersion(glVer)
	if err != nil {
		return nil, err
	}
	floatTriple, err := floatTripleFor(ver, exts)
	if err != nil {
		return nil, err
	}
	srgbaTriple, err := srgbaTripleFor(ver, exts)
	if err != nil {
		return nil, err
	}
	ctx.caps = caps{
		EXT_disjoint_timer_query: strings.Contains(exts, "GL_EXT_disjoint_timer_query"),
		floatTriple:              floatTriple,
		alphaTriple:              alphaTripleFor(ver),
		srgbaTriple:              srgbaTriple,
	}
	return ctx, nil
}

func srgbaTripleFor(ver [2]int, exts string) (textureTriple, error) {
	switch {
	case ver[0] >= 3:
		return textureTriple{gl.SRGB8_ALPHA8, gl.Enum(gl.RGBA), gl.Enum(gl.UNSIGNED_BYTE)}, nil
	case strings.Contains(exts, "GL_EXT_sRGB"):
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

func floatTripleFor(ver [2]int, exts string) (textureTriple, error) {
	switch {
	case ver[0] >= 3:
		return textureTriple{gl.R16F, gl.Enum(gl.RED), gl.Enum(gl.HALF_FLOAT)}, nil
	case strings.Contains(exts, "GL_OES_texture_half_float"):
		return textureTriple{gl.RGBA, gl.Enum(gl.RGBA), gl.Enum(gl.HALF_FLOAT_OES)}, nil
	case strings.Contains(exts, "GL_OES_texture_float"):
		return textureTriple{gl.RGBA, gl.Enum(gl.RGBA), gl.Enum(gl.FLOAT)}, nil
	default:
		return textureTriple{}, errors.New("floating point texture not supported")
	}
}
