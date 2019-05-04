package gpu

import (
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
	ctx.caps = caps{
		EXT_disjoint_timer_query: strings.Contains(exts, "GL_EXT_disjoint_timer_query"),
		srgbMode:                 srgbModeFor(ver, exts),
	}
	return ctx, nil
}

func srgbModeFor(ver [2]int, exts string) srgbMode {
	switch {
	case ver[0] >= 3:
		return srgbES3
	case strings.Contains(exts, "EXT_sRGB"):
		return srgbEXT
	default:
		panic("neither OpenGL ES 3 nor EXT_sRGB is supported")
	}
}
