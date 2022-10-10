// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"

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

func (l Label) Layout(gtx layout.Context, lt *text.Shaper, font text.Font, size unit.Sp, txt string) layout.Dimensions {
	cs := gtx.Constraints
	textSize := fixed.I(gtx.Sp(size))
	lt.LayoutString(text.Parameters{
		Font:      font,
		PxPerEm:   textSize,
		MaxLines:  l.MaxLines,
		Alignment: l.Alignment,
	}, cs.Min.X, cs.Max.X, gtx.Locale, txt)
	m := op.Record(gtx.Ops)
	viewport := image.Rectangle{Max: cs.Max}
	it := textIterator{viewport: viewport}
	semantic.LabelOp(txt).Add(gtx.Ops)
	var gs [32]text.Glyph
	line := gs[:0]
	var lineOff image.Point
	for it.Glyph(lt.NextGlyph()) {
		if it.visible {
			if len(line) == 0 {
				lineOff = image.Point{X: it.g.X.Floor(), Y: int(it.g.Y)}
			}
			line = append(line, it.g)
		}
		if it.g.Flags&text.FlagLineBreak > 0 || cap(line)-len(line) == 0 {
			t := op.Offset(lineOff).Push(gtx.Ops)
			op := clip.Outline{Path: lt.Shape(line)}.Op().Push(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			op.Pop()
			t.Pop()
			line = line[:0]
		}
	}
	call := m.Stop()
	viewport.Min = viewport.Min.Add(it.padding.Min)
	viewport.Max = viewport.Max.Add(it.padding.Max)
	clipStack := clip.Rect(viewport).Push(gtx.Ops)
	call.Add(gtx.Ops)
	dims := layout.Dimensions{Size: it.bounds.Size()}
	dims.Size = cs.Constrain(dims.Size)
	dims.Baseline = dims.Size.Y - it.baseline
	clipStack.Pop()
	return dims
}

func r2p(r clip.Rect) clip.Op {
	return clip.Stroke{Path: r.Path(), Width: 1}.Op()
}

type textIterator struct {
	g        text.Glyph
	viewport image.Rectangle
	padding  image.Rectangle
	bounds   image.Rectangle
	visible  bool
	first    bool
	baseline int
}

func (t *textIterator) Glyph(g text.Glyph, ok bool) bool {
	t.g = g
	bounds := image.Rectangle{
		Min: image.Pt(g.Bounds.Min.X.Floor(), g.Bounds.Min.Y.Floor()),
		Max: image.Pt(g.Bounds.Max.X.Ceil(), g.Bounds.Max.Y.Ceil()),
	}
	// Compute the maximum extent to which glyphs overhang on the horizontal
	// axis.
	if d := g.Bounds.Min.X.Floor(); d < t.padding.Min.X {
		t.padding.Min.X = d
	}
	if d := (g.Bounds.Max.X - g.Advance).Ceil(); d > t.padding.Max.X {
		t.padding.Max.X = d
	}
	// Convert the bounds from dot-relative coordinates to document coordinates.
	bounds = bounds.Add(image.Pt(g.X.Round(), int(g.Y)))
	if !t.first {
		t.first = true
		t.baseline = int(g.Y)
		t.bounds = bounds
	}

	above := bounds.Max.Y < t.viewport.Min.Y
	below := bounds.Min.Y > t.viewport.Max.Y
	left := bounds.Max.X < t.viewport.Min.X
	right := bounds.Min.X > t.viewport.Max.X
	t.visible = !above && !below && !left && !right
	if t.visible {
		t.bounds.Min.X = min(t.bounds.Min.X, bounds.Min.X)
		t.bounds.Min.Y = min(t.bounds.Min.Y, int(g.Y)-g.Ascent.Ceil())
		t.bounds.Max.X = max(t.bounds.Max.X, bounds.Max.X)
		t.bounds.Max.Y = max(t.bounds.Max.Y, int(g.Y)+g.Descent.Ceil())
	}
	if t.bounds.Dy() == 0 {
		t.bounds.Min.Y = -g.Ascent.Ceil()
		t.bounds.Max.Y = g.Descent.Ceil()
	}
	return ok && !below
}
