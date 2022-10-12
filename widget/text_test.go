package widget

import (
	"strconv"
	"testing"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

func TestFirstPos(t *testing.T) {
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
	fontSize := 16
	lineWidth := int(fontSize) * 10
	ltrText := shaper.LayoutString(text.Font{Typeface: "LTR"}, fixed.I(fontSize), lineWidth, english, "The quick brown fox\njumps over the lazy dog.")
	rtlText := shaper.LayoutString(text.Font{Typeface: "RTL"}, fixed.I(fontSize), lineWidth, arabic, "الحب سماء لا\nتمط غير الأحلام")

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
