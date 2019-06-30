// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"image"
	"math"
	"unicode/utf8"

	"gioui.org/ui"
	"gioui.org/ui/draw"
	"gioui.org/ui/f32"
	"gioui.org/ui/layout"

	"golang.org/x/image/math/fixed"
)

type Label struct {
	Face      Face
	Alignment Alignment
	Text      string
	MaxLines  int

	it lineIterator
}

type lineIterator struct {
	Lines     []Line
	Clip      image.Rectangle
	Alignment Alignment
	Width     int
	Offset    image.Point

	y, prevDesc fixed.Int26_6
}

func (l *lineIterator) Next() (String, f32.Point, bool) {
	for len(l.Lines) > 0 {
		line := l.Lines[0]
		l.Lines = l.Lines[1:]
		x := align(l.Alignment, line.Width, l.Width) + fixed.I(l.Offset.X)
		l.y += l.prevDesc + line.Ascent
		l.prevDesc = line.Descent
		// Align baseline and line start to the pixel grid.
		off := fixed.Point26_6{X: fixed.I(x.Floor()), Y: fixed.I(l.y.Ceil())}
		x, l.y = off.X, off.Y
		off.Y += fixed.I(l.Offset.Y)
		if (off.Y + line.Bounds.Min.Y).Floor() > l.Clip.Max.Y {
			break
		}
		if (off.Y + line.Bounds.Max.Y).Ceil() < l.Clip.Min.Y {
			continue
		}
		str := line.Text
		for len(str.Advances) > 0 {
			adv := str.Advances[0]
			if (off.X + adv + line.Bounds.Max.X - line.Width).Ceil() >= l.Clip.Min.X {
				break
			}
			off.X += adv
			_, s := utf8.DecodeRuneInString(str.String)
			str.String = str.String[s:]
			str.Advances = str.Advances[1:]
		}
		n := 0
		endx := off.X
		for i, adv := range str.Advances {
			if (endx + line.Bounds.Min.X).Floor() > l.Clip.Max.X {
				str.String = str.String[:n]
				str.Advances = str.Advances[:i]
				break
			}
			_, s := utf8.DecodeRuneInString(str.String[n:])
			n += s
			endx += adv
		}
		offf := f32.Point{X: float32(off.X) / 64, Y: float32(off.Y) / 64}
		return str, offf, true
	}
	return String{}, f32.Point{}, false
}

func (l Label) Layout(ops *ui.Ops, cs layout.Constraints) layout.Dimens {
	textLayout := l.Face.Layout(l.Text, LayoutOptions{MaxWidth: cs.Width.Max})
	lines := textLayout.Lines
	if max := l.MaxLines; max > 0 && len(lines) > max {
		lines = lines[:max]
	}
	dims := linesDimens(lines)
	dims.Size = cs.Constrain(dims.Size)
	padTop, padBottom := textPadding(lines)
	clip := image.Rectangle{
		Min: image.Point{X: -ui.Inf, Y: -padTop},
		Max: image.Point{X: ui.Inf, Y: dims.Size.Y + padBottom},
	}
	l.it = lineIterator{
		Lines:     lines,
		Clip:      clip,
		Alignment: l.Alignment,
		Width:     dims.Size.X,
	}
	for {
		str, off, ok := l.it.Next()
		if !ok {
			break
		}
		lclip := toRectF(clip).Sub(off)
		ui.PushOp{}.Add(ops)
		ui.TransformOp{Transform: ui.Offset(off)}.Add(ops)
		l.Face.Path(str).Add(ops)
		draw.DrawOp{Rect: lclip}.Add(ops)
		ui.PopOp{}.Add(ops)
	}
	return dims
}

func itof(i int) float32 {
	switch i {
	case ui.Inf:
		return float32(math.Inf(+1))
	case -ui.Inf:
		return float32(math.Inf(-1))
	default:
		return float32(i)
	}
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{X: itof(r.Min.X), Y: itof(r.Min.Y)},
		Max: f32.Point{X: itof(r.Max.X), Y: itof(r.Max.Y)},
	}
}

func textPadding(lines []Line) (padTop int, padBottom int) {
	if len(lines) > 0 {
		first := lines[0]
		if d := -first.Bounds.Min.Y - first.Ascent; d > 0 {
			padTop = d.Ceil()
		}
		last := lines[len(lines)-1]
		if d := last.Bounds.Max.Y - last.Descent; d > 0 {
			padBottom = d.Ceil()
		}
	}
	return
}
