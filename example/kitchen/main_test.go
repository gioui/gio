// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/app/headless"
	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/widget/material"
)

func BenchmarkUI(b *testing.B) { benchmarkUI(b, transformation{}) }
func BenchmarkUI_Offset(b *testing.B) { benchmarkUI(b, transformation{offset: true}) }
func BenchmarkUI_Scale(b *testing.B) { benchmarkUI(b, transformation{scale: true}) }
func BenchmarkUI_Rotate(b *testing.B) { benchmarkUI(b, transformation{rotate: true}) }
func BenchmarkUI_All(b *testing.B) { benchmarkUI(b, transformation{offset: true, rotate: true, scale: true}) }

func benchmarkUI(b *testing.B, transform transformation) {
	th := material.NewTheme(gofont.Collection())

	w, err := headless.NewWindow(800, 600)
	if err != nil {
		b.Fatal(err)
	}
	defer w.Release()

	var layoutTime time.Duration
	var frameTime time.Duration

	b.ResetTimer()
	var ops op.Ops
	for i := 0; i < b.N; i++ {
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Constraints: layout.Exact(image.Pt(800, 600)),
		}
		addTransform(i, transform, gtx.Ops)
		layoutTime += measure(func(){ kitchen(gtx, th) })
		frameTime += measure(func(){ w.Frame(&ops) })
	}
	b.StopTimer()

	b.ReportMetric(float64(layoutTime.Nanoseconds()) / float64(b.N), "ns/layout")
	b.ReportMetric(float64(frameTime.Nanoseconds()) / float64(b.N), "ns/frame")
}

type transformation struct {
	offset bool
	rotate bool
	scale bool
}

func addTransform(i int, transform transformation, ops *op.Ops) {
	if !(transform.offset || transform.rotate || transform.scale) {
		return
	}
	dt := float32(i)
	tr := f32.Affine2D{}
	if transform.rotate {
		angle := dt * .1
		tr = tr.Rotate(f32.Pt(300, 20), -angle)
	}
	if transform.scale {
		scale := 1.0 - dt*.5
		if scale < 0.5 {
			scale = 0.5
		}
		tr = tr.Scale(f32.Pt(300, 20), f32.Pt(scale, scale))
	}
	if transform.offset {
		offset := dt * 50
		if offset > 200 {
			offset = 200
		}
		tr = tr.Offset(f32.Pt(0, offset))
	}
	op.Affine(tr).Add(ops)
}

func measure(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}