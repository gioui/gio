// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"math"
	"sort"

	"gioui.org/text"
	"golang.org/x/image/math/fixed"
)

type lineInfo struct {
	xOff            fixed.Int26_6
	yOff            int
	width           fixed.Int26_6
	ascent, descent fixed.Int26_6
	glyphs          int
}

type glyphIndex struct {
	glyphs []text.Glyph
	// positions contain all possible caret positions, sorted by rune index.
	positions []combinedPos
	// lines contains metadata about the size and position of each line of
	// text.
	lines []lineInfo

	// currentLineMin and currentLineMax track the dimensions of the line
	// that is being indexed.
	currentLineMin, currentLineMax fixed.Int26_6
	// currentLineGlyphs tracks how many glyphs are contained within the
	// line that is being indexed.
	currentLineGlyphs int
	// pos tracks attributes of the next valid cursor position within the indexed
	// text.
	pos combinedPos
	// prog tracks the current glyph text progression to detect bidi changes.
	prog text.Flags
	// clusterAdvance accumulates the advances of glyphs in a glyph cluster.
	clusterAdvance fixed.Int26_6
	// skipPrior controls whether a text position is inserted "before" the
	// next glyph. Usually this should not happen, but the boundaries of
	// lines and bidi runs require it.
	skipPrior bool
}

// screenPos represents a character position in text line and column numbers,
// not pixels.
type screenPos struct {
	// col is the column, measured in runes.
	// FIXME: we only ever use col for start or end of lines.
	// We don't need accurate accounting, so can we get rid of it?
	col  int
	line int
}

// combinedPos is a point in the editor.
type combinedPos struct {
	// runes is the offset in runes.
	runes int

	lineCol screenPos

	// Pixel coordinates
	x fixed.Int26_6
	y int

	ascent, descent fixed.Int26_6

	// runIndex tracks which run this position is within, counted each time
	// the index processes an end of run marker.
	runIndex int
	// towardOrigin tracks whether this glyph's run is progressing toward the
	// origin or away from it.
	towardOrigin bool
}

// incrementPosition returns the next position after pos (if any). Pos _must_ be
// an unmodified position acquired from one of the closest* methods. If eof is
// true, there was no next position.
func (g *glyphIndex) incrementPosition(pos combinedPos) (next combinedPos, eof bool) {
	candidate, index := g.closestToRune(pos.runes)
	for candidate != pos && index+1 < len(g.positions) {
		index++
		candidate = g.positions[index]
	}
	if index+1 < len(g.positions) {
		return g.positions[index+1], false
	}
	return candidate, true

}

// Glyph indexes the provided glyph, generating text cursor positions for it.
func (g *glyphIndex) Glyph(gl text.Glyph) {
	g.glyphs = append(g.glyphs, gl)
	g.currentLineGlyphs++
	if len(g.positions) == 0 {
		// First-iteration setup.
		g.currentLineMin = math.MaxInt32
		g.currentLineMax = 0
	}
	if gl.X < g.currentLineMin {
		g.currentLineMin = gl.X
	}
	if end := gl.X + gl.Advance; end > g.currentLineMax {
		g.currentLineMax = end
	}
	if !g.skipPrior || gl.Flags&text.FlagTowardOrigin != g.prog || gl.Flags&text.FlagParagraphStart != 0 {
		// Set the new text progression based on that of the first glyph.
		g.prog = gl.Flags & text.FlagTowardOrigin
		g.pos.towardOrigin = g.prog == text.FlagTowardOrigin
		// Create the text position prior to the first glyph.
		pos := g.pos
		pos.x = gl.X
		pos.y = int(gl.Y)
		pos.ascent = gl.Ascent
		pos.descent = gl.Descent
		if pos.towardOrigin {
			pos.x += gl.Advance
		}
		g.pos = pos
		g.positions = append(g.positions, pos)
		g.skipPrior = true
	}
	needsNewLine := gl.Flags&text.FlagLineBreak != 0
	needsNewRun := gl.Flags&text.FlagRunBreak != 0
	breaksParagraph := gl.Flags&text.FlagParagraphBreak != 0

	// We should insert new positions if the glyph we're processing terminates
	// a glyph cluster.
	insertPositionAfter := gl.Flags&text.FlagClusterBreak != 0 && !breaksParagraph && gl.Runes > 0
	if breaksParagraph {
		// Paragraph breaking clusters shouldn't have positions generated for both
		// sides of them. They're always zero-width, so doing so would
		// create two visually identical cursor positions. Just reset
		// cluster state, increment by their runes, and move on to the
		// next glyph.
		g.clusterAdvance = 0
		g.pos.runes += int(gl.Runes)
	}
	// Always track the cumulative advance added by the glyph, even if it
	// doesn't terminate a cluster itself.
	g.clusterAdvance += gl.Advance
	if insertPositionAfter {
		// Construct the text position _after_ gl.
		pos := g.pos
		pos.y = int(gl.Y)
		pos.ascent = gl.Ascent
		pos.descent = gl.Descent
		width := g.clusterAdvance
		perRune := width / fixed.Int26_6(gl.Runes)
		adjust := fixed.Int26_6(0)
		if pos.towardOrigin {
			// If RTL, subtract increments from the width of the cluster
			// instead of adding.
			adjust = width
			perRune = -perRune
		}
		for i := 1; i <= int(gl.Runes); i++ {
			pos.x = gl.X + adjust + perRune*fixed.Int26_6(i)
			pos.runes++
			pos.lineCol.col++
			g.positions = append(g.positions, pos)
		}
		g.pos = pos
		g.clusterAdvance = 0
	}
	if needsNewRun {
		g.pos.runIndex++
	}
	if needsNewLine {
		g.lines = append(g.lines, lineInfo{
			xOff:    g.currentLineMin,
			yOff:    int(gl.Y),
			width:   g.currentLineMax - g.currentLineMin,
			ascent:  g.positions[len(g.positions)-1].ascent,
			descent: g.positions[len(g.positions)-1].descent,
			glyphs:  g.currentLineGlyphs,
		})
		g.pos.lineCol.line++
		g.pos.lineCol.col = 0
		g.pos.runIndex = 0
		g.currentLineMin = math.MaxInt32
		g.currentLineMax = 0
		g.currentLineGlyphs = 0
		g.skipPrior = false
	}
}

func (g *glyphIndex) closestToRune(runeIdx int) (combinedPos, int) {
	if len(g.positions) == 0 {
		return combinedPos{}, 0
	}
	i := sort.Search(len(g.positions), func(i int) bool {
		pos := g.positions[i]
		return pos.runes >= runeIdx
	})
	if i > 0 {
		i--
	}
	closest := g.positions[i]
	closestI := i
	for ; i < len(g.positions); i++ {
		if g.positions[i].runes == runeIdx {
			return g.positions[i], i
		}
	}
	return closest, closestI
}

func (g *glyphIndex) closestToLineCol(lineCol screenPos) combinedPos {
	if len(g.positions) == 0 {
		return combinedPos{}
	}
	i := sort.Search(len(g.positions), func(i int) bool {
		pos := g.positions[i]
		return pos.lineCol.line > lineCol.line || (pos.lineCol.line == lineCol.line && pos.lineCol.col >= lineCol.col)
	})
	if i > 0 {
		i--
	}
	prior := g.positions[i]
	if i+1 >= len(g.positions) {
		return prior
	}
	next := g.positions[i+1]
	if next.lineCol != lineCol {
		return prior
	}
	return next
}

func dist(a, b fixed.Int26_6) fixed.Int26_6 {
	if a > b {
		return a - b
	}
	return b - a
}

func (g *glyphIndex) closestToXY(x fixed.Int26_6, y int) combinedPos {
	if len(g.positions) == 0 {
		return combinedPos{}
	}
	i := sort.Search(len(g.positions), func(i int) bool {
		pos := g.positions[i]
		return pos.y+pos.descent.Round() >= y
	})
	// If no position was greater than the provided Y, the text is too
	// short. Return either the last position or (if there are no
	// positions) the zero position.
	if i == len(g.positions) {
		return g.positions[i-1]
	}
	first := g.positions[i]
	// Find the best X coordinate.
	closest := i
	closestDist := dist(first.x, x)
	line := first.lineCol.line
	// NOTE(whereswaldon): there isn't a simple way to accelerate this. Bidi text means that the x coordinates
	// for positions have no fixed relationship. In the future, we can consider sorting the positions
	// on a line by their x coordinate and caching that. It'll be a one-time O(nlogn) per line, but
	// subsequent uses of this function for that line become O(logn). Right now it's always O(n).
	for i := i + 1; i < len(g.positions) && g.positions[i].lineCol.line == line; i++ {
		candidate := g.positions[i]
		distance := dist(candidate.x, x)
		// If we are *really* close to the current position candidate, just choose it.
		if distance.Round() == 0 {
			return g.positions[i]
		}
		if distance < closestDist {
			closestDist = distance
			closest = i
		}
	}
	return g.positions[closest]
}

// makeRegion creates a text-aligned rectangle from start to end. The vertical
// dimensions of the rectangle are derived from the provided line's ascent and
// descent, and the y offset of the line's baseline is provided as y.
func makeRegion(line lineInfo, y int, start, end fixed.Int26_6) region {
	if start > end {
		start, end = end, start
	}
	dotStart := image.Pt(start.Round(), y)
	dotEnd := image.Pt(end.Round(), y)
	return region{
		bounds: image.Rectangle{
			Min: dotStart.Sub(image.Point{Y: line.ascent.Ceil()}),
			Max: dotEnd.Add(image.Point{Y: line.descent.Floor()}),
		},
		baseline: line.descent.Floor(),
	}
}

// region describes an area of interest within shaped text.
type region struct {
	// bounds is the coordinates of the bounding box in document space.
	bounds image.Rectangle
	// baseline is the quantity of vertical pixels between the baseline and
	// the bottom of bounds.
	baseline int
}

// locate returns highlight regions covering the glyphs that represent the runes in
// [startRune,endRune). If the rects parameter is non-nil, locate will use it to
// return results instead of allocating, provided that there is enough capacity.
func (g *glyphIndex) locate(viewport image.Rectangle, startRune, endRune int, rects []region) []region {
	if startRune > endRune {
		startRune, endRune = endRune, startRune
	}
	rects = rects[:0]
	caretStart, _ := g.closestToRune(startRune)
	caretEnd, _ := g.closestToRune(endRune)

	for lineIdx := caretStart.lineCol.line; lineIdx < len(g.lines); lineIdx++ {
		if lineIdx > caretEnd.lineCol.line {
			break
		}
		pos := g.closestToLineCol(screenPos{line: lineIdx})
		if int(pos.y)+pos.descent.Ceil() < viewport.Min.Y {
			continue
		}
		if int(pos.y)-pos.ascent.Ceil() > viewport.Max.Y {
			break
		}
		line := g.lines[lineIdx]
		if lineIdx > caretStart.lineCol.line && lineIdx < caretEnd.lineCol.line {
			startX := line.xOff
			endX := startX + line.width
			// The entire line is selected.
			rects = append(rects, makeRegion(line, pos.y, startX, endX))
			continue
		}
		selectionStart := caretStart
		selectionEnd := caretEnd
		if lineIdx != caretStart.lineCol.line {
			// This line does not contain the beginning of the selection.
			selectionStart = g.closestToLineCol(screenPos{line: lineIdx})
		}
		if lineIdx != caretEnd.lineCol.line {
			// This line does not contain the end of the selection.
			selectionEnd = g.closestToLineCol(screenPos{line: lineIdx, col: math.MaxInt})
		}

		var (
			startX, endX fixed.Int26_6
			eof          bool
		)
	lineLoop:
		for !eof {
			startX = selectionStart.x
			if selectionStart.runIndex == selectionEnd.runIndex {
				// Commit selection.
				endX = selectionEnd.x
				rects = append(rects, makeRegion(line, pos.y, startX, endX))
				break
			} else {
				currentDirection := selectionStart.towardOrigin
				previous := selectionStart
			runLoop:
				for !eof {
					// Increment the start position until the next logical run.
					for startRun := selectionStart.runIndex; selectionStart.runIndex == startRun; {
						previous = selectionStart
						selectionStart, eof = g.incrementPosition(selectionStart)
						if eof {
							endX = selectionStart.x
							rects = append(rects, makeRegion(line, pos.y, startX, endX))
							break runLoop
						}
					}
					if selectionStart.towardOrigin != currentDirection {
						endX = previous.x
						rects = append(rects, makeRegion(line, pos.y, startX, endX))
						break
					}
					if selectionStart.runIndex == selectionEnd.runIndex {
						// Commit selection.
						endX = selectionEnd.x
						rects = append(rects, makeRegion(line, pos.y, startX, endX))
						break lineLoop
					}
				}
			}
		}
	}
	return rects
}
