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
	NewTexture(minFilter, magFilter TextureFilter) Texture
	DefaultFramebuffer() Framebuffer
	NilTexture() Texture
	NewFramebuffer() Framebuffer
	NewBuffer(typ BufferType, data []byte) Buffer
	NewProgram(vertexShader, fragmentShader string, attribMap []string) (Program, error)
	SetupVertexArray(slot int, size int, dataType DataType, stride, offset int)

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
	UniformFor(uniform string) Uniform
	Uniform1i(u Uniform, v int)
	Uniform1f(u Uniform, v float32)
	Uniform2f(u Uniform, v0, v1 float32)
	Uniform4f(u Uniform, v0, v1, v2, v3 float32)
}

type Uniform interface{}

type Buffer interface {
	Bind()
	Release()
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
	Resize(format TextureFormat, width, height int)
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
	BufferTypeData
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
