// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
)

// GlyphMetrics holds the screen dimensions of a single character.
type GlyphMetrics struct {
	X      int // left edge X (in pixels)
	Y      int // top edge Y (in pixels)
	Width  int // glyph width in pixels
	Height int // glyph height in pixels (ascent + descent)
	Line   int // line index (0-based)
}

// CaretPosition describes where the caret sits relative to a glyph.
type CaretPosition int

const (
	CaretBefore CaretPosition = iota
	CaretAfter
)

// CaretCoords is the screen position of the caret.
type CaretCoords struct {
	X      int
	Y      int
	Height int
}

// GlyphMeasurer provides font-specific width/height for each rune + style.
type GlyphMeasurer interface {
	MeasureGlyph(ch rune, style CharStyle) (width, height int)
}

// CaretLayout computes per-character positions for a FormattedBuffer.
type CaretLayout struct {
	glyphs []GlyphMetrics
	lines  []LineInfo
}

// LineInfo holds layout information for a single line.
type LineInfo struct {
	StartIdx int
	EndIdx   int
	Y        int
	Height   int
	Width    int
}

// ComputeLayout calculates per-character positions for the given buffer.
// Text is wrapped at word boundaries (spaces) to match the editor's rendering.
func ComputeLayout(buf *FormattedBuffer, measurer GlyphMeasurer, maxWidth int, startX, startY int, defaultStyle CharStyle) CaretLayout {
	if buf == nil || buf.Len() == 0 {
		return CaretLayout{}
	}

	// Collect all runes and their styles, preserving word boundaries
	type runeInfo struct {
		ch    rune
		style CharStyle
	}
	var allRunes []runeInfo
	for _, run := range buf.Runs() {
		for _, ch := range run.Text {
			s := run.Style
			if s.S == 0 {
				s.S = defaultStyle.S
			}
			if s.F == "" {
				s.F = defaultStyle.F
			}
			allRunes = append(allRunes, runeInfo{ch: ch, style: s})
		}
	}

	if len(allRunes) == 0 {
		return CaretLayout{}
	}

	// Measure and position each glyph with word-level wrapping
	metrics := make([]GlyphMetrics, len(allRunes))
	lineMaxH := make(map[int]int)

	curX := startX
	curY := startY
	curLine := 0
	// track the start X of the current word to handle word wrapping
	wordStartX := startX
	wordStartIdx := 0
	wordMaxH := 0

	flushWord := func(endIdx int) {
		// Word finished: check if it exceeds maxWidth
		if maxWidth > 0 && wordMaxH > 0 {
			// Calculate word width
			wordWidth := 0
			for i := wordStartIdx; i < endIdx; i++ {
				wordWidth += metrics[i].Width
			}
			// If word would exceed line, wrap before this word (if we have something on the current line)
			if wordStartX+wordWidth > startX+maxWidth && wordStartX > startX {
				curLine++
				curX = startX
				curY += lineMaxH[curLine-1]
				// Reposition all glyphs in this word to the new line
				for i := wordStartIdx; i < endIdx; i++ {
					metrics[i].Y = curY
					metrics[i].Line = curLine
				}
			}
		}
		// Update curX to end of word
		if endIdx > 0 && endIdx <= len(metrics) {
			last := metrics[endIdx-1]
			curX = last.X + last.Width
			wordStartX = curX
		}
	}

	for i, ri := range allRunes {
		w, h := measurer.MeasureGlyph(ri.ch, ri.style)
		if w <= 0 {
			w = 1
		}
		if h <= 0 {
			h = int(defaultStyle.S)
			if h <= 0 {
				h = 14
			}
		}

		// Check for line break: explicit newline
		if ri.ch == '\n' {
			metrics[i] = GlyphMetrics{
				X:      curX,
				Y:      curY,
				Width:  0,
				Height: h,
				Line:   curLine,
			}
			if h > lineMaxH[curLine] {
				lineMaxH[curLine] = h
			}
			// Flush current word and start new line
			flushWord(i)
			curLine++
			curX = startX
			curY += lineMaxH[curLine-1]
			wordStartX = startX
			wordStartIdx = i + 1
			wordMaxH = 0
			continue
		}

		metrics[i] = GlyphMetrics{
			X:      curX,
			Y:      curY,
			Width:  w,
			Height: h,
			Line:   curLine,
		}

		if h > lineMaxH[curLine] {
			lineMaxH[curLine] = h
		}
		if h > wordMaxH {
			wordMaxH = h
		}

		curX += w

		// Word boundary: space or end of text
		isSpace := ri.ch == ' ' || ri.ch == '\t'
		isLast := i == len(allRunes)-1
		if isSpace || isLast {
			endIdx := i + 1
			if isLast && !isSpace {
				endIdx = i + 1
			}
			flushWord(endIdx)
			if isSpace {
				wordStartIdx = i + 1
				wordMaxH = 0
			}
		}
	}

	// Build line info
	lines := make([]LineInfo, 0, curLine+1)
	for l := 0; l <= curLine; l++ {
		start := -1
		end := len(metrics)
		lh := lineMaxH[l]

		for i, g := range metrics {
			if g.Line == l {
				if start == -1 {
					start = i
				}
				end = i + 1
			}
		}
		if start == -1 {
			start = 0
			end = 0
		}

		var ly int
		if start < len(metrics) {
			ly = metrics[start].Y
		}

		lw := 0
		for i := start; i < end; i++ {
			lw += metrics[i].Width
		}

		lines = append(lines, LineInfo{
			StartIdx: start,
			EndIdx:   end,
			Y:        ly,
			Height:   lh,
			Width:    lw,
		})
	}

	return CaretLayout{glyphs: metrics, lines: lines}
}

// GlyphAt returns the metrics for the glyph at the given rune index.
func (cl *CaretLayout) GlyphAt(runeIdx int) GlyphMetrics {
	if runeIdx < 0 || runeIdx >= len(cl.glyphs) {
		return GlyphMetrics{}
	}
	return cl.glyphs[runeIdx]
}

// Len returns the total number of glyphs.
func (cl *CaretLayout) Len() int {
	return len(cl.glyphs)
}

// Lines returns the line layout info.
func (cl *CaretLayout) Lines() []LineInfo {
	return cl.lines
}

// LineCount returns the number of lines.
func (cl *CaretLayout) LineCount() int {
	return len(cl.lines)
}

// CaretCoordsAt returns the screen coordinates for the caret at the given rune position.
func (cl *CaretLayout) CaretCoordsAt(runeIdx int) CaretCoords {
	if len(cl.glyphs) == 0 {
		return CaretCoords{}
	}

	if runeIdx < 0 {
		runeIdx = 0
	}
	if runeIdx > len(cl.glyphs) {
		runeIdx = len(cl.glyphs)
	}

	if runeIdx == 0 {
		g := cl.glyphs[0]
		return CaretCoords{X: g.X, Y: g.Y, Height: g.Height}
	}

	if runeIdx >= len(cl.glyphs) {
		// End of text: find the last visible glyph and place caret after it
		for i := len(cl.glyphs) - 1; i >= 0; i-- {
			g := cl.glyphs[i]
			if g.Width > 0 {
				return CaretCoords{X: g.X + g.Width, Y: g.Y, Height: g.Height}
			}
		}
		g := cl.glyphs[len(cl.glyphs)-1]
		return CaretCoords{X: g.X, Y: g.Y, Height: g.Height}
	}

	prev := cl.glyphs[runeIdx-1]
	next := cl.glyphs[runeIdx]

	// Handle newlines and zero-width glyphs: place caret at end of previous visible glyph
	// on the same line, or at start of next line if prev is a newline
	if next.Width <= 0 || prev.Y != next.Y {
		// Find the last visible glyph before runeIdx on the same line as prev
		for i := runeIdx - 1; i >= 0; i-- {
			g := cl.glyphs[i]
			if g.Width > 0 {
				if g.Y == prev.Y {
					// Same line: caret at end of this glyph
					return CaretCoords{X: g.X + g.Width, Y: g.Y, Height: g.Height}
				}
				// Different line: caret at start of current line
				return CaretCoords{X: next.X, Y: next.Y, Height: next.Height}
			}
		}
		// All previous glyphs are zero-width: use next glyph's position
		return CaretCoords{X: next.X, Y: next.Y, Height: next.Height}
	}

	h := prev.Height
	if next.Height > h {
		h = next.Height
	}
	y := prev.Y
	if next.Y < y {
		y = next.Y
	}

	return CaretCoords{X: next.X, Y: y, Height: h}
}// HitTest maps a local screen point to the nearest insertion boundary.
// The boundary is chosen by each glyph midpoint, not by the nearest glyph edge;
// this makes clicks inside a glyph and between glyphs deterministic.
func (cl *CaretLayout) HitTest(screenX, screenY int) int {
	if len(cl.glyphs) == 0 {
		return 0
	}

	lineIdx := 0
	bestLineDist := 1 << 30
	if len(cl.lines) > 0 {
		for i, line := range cl.lines {
			center := line.Y + line.Height/2
			d := screenY - center
			if d < 0 {
				d = -d
			}
			if d < bestLineDist {
				bestLineDist = d
				lineIdx = i
			}
		}
	}

	line := LineInfo{StartIdx: 0, EndIdx: len(cl.glyphs)}
	if lineIdx < len(cl.lines) {
		line = cl.lines[lineIdx]
	}
	start, end := line.StartIdx, line.EndIdx
	if start < 0 { start = 0 }
	if end > len(cl.glyphs) { end = len(cl.glyphs) }
	if start >= end {
		return start
	}

	for i := start; i < end; i++ {
		g := cl.glyphs[i]
		if g.Width <= 0 {
			if screenX <= g.X {
				return i
			}
			continue
		}
		if screenX < g.X+g.Width/2 {
			return i
		}
	}
	return end
}

// TotalHeight returns the total height of all lines.
func (cl *CaretLayout) TotalHeight() int {
	if len(cl.lines) == 0 {
		return 0
	}
	last := cl.lines[len(cl.lines)-1]
	return last.Y + last.Height
}

// BoundingBox returns the bounding rectangle of all glyphs.
func (cl *CaretLayout) BoundingBox() image.Rectangle {
	if len(cl.glyphs) == 0 {
		return image.Rectangle{}
	}
	minX := cl.glyphs[0].X
	minY := cl.glyphs[0].Y
	maxX := 0
	maxY := 0

	for _, g := range cl.glyphs {
		if g.X < minX {
			minX = g.X
		}
		if g.Y < minY {
			minY = g.Y
		}
		if g.X+g.Width > maxX {
			maxX = g.X + g.Width
		}
		if g.Y+g.Height > maxY {
			maxY = g.Y + g.Height
		}
	}

	return image.Rect(minX, minY, maxX, maxY)
}

// MoveToLine moves the caret position to the nearest glyph on the target line
// (relative: -1 for previous, +1 for next) at approximately the same X coordinate.
// Returns the new rune index, or the original index if no move is possible.
func (cl *CaretLayout) MoveToLine(currentPos int, delta int) int {
	if len(cl.glyphs) == 0 || len(cl.lines) <= 1 {
		return currentPos
	}

	// Find current line
	curLine := 0
	if currentPos >= 0 && currentPos < len(cl.glyphs) {
		curLine = cl.glyphs[currentPos].Line
	}

	targetLine := curLine + delta
	if targetLine < 0 || targetLine >= len(cl.lines) {
		return currentPos
	}

	// Get current X position
	curX := 0
	if currentPos >= 0 && currentPos < len(cl.glyphs) {
		curX = cl.glyphs[currentPos].X
	}

	// Find closest glyph on target line
	line := cl.lines[targetLine]
	bestIdx := line.StartIdx
	bestDist := 1 << 30

	for i := line.StartIdx; i < line.EndIdx && i < len(cl.glyphs); i++ {
		g := cl.glyphs[i]
		dx := g.X - curX
		if dx < 0 {
			dx = -dx
		}
		if dx < bestDist {
			bestDist = dx
			bestIdx = i
		}
	}

	return bestIdx
}
