package text

import (
	"fmt"
	"strings"
	"testing"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/system"
	"golang.org/x/exp/slices"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

// TestWrappingTruncation checks that the line wrapper's truncation features
// behave as expected.
func TestWrappingTruncation(t *testing.T) {
	// Use a test string containing multiple newlines to ensure that they are shaped
	// as separate paragraphs.
	textInput := "Lorem ipsum dolor sit amet, consectetur adipiscing elit,\nsed do eiusmod tempor incididunt ut labore et\ndolore magna aliqua.\n"
	ltrFace, _ := opentype.Parse(goregular.TTF)
	collection := []FontFace{{Face: ltrFace}}
	cache := NewShaper(NoSystemFonts(), WithCollection(collection))
	cache.LayoutString(Parameters{
		Alignment: Middle,
		PxPerEm:   fixed.I(10),
		MinWidth:  200,
		MaxWidth:  200,
		Locale:    english,
	}, textInput)
	untruncatedCount := len(cache.txt.lines)

	for i := untruncatedCount + 1; i > 0; i-- {
		t.Run(fmt.Sprintf("truncated to %d/%d lines", i, untruncatedCount), func(t *testing.T) {
			cache.LayoutString(Parameters{
				Alignment: Middle,
				PxPerEm:   fixed.I(10),
				MaxLines:  i,
				MinWidth:  200,
				MaxWidth:  200,
				Locale:    english,
			}, textInput)
			lineCount := 0
			lastGlyphWasLineBreak := false
			glyphs := []Glyph{}
			untruncatedRunes := 0
			truncatedRunes := 0
			for g, ok := cache.NextGlyph(); ok; g, ok = cache.NextGlyph() {
				glyphs = append(glyphs, g)
				if g.Flags&FlagTruncator != 0 && g.Flags&FlagClusterBreak != 0 {
					truncatedRunes += int(g.Runes)
				} else {
					untruncatedRunes += int(g.Runes)
				}
				if g.Flags&FlagLineBreak != 0 {
					lineCount++
					lastGlyphWasLineBreak = true
				} else {
					lastGlyphWasLineBreak = false
				}
			}
			if lastGlyphWasLineBreak && truncatedRunes == 0 {
				// There was no actual line of text following this break.
				lineCount--
			}
			if i <= untruncatedCount {
				if lineCount != i {
					t.Errorf("expected %d lines, got %d", i, lineCount)
				}
			} else if i > untruncatedCount {
				if lineCount != untruncatedCount {
					t.Errorf("expected %d lines, got %d", untruncatedCount, lineCount)
				}
			}
			if expected := len([]rune(textInput)); truncatedRunes+untruncatedRunes != expected {
				t.Errorf("expected %d total runes, got %d (%d truncated)", expected, truncatedRunes+untruncatedRunes, truncatedRunes)
			}
		})
	}
}

// TestWrappingForcedTruncation checks that the line wrapper's truncation features
// activate correctly on multi-paragraph text when later paragraphs are truncated.
func TestWrappingForcedTruncation(t *testing.T) {
	// Use a test string containing multiple newlines to ensure that they are shaped
	// as separate paragraphs.
	textInput := "Lorem ipsum\ndolor sit\namet"
	ltrFace, _ := opentype.Parse(goregular.TTF)
	collection := []FontFace{{Face: ltrFace}}
	cache := NewShaper(NoSystemFonts(), WithCollection(collection))
	cache.LayoutString(Parameters{
		Alignment: Middle,
		PxPerEm:   fixed.I(10),
		MinWidth:  200,
		MaxWidth:  200,
		Locale:    english,
	}, textInput)
	untruncatedCount := len(cache.txt.lines)

	for i := untruncatedCount + 1; i > 0; i-- {
		t.Run(fmt.Sprintf("truncated to %d/%d lines", i, untruncatedCount), func(t *testing.T) {
			cache.LayoutString(Parameters{
				Alignment: Middle,
				PxPerEm:   fixed.I(10),
				MaxLines:  i,
				MinWidth:  200,
				MaxWidth:  200,
				Locale:    english,
			}, textInput)
			lineCount := 0
			glyphs := []Glyph{}
			untruncatedRunes := 0
			truncatedRunes := 0
			for g, ok := cache.NextGlyph(); ok; g, ok = cache.NextGlyph() {
				glyphs = append(glyphs, g)
				if g.Flags&FlagTruncator != 0 && g.Flags&FlagClusterBreak != 0 {
					truncatedRunes += int(g.Runes)
				} else {
					untruncatedRunes += int(g.Runes)
				}
				if g.Flags&FlagLineBreak != 0 {
					lineCount++
				}
			}
			expectedTruncated := false
			expectedLines := 0
			if i < untruncatedCount {
				expectedLines = i
				expectedTruncated = true
			} else if i == untruncatedCount {
				expectedLines = i
				expectedTruncated = false
			} else if i > untruncatedCount {
				expectedLines = untruncatedCount
				expectedTruncated = false
			}
			if lineCount != expectedLines {
				t.Errorf("expected %d lines, got %d", expectedLines, lineCount)
			}
			if truncatedRunes > 0 != expectedTruncated {
				t.Errorf("expected expectedTruncated=%v, truncatedRunes=%d", expectedTruncated, truncatedRunes)
			}
			if expected := len([]rune(textInput)); truncatedRunes+untruncatedRunes != expected {
				t.Errorf("expected %d total runes, got %d (%d truncated)", expected, truncatedRunes+untruncatedRunes, truncatedRunes)
			}
		})
	}
}

// TestShapingNewlineHandling checks that the shaper's newline splitting behaves
// consistently and does not create spurious lines of text.
func TestShapingNewlineHandling(t *testing.T) {
	type testcase struct {
		textInput         string
		expectedLines     int
		expectedGlyphs    int
		maxLines          int
		expectedTruncated int
	}
	for _, tc := range []testcase{
		{textInput: "a\n", expectedLines: 1, expectedGlyphs: 3},
		{textInput: "a\nb", expectedLines: 2, expectedGlyphs: 3},
		{textInput: "", expectedLines: 1, expectedGlyphs: 1},
		{textInput: "\n", expectedLines: 1, expectedGlyphs: 2},
		{textInput: "\n\n", expectedLines: 2, expectedGlyphs: 3},
		{textInput: "\n\n\n", expectedLines: 3, expectedGlyphs: 4},
		{textInput: "\n", expectedLines: 1, maxLines: 1, expectedGlyphs: 1, expectedTruncated: 1},
		{textInput: "\n\n", expectedLines: 1, maxLines: 1, expectedGlyphs: 1, expectedTruncated: 2},
		{textInput: "\n\n\n", expectedLines: 1, maxLines: 1, expectedGlyphs: 1, expectedTruncated: 3},
		{textInput: "a\n", expectedLines: 1, maxLines: 1, expectedGlyphs: 2, expectedTruncated: 1},
		{textInput: "a\n\n", expectedLines: 1, maxLines: 1, expectedGlyphs: 2, expectedTruncated: 2},
		{textInput: "a\n\n\n", expectedLines: 1, maxLines: 1, expectedGlyphs: 2, expectedTruncated: 3},
		{textInput: "\n", expectedLines: 1, maxLines: 2, expectedGlyphs: 2},
		{textInput: "\n\n", expectedLines: 2, maxLines: 2, expectedGlyphs: 2, expectedTruncated: 1},
		{textInput: "\n\n\n", expectedLines: 2, maxLines: 2, expectedGlyphs: 2, expectedTruncated: 2},
		{textInput: "a\n", expectedLines: 1, maxLines: 2, expectedGlyphs: 3},
		{textInput: "a\n\n", expectedLines: 2, maxLines: 2, expectedGlyphs: 3, expectedTruncated: 1},
		{textInput: "a\n\n\n", expectedLines: 2, maxLines: 2, expectedGlyphs: 3, expectedTruncated: 2},
	} {
		t.Run(fmt.Sprintf("%q-maxLines%d", tc.textInput, tc.maxLines), func(t *testing.T) {
			ltrFace, _ := opentype.Parse(goregular.TTF)
			collection := []FontFace{{Face: ltrFace}}
			cache := NewShaper(NoSystemFonts(), WithCollection(collection))
			checkGlyphs := func() {
				glyphs := []Glyph{}
				runes := 0
				truncated := 0
				for g, ok := cache.NextGlyph(); ok; g, ok = cache.NextGlyph() {
					glyphs = append(glyphs, g)
					if g.Flags&FlagTruncator == 0 {
						runes += int(g.Runes)
					} else {
						truncated += int(g.Runes)
					}
				}
				if expected := len([]rune(tc.textInput)) - tc.expectedTruncated; expected != runes {
					t.Errorf("expected %d runes, got %d", expected, runes)
				}
				if truncated != tc.expectedTruncated {
					t.Errorf("expected %d truncated runes, got %d", tc.expectedTruncated, truncated)
				}
				if len(glyphs) != tc.expectedGlyphs {
					t.Errorf("expected %d glyphs, got %d", tc.expectedGlyphs, len(glyphs))
				}
				findBreak := func(g Glyph) bool {
					return g.Flags&FlagParagraphBreak != 0
				}
				found := 0
				for idx := slices.IndexFunc(glyphs, findBreak); idx != -1; idx = slices.IndexFunc(glyphs, findBreak) {
					found++
					breakGlyph := glyphs[idx]
					startGlyph := glyphs[idx+1]
					glyphs = glyphs[idx+1:]
					if flags := breakGlyph.Flags; flags&FlagParagraphBreak == 0 {
						t.Errorf("expected newline glyph to have P flag, got %s", flags)
					}
					if flags := startGlyph.Flags; flags&FlagParagraphStart == 0 {
						t.Errorf("expected newline glyph to have S flag, got %s", flags)
					}
					breakX, breakY := breakGlyph.X, breakGlyph.Y
					startX, startY := startGlyph.X, startGlyph.Y
					if breakX == startX && idx != 0 {
						t.Errorf("expected paragraph start glyph to have cursor x, got %v", startX)
					}
					if breakY == startY {
						t.Errorf("expected paragraph start glyph to have cursor y")
					}
				}
				if count := strings.Count(tc.textInput, "\n"); found != count && tc.maxLines == 0 {
					t.Errorf("expected %d paragraph breaks, found %d", count, found)
				} else if tc.maxLines > 0 && found > tc.maxLines {
					t.Errorf("expected %d paragraph breaks due to truncation, found %d", tc.maxLines, found)
				}
			}
			params := Parameters{
				Alignment: Middle,
				PxPerEm:   fixed.I(10),
				MinWidth:  200,
				MaxWidth:  200,
				Locale:    english,
				MaxLines:  tc.maxLines,
			}
			cache.LayoutString(params, tc.textInput)
			if lineCount := len(cache.txt.lines); lineCount > tc.expectedLines {
				t.Errorf("shaping string %q created %d lines", tc.textInput, lineCount)
			}
			checkGlyphs()

			cache.Layout(params, strings.NewReader(tc.textInput))
			if lineCount := len(cache.txt.lines); lineCount > tc.expectedLines {
				t.Errorf("shaping reader %q created %d lines", tc.textInput, lineCount)
			}
			checkGlyphs()
		})
	}
}

// TestCacheEmptyString ensures that shaping the empty string returns a
// single synthetic glyph with ascent/descent info.
func TestCacheEmptyString(t *testing.T) {
	ltrFace, _ := opentype.Parse(goregular.TTF)
	collection := []FontFace{{Face: ltrFace}}
	cache := NewShaper(NoSystemFonts(), WithCollection(collection))
	cache.LayoutString(Parameters{
		Alignment: Middle,
		PxPerEm:   fixed.I(10),
		MinWidth:  200,
		MaxWidth:  200,
		Locale:    english,
	}, "")
	glyphs := make([]Glyph, 0, 1)
	for g, ok := cache.NextGlyph(); ok; g, ok = cache.NextGlyph() {
		glyphs = append(glyphs, g)
	}
	if len(glyphs) != 1 {
		t.Errorf("expected %d glyphs, got %d", 1, len(glyphs))
	}
	glyph := glyphs[0]
	checkFlag(t, true, FlagClusterBreak, glyph, 0)
	checkFlag(t, true, FlagRunBreak, glyph, 0)
	checkFlag(t, true, FlagLineBreak, glyph, 0)
	checkFlag(t, false, FlagParagraphBreak, glyph, 0)
	if glyph.Ascent == 0 {
		t.Errorf("expected non-zero ascent")
	}
	if glyph.Descent == 0 {
		t.Errorf("expected non-zero descent")
	}
	if glyph.Y == 0 {
		t.Errorf("expected non-zero y offset")
	}
	if glyph.X == 0 {
		t.Errorf("expected non-zero x offset")
	}
}

// TestCacheAlignment ensures that shaping with different alignments or dominant
// text directions results in different X offsets.
func TestCacheAlignment(t *testing.T) {
	ltrFace, _ := opentype.Parse(goregular.TTF)
	collection := []FontFace{{Face: ltrFace}}
	cache := NewShaper(NoSystemFonts(), WithCollection(collection))
	params := Parameters{
		Alignment: Start,
		PxPerEm:   fixed.I(10),
		MinWidth:  200,
		MaxWidth:  200,
		Locale:    english,
	}
	cache.LayoutString(params, "A")
	glyph, _ := cache.NextGlyph()
	startX := glyph.X
	params.Alignment = Middle
	cache.LayoutString(params, "A")
	glyph, _ = cache.NextGlyph()
	middleX := glyph.X
	params.Alignment = End
	cache.LayoutString(params, "A")
	glyph, _ = cache.NextGlyph()
	endX := glyph.X
	if startX == middleX || startX == endX || endX == middleX {
		t.Errorf("[LTR] shaping with with different alignments should not produce the same X, start %d, middle %d, end %d", startX, middleX, endX)
	}
	params.Locale = arabic
	params.Alignment = Start
	cache.LayoutString(params, "A")
	glyph, _ = cache.NextGlyph()
	rtlStartX := glyph.X
	params.Alignment = Middle
	cache.LayoutString(params, "A")
	glyph, _ = cache.NextGlyph()
	rtlMiddleX := glyph.X
	params.Alignment = End
	cache.LayoutString(params, "A")
	glyph, _ = cache.NextGlyph()
	rtlEndX := glyph.X
	if rtlStartX == rtlMiddleX || rtlStartX == rtlEndX || rtlEndX == rtlMiddleX {
		t.Errorf("[RTL] shaping with with different alignments should not produce the same X, start %d, middle %d, end %d", rtlStartX, rtlMiddleX, rtlEndX)
	}
	if startX == rtlStartX || endX == rtlEndX {
		t.Errorf("shaping with with different dominant text directions and the same alignment should not produce the same X unless it's middle-aligned")
	}
}

func TestCacheGlyphConverstion(t *testing.T) {
	ltrFace, _ := opentype.Parse(goregular.TTF)
	rtlFace, _ := opentype.Parse(nsareg.TTF)
	collection := []FontFace{{Face: ltrFace}, {Face: rtlFace}}
	type testcase struct {
		name     string
		text     string
		locale   system.Locale
		expected []Glyph
	}
	for _, tc := range []testcase{
		{
			name:   "bidi ltr",
			text:   "The quick سماء שלום لا fox تمط שלום\nغير the\nlazy dog.",
			locale: english,
		},
		{
			name:   "bidi rtl",
			text:   "الحب سماء brown привет fox تمط jumps\nпривет over\nغير الأحلام.",
			locale: arabic,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := NewShaper(NoSystemFonts(), WithCollection(collection))
			cache.LayoutString(Parameters{
				PxPerEm:  fixed.I(10),
				MaxWidth: 200,
				Locale:   tc.locale,
			}, tc.text)
			doc := cache.txt
			glyphs := make([]Glyph, 0, len(tc.expected))
			for g, ok := cache.NextGlyph(); ok; g, ok = cache.NextGlyph() {
				glyphs = append(glyphs, g)
			}
			glyphCursor := 0
			for _, line := range doc.lines {
				for runIdx, run := range line.runs {
					lastRun := runIdx == len(line.runs)-1
					start := 0
					end := len(run.Glyphs) - 1
					inc := 1
					towardOrigin := false
					if run.Direction.Progression() == system.TowardOrigin {
						start = len(run.Glyphs) - 1
						end = 0
						inc = -1
						towardOrigin = true
					}
					for glyphIdx := start; ; glyphIdx += inc {
						endOfRun := glyphIdx == end
						glyph := run.Glyphs[glyphIdx]
						endOfCluster := glyphIdx == end || run.Glyphs[glyphIdx+inc].clusterIndex != glyph.clusterIndex

						actual := glyphs[glyphCursor]
						if actual.ID != glyph.id {
							t.Errorf("glyphs[%d] expected id %d, got id %d", glyphCursor, glyph.id, actual.ID)
						}
						// Synthetic glyphs should only ever show up at the end of lines.
						endOfLine := lastRun && endOfRun
						synthetic := glyph.glyphCount == 0 && endOfLine
						checkFlag(t, endOfLine, FlagLineBreak, actual, glyphCursor)
						checkFlag(t, endOfRun, FlagRunBreak, actual, glyphCursor)
						checkFlag(t, towardOrigin, FlagTowardOrigin, actual, glyphCursor)
						checkFlag(t, synthetic, FlagParagraphBreak, actual, glyphCursor)
						checkFlag(t, endOfCluster, FlagClusterBreak, actual, glyphCursor)
						glyphCursor++
						if glyphIdx == end {
							break
						}
					}
				}
			}

			printLinePositioning(t, doc.lines, glyphs)
		})
	}
}

func checkFlag(t *testing.T, shouldHave bool, flag Flags, actual Glyph, glyphCursor int) {
	t.Helper()
	if shouldHave && actual.Flags&flag == 0 {
		t.Errorf("glyphs[%d] should have %s set", glyphCursor, flag)
	} else if !shouldHave && actual.Flags&flag != 0 {
		t.Errorf("glyphs[%d] should not have %s set", glyphCursor, flag)
	}
}

func printLinePositioning(t *testing.T, lines []line, glyphs []Glyph) {
	t.Helper()
	glyphCursor := 0
	for i, line := range lines {
		t.Logf("line %d, dir %s, width %d, visual %v, runeCount: %d", i, line.direction, line.width, line.visualOrder, line.runeCount)
		for k, run := range line.runs {
			t.Logf("run: %d, dir %s, width %d, runes {count: %d, offset: %d}", k, run.Direction, run.Advance, run.Runes.Count, run.Runes.Offset)
			start := 0
			end := len(run.Glyphs) - 1
			inc := 1
			if run.Direction.Progression() == system.TowardOrigin {
				start = len(run.Glyphs) - 1
				end = 0
				inc = -1
			}
			for g := start; ; g += inc {
				glyph := run.Glyphs[g]
				if glyphCursor < len(glyphs) {
					t.Logf("glyph %2d, adv %3d, runes %2d, glyphs %d - glyphs[%2d] flags %s", g, glyph.xAdvance, glyph.runeCount, glyph.glyphCount, glyphCursor, glyphs[glyphCursor].Flags)
					t.Logf("glyph %2d, adv %3d, runes %2d, glyphs %d - n/a", g, glyph.xAdvance, glyph.runeCount, glyph.glyphCount)
				}
				glyphCursor++
				if g == end {
					break
				}
			}
		}
	}
}

// TestShapeStringRuneAccounting tries shaping the same string/parameter combinations with both
// shaping methods and ensures that the resulting glyph stream always has the right number of
// runes accounted for.
func TestShapeStringRuneAccounting(t *testing.T) {
	type testcase struct {
		name   string
		input  string
		params Parameters
	}
	type setup struct {
		kind string
		do   func(*Shaper, Parameters, string)
	}
	for _, tc := range []testcase{
		{
			name:  "simple truncated",
			input: "abc",
			params: Parameters{
				PxPerEm:  fixed.Int26_6(16),
				MaxWidth: 100,
				MaxLines: 1,
			},
		},
		{
			name:  "simple",
			input: "abc",
			params: Parameters{
				PxPerEm:  fixed.Int26_6(16),
				MaxWidth: 100,
			},
		},
		{
			name:  "newline regression",
			input: "\n",
			params: Parameters{
				Font:       font.Font{Typeface: "Go", Style: font.Regular, Weight: font.Normal},
				Alignment:  Start,
				PxPerEm:    768,
				MaxLines:   1,
				Truncator:  "\u200b",
				WrapPolicy: WrapHeuristically,
				MaxWidth:   999929,
			},
		},
		{
			name:  "newline zero-width regression",
			input: "\n",
			params: Parameters{
				Font:       font.Font{Typeface: "Go", Style: font.Regular, Weight: font.Normal},
				Alignment:  Start,
				PxPerEm:    768,
				MaxLines:   1,
				Truncator:  "\u200b",
				WrapPolicy: WrapHeuristically,
				MaxWidth:   0,
			},
		},
		{
			name:  "double newline regression",
			input: "\n\n",
			params: Parameters{
				Font:       font.Font{Typeface: "Go", Style: font.Regular, Weight: font.Normal},
				Alignment:  Start,
				PxPerEm:    768,
				MaxLines:   1,
				Truncator:  "\u200b",
				WrapPolicy: WrapHeuristically,
				MaxWidth:   1000,
			},
		},
		{
			name:  "triple newline regression",
			input: "\n\n\n",
			params: Parameters{
				Font:       font.Font{Typeface: "Go", Style: font.Regular, Weight: font.Normal},
				Alignment:  Start,
				PxPerEm:    768,
				MaxLines:   1,
				Truncator:  "\u200b",
				WrapPolicy: WrapHeuristically,
				MaxWidth:   1000,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, setup := range []setup{
				{
					kind: "LayoutString",
					do: func(shaper *Shaper, params Parameters, input string) {
						shaper.LayoutString(params, input)
					},
				},
				{
					kind: "Layout",
					do: func(shaper *Shaper, params Parameters, input string) {
						shaper.Layout(params, strings.NewReader(input))
					},
				},
			} {
				t.Run(setup.kind, func(t *testing.T) {
					shaper := NewShaper(NoSystemFonts(), WithCollection(gofont.Collection()))
					setup.do(shaper, tc.params, tc.input)

					glyphs := []Glyph{}
					for g, ok := shaper.NextGlyph(); ok; g, ok = shaper.NextGlyph() {
						glyphs = append(glyphs, g)
					}
					totalRunes := 0
					for _, g := range glyphs {
						totalRunes += int(g.Runes)
					}
					if inputRunes := len([]rune(tc.input)); totalRunes != inputRunes {
						t.Errorf("input contained %d runes, but glyphs contained %d", inputRunes, totalRunes)
					}
				})
			}
		})
	}
}
