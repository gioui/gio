// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
)

// GLSLValidator is OpenGL reference compiler.
type GLSLValidator struct {
	Bin     string
	WorkDir WorkDir
}

func NewGLSLValidator() *GLSLValidator { return &GLSLValidator{Bin: "glslangValidator"} }

// Convert converts a glsl shader to spirv.
func (glsl *GLSLValidator) Convert(path, variant string, lang string, input []byte) ([]byte, error) {
	base := glsl.WorkDir.Path(filepath.Base(path), variant)
	pathout := base + ".out"

	cmd := exec.Command(glsl.Bin,
		"--stdin",
		"-DLANG_"+strings.ToUpper(lang),
		"-I"+filepath.Dir(path),
		"-V", // OpenGL ES 3.1.
		"-w", // Suppress warnings.
		"-S", filepath.Ext(path)[1:],
		"-o", pathout,
	)
	cmd.Stdin = bytes.NewBuffer(input)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s\nfailed to run %v: %w", out, cmd.Args, err)
	}

	compiled, err := ioutil.ReadFile(pathout)
	if err != nil {
		return nil, fmt.Errorf("unable to read output %q: %w", pathout, err)
	}

	return compiled, nil
}

func spirvOpt(spirv []byte) ([]byte, error) {
	cmd := exec.Command("spirv-opt",
		"-O",
		"-",
		"-o", "-",
	)
	cmd.Stdin = bytes.NewBuffer(spirv)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s\nfailed to run %v: %w", out, cmd.Args, err)
	}
	return out, nil
}
