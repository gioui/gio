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

// Label is a widget for laying out and drawing text. Labels are always
// non-interactive text. They cannot be selected or copied.
type Label struct {
	// Alignment specifies the text alignment.
	Alignment text.Alignment
	// MaxLines limits the number of lines. Zero means no limit.
	MaxLines int
	// Truncator is the text that will be shown at the end of the final
	// line if MaxLines is exceeded. Defaults to "…" if empty.
	Truncator string
}

// Layout the label with the given shaper, font, size, text, and material.
func (l Label) Layout(gtx layout.Context, lt *text.Shaper, font text.Font, size unit.Sp, txt string, textMaterial op.CallOp) layout.Dimensions {
	cs := gtx.Constraints
	textSize := fixed.I(gtx.Sp(size))
	lt.LayoutString(text.Parameters{
		Font:      font,
		PxPerEm:   textSize,
		MaxLines:  l.MaxLines,
		Truncator: l.Truncator,
		Alignment: l.Alignment,
	}, cs.Min.X, cs.Max.X, gtx.Locale, txt)
	m := op.Record(gtx.Ops)
	viewport := image.Rectangle{Max: cs.Max}
	it := textIterator{
		viewport: viewport,
		maxLines: l.MaxLines,
		material: textMaterial,
	}
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
	// maxLines is the maximum number of text lines that should be displayed.
	maxLines int
	// material sets the paint material for the text glyphs. If none is provided
	// the glyphs will be invisible.
	material op.CallOp

	// linesSeen tracks the quantity of line endings this iterator has seen.
	linesSeen int
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
	if it.maxLines > 0 {
		if g.Flags&text.FlagLineBreak != 0 {
			it.linesSeen++
		}
		if it.linesSeen == it.maxLines && g.Flags&text.FlagParagraphBreak != 0 {
			return g, false
		}
	}
	// Compute the maximum extent to which glyphs overhang on the horizontal
	// axis.
	if d := g.Bounds.Min.X.Floor(); d < it.padding.Min.X {
		it.padding.Min.X = d
	}
	if d := (g.Bounds.Max.X - g.Advance).Ceil(); d > it.padding.Max.X {
		it.padding.Max.X = d
	}
	logicalBounds := image.Rectangle{
		Min: image.Pt(g.X.Floor(), int(g.Y)-g.Ascent.Ceil()),
		Max: image.Pt((g.X + g.Advance).Ceil(), int(g.Y)+g.Descent.Ceil()),
	}
	if !it.first {
		it.first = true
		it.baseline = int(g.Y)
		it.bounds = logicalBounds
	}

	above := logicalBounds.Max.Y < it.viewport.Min.Y
	below := logicalBounds.Min.Y > it.viewport.Max.Y
	left := logicalBounds.Max.X < it.viewport.Min.X
	right := logicalBounds.Min.X > it.viewport.Max.X
	it.visible = !above && !below && !left && !right
	if it.visible {
		it.bounds.Min.X = min(it.bounds.Min.X, logicalBounds.Min.X)
		it.bounds.Min.Y = min(it.bounds.Min.Y, logicalBounds.Min.Y)
		it.bounds.Max.X = max(it.bounds.Max.X, logicalBounds.Max.X)
		it.bounds.Max.Y = max(it.bounds.Max.Y, logicalBounds.Max.Y)
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
	if it.visible {
		if len(line) == 0 {
			it.lineOff = image.Point{X: glyph.X.Floor(), Y: int(glyph.Y)}.Sub(it.viewport.Min)
		}
		line = append(line, glyph)
	}
	if glyph.Flags&text.FlagLineBreak != 0 || cap(line)-len(line) == 0 || !visibleOrBefore {
		t := op.Offset(it.lineOff).Push(gtx.Ops)
		path := shaper.Shape(line)
		outline := clip.Outline{Path: path}.Op().Push(gtx.Ops)
		it.material.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		outline.Pop()
		if call := shaper.Bitmaps(line); call != (op.CallOp{}) {
			call.Add(gtx.Ops)
		}
		t.Pop()
		line = line[:0]
	}
	return line, visibleOrBefore
}
