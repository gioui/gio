package widget

import (
	"bytes"
	"io"
	"testing"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

// makePosTestText returns two bidi samples of shaped text at the given
// font size and wrapped to the given line width. The runeLimit, if nonzero,
// truncates the sample text to ensure shorter output for expensive tests.
func makePosTestText(fontSize, lineWidth int, alignOpposite bool) (source string, bidiLTR, bidiRTL []text.Glyph) {
	ltrFace, _ := opentype.Parse(goregular.TTF)
	rtlFace, _ := opentype.Parse(nsareg.TTF)

	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection([]font.FontFace{
		{
			Font: font.Font{Typeface: "LTR"},
			Face: ltrFace,
		},
		{
			Font: font.Font{Typeface: "RTL"},
			Face: rtlFace,
		},
	}))
	// bidiSource is crafted to contain multiple consecutive RTL runs (by
	// changing scripts within the RTL).
	bidiSource := "The quick سماء שלום لا fox تمط שלום غير the lazy dog."
	ltrParams := text.Parameters{
		PxPerEm:  fixed.I(fontSize),
		MaxWidth: lineWidth,
		MinWidth: lineWidth,
		Locale:   english,
	}
	rtlParams := text.Parameters{
		Alignment: text.End,
		PxPerEm:   fixed.I(fontSize),
		MaxWidth:  lineWidth,
		MinWidth:  lineWidth,
		Locale:    arabic,
	}
	if alignOpposite {
		ltrParams.Alignment = text.End
		rtlParams.Alignment = text.Start
	}
	shaper.LayoutString(ltrParams, bidiSource)
	for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
		bidiLTR = append(bidiLTR, g)
	}
	shaper.LayoutString(rtlParams, bidiSource)
	for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
		bidiRTL = append(bidiRTL, g)
	}
	return bidiSource, bidiLTR, bidiRTL
}

// makeAccountingTestText shapes text designed to stress rune accounting
// logic within the index.
func makeAccountingTestText(str string, fontSize, lineWidth int) (txt []text.Glyph) {
	ltrFace, _ := opentype.Parse(goregular.TTF)
	rtlFace, _ := opentype.Parse(nsareg.TTF)

	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection([]font.FontFace{{
		Font: font.Font{Typeface: "LTR"},
		Face: ltrFace,
	},
		{
			Font: font.Font{Typeface: "RTL"},
			Face: rtlFace,
		},
	}))
	params := text.Parameters{
		PxPerEm:  fixed.I(fontSize),
		MaxWidth: lineWidth,
		Locale:   english,
	}
	shaper.LayoutString(params, str)
	for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
		txt = append(txt, g)
	}
	return txt
}

// getGlyphs shapes text as english.
func getGlyphs(fontSize, minWidth, lineWidth int, align text.Alignment, str string) (txt []text.Glyph) {
	ltrFace, _ := opentype.Parse(goregular.TTF)
	rtlFace, _ := opentype.Parse(nsareg.TTF)

	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection([]font.FontFace{{
		Font: font.Font{Typeface: "LTR"},
		Face: ltrFace,
	},
		{
			Font: font.Font{Typeface: "RTL"},
			Face: rtlFace,
		},
	}))
	params := text.Parameters{
		PxPerEm:    fixed.I(fontSize),
		Alignment:  align,
		MinWidth:   minWidth,
		MaxWidth:   lineWidth,
		Locale:     english,
		WrapPolicy: text.WrapWords,
	}
	shaper.LayoutString(params, str)
	for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
		txt = append(txt, g)
	}
	return txt
}

// TestIndexPositionWhitespace checks that the index correctly generates cursor positions
// for empty lines and the empty string.
func TestIndexPositionWhitespace(t *testing.T) {
	type testcase struct {
		name      string
		str       string
		lineWidth int
		align     text.Alignment
		expected  []combinedPos
	}
	for _, tc := range []testcase{
		{
			name:      "empty string",
			str:       "",
			lineWidth: 200,
			expected: []combinedPos{
				{x: fixed.Int26_6(0), y: 16, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216)},
			},
		},
		{
			name:      "just hard newline",
			str:       "\n",
			lineWidth: 200,
			expected: []combinedPos{
				{x: fixed.Int26_6(0), y: 16, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216)},
				{x: fixed.Int26_6(0), y: 35, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 1, lineCol: screenPos{line: 1}},
			},
		},
		{
			name:      "trailing newline",
			str:       "a\n",
			lineWidth: 200,
			expected: []combinedPos{
				{x: fixed.Int26_6(0), y: 16, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216)},
				{x: fixed.Int26_6(570), y: 16, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 1, lineCol: screenPos{col: 1}},
				{x: fixed.Int26_6(0), y: 35, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 2, lineCol: screenPos{line: 1}},
			},
		},
		{
			name:      "just blank line",
			str:       "\n\n",
			lineWidth: 200,
			expected: []combinedPos{
				{x: fixed.Int26_6(0), y: 16, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216)},
				{x: fixed.Int26_6(0), y: 35, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 1, lineCol: screenPos{line: 1}},
				{x: fixed.Int26_6(0), y: 54, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 2, lineCol: screenPos{line: 2}},
			},
		},
		{
			name:      "middle aligned blank lines",
			str:       "\n\n\nabc",
			align:     text.Middle,
			lineWidth: 200,
			expected: []combinedPos{
				{x: fixed.Int26_6(832), y: 16, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216)},
				{x: fixed.Int26_6(832), y: 35, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 1, lineCol: screenPos{line: 1}},
				{x: fixed.Int26_6(832), y: 54, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 2, lineCol: screenPos{line: 2}},
				{x: fixed.Int26_6(6), y: 73, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 3, lineCol: screenPos{line: 3}},
				{x: fixed.Int26_6(576), y: 73, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 4, lineCol: screenPos{line: 3, col: 1}},
				{x: fixed.Int26_6(1146), y: 73, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 5, lineCol: screenPos{line: 3, col: 2}},
				{x: fixed.Int26_6(1658), y: 73, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 6, lineCol: screenPos{line: 3, col: 3}},
			},
		},
		{
			name:      "blank line",
			str:       "a\n\nb",
			lineWidth: 200,
			expected: []combinedPos{
				{x: fixed.Int26_6(0), y: 16, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216)},
				{x: fixed.Int26_6(570), y: 16, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 1, lineCol: screenPos{col: 1}},
				{x: fixed.Int26_6(0), y: 35, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 2, lineCol: screenPos{line: 1}},
				{x: fixed.Int26_6(0), y: 54, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 3, lineCol: screenPos{line: 2}},
				{x: fixed.Int26_6(570), y: 54, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), runes: 4, lineCol: screenPos{line: 2, col: 1}},
			},
		},
		{
			name:      "soft wrap",
			str:       "abc def",
			lineWidth: 30,
			expected: []combinedPos{
				{runes: 0, lineCol: screenPos{line: 0, col: 0}, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), x: 0, y: 16},
				{runes: 1, lineCol: screenPos{line: 0, col: 1}, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), x: 570, y: 16},
				{runes: 2, lineCol: screenPos{line: 0, col: 2}, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), x: 1140, y: 16},
				{runes: 3, lineCol: screenPos{line: 0, col: 3}, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), x: 1652, y: 16},
				{runes: 4, lineCol: screenPos{line: 1, col: 0}, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), x: 0, y: 35},
				{runes: 5, lineCol: screenPos{line: 1, col: 1}, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), x: 570, y: 35},
				{runes: 6, lineCol: screenPos{line: 1, col: 2}, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), x: 1140, y: 35},
				{runes: 7, lineCol: screenPos{line: 1, col: 3}, ascent: fixed.Int26_6(968), descent: fixed.Int26_6(216), x: 1425, y: 35},
			},
		},
		{
			name:      "soft wrap arabic",
			str:       "ثنائي الاتجاه",
			lineWidth: 30,
			expected: []combinedPos{
				{runes: 0, lineCol: screenPos{line: 0, col: 0}, ascent: 1407, descent: 756, x: 2250, y: 22, towardOrigin: true},
				{runes: 1, lineCol: screenPos{line: 0, col: 1}, ascent: 1407, descent: 756, x: 1944, y: 22, towardOrigin: true},
				{runes: 2, lineCol: screenPos{line: 0, col: 2}, ascent: 1407, descent: 756, x: 1593, y: 22, towardOrigin: true},
				{runes: 3, lineCol: screenPos{line: 0, col: 3}, ascent: 1407, descent: 756, x: 1295, y: 22, towardOrigin: true},
				{runes: 4, lineCol: screenPos{line: 0, col: 4}, ascent: 1407, descent: 756, x: 1020, y: 22, towardOrigin: true},
				{runes: 5, lineCol: screenPos{line: 0, col: 5}, ascent: 1407, descent: 756, x: 266, y: 22, towardOrigin: true},
				{runes: 6, lineCol: screenPos{line: 1, col: 0}, ascent: 1407, descent: 756, x: 2511, y: 41, towardOrigin: true},
				{runes: 7, lineCol: screenPos{line: 1, col: 1}, ascent: 1407, descent: 756, x: 2267, y: 41, towardOrigin: true},
				{runes: 8, lineCol: screenPos{line: 1, col: 2}, ascent: 1407, descent: 756, x: 1969, y: 41, towardOrigin: true},
				{runes: 9, lineCol: screenPos{line: 1, col: 3}, ascent: 1407, descent: 756, x: 1671, y: 41, towardOrigin: true},
				{runes: 10, lineCol: screenPos{line: 1, col: 4}, ascent: 1407, descent: 756, x: 1365, y: 41, towardOrigin: true},
				{runes: 11, lineCol: screenPos{line: 1, col: 5}, ascent: 1407, descent: 756, x: 713, y: 41, towardOrigin: true},
				{runes: 12, lineCol: screenPos{line: 1, col: 6}, ascent: 1407, descent: 756, x: 415, y: 41, towardOrigin: true},
				{runes: 13, lineCol: screenPos{line: 1, col: 7}, ascent: 1407, descent: 756, x: 0, y: 41, towardOrigin: true},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			glyphs := getGlyphs(16, 0, tc.lineWidth, tc.align, tc.str)
			var gi glyphIndex
			gi.reset()
			for _, g := range glyphs {
				gi.Glyph(g)
			}
			if len(gi.positions) != len(tc.expected) {
				t.Errorf("expected %d positions, got %d", len(tc.expected), len(gi.positions))
			}
			for i := 0; i < min(len(gi.positions), len(tc.expected)); i++ {
				actual := gi.positions[i]
				expected := tc.expected[i]
				if actual != expected {
					t.Errorf("position %d: expected:\n%#+v, got:\n%#+v", i, expected, actual)
				}
			}
			if t.Failed() {
				printPositions(t, gi.positions)
				printGlyphs(t, glyphs)
			}
		})
	}

}

// TestIndexPositionBidi tests whether the index correct generates cursor positions for
// complex bidirectional text.
func TestIndexPositionBidi(t *testing.T) {
	fontSize := 16
	lineWidth := fontSize * 10
	_, bidiLTRText, bidiRTLText := makePosTestText(fontSize, lineWidth, false)
	type testcase struct {
		name       string
		glyphs     []text.Glyph
		expectedXs []fixed.Int26_6
	}
	for _, tc := range []testcase{
		{
			name:   "bidi ltr",
			glyphs: bidiLTRText,
			expectedXs: []fixed.Int26_6{
				0, 626, 1196, 1766, 2051, 2621, 3191, 3444, 3956, 4468, 4753, 7133, 6330, 5738, 5440, 5019, // Positions on line 0.

				3953, 3185, 2417, 1649, 881, 596, 298, 0, 3953, 4238, 4523, 5093, 5605, 5890, 7905, 7599, 7007, 6156, // Positions on line 1.

				4660, 3892, 3124, 2356, 1588, 1303, 788, 406, 0, 4660, 4945, 5235, 5805, 6375, 6660, 6934, 7504, 8016, 8528, // Positions on line 2.

				0, 570, 1140, 1710, 2034, // Positions on line 3.
			},
		},
		{
			name:   "bidi rtl",
			glyphs: bidiRTLText,
			expectedXs: []fixed.Int26_6{
				2646, 3272, 3842, 4412, 4697, 5267, 5837, 6090, 6602, 7114, 2646, 2380, 1577, 985, 687, 266, // Positions on line 0.

				7867, 7099, 6331, 5563, 4795, 4510, 4212, 3914, 3648, 2281, 2566, 3136, 3648, 2281, 2015, 1709, 1117, 266, // Positions on line 1.

				8794, 8026, 7258, 6490, 5722, 5437, 4922, 4540, 4134, 3868, 0, 290, 860, 1430, 1715, 1989, 2559, 3071, 3583, // Positions on line 2.

				324, 894, 1464, 2034, 324, 0, // Positions on line 3.
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gi glyphIndex
			gi.reset()
			for _, g := range tc.glyphs {
				gi.Glyph(g)
			}
			if len(gi.positions) != len(tc.expectedXs) {
				t.Errorf("expected %d positions, got %d", len(tc.expectedXs), len(gi.positions))
			}
			lastRunes := 0
			lastLine := 0
			lastCol := -1
			lastY := 0
			for i := 0; i < min(len(gi.positions), len(tc.expectedXs)); i++ {
				actualX := gi.positions[i].x
				expectedX := tc.expectedXs[i]
				if actualX != expectedX {
					t.Errorf("position %d: expected x=%v(%d), got x=%v(%d)", i, expectedX, expectedX, actualX, actualX)
				}
				if r := gi.positions[i].runes; r < lastRunes {
					t.Errorf("position %d: expected runes >= %d, got %d", i, lastRunes, r)
				}
				lastRunes = gi.positions[i].runes
				if y := gi.positions[i].y; y < lastY {
					t.Errorf("position %d: expected y>= %d, got %d", i, lastY, y)
				}
				lastY = gi.positions[i].y
				if y := gi.positions[i].y; y < lastY {
					t.Errorf("position %d: expected y>= %d, got %d", i, lastY, y)
				}
				lastY = gi.positions[i].y
				if lineCol := gi.positions[i].lineCol; lineCol.line == lastLine && lineCol.col < lastCol {
					t.Errorf("position %d: expected col >= %d, got %d", i, lastCol, lineCol.col)
				}
				lastCol = gi.positions[i].lineCol.col
				if line := gi.positions[i].lineCol.line; line < lastLine {
					t.Errorf("position %d: expected line >= %d, got %d", i, lastLine, line)
				}
				lastLine = gi.positions[i].lineCol.line
			}
			printPositions(t, gi.positions)
			if t.Failed() {
				printGlyphs(t, tc.glyphs)
			}
		})
	}
}

func TestIndexPositionLines(t *testing.T) {
	fontSize := 16
	lineWidth := fontSize * 10
	source1, bidiLTRText, bidiRTLText := makePosTestText(fontSize, lineWidth, false)
	source2, bidiLTRTextOpp, bidiRTLTextOpp := makePosTestText(fontSize, lineWidth, true)
	type testcase struct {
		name          string
		source        string
		glyphs        []text.Glyph
		expectedLines []lineInfo
	}
	for _, tc := range []testcase{
		{
			name:   "bidi ltr",
			source: source1,
			glyphs: bidiLTRText,
			expectedLines: []lineInfo{
				{
					xOff:    fixed.Int26_6(0),
					yOff:    22,
					glyphs:  15,
					width:   fixed.Int26_6(7133),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(0),
					yOff:    41,
					glyphs:  15,
					width:   fixed.Int26_6(7905),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(0),
					yOff:    60,
					glyphs:  18,
					width:   fixed.Int26_6(8813),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(0),
					yOff:    79,
					glyphs:  4,
					width:   fixed.Int26_6(2034),
					ascent:  fixed.Int26_6(968),
					descent: fixed.Int26_6(216),
				},
			},
		},
		{
			name:   "bidi rtl",
			source: source1,
			glyphs: bidiRTLText,
			expectedLines: []lineInfo{
				{
					xOff:    fixed.Int26_6(0),
					yOff:    22,
					glyphs:  15,
					width:   fixed.Int26_6(7114),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(0),
					yOff:    41,
					glyphs:  15,
					width:   fixed.Int26_6(7867),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(0),
					yOff:    60,
					glyphs:  18,
					width:   fixed.Int26_6(8794),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(0),
					yOff:    79,
					glyphs:  4,
					width:   fixed.Int26_6(2034),
					ascent:  fixed.Int26_6(968),
					descent: fixed.Int26_6(216),
				},
			},
		},
		{
			name:   "bidi ltr opposite alignment",
			source: source2,
			glyphs: bidiLTRTextOpp,
			expectedLines: []lineInfo{
				{
					xOff:    fixed.Int26_6(3107),
					yOff:    22,
					glyphs:  15,
					width:   fixed.Int26_6(7133),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(2335),
					yOff:    41,
					glyphs:  15,
					width:   fixed.Int26_6(7905),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(1427),
					yOff:    60,
					glyphs:  18,
					width:   fixed.Int26_6(8813),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(8206),
					yOff:    79,
					glyphs:  4,
					width:   fixed.Int26_6(2034),
					ascent:  fixed.Int26_6(968),
					descent: fixed.Int26_6(216),
				},
			},
		},
		{
			name:   "bidi rtl opposite alignment",
			source: source2,
			glyphs: bidiRTLTextOpp,
			expectedLines: []lineInfo{
				{
					xOff:    fixed.Int26_6(3126),
					yOff:    22,
					glyphs:  15,
					width:   fixed.Int26_6(7114),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(2373),
					yOff:    41,
					glyphs:  15,
					width:   fixed.Int26_6(7867),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(1446),
					yOff:    60,
					glyphs:  18,
					width:   fixed.Int26_6(8794),
					ascent:  fixed.Int26_6(1407),
					descent: fixed.Int26_6(756),
				},
				{
					xOff:    fixed.Int26_6(8206),
					yOff:    79,
					glyphs:  4,
					width:   fixed.Int26_6(2034),
					ascent:  fixed.Int26_6(968),
					descent: fixed.Int26_6(216),
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gi glyphIndex
			gi.reset()
			for _, g := range tc.glyphs {
				gi.Glyph(g)
			}
			if len(gi.lines) != len(tc.expectedLines) {
				t.Errorf("expected %d lines, got %d", len(tc.expectedLines), len(gi.lines))
			}
			for i := 0; i < min(len(gi.lines), len(tc.expectedLines)); i++ {
				actual := gi.lines[i]
				expected := tc.expectedLines[i]
				if actual != expected {
					t.Errorf("line %d: expected:\n%#+v, got:\n%#+v", i, expected, actual)
				}
			}
		})
	}
}

// TestIndexPositionRunes checks for rune accounting errors in positions
// generated by the index.
func TestIndexPositionRunes(t *testing.T) {
	fontSize := 16
	lineWidth := fontSize * 10
	// source is crafted to contain multiple consecutive RTL runs (by
	// changing scripts within the RTL).
	source := "The\nquick سماء של\nום لا fox\nتمط של\nום."
	testText := makeAccountingTestText(source, fontSize, lineWidth)
	type testcase struct {
		name     string
		source   string
		glyphs   []text.Glyph
		expected []combinedPos
	}
	for _, tc := range []testcase{
		{
			name:   "many newlines",
			source: source,
			glyphs: testText,
			expected: []combinedPos{
				{runes: 0, lineCol: screenPos{line: 0, col: 0}, runIndex: 0, towardOrigin: false},
				{runes: 1, lineCol: screenPos{line: 0, col: 1}, runIndex: 0, towardOrigin: false},
				{runes: 2, lineCol: screenPos{line: 0, col: 2}, runIndex: 0, towardOrigin: false},
				{runes: 3, lineCol: screenPos{line: 0, col: 3}, runIndex: 0, towardOrigin: false},
				{runes: 4, lineCol: screenPos{line: 1, col: 0}, runIndex: 0, towardOrigin: false},
				{runes: 5, lineCol: screenPos{line: 1, col: 1}, runIndex: 0, towardOrigin: false},
				{runes: 6, lineCol: screenPos{line: 1, col: 2}, runIndex: 0, towardOrigin: false},
				{runes: 7, lineCol: screenPos{line: 1, col: 3}, runIndex: 0, towardOrigin: false},
				{runes: 8, lineCol: screenPos{line: 1, col: 4}, runIndex: 0, towardOrigin: false},
				{runes: 9, lineCol: screenPos{line: 1, col: 5}, runIndex: 0, towardOrigin: false},
				{runes: 10, lineCol: screenPos{line: 1, col: 6}, runIndex: 0, towardOrigin: false},
				{runes: 10, lineCol: screenPos{line: 1, col: 6}, runIndex: 1, towardOrigin: true},
				{runes: 11, lineCol: screenPos{line: 1, col: 7}, runIndex: 1, towardOrigin: true},
				{runes: 12, lineCol: screenPos{line: 1, col: 8}, runIndex: 1, towardOrigin: true},
				{runes: 13, lineCol: screenPos{line: 1, col: 9}, runIndex: 1, towardOrigin: true},
				{runes: 14, lineCol: screenPos{line: 1, col: 10}, runIndex: 1, towardOrigin: true},
				{runes: 15, lineCol: screenPos{line: 1, col: 11}, runIndex: 2, towardOrigin: true},
				{runes: 16, lineCol: screenPos{line: 1, col: 12}, runIndex: 2, towardOrigin: true},
				{runes: 17, lineCol: screenPos{line: 1, col: 13}, runIndex: 2, towardOrigin: true},
				{runes: 18, lineCol: screenPos{line: 2, col: 0}, runIndex: 0, towardOrigin: true},
				{runes: 19, lineCol: screenPos{line: 2, col: 1}, runIndex: 0, towardOrigin: true},
				{runes: 20, lineCol: screenPos{line: 2, col: 2}, runIndex: 0, towardOrigin: true},
				{runes: 21, lineCol: screenPos{line: 2, col: 3}, runIndex: 1, towardOrigin: true},
				{runes: 22, lineCol: screenPos{line: 2, col: 4}, runIndex: 1, towardOrigin: true},
				{runes: 23, lineCol: screenPos{line: 2, col: 5}, runIndex: 1, towardOrigin: true},
				{runes: 24, lineCol: screenPos{line: 2, col: 6}, runIndex: 1, towardOrigin: true},
				{runes: 24, lineCol: screenPos{line: 2, col: 6}, runIndex: 2, towardOrigin: false},
				{runes: 25, lineCol: screenPos{line: 2, col: 7}, runIndex: 2, towardOrigin: false},
				{runes: 26, lineCol: screenPos{line: 2, col: 8}, runIndex: 2, towardOrigin: false},
				{runes: 27, lineCol: screenPos{line: 2, col: 9}, runIndex: 2, towardOrigin: false},
				{runes: 28, lineCol: screenPos{line: 3, col: 0}, runIndex: 0, towardOrigin: true},
				{runes: 29, lineCol: screenPos{line: 3, col: 1}, runIndex: 0, towardOrigin: true},
				{runes: 30, lineCol: screenPos{line: 3, col: 2}, runIndex: 0, towardOrigin: true},
				{runes: 31, lineCol: screenPos{line: 3, col: 3}, runIndex: 0, towardOrigin: true},
				{runes: 32, lineCol: screenPos{line: 3, col: 4}, runIndex: 1, towardOrigin: true},
				{runes: 33, lineCol: screenPos{line: 3, col: 5}, runIndex: 1, towardOrigin: true},
				{runes: 34, lineCol: screenPos{line: 3, col: 6}, runIndex: 1, towardOrigin: true},
				{runes: 35, lineCol: screenPos{line: 4, col: 0}, runIndex: 0, towardOrigin: true},
				{runes: 36, lineCol: screenPos{line: 4, col: 1}, runIndex: 0, towardOrigin: true},
				{runes: 37, lineCol: screenPos{line: 4, col: 2}, runIndex: 0, towardOrigin: true},
				{runes: 38, lineCol: screenPos{line: 4, col: 3}, runIndex: 0, towardOrigin: true},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var gi glyphIndex
			gi.reset()
			for _, g := range tc.glyphs {
				gi.Glyph(g)
			}
			if len(gi.positions) != len(tc.expected) {
				t.Errorf("expected %d positions, got %d", len(tc.expected), len(gi.positions))
			}
			for i := 0; i < min(len(gi.positions), len(tc.expected)); i++ {
				actual := gi.positions[i]
				expected := tc.expected[i]
				if expected.runes != actual.runes {
					t.Errorf("position %d: expected runes=%d, got %d", i, expected.runes, actual.runes)
				}
				if expected.lineCol != actual.lineCol {
					t.Errorf("position %d: expected lineCol=%v, got %v", i, expected.lineCol, actual.lineCol)
				}
				if expected.runIndex != actual.runIndex {
					t.Errorf("position %d: expected runIndex=%d, got %d", i, expected.runIndex, actual.runIndex)
				}
				if expected.towardOrigin != actual.towardOrigin {
					t.Errorf("position %d: expected towardOrigin=%v, got %v", i, expected.towardOrigin, actual.towardOrigin)
				}
			}
			printPositions(t, gi.positions)
			if t.Failed() {
				printGlyphs(t, tc.glyphs)
			}
		})
	}
}
func printPositions(t *testing.T, positions []combinedPos) {
	t.Helper()
	for i, p := range positions {
		t.Logf("positions[%2d] = {runes: %2d, line: %2d, col: %2d, x: %5d, y: %3d}", i, p.runes, p.lineCol.line, p.lineCol.col, p.x, p.y)
	}
}

func printGlyphs(t *testing.T, glyphs []text.Glyph) {
	t.Helper()
	for i, g := range glyphs {
		t.Logf("glyphs[%2d] = {ID: 0x%013x, Flags: %4s, Advance: %4d(%6v), Runes: %d, Y: %3d, X: %4d(%6v)} ", i, g.ID, g.Flags, g.Advance, g.Advance, g.Runes, g.Y, g.X, g.X)
	}
}

func TestGraphemeReaderNext(t *testing.T) {
	latinDoc := bytes.NewReader([]byte(latinDocument))
	arabicDoc := bytes.NewReader([]byte(arabicDocument))
	emojiDoc := bytes.NewReader([]byte(emojiDocument))
	complexDoc := bytes.NewReader([]byte(complexDocument))
	type testcase struct {
		name  string
		input *bytes.Reader
		read  func() ([]rune, bool)
	}
	var pr graphemeReader
	for _, tc := range []testcase{
		{
			name:  "latin",
			input: latinDoc,
			read:  pr.next,
		},
		{
			name:  "arabic",
			input: arabicDoc,
			read:  pr.next,
		},
		{
			name:  "emoji",
			input: emojiDoc,
			read:  pr.next,
		},
		{
			name:  "complex",
			input: complexDoc,
			read:  pr.next,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pr.SetSource(tc.input)

			runes := []rune{}
			var paragraph []rune
			ok := true
			for ok {
				paragraph, ok = tc.read()
				if ok && len(paragraph) > 0 && paragraph[len(paragraph)-1] != '\n' {
				}
				for i, r := range paragraph {
					if i == len(paragraph)-1 {
						if r != '\n' && ok {
							t.Error("non-final paragraph does not end with newline")
						}
					} else if r == '\n' {
						t.Errorf("paragraph[%d] contains newline", i)
					}
				}
				runes = append(runes, paragraph...)
			}
			tc.input.Seek(0, 0)
			b, _ := io.ReadAll(tc.input)
			asRunes := []rune(string(b))
			if len(asRunes) != len(runes) {
				t.Errorf("expected %d runes, got %d", len(asRunes), len(runes))
			}
			for i := 0; i < max(len(asRunes), len(runes)); i++ {
				if i < min(len(asRunes), len(runes)) {
					if runes[i] != asRunes[i] {
						t.Errorf("expected runes[%d]=%d, got %d", i, asRunes[i], runes[i])
					}
				} else if i < len(asRunes) {
					t.Errorf("expected runes[%d]=%d, got nothing", i, asRunes[i])
				} else if i < len(runes) {
					t.Errorf("expected runes[%d]=nothing, got %d", i, runes[i])
				}
			}
		})
	}
}
func TestGraphemeReaderGraphemes(t *testing.T) {
	latinDoc := bytes.NewReader([]byte(latinDocument))
	arabicDoc := bytes.NewReader([]byte(arabicDocument))
	emojiDoc := bytes.NewReader([]byte(emojiDocument))
	complexDoc := bytes.NewReader([]byte(complexDocument))
	type testcase struct {
		name  string
		input *bytes.Reader
		read  func() []int
	}
	var pr graphemeReader
	for _, tc := range []testcase{
		{
			name:  "latin",
			input: latinDoc,
			read:  pr.Graphemes,
		},
		{
			name:  "arabic",
			input: arabicDoc,
			read:  pr.Graphemes,
		},
		{
			name:  "emoji",
			input: emojiDoc,
			read:  pr.Graphemes,
		},
		{
			name:  "complex",
			input: complexDoc,
			read:  pr.Graphemes,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pr.SetSource(tc.input)

			graphemes := []int{}
			for g := tc.read(); len(g) > 0; g = tc.read() {
				if len(graphemes) > 0 && g[0] != graphemes[len(graphemes)-1] {
					t.Errorf("expected first boundary in new paragraph %d to match final boundary in previous %d", g[0], graphemes[len(graphemes)-1])
				}
				if len(graphemes) > 0 {
					// Drop duplicated boundary.
					g = g[1:]
				}
				graphemes = append(graphemes, g...)
			}
			tc.input.Seek(0, 0)
			b, _ := io.ReadAll(tc.input)
			asRunes := []rune(string(b))
			if len(asRunes)+1 < len(graphemes) {
				t.Errorf("expected <= %d graphemes, got %d", len(asRunes)+1, len(graphemes))
			}
			for i := 0; i < len(graphemes)-1; i++ {
				if graphemes[i] >= graphemes[i+1] {
					t.Errorf("graphemes[%d](%d) >= graphemes[%d](%d)", i, graphemes[i], i+1, graphemes[i+1])
				}
			}
		})
	}
}
func BenchmarkGraphemeReaderNext(b *testing.B) {
	latinDoc := bytes.NewReader([]byte(latinDocument))
	arabicDoc := bytes.NewReader([]byte(arabicDocument))
	emojiDoc := bytes.NewReader([]byte(emojiDocument))
	complexDoc := bytes.NewReader([]byte(complexDocument))
	type testcase struct {
		name  string
		input *bytes.Reader
		read  func() ([]rune, bool)
	}
	pr := &graphemeReader{}
	for _, tc := range []testcase{
		{
			name:  "latin",
			input: latinDoc,
			read:  pr.next,
		},
		{
			name:  "arabic",
			input: arabicDoc,
			read:  pr.next,
		},
		{
			name:  "emoji",
			input: emojiDoc,
			read:  pr.next,
		},
		{
			name:  "complex",
			input: complexDoc,
			read:  pr.next,
		},
	} {
		var paragraph []rune = make([]rune, 4096)
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pr.SetSource(tc.input)

				ok := true
				for ok {
					paragraph, ok = tc.read()
					_ = paragraph
				}
				_ = paragraph
			}
		})
	}
}
func BenchmarkGraphemeReaderGraphemes(b *testing.B) {
	latinDoc := bytes.NewReader([]byte(latinDocument))
	arabicDoc := bytes.NewReader([]byte(arabicDocument))
	emojiDoc := bytes.NewReader([]byte(emojiDocument))
	complexDoc := bytes.NewReader([]byte(complexDocument))
	type testcase struct {
		name  string
		input *bytes.Reader
		read  func() []int
	}
	pr := &graphemeReader{}
	for _, tc := range []testcase{
		{
			name:  "latin",
			input: latinDoc,
			read:  pr.Graphemes,
		},
		{
			name:  "arabic",
			input: arabicDoc,
			read:  pr.Graphemes,
		},
		{
			name:  "emoji",
			input: emojiDoc,
			read:  pr.Graphemes,
		},
		{
			name:  "complex",
			input: complexDoc,
			read:  pr.Graphemes,
		},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pr.SetSource(tc.input)
				for g := tc.read(); len(g) > 0; g = tc.read() {
					_ = g
				}
			}
		})
	}
}
