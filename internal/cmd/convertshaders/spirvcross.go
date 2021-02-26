// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

// SPIRVCross cross-compiles spirv shaders to es, hlsl and others.
type SPIRVCross struct {
	Bin string
}

func NewSPIRVCross() *SPIRVCross { return &SPIRVCross{Bin: "spirv-cross"} }

// Convert converts compute shader from spirv format to a target format.
func (spirv *SPIRVCross) Convert(input []byte, target, version string) (string, error) {
	var cmd *exec.Cmd
	switch target {
	case "es":
		cmd = exec.Command(spirv.Bin,
			"--es",
			"--version", version,
			"-",
		)
	case "hlsl":
		cmd = exec.Command(spirv.Bin,
			"--hlsl",
			"--shader-model", version,
			"-",
		)
	default:
		return "", fmt.Errorf("unknown target %q", target)
	}

	cmd.Stdin = bytes.NewBuffer(input)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run %v: %w", cmd.Args, err)
	}

	return string(out), nil
}
