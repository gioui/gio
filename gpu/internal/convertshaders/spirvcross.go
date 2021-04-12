// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gioui.org/gpu/internal/driver"
)

// Metadata contains reflection data about a shader.
type Metadata struct {
	Uniforms driver.UniformsReflection
	Inputs   []driver.InputLocation
	Textures []driver.TextureBinding
}

// SPIRVCross cross-compiles spirv shaders to es, hlsl and others.
type SPIRVCross struct {
	Bin     string
	WorkDir WorkDir
}

func NewSPIRVCross() *SPIRVCross { return &SPIRVCross{Bin: "spirv-cross"} }

// Convert converts compute shader from spirv format to a target format.
func (spirv *SPIRVCross) Convert(path, variant string, shader []byte, target, version string) (string, error) {
	base := spirv.WorkDir.Path(filepath.Base(path), variant)

	if err := spirv.WorkDir.WriteFile(base, shader); err != nil {
		return "", fmt.Errorf("unable to write shader to disk: %w", err)
	}

	var cmd *exec.Cmd
	switch target {
	case "glsl":
		cmd = exec.Command(spirv.Bin,
			"--no-es",
			"--version", version,
		)
	case "es":
		cmd = exec.Command(spirv.Bin,
			"--es",
			"--version", version,
		)
	case "hlsl":
		cmd = exec.Command(spirv.Bin,
			"--hlsl",
			"--shader-model", version,
		)
	default:
		return "", fmt.Errorf("unknown target %q", target)
	}
	cmd.Args = append(cmd.Args, base)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s\nfailed to run %v: %w", out, cmd.Args, err)
	}
	s := string(out)
	if target != "hlsl" {
		// Strip Windows \r in line endings.
		s = unixLineEnding(s)
	}

	return s, nil
}

// Metadata extracts metadata for a SPIR-V shader.
func (spirv *SPIRVCross) Metadata(path, variant string, shader []byte) (Metadata, error) {
	base := spirv.WorkDir.Path(filepath.Base(path), variant)

	if err := spirv.WorkDir.WriteFile(base, shader); err != nil {
		return Metadata{}, fmt.Errorf("unable to write shader to disk: %w", err)
	}

	cmd := exec.Command(spirv.Bin,
		base,
		"--reflect",
	)

	out, err := cmd.Output()
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to run %v: %w", cmd.Args, err)
	}

	meta, err := parseMetadata(out)
	if err != nil {
		return Metadata{}, fmt.Errorf("%s\nfailed to parse metadata: %w", out, err)
	}

	return meta, nil
}

func parseMetadata(data []byte) (Metadata, error) {
	var reflect struct {
		Types map[string]struct {
			Name    string `json:"name"`
			Members []struct {
				Name   string `json:"name"`
				Type   string `json:"type"`
				Offset int    `json:"offset"`
			} `json:"members"`
		} `json:"types"`
		Inputs []struct {
			Name     string `json:"name"`
			Type     string `json:"type"`
			Location int    `json:"location"`
		} `json:"inputs"`
		Textures []struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Set     int    `json:"set"`
			Binding int    `json:"binding"`
		} `json:"textures"`
		UBOs []struct {
			Name      string `json:"name"`
			Type      string `json:"type"`
			BlockSize int    `json:"block_size"`
			Set       int    `json:"set"`
			Binding   int    `json:"binding"`
		} `json:"ubos"`
	}
	if err := json.Unmarshal(data, &reflect); err != nil {
		return Metadata{}, fmt.Errorf("failed to parse reflection data: %w", err)
	}

	var m Metadata

	for _, input := range reflect.Inputs {
		dataType, dataSize, err := parseDataType(input.Type)
		if err != nil {
			return Metadata{}, fmt.Errorf("parseReflection: %v", err)
		}
		m.Inputs = append(m.Inputs, driver.InputLocation{
			Name:          input.Name,
			Location:      input.Location,
			Semantic:      "TEXCOORD",
			SemanticIndex: input.Location,
			Type:          dataType,
			Size:          dataSize,
		})
	}

	sort.Slice(m.Inputs, func(i, j int) bool {
		return m.Inputs[i].Location < m.Inputs[j].Location
	})

	blockOffset := 0
	for _, block := range reflect.UBOs {
		m.Uniforms.Blocks = append(m.Uniforms.Blocks, driver.UniformBlock{
			Name:    block.Name,
			Binding: block.Binding,
		})
		t := reflect.Types[block.Type]
		// By convention uniform block variables are named by prepending an underscore
		// and converting to lowercase.
		blockVar := "_" + strings.ToLower(block.Name)
		for _, member := range t.Members {
			dataType, size, err := parseDataType(member.Type)
			if err != nil {
				return Metadata{}, fmt.Errorf("failed to parse reflection data: %v", err)
			}
			m.Uniforms.Locations = append(m.Uniforms.Locations, driver.UniformLocation{
				Name:   fmt.Sprintf("%s.%s", blockVar, member.Name),
				Type:   dataType,
				Size:   size,
				Offset: blockOffset + member.Offset,
			})
		}
		blockOffset += block.BlockSize
	}
	m.Uniforms.Size = blockOffset

	for _, texture := range reflect.Textures {
		m.Textures = append(m.Textures, driver.TextureBinding{
			Name:    texture.Name,
			Binding: texture.Binding,
		})
	}

	//return m, fmt.Errorf("not yet!: %+v", reflect)
	return m, nil
}

func parseDataType(t string) (driver.DataType, int, error) {
	switch t {
	case "float":
		return driver.DataTypeFloat, 1, nil
	case "vec2":
		return driver.DataTypeFloat, 2, nil
	case "vec3":
		return driver.DataTypeFloat, 3, nil
	case "vec4":
		return driver.DataTypeFloat, 4, nil
	case "int":
		return driver.DataTypeInt, 1, nil
	case "int2":
		return driver.DataTypeInt, 2, nil
	case "int3":
		return driver.DataTypeInt, 3, nil
	case "int4":
		return driver.DataTypeInt, 4, nil
	default:
		return 0, 0, fmt.Errorf("unsupported input data type: %s", t)
	}
}
