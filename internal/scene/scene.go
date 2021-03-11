// SPDX-License-Identifier: Unlicense OR MIT

// Package scene encodes and decodes graphics commands in the format used by the
// compute renderer.
package scene

import (
	"image/color"
	"math"
	"unsafe"

	"gioui.org/f32"
)

type Op uint32

type Command [sceneElemSize / 4]uint32

const sceneElemSize = 36

// GPU commands from scene.h
const (
	OpNop Op = iota
	OpStrokeLine
	OpFillLine
	OpStrokeQuad
	OpFillQuad
	OpStrokeCubic
	OpFillCubic
	OpStroke
	OpFill
	OpLineWidth
	OpTransform
	OpBeginClip
	OpEndClip
	OpFillImage
)

const CommandSize = int(unsafe.Sizeof(Command{}))

func (c Command) Op() Op {
	return Op(c[0])
}

func Line(start, end f32.Point, stroke bool, flags uint32) Command {
	tag := uint32(OpFillLine)
	if stroke {
		tag = uint32(OpStrokeLine)
	}
	return Command{
		0: flags<<16 | tag,
		1: math.Float32bits(start.X),
		2: math.Float32bits(start.Y),
		3: math.Float32bits(end.X),
		4: math.Float32bits(end.Y),
	}
}

func Cubic(start, ctrl0, ctrl1, end f32.Point, stroke bool) Command {
	tag := uint32(OpFillCubic)
	if stroke {
		tag = uint32(OpStrokeCubic)
	}
	return Command{
		0: tag,
		1: math.Float32bits(start.X),
		2: math.Float32bits(start.Y),
		3: math.Float32bits(ctrl0.X),
		4: math.Float32bits(ctrl0.Y),
		5: math.Float32bits(ctrl1.X),
		6: math.Float32bits(ctrl1.Y),
		7: math.Float32bits(end.X),
		8: math.Float32bits(end.Y),
	}
}

func Quad(start, ctrl, end f32.Point, stroke bool) Command {
	tag := uint32(OpFillQuad)
	if stroke {
		tag = uint32(OpStrokeQuad)
	}
	return Command{
		0: tag,
		1: math.Float32bits(start.X),
		2: math.Float32bits(start.Y),
		3: math.Float32bits(ctrl.X),
		4: math.Float32bits(ctrl.Y),
		5: math.Float32bits(end.X),
		6: math.Float32bits(end.Y),
	}
}

func Transform(m f32.Affine2D) Command {
	sx, hx, ox, hy, sy, oy := m.Elems()
	return Command{
		0: uint32(OpTransform),
		1: math.Float32bits(sx),
		2: math.Float32bits(hy),
		3: math.Float32bits(hx),
		4: math.Float32bits(sy),
		5: math.Float32bits(ox),
		6: math.Float32bits(oy),
	}
}

func LineWidth(width float32) Command {
	return Command{
		0: uint32(OpLineWidth),
		1: math.Float32bits(width),
	}
}

func Stroke(col color.RGBA) Command {
	return Command{
		0: uint32(OpStroke),
		1: uint32(col.R)<<24 | uint32(col.G)<<16 | uint32(col.B)<<8 | uint32(col.A),
	}
}

func BeginClip(bbox f32.Rectangle) Command {
	return Command{
		0: uint32(OpBeginClip),
		1: math.Float32bits(bbox.Min.X),
		2: math.Float32bits(bbox.Min.Y),
		3: math.Float32bits(bbox.Max.X),
		4: math.Float32bits(bbox.Max.Y),
	}
}

func EndClip(bbox f32.Rectangle) Command {
	return Command{
		0: uint32(OpEndClip),
		1: math.Float32bits(bbox.Min.X),
		2: math.Float32bits(bbox.Min.Y),
		3: math.Float32bits(bbox.Max.X),
		4: math.Float32bits(bbox.Max.Y),
	}
}

func Fill(col color.RGBA) Command {
	return Command{
		0: uint32(OpFill),
		1: uint32(col.R)<<24 | uint32(col.G)<<16 | uint32(col.B)<<8 | uint32(col.A),
	}
}

func FillImage(index int) Command {
	return Command{
		0: uint32(OpFillImage),
		1: uint32(index),
	}
}

func DecodeLine(cmd Command) (from, to f32.Point) {
	if cmd[0] != uint32(OpFillLine) {
		panic("invalid command")
	}
	from = f32.Pt(math.Float32frombits(cmd[1]), math.Float32frombits(cmd[2]))
	to = f32.Pt(math.Float32frombits(cmd[3]), math.Float32frombits(cmd[4]))
	return
}

func DecodeQuad(cmd Command) (from, ctrl, to f32.Point) {
	if cmd[0] != uint32(OpFillQuad) {
		panic("invalid command")
	}
	from = f32.Pt(math.Float32frombits(cmd[1]), math.Float32frombits(cmd[2]))
	ctrl = f32.Pt(math.Float32frombits(cmd[3]), math.Float32frombits(cmd[4]))
	to = f32.Pt(math.Float32frombits(cmd[5]), math.Float32frombits(cmd[6]))
	return
}

func DecodeCubic(cmd Command) (from, ctrl0, ctrl1, to f32.Point) {
	if cmd[0] != uint32(OpFillCubic) {
		panic("invalid command")
	}
	from = f32.Pt(math.Float32frombits(cmd[1]), math.Float32frombits(cmd[2]))
	ctrl0 = f32.Pt(math.Float32frombits(cmd[3]), math.Float32frombits(cmd[4]))
	ctrl1 = f32.Pt(math.Float32frombits(cmd[5]), math.Float32frombits(cmd[6]))
	to = f32.Pt(math.Float32frombits(cmd[7]), math.Float32frombits(cmd[8]))
	return
}
