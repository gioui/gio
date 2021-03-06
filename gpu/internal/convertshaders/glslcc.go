// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gioui.org/gpu/internal/driver"
)

// GLSLCC is a shader cross-compilation tool.
type GLSLCC struct {
	Bin        string
	IncludeDir string
	WorkDir    WorkDir
}

func NewGLSLCC() *GLSLCC { return &GLSLCC{Bin: "glslcc"} }

// Metadata contains reflection data about the shader.
type Metadata struct {
	Uniforms       driver.UniformsReflection
	Inputs         []driver.InputLocation
	Textures       []driver.TextureBinding
	StorageBuffers []driver.StorageBufferBinding
}

// Convert converts input data to the target shader.
func (glslcc *GLSLCC) Convert(path, variant string, input []byte, lang, profile string) (_ string, _ Metadata, err error) {
	base := glslcc.WorkDir.Path(filepath.Base(path), variant, lang, profile)
	pathin := base + ".in"
	pathout := base + ".out"
	reflectout := base + ".json"

	if err := glslcc.WorkDir.WriteFile(pathin, input); err != nil {
		return "", Metadata{}, fmt.Errorf("unable to write shader to disk: %w", err)
	}

	var progFlag, progSuffix string
	switch filepath.Ext(path) {
	case ".vert":
		progFlag = "--vert"
		progSuffix = "vs"
	case ".frag":
		progFlag = "--frag"
		progSuffix = "fs"
	case ".comp":
		progFlag = "--compute"
		progSuffix = "cs"
	default:
		return "", Metadata{}, fmt.Errorf("unrecognized shader type: %q", path)
	}

	cmd := exec.Command(glslcc.Bin,
		"--silent",
		"--optimize",
		"--include-dirs", glslcc.IncludeDir,
		"--reflect="+reflectout,
		"--output", pathout,
		"--lang", lang,
		"--profile", profile,
		progFlag, pathin,
	)
	if lang == "hlsl" {
		cmd.Args = append(cmd.Args, "--defines=HLSL")
	}

	output, err := cmd.Output()
	if err != nil {
		return "", Metadata{}, fmt.Errorf("%s\nfailed to run %v: %w", output, cmd.Args, err)
	}

	// glslcc modifies the output path
	p := strings.IndexByte(pathout, '.')
	pathout = pathout[:p] + "_" + progSuffix + pathout[p:]
	shader, err := ioutil.ReadFile(pathout)
	if err != nil {
		return "", Metadata{}, fmt.Errorf("missing shader output %q: %w", pathout, err)
	}

	reflectdata, err := ioutil.ReadFile(reflectout)
	if err != nil {
		return "", Metadata{}, fmt.Errorf("missing reflection output %q: %w", pathout, err)
	}

	metadata, err := glslcc.parseReflection(reflectdata)
	if err != nil {
		return "", Metadata{}, fmt.Errorf("invalid reflection output %q: %w", reflectout, err)
	}

	return string(shader), metadata, nil
}

// parseReflection parses glslcc -reflect output.
func (glslcc *GLSLCC) parseReflection(jsonData []byte) (Metadata, error) {
	type (
		Input struct {
			ID            int    `json:"id"`
			Name          string `json:"name"`
			Location      int    `json:"location"`
			Semantic      string `json:"semantic"`
			SemanticIndex int    `json:"semantic_index"`
			Type          string `json:"type"`
		}
		UniformMember struct {
			Name   string `json:"name"`
			Type   string `json:"type"`
			Offset int    `json:"offset"`
			Size   int    `json:"size"`
		}
		UniformBuffer struct {
			ID      int             `json:"id"`
			Name    string          `json:"name"`
			Set     int             `json:"set"`
			Binding int             `json:"binding"`
			Size    int             `json:"block_size"`
			Members []UniformMember `json:"members"`
		}
		Texture struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Set       int    `json:"set"`
			Binding   int    `json:"binding"`
			Dimension string `json:"dimension"`
			Format    string `json:"format"`
		}
		StorageBuffer struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Set       int    `json:"set"`
			Binding   int    `json:"binding"`
			BlockSize int    `json:"block_size"`
		}
		Shader struct {
			Inputs         []Input         `json:"inputs"`
			UniformBuffers []UniformBuffer `json:"uniform_buffers"`
			Textures       []Texture       `json:"textures"`
			StorageBuffers []StorageBuffer `json:"storage_buffers"`
		}
		ReflectMetadata struct {
			VS Shader `json:"vs"`
			FS Shader `json:"fs"`
			CS Shader `json:"cs"`
		}
	)

	var info Metadata

	var reflect ReflectMetadata
	if err := json.Unmarshal(jsonData, &reflect); err != nil {
		return info, fmt.Errorf("parseReflection: %v", err)
	}

	inputRef := reflect.VS.Inputs
	for _, input := range inputRef {
		dataType, dataSize, err := parseDataType(input.Type)
		if err != nil {
			return info, fmt.Errorf("parseReflection: %v", err)
		}
		info.Inputs = append(info.Inputs, driver.InputLocation{
			Name:          input.Name,
			Location:      input.Location,
			Semantic:      input.Semantic,
			SemanticIndex: input.SemanticIndex,
			Type:          dataType,
			Size:          dataSize,
		})
	}
	sort.Slice(info.Inputs, func(i, j int) bool {
		return info.Inputs[i].Location < info.Inputs[j].Location
	})

	shaderBlocks := reflect.VS.UniformBuffers
	if len(shaderBlocks) == 0 {
		shaderBlocks = reflect.FS.UniformBuffers
	}

	blockOffset := 0
	for _, block := range shaderBlocks {
		info.Uniforms.Blocks = append(info.Uniforms.Blocks, driver.UniformBlock{
			Name:    block.Name,
			Binding: block.Binding,
		})
		for _, member := range block.Members {
			dataType, size, err := parseDataType(member.Type)
			if err != nil {
				return info, fmt.Errorf("parseReflection: %v", err)
			}
			info.Uniforms.Locations = append(info.Uniforms.Locations, driver.UniformLocation{
				// Synthetic name generated by glslcc.
				Name:   fmt.Sprintf("_%d.%s", block.ID, member.Name),
				Type:   dataType,
				Size:   size,
				Offset: blockOffset + member.Offset,
			})
		}
		blockOffset += block.Size
	}
	info.Uniforms.Size = blockOffset

	textures := reflect.VS.Textures
	if len(textures) == 0 {
		textures = reflect.FS.Textures
	}
	for _, texture := range textures {
		info.Textures = append(info.Textures, driver.TextureBinding{
			Name:    texture.Name,
			Binding: texture.Binding,
		})
	}

	for _, sb := range reflect.CS.StorageBuffers {
		info.StorageBuffers = append(info.StorageBuffers, driver.StorageBufferBinding{
			Binding:   sb.Binding,
			BlockSize: sb.BlockSize,
		})
	}

	return info, nil
}

func parseDataType(t string) (driver.DataType, int, error) {
	switch t {
	case "float":
		return driver.DataTypeFloat, 1, nil
	case "float2":
		return driver.DataTypeFloat, 2, nil
	case "float3":
		return driver.DataTypeFloat, 3, nil
	case "float4":
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
