// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"

	"gioui.org/f32"
	"gioui.org/font"
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
	// WrapPolicy configures how displayed text will be broken into lines.
	WrapPolicy text.WrapPolicy
	// LineHeight controls the distance between the baselines of lines of text.
	// If zero, a sensible default will be used.
	LineHeight unit.Sp
	// LineHeightScale applies a scaling factor to the LineHeight. If zero, a
	// sensible default will be used.
	LineHeightScale float32
}

// Layout the label with the given shaper, font, size, text, and material.
func (l Label) Layout(gtx layout.Context, lt *text.Shaper, font font.Font, size unit.Sp, txt string, textMaterial op.CallOp) layout.Dimensions {
	dims, _ := l.LayoutDetailed(gtx, lt, font, size, txt, textMaterial)
	return dims
}

// TextInfo provides metadata about shaped text.
type TextInfo struct {
	// Truncated contains the number of runes of text that are represented by a truncator
	// symbol in the text. If zero, there is no truncator symbol.
	Truncated int
}

// Layout the label with the given shaper, font, size, text, and material, returning metadata about the shaped text.
func (l Label) LayoutDetailed(gtx layout.Context, lt *text.Shaper, font font.Font, size unit.Sp, txt string, textMaterial op.CallOp) (layout.Dimensions, TextInfo) {
	cs := gtx.Constraints
	textSize := fixed.I(gtx.Sp(size))
	lineHeight := fixed.I(gtx.Sp(l.LineHeight))
	lt.LayoutString(text.Parameters{
		Font:            font,
		PxPerEm:         textSize,
		MaxLines:        l.MaxLines,
		Truncator:       l.Truncator,
		Alignment:       l.Alignment,
		WrapPolicy:      l.WrapPolicy,
		MaxWidth:        cs.Max.X,
		MinWidth:        cs.Min.X,
		Locale:          gtx.Locale,
		LineHeight:      lineHeight,
		LineHeightScale: l.LineHeightScale,
	}, txt)
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
	return dims, TextInfo{Truncated: it.truncated}
}

// textIterator computes the bounding box of and paints text.
type textIterator struct {
	// viewport is the rectangle of document coordinates that the iterator is
	// trying to fill with text.
	viewport image.Rectangle
	// maxLines is the maximum number of text lines that should be displayed.
	maxLines int
	// material sets the paint material for the text glyphs. If none is provided
	// the color of the glyphs is undefined and may change unpredictably if the
	// text contains color glyphs.
	material op.CallOp
	// selectedMaterial replaces material for selected glyphs. Zero disables it.
	selectedMaterial op.CallOp
	// selectionRegions are the highlight rectangles in viewport coordinates,
	// ordered from left to right.
	selectionRegions []Region
	// selectionStart and selectionEnd delimit the selected glyphs.
	selectionStart, selectionEnd int
	// glyphIndex is the index of the next glyph.
	glyphIndex int
	// batchSelected reports whether the buffered glyphs are selected.
	batchSelected bool
	// truncated tracks the count of truncated runes in the text.
	truncated int
	// linesSeen tracks the quantity of line endings this iterator has seen.
	linesSeen int
	// lineOff tracks the origin for the glyphs in the current line.
	lineOff f32.Point
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
func (it *textIterator) processGlyph(g text.Glyph, ok bool) (visibleOrBefore bool) {
	if it.maxLines > 0 {
		if g.Flags&text.FlagTruncator != 0 && g.Flags&text.FlagClusterBreak != 0 {
			// A glyph carrying both of these flags provides the count of truncated runes.
			it.truncated = int(g.Runes)
		}
		if g.Flags&text.FlagLineBreak != 0 {
			it.linesSeen++
		}
		if it.linesSeen == it.maxLines && g.Flags&text.FlagParagraphBreak != 0 {
			return false
		}
	}
	// Compute the maximum extent to which glyphs overhang on the horizontal
	// axis.
	if d := g.Bounds.Min.X.Floor(); d < it.padding.Min.X {
		// If the distance between the dot and the left edge of this glyph is
		// less than the current padding, increase the left padding.
		it.padding.Min.X = d
	}
	if d := (g.Bounds.Max.X - g.Advance).Ceil(); d > it.padding.Max.X {
		// If the distance between the dot and the right edge of this glyph
		// minus the logical advance of this glyph is greater than the current
		// padding, increase the right padding.
		it.padding.Max.X = d
	}
	if d := (g.Bounds.Min.Y + g.Ascent).Floor(); d < it.padding.Min.Y {
		// If the distance between the dot and the top of this glyph is greater
		// than the ascent of the glyph, increase the top padding.
		it.padding.Min.Y = d
	}
	if d := (g.Bounds.Max.Y - g.Descent).Ceil(); d > it.padding.Max.Y {
		// If the distance between the dot and the bottom of this glyph is greater
		// than the descent of the glyph, increase the bottom padding.
		it.padding.Max.Y = d
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
	return ok && !below
}

func fixedToFloat(i fixed.Int26_6) float32 {
	return float32(i) / 64.0
}

// paintGlyph buffers up and paints text glyphs. It should be invoked iteratively upon each glyph
// until it returns false. The line parameter should be a slice with
// a backing array of sufficient size to buffer multiple glyphs.
// A modified slice will be returned with each invocation, and is
// expected to be passed back in on the following invocation.
// This design is awkward, but prevents the line slice from escaping
// to the heap.
func (it *textIterator) paintGlyph(gtx layout.Context, shaper *text.Shaper, glyph text.Glyph, line []text.Glyph) ([]text.Glyph, bool) {
	visibleOrBefore := it.processGlyph(glyph, true)
	selected := it.selectedMaterial != (op.CallOp{}) &&
		it.glyphIndex >= it.selectionStart && it.glyphIndex < it.selectionEnd
	it.glyphIndex++
	if selected != it.batchSelected {
		line = it.flush(gtx, shaper, line)
		it.batchSelected = selected
	}
	if it.visible {
		if len(line) == 0 {
			it.lineOff = f32.Point{X: fixedToFloat(glyph.X), Y: float32(glyph.Y)}.Sub(layout.FPt(it.viewport.Min))
		}
		line = append(line, glyph)
	}
	if glyph.Flags&text.FlagLineBreak != 0 || cap(line)-len(line) == 0 || !visibleOrBefore {
		line = it.flush(gtx, shaper, line)
	}
	return line, visibleOrBefore
}

// flush paints the buffered glyphs and resets the buffer.
func (it *textIterator) flush(gtx layout.Context, shaper *text.Shaper, line []text.Glyph) []text.Glyph {
	if len(line) == 0 {
		return line[:0]
	}
	path := shaper.Shape(line)
	if it.batchSelected {
		it.paintSelectedBatch(gtx, path, int(line[0].Y))
	} else {
		it.paintPath(gtx, path, it.material)
	}
	if call := shaper.Bitmaps(line); call != (op.CallOp{}) {
		// Color glyphs are bitmaps, so they are never inverted.
		t := op.Affine(f32.AffineId().Offset(it.lineOff)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		t.Pop()
	}
	return line[:0]
}

// paintPath fills the shaped glyphs with material.
func (it *textIterator) paintPath(gtx layout.Context, path clip.PathSpec, material op.CallOp) {
	t := op.Affine(f32.AffineId().Offset(it.lineOff)).Push(gtx.Ops)
	outline := clip.Outline{Path: path}.Op().Push(gtx.Ops)
	material.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	outline.Pop()
	t.Pop()
}

// paintSelectedBatch paints one line of selected glyphs: the parts covered by the
// selection rectangles get the selected material, everything else the regular one.
// They are painted separately so translucent colors are blended once.
//
// Rectangles are viewport-relative while glyphs use document coordinates, so the
// line baseline is converted before matching them.
func (it *textIterator) paintSelectedBatch(gtx layout.Context, path clip.PathSpec, baseline int) {
	viewport := it.viewport.Sub(it.viewport.Min)
	viewport.Min = viewport.Min.Add(it.padding.Min)
	viewport.Max = viewport.Max.Add(it.padding.Max)
	paintRect := func(rect image.Rectangle, material op.CallOp) {
		if rect.Empty() {
			return
		}
		area := clip.Rect(rect).Push(gtx.Ops)
		it.paintPath(gtx, path, material)
		area.Pop()
	}
	x := viewport.Min.X
	var top, bottom int
	found := false
	for _, region := range it.selectionRegions {
		if region.Bounds.Max.Y-region.Baseline != baseline-it.viewport.Min.Y {
			continue
		}
		bounds := region.Bounds.Intersect(viewport)
		if bounds.Empty() {
			continue
		}
		if !found {
			top, bottom = bounds.Min.Y, bounds.Max.Y
			found = true
			paintRect(image.Rect(viewport.Min.X, viewport.Min.Y, viewport.Max.X, top), it.material)
			paintRect(image.Rect(viewport.Min.X, bottom, viewport.Max.X, viewport.Max.Y), it.material)
		}
		paintRect(image.Rect(x, top, max(x, bounds.Min.X), bottom), it.material)
		bounds.Min.X = max(x, bounds.Min.X)
		paintRect(bounds, it.selectedMaterial)
		x = max(x, bounds.Max.X)
	}
	if found {
		paintRect(image.Rect(x, top, viewport.Max.X, bottom), it.material)
	} else {
		it.paintPath(gtx, path, it.material)
	}
}
