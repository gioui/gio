// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"sort"

	"gioui.org/shader"
)

// Metadata contains reflection data about a shader.
type Metadata struct {
	Uniforms       shader.UniformsReflection
	Inputs         []shader.InputLocation
	Textures       []shader.TextureBinding
	Images         []shader.ImageBinding
	StorageBuffers []shader.BufferBinding
	WorkgroupSize  [3]int
}

// SPIRVCross cross-compiles spirv shaders to es, hlsl and others.
type SPIRVCross struct {
	Bin     string
	WorkDir WorkDir
}

func NewSPIRVCross() *SPIRVCross { return &SPIRVCross{Bin: "spirv-cross"} }

// Convert converts compute shader from spirv format to a target format.
func (spirv *SPIRVCross) Convert(path, variant string, shader []byte, target, version string) ([]byte, error) {
	base := spirv.WorkDir.Path(filepath.Base(path), variant)

	if err := spirv.WorkDir.WriteFile(base, shader); err != nil {
		return nil, fmt.Errorf("unable to write shader to disk: %w", err)
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
	case "msl", "mslios":
		cmd = exec.Command(spirv.Bin,
			"--msl",
			"--msl-decoration-binding",
			"--msl-version", version,
		)
		if target == "mslios" {
			cmd.Args = append(cmd.Args, "--msl-ios")
		}
	default:
		return nil, fmt.Errorf("unknown target %q", target)
	}
	cmd.Args = append(cmd.Args, "--no-420pack-extension", base)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s\nfailed to run %v: %w", out, cmd.Args, err)
	}
	// Force "highp" precision even for GLSL 1.00 targets, to avoid rendering
	// artifacts[0]
	//
	// [0]: https://gioui.org/issue/686
	switch target {
	case "es":
		out = bytes.ReplaceAll(out, []byte("precision mediump float;"), []byte("precision highp float;"))
	}
	if target != "hlsl" {
		// Strip Windows \r in line endings.
		out = unixLineEnding(out)
	}

	return out, nil
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
		PushConstants []struct {
			Name         string `json:"name"`
			Type         string `json:"type"`
			PushConstant bool   `json:"push_constant"`
		} `json:"push_constants"`
		EntryPoints []struct {
			Name          string `json:"name"`
			Mode          string `json:"mode"`
			WorkgroupSize [3]int `json:"workgroup_size"`
		} `json:"entryPoints"`
		StorageBuffers []struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Binding int    `json:"binding"`
		} `json:"ssbos"`
		Images []struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Binding int    `json:"binding"`
		} `json:"images"`
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
		m.Inputs = append(m.Inputs, shader.InputLocation{
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

	blockSize := 0
	minOffset := math.MaxInt
	for _, block := range reflect.PushConstants {
		t := reflect.Types[block.Type]
		for _, member := range t.Members {
			dataType, size, err := parseDataType(member.Type)
			if err != nil {
				return Metadata{}, fmt.Errorf("failed to parse reflection data: %v", err)
			}
			blockSize += size * 4
			if member.Offset < minOffset {
				minOffset = member.Offset
			}
			m.Uniforms.Locations = append(m.Uniforms.Locations, shader.UniformLocation{
				Name:   fmt.Sprintf("%s.%s", block.Name, member.Name),
				Type:   dataType,
				Size:   size,
				Offset: member.Offset,
			})
		}
	}
	m.Uniforms.Size = blockSize

	for _, texture := range reflect.Textures {
		m.Textures = append(m.Textures, shader.TextureBinding{
			Name:    texture.Name,
			Binding: texture.Binding,
		})
	}

	for _, img := range reflect.Images {
		m.Images = append(m.Images, shader.ImageBinding{
			Name:    img.Name,
			Binding: img.Binding,
		})
	}

	for _, sb := range reflect.StorageBuffers {
		m.StorageBuffers = append(m.StorageBuffers, shader.BufferBinding{
			Name:    sb.Name,
			Binding: sb.Binding,
		})
	}

	for _, e := range reflect.EntryPoints {
		if e.Name == "main" && e.Mode == "comp" {
			m.WorkgroupSize = e.WorkgroupSize
		}
	}

	return m, nil
}

func parseDataType(t string) (shader.DataType, int, error) {
	switch t {
	case "float":
		return shader.DataTypeFloat, 1, nil
	case "vec2":
		return shader.DataTypeFloat, 2, nil
	case "vec3":
		return shader.DataTypeFloat, 3, nil
	case "vec4":
		return shader.DataTypeFloat, 4, nil
	case "int":
		return shader.DataTypeInt, 1, nil
	case "int2":
		return shader.DataTypeInt, 2, nil
	case "int3":
		return shader.DataTypeInt, 3, nil
	case "int4":
		return shader.DataTypeInt, 4, nil
	default:
		return 0, 0, fmt.Errorf("unsupported input data type: %s", t)
	}
}
