// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"unicode"
	"unicode/utf8"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"eliasnaur.com/font/roboto/robotoregular"
	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

var english = system.Locale{
	Language:  "EN",
	Direction: system.LTR,
}

// TestEditorZeroDimensions ensures that an empty editor still reserves
// space for displaying its caret when the constraints allow for it.
func TestEditorZeroDimensions(t *testing.T) {
	gtx := layout.Context{
		Ops: new(op.Ops),
		Constraints: layout.Constraints{
			Max: image.Pt(100, 100),
		},
		Locale: english,
	}
	cache := text.NewCache(gofont.Collection())
	fontSize := unit.Sp(10)
	font := text.Font{}
	e := new(Editor)
	dims := e.Layout(gtx, cache, font, fontSize, nil)
	if dims.Size.X < 1 || dims.Size.Y < 1 {
		t.Errorf("expected empty editor to occupy enough space to display cursor, but returned dimensions %v", dims)
	}
}

func TestEditorConfigurations(t *testing.T) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Locale:      english,
	}
	cache := text.NewCache(gofont.Collection())
	fontSize := unit.Sp(10)
	font := text.Font{}
	sentence := "\n\n\n\n\n\n\n\n\n\n\n\nthe quick brown fox jumps over the lazy dog"
	runes := len([]rune(sentence))

	// Ensure that both ends of the text are reachable in all permutations
	// of settings that influence layout.
	for _, lineMode := range []bool{true, false} {
		for _, alignment := range []text.Alignment{text.Start, text.Middle, text.End} {
			t.Run(fmt.Sprintf("SingleLine: %v Alignment: %v", lineMode, alignment), func(t *testing.T) {
				defer func() {
					if err := recover(); err != nil {
						t.Error(err)
					}
				}()
				e := new(Editor)
				e.SingleLine = lineMode
				e.Alignment = alignment
				e.SetText(sentence)
				e.SetCaret(0, 0)
				e.Layout(gtx, cache, font, fontSize, nil)
				e.SetCaret(runes, runes)
				e.Layout(gtx, cache, font, fontSize, nil)
				coords := e.CaretCoords()
				if int(coords.X) > gtx.Constraints.Max.X || int(coords.Y) > gtx.Constraints.Max.Y {
					t.Errorf("caret coordinates %v exceed constraints %v", coords, gtx.Constraints.Max)
				}
			})
		}
	}
}

func TestEditor(t *testing.T) {
	e := new(Editor)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Locale:      english,
	}
	cache := text.NewCache(gofont.Collection())
	fontSize := unit.Sp(10)
	font := text.Font{}

	// Regression test for bad in-cluster rune offset math.
	e.SetText("æbc")
	e.Layout(gtx, cache, font, fontSize, nil)
	e.moveEnd(selectionClear)
	assertCaret(t, e, 0, 3, len("æbc"))

	textSample := "æbc\naøå••"
	e.SetCaret(0, 0) // shouldn't panic
	assertCaret(t, e, 0, 0, 0)
	e.SetText(textSample)
	if got, exp := e.Len(), utf8.RuneCountInString(e.Text()); got != exp {
		t.Errorf("got length %d, expected %d", got, exp)
	}
	e.Layout(gtx, cache, font, fontSize, nil)
	assertCaret(t, e, 0, 0, 0)
	e.moveEnd(selectionClear)
	assertCaret(t, e, 0, 3, len("æbc"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 0, len("æbc\n"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 3, len("æbc"))
	e.moveLines(+1, selectionClear)
	assertCaret(t, e, 1, 4, len("æbc\naøå•"))
	e.moveEnd(selectionClear)
	assertCaret(t, e, 1, 5, len("æbc\naøå••"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 5, len("æbc\naøå••"))
	e.moveLines(3, selectionClear)

	e.SetCaret(0, 0)
	assertCaret(t, e, 0, 0, 0)
	e.SetCaret(utf8.RuneCountInString("æ"), utf8.RuneCountInString("æ"))
	assertCaret(t, e, 0, 1, 2)
	e.SetCaret(utf8.RuneCountInString("æbc\naøå•"), utf8.RuneCountInString("æbc\naøå•"))
	assertCaret(t, e, 1, 4, len("æbc\naøå•"))

	// Ensure that password masking does not affect caret behavior
	e.MoveCaret(-3, -3)
	assertCaret(t, e, 1, 1, len("æbc\na"))
	e.Mask = '*'
	e.Layout(gtx, cache, font, fontSize, nil)
	assertCaret(t, e, 1, 1, len("æbc\na"))
	e.MoveCaret(-3, -3)
	assertCaret(t, e, 0, 2, len("æb"))
	e.Mask = '\U0001F92B'
	e.Layout(gtx, cache, font, fontSize, nil)
	e.moveEnd(selectionClear)
	assertCaret(t, e, 0, 3, len("æbc"))

	// When a password mask is applied, it should replace all visible glyphs
	spaces := 0
	for _, r := range textSample {
		if unicode.IsSpace(r) {
			spaces++
		}
	}
	nonSpaces := len([]rune(textSample)) - spaces
	glyphCounts := make(map[int]int)
	for _, line := range e.lines {
		for _, glyph := range line.Layout.Glyphs {
			glyphCounts[int(glyph.ID)]++
		}
	}
	if len(glyphCounts) > 2 {
		t.Errorf("masked text contained glyphs other than mask and whitespace")
	}

	for gid, count := range glyphCounts {
		if count != spaces && count != nonSpaces {
			t.Errorf("glyph with id %d occurred %d times, expected either %d or %d", gid, count, spaces, nonSpaces)
		}
	}

	// Test that moveLine applies x offsets from previous moves.
	e.SetText("long line\nshort")
	e.SetCaret(0, 0)
	e.moveEnd(selectionClear)
	e.moveLines(+1, selectionClear)
	e.moveLines(-1, selectionClear)
	assertCaret(t, e, 0, utf8.RuneCountInString("long line"), len("long line"))
}

var arabic = system.Locale{
	Language:  "AR",
	Direction: system.RTL,
}

var arabicCollection = func() []text.FontFace {
	parsed, _ := opentype.Parse(nsareg.TTF)
	return []text.FontFace{{Font: text.Font{}, Face: parsed}}
}()

func TestEditorRTL(t *testing.T) {
	e := new(Editor)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Locale:      arabic,
	}
	cache := text.NewCache(arabicCollection)
	fontSize := unit.Sp(10)
	font := text.Font{}

	e.SetCaret(0, 0) // shouldn't panic
	assertCaret(t, e, 0, 0, 0)

	// Set the text to a single RTL word. The caret should start at 0 column
	// zero, but this is the first column on the right.
	e.SetText("الحب")
	e.Layout(gtx, cache, font, fontSize, nil)
	assertCaret(t, e, 0, 0, 0)
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 0, 1, len("ا"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 0, 2, len("ال"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 0, 3, len("الح"))
	// Move to the "end" of the line. This moves to the left edge of the line.
	e.moveEnd(selectionClear)
	assertCaret(t, e, 0, 4, len("الحب"))

	sentence := "الحب سماء لا\nتمط غير الأحلام"
	e.SetText(sentence)
	e.Layout(gtx, cache, font, fontSize, nil)
	assertCaret(t, e, 0, 0, 0)
	e.moveEnd(selectionClear)
	assertCaret(t, e, 0, 12, len("الحب سماء لا"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 0, len("الحب سماء لا\n"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 1, len("الحب سماء لا\nت"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 1, 0, len("الحب سماء لا\n"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 12, len("الحب سماء لا"))
	e.moveLines(+1, selectionClear)
	assertCaret(t, e, 1, 14, len("الحب سماء لا\nتمط غير الأحلا"))
	e.moveEnd(selectionClear)
	assertCaret(t, e, 1, 15, len("الحب سماء لا\nتمط غير الأحلام"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 15, len("الحب سماء لا\nتمط غير الأحلام"))
	e.moveLines(3, selectionClear)
	assertCaret(t, e, 1, 15, len("الحب سماء لا\nتمط غير الأحلام"))
	e.SetCaret(utf8.RuneCountInString(sentence), 0)
	assertCaret(t, e, 1, 15, len("الحب سماء لا\nتمط غير الأحلام"))
	if selection := e.SelectedText(); selection != sentence {
		t.Errorf("expected selection %s, got %s", sentence, selection)
	}

	e.SetCaret(0, 0)
	assertCaret(t, e, 0, 0, 0)
	e.SetCaret(utf8.RuneCountInString("ا"), utf8.RuneCountInString("ا"))
	assertCaret(t, e, 0, 1, len("ا"))
	e.SetCaret(utf8.RuneCountInString("الحب سماء لا\nتمط غ"), utf8.RuneCountInString("الحب سماء لا\nتمط غ"))
	assertCaret(t, e, 1, 5, len("الحب سماء لا\nتمط غ"))
}

func TestEditorLigature(t *testing.T) {
	e := new(Editor)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Locale:      english,
	}
	face, err := opentype.Parse(robotoregular.TTF)
	if err != nil {
		t.Skipf("failed parsing test font: %v", err)
	}
	cache := text.NewCache([]text.FontFace{
		{
			Font: text.Font{
				Typeface: "Roboto",
			},
			Face: face,
		},
	})
	fontSize := unit.Sp(10)
	font := text.Font{}

	/*
		In this font, the following rune sequences form ligatures:

		- ffi
		- ffl
		- fi
		- fl
	*/

	e.SetCaret(0, 0) // shouldn't panic
	assertCaret(t, e, 0, 0, 0)
	e.SetText("fl") // just a ligature
	e.Layout(gtx, cache, font, fontSize, nil)
	e.moveEnd(selectionClear)
	assertCaret(t, e, 0, 2, len("fl"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 1, len("f"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 0, 0)
	e.MoveCaret(+2, +2)
	assertCaret(t, e, 0, 2, len("fl"))
	e.SetText("flaffl•ffi\n•fflfi") // 3 ligatures on line 0, 2 on line 1
	e.Layout(gtx, cache, font, fontSize, nil)
	assertCaret(t, e, 0, 0, 0)
	e.moveEnd(selectionClear)
	assertCaret(t, e, 0, 10, len("ffaffl•ffi"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 0, len("ffaffl•ffi\n"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 1, len("ffaffl•ffi\n•"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 2, len("ffaffl•ffi\n•f"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 3, len("ffaffl•ffi\n•ff"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 4, len("ffaffl•ffi\n•ffl"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 5, len("ffaffl•ffi\n•fflf"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 6, len("ffaffl•ffi\n•fflfi"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 1, 5, len("ffaffl•ffi\n•fflf"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 1, 4, len("ffaffl•ffi\n•ffl"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 1, 3, len("ffaffl•ffi\n•ff"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 1, 2, len("ffaffl•ffi\n•f"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 1, 1, len("ffaffl•ffi\n•"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 1, 0, len("ffaffl•ffi\n"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 10, len("ffaffl•ffi"))
	e.MoveCaret(-2, -2)
	assertCaret(t, e, 0, 8, len("ffaffl•f"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 7, len("ffaffl•"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 6, len("ffaffl"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 5, len("ffaff"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 4, len("ffaf"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 3, len("ffa"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 2, len("ff"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 1, len("f"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 0, 0)
	gtx.Constraints = layout.Exact(image.Pt(50, 50))
	e.SetText("fflffl fflffl fflffl fflffl") // Many ligatures broken across lines.
	e.Layout(gtx, cache, font, fontSize, nil)
	// Ensure that all runes in the final cluster of a line are properly
	// decoded when moving to the end of the line. This is a regression test.
	e.moveEnd(selectionClear)
	// Because the first line is broken due to line wrapping, the last
	// rune of the line will not be before the cursor.
	assertCaret(t, e, 0, 13, len("fflffl fflffl"))
	e.moveLines(1, selectionClear)
	// Because the space is at the beginning of the second line and
	// the second line is the final line, there are two more runes
	// before the cursor than on the first line.
	assertCaret(t, e, 1, 13, len("fflffl fflffl fflffl fflffl"))
	e.moveLines(-1, selectionClear)
	assertCaret(t, e, 0, 13, len("fflffl fflffl"))

	// Absurdly narrow constraints to force each ligature onto its own line.
	gtx.Constraints = layout.Exact(image.Pt(10, 10))
	e.SetText("ffl ffl") // Two ligatures on separate lines.
	e.Layout(gtx, cache, font, fontSize, nil)
	assertCaret(t, e, 0, 0, 0)
	e.MoveCaret(1, 1) // Move the caret into the first ligature.
	assertCaret(t, e, 0, 1, len("f"))
	e.MoveCaret(4, 4) // Move the caret several positions.
	assertCaret(t, e, 1, 1, len("ffl f"))
}

func TestEditorDimensions(t *testing.T) {
	e := new(Editor)
	tq := &testQueue{
		events: []event.Event{
			key.EditEvent{Text: "A"},
		},
	}
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Constraints{Max: image.Pt(100, 100)},
		Queue:       tq,
		Locale:      english,
	}
	cache := text.NewCache(gofont.Collection())
	fontSize := unit.Sp(10)
	font := text.Font{}
	dims := e.Layout(gtx, cache, font, fontSize, nil)
	if dims.Size.X == 0 {
		t.Errorf("EditEvent was not reflected in Editor width")
	}
}

// assertCaret asserts that the editor caret is at a particular line
// and column, and that the byte position matches as well.
func assertCaret(t *testing.T, e *Editor, line, col, bytes int) {
	t.Helper()
	gotLine, gotCol := e.CaretPos()
	if gotLine != line || gotCol != col {
		t.Errorf("caret at (%d, %d), expected (%d, %d)", gotLine, gotCol, line, col)
	}
	caretBytes := e.runeOffset(e.caret.start)
	if bytes != caretBytes {
		t.Errorf("caret at buffer position %d, expected %d", caretBytes, bytes)
	}
	// Ensure that SelectedText() does not panic no matter what the
	// editor's state is.
	_ = e.SelectedText()
}

type editMutation int

const (
	setText editMutation = iota
	moveRune
	moveLine
	movePage
	moveStart
	moveEnd
	moveCoord
	moveWord
	deleteWord
	moveLast // Mark end; never generated.
)

func TestEditorCaretConsistency(t *testing.T) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Locale:      english,
	}
	cache := text.NewCache(gofont.Collection())
	fontSize := unit.Sp(10)
	font := text.Font{}
	for _, a := range []text.Alignment{text.Start, text.Middle, text.End} {
		e := &Editor{
			Alignment: a,
		}
		e.Layout(gtx, cache, font, fontSize, nil)

		consistent := func() error {
			t.Helper()
			gotLine, gotCol := e.CaretPos()
			gotCoords := e.CaretCoords()
			// Blow away index to re-compute position from scratch.
			e.invalidate()
			want := e.closestPosition(combinedPos{runes: e.caret.start})
			wantCoords := f32.Pt(float32(want.x)/64, float32(want.y))
			if want.lineCol.Y != gotLine || want.lineCol.X != gotCol || gotCoords != wantCoords {
				return fmt.Errorf("caret (%d,%d) pos %s, want (%d,%d) pos %s",
					gotLine, gotCol, gotCoords, want.lineCol.Y, want.lineCol.X, wantCoords)
			}
			return nil
		}
		if err := consistent(); err != nil {
			t.Errorf("initial editor inconsistency (alignment %s): %v", a, err)
		}

		move := func(mutation editMutation, str string, distance int8, x, y uint16) bool {
			switch mutation {
			case setText:
				e.SetText(str)
				e.Layout(gtx, cache, font, fontSize, nil)
			case moveRune:
				e.MoveCaret(int(distance), int(distance))
			case moveLine:
				e.moveLines(int(distance), selectionClear)
			case movePage:
				e.movePages(int(distance), selectionClear)
			case moveStart:
				e.moveStart(selectionClear)
			case moveEnd:
				e.moveEnd(selectionClear)
			case moveCoord:
				e.moveCoord(image.Pt(int(x), int(y)))
			case moveWord:
				e.moveWord(int(distance), selectionClear)
			case deleteWord:
				e.deleteWord(int(distance))
			default:
				return false
			}
			if err := consistent(); err != nil {
				t.Error(err)
				return false
			}
			return true
		}
		if err := quick.Check(move, nil); err != nil {
			t.Errorf("editor inconsistency (alignment %s): %v", a, err)
		}
	}
}

func TestEditorMoveWord(t *testing.T) {
	type Test struct {
		Text  string
		Start int
		Skip  int
		Want  int
	}
	tests := []Test{
		{"", 0, 0, 0},
		{"", 0, -1, 0},
		{"", 0, 1, 0},
		{"hello", 0, -1, 0},
		{"hello", 0, 1, 5},
		{"hello world", 3, 1, 5},
		{"hello world", 3, -1, 0},
		{"hello world", 8, -1, 6},
		{"hello world", 8, 1, 11},
		{"hello    world", 3, 1, 5},
		{"hello    world", 3, 2, 14},
		{"hello    world", 8, 1, 14},
		{"hello    world", 8, -1, 0},
		{"hello brave new world", 0, 3, 15},
	}
	setup := func(t string) *Editor {
		e := new(Editor)
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Constraints: layout.Exact(image.Pt(100, 100)),
			Locale:      english,
		}
		cache := text.NewCache(gofont.Collection())
		fontSize := unit.Sp(10)
		font := text.Font{}
		e.SetText(t)
		e.Layout(gtx, cache, font, fontSize, nil)
		return e
	}
	for ii, tt := range tests {
		e := setup(tt.Text)
		e.MoveCaret(tt.Start, tt.Start)
		e.moveWord(tt.Skip, selectionClear)
		caretBytes := e.runeOffset(e.caret.start)
		if caretBytes != tt.Want {
			t.Fatalf("[%d] moveWord: bad caret position: got %d, want %d", ii, caretBytes, tt.Want)
		}
	}
}

func TestEditorInsert(t *testing.T) {
	type Test struct {
		Text      string
		Start     int
		Selection int
		Insertion string

		Result string
	}
	tests := []Test{
		// Nothing inserted
		{"", 0, 0, "", ""},
		{"", 0, -1, "", ""},
		{"", 0, 1, "", ""},
		{"", 0, -2, "", ""},
		{"", 0, 2, "", ""},
		{"world", 0, 0, "", "world"},
		{"world", 0, -1, "", "world"},
		{"world", 0, 1, "", "orld"},
		{"world", 2, 0, "", "world"},
		{"world", 2, -1, "", "wrld"},
		{"world", 2, 1, "", "wold"},
		{"world", 5, 0, "", "world"},
		{"world", 5, -1, "", "worl"},
		{"world", 5, 1, "", "world"},
		// One rune inserted
		{"", 0, 0, "_", "_"},
		{"", 0, -1, "_", "_"},
		{"", 0, 1, "_", "_"},
		{"", 0, -2, "_", "_"},
		{"", 0, 2, "_", "_"},
		{"world", 0, 0, "_", "_world"},
		{"world", 0, -1, "_", "_world"},
		{"world", 0, 1, "_", "_orld"},
		{"world", 2, 0, "_", "wo_rld"},
		{"world", 2, -1, "_", "w_rld"},
		{"world", 2, 1, "_", "wo_ld"},
		{"world", 5, 0, "_", "world_"},
		{"world", 5, -1, "_", "worl_"},
		{"world", 5, 1, "_", "world_"},
		// More runes inserted
		{"", 0, 0, "-3-", "-3-"},
		{"", 0, -1, "-3-", "-3-"},
		{"", 0, 1, "-3-", "-3-"},
		{"", 0, -2, "-3-", "-3-"},
		{"", 0, 2, "-3-", "-3-"},
		{"world", 0, 0, "-3-", "-3-world"},
		{"world", 0, -1, "-3-", "-3-world"},
		{"world", 0, 1, "-3-", "-3-orld"},
		{"world", 2, 0, "-3-", "wo-3-rld"},
		{"world", 2, -1, "-3-", "w-3-rld"},
		{"world", 2, 1, "-3-", "wo-3-ld"},
		{"world", 5, 0, "-3-", "world-3-"},
		{"world", 5, -1, "-3-", "worl-3-"},
		{"world", 5, 1, "-3-", "world-3-"},
		// Runes with length > 1 inserted
		{"", 0, 0, "éêè", "éêè"},
		{"", 0, -1, "éêè", "éêè"},
		{"", 0, 1, "éêè", "éêè"},
		{"", 0, -2, "éêè", "éêè"},
		{"", 0, 2, "éêè", "éêè"},
		{"world", 0, 0, "éêè", "éêèworld"},
		{"world", 0, -1, "éêè", "éêèworld"},
		{"world", 0, 1, "éêè", "éêèorld"},
		{"world", 2, 0, "éêè", "woéêèrld"},
		{"world", 2, -1, "éêè", "wéêèrld"},
		{"world", 2, 1, "éêè", "woéêèld"},
		{"world", 5, 0, "éêè", "worldéêè"},
		{"world", 5, -1, "éêè", "worléêè"},
		{"world", 5, 1, "éêè", "worldéêè"},
		// Runes with length > 1 deleted from selection
		{"élançé", 0, 1, "", "lançé"},
		{"élançé", 0, 1, "-3-", "-3-lançé"},
		{"élançé", 3, 2, "-3-", "éla-3-é"},
		{"élançé", 3, 3, "-3-", "éla-3-"},
		{"élançé", 3, 10, "-3-", "éla-3-"},
		{"élançé", 5, -1, "-3-", "élan-3-é"},
		{"élançé", 6, -1, "-3-", "élanç-3-"},
		{"élançé", 6, -3, "-3-", "éla-3-"},
	}
	setup := func(t string) *Editor {
		e := new(Editor)
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Constraints: layout.Exact(image.Pt(100, 100)),
			Locale:      english,
		}
		cache := text.NewCache(gofont.Collection())
		fontSize := unit.Sp(10)
		font := text.Font{}
		e.SetText(t)
		e.Layout(gtx, cache, font, fontSize, nil)
		return e
	}
	for ii, tt := range tests {
		e := setup(tt.Text)
		e.MoveCaret(tt.Start, tt.Start)
		e.MoveCaret(0, tt.Selection)
		e.Insert(tt.Insertion)
		if e.Text() != tt.Result {
			t.Fatalf("[%d] Insert: invalid result: got %q, want %q", ii, e.Text(), tt.Result)
		}
	}
}

func TestEditorDeleteWord(t *testing.T) {
	type Test struct {
		Text      string
		Start     int
		Selection int
		Delete    int

		Want   int
		Result string
	}
	tests := []Test{
		// No text selected
		{"", 0, 0, 0, 0, ""},
		{"", 0, 0, -1, 0, ""},
		{"", 0, 0, 1, 0, ""},
		{"", 0, 0, -2, 0, ""},
		{"", 0, 0, 2, 0, ""},
		{"hello", 0, 0, -1, 0, "hello"},
		{"hello", 0, 0, 1, 0, ""},

		// Document (imho) incorrect behavior w.r.t. deleting spaces following
		// words.
		{"hello world", 0, 0, 1, 0, " world"},   // Should be "world", if you ask me.
		{"hello world", 0, 0, 2, 0, "world"},    // Should be "".
		{"hello ", 0, 0, 1, 0, " "},             // Should be "".
		{"hello world", 11, 0, -1, 6, "hello "}, // Should be "hello".
		{"hello world", 11, 0, -2, 5, "hello"},  // Should be "".
		{"hello ", 6, 0, -1, 0, ""},             // Correct result.

		{"hello world", 3, 0, 1, 3, "hel world"},
		{"hello world", 3, 0, -1, 0, "lo world"},
		{"hello world", 8, 0, -1, 6, "hello rld"},
		{"hello world", 8, 0, 1, 8, "hello wo"},
		{"hello    world", 3, 0, 1, 3, "hel    world"},
		{"hello    world", 3, 0, 2, 3, "helworld"},
		{"hello    world", 8, 0, 1, 8, "hello   "},
		{"hello    world", 8, 0, -1, 5, "hello world"},
		{"hello brave new world", 0, 0, 3, 0, " new world"},
		{"helléèçàô world", 3, 0, 1, 3, "hel world"}, // unicode char with length > 1 in deleted part
		// Add selected text.
		//
		// Several permutations must be tested:
		// - select from the left or right
		// - Delete + or -
		// - abs(Delete) == 1 or > 1
		//
		// "brave |" selected; caret at |
		{"hello there brave new world", 12, 6, 1, 12, "hello there new world"}, // #16
		{"hello there brave new world", 12, 6, 2, 12, "hello there  world"},    // The two spaces after "there" are actually suboptimal, if you ask me. See also above cases.
		{"hello there brave new world", 12, 6, -1, 12, "hello there new world"},
		{"hello there brave new world", 12, 6, -2, 6, "hello new world"},
		{"hello there b®âve new world", 12, 6, 1, 12, "hello there new world"},  // unicode chars with length > 1 in selection
		{"hello there b®âve new world", 12, 6, 2, 12, "hello there  world"},     // ditto
		{"hello there b®âve new world", 12, 6, -1, 12, "hello there new world"}, // ditto
		{"hello there b®âve new world", 12, 6, -2, 6, "hello new world"},        // ditto
		// "|brave " selected
		{"hello there brave new world", 18, -6, 1, 12, "hello there new world"}, // #20
		{"hello there brave new world", 18, -6, 2, 12, "hello there  world"},    // ditto
		{"hello there brave new world", 18, -6, -1, 12, "hello there new world"},
		{"hello there brave new world", 18, -6, -2, 6, "hello new world"},
		{"hello there b®âve new world", 18, -6, 1, 12, "hello there new world"}, // unicode chars with length > 1 in selection
		// Random edge cases
		{"hello there brave new world", 12, 6, 99, 12, "hello there "},
		{"hello there brave new world", 18, -6, -99, 0, "new world"},
	}
	setup := func(t string) *Editor {
		e := new(Editor)
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Constraints: layout.Exact(image.Pt(100, 100)),
			Locale:      english,
		}
		cache := text.NewCache(gofont.Collection())
		fontSize := unit.Sp(10)
		font := text.Font{}
		e.SetText(t)
		e.Layout(gtx, cache, font, fontSize, nil)
		return e
	}
	for ii, tt := range tests {
		e := setup(tt.Text)
		e.MoveCaret(tt.Start, tt.Start)
		e.MoveCaret(0, tt.Selection)
		e.deleteWord(tt.Delete)
		caretBytes := e.runeOffset(e.caret.start)
		if caretBytes != tt.Want {
			t.Fatalf("[%d] deleteWord: bad caret position: got %d, want %d", ii, caretBytes, tt.Want)
		}
		if e.Text() != tt.Result {
			t.Fatalf("[%d] deleteWord: invalid result: got %q, want %q", ii, e.Text(), tt.Result)
		}
	}
}

func TestEditorNoLayout(t *testing.T) {
	var e Editor
	e.SetText("hi!\n")
	e.MoveCaret(1, 1)
}

// Generate generates a value of itself, for testing/quick.
func (editMutation) Generate(rand *rand.Rand, size int) reflect.Value {
	t := editMutation(rand.Intn(int(moveLast)))
	return reflect.ValueOf(t)
}

// TestSelect tests the selection code. It lays out an editor with several
// lines in it, selects some text, verifies the selection, resizes the editor
// to make it much narrower (which makes the lines in the editor reflow), and
// then verifies that the updated (col, line) positions of the selected text
// are where we expect.
func TestSelect(t *testing.T) {
	e := new(Editor)
	e.SetText(`a 2 4 6 8 a
b 2 4 6 8 b
c 2 4 6 8 c
d 2 4 6 8 d
e 2 4 6 8 e
f 2 4 6 8 f
g 2 4 6 8 g
`)

	gtx := layout.Context{
		Ops:    new(op.Ops),
		Locale: english,
	}
	cache := text.NewCache(gofont.Collection())
	font := text.Font{}
	fontSize := unit.Sp(10)

	selected := func(start, end int) string {
		// Layout once with no events; populate e.lines.
		gtx.Queue = nil
		e.Layout(gtx, cache, font, fontSize, nil)
		_ = e.Events() // throw away any events from this layout

		// Build the selection events
		startPos := e.closestPosition(combinedPos{runes: start})
		endPos := e.closestPosition(combinedPos{runes: end})
		tq := &testQueue{
			events: []event.Event{
				pointer.Event{
					Buttons:  pointer.ButtonPrimary,
					Type:     pointer.Press,
					Source:   pointer.Mouse,
					Position: f32.Pt(textWidth(e, startPos.lineCol.Y, 0, startPos.lineCol.X), textHeight(e, startPos.lineCol.Y)),
				},
				pointer.Event{
					Type:     pointer.Release,
					Source:   pointer.Mouse,
					Position: f32.Pt(textWidth(e, endPos.lineCol.Y, 0, endPos.lineCol.X), textHeight(e, endPos.lineCol.Y)),
				},
			},
		}
		gtx.Queue = tq

		e.Layout(gtx, cache, font, fontSize, nil)
		for _, evt := range e.Events() {
			switch evt.(type) {
			case SelectEvent:
				return e.SelectedText()
			}
		}
		return ""
	}

	type testCase struct {
		// input text offsets
		start, end int

		// expected selected text
		selection string
		// expected line/col positions of selection after resize
		startPos, endPos screenPos
	}

	for n, tst := range []testCase{
		{0, 1, "a", screenPos{}, screenPos{Y: 0, X: 1}},
		{0, 4, "a 2 ", screenPos{}, screenPos{Y: 0, X: 4}},
		{0, 11, "a 2 4 6 8 a", screenPos{}, screenPos{Y: 1, X: 3}},
		{6, 10, "6 8 ", screenPos{Y: 0, X: 6}, screenPos{Y: 1, X: 2}},
		{41, 66, " 6 8 d\ne 2 4 6 8 e\nf 2 4 ", screenPos{Y: 6, X: 5}, screenPos{Y: 10, X: 6}},
	} {
		gtx.Constraints = layout.Exact(image.Pt(100, 100))
		if got := selected(tst.start, tst.end); got != tst.selection {
			t.Errorf("Test %d pt1: Expected %q, got %q", n, tst.selection, got)
			continue
		}

		// Constrain the editor to roughly 6 columns wide and redraw
		gtx.Constraints = layout.Exact(image.Pt(36, 36))
		// Keep existing selection
		gtx.Queue = nil
		e.Layout(gtx, cache, font, fontSize, nil)

		caretStart := e.closestPosition(combinedPos{runes: e.caret.start})
		caretEnd := e.closestPosition(combinedPos{runes: e.caret.end})
		if caretEnd.lineCol != tst.startPos || caretStart.lineCol != tst.endPos {
			t.Errorf("Test %d pt2: Expected %#v, %#v; got %#v, %#v",
				n,
				caretEnd.lineCol, caretStart.lineCol,
				tst.startPos, tst.endPos)
			continue
		}
	}
}

// Verify that an existing selection is dismissed when you press arrow keys.
func TestSelectMove(t *testing.T) {
	e := new(Editor)
	e.SetText(`0123456789`)

	gtx := layout.Context{
		Ops:    new(op.Ops),
		Locale: english,
	}
	cache := text.NewCache(gofont.Collection())
	font := text.Font{}
	fontSize := unit.Sp(10)

	// Layout once to populate e.lines and get focus.
	gtx.Queue = newQueue(key.FocusEvent{Focus: true})
	e.Layout(gtx, cache, font, fontSize, nil)

	testKey := func(keyName string) {
		// Select 345
		e.SetCaret(3, 6)
		if expected, got := "345", e.SelectedText(); expected != got {
			t.Errorf("KeyName %s, expected %q, got %q", keyName, expected, got)
		}

		// Press the key
		gtx.Queue = newQueue(key.Event{State: key.Press, Name: keyName})
		e.Layout(gtx, cache, font, fontSize, nil)

		if expected, got := "", e.SelectedText(); expected != got {
			t.Errorf("KeyName %s, expected %q, got %q", keyName, expected, got)
		}
	}

	testKey(key.NameLeftArrow)
	testKey(key.NameRightArrow)
	testKey(key.NameUpArrow)
	testKey(key.NameDownArrow)
}

func TestEditor_Read(t *testing.T) {
	s := "hello world"
	buf := make([]byte, len(s))
	e := new(Editor)
	e.SetText(s)

	_, err := e.Seek(0, io.SeekStart)
	if err != nil {
		t.Error(err)
	}
	n, err := io.ReadFull(e, buf)
	if err != nil {
		t.Error(err)
	}
	if got, want := n, len(s); got != want {
		t.Errorf("got %d; want %d", got, want)
	}
	if got, want := string(buf), s; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestEditor_WriteTo(t *testing.T) {
	s := "hello world"
	var buf bytes.Buffer
	e := new(Editor)
	e.SetText(s)

	n, err := io.Copy(&buf, e)
	if err != nil {
		t.Error(err)
	}
	if got, want := int(n), len(s); got != want {
		t.Errorf("got %d; want %d", got, want)
	}
	if got, want := buf.String(), s; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestEditor_MaxLen(t *testing.T) {
	e := new(Editor)

	e.MaxLen = 8
	e.SetText("123456789")
	if got, want := e.Text(), "12345678"; got != want {
		t.Errorf("editor failed to cap SetText")
	}

	e.SetText("2345678")
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Queue: newQueue(
			key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: "1234"},
			key.SelectionEvent{Start: 4, End: 4},
		),
	}
	cache := text.NewCache(gofont.Collection())
	fontSize := unit.Sp(10)
	font := text.Font{}
	e.Layout(gtx, cache, font, fontSize, nil)

	if got, want := e.Text(), "12345678"; got != want {
		t.Errorf("editor failed to cap EditEvent")
	}
	if start, end := e.Selection(); start != 1 || end != 1 {
		t.Errorf("editor failed to adjust SelectionEvent")
	}
}

func textWidth(e *Editor, lineNum, colStart, colEnd int) float32 {
	var w fixed.Int26_6
	glyphs := e.lines[lineNum].Layout.Glyphs
	if colEnd > len(glyphs) {
		colEnd = len(glyphs)
	}
	for _, glyph := range glyphs[colStart:colEnd] {
		w += glyph.XAdvance
	}
	return float32(w.Floor())
}

func textHeight(e *Editor, lineNum int) float32 {
	var h int
	var prevDesc fixed.Int26_6
	for _, line := range e.lines[0:lineNum] {
		h += (line.Ascent + prevDesc).Ceil()
		prevDesc = line.Descent
	}
	return float32(h + prevDesc.Ceil() + 1)
}

type testQueue struct {
	events []event.Event
}

func newQueue(e ...event.Event) *testQueue {
	return &testQueue{events: e}
}

func (q *testQueue) Events(_ event.Tag) []event.Event {
	return q.events
}

func printLines(e *Editor) {
	for _, line := range e.lines {
		start := e.runeOffset(line.Layout.Runes.Offset)
		buf := make([]byte, 0, 4*line.Layout.Runes.Count)
		e.Seek(int64(start), 0)
		n, _ := e.Read(buf)
		buf = buf[:n]
		asStr := string([]rune(string(buf))[:line.Layout.Runes.Count])
		text := strings.TrimSuffix(asStr, "\n")
		fmt.Printf("%d: %s\n", n, text)
	}
}
