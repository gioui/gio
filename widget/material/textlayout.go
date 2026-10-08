// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
)

// TextRun is an intermediate representation of a text fragment with its style,
// used for multi-pass line layout with mixed font sizes.
type TextRun struct {
	Text      string
	Size      float32 // unscaled font size
	Col       color.NRGBA
	Bold      bool
	Italic    bool
	Underline bool
	FontRef   string
	BGC       string
	LH        float32 // line height multiplier (0 = default 1.25)
	Align     string  // paragraph alignment: "" = default, "center", "right", "justify"
}

// MeasuredRun holds a pre-measured text run within a line.
type MeasuredRun struct {
	Text       string
	Width      int
	ScaledSize int
	OrigRunIdx int
	// Style data needed by the render phase.
	Col       color.NRGBA
	Bold      bool
	Italic    bool
	Underline bool
	FontRef   string
	BGC       string
	Align     string
}

// MeasuredLine holds a pre-computed line layout for one line of text.
type MeasuredLine struct {
	Runs   []MeasuredRun
	Width  int
	Height int
	Y      int
}

// MeasuredText holds the complete pre-computed text layout for a text zone.
type MeasuredText struct {
	Lines       []MeasuredLine
	Runs        []TextRun    // resolved runs (Phase 1 output)
	SegRunStart []int        // segment-to-run index mapping
	TextScale   float32
}

// TextSegment is a styled text segment with optional overrides.
// This is the input to BuildRuns — a simplified version of the editor's
// RichTextSegment type.
type TextSegment struct {
	T   string  // text content
	B   bool    // bold
	I   bool    // italic
	U   bool    // underline
	S   float32 // font size override (0 = inherit)
	F   string  // font ref override ("" = inherit)
	C   string  // color override ("" = inherit)
	BGC string  // background color
	LH  float32 // line height multiplier
	A   string  // alignment override
}

// BuildRuns resolves style inheritance and builds TextRuns from segments.
// Extracted from the editor's drawRichTextSegments Phase 1.
func BuildRuns(segs []TextSegment, defaultCol color.NRGBA, defaultFont string) ([]TextRun, []int) {
	var runs []TextRun
	var segRunStart []int
	inheritSize := float32(14)
	inheritBold := false
	inheritItalic := false
	inheritFont := defaultFont
	inheritLH := float32(0)
	inheritAlign := ""

	for _, seg := range segs {
		if seg.T == "" {
			segRunStart = append(segRunStart, -1)
			continue
		}

		col := defaultCol
		if seg.C != "" {
			col = HexToColor(seg.C)
		}
		size := inheritSize
		if seg.S > 0 {
			size = seg.S
			inheritSize = size
		}
		bold := inheritBold
		if seg.B {
			bold = true
		} else if seg.S > 0 || seg.C != "" || seg.F != "" {
			bold = false
		}
		inheritBold = bold
		italic := inheritItalic
		if seg.I {
			italic = true
		} else if seg.S > 0 || seg.C != "" || seg.F != "" {
			italic = false
		}
		inheritItalic = italic
		fontRef := inheritFont
		if seg.F != "" {
			fontRef = seg.F
			inheritFont = fontRef
		}
		lh := inheritLH
		if seg.LH > 0 {
			lh = seg.LH
			inheritLH = lh
		}
		if seg.A != "" {
			inheritAlign = seg.A
		}

		startIdx := len(runs)
		segRunStart = append(segRunStart, startIdx)

		parts := strings.Split(seg.T, "\n")
		for pi, part := range parts {
			if pi > 0 {
				runs = append(runs, TextRun{Text: "\n", Size: size, Col: col, LH: lh})
				inheritAlign = ""
			}
			if part != "" {
				runs = append(runs, TextRun{
					Text: part, Size: size, Col: col,
					Bold: bold, Italic: italic, Underline: seg.U,
					FontRef: fontRef, BGC: seg.BGC, LH: lh, Align: inheritAlign,
				})
			}
		}
	}
	return runs, segRunStart
}

// HexToColor parses a "#rrggbb" hex color string.
func HexToColor(s string) color.NRGBA {
	if len(s) < 7 || s[0] != '#' {
		return color.NRGBA{R: 0, G: 0, B: 0, A: 255}
	}
	var r, g, b uint8
	n, _ := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	if n != 3 {
		return color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}

// MeasureTextLines packs text runs into lines, measuring word widths.
// This is the expensive operation — it calls MeasureWidth for each word.
func MeasureTextLines(gtx layout.Context, runs []TextRun, maxW, startY int, textScale float32) []MeasuredLine {
	if len(runs) == 0 {
		return nil
	}

	type tempRun struct {
		text  string
		width int
		l     LabelStyle
		idx   int
	}
	type tempLine struct {
		runs   []tempRun
		width  int
		height int
		y      int
	}

	var lines []MeasuredLine
	var curLine tempLine
	curY := startY

	computeLineHeight := func(sz float32, lhMul float32) int {
		scaled := int(float32(sz) * textScale)
		if scaled < 4 {
			scaled = 4
		}
		mul := float32(1.25)
		if lhMul > 0 {
			mul = lhMul
		}
		h := int(float32(scaled) * mul)
		if h < 4 {
			h = 4
		}
		return h
	}

	flushLine := func() {
		if len(curLine.runs) > 0 || curLine.height > 0 {
			curLine.y = curY
			curY += curLine.height

			mLine := MeasuredLine{
				Width:  curLine.width,
				Height: curLine.height,
				Y:      curLine.y,
			}
			for _, tr := range curLine.runs {
				origRun := runs[tr.idx]
				mLine.Runs = append(mLine.Runs, MeasuredRun{
					Text:       tr.text,
					Width:      tr.width,
					ScaledSize: int(float32(origRun.Size) * textScale),
					OrigRunIdx: tr.idx,
					Col:        origRun.Col,
					Bold:       origRun.Bold,
					Italic:     origRun.Italic,
					Underline:  origRun.Underline,
					FontRef:    origRun.FontRef,
					BGC:        origRun.BGC,
					Align:      origRun.Align,
				})
			}
			lines = append(lines, mLine)
			curLine = tempLine{}
		}
	}

	for ri, run := range runs {
		if run.Text == "\n" {
			flushLine()
			continue
		}

		scaledSize := int(float32(run.Size) * textScale)
		if scaledSize < 4 {
			scaledSize = 4
		}
		lh := computeLineHeight(run.Size, run.LH)
		l := MakeStyledLabel("", scaledSize, run.Col, run.Bold, run.Italic, run.FontRef)

		parts := strings.Split(run.Text, " ")
		for i, p := range parts {
			word := p
			if i < len(parts)-1 {
				word += " "
			}
			if word == "" {
				continue
			}

			l.Text = word
			wordW := l.MeasureWidth(gtx, word)

			if curLine.width+wordW > maxW && curLine.width > 0 {
				flushLine()
			}

			// If a single word is wider than maxW, sub-wrap it.
			if wordW > maxW {
				subWords := wrapWord(gtx, l, word, maxW)
				for swi, sw := range subWords {
					l.Text = sw
					swW := l.MeasureWidth(gtx, sw)
					if curLine.width+swW > maxW && curLine.width > 0 {
						flushLine()
					}
					curLine.runs = append(curLine.runs, tempRun{
						text: sw, width: swW, l: l, idx: ri,
					})
					curLine.width += swW
					if lh > curLine.height {
						curLine.height = lh
					}
					if swi < len(subWords)-1 {
						flushLine()
					}
				}
				continue
			}

			curLine.runs = append(curLine.runs, tempRun{
				text: word, width: wordW, l: l, idx: ri,
			})
			curLine.width += wordW
			if lh > curLine.height {
				curLine.height = lh
			}
		}
	}
	flushLine()
	return lines
}

// wrapWord wraps a single word to fit within maxW pixels.
func wrapWord(gtx layout.Context, l LabelStyle, word string, maxW int) []string {
	if maxW <= 0 || word == "" {
		return []string{word}
	}
	l.Text = word
	if l.MeasureWidth(gtx, word) <= maxW {
		return []string{word}
	}
	var result []string
	remaining := word
	for remaining != "" {
		l.Text = remaining
		if l.MeasureWidth(gtx, remaining) <= maxW {
			result = append(result, remaining)
			break
		}
		// Binary search for the longest fitting prefix
		lo, hi := 1, len(remaining)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			l.Text = remaining[:mid]
			if l.MeasureWidth(gtx, remaining[:mid]) <= maxW {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		if lo < 1 {
			lo = 1
		}
		result = append(result, remaining[:lo])
		remaining = remaining[lo:]
	}
	return result
}

// MakeStyledLabel creates a LabelStyle with the given properties.
// The caller must set the Shaper field from the theme before calling
// Layout or MeasureWidth.
func MakeStyledLabel(txt string, size int, col color.NRGBA, bold, italic bool, fontRef string) LabelStyle {
	l := LabelStyle{
		Text:     txt,
		Color:    col,
		TextSize: unit.Sp(size),
	}
	l.Alignment = text.Start
	if bold {
		l.Font.Weight = font.Bold
	}
	if italic {
		l.Font.Style = font.Italic
	}
	if fontRef != "" {
		l.Font.Typeface = font.Typeface(fontRef)
	}
	return l
}

// ParallelMeasureText measures multiple text layouts in parallel goroutines.
// The measureFn is called for each index; it receives a gtx with a temporary
// op.Ops and should return a MeasuredText (or nil to skip).
// Returns a slice of results indexed by the input indices.
func ParallelMeasureText(gtx layout.Context, indices []int, measureFn func(gtx layout.Context, idx int) *MeasuredText) []*MeasuredText {
	results := make([]*MeasuredText, len(indices))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // limit concurrency

	for i, idx := range indices {
		wg.Add(1)
		go func(i, idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Each goroutine gets its own temporary op.Ops.
			var tempOps op.Ops
			sub := gtx
			sub.Ops = &tempOps
			results[i] = measureFn(sub, idx)
		}(i, idx)
	}
	wg.Wait()
	return results
}

// TextLayoutCache stores pre-computed MeasuredText results across frames.
// Invalidate when text content changes.
type TextLayoutCache struct {
	cache map[int]*MeasuredText
}

// NewTextLayoutCache creates a new empty cache.
func NewTextLayoutCache() *TextLayoutCache {
	return &TextLayoutCache{cache: make(map[int]*MeasuredText)}
}

// Get returns the cached layout for the given index, or nil if not cached.
func (c *TextLayoutCache) Get(idx int) *MeasuredText {
	return c.cache[idx]
}

// Set stores a layout result for the given index.
func (c *TextLayoutCache) Set(idx int, mt *MeasuredText) {
	c.cache[idx] = mt
}

// Clear removes all cached entries.
func (c *TextLayoutCache) Clear() {
	for k := range c.cache {
		delete(c.cache, k)
	}
}

// Len returns the number of cached entries.
func (c *TextLayoutCache) Len() int {
	return len(c.cache)
}
