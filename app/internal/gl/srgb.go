// SPDX-License-Identifier: Unlicense OR MIT

package gl

import (
	"fmt"
	"runtime"
	"strings"
)

// SRGBFBO implements an intermediate sRGB FBO
// for gamma-correct rendering on platforms without
// sRGB enabled native framebuffers.
type SRGBFBO struct {
	c             *Functions
	width, height int
	frameBuffer   Framebuffer
	depthBuffer   Renderbuffer
	colorTex      Texture
	blitted       bool
	quad          Buffer
	prog          Program
	es3           bool
}

func NewSRGBFBO(f *Functions) (*SRGBFBO, error) {
	var es3 bool
	glVer := f.GetString(VERSION)
	ver, err := ParseGLVersion(glVer)
	if err != nil {
		return nil, err
	}
	if ver[0] >= 3 {
		es3 = true
	} else {
		exts := f.GetString(EXTENSIONS)
		if !strings.Contains(exts, "EXT_sRGB") {
			return nil, fmt.Errorf("no support for OpenGL ES 3 nor EXT_sRGB")
		}
	}
	s := &SRGBFBO{
		c:           f,
		es3:         es3,
		frameBuffer: f.CreateFramebuffer(),
		colorTex:    f.CreateTexture(),
		depthBuffer: f.CreateRenderbuffer(),
	}
	f.BindTexture(TEXTURE_2D, s.colorTex)
	f.TexParameteri(TEXTURE_2D, TEXTURE_WRAP_S, CLAMP_TO_EDGE)
	f.TexParameteri(TEXTURE_2D, TEXTURE_WRAP_T, CLAMP_TO_EDGE)
	f.TexParameteri(TEXTURE_2D, TEXTURE_MAG_FILTER, NEAREST)
	f.TexParameteri(TEXTURE_2D, TEXTURE_MIN_FILTER, NEAREST)
	return s, nil
}

func (s *SRGBFBO) Blit() {
	if !s.blitted {
		prog, err := CreateProgram(s.c, blitVSrc, blitFSrc, []string{"pos", "uv"})
		if err != nil {
			panic(err)
		}
		s.prog = prog
		s.c.UseProgram(prog)
		s.c.Uniform1i(GetUniformLocation(s.c, prog, "tex"), 0)
		s.quad = s.c.CreateBuffer()
		s.c.BindBuffer(ARRAY_BUFFER, s.quad)
		s.c.BufferData(ARRAY_BUFFER,
			BytesView([]float32{
				-1, +1, 0, 1,
				+1, +1, 1, 1,
				-1, -1, 0, 0,
				+1, -1, 1, 0,
			}),
			STATIC_DRAW)
		s.blitted = true
	}
	s.c.BindFramebuffer(FRAMEBUFFER, Framebuffer{})
	s.c.ClearColor(1, 0, 1, 1)
	s.c.Clear(COLOR_BUFFER_BIT)
	s.c.UseProgram(s.prog)
	s.c.BindTexture(TEXTURE_2D, s.colorTex)
	s.c.BindBuffer(ARRAY_BUFFER, s.quad)
	s.c.VertexAttribPointer(0 /* pos */, 2, FLOAT, false, 4*4, 0)
	s.c.VertexAttribPointer(1 /* uv */, 2, FLOAT, false, 4*4, 4*2)
	s.c.EnableVertexAttribArray(0)
	s.c.EnableVertexAttribArray(1)
	s.c.DrawArrays(TRIANGLE_STRIP, 0, 4)
	s.c.BindTexture(TEXTURE_2D, Texture{})
	s.c.DisableVertexAttribArray(0)
	s.c.DisableVertexAttribArray(1)
	s.c.BindFramebuffer(FRAMEBUFFER, s.frameBuffer)
	s.c.InvalidateFramebuffer(FRAMEBUFFER, COLOR_ATTACHMENT0)
	s.c.InvalidateFramebuffer(FRAMEBUFFER, DEPTH_ATTACHMENT)
	// The Android emulator requires framebuffer 0 bound at eglSwapBuffer time.
	// Bind the sRGB framebuffer again in afterPresent.
	s.c.BindFramebuffer(FRAMEBUFFER, Framebuffer{})
}

func (s *SRGBFBO) AfterPresent() {
	s.c.BindFramebuffer(FRAMEBUFFER, s.frameBuffer)
}

func (s *SRGBFBO) Refresh(w, h int) error {
	s.width, s.height = w, h
	if w == 0 || h == 0 {
		return nil
	}
	s.c.BindTexture(TEXTURE_2D, s.colorTex)
	if s.es3 {
		s.c.TexImage2D(TEXTURE_2D, 0, SRGB8_ALPHA8, w, h, RGBA, UNSIGNED_BYTE, nil)
	} else /* EXT_sRGB */ {
		s.c.TexImage2D(TEXTURE_2D, 0, SRGB_ALPHA_EXT, w, h, SRGB_ALPHA_EXT, UNSIGNED_BYTE, nil)
	}
	currentRB := Renderbuffer(s.c.GetBinding(RENDERBUFFER_BINDING))
	s.c.BindRenderbuffer(RENDERBUFFER, s.depthBuffer)
	s.c.RenderbufferStorage(RENDERBUFFER, DEPTH_COMPONENT16, w, h)
	s.c.BindRenderbuffer(RENDERBUFFER, currentRB)
	s.c.BindFramebuffer(FRAMEBUFFER, s.frameBuffer)
	s.c.FramebufferTexture2D(FRAMEBUFFER, COLOR_ATTACHMENT0, TEXTURE_2D, s.colorTex, 0)
	s.c.FramebufferRenderbuffer(FRAMEBUFFER, DEPTH_ATTACHMENT, RENDERBUFFER, s.depthBuffer)
	if st := s.c.CheckFramebufferStatus(FRAMEBUFFER); st != FRAMEBUFFER_COMPLETE {
		return fmt.Errorf("sRGB framebuffer incomplete (%dx%d), status: %#x error: %x", s.width, s.height, st, s.c.GetError())
	}

	if runtime.GOOS == "js" {
		// With macOS Safari, rendering to and then reading from a SRGB8_ALPHA8
		// texture result in twice gamma corrected colors. Using a plain RGBA
		// texture seems to work.
		s.c.ClearColor(.5, .5, .5, 1.0)
		s.c.Clear(COLOR_BUFFER_BIT)
		var pixel [4]byte
		s.c.ReadPixels(0, 0, 1, 1, RGBA, UNSIGNED_BYTE, pixel[:])
		if pixel[0] == 128 { // Correct sRGB color value is ~188
			s.c.TexImage2D(TEXTURE_2D, 0, RGBA, w, h, RGBA, UNSIGNED_BYTE, nil)
			if st := s.c.CheckFramebufferStatus(FRAMEBUFFER); st != FRAMEBUFFER_COMPLETE {
				return fmt.Errorf("fallback RGBA framebuffer incomplete (%dx%d), status: %#x error: %x", s.width, s.height, st, s.c.GetError())
			}
		}
	}

	return nil
}

func (s *SRGBFBO) Release() {
	s.c.DeleteFramebuffer(s.frameBuffer)
	s.c.DeleteTexture(s.colorTex)
	s.c.DeleteRenderbuffer(s.depthBuffer)
	if s.blitted {
		s.c.DeleteBuffer(s.quad)
		s.c.DeleteProgram(s.prog)
	}
	s.c = nil
}

const (
	blitVSrc = `
#version 100

precision highp float;

attribute vec2 pos;
attribute vec2 uv;

varying vec2 vUV;

void main() {
    gl_Position = vec4(pos, 0, 1);
    vUV = uv;
}
`
	blitFSrc = `
#version 100

precision mediump float;

uniform sampler2D tex;
varying vec2 vUV;

vec3 gamma(vec3 rgb) {
	vec3 exp = vec3(1.055)*pow(rgb, vec3(0.41666)) - vec3(0.055);
	vec3 lin = rgb * vec3(12.92);
	bvec3 cut = lessThan(rgb, vec3(0.0031308));
	return vec3(cut.r ? lin.r : exp.r, cut.g ? lin.g : exp.g, cut.b ? lin.b : exp.b);
}

void main() {
    vec4 col = texture2D(tex, vUV);
	vec3 rgb = col.rgb;
	rgb = gamma(rgb);
	gl_FragColor = vec4(rgb, col.a);
}
`
)
