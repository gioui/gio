package widget

import (
	"strconv"
	"testing"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"gioui.org/font/opentype"
	"gioui.org/io/system"
	"gioui.org/text"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

func makeTestText(fontSize, lineWidth int) ([]text.Line, []text.Line) {
	ltrFace, _ := opentype.Parse(goregular.TTF)
	rtlFace, _ := opentype.Parse(nsareg.TTF)

	shaper := text.NewCache([]text.FontFace{
		{
			Font: text.Font{Typeface: "LTR"},
			Face: ltrFace,
		},
		{
			Font: text.Font{Typeface: "RTL"},
			Face: rtlFace,
		},
	})
	ltrText := shaper.LayoutString(text.Font{Typeface: "LTR"}, fixed.I(fontSize), lineWidth, english, "The quick brown fox\njumps over the lazy dog.")
	rtlText := shaper.LayoutString(text.Font{Typeface: "RTL"}, fixed.I(fontSize), lineWidth, arabic, "الحب سماء لا\nتمط غير الأحلام")
	return ltrText, rtlText
}

func TestFirstPos(t *testing.T) {
	fontSize := 16
	lineWidth := fontSize * 10
	ltrText, rtlText := makeTestText(fontSize, lineWidth)
	type testcase struct {
		name      string
		line      text.Line
		xAlignMap map[text.Alignment]map[int]fixed.Int26_6
		expected  combinedPos
	}
	for _, tc := range []testcase{
		{
			name: "ltr line 0",
			line: ltrText[0],
			xAlignMap: map[text.Alignment]map[int]fixed.Int26_6{
				text.Start: {
					lineWidth:     0,
					lineWidth * 2: 0,
				},
				text.Middle: {
					lineWidth:     fixed.I(8),
					lineWidth * 2: fixed.I(88),
				},
				text.End: {
					lineWidth:     fixed.I(16),
					lineWidth * 2: fixed.I(176),
				},
			},
			expected: combinedPos{
				x: 0,
				y: ltrText[0].Ascent.Ceil(),
			},
		},
		{
			name: "ltr line 1",
			line: ltrText[1],
			xAlignMap: map[text.Alignment]map[int]fixed.Int26_6{
				text.Start: {
					lineWidth:     0,
					lineWidth * 2: 0,
				},
				text.Middle: {
					lineWidth:     fixed.I(8),
					lineWidth * 2: fixed.I(88),
				},
				text.End: {
					lineWidth:     fixed.I(16),
					lineWidth * 2: fixed.I(176),
				},
			},
			expected: combinedPos{
				x: 0,
				y: ltrText[1].Ascent.Ceil(),
			},
		},
		{
			name: "rtl line 0",
			line: rtlText[0],
			xAlignMap: map[text.Alignment]map[int]fixed.Int26_6{
				text.End: {
					lineWidth:     rtlText[0].Width,
					lineWidth * 2: rtlText[0].Width,
				},
				text.Middle: {
					lineWidth:     fixed.Int26_6(7827),
					lineWidth * 2: fixed.Int26_6(12947),
				},
				text.Start: {
					lineWidth:     fixed.Int26_6(10195),
					lineWidth * 2: fixed.Int26_6(20435),
				},
			},
			expected: combinedPos{
				x: 0,
				y: rtlText[0].Ascent.Ceil(),
			},
		},
		{
			name: "rtl line 1",
			line: rtlText[1],
			xAlignMap: map[text.Alignment]map[int]fixed.Int26_6{
				text.End: {
					lineWidth:     rtlText[1].Width,
					lineWidth * 2: rtlText[1].Width,
				},
				text.Middle: {
					lineWidth:     fixed.Int26_6(8184),
					lineWidth * 2: fixed.Int26_6(13304),
				},
				text.Start: {
					lineWidth:     fixed.Int26_6(10232),
					lineWidth * 2: fixed.Int26_6(20472),
				},
			},
			expected: combinedPos{
				x: 0,
				y: rtlText[1].Ascent.Ceil(),
			},
		},
	} {
		for align, cases := range tc.xAlignMap {
			for width, expectedX := range cases {
				t.Run(tc.name+" "+align.String()+" "+strconv.Itoa(width), func(t *testing.T) {
					actual := firstPos(tc.line, align, width)
					tc.expected.x = expectedX
					if tc.expected.x != actual.x {
						t.Errorf("expected x=%s(%d), got %s(%d)", tc.expected.x, tc.expected.x, actual.x, actual.x)
					}
					if tc.expected.y != actual.y {
						t.Errorf("expected y=%d(%d), got %d(%d)", tc.expected.y, tc.expected.y, actual.y, actual.y)
					}
				})
			}
		}
	}
}

func TestIncrementPosition(t *testing.T) {
	fontSize := 16
	lineWidth := fontSize * 3
	ltrText, rtlText := makeTestText(fontSize, lineWidth)
	type trial struct {
		input, output combinedPos
	}
	type testcase struct {
		name       string
		align      text.Alignment
		width      int
		lines      []text.Line
		firstInput combinedPos
		check      func(t *testing.T, iteration int, input, output combinedPos, end bool)
	}
	for _, tc := range []testcase{
		{
			name:       "ltr",
			align:      text.Start,
			width:      lineWidth,
			lines:      ltrText,
			firstInput: firstPos(ltrText[0], text.Start, lineWidth),
		},
		{
			name:       "rtl",
			align:      text.Start,
			width:      lineWidth,
			lines:      rtlText,
			firstInput: firstPos(rtlText[0], text.Start, lineWidth),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := tc.firstInput
			for i := 0; true; i++ {
				output, end := incrementPosition(tc.lines, tc.align, tc.width, input)
				finalRunes := tc.lines[len(tc.lines)-1].Layout.Runes
				finalRune := finalRunes.Count + finalRunes.Offset
				if end && output.runes != finalRune {
					t.Errorf("iteration %d ended prematurely. Has runes %d, expected %d", i, output.runes, finalRune)
				}
				if end {
					break
				}
				if input == output {
					t.Errorf("iteration %d: identical output:\ninput:  %#+v\noutput: %#+v", i, input, output)
				}
				// We should always advance on either the X or Y axis.
				if input.y == output.y {
					expectedAdvance := tc.lines[input.lineCol.Y].Layout.Clusters[input.clusterIndex].Advance != 0
					rtl := tc.lines[input.lineCol.Y].Layout.Direction.Progression() == system.TowardOrigin
					if expectedAdvance {
						if (rtl && input.x <= output.x) || (!rtl && input.x >= output.x) {
							t.Errorf("iteration %d advanced the wrong way on x axis: input %v(%d) output %v(%d)", i, input.x, input.x, output.x, output.x)
						}
					} else if input.x != output.x {
						t.Errorf("iteration %d advanced x axis when it should not have: input %v(%d) output %v(%d)", i, input.x, input.x, output.x, output.x)
					}
					// If we stayed on the same line, the line-local rune count should
					// be incremented.
					if input.lineCol.X >= output.lineCol.X {
						t.Errorf("iteration %d advanced lineCol.X incorrectly: input %d output %d", i, input.lineCol.X, output.lineCol.X)
					}
					// We don't necessarily increment clusters every time, but it should never
					// go down.
					if input.clusterIndex > output.clusterIndex {
						t.Errorf("iteration %d advanced clusterIndex incorrectly: input %d output %d", i, input.clusterIndex, output.clusterIndex)
					}
				} else {
					if input.y >= output.y {
						t.Errorf("iteration %d advanced the wrong way on y axis: input %v(%d) output %v(%d)", i, input.y, input.y, output.y, output.y)
					} else {
						// We correctly advanced on Y axis, so X should be reset to "start of line"
						// for the text direction.
						rtl := tc.lines[input.lineCol.Y].Layout.Direction.Progression() == system.TowardOrigin
						if (rtl && input.x >= output.x) || (!rtl && input.x <= output.x) {
							t.Errorf("iteration %d reset x axis incorrectly: input %v(%d) output %v(%d)", i, input.x, input.x, output.x, output.x)
						}
					}
					if input.lineCol.Y >= output.lineCol.Y {
						t.Errorf("iteration %d advanced lineCol.Y incorrectly: input %d output %d", i, input.lineCol.Y, output.lineCol.Y)
					}
					if output.clusterIndex != 0 {
						t.Errorf("iteration %d should have zeroed clusterIndex, got: %d", i, output.clusterIndex)
					}
					if output.lineCol.X != 0 {
						t.Errorf("iteration %d should have zeroed lineCol.X, got: %d", i, output.lineCol.X)
					}
				}
				if output.runes != input.runes+1 {
					t.Errorf("iteration %d advanced runes incorrectly: input %d output %d", i, input.runes, output.runes)
				}
				input = output
			}
		})
	}
}
