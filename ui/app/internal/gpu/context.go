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
	srgbMode                 srgbMode
	// floatTriple holds the settings for floating point
	// textures.
	floatTriple textureTriple
}

// textureTriple holds the type settings for
// a TexImage2D call.
type textureTriple struct {
	internalFormat int
	format         gl.Enum
	typ            gl.Enum
}

type srgbMode uint8

const (
	srgbES3 srgbMode = iota
	srgbEXT
)

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
	srgbMode, err := srgbModeFor(ver, exts)
	if err != nil {
		return nil, err
	}
	floatTriple, err := floatTripleFor(ver, exts)
	if err != nil {
		return nil, err
	}
	ctx.caps = caps{
		EXT_disjoint_timer_query: strings.Contains(exts, "GL_EXT_disjoint_timer_query"),
		srgbMode:                 srgbMode,
		floatTriple:              floatTriple,
	}
	return ctx, nil
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

func srgbModeFor(ver [2]int, exts string) (srgbMode, error) {
	switch {
	case ver[0] >= 3:
		return srgbES3, nil
	case strings.Contains(exts, "EXT_sRGB"):
		return srgbEXT, nil
	default:
		return 0, errors.New("neither OpenGL ES 3 nor EXT_sRGB is supported")
	}
}
