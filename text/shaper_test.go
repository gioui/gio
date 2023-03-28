package text

import (
	"fmt"
	"strings"
	"testing"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
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
	cache := NewShaper(collection)
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
					truncatedRunes += g.Runes
				} else {
					untruncatedRunes += g.Runes
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
	cache := NewShaper(collection)
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
					truncatedRunes += g.Runes
				} else {
					untruncatedRunes += g.Runes
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
		textInput      string
		expectedLines  int
		expectedGlyphs int
	}
	for _, tc := range []testcase{
		{textInput: "a\n", expectedLines: 1, expectedGlyphs: 3},
		{textInput: "a\nb", expectedLines: 2, expectedGlyphs: 3},
		{textInput: "", expectedLines: 1, expectedGlyphs: 1},
	} {
		t.Run(fmt.Sprintf("%q", tc.textInput), func(t *testing.T) {
			ltrFace, _ := opentype.Parse(goregular.TTF)
			collection := []FontFace{{Face: ltrFace}}
			cache := NewShaper(collection)
			checkGlyphs := func() {
				glyphs := []Glyph{}
				for g, ok := cache.NextGlyph(); ok; g, ok = cache.NextGlyph() {
					glyphs = append(glyphs, g)
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
					if breakX == startX {
						t.Errorf("expected paragraph start glyph to have cursor x")
					}
					if breakY == startY {
						t.Errorf("expected paragraph start glyph to have cursor y")
					}
				}
				if count := strings.Count(tc.textInput, "\n"); found != count {
					t.Errorf("expected %d paragraph breaks, found %d", count, found)
				}
			}
			cache.LayoutString(Parameters{
				Alignment: Middle,
				PxPerEm:   fixed.I(10),
				MinWidth:  200,
				MaxWidth:  200,
				Locale:    english,
			}, tc.textInput)
			if lineCount := len(cache.txt.lines); lineCount > tc.expectedLines {
				t.Errorf("shaping string %q created %d lines", tc.textInput, lineCount)
			}
			checkGlyphs()

			cache.Layout(Parameters{
				Alignment: Middle,
				PxPerEm:   fixed.I(10),
				MinWidth:  200,
				MaxWidth:  200,
				Locale:    english,
			}, strings.NewReader(tc.textInput))
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
	cache := NewShaper(collection)
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
	cache := NewShaper(collection)
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
			cache := NewShaper(collection)
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
