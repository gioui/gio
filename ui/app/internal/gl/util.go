// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unsafe"
)

func CreateProgram(ctx *Functions, vsSrc, fsSrc string, attribs []string) (Program, error) {
	vs, err := createShader(ctx, VERTEX_SHADER, vsSrc)
	if err != nil {
		return 0, err
	}
	defer ctx.DeleteShader(vs)
	fs, err := createShader(ctx, FRAGMENT_SHADER, fsSrc)
	if err != nil {
		return 0, err
	}
	defer ctx.DeleteShader(fs)
	prog := ctx.CreateProgram()
	if prog == 0 {
		return 0, errors.New("glCreateProgram failed")
	}
	ctx.AttachShader(prog, vs)
	ctx.AttachShader(prog, fs)
	for i, a := range attribs {
		ctx.BindAttribLocation(prog, Attrib(i), a)
	}
	ctx.LinkProgram(prog)
	if ctx.GetProgrami(prog, LINK_STATUS) == 0 {
		log := ctx.GetProgramInfoLog(prog)
		ctx.DeleteProgram(prog)
		return 0, fmt.Errorf("program link failed: %s", strings.TrimSpace(log))
	}
	return prog, nil
}

func GetUniformLocation(ctx *Functions, prog Program, name string) Uniform {
	loc := ctx.GetUniformLocation(prog, name)
	if loc == -1 {
		panic(fmt.Errorf("uniform %s not found", name))
	}
	return loc
}

func createShader(ctx *Functions, typ Enum, src string) (Shader, error) {
	sh := ctx.CreateShader(typ)
	if sh == 0 {
		return 0, errors.New("glCreateShader failed")
	}
	ctx.ShaderSource(sh, src)
	ctx.CompileShader(sh)
	if ctx.GetShaderi(sh, COMPILE_STATUS) == 0 {
		log := ctx.GetShaderInfoLog(sh)
		ctx.DeleteShader(sh)
		return 0, fmt.Errorf("shader compilation failed: %s", strings.TrimSpace(log))
	}
	return sh, nil
}

// BytesView returns a byte slice view of a slice.
func BytesView(s interface{}) []byte {
	v := reflect.ValueOf(s)
	first := v.Index(0)
	sz := int(first.Type().Size())
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(first.UnsafeAddr())))),
		Len:  v.Len() * sz,
		Cap:  v.Cap() * sz,
	}))
}

func ParseGLVersion(glVer string) ([2]int, error) {
	var ver [2]int
	if _, err := fmt.Sscanf(glVer, "OpenGL ES %d.%d", &ver[0], &ver[1]); err != nil {
		if _, err := fmt.Sscanf(glVer, "%d.%d", &ver[0], &ver[1]); err != nil {
			return [2]int{}, fmt.Errorf("failed to parse OpenGL ES version (%s): %v", glVer, err)
		}
	}
	return ver, nil
}
