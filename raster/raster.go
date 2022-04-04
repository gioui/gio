// SPDX-License-Identifier: Unlicense OR MIT

/*
Package raster implements a rasterizer for Gio optimized for embedded
platforms.

Note: the implementation is incomplete.
*/
package raster

import (
	"image"
	"image/color"
	"image/draw"

	"gioui.org/f32"
	"gioui.org/internal/ops"
	"gioui.org/internal/scene"
	"gioui.org/layout"
	"gioui.org/op"
	"golang.org/x/image/vector"
)

type Rasterizer struct {
	reader ops.Reader

	scratch struct {
		transforms []f32.Affine2D
		states     []f32.Affine2D
		clips      []clipState
	}
}

type clipState struct {
	path   []byte
	trans  f32.Affine2D
	bounds image.Rectangle
	paths  int
}

func (r *Rasterizer) Frame(frame *op.Ops, frameBuf *image.RGBA) {
	if frame == nil {
		return
	}
	d := &r.reader
	d.Reset(&frame.Internal)

	stack := r.scratch.transforms[:0]
	states := r.scratch.states[:0]
	clips := r.scratch.clips
	defer func() {
		r.scratch.transforms = stack
		r.scratch.states = states
		r.scratch.clips = clips
	}()
	type decodeState struct {
		t        f32.Affine2D
		material image.Image
	}
	var pathData struct {
		data []byte
	}
	zeroState := decodeState{
		material: image.NewUniform(&color.NRGBA{}),
	}
	state := zeroState
	for encOp, ok := d.Decode(); ok; encOp, ok = d.Decode() {
		switch ops.OpType(encOp.Data[0]) {
		case ops.TypeTransform:
			dop, push := ops.DecodeTransform(encOp.Data)
			if push {
				stack = append(stack, state.t)
			}
			state.t = state.t.Mul(dop)
		case ops.TypePopTransform:
			state.t = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
		case ops.TypeStroke:
			// TODO.
		case ops.TypePath:
			op, ok := d.Decode()
			if !ok {
				panic("unexpected end of path operation")
			}
			pathData.data = op.Data[ops.TypeAuxLen:]
		case ops.TypeClip:
			var op ops.ClipOp
			op.Decode(encOp.Data)
			bounds := transformBounds(state.t, op.Bounds)
			paths := 0
			if pathData.data != nil {
				paths = 1
			}
			if len(clips) > 0 {
				parent := clips[len(clips)-1]
				bounds = bounds.Intersect(parent.bounds)
				paths += parent.paths
			}
			clips = append(clips, clipState{
				path:   pathData.data,
				trans:  state.t,
				bounds: bounds,
				paths:  paths,
			})
			pathData.data = nil
		case ops.TypePopClip:
			clips = clips[:len(clips)-1]
		case ops.TypeColor:
			col := decodeColorOp(encOp.Data)
			state.material = image.NewUniform(col)
		case ops.TypeLinearGradient:
			// TODO.
		case ops.TypeImage:
			state.material = encOp.Refs[0].(*image.RGBA)
		case ops.TypePaint:
			bounds := frameBuf.Bounds()
			paths := 0
			if len(clips) > 0 {
				parent := clips[len(clips)-1]
				bounds = bounds.Intersect(parent.bounds)
				paths = parent.paths
			}
			if bounds.Empty() {
				break
			}
			switch paths {
			case 0:
				draw.Draw(frameBuf, bounds, state.material, state.material.Bounds().Min, draw.Over)
			case 1:
				vr := vector.NewRasterizer(bounds.Dx(), bounds.Dy())
				vr.DrawOp = draw.Over
				for i := len(clips) - 1; i >= 0; i-- {
					c := clips[i]
					if c.path == nil {
						continue
					}
					off := layout.FPt(bounds.Min.Mul(-1))
					decodePath(vr, c.path, c.trans.Offset(off))
				}
				vr.Draw(frameBuf, bounds, state.material, state.material.Bounds().Min)
			}
		case ops.TypeSave:
			id := ops.DecodeSave(encOp.Data)
			if extra := id - len(states) + 1; extra > 0 {
				states = append(states, make([]f32.Affine2D, extra)...)
			}
			states[id] = state.t
		case ops.TypeLoad:
			id := ops.DecodeLoad(encOp.Data)
			state = zeroState
			state.t = states[id]
		}
	}
}

func decodePath(r *vector.Rasterizer, pathData []byte, t f32.Affine2D) {
	for len(pathData) >= scene.CommandSize+4 {
		cmd := ops.DecodeCommand(pathData[4:])
		var pen f32.Point
		switch cmd.Op() {
		case scene.OpLine:
			from, to := scene.DecodeLine(cmd)
			from, to = t.Transform(from), t.Transform(to)
			if from != pen {
				r.MoveTo(from.X, from.Y)
			}
			r.LineTo(to.X, to.Y)
			pen = to
		case scene.OpGap:
			from, to := scene.DecodeGap(cmd)
			from, to = t.Transform(from), t.Transform(to)
			if from != pen {
				r.MoveTo(from.X, from.Y)
			}
			r.LineTo(to.X, to.Y)
			pen = to
		case scene.OpQuad:
			from, ctrl, to := scene.DecodeQuad(cmd)
			from, ctrl, to = t.Transform(from), t.Transform(ctrl), t.Transform(to)
			if from != pen {
				r.MoveTo(from.X, from.Y)
			}
			r.QuadTo(ctrl.X, ctrl.Y, to.X, to.Y)
			pen = to
		case scene.OpCubic:
			from, ctrl0, ctrl1, to := scene.DecodeCubic(cmd)
			from, ctrl0, ctrl1, to = t.Transform(from), t.Transform(ctrl0), t.Transform(ctrl1), t.Transform(to)
			if from != pen {
				r.MoveTo(from.X, from.Y)
			}
			r.CubeTo(ctrl0.X, ctrl0.Y, ctrl1.X, ctrl1.Y, to.X, to.Y)
			pen = to
		default:
			panic("unsupported scene command")
		}
		pathData = pathData[scene.CommandSize+4:]
	}
}

func transformBounds(t f32.Affine2D, bounds image.Rectangle) image.Rectangle {
	b0 := f32.Rectangle{
		Min: t.Transform(layout.FPt(bounds.Min)),
		Max: t.Transform(layout.FPt(bounds.Max)),
	}.Canon()
	b1 := f32.Rectangle{
		Min: t.Transform(layout.FPt(image.Pt(bounds.Max.X, bounds.Min.Y))),
		Max: t.Transform(layout.FPt(image.Pt(bounds.Min.X, bounds.Max.Y))),
	}.Canon()
	return b0.Union(b1).Round()
}

func decodeColorOp(data []byte) color.NRGBA {
	return color.NRGBA{
		R: data[1],
		G: data[2],
		B: data[3],
		A: data[4],
	}
}
