// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
	"time"
)

// Backend represents the abstraction of underlying GPU
// APIs such as OpenGL, Direct3D useful for rendering Gio
// operations.
type Backend interface {
	BeginFrame()
	EndFrame()
	Caps() Caps
	NewTimer() Timer
	// IsContinuousTime reports whether all timer measurements
	// are valid at the point of call.
	IsTimeContinuous() bool
	NewTexture(format TextureFormat, width, height int, minFilter, magFilter TextureFilter) Texture
	DefaultFramebuffer() Framebuffer
	NewFramebuffer() Framebuffer
	NewImmutableBuffer(typ BufferType, data []byte) Buffer
	NewBuffer(typ BufferType, size int) Buffer
	NewProgram(vertexShader, fragmentShader ShaderSources) (Program, error)
	NewInputLayout(vertexShader ShaderSources, layout []InputDesc) (InputLayout, error)

	DepthFunc(f DepthFunc)
	ClearColor(r, g, b, a float32)
	ClearDepth(d float32)
	Clear(buffers BufferAttachments)
	Viewport(x, y, width, height int)
	DrawArrays(mode DrawMode, off, count int)
	DrawElements(mode DrawMode, off, count int)
	SetBlend(enable bool)
	SetDepthTest(enable bool)
	DepthMask(mask bool)
	BlendFunc(sfactor, dfactor BlendFactor)
}

type ShaderSources struct {
	GLES2       string
	HLSL        []byte
	Uniforms    []UniformLocation
	UniformSize int
	Inputs      []InputLocation
	Textures    []TextureBinding
}

type TextureBinding struct {
	Name    string
	Binding int
}

type UniformLocation struct {
	Name   string
	Type   DataType
	Size   int
	Offset int
}

type InputLocation struct {
	// For GLSL.
	Name     string
	Location int
	// For HLSL.
	Semantic      string
	SemanticIndex int

	Type DataType
	Size int
}

// InputDesc describes a vertex attribute as laid out in a Buffer.
type InputDesc struct {
	Type DataType
	Size int

	Offset int
}

// InputLayout is the backend specific representation of the mapping
// between Buffers and shader attributes.
type InputLayout interface {
	Bind()
	Release()
}

type BlendFactor uint8

type DrawMode uint8

type BufferAttachments uint

type TextureFilter uint8
type TextureFormat uint8

type BufferType uint8

type DataType uint8

type DepthFunc uint8

type Features uint

type Caps struct {
	Features       Features
	MaxTextureSize int
}

type Program interface {
	Bind()
	Release()
	SetVertexUniforms(buf Buffer)
	SetFragmentUniforms(buf Buffer)
}

type Buffer interface {
	BindVertex(stride, offset int)
	BindIndex()
	Release()
	Upload(data []byte)
}

type Framebuffer interface {
	Bind()
	BindTexture(t Texture)
	Invalidate()
	Release()
	IsComplete() error
}

type Timer interface {
	Begin()
	End()
	Duration() (time.Duration, bool)
	Release()
}

type Texture interface {
	Upload(img *image.RGBA)
	Release()
	Bind(unit int)
}

const (
	BufferAttachmentColor BufferAttachments = 1 << iota
	BufferAttachmentDepth
)

const (
	DepthFuncGreater DepthFunc = iota
)

const (
	DataTypeFloat DataType = iota
	DataTypeShort
)

const (
	BufferTypeIndices BufferType = iota
	BufferTypeVertices
	BufferTypeUniforms
)

const (
	TextureFormatSRGB TextureFormat = iota
	TextureFormatFloat
)

const (
	FilterNearest TextureFilter = iota
	FilterLinear
)

const (
	FeatureTimers Features = iota
)

const (
	DrawModeTriangleStrip DrawMode = iota
	DrawModeTriangles
)

const (
	BlendFactorOne BlendFactor = iota
	BlendFactorOneMinusSrcAlpha
	BlendFactorZero
	BlendFactorDstColor
)

func (f Features) Has(feats Features) bool {
	return f&feats == feats
}
