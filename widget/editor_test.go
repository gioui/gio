// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"time"
	"unicode/utf8"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"eliasnaur.com/font/roboto/robotoregular"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
)

var english = system.Locale{
	Language:  "EN",
	Direction: system.LTR,
}

// TestEditorHistory ensures that undo and redo behave correctly.
func TestEditorHistory(t *testing.T) {
	e := new(Editor)
	// Insert some multi-byte unicode text.
	e.SetText("안П你 hello 안П你")
	assertContents(t, e, "안П你 hello 안П你", 0, 0)
	// Overwrite all of the text with the empty string.
	e.SetCaret(0, len([]rune("안П你 hello 안П你")))
	e.Insert("")
	assertContents(t, e, "", 0, 0)
	// Ensure that undoing the overwrite succeeds.
	e.undo()
	assertContents(t, e, "안П你 hello 안П你", 13, 0)
	// Ensure that redoing the overwrite succeeds.
	e.redo()
	assertContents(t, e, "", 0, 0)
	// Insert some smaller text.
	e.Insert("안П你 hello")
	assertContents(t, e, "안П你 hello", 9, 9)
	// Replace a region in the middle of the text.
	e.SetCaret(1, 5)
	e.Insert("П")
	assertContents(t, e, "안Пello", 2, 2)
	// Replace a second region in the middle.
	e.SetCaret(3, 4)
	e.Insert("П")
	assertContents(t, e, "안ПeПlo", 4, 4)
	// Ensure both operations undo successfully.
	e.undo()
	assertContents(t, e, "안Пello", 4, 3)
	e.undo()
	assertContents(t, e, "안П你 hello", 5, 1)
	// Make a new modification.
	e.Insert("Something New")
	// Ensure that redo history is discarded now that
	// we've diverged from the linear editing history.
	// This redo() call should do nothing.
	text := e.Text()
	start, end := e.Selection()
	e.redo()
	assertContents(t, e, text, start, end)
}

func assertContents(t *testing.T, e *Editor, contents string, selectionStart, selectionEnd int) {
	t.Helper()
	actualContents := e.Text()
	if actualContents != contents {
		t.Errorf("expected editor to contain %s, got %s", contents, actualContents)
	}
	actualStart, actualEnd := e.Selection()
	if actualStart != selectionStart {
		t.Errorf("expected selection start to be %d, got %d", selectionStart, actualStart)
	}
	if actualEnd != selectionEnd {
		t.Errorf("expected selection end to be %d, got %d", selectionEnd, actualEnd)
	}
}

// TestEditorReadOnly ensures that mouse and keyboard interactions with readonly
// editors do nothing but manipulate the text selection.
func TestEditorReadOnly(t *testing.T) {
	r := new(input.Router)
	gtx := layout.Context{
		Ops: new(op.Ops),
		Constraints: layout.Constraints{
			Max: image.Pt(100, 100),
		},
		Locale: english,
		Source: r.Source(),
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fontSize := unit.Sp(10)
	font := font.Font{}
	e := new(Editor)
	e.ReadOnly = true
	e.SetText("The quick brown fox jumps over the lazy dog. We just need a few lines of text in the editor so that it can adequately test a few different modes of selection. The quick brown fox jumps over the lazy dog. We just need a few lines of text in the editor so that it can adequately test a few different modes of selection.")
	cStart, cEnd := e.Selection()
	if cStart != cEnd {
		t.Errorf("unexpected initial caret positions")
	}
	gtx.Execute(key.FocusCmd{Tag: e})
	layoutEditor := func() layout.Dimensions {
		return e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	}
	layoutEditor()
	r.Frame(gtx.Ops)
	gtx.Ops.Reset()
	layoutEditor()
	r.Frame(gtx.Ops)
	gtx.Ops.Reset()
	layoutEditor()
	r.Frame(gtx.Ops)

	// Select everything.
	gtx.Ops.Reset()
	r.Queue(key.Event{Name: "A", Modifiers: key.ModShortcut})
	layoutEditor()
	textContent := e.Text()
	cStart2, cEnd2 := e.Selection()
	if cStart2 > cEnd2 {
		cStart2, cEnd2 = cEnd2, cStart2
	}
	if cEnd2 != e.Len() {
		t.Errorf("expected selection to contain %d runes, got %d", e.Len(), cEnd2)
	}
	if cStart2 != 0 {
		t.Errorf("expected selection to start at rune 0, got %d", cStart2)
	}

	// Type some new characters.
	gtx.Ops.Reset()
	r.Queue(key.EditEvent{Range: key.Range{Start: cStart2, End: cEnd2}, Text: "something else"})
	e.Update(gtx)
	textContent2 := e.Text()
	if textContent2 != textContent {
		t.Errorf("readonly editor modified by key.EditEvent")
	}

	// Try to delete selection.
	gtx.Ops.Reset()
	r.Queue(key.Event{Name: key.NameDeleteBackward})
	dims := layoutEditor()
	textContent2 = e.Text()
	if textContent2 != textContent {
		t.Errorf("readonly editor modified by delete key.Event")
	}

	// Click and drag from the middle of the first line
	// to the center.
	gtx.Ops.Reset()
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Buttons:  pointer.ButtonPrimary,
			Position: f32.Pt(float32(dims.Size.X)*.5, 5),
		},
		pointer.Event{
			Kind:     pointer.Move,
			Buttons:  pointer.ButtonPrimary,
			Position: layout.FPt(dims.Size).Mul(.5),
		},
		pointer.Event{
			Kind:     pointer.Release,
			Buttons:  pointer.ButtonPrimary,
			Position: layout.FPt(dims.Size).Mul(.5),
		},
	)
	e.Update(gtx)
	cStart3, cEnd3 := e.Selection()
	if cStart3 == cStart2 || cEnd3 == cEnd2 {
		t.Errorf("expected mouse interaction to change selection.")
	}
}

func TestEditorConfigurations(t *testing.T) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(300, 300)),
		Locale:      english,
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fontSize := unit.Sp(10)
	font := font.Font{}
	sentence := "\n\n\n\n\n\n\n\n\n\n\n\nthe quick brown fox jumps over the lazy dog"
	runes := len([]rune(sentence))

	// Ensure that both ends of the text are reachable in all permutations
	// of settings that influence layout.
	for _, singleLine := range []bool{true, false} {
		for _, alignment := range []text.Alignment{text.Start, text.Middle, text.End} {
			for _, zeroMin := range []bool{true, false} {
				t.Run(fmt.Sprintf("SingleLine: %v Alignment: %v ZeroMinConstraint: %v", singleLine, alignment, zeroMin), func(t *testing.T) {
					defer func() {
						if err := recover(); err != nil {
							t.Error(err)
						}
					}()
					if zeroMin {
						gtx.Constraints.Min = image.Point{}
					} else {
						gtx.Constraints.Min = gtx.Constraints.Max
					}
					e := new(Editor)
					e.SingleLine = singleLine
					e.Alignment = alignment
					e.SetText(sentence)
					e.SetCaret(0, 0)
					dims := e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
					if dims.Size.X < gtx.Constraints.Min.X || dims.Size.Y < gtx.Constraints.Min.Y {
						t.Errorf("expected min size %#+v, got %#+v", gtx.Constraints.Min, dims.Size)
					}
					coords := e.CaretCoords()
					if halfway := float32(gtx.Constraints.Min.X) * .5; !singleLine && alignment == text.Middle && !zeroMin && coords.X != halfway {
						t.Errorf("expected caret X to be %f, got %f", halfway, coords.X)
					}
					e.SetCaret(runes, runes)
					e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
					coords = e.CaretCoords()
					if int(coords.X) > gtx.Constraints.Max.X || int(coords.Y) > gtx.Constraints.Max.Y {
						t.Errorf("caret coordinates %v exceed constraints %v", coords, gtx.Constraints.Max)
					}
				})
			}
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
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fontSize := unit.Sp(10)
	font := font.Font{}

	// Regression test for bad in-cluster rune offset math.
	e.SetText("æbc")
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	e.text.MoveLineEnd(selectionClear)
	assertCaret(t, e, 0, 3, len("æbc"))

	textSample := "æbc\naøå••"
	e.SetCaret(0, 0) // shouldn't panic
	assertCaret(t, e, 0, 0, 0)
	e.SetText(textSample)
	if got, exp := e.Len(), utf8.RuneCountInString(e.Text()); got != exp {
		t.Errorf("got length %d, expected %d", got, exp)
	}
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	assertCaret(t, e, 0, 0, 0)
	e.text.MoveLineEnd(selectionClear)
	assertCaret(t, e, 0, 3, len("æbc"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 0, len("æbc\n"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 3, len("æbc"))
	e.text.MoveLines(+1, selectionClear)
	assertCaret(t, e, 1, 4, len("æbc\naøå•"))
	e.text.MoveLineEnd(selectionClear)
	assertCaret(t, e, 1, 5, len("æbc\naøå••"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 5, len("æbc\naøå••"))
	e.text.MoveLines(3, selectionClear)

	e.SetCaret(0, 0)
	assertCaret(t, e, 0, 0, 0)
	e.SetCaret(utf8.RuneCountInString("æ"), utf8.RuneCountInString("æ"))
	assertCaret(t, e, 0, 1, 2)
	e.SetCaret(utf8.RuneCountInString("æbc\naøå•"), utf8.RuneCountInString("æbc\naøå•"))
	assertCaret(t, e, 1, 4, len("æbc\naøå•"))

	// Ensure that password masking does not affect caret behavior
	e.MoveCaret(-3, -3)
	assertCaret(t, e, 1, 1, len("æbc\na"))
	e.text.Mask = '*'
	e.Update(gtx)
	assertCaret(t, e, 1, 1, len("æbc\na"))
	e.MoveCaret(-3, -3)
	assertCaret(t, e, 0, 2, len("æb"))
	// Test that moveLine applies x offsets from previous moves.
	e.SetText("long line\nshort")
	e.SetCaret(0, 0)
	e.text.MoveLineEnd(selectionClear)
	e.text.MoveLines(+1, selectionClear)
	e.text.MoveLines(-1, selectionClear)
	assertCaret(t, e, 0, utf8.RuneCountInString("long line"), len("long line"))
}

var arabic = system.Locale{
	Language:  "AR",
	Direction: system.RTL,
}

var arabicCollection = func() []font.FontFace {
	parsed, _ := opentype.Parse(nsareg.TTF)
	return []font.FontFace{{Font: font.Font{}, Face: parsed}}
}()

func TestEditorRTL(t *testing.T) {
	e := new(Editor)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Locale:      arabic,
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(arabicCollection))
	fontSize := unit.Sp(10)
	font := font.Font{}

	e.SetCaret(0, 0) // shouldn't panic
	assertCaret(t, e, 0, 0, 0)

	// Set the text to a single RTL word. The caret should start at 0 column
	// zero, but this is the first column on the right.
	e.SetText("الحب")
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	assertCaret(t, e, 0, 0, 0)
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 0, 1, len("ا"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 0, 2, len("ال"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 0, 3, len("الح"))
	// Move to the "end" of the line. This moves to the left edge of the line.
	e.text.MoveLineEnd(selectionClear)
	assertCaret(t, e, 0, 4, len("الحب"))

	sentence := "الحب سماء لا\nتمط غير الأحلام"
	e.SetText(sentence)
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	assertCaret(t, e, 0, 0, 0)
	e.text.MoveLineEnd(selectionClear)
	assertCaret(t, e, 0, 12, len("الحب سماء لا"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 0, len("الحب سماء لا\n"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 1, len("الحب سماء لا\nت"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 1, 0, len("الحب سماء لا\n"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 12, len("الحب سماء لا"))
	e.text.MoveLines(+1, selectionClear)
	assertCaret(t, e, 1, 14, len("الحب سماء لا\nتمط غير الأحلا"))
	e.text.MoveLineEnd(selectionClear)
	assertCaret(t, e, 1, 15, len("الحب سماء لا\nتمط غير الأحلام"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 15, len("الحب سماء لا\nتمط غير الأحلام"))
	e.text.MoveLines(3, selectionClear)
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
	e.WrapPolicy = text.WrapWords
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Locale:      english,
	}
	face, err := opentype.Parse(robotoregular.TTF)
	if err != nil {
		t.Skipf("failed parsing test font: %v", err)
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection([]font.FontFace{
		{
			Font: font.Font{
				Typeface: "Roboto",
			},
			Face: face,
		},
	}))
	fontSize := unit.Sp(10)
	font := font.Font{}

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
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	e.text.MoveLineEnd(selectionClear)
	assertCaret(t, e, 0, 2, len("fl"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 1, len("f"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 0, 0)
	e.MoveCaret(+2, +2)
	assertCaret(t, e, 0, 2, len("fl"))
	e.SetText("flaffl•ffi\n•fflfi") // 3 ligatures on line 0, 2 on line 1
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	assertCaret(t, e, 0, 0, 0)
	e.text.MoveLineEnd(selectionClear)
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
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	// Ensure that all runes in the final cluster of a line are properly
	// decoded when moving to the end of the line. This is a regression test.
	e.text.MoveLineEnd(selectionClear)
	// The first line was broken by line wrapping, not a newline character, and has a trailing
	// whitespace. However, we should never be able to reach the "other side" of such a trailing
	// whitespace glyph.
	assertCaret(t, e, 0, 13, len("fflffl fflffl"))
	e.text.MoveLines(1, selectionClear)
	assertCaret(t, e, 1, 13, len("fflffl fflffl fflffl fflffl"))
	e.text.MoveLines(-1, selectionClear)
	assertCaret(t, e, 0, 13, len("fflffl fflffl"))

	// Absurdly narrow constraints to force each ligature onto its own line.
	gtx.Constraints = layout.Exact(image.Pt(10, 10))
	e.SetText("ffl ffl") // Two ligatures on separate lines.
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	assertCaret(t, e, 0, 0, 0)
	e.MoveCaret(1, 1) // Move the caret into the first ligature.
	assertCaret(t, e, 0, 1, len("f"))
	e.MoveCaret(4, 4) // Move the caret several positions.
	assertCaret(t, e, 1, 1, len("ffl f"))
}

func TestEditorDimensions(t *testing.T) {
	e := new(Editor)
	r := new(input.Router)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Constraints{Max: image.Pt(100, 100)},
		Source:      r.Source(),
		Locale:      english,
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fontSize := unit.Sp(10)
	font := font.Font{}
	gtx.Execute(key.FocusCmd{Tag: e})
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)
	r.Queue(key.EditEvent{Text: "A"})
	dims := e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	if dims.Size.X < 5 {
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
	caretBytes := e.text.runeOffset(e.text.caret.start)
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
	moveTextStart
	moveTextEnd
	moveLineStart
	moveLineEnd
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
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fontSize := unit.Sp(10)
	font := font.Font{}
	for _, a := range []text.Alignment{text.Start, text.Middle, text.End} {
		e := &Editor{}
		e.Alignment = a
		e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})

		consistent := func() error {
			t.Helper()
			gotLine, gotCol := e.CaretPos()
			gotCoords := e.CaretCoords()
			// Blow away index to re-compute position from scratch.
			e.text.invalidate()
			want := e.text.closestToRune(e.text.caret.start)
			wantCoords := f32.Pt(float32(want.x)/64, float32(want.y))
			if want.lineCol.line != gotLine || int(want.lineCol.col) != gotCol || gotCoords != wantCoords {
				return fmt.Errorf("caret (%d,%d) pos %s, want (%d,%d) pos %s",
					gotLine, gotCol, gotCoords, want.lineCol.line, want.lineCol.col, wantCoords)
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
				e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
			case moveRune:
				e.MoveCaret(int(distance), int(distance))
			case moveLine:
				e.text.MoveLines(int(distance), selectionClear)
			case movePage:
				e.text.MovePages(int(distance), selectionClear)
			case moveLineStart:
				e.text.MoveLineStart(selectionClear)
			case moveLineEnd:
				e.text.MoveLineEnd(selectionClear)
			case moveTextStart:
				e.text.MoveTextStart(selectionClear)
			case moveTextEnd:
				e.text.MoveTextEnd(selectionClear)
			case moveCoord:
				e.text.MoveCoord(image.Pt(int(x), int(y)))
			case moveWord:
				e.text.MoveWord(int(distance), selectionClear)
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
		e.SetText(t)
		e.Update(gtx)
		return e
	}
	for ii, tt := range tests {
		e := setup(tt.Text)
		e.MoveCaret(tt.Start, tt.Start)
		e.text.MoveWord(tt.Skip, selectionClear)
		caretBytes := e.text.runeOffset(e.text.caret.start)
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
		e.SetText(t)
		e.Update(gtx)
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
		e.SetText(t)
		e.Update(gtx)
		return e
	}
	for ii, tt := range tests {
		e := setup(tt.Text)
		e.MoveCaret(tt.Start, tt.Start)
		e.MoveCaret(0, tt.Selection)
		e.deleteWord(tt.Delete)
		caretBytes := e.text.runeOffset(e.text.caret.start)
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

// TestEditorSelect tests the selection code. It lays out an editor with several
// lines in it, selects some text, verifies the selection, resizes the editor
// to make it much narrower (which makes the lines in the editor reflow), and
// then verifies that the updated (col, line) positions of the selected text
// are where we expect.
func TestEditorSelectReflow(t *testing.T) {
	e := new(Editor)
	e.SetText(`a 2 4 6 8 a
b 2 4 6 8 b
c 2 4 6 8 c
d 2 4 6 8 d
e 2 4 6 8 e
f 2 4 6 8 f
g 2 4 6 8 g
`)

	r := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Locale: english,
		Source: r.Source(),
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	font := font.Font{}
	fontSize := unit.Sp(10)

	var tim time.Duration
	selected := func(start, end int) string {
		gtx.Execute(key.FocusCmd{Tag: e})
		// Layout once with no events; populate e.lines.
		e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})

		r.Frame(gtx.Ops)
		gtx.Source = r.Source()
		// Build the selection events
		startPos := e.text.closestToRune(start)
		endPos := e.text.closestToRune(end)
		r.Queue(
			pointer.Event{
				Buttons:  pointer.ButtonPrimary,
				Kind:     pointer.Press,
				Source:   pointer.Mouse,
				Time:     tim,
				Position: f32.Pt(textWidth(e, startPos.lineCol.line, 0, startPos.lineCol.col), textBaseline(e, startPos.lineCol.line)),
			},
			pointer.Event{
				Kind:     pointer.Release,
				Source:   pointer.Mouse,
				Time:     tim,
				Position: f32.Pt(textWidth(e, endPos.lineCol.line, 0, endPos.lineCol.col), textBaseline(e, endPos.lineCol.line)),
			},
		)
		tim += time.Second // Avoid multi-clicks.

		for {
			_, ok := e.Update(gtx) // throw away any events from this layout
			if !ok {
				break
			}
		}
		return e.SelectedText()
	}
	type screenPos image.Point
	logicalPosMatch := func(t *testing.T, n int, label string, expected screenPos, actual combinedPos) {
		t.Helper()
		if actual.lineCol.line != expected.Y || actual.lineCol.col != expected.X {
			t.Errorf("Test %d: Expected %s %#v; got %#v",
				n, label,
				expected, actual)
		}
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
		gtx = gtx.Disabled()
		e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})

		caretStart := e.text.closestToRune(e.text.caret.start)
		caretEnd := e.text.closestToRune(e.text.caret.end)
		logicalPosMatch(t, n, "start", tst.startPos, caretEnd)
		logicalPosMatch(t, n, "end", tst.endPos, caretStart)
	}
}

func TestEditorSelectShortcuts(t *testing.T) {
	tFont := font.Font{}
	tFontSize := unit.Sp(10)
	tShaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	var tEditor = &Editor{
		SingleLine: false,
		ReadOnly:   true,
	}
	lines := "abc abc abc\ndef def def\nghi ghi ghi"
	tEditor.SetText(lines)
	type testCase struct {
		// Initial text selection.
		startPos, endPos int
		// Keyboard shortcut to execute.
		keyEvent key.Event
		// Expected text selection.
		selection string
	}

	pos1, pos2 := 14, 21
	for n, tst := range []testCase{
		{pos1, pos2, key.Event{Name: "A", Modifiers: key.ModShortcut}, lines},
		{pos2, pos1, key.Event{Name: "A", Modifiers: key.ModShortcut}, lines},
		{pos1, pos2, key.Event{Name: key.NameHome, Modifiers: key.ModShift}, "def def d"},
		{pos1, pos2, key.Event{Name: key.NameEnd, Modifiers: key.ModShift}, "ef"},
		{pos2, pos1, key.Event{Name: key.NameHome, Modifiers: key.ModShift}, "de"},
		{pos2, pos1, key.Event{Name: key.NameEnd, Modifiers: key.ModShift}, "f def def"},
		{pos1, pos2, key.Event{Name: key.NameHome, Modifiers: key.ModShortcut | key.ModShift}, "abc abc abc\ndef def d"},
		{pos1, pos2, key.Event{Name: key.NameEnd, Modifiers: key.ModShortcut | key.ModShift}, "ef\nghi ghi ghi"},
		{pos2, pos1, key.Event{Name: key.NameHome, Modifiers: key.ModShortcut | key.ModShift}, "abc abc abc\nde"},
		{pos2, pos1, key.Event{Name: key.NameEnd, Modifiers: key.ModShortcut | key.ModShift}, "f def def\nghi ghi ghi"},
	} {
		tRouter := new(input.Router)
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Locale:      english,
			Constraints: layout.Exact(image.Pt(100, 100)),
			Source:      tRouter.Source(),
		}
		gtx.Execute(key.FocusCmd{Tag: tEditor})
		tEditor.Layout(gtx, tShaper, tFont, tFontSize, op.CallOp{}, op.CallOp{})

		tEditor.SetCaret(tst.startPos, tst.endPos)
		if cStart, cEnd := tEditor.Selection(); cStart != tst.startPos || cEnd != tst.endPos {
			t.Errorf("TestEditorSelect %d: initial selection", n)
		}
		tRouter.Queue(tst.keyEvent)
		tEditor.Update(gtx)
		if got := tEditor.SelectedText(); got != tst.selection {
			t.Errorf("TestEditorSelect %d: Expected %q, got %q", n, tst.selection, got)
		}
	}
}

// Verify that an existing selection is dismissed when you press arrow keys.
func TestSelectMove(t *testing.T) {
	e := new(Editor)
	e.SetText(`0123456789`)

	r := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Locale: english,
		Source: r.Source(),
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	font := font.Font{}
	fontSize := unit.Sp(10)

	// Layout once to populate e.lines and get focus.
	gtx.Execute(key.FocusCmd{Tag: e})
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)
	// Set up selecton so the Editor key handler filters for all 4 directional keys.
	e.SetCaret(3, 6)
	gtx.Ops.Reset()
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)
	gtx.Ops.Reset()
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)

	for _, keyName := range []key.Name{key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow} {
		// Select 345
		e.SetCaret(3, 6)
		if expected, got := "345", e.SelectedText(); expected != got {
			t.Errorf("KeyName %s, expected %q, got %q", keyName, expected, got)
		}

		// Press the key
		r.Queue(key.Event{State: key.Press, Name: keyName})
		gtx.Ops.Reset()
		e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
		r.Frame(gtx.Ops)

		if expected, got := "", e.SelectedText(); expected != got {
			t.Errorf("KeyName %s, expected %q, got %q", keyName, expected, got)
		}
	}
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
	r := new(input.Router)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Source:      r.Source(),
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fontSize := unit.Sp(10)
	font := font.Font{}
	gtx.Execute(key.FocusCmd{Tag: e})
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)
	r.Queue(
		key.EditEvent{Range: key.Range{Start: 0, End: 2}, Text: "1234"},
		key.SelectionEvent{Start: 4, End: 4},
	)
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})

	if got, want := e.Text(), "12345678"; got != want {
		t.Errorf("editor failed to cap EditEvent")
	}
	if start, end := e.Selection(); start != 3 || end != 3 {
		t.Errorf("editor failed to adjust SelectionEvent")
	}
}

func TestEditor_Filter(t *testing.T) {
	e := new(Editor)

	e.Filter = "123456789"
	e.SetText("abcde1234")
	if got, want := e.Text(), "1234"; got != want {
		t.Errorf("editor failed to filter SetText")
	}

	e.SetText("2345678")
	r := new(input.Router)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Source:      r.Source(),
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fontSize := unit.Sp(10)
	font := font.Font{}
	gtx.Execute(key.FocusCmd{Tag: e})
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)
	r.Queue(
		key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: "ab1"},
		key.SelectionEvent{Start: 4, End: 4},
	)
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})

	if got, want := e.Text(), "12345678"; got != want {
		t.Errorf("editor failed to filter EditEvent")
	}
	if start, end := e.Selection(); start != 2 || end != 2 {
		t.Errorf("editor failed to adjust SelectionEvent")
	}
}

func TestEditor_Submit(t *testing.T) {
	e := new(Editor)
	e.Submit = true

	r := new(input.Router)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Source:      r.Source(),
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fontSize := unit.Sp(10)
	font := font.Font{}
	gtx.Execute(key.FocusCmd{Tag: e})
	e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)
	r.Queue(
		key.EditEvent{Range: key.Range{Start: 0, End: 0}, Text: "ab1\n"},
	)

	got := []EditorEvent{}
	for {
		ev, ok := e.Update(gtx)
		if !ok {
			break
		}
		got = append(got, ev)
	}
	if got, want := e.Text(), "ab1"; got != want {
		t.Errorf("editor failed to filter newline")
	}
	want := []EditorEvent{
		ChangeEvent{},
		SubmitEvent{Text: e.Text()},
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("editor failed to register submit")
	}
}

func TestNoFilterAllocs(t *testing.T) {
	b := testing.Benchmark(func(b *testing.B) {
		r := new(input.Router)
		e := new(Editor)
		gtx := layout.Context{
			Ops: new(op.Ops),
			Constraints: layout.Constraints{
				Max: image.Pt(100, 100),
			},
			Locale: english,
			Source: r.Source(),
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			e.Update(gtx)
		}
	})
	if allocs := b.AllocsPerOp(); allocs != 0 {
		t.Fatalf("expected 0 AllocsPerOp, got %d", allocs)
	}
}

// textWidth is a text helper for building simple selection events.
// It assumes single-run lines, which isn't safe with non-test text
// data.
func textWidth(e *Editor, lineNum, colStart, colEnd int) float32 {
	start := e.text.closestToLineCol(lineNum, colStart)
	end := e.text.closestToLineCol(lineNum, colEnd)
	delta := start.x - end.x
	if delta < 0 {
		delta = -delta
	}
	return float32(delta.Round())
}

// testBaseline returns the y coordinate of the baseline for the
// given line number.
func textBaseline(e *Editor, lineNum int) float32 {
	start := e.text.closestToLineCol(lineNum, 0)
	return float32(start.y)
}
