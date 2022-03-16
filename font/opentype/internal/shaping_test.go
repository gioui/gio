package internal

import (
	"bytes"
	"reflect"
	"sort"
	"testing"
	"testing/quick"

	"gioui.org/io/system"
	"gioui.org/text"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// glyph returns a glyph with the given cluster. Its dimensions
// are a square sitting atop the baseline, with 10 units to a side.
func glyph(cluster int) shaping.Glyph {
	return shaping.Glyph{
		XAdvance:     fixed.I(10),
		YAdvance:     fixed.I(10),
		Width:        fixed.I(10),
		Height:       fixed.I(10),
		YBearing:     fixed.I(10),
		ClusterIndex: cluster,
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// glyphs returns a slice of glyphs with clusters from start to
// end. If start is greater than end, the glyphs will be returned
// with descending cluster values.
func glyphs(start, end int) []shaping.Glyph {
	inc := 1
	if start > end {
		inc = -inc
	}
	num := max(start, end) - min(start, end) + 1
	g := make([]shaping.Glyph, 0, num)
	for i := start; i >= 0 && i <= max(start, end); i += inc {
		g = append(g, glyph(i))
	}
	return g
}

func TestMapRunesToClusterIndices(t *testing.T) {
	type testcase struct {
		name     string
		runes    []rune
		glyphs   []shaping.Glyph
		expected []int
	}
	for _, tc := range []testcase{
		{
			name:  "simple",
			runes: make([]rune, 5),
			glyphs: []shaping.Glyph{
				glyph(0),
				glyph(1),
				glyph(2),
				glyph(3),
				glyph(4),
			},
			expected: []int{0, 1, 2, 3, 4},
		},
		{
			name:  "simple rtl",
			runes: make([]rune, 5),
			glyphs: []shaping.Glyph{
				glyph(4),
				glyph(3),
				glyph(2),
				glyph(1),
				glyph(0),
			},
			expected: []int{4, 3, 2, 1, 0},
		},
		{
			name:  "fused clusters",
			runes: make([]rune, 5),
			glyphs: []shaping.Glyph{
				glyph(0),
				glyph(0),
				glyph(2),
				glyph(3),
				glyph(3),
			},
			expected: []int{0, 0, 2, 3, 3},
		},
		{
			name:  "fused clusters rtl",
			runes: make([]rune, 5),
			glyphs: []shaping.Glyph{
				glyph(3),
				glyph(3),
				glyph(2),
				glyph(0),
				glyph(0),
			},
			expected: []int{3, 3, 2, 0, 0},
		},
		{
			name:  "ligatures",
			runes: make([]rune, 5),
			glyphs: []shaping.Glyph{
				glyph(0),
				glyph(2),
				glyph(3),
			},
			expected: []int{0, 0, 1, 2, 2},
		},
		{
			name:  "ligatures rtl",
			runes: make([]rune, 5),
			glyphs: []shaping.Glyph{
				glyph(3),
				glyph(2),
				glyph(0),
			},
			expected: []int{2, 2, 1, 0, 0},
		},
		{
			name:  "expansion",
			runes: make([]rune, 5),
			glyphs: []shaping.Glyph{
				glyph(0),
				glyph(1),
				glyph(1),
				glyph(1),
				glyph(2),
				glyph(3),
				glyph(4),
			},
			expected: []int{0, 1, 4, 5, 6},
		},
		{
			name:  "expansion rtl",
			runes: make([]rune, 5),
			glyphs: []shaping.Glyph{
				glyph(4),
				glyph(3),
				glyph(2),
				glyph(1),
				glyph(1),
				glyph(1),
				glyph(0),
			},
			expected: []int{6, 3, 2, 1, 0},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mapping := mapRunesToClusterIndices(tc.runes, tc.glyphs)
			if !reflect.DeepEqual(tc.expected, mapping) {
				t.Errorf("expected %v, got %v", tc.expected, mapping)
			}
		})
	}
}

func TestInclusiveRange(t *testing.T) {
	type testcase struct {
		name string
		// inputs
		start       int
		breakAfter  int
		runeToGlyph []int
		numGlyphs   int
		// expected outputs
		gs, ge int
	}
	for _, tc := range []testcase{
		{
			name:        "simple at start",
			numGlyphs:   5,
			start:       0,
			breakAfter:  2,
			runeToGlyph: []int{0, 1, 2, 3, 4},
			gs:          0,
			ge:          2,
		},
		{
			name:        "simple in middle",
			numGlyphs:   5,
			start:       1,
			breakAfter:  3,
			runeToGlyph: []int{0, 1, 2, 3, 4},
			gs:          1,
			ge:          3,
		},
		{
			name:        "simple at end",
			numGlyphs:   5,
			start:       2,
			breakAfter:  4,
			runeToGlyph: []int{0, 1, 2, 3, 4},
			gs:          2,
			ge:          4,
		},
		{
			name:        "simple at start rtl",
			numGlyphs:   5,
			start:       0,
			breakAfter:  2,
			runeToGlyph: []int{4, 3, 2, 1, 0},
			gs:          2,
			ge:          4,
		},
		{
			name:        "simple in middle rtl",
			numGlyphs:   5,
			start:       1,
			breakAfter:  3,
			runeToGlyph: []int{4, 3, 2, 1, 0},
			gs:          1,
			ge:          3,
		},
		{
			name:        "simple at end rtl",
			numGlyphs:   5,
			start:       2,
			breakAfter:  4,
			runeToGlyph: []int{4, 3, 2, 1, 0},
			gs:          0,
			ge:          2,
		},
		{
			name:        "fused clusters at start",
			numGlyphs:   5,
			start:       0,
			breakAfter:  1,
			runeToGlyph: []int{0, 0, 2, 3, 3},
			gs:          0,
			ge:          1,
		},
		{
			name:        "fused clusters start and middle",
			numGlyphs:   5,
			start:       0,
			breakAfter:  2,
			runeToGlyph: []int{0, 0, 2, 3, 3},
			gs:          0,
			ge:          2,
		},
		{
			name:        "fused clusters middle and end",
			numGlyphs:   5,
			start:       2,
			breakAfter:  4,
			runeToGlyph: []int{0, 0, 2, 3, 3},
			gs:          2,
			ge:          4,
		},
		{
			name:        "fused clusters at end",
			numGlyphs:   5,
			start:       3,
			breakAfter:  4,
			runeToGlyph: []int{0, 0, 2, 3, 3},
			gs:          3,
			ge:          4,
		},
		{
			name:        "fused clusters at start rtl",
			numGlyphs:   5,
			start:       0,
			breakAfter:  1,
			runeToGlyph: []int{3, 3, 2, 0, 0},
			gs:          3,
			ge:          4,
		},
		{
			name:        "fused clusters start and middle rtl",
			numGlyphs:   5,
			start:       0,
			breakAfter:  2,
			runeToGlyph: []int{3, 3, 2, 0, 0},
			gs:          2,
			ge:          4,
		},
		{
			name:        "fused clusters middle and end rtl",
			numGlyphs:   5,
			start:       2,
			breakAfter:  4,
			runeToGlyph: []int{3, 3, 2, 0, 0},
			gs:          0,
			ge:          2,
		},
		{
			name:        "fused clusters at end rtl",
			numGlyphs:   5,
			start:       3,
			breakAfter:  4,
			runeToGlyph: []int{3, 3, 2, 0, 0},
			gs:          0,
			ge:          1,
		},
		{
			name:        "ligatures at start",
			numGlyphs:   3,
			start:       0,
			breakAfter:  2,
			runeToGlyph: []int{0, 0, 1, 2, 2},
			gs:          0,
			ge:          1,
		},
		{
			name:        "ligatures in middle",
			numGlyphs:   3,
			start:       2,
			breakAfter:  2,
			runeToGlyph: []int{0, 0, 1, 2, 2},
			gs:          1,
			ge:          1,
		},
		{
			name:        "ligatures at end",
			numGlyphs:   3,
			start:       2,
			breakAfter:  4,
			runeToGlyph: []int{0, 0, 1, 2, 2},
			gs:          1,
			ge:          2,
		},
		{
			name:        "ligatures at start rtl",
			numGlyphs:   3,
			start:       0,
			breakAfter:  2,
			runeToGlyph: []int{2, 2, 1, 0, 0},
			gs:          1,
			ge:          2,
		},
		{
			name:        "ligatures in middle rtl",
			numGlyphs:   3,
			start:       2,
			breakAfter:  2,
			runeToGlyph: []int{2, 2, 1, 0, 0},
			gs:          1,
			ge:          1,
		},
		{
			name:        "ligatures at end rtl",
			numGlyphs:   3,
			start:       2,
			breakAfter:  4,
			runeToGlyph: []int{2, 2, 1, 0, 0},
			gs:          0,
			ge:          1,
		},
		{
			name:        "expansion at start",
			numGlyphs:   7,
			start:       0,
			breakAfter:  2,
			runeToGlyph: []int{0, 1, 4, 5, 6},
			gs:          0,
			ge:          4,
		},
		{
			name:        "expansion in middle",
			numGlyphs:   7,
			start:       1,
			breakAfter:  3,
			runeToGlyph: []int{0, 1, 4, 5, 6},
			gs:          1,
			ge:          5,
		},
		{
			name:        "expansion at end",
			numGlyphs:   7,
			start:       2,
			breakAfter:  4,
			runeToGlyph: []int{0, 1, 4, 5, 6},
			gs:          4,
			ge:          6,
		},
		{
			name:        "expansion at start rtl",
			numGlyphs:   7,
			start:       0,
			breakAfter:  2,
			runeToGlyph: []int{6, 3, 2, 1, 0},
			gs:          2,
			ge:          6,
		},
		{
			name:        "expansion in middle rtl",
			numGlyphs:   7,
			start:       1,
			breakAfter:  3,
			runeToGlyph: []int{6, 3, 2, 1, 0},
			gs:          1,
			ge:          5,
		},
		{
			name:        "expansion at end rtl",
			numGlyphs:   7,
			start:       2,
			breakAfter:  4,
			runeToGlyph: []int{6, 3, 2, 1, 0},
			gs:          0,
			ge:          2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gs, ge := inclusiveGlyphRange(tc.start, tc.breakAfter, tc.runeToGlyph, tc.numGlyphs)
			if gs != tc.gs {
				t.Errorf("glyphStart mismatch, got %d, expected %d", gs, tc.gs)
			}
			if ge != tc.ge {
				t.Errorf("glyphEnd mismatch, got %d, expected %d", ge, tc.ge)
			}
		})
	}
}

var (
	// Assume the simple case of 1:1:1 glyph:rune:byte for this input.
	text1       = "text one is ltr"
	shapedText1 = shaping.Output{
		Advance: fixed.I(10 * len([]rune(text1))),
		LineBounds: shaping.Bounds{
			Ascent:  fixed.I(10),
			Descent: fixed.I(5),
			// No line gap.
		},
		GlyphBounds: shaping.Bounds{
			Ascent: fixed.I(10),
			// No glyphs descend.
		},
		Glyphs: glyphs(0, 14),
	}
	text1Trailing       = text1 + " "
	shapedText1Trailing = func() shaping.Output {
		out := shapedText1
		out.Glyphs = append(out.Glyphs, glyph(len(out.Glyphs)))
		out.RecalculateAll()
		return out
	}()
	// Test M:N:O glyph:rune:byte for this input.
	// The substring `lig` is shaped as a ligature.
	// The substring `DROP` is not shaped at all.
	text2       = "안П你 ligDROP 안П你 ligDROP"
	shapedText2 = shaping.Output{
		// There are 11 glyphs shaped for this string.
		Advance: fixed.I(10 * 11),
		LineBounds: shaping.Bounds{
			Ascent:  fixed.I(10),
			Descent: fixed.I(5),
			// No line gap.
		},
		GlyphBounds: shaping.Bounds{
			Ascent: fixed.I(10),
			// No glyphs descend.
		},
		Glyphs: []shaping.Glyph{
			0: glyph(0), // 안        - 4 bytes
			1: glyph(1), // П         - 3 bytes
			2: glyph(2), // 你        - 4 bytes
			3: glyph(3), // <space>   - 1 byte
			4: glyph(4), // lig       - 3 runes, 3 bytes
			// DROP                   - 4 runes, 4 bytes
			5:  glyph(11), // <space> - 1 byte
			6:  glyph(12), // 안      - 4 bytes
			7:  glyph(13), // П       - 3 bytes
			8:  glyph(14), // 你      - 4 bytes
			9:  glyph(15), // <space> - 1 byte
			10: glyph(16), // lig     - 3 runes, 3 bytes
			// DROP                   - 4 runes, 4 bytes
		},
	}
	// Test RTL languages.
	text3       = "שלום أهلا שלום أهلا"
	shapedText3 = shaping.Output{
		// There are 15 glyphs shaped for this string.
		Advance: fixed.I(10 * 15),
		LineBounds: shaping.Bounds{
			Ascent:  fixed.I(10),
			Descent: fixed.I(5),
			// No line gap.
		},
		GlyphBounds: shaping.Bounds{
			Ascent: fixed.I(10),
			// No glyphs descend.
		},
		Glyphs: []shaping.Glyph{
			0: glyph(16), // LIGATURE of three runes:
			//               ا - 3 bytes
			//               ل - 3 bytes
			//               ه - 3 bytes
			1: glyph(15), // أ - 3 bytes
			2: glyph(14), // <space> - 1 byte
			3: glyph(13), // ם - 3 bytes
			4: glyph(12), // ו - 3 bytes
			5: glyph(11), // ל - 3 bytes
			6: glyph(10), // ש - 3 bytes
			7: glyph(9),  // <space> - 1 byte
			8: glyph(6),  // LIGATURE of three runes:
			//               ا - 3 bytes
			//               ل - 3 bytes
			//               ه - 3 bytes
			9:  glyph(5), // أ - 3 bytes
			10: glyph(4), // <space> - 1 byte
			11: glyph(3), // ם - 3 bytes
			12: glyph(2), // ו - 3 bytes
			13: glyph(1), // ל - 3 bytes
			14: glyph(0), // ש - 3 bytes
		},
	}
)

//splitShapedAt splits a single shaped output into multiple. It splits
// on each provided glyph index in indices, with the index being the end of
// a slice range (so it's exclusive). You can think of the index as the
// first glyph of the next output.
func splitShapedAt(shaped shaping.Output, direction di.Direction, indices ...int) []shaping.Output {
	numOut := len(indices) + 1
	outputs := make([]shaping.Output, 0, numOut)
	start := 0
	for _, i := range indices {
		newOut := shaped
		newOut.Glyphs = newOut.Glyphs[start:i]
		newOut.RecalculateAll()
		outputs = append(outputs, newOut)
		start = i
	}
	newOut := shaped
	newOut.Glyphs = newOut.Glyphs[start:]
	newOut.RecalculateAll()
	outputs = append(outputs, newOut)
	return outputs
}

func TestEngineLineWrap(t *testing.T) {
	type testcase struct {
		name      string
		direction di.Direction
		shaped    shaping.Output
		paragraph []rune
		maxWidth  int
		expected  []output
	}
	for _, tc := range []testcase{
		{
			// This test case verifies that no line breaks occur if they are not
			// necessary, and that the proper Offsets are reported in the output.
			name:      "all one line",
			shaped:    shapedText1,
			direction: di.DirectionLTR,
			paragraph: []rune(text1),
			maxWidth:  1000,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: len([]rune(text1)),
					},
					Shaped: shapedText1,
				},
			},
		},
		{
			// This test case verifies that trailing whitespace characters on a
			// line do not just disappear if it's the first line.
			name:      "trailing whitespace",
			shaped:    shapedText1Trailing,
			direction: di.DirectionLTR,
			paragraph: []rune(text1Trailing),
			maxWidth:  1000,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: len([]rune(text1)) + 1,
					},
					Shaped: shapedText1Trailing,
				},
			},
		},
		{
			// This test case verifies that the line wrapper rejects line break
			// candidates that would split a glyph cluster.
			name: "reject mid-cluster line breaks",
			shaped: shaping.Output{
				Advance: fixed.I(10 * 3),
				LineBounds: shaping.Bounds{
					Ascent:  fixed.I(10),
					Descent: fixed.I(5),
					// No line gap.
				},
				GlyphBounds: shaping.Bounds{
					Ascent: fixed.I(10),
					// No glyphs descend.
				},
				Glyphs: []shaping.Glyph{
					simpleGlyph(0),
					complexGlyph(1, 2, 2),
					complexGlyph(1, 2, 2),
				},
			},
			direction: di.DirectionLTR,
			// This unicode data was discovered in a testing/quick failure
			// for widget.Editor. It has the property that the middle two
			// runes form a harfbuzz cluster but also have a legal UAX#14
			// segment break between them.
			paragraph: []rune{0xa8e58, 0x3a4fd, 0x119dd},
			maxWidth:  20,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: 1,
					},
					Shaped: shaping.Output{
						Direction: di.DirectionLTR,
						Advance:   fixed.I(10),
						LineBounds: shaping.Bounds{
							Ascent:  fixed.I(10),
							Descent: fixed.I(5),
						},
						GlyphBounds: shaping.Bounds{
							Ascent: fixed.I(10),
						},
						Glyphs: []shaping.Glyph{
							simpleGlyph(0),
						},
					},
				},
				{
					RuneRange: text.Range{
						Count:  2,
						Offset: 1,
					},
					Shaped: shaping.Output{
						Direction: di.DirectionLTR,
						Advance:   fixed.I(20),
						LineBounds: shaping.Bounds{
							Ascent:  fixed.I(10),
							Descent: fixed.I(5),
						},
						GlyphBounds: shaping.Bounds{
							Ascent: fixed.I(10),
						},
						Glyphs: []shaping.Glyph{
							complexGlyph(1, 2, 2),
							complexGlyph(1, 2, 2),
						},
					},
				},
			},
		},
		{
			// This test case verifies that line breaking does occur, and that
			// all lines have proper offsets.
			name:      "line break on last word",
			shaped:    shapedText1,
			direction: di.DirectionLTR,
			paragraph: []rune(text1),
			maxWidth:  120,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: len([]rune(text1)) - 3,
					},
					Shaped: splitShapedAt(shapedText1, di.DirectionLTR, 12)[0],
				},
				{
					RuneRange: text.Range{
						Offset: len([]rune(text1)) - 3,
						Count:  3,
					},
					Shaped: splitShapedAt(shapedText1, di.DirectionLTR, 12)[1],
				},
			},
		},
		{
			// This test case verifies that many line breaks still result in
			// correct offsets. This test also ensures that leading whitespace
			// is correctly hidden on lines after the first.
			name:      "line break several times",
			shaped:    shapedText1,
			direction: di.DirectionLTR,
			paragraph: []rune(text1),
			maxWidth:  70,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: 5,
					},
					Shaped: splitShapedAt(shapedText1, di.DirectionLTR, 5)[0],
				},
				{
					RuneRange: text.Range{
						Offset: 5,
						Count:  7,
					},
					Shaped: splitShapedAt(shapedText1, di.DirectionLTR, 5, 12)[1],
				},
				{
					RuneRange: text.Range{
						Offset: 12,
						Count:  3,
					},
					Shaped: splitShapedAt(shapedText1, di.DirectionLTR, 12)[1],
				},
			},
		},
		{
			// This test case verifies baseline offset math for more complicated input.
			name:      "all one line 2",
			shaped:    shapedText2,
			direction: di.DirectionLTR,
			paragraph: []rune(text2),
			maxWidth:  1000,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: len([]rune(text2)),
					},
					Shaped: shapedText2,
				},
			},
		},
		{
			// This test case verifies that offset accounting correctly handles complex
			// input across line breaks. It is legal to line-break within words composed
			// of more than one script, so this test expects that to occur.
			name:      "line break several times 2",
			shaped:    shapedText2,
			direction: di.DirectionLTR,
			paragraph: []rune(text2),
			maxWidth:  40,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: len([]rune("안П你 ")),
					},
					Shaped: splitShapedAt(shapedText2, di.DirectionLTR, 4)[0],
				},
				{
					RuneRange: text.Range{
						Count:  len([]rune("ligDROP 안П")),
						Offset: len([]rune("안П你 ")),
					},
					Shaped: splitShapedAt(shapedText2, di.DirectionLTR, 4, 8)[1],
				},
				{
					RuneRange: text.Range{
						Count:  len([]rune("你 ligDROP")),
						Offset: len([]rune("안П你 ligDROP 안П")),
					},
					Shaped: splitShapedAt(shapedText2, di.DirectionLTR, 8, 11)[1],
				},
			},
		},
		{
			// This test case verifies baseline offset math for complex RTL input.
			name:      "all one line 3",
			shaped:    shapedText3,
			direction: di.DirectionLTR,
			paragraph: []rune(text3),
			maxWidth:  1000,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: len([]rune(text3)),
					},
					Shaped: shapedText3,
				},
			},
		},
		{
			// This test case verifies line wrapping logic in RTL mode.
			name:      "line break once [RTL]",
			shaped:    shapedText3,
			direction: di.DirectionRTL,
			paragraph: []rune(text3),
			maxWidth:  100,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: len([]rune("שלום أهلا ")),
					},
					Shaped: splitShapedAt(shapedText3, di.DirectionRTL, 7)[1],
				},
				{
					RuneRange: text.Range{
						Count:  len([]rune("שלום أهلا")),
						Offset: len([]rune("שלום أهلا ")),
					},
					Shaped: splitShapedAt(shapedText3, di.DirectionRTL, 7)[0],
				},
			},
		},
		{
			// This test case verifies line wrapping logic in RTL mode.
			name:      "line break several times [RTL]",
			shaped:    shapedText3,
			direction: di.DirectionRTL,
			paragraph: []rune(text3),
			maxWidth:  50,
			expected: []output{
				{
					RuneRange: text.Range{
						Count: len([]rune("שלום ")),
					},
					Shaped: splitShapedAt(shapedText3, di.DirectionRTL, 10)[1],
				},
				{
					RuneRange: text.Range{
						Count:  len([]rune("أهلا ")),
						Offset: len([]rune("שלום ")),
					},
					Shaped: splitShapedAt(shapedText3, di.DirectionRTL, 7, 10)[1],
				},
				{
					RuneRange: text.Range{
						Count:  len([]rune("שלום ")),
						Offset: len([]rune("שלום أهلا ")),
					},
					Shaped: splitShapedAt(shapedText3, di.DirectionRTL, 2, 7)[1],
				},
				{
					RuneRange: text.Range{
						Count:  len([]rune("أهلا")),
						Offset: len([]rune("שלום أهلا שלום ")),
					},
					Shaped: splitShapedAt(shapedText3, di.DirectionRTL, 2)[0],
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Get a mapping from input runes to output glyphs.
			runeToGlyph := mapRunesToClusterIndices(tc.paragraph, tc.shaped.Glyphs)

			// Fetch line break candidates.
			breaks := getBreakOptions(tc.paragraph)

			outs := lineWrap(tc.shaped, tc.direction, tc.paragraph, runeToGlyph, breaks, tc.maxWidth)
			if len(tc.expected) != len(outs) {
				t.Errorf("expected %d lines, got %d", len(tc.expected), len(outs))
			}
			for i := range tc.expected {
				e := tc.expected[i]
				o := outs[i]
				lenE := len(e.Shaped.Glyphs)
				lenO := len(o.Shaped.Glyphs)
				if lenE != lenO {
					t.Errorf("line %d: expected %d glyphs, got %d", i, lenE, lenO)
				} else {
					for k := range e.Shaped.Glyphs {
						e := e.Shaped.Glyphs[k]
						o := o.Shaped.Glyphs[k]
						if !reflect.DeepEqual(e, o) {
							t.Errorf("line %d: glyph mismatch at index %d, expected: %#v, got %#v", i, k, e, o)
						}
					}
				}
				if e.RuneRange != o.RuneRange {
					t.Errorf("line %d: expected %#v offsets, got %#v", i, e.RuneRange, o.RuneRange)
				}
				if e.Shaped.Direction != o.Shaped.Direction {
					t.Errorf("line %d: expected %v direction, got %v", i, e.Shaped.Direction, o.Shaped.Direction)
				}
				// Reduce the verbosity of the reflect mismatch since we already
				// compared the glyphs.
				e.Shaped.Glyphs = nil
				o.Shaped.Glyphs = nil
				if !reflect.DeepEqual(e.Shaped, o.Shaped) {
					t.Errorf("line %d: expected: %#v, got %#v", i, e, o)
				}
			}
		})
	}
}

func TestEngineDocument(t *testing.T) {
	const doc = `Rutrum quisque non tellus orci ac auctor augue.
At risus viverra adipiscing at.`
	const numLines = 2
	english := system.Locale{
		Language:  "EN",
		Direction: system.LTR,
	}
	docRunes := len([]rune(doc))

	// Override the shaping engine with one that will return a simple
	// square glyph info for each rune in the input.
	shaper := func(in shaping.Input) (shaping.Output, error) {
		o := shaping.Output{
			// TODO: ensure that this is either inclusive or exclusive
			Glyphs: glyphs(in.RunStart, in.RunEnd),
		}
		o.RecalculateAll()
		return o, nil
	}

	lines := Document(shaper, nil, 10, 100, english, bytes.NewBufferString(doc))

	lineRunes := 0
	for i, line := range lines {
		t.Logf("Line %d: runeOffset %d, runes %d",
			i, line.Layout.Runes.Offset, line.Layout.Runes.Count)
		if line.Layout.Runes.Offset != lineRunes {
			t.Errorf("expected line %d to start at byte %d, got %d", i, lineRunes, line.Layout.Runes.Offset)
		}
		lineRunes += line.Layout.Runes.Count
	}
	if lineRunes != docRunes {
		t.Errorf("unexpected count: expected %d runes, got %d runes",
			docRunes, lineRunes)
	}
}

// simpleGlyph returns a simple square glyph with the provided cluster
//value.
func simpleGlyph(cluster int) shaping.Glyph {
	return complexGlyph(cluster, 1, 1)
}

// ligatureGlyph returns a simple square glyph with the provided cluster
//value and number of runes.
func ligatureGlyph(cluster, runes int) shaping.Glyph {
	return complexGlyph(cluster, runes, 1)
}

// expansionGlyph returns a simple square glyph with the provided cluster
//value and number of glyphs.
func expansionGlyph(cluster, glyphs int) shaping.Glyph {
	return complexGlyph(cluster, 1, glyphs)
}

// complexGlyph returns a simple square glyph with the provided cluster
//value, number of associated runes, and number of glyphs in the cluster.
func complexGlyph(cluster, runes, glyphs int) shaping.Glyph {
	return shaping.Glyph{
		Width:        fixed.I(10),
		Height:       fixed.I(10),
		XAdvance:     fixed.I(10),
		YAdvance:     fixed.I(10),
		YBearing:     fixed.I(10),
		ClusterIndex: cluster,
		GlyphCount:   glyphs,
		RuneCount:    runes,
	}
}

func simpleCluster(runeOffset, glyphOffset int, ltr bool) text.GlyphCluster {
	g := text.GlyphCluster{
		Advance: fixed.I(10),
		Runes: text.Range{
			Count:  1,
			Offset: runeOffset,
		},
		Glyphs: text.Range{
			Count:  1,
			Offset: glyphOffset,
		},
	}
	if !ltr {
		g.Advance = -g.Advance
	}
	return g
}

func TestLayoutComputeClusters(t *testing.T) {
	type testcase struct {
		name     string
		line     text.Layout
		lastLine bool
		expected []text.GlyphCluster
	}
	for _, tc := range []testcase{
		{
			name:     "empty",
			expected: []text.GlyphCluster{},
		},
		{
			name: "just newline",
			line: text.Layout{
				Direction: system.LTR,
				Glyphs:    toGioGlyphs([]shaping.Glyph{}),
				Runes: text.Range{
					Count: 1,
				},
			},
			expected: []text.GlyphCluster{
				{
					Runes: text.Range{
						Count: 1,
					},
				},
			},
		},
		{
			name: "simple",
			line: text.Layout{
				Direction: system.LTR,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					simpleGlyph(0),
					simpleGlyph(1),
					simpleGlyph(2),
				}),
				Runes: text.Range{
					Count: 3,
				},
			},
			expected: []text.GlyphCluster{
				simpleCluster(0, 0, true),
				simpleCluster(1, 1, true),
				simpleCluster(2, 2, true),
			},
		},
		{
			name: "simple with newline",
			line: text.Layout{
				Direction: system.LTR,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					simpleGlyph(0),
					simpleGlyph(1),
					simpleGlyph(2),
				}),
				Runes: text.Range{
					Count: 4,
				},
			},
			expected: []text.GlyphCluster{
				simpleCluster(0, 0, true),
				simpleCluster(1, 1, true),
				simpleCluster(2, 2, true),
				{
					Runes: text.Range{
						Count:  1,
						Offset: 3,
					},
					Glyphs: text.Range{
						Offset: 3,
					},
				},
			},
		},
		{
			name: "ligature",
			line: text.Layout{
				Direction: system.LTR,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					ligatureGlyph(0, 2),
					simpleGlyph(2),
					simpleGlyph(3),
				}),
				Runes: text.Range{
					Count: 4,
				},
			},
			expected: []text.GlyphCluster{
				{
					Advance: fixed.I(10),
					Runes: text.Range{
						Count: 2,
					},
					Glyphs: text.Range{
						Count: 1,
					},
				},
				simpleCluster(2, 1, true),
				simpleCluster(3, 2, true),
			},
		},
		{
			name: "ligature with newline",
			line: text.Layout{
				Direction: system.LTR,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					ligatureGlyph(0, 2),
				}),
				Runes: text.Range{
					Count: 3,
				},
			},
			expected: []text.GlyphCluster{
				{
					Advance: fixed.I(10),
					Runes: text.Range{
						Count: 2,
					},
					Glyphs: text.Range{
						Count: 1,
					},
				},
				{
					Runes: text.Range{
						Count:  1,
						Offset: 2,
					},
					Glyphs: text.Range{
						Offset: 1,
					},
				},
			},
		},
		{
			name: "expansion",
			line: text.Layout{
				Direction: system.LTR,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					expansionGlyph(0, 2),
					expansionGlyph(0, 2),
					simpleGlyph(1),
					simpleGlyph(2),
				}),
				Runes: text.Range{
					Count: 3,
				},
			},
			expected: []text.GlyphCluster{
				{
					Advance: fixed.I(20),
					Runes: text.Range{
						Count: 1,
					},
					Glyphs: text.Range{
						Count: 2,
					},
				},
				simpleCluster(1, 2, true),
				simpleCluster(2, 3, true),
			},
		},
		{
			name: "deletion",
			line: text.Layout{
				Direction: system.LTR,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					simpleGlyph(0),
					ligatureGlyph(1, 2),
					simpleGlyph(3),
					simpleGlyph(4),
				}),
				Runes: text.Range{
					Count: 5,
				},
			},
			expected: []text.GlyphCluster{
				simpleCluster(0, 0, true),
				{
					Advance: fixed.I(10),
					Runes: text.Range{
						Count:  2,
						Offset: 1,
					},
					Glyphs: text.Range{
						Count:  1,
						Offset: 1,
					},
				},
				simpleCluster(3, 2, true),
				simpleCluster(4, 3, true),
			},
		},
		{
			name: "simple rtl",
			line: text.Layout{
				Direction: system.RTL,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					simpleGlyph(2),
					simpleGlyph(1),
					simpleGlyph(0),
				}),
				Runes: text.Range{
					Count: 3,
				},
			},
			expected: []text.GlyphCluster{
				simpleCluster(0, 2, false),
				simpleCluster(1, 1, false),
				simpleCluster(2, 0, false),
			},
		},
		{
			name: "simple rtl with newline",
			line: text.Layout{
				Direction: system.RTL,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					simpleGlyph(2),
					simpleGlyph(1),
					simpleGlyph(0),
				}),
				Runes: text.Range{
					Count: 4,
				},
			},
			expected: []text.GlyphCluster{
				simpleCluster(0, 2, false),
				simpleCluster(1, 1, false),
				simpleCluster(2, 0, false),
				{
					Runes: text.Range{
						Count:  1,
						Offset: 3,
					},
				},
			},
		},
		{
			name: "ligature rtl",
			line: text.Layout{
				Direction: system.RTL,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					simpleGlyph(3),
					simpleGlyph(2),
					ligatureGlyph(0, 2),
				}),
				Runes: text.Range{
					Count: 4,
				},
			},
			expected: []text.GlyphCluster{
				{
					Advance: fixed.I(-10),
					Runes: text.Range{
						Count: 2,
					},
					Glyphs: text.Range{
						Count:  1,
						Offset: 2,
					},
				},
				simpleCluster(2, 1, false),
				simpleCluster(3, 0, false),
			},
		},
		{
			name: "ligature rtl with newline",
			line: text.Layout{
				Direction: system.RTL,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					ligatureGlyph(0, 2),
				}),
				Runes: text.Range{
					Count: 3,
				},
			},
			expected: []text.GlyphCluster{
				{
					Advance: fixed.I(-10),
					Runes: text.Range{
						Count: 2,
					},
					Glyphs: text.Range{
						Count: 1,
					},
				},
				{
					Runes: text.Range{
						Count:  1,
						Offset: 2,
					},
				},
			},
		},
		{
			name: "expansion rtl",
			line: text.Layout{
				Direction: system.RTL,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					simpleGlyph(2),
					simpleGlyph(1),
					expansionGlyph(0, 2),
					expansionGlyph(0, 2),
				}),
				Runes: text.Range{
					Count: 3,
				},
			},
			expected: []text.GlyphCluster{
				{
					Advance: fixed.I(-20),
					Runes: text.Range{
						Count: 1,
					},
					Glyphs: text.Range{
						Count:  2,
						Offset: 2,
					},
				},
				simpleCluster(1, 1, false),
				simpleCluster(2, 0, false),
			},
		},
		{
			name: "deletion rtl",
			line: text.Layout{
				Direction: system.RTL,
				Glyphs: toGioGlyphs([]shaping.Glyph{
					simpleGlyph(4),
					simpleGlyph(3),
					ligatureGlyph(1, 2),
					simpleGlyph(0),
				}),
				Runes: text.Range{
					Count: 5,
				},
			},
			expected: []text.GlyphCluster{
				simpleCluster(0, 3, false),
				{
					Advance: fixed.I(-10),
					Runes: text.Range{
						Count:  2,
						Offset: 1,
					},
					Glyphs: text.Range{
						Count:  1,
						Offset: 2,
					},
				},
				simpleCluster(3, 1, false),
				simpleCluster(4, 0, false),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			computeGlyphClusters(&tc.line)
			actual := tc.line.Clusters
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, actual)
			}
		})
	}
}

func TestGetBreakOptions(t *testing.T) {
	if err := quick.Check(func(runes []rune) bool {
		options := getBreakOptions(runes)
		// Ensure breaks are in valid range.
		for _, o := range options {
			if o.breakAtRune < 0 || o.breakAtRune > len(runes)-1 {
				return false
			}
		}
		// Ensure breaks are sorted.
		if !sort.SliceIsSorted(options, func(i, j int) bool {
			return options[i].breakAtRune < options[j].breakAtRune
		}) {
			return false
		}

		// Ensure breaks are unique.
		m := make([]bool, len(runes))
		for _, o := range options {
			if m[o.breakAtRune] {
				return false
			} else {
				m[o.breakAtRune] = true
			}
		}

		return true
	}, nil); err != nil {
		t.Errorf("generated invalid break options: %v", err)
	}
}

func TestLayoutSlice(t *testing.T) {
	type testcase struct {
		name       string
		in         text.Layout
		expected   text.Layout
		start, end int
	}

	ltrGlyphs := toGioGlyphs([]shaping.Glyph{
		simpleGlyph(0),
		complexGlyph(1, 2, 2),
		complexGlyph(1, 2, 2),
		simpleGlyph(3),
		simpleGlyph(4),
		simpleGlyph(5),
		ligatureGlyph(6, 3),
		simpleGlyph(9),
		simpleGlyph(10),
	})
	rtlGlyphs := toGioGlyphs([]shaping.Glyph{
		simpleGlyph(10),
		simpleGlyph(9),
		ligatureGlyph(6, 3),
		simpleGlyph(5),
		simpleGlyph(4),
		simpleGlyph(3),
		complexGlyph(1, 2, 2),
		complexGlyph(1, 2, 2),
		simpleGlyph(0),
	})

	for _, tc := range []testcase{
		{
			name: "ltr",
			in: func() text.Layout {
				l := text.Layout{
					Glyphs:    ltrGlyphs,
					Direction: system.LTR,
					Runes: text.Range{
						Count: 11,
					},
				}
				computeGlyphClusters(&l)
				return l
			}(),
			expected: func() text.Layout {
				l := text.Layout{
					Glyphs:    ltrGlyphs[5:],
					Direction: system.LTR,
					Runes: text.Range{
						Count:  6,
						Offset: 5,
					},
				}
				return l
			}(),
			start: 4,
			end:   8,
		},
		{
			name: "ltr different range",
			in: func() text.Layout {
				l := text.Layout{
					Glyphs:    ltrGlyphs,
					Direction: system.LTR,
					Runes: text.Range{
						Count: 11,
					},
				}
				computeGlyphClusters(&l)
				return l
			}(),
			expected: func() text.Layout {
				l := text.Layout{
					Glyphs:    ltrGlyphs[3:7],
					Direction: system.LTR,
					Runes: text.Range{
						Count:  6,
						Offset: 3,
					},
				}
				return l
			}(),
			start: 2,
			end:   6,
		},
		{
			name: "ltr zero len",
			in: func() text.Layout {
				l := text.Layout{
					Glyphs:    ltrGlyphs,
					Direction: system.LTR,
					Runes: text.Range{
						Count: 11,
					},
				}
				computeGlyphClusters(&l)
				return l
			}(),
			expected: text.Layout{},
			start:    0,
			end:      0,
		},
		{
			name: "rtl",
			in: func() text.Layout {
				l := text.Layout{
					Glyphs:    rtlGlyphs,
					Direction: system.RTL,
					Runes: text.Range{
						Count: 11,
					},
				}
				computeGlyphClusters(&l)
				return l
			}(),
			expected: func() text.Layout {
				l := text.Layout{
					Glyphs:    rtlGlyphs[:4],
					Direction: system.RTL,
					Runes: text.Range{
						Count:  6,
						Offset: 5,
					},
				}
				return l
			}(),
			start: 4,
			end:   8,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.in.Slice(tc.start, tc.end)
			if len(out.Glyphs) != len(tc.expected.Glyphs) {
				t.Errorf("expected %v glyphs, got %v", len(tc.expected.Glyphs), len(out.Glyphs))
			}
			if len(out.Clusters) != len(tc.expected.Clusters) {
				t.Errorf("expected %v clusters, got %v", len(tc.expected.Clusters), len(out.Clusters))
			}
			if out.Runes != tc.expected.Runes {
				t.Errorf("expected %#+v, got %#+v", tc.expected.Runes, out.Runes)
			}
			if out.Direction != tc.expected.Direction {
				t.Errorf("expected %#+v, got %#+v", tc.expected.Direction, out.Direction)
			}
		})
	}
}
