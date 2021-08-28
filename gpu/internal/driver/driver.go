// SPDX-License-Identifier: Unlicense OR MIT

package driver

import (
	"errors"
	"image"
	"time"

	"gioui.org/shader"
)

// Device represents the abstraction of underlying GPU
// APIs such as OpenGL, Direct3D useful for rendering Gio
// operations.
type Device interface {
	BeginFrame(target RenderTarget, clear bool, viewport image.Point) Framebuffer
	EndFrame()
	Caps() Caps
	NewTimer() Timer
	// IsContinuousTime reports whether all timer measurements
	// are valid at the point of call.
	IsTimeContinuous() bool
	NewTexture(format TextureFormat, width, height int, minFilter, magFilter TextureFilter, bindings BufferBinding) (Texture, error)
	NewFramebuffer(tex Texture) (Framebuffer, error)
	NewImmutableBuffer(typ BufferBinding, data []byte) (Buffer, error)
	NewBuffer(typ BufferBinding, size int) (Buffer, error)
	NewComputeProgram(shader shader.Sources) (Program, error)
	NewVertexShader(src shader.Sources) (VertexShader, error)
	NewFragmentShader(src shader.Sources) (FragmentShader, error)
	NewPipeline(desc PipelineDesc) (Pipeline, error)

	Viewport(x, y, width, height int)
	DrawArrays(mode DrawMode, off, count int)
	DrawElements(mode DrawMode, off, count int)

	BindProgram(p Program)
	BindPipeline(p Pipeline)
	BindFramebuffer(f Framebuffer, a LoadDesc)
	BindTexture(unit int, t Texture)
	BindVertexBuffer(b Buffer, offset int)
	BindIndexBuffer(b Buffer)
	BindImageTexture(unit int, texture Texture, access AccessBits, format TextureFormat)
	BindVertexUniforms(buf Buffer)
	BindFragmentUniforms(buf Buffer)
	BindStorageBuffer(binding int, buf Buffer)

	CopyTexture(dst Texture, dstOrigin image.Point, src Framebuffer, srcRect image.Rectangle)
	MemoryBarrier()
	DispatchCompute(x, y, z int)

	Release()
}

type LoadDesc struct {
	Action     LoadAction
	ClearColor struct {
		R float32
		G float32
		B float32
		A float32
	}
}

type Pipeline interface {
	Release()
}

type PipelineDesc struct {
	VertexShader   VertexShader
	FragmentShader FragmentShader
	VertexLayout   VertexLayout
	BlendDesc      BlendDesc
	PixelFormat    TextureFormat
}

type VertexLayout struct {
	Inputs []InputDesc
	Stride int
}

// InputDesc describes a vertex attribute as laid out in a Buffer.
type InputDesc struct {
	Type shader.DataType
	Size int

	Offset int
}

type BlendDesc struct {
	Enable               bool
	SrcFactor, DstFactor BlendFactor
}

type AccessBits uint8

type BlendFactor uint8

type DrawMode uint8

type TextureFilter uint8
type TextureFormat uint8

type BufferBinding uint8

type LoadAction uint8

type Features uint

type Caps struct {
	// BottomLeftOrigin is true if the driver has the origin in the lower left
	// corner. The OpenGL driver returns true.
	BottomLeftOrigin bool
	Features         Features
	MaxTextureSize   int
}

type VertexShader interface {
	Release()
}

type FragmentShader interface {
	Release()
}

type Program interface {
	Release()
}

type Buffer interface {
	Release()
	Upload(data []byte)
	Download(data []byte) error
}

type Framebuffer interface {
	RenderTarget
	Release()
	ReadPixels(src image.Rectangle, pixels []byte, stride int) error
}

type Timer interface {
	Begin()
	End()
	Duration() (time.Duration, bool)
	Release()
}

type Texture interface {
	Upload(offset, size image.Point, pixels []byte, stride int)
	Release()
}

const (
	BufferBindingIndices BufferBinding = 1 << iota
	BufferBindingVertices
	BufferBindingUniforms
	BufferBindingTexture
	BufferBindingFramebuffer
	BufferBindingShaderStorageRead
	BufferBindingShaderStorageWrite
)

const (
	TextureFormatSRGBA TextureFormat = iota
	TextureFormatFloat
	TextureFormatRGBA8
	// TextureFormatOutput denotes the format used by the output framebuffer.
	TextureFormatOutput
)

const (
	AccessRead AccessBits = 1 << iota
	AccessWrite
)

const (
	FilterNearest TextureFilter = iota
	FilterLinear
)

const (
	FeatureTimers Features = 1 << iota
	FeatureFloatRenderTargets
	FeatureCompute
	FeatureSRGB
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

const (
	LoadActionKeep LoadAction = iota
	LoadActionClear
	LoadActionInvalidate
)

var ErrContentLost = errors.New("buffer content lost")

func (f Features) Has(feats Features) bool {
	return f&feats == feats
}

func DownloadImage(d Device, f Framebuffer, r image.Rectangle) (*image.RGBA, error) {
	img := image.NewRGBA(r)
	if err := f.ReadPixels(r, img.Pix, img.Stride); err != nil {
		return nil, err
	}
	if d.Caps().BottomLeftOrigin {
		// OpenGL origin is in the lower-left corner. Flip the image to
		// match.
		flipImageY(r.Dx()*4, r.Dy(), img.Pix)
	}
	return img, nil
}

func flipImageY(stride, height int, pixels []byte) {
	// Flip image in y-direction. OpenGL's origin is in the lower
	// left corner.
	row := make([]uint8, stride)
	for y := 0; y < height/2; y++ {
		y1 := height - y - 1
		dest := y1 * stride
		src := y * stride
		copy(row, pixels[dest:])
		copy(pixels[dest:], pixels[src:src+len(row)])
		copy(pixels[src:], row)
	}
}

func UploadImage(t Texture, offset image.Point, img *image.RGBA) {
	var pixels []byte
	size := img.Bounds().Size()
	start := img.PixOffset(0, 0)
	end := img.PixOffset(size.X, size.Y-1)
	pixels = img.Pix[start:end]
	t.Upload(offset, size, pixels, img.Stride)
}
