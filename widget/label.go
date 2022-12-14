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
	var glyphs [32]text.Glyph
	line := glyphs[:0]
	for g, ok := lt.NextGlyph(); ok; g, ok = lt.NextGlyph() {
		var ok bool
		if line, ok = it.paintGlyph(gtx, lt, g, line); !ok {
			break
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

// textIterator computes the bounding box of and paints text.
type textIterator struct {
	// viewport is the rectangle of document coordinates that the iterator is
	// trying to fill with text.
	viewport image.Rectangle

	// lineOff tracks the origin for the glyphs in the current line.
	lineOff image.Point
	// padding is the space needed outside of the bounds of the text to ensure no
	// part of a glyph is clipped.
	padding image.Rectangle
	// bounds is the logical bounding box of the text.
	bounds image.Rectangle
	// visible tracks whether the most recently iterated glyph is visible within
	// the viewport.
	visible bool
	// first tracks whether the iterator has processed a glyph yet.
	first bool
	// baseline tracks the location of the first line of text's baseline.
	baseline int
}

// processGlyph checks whether the glyph is visible within the iterator's configured
// viewport and (if so) updates the iterator's text dimensions to include the glyph.
func (it *textIterator) processGlyph(g text.Glyph, ok bool) (_ text.Glyph, visibleOrBefore bool) {
	bounds := image.Rectangle{
		Min: image.Pt(g.Bounds.Min.X.Floor(), g.Bounds.Min.Y.Floor()),
		Max: image.Pt(g.Bounds.Max.X.Ceil(), g.Bounds.Max.Y.Ceil()),
	}
	// Compute the maximum extent to which glyphs overhang on the horizontal
	// axis.
	if d := g.Bounds.Min.X.Floor(); d < it.padding.Min.X {
		it.padding.Min.X = d
	}
	if d := (g.Bounds.Max.X - g.Advance).Ceil(); d > it.padding.Max.X {
		it.padding.Max.X = d
	}
	// Convert the bounds from dot-relative coordinates to document coordinates.
	bounds = bounds.Add(image.Pt(g.X.Round(), int(g.Y)))
	if !it.first {
		it.first = true
		it.baseline = int(g.Y)
		it.bounds = bounds
	}

	lineTop := int(g.Y) - g.Ascent.Ceil()
	lineBottom := int(g.Y) + g.Descent.Ceil()
	above := lineBottom < it.viewport.Min.Y
	below := lineTop > it.viewport.Max.Y
	left := bounds.Max.X < it.viewport.Min.X
	right := bounds.Min.X > it.viewport.Max.X
	it.visible = !above && !below && !left && !right
	if it.visible {
		it.bounds.Min.X = min(it.bounds.Min.X, bounds.Min.X)
		it.bounds.Min.Y = min(it.bounds.Min.Y, lineTop)
		it.bounds.Max.X = max(it.bounds.Max.X, bounds.Max.X)
		it.bounds.Max.Y = max(it.bounds.Max.Y, lineBottom)
	}
	return g, ok && !below
}

// paintGlyph buffers up and paints text glyphs. It should be invoked iteratively upon each glyph
// until it returns false. The line parameter should be a slice with
// a backing array of sufficient size to buffer multiple glyphs.
// A modified slice will be returned with each invocation, and is
// expected to be passed back in on the following invocation.
// This design is awkward, but prevents the line slice from escaping
// to the heap.
func (it *textIterator) paintGlyph(gtx layout.Context, shaper *text.Shaper, glyph text.Glyph, line []text.Glyph) ([]text.Glyph, bool) {
	_, visibleOrBefore := it.processGlyph(glyph, true)
	done := !visibleOrBefore
	if it.visible {
		if len(line) == 0 {
			it.lineOff = image.Point{X: glyph.X.Floor(), Y: int(glyph.Y)}.Sub(it.viewport.Min)
		}
		line = append(line, glyph)
	}
	if glyph.Flags&text.FlagLineBreak > 0 || cap(line)-len(line) == 0 || done {
		t := op.Offset(it.lineOff).Push(gtx.Ops)
		op := clip.Outline{Path: shaper.Shape(line)}.Op().Push(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		op.Pop()
		t.Pop()
		line = line[:0]
	}
	return line, !done
}
