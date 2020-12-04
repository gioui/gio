// SPDX-License-Identifier: Unlicense OR MIT

// GLFW doesn't build on OpenBSD and FreeBSD.
// +build !openbsd,!freebsd,!windows,!android,!ios,!js

// The glfw example demonstrates integration of Gio into a foreign
// windowing and rendering library, in this case GLFW
// (https://www.glfw.org).
//
// See the go-glfw package for installation of the native
// dependencies:
//
// https://github.com/go-gl/glfw
package main

import (
	"image"
	"log"
	"math"
	"runtime"
	"time"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gpu"
	giogl "gioui.org/gpu/gl"
	"gioui.org/io/pointer"
	"gioui.org/io/router"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type glfwConfig struct {
	Scale float32
}

type goglFunctions struct {
}

func main() {
	// Required by the OpenGL threading model.
	runtime.LockOSThread()

	err := glfw.Init()
	if err != nil {
		log.Fatal(err)
	}
	defer glfw.Terminate()
	// Gio assumes a sRGB backbuffer.
	glfw.WindowHint(glfw.SRGBCapable, glfw.True)

	window, err := glfw.CreateWindow(800, 600, "Gio + GLFW", nil, nil)
	if err != nil {
		log.Fatal(err)
	}

	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		log.Fatal(err)
	}
	// Enable sRGB.
	gl.Enable(gl.FRAMEBUFFER_SRGB)

	var queue router.Router
	var ops op.Ops
	th := material.NewTheme(gofont.Collection())
	backend, err := giogl.NewBackend(nil)
	if err != nil {
		log.Fatal(err)
	}
	gpu, err := gpu.New(backend)
	if err != nil {
		log.Fatal(err)
	}

	registerCallbacks(window, &queue)
	for !window.ShouldClose() {
		glfw.PollEvents()
		scale := float32(1.0)
		if monitor := window.GetMonitor(); monitor != nil {
			scalex, _ := window.GetMonitor().GetContentScale()
			scale = scalex
		}
		width, height := window.GetSize()
		sz := image.Point{X: width, Y: height}
		ops.Reset()
		gtx := layout.Context{
			Ops:   &ops,
			Now:   time.Now(),
			Queue: &queue,
			Metric: unit.Metric{
				PxPerDp: scale,
				PxPerSp: scale,
			},
			Constraints: layout.Exact(sz),
		}
		draw(gtx, th)
		gpu.Collect(sz, gtx.Ops)
		gpu.BeginFrame()
		queue.Frame(gtx.Ops)
		gpu.EndFrame()
		window.SwapBuffers()
	}
}

var button widget.Clickable

func draw(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Center.Layout(gtx,
		material.Button(th, &button, "Button").Layout,
	)
}

func registerCallbacks(window *glfw.Window, q *router.Router) {
	var btns pointer.Buttons
	beginning := time.Now()
	var lastPos f32.Point
	window.SetCursorPosCallback(func(w *glfw.Window, xpos float64, ypos float64) {
		lastPos = f32.Point{X: float32(xpos), Y: float32(ypos)}
		q.Add(pointer.Event{
			Type:     pointer.Move,
			Position: lastPos,
			Source:   pointer.Mouse,
			Time:     time.Since(beginning),
			Buttons:  btns,
		})
	})
	window.SetMouseButtonCallback(func(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
		var btn pointer.Buttons
		switch button {
		case glfw.MouseButton1:
			btn = pointer.ButtonLeft
		case glfw.MouseButton2:
			btn = pointer.ButtonRight
		case glfw.MouseButton3:
			btn = pointer.ButtonMiddle
		}
		var typ pointer.Type
		switch action {
		case glfw.Release:
			typ = pointer.Release
			btns &^= btn
		case glfw.Press:
			typ = pointer.Press
			btns |= btn
		}
		q.Add(pointer.Event{
			Type:     typ,
			Source:   pointer.Mouse,
			Time:     time.Since(beginning),
			Position: lastPos,
			Buttons:  btns,
		})
	})
}

func (s *glfwConfig) Px(v unit.Value) int {
	scale := s.Scale
	if v.U == unit.UnitPx {
		scale = 1
	}
	return int(math.Round(float64(scale * v.V)))
}
