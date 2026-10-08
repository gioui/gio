// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FXC is hlsl compiler that targets ShaderModel 5.x and lower.
type FXC struct {
	Bin     string
	WorkDir WorkDir
}

func NewFXC() *FXC { return &FXC{Bin: "fxc.exe"} }

// Compile compiles the input shader.
func (fxc *FXC) Compile(path, variant string, input []byte, entryPoint string, profileVersion string) ([]byte, error) {
	base := fxc.WorkDir.Path(filepath.Base(path), variant, profileVersion)
	pathin := base + ".in"
	pathout := base + ".out"
	result := pathout

	if err := fxc.WorkDir.WriteFile(pathin, input); err != nil {
		return nil, fmt.Errorf("unable to write shader to disk: %w", err)
	}

	cmd := exec.Command(fxc.Bin)
	if runtime.GOOS != "windows" {
		cmd = exec.Command("wine", fxc.Bin)
		if err := winepath(&pathin, &pathout); err != nil {
			return nil, err
		}
	}

	var profile string
	switch filepath.Ext(path) {
	case ".frag":
		profile = "ps_" + profileVersion
	case ".vert":
		profile = "vs_" + profileVersion
	case ".comp":
		profile = "cs_" + profileVersion
	default:
		return nil, fmt.Errorf("unrecognized shader type %s", path)
	}

	cmd.Args = append(cmd.Args,
		"/Fo", pathout,
		"/T", profile,
		"/E", entryPoint,
		pathin,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		info := ""
		if runtime.GOOS != "windows" {
			info = "If the fxc tool cannot be found, set WINEPATH to the Windows path for the Windows SDK.\n"
		}
		return nil, fmt.Errorf("%s\n%sfailed to run %v: %w", output, info, cmd.Args, err)
	}

	compiled, err := ioutil.ReadFile(result)
	if err != nil {
		return nil, fmt.Errorf("unable to read output %q: %w", pathout, err)
	}

	return compiled, nil
}

// DXC is hlsl compiler that targets ShaderModel 6.0 and newer.
type DXC struct {
	Bin     string
	WorkDir WorkDir
}

func NewDXC() *DXC { return &DXC{Bin: "dxc"} }

// Compile compiles the input shader.
func (dxc *DXC) Compile(path, variant string, input []byte, entryPoint string, profile string) (string, error) {
	base := dxc.WorkDir.Path(filepath.Base(path), variant, profile)
	pathin := base + ".in"
	pathout := base + ".out"
	result := pathout

	if err := dxc.WorkDir.WriteFile(pathin, input); err != nil {
		return "", fmt.Errorf("unable to write shader to disk: %w", err)
	}

	cmd := exec.Command(dxc.Bin)

	cmd.Args = append(cmd.Args,
		"-Fo", pathout,
		"-T", profile,
		"-E", entryPoint,
		"-Qstrip_reflect",
		pathin,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s\nfailed to run %v: %w", output, cmd.Args, err)
	}

	compiled, err := ioutil.ReadFile(result)
	if err != nil {
		return "", fmt.Errorf("unable to read output %q: %w", pathout, err)
	}

	return string(compiled), nil
}

// winepath uses the winepath tool to convert a paths to Windows format.
// The returned path can be used as arguments for Windows command line tools.
func winepath(paths ...*string) error {
	winepath := exec.Command("winepath", "--windows")
	for _, path := range paths {
		winepath.Args = append(winepath.Args, *path)
	}
	// Use a pipe instead of Output, because winepath may have left wineserver
	// running for several seconds as a grandchild.
	out, err := winepath.StdoutPipe()
	if err != nil {
		return fmt.Errorf("unable to start winepath: %w", err)
	}
	if err := winepath.Start(); err != nil {
		return fmt.Errorf("unable to start winepath: %w", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, out); err != nil {
		return fmt.Errorf("unable to run winepath: %w", err)
	}
	winPaths := strings.Split(strings.TrimSpace(buf.String()), "\n")
	for i, path := range paths {
		*path = winPaths[i]
	}
	return nil
}
