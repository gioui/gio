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
		return Program{}, err
	}
	defer ctx.DeleteShader(vs)
	fs, err := createShader(ctx, FRAGMENT_SHADER, fsSrc)
	if err != nil {
		return Program{}, err
	}
	defer ctx.DeleteShader(fs)
	prog := ctx.CreateProgram()
	if !prog.Valid() {
		return Program{}, errors.New("glCreateProgram failed")
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
		return Program{}, fmt.Errorf("program link failed: %s", strings.TrimSpace(log))
	}
	return prog, nil
}

func GetUniformLocation(ctx *Functions, prog Program, name string) Uniform {
	loc := ctx.GetUniformLocation(prog, name)
	if !loc.Valid() {
		panic(fmt.Errorf("uniform %s not found", name))
	}
	return loc
}

func createShader(ctx *Functions, typ Enum, src string) (Shader, error) {
	sh := ctx.CreateShader(typ)
	if !sh.Valid() {
		return Shader{}, errors.New("glCreateShader failed")
	}
	ctx.ShaderSource(sh, src)
	ctx.CompileShader(sh)
	if ctx.GetShaderi(sh, COMPILE_STATUS) == 0 {
		log := ctx.GetShaderInfoLog(sh)
		ctx.DeleteShader(sh)
		return Shader{}, fmt.Errorf("shader compilation failed: %s", strings.TrimSpace(log))
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
	if _, err := fmt.Sscanf(glVer, "OpenGL ES %d.%d", &ver[0], &ver[1]); err == nil {
		return ver, nil
	} else if _, err := fmt.Sscanf(glVer, "WebGL %d.%d", &ver[0], &ver[1]); err == nil {
		// WebGL major version v corresponds to OpenGL ES version v + 1
		ver[0]++
		return ver, nil
	} else if _, err := fmt.Sscanf(glVer, "%d.%d", &ver[0], &ver[1]); err == nil {
		return ver, nil
	}
	return ver, fmt.Errorf("failed to parse OpenGL ES version (%s)", glVer)
}

func SliceOf(s uintptr) []byte {
	if s == 0 {
		return nil
	}
	sh := reflect.SliceHeader{
		Data: s,
		Len:  1 << 30,
		Cap:  1 << 30,
	}
	return *(*[]byte)(unsafe.Pointer(&sh))
}

// GoString convert a NUL-terminated C string
// to a Go string.
func GoString(s []byte) string {
	i := 0
	for {
		if s[i] == 0 {
			break
		}
		i++
	}
	return string(s[:i])
}
