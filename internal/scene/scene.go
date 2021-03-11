// SPDX-License-Identifier: Unlicense OR MIT

// Package scene encodes and decodes graphics commands in the format used by the
// compute renderer.
package scene

import (
	"image/color"
	"math"

	"gioui.org/f32"
)

type Command [sceneElemSize / 4]uint32

const sceneElemSize = 36

// GPU commands from scene.h
const (
	elemNop = iota
	elemStrokeLine
	elemFillLine
	elemStrokeQuad
	elemFillQuad
	elemStrokeCubic
	elemFillCubic
	elemStroke
	elemFill
	elemLineWidth
	elemTransform
	elemBeginClip
	elemEndClip
	elemFillImage
)

func Line(start, end f32.Point, stroke bool, flags uint32) Command {
	tag := uint32(elemFillLine)
	if stroke {
		tag = elemStrokeLine
	}
	return Command{
		0: flags<<16 | tag,
		1: math.Float32bits(start.X),
		2: math.Float32bits(start.Y),
		3: math.Float32bits(end.X),
		4: math.Float32bits(end.Y),
	}
}

func Quad(start, ctrl, end f32.Point, stroke bool) Command {
	tag := uint32(elemFillQuad)
	if stroke {
		tag = elemStrokeQuad
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
		0: elemTransform,
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
		0: elemLineWidth,
		1: math.Float32bits(width),
	}
}

func Stroke(col color.RGBA) Command {
	return Command{
		0: elemStroke,
		1: uint32(col.R)<<24 | uint32(col.G)<<16 | uint32(col.B)<<8 | uint32(col.A),
	}
}

func BeginClip(bbox f32.Rectangle) Command {
	return Command{
		0: elemBeginClip,
		1: math.Float32bits(bbox.Min.X),
		2: math.Float32bits(bbox.Min.Y),
		3: math.Float32bits(bbox.Max.X),
		4: math.Float32bits(bbox.Max.Y),
	}
}

func EndClip(bbox f32.Rectangle) Command {
	return Command{
		0: elemEndClip,
		1: math.Float32bits(bbox.Min.X),
		2: math.Float32bits(bbox.Min.Y),
		3: math.Float32bits(bbox.Max.X),
		4: math.Float32bits(bbox.Max.Y),
	}
}

func Fill(col color.RGBA) Command {
	return Command{
		0: elemFill,
		1: uint32(col.R)<<24 | uint32(col.G)<<16 | uint32(col.B)<<8 | uint32(col.A),
	}
}

func FillImage(index int) Command {
	return Command{
		0: elemFillImage,
		1: uint32(index),
	}
}

func DecodeQuad(cmd Command) (from, ctrl, to f32.Point) {
	if cmd[0] != elemFillQuad {
		panic("invalid command")
	}
	from = f32.Pt(math.Float32frombits(cmd[1]), math.Float32frombits(cmd[2]))
	ctrl = f32.Pt(math.Float32frombits(cmd[3]), math.Float32frombits(cmd[4]))
	to = f32.Pt(math.Float32frombits(cmd[5]), math.Float32frombits(cmd[6]))
	return
}
