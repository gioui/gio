package widget

import (
	"image"
	"math"
	"testing"

	"gioui.org/text"
	"golang.org/x/image/math/fixed"
)

// TestGlyphIterator ensures that the glyph iterator computes correct bounding
// boxes and baselines for a variety of glyph sequences.
func TestGlyphIterator(t *testing.T) {
	fontSize := 16
	stdAscent := fixed.I(fontSize)
	stdDescent := fixed.I(4)
	stdLineHeight := stdAscent + stdDescent
	type testcase struct {
		name             string
		str              string
		maxWidth         int
		maxLines         int
		viewport         image.Rectangle
		expectedDims     image.Rectangle
		expectedBaseline int
		stopAtGlyph      int
	}
	for _, tc := range []testcase{
		{
			name:     "empty string",
			str:      "",
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)},
			expectedDims: image.Rectangle{
				Max: image.Point{X: 0, Y: stdLineHeight.Round()},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      0,
		},
		{
			name:     "simple",
			str:      "MMM",
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)},
			expectedDims: image.Rectangle{
				Max: image.Point{X: 40, Y: stdLineHeight.Round()},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      2,
		},
		{
			name:     "simple clipped horizontally",
			str:      "MMM",
			viewport: image.Rectangle{Max: image.Pt(20, math.MaxInt)},
			// The dimensions should only include the first two glyphs.
			expectedDims: image.Rectangle{
				Max: image.Point{X: 27, Y: stdLineHeight.Round()},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      2,
		},
		{
			name:     "simple clipped vertically",
			str:      "M\nM\nM\nM",
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, 2*stdLineHeight.Floor()-3)},
			// The dimensions should only include the first two lines.
			expectedDims: image.Rectangle{
				Max: image.Point{X: 14, Y: 39},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      4,
		},
		{
			name:     "simple truncated",
			str:      "mmm",
			maxLines: 1,
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)},
			// This truncation should have no effect because the text is already one line.
			expectedDims: image.Rectangle{
				Max: image.Point{X: 40, Y: stdLineHeight.Round()},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      2,
		},
		{
			name:     "whitespace",
			str:      "   ",
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)},
			expectedDims: image.Rectangle{
				Max: image.Point{X: 14, Y: stdLineHeight.Round()},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      2,
		},
		{
			name:     "multi-line with hard newline",
			str:      "你\n好",
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)},
			expectedDims: image.Rectangle{
				Max: image.Point{X: 12, Y: 39},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      3,
		},
		{
			name:     "multi-line with soft newline",
			str:      "你好", // UAX#14 allows line breaking between these characters.
			maxWidth: fontSize,
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)},
			expectedDims: image.Rectangle{
				Max: image.Point{X: 12, Y: 39},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      2,
		},
		{
			name:     "trailing hard newline",
			str:      "m\n",
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)},
			// We expect the dimensions to account for two vertical lines because of the
			// newline at the end.
			expectedDims: image.Rectangle{
				Max: image.Point{X: 14, Y: 39},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      1,
		},
		{
			name:     "truncated trailing hard newline",
			str:      "m\n",
			maxLines: 1,
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)},
			// We expect the dimensions to reflect only a single line despite the newline
			// at the end.
			expectedDims: image.Rectangle{
				Max: image.Point{X: 14, Y: 20},
			},
			expectedBaseline: fontSize,
			stopAtGlyph:      1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			maxWidth := 200
			if tc.maxWidth != 0 {
				maxWidth = tc.maxWidth
			}
			glyphs := getGlyphs(16, 0, maxWidth, text.Start, tc.str)
			it := textIterator{viewport: tc.viewport, maxLines: tc.maxLines}
			for i, g := range glyphs {
				ok := it.processGlyph(g, true)
				if !ok && i != tc.stopAtGlyph {
					t.Errorf("expected iterator to stop at glyph %d, stopped at %d", tc.stopAtGlyph, i)
				}
				if !ok {
					break
				}
			}
			if it.bounds != tc.expectedDims {
				t.Errorf("expected bounds %#+v, got %#+v", tc.expectedDims, it.bounds)
			}
			if it.baseline != tc.expectedBaseline {
				t.Errorf("expected baseline %d, got %d", tc.expectedBaseline, it.baseline)
			}
		})
	}
}

// TestGlyphIteratorPadding ensures that the glyph iterator computes correct padding
// around glyphs with unusual bounding boxes.
func TestGlyphIteratorPadding(t *testing.T) {
	type testcase struct {
		name             string
		glyph            text.Glyph
		viewport         image.Rectangle
		expectedDims     image.Rectangle
		expectedPadding  image.Rectangle
		expectedBaseline int
	}
	for _, tc := range []testcase{
		{
			name: "simple",
			glyph: text.Glyph{
				X:       0,
				Y:       50,
				Advance: fixed.I(50),
				Ascent:  fixed.I(50),
				Descent: fixed.I(50),
				Bounds: fixed.Rectangle26_6{
					Min: fixed.Point26_6{
						X: fixed.I(-5),
						Y: fixed.I(-56),
					},
					Max: fixed.Point26_6{
						X: fixed.I(57),
						Y: fixed.I(58),
					},
				},
			},
			viewport: image.Rectangle{Max: image.Pt(math.MaxInt, math.MaxInt)},
			expectedDims: image.Rectangle{
				Max: image.Point{X: 50, Y: 100},
			},
			expectedBaseline: 50,
			expectedPadding: image.Rectangle{
				Min: image.Point{
					X: -5,
					Y: -6,
				},
				Max: image.Point{
					X: 7,
					Y: 8,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			it := textIterator{viewport: tc.viewport}
			it.processGlyph(tc.glyph, true)
			if it.bounds != tc.expectedDims {
				t.Errorf("expected bounds %#+v, got %#+v", tc.expectedDims, it.bounds)
			}
			if it.baseline != tc.expectedBaseline {
				t.Errorf("expected baseline %d, got %d", tc.expectedBaseline, it.baseline)
			}
			if it.padding != tc.expectedPadding {
				t.Errorf("expected padding %d, got %d", tc.expectedPadding, it.padding)
			}
		})
	}
}
