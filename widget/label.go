// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"fmt"
	"image"
	"unicode/utf8"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"golang.org/x/image/math/fixed"
)

// Label is a widget for laying out and drawing text.
type Label struct {
	// Alignment specify the text alignment.
	Alignment text.Alignment
	// MaxLines limits the number of lines. Zero means no limit.
	MaxLines int
}

// screenPos describes a character position (in text line and column numbers,
// not pixels): Y = line number, X = rune column.
type screenPos image.Point

const inf = 1e6

func posIsAbove(lines []text.Line, pos combinedPos, y int) bool {
	line := lines[pos.lineCol.Y]
	return pos.y+line.Bounds.Max.Y.Ceil() < y
}

func posIsBelow(lines []text.Line, pos combinedPos, y int) bool {
	line := lines[pos.lineCol.Y]
	return pos.y+line.Bounds.Min.Y.Floor() > y
}

func clipLine(lines []text.Line, alignment text.Alignment, width int, clip image.Rectangle, linePos combinedPos) (start combinedPos, end combinedPos) {
	// Seek to first (potentially) visible column.
	lineIdx := linePos.lineCol.Y
	line := lines[lineIdx]
	// runeWidth is the width of the widest rune in line.
	runeWidth := (line.Bounds.Max.X - line.Width).Ceil()
	q := combinedPos{y: start.y, x: fixed.I(clip.Min.X - runeWidth)}
	start, _ = seekPosition(lines, alignment, width, linePos, q, 0)
	// Seek to first invisible column after start.
	q = combinedPos{y: start.y, x: fixed.I(clip.Max.X + runeWidth)}
	end, _ = seekPosition(lines, alignment, width, start, q, 0)
	return start, end
}

func subLayout(line text.Line, startCol, endCol int) text.Layout {
	adv := line.Layout.Advances
	if startCol == len(adv) {
		return text.Layout{}
	}
	adv = adv[startCol:endCol]
	txt := line.Layout.Text
	for i := 0; i < startCol; i++ {
		_, s := utf8.DecodeRuneInString(txt)
		txt = txt[s:]
	}
	n := 0
	for i := startCol; i < endCol; i++ {
		_, s := utf8.DecodeRuneInString(txt[n:])
		n += s
	}
	txt = txt[:n]
	return text.Layout{Text: txt, Advances: adv}
}

func firstPos(line text.Line, alignment text.Alignment, width int) combinedPos {
	return combinedPos{
		x: align(alignment, line.Width, width),
		y: line.Ascent.Ceil(),
	}
}

func (p1 screenPos) Less(p2 screenPos) bool {
	return p1.Y < p2.Y || (p1.Y == p2.Y && p1.X < p2.X)
}

func (l Label) Layout(gtx layout.Context, s text.Shaper, font text.Font, size unit.Value, txt string) layout.Dimensions {
	cs := gtx.Constraints
	textSize := fixed.I(gtx.Px(size))
	lines := s.LayoutString(font, textSize, cs.Max.X, txt)
	if max := l.MaxLines; max > 0 && len(lines) > max {
		lines = lines[:max]
	}
	dims := linesDimens(lines)
	dims.Size = cs.Constrain(dims.Size)
	if len(lines) == 0 {
		return dims
	}
	cl := textPadding(lines)
	cl.Max = cl.Max.Add(dims.Size)
	defer clip.Rect(cl).Push(gtx.Ops).Pop()
	semantic.LabelOp(txt).Add(gtx.Ops)
	pos := firstPos(lines[0], l.Alignment, dims.Size.X)
	for !posIsBelow(lines, pos, cl.Max.Y) {
		start, end := clipLine(lines, l.Alignment, dims.Size.X, cl, pos)
		line := lines[start.lineCol.Y]
		lt := subLayout(line, start.lineCol.X, end.lineCol.X)

		off := image.Point{X: start.x.Floor(), Y: start.y}
		t := op.Offset(layout.FPt(off)).Push(gtx.Ops)
		op := clip.Outline{Path: s.Shape(font, textSize, lt)}.Op().Push(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		op.Pop()
		t.Pop()

		if pos.lineCol.Y == len(lines)-1 {
			break
		}
		pos, _ = seekPosition(lines, l.Alignment, dims.Size.X, pos, combinedPos{lineCol: screenPos{Y: pos.lineCol.Y + 1}}, 0)
	}
	return dims
}

func textPadding(lines []text.Line) (padding image.Rectangle) {
	if len(lines) == 0 {
		return
	}
	first := lines[0]
	if d := first.Ascent + first.Bounds.Min.Y; d < 0 {
		padding.Min.Y = d.Ceil()
	}
	last := lines[len(lines)-1]
	if d := last.Bounds.Max.Y - last.Descent; d > 0 {
		padding.Max.Y = d.Ceil()
	}
	if d := first.Bounds.Min.X; d < 0 {
		padding.Min.X = d.Ceil()
	}
	if d := first.Bounds.Max.X - first.Width; d > 0 {
		padding.Max.X = d.Ceil()
	}
	return
}

func linesDimens(lines []text.Line) layout.Dimensions {
	var width fixed.Int26_6
	var h int
	var baseline int
	if len(lines) > 0 {
		baseline = lines[0].Ascent.Ceil()
		var prevDesc fixed.Int26_6
		for _, l := range lines {
			h += (prevDesc + l.Ascent).Ceil()
			prevDesc = l.Descent
			if l.Width > width {
				width = l.Width
			}
		}
		h += lines[len(lines)-1].Descent.Ceil()
	}
	w := width.Ceil()
	return layout.Dimensions{
		Size: image.Point{
			X: w,
			Y: h,
		},
		Baseline: h - baseline,
	}
}

func align(align text.Alignment, width fixed.Int26_6, maxWidth int) fixed.Int26_6 {
	mw := fixed.I(maxWidth)
	switch align {
	case text.Middle:
		return fixed.I(((mw - width) / 2).Floor())
	case text.End:
		return fixed.I((mw - width).Floor())
	case text.Start:
		return 0
	default:
		panic(fmt.Errorf("unknown alignment %v", align))
	}
}
