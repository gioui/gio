// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"fmt"
	"image"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
	"unicode"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

func TestEditor(t *testing.T) {
	e := new(Editor)
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
	}
	cache := text.NewCache(gofont.Collection())
	fontSize := unit.Px(10)
	font := text.Font{}

	e.SetText("æbc\naøå•")
	e.Layout(gtx, cache, font, fontSize)
	assertCaret(t, e, 0, 0, 0)
	e.moveEnd(selectionClear)
	assertCaret(t, e, 0, 3, len("æbc"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 0, len("æbc\n"))
	e.MoveCaret(-1, -1)
	assertCaret(t, e, 0, 3, len("æbc"))
	e.moveLines(+1, +1)
	assertCaret(t, e, 1, 3, len("æbc\naøå"))
	e.moveEnd(selectionClear)
	assertCaret(t, e, 1, 4, len("æbc\naøå•"))
	e.MoveCaret(+1, +1)
	assertCaret(t, e, 1, 4, len("æbc\naøå•"))

	e.SetCaret(0, 0)
	assertCaret(t, e, 0, 0, 0)
	e.SetCaret(len("æ"), len("æ"))
	assertCaret(t, e, 0, 1, 2)
	e.SetCaret(len("æbc\naøå•"), len("æbc\naøå•"))
	assertCaret(t, e, 1, 4, len("æbc\naøå•"))

	// Ensure that password masking does not affect caret behavior
	e.MoveCaret(-3, -3)
	assertCaret(t, e, 1, 1, len("æbc\na"))
	e.Mask = '*'
	e.Layout(gtx, cache, font, fontSize)
	assertCaret(t, e, 1, 1, len("æbc\na"))
	e.MoveCaret(-3, -3)
	assertCaret(t, e, 0, 2, len("æb"))
	e.Mask = '\U0001F92B'
	e.Layout(gtx, cache, font, fontSize)
	e.moveEnd(selectionClear)
	assertCaret(t, e, 0, 3, len("æbc"))

	// When a password mask is applied, it should replace all visible glyphs
	for i, line := range e.lines {
		for j, r := range line.Layout.Text {
			if r != e.Mask && !unicode.IsSpace(r) {
				t.Errorf("glyph at (%d, %d) is unmasked rune %d", i, j, r)
			}
		}
	}
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
	}
	cache := text.NewCache(gofont.Collection())
	fontSize := unit.Px(10)
	font := text.Font{}
	dims := e.Layout(gtx, cache, font, fontSize)
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
	if bytes != e.caret.start.ofs {
		t.Errorf("caret at buffer position %d, expected %d", e.caret.start.ofs, bytes)
	}
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
	}
	cache := text.NewCache(gofont.Collection())
	fontSize := unit.Px(10)
	font := text.Font{}
	for _, a := range []text.Alignment{text.Start, text.Middle, text.End} {
		e := &Editor{
			Alignment: a,
		}
		e.Layout(gtx, cache, font, fontSize)

		consistent := func() error {
			t.Helper()
			gotLine, gotCol := e.CaretPos()
			gotCoords := e.CaretCoords()
			want, _ := e.offsetToScreenPos(e.caret.start.ofs)
			wantCoords := f32.Pt(float32(want.x)/64, float32(want.y))
			if want.lineCol.Y == gotLine && want.lineCol.X == gotCol && gotCoords == wantCoords {
				return nil
			}
			return fmt.Errorf("caret (%d,%d) pos %s, want (%d,%d) pos %s",
				gotLine, gotCol, gotCoords, want.lineCol.Y, want.lineCol.X, wantCoords)
		}
		if err := consistent(); err != nil {
			t.Errorf("initial editor inconsistency (alignment %s): %v", a, err)
		}

		move := func(mutation editMutation, str string, distance int8, x, y uint16) bool {
			switch mutation {
			case setText:
				e.SetText(str)
				e.Layout(gtx, cache, font, fontSize)
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
		}
		cache := text.NewCache(gofont.Collection())
		fontSize := unit.Px(10)
		font := text.Font{}
		e.SetText(t)
		e.Layout(gtx, cache, font, fontSize)
		return e
	}
	for ii, tt := range tests {
		e := setup(tt.Text)
		e.MoveCaret(tt.Start, tt.Start)
		e.moveWord(tt.Skip, selectionClear)
		if e.caret.start.ofs != tt.Want {
			t.Fatalf("[%d] moveWord: bad caret position: got %d, want %d", ii, e.caret.start.ofs, tt.Want)
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
		// "|brave " selected
		{"hello there brave new world", 18, -6, 1, 12, "hello there new world"}, // #20
		{"hello there brave new world", 18, -6, 2, 12, "hello there  world"},    // ditto
		{"hello there brave new world", 18, -6, -1, 12, "hello there new world"},
		{"hello there brave new world", 18, -6, -2, 6, "hello new world"},
		// Random edge cases
		{"hello there brave new world", 12, 6, 99, 12, "hello there "},
		{"hello there brave new world", 18, -6, -99, 0, "new world"},
	}
	setup := func(t string) *Editor {
		e := new(Editor)
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Constraints: layout.Exact(image.Pt(100, 100)),
		}
		cache := text.NewCache(gofont.Collection())
		fontSize := unit.Px(10)
		font := text.Font{}
		e.SetText(t)
		e.Layout(gtx, cache, font, fontSize)
		return e
	}
	for ii, tt := range tests {
		e := setup(tt.Text)
		e.MoveCaret(tt.Start, tt.Start)
		e.MoveCaret(0, tt.Selection)
		e.deleteWord(tt.Delete)
		if e.caret.start.ofs != tt.Want {
			t.Fatalf("[%d] deleteWord: bad caret position: got %d, want %d", ii, e.caret.start.ofs, tt.Want)
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
	e.SetText(`a123456789a
b123456789b
c123456789c
d123456789d
e123456789e
f123456789f
g123456789g
`)

	gtx := layout.Context{Ops: new(op.Ops)}
	cache := text.NewCache(gofont.Collection())
	font := text.Font{}
	fontSize := unit.Px(10)

	selected := func(start, end int) string {
		// Layout once with no events; populate e.lines.
		gtx.Queue = nil
		e.Layout(gtx, cache, font, fontSize)
		_ = e.Events() // throw away any events from this layout

		// Build the selection events
		startPos, endPos := e.offsetToScreenPos2(sortInts(start, end))
		tq := &testQueue{
			events: []event.Event{
				pointer.Event{
					Buttons:  pointer.ButtonLeft,
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

		e.Layout(gtx, cache, font, fontSize)
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
		{0, 4, "a123", screenPos{}, screenPos{Y: 0, X: 4}},
		{0, 11, "a123456789a", screenPos{}, screenPos{Y: 1, X: 5}},
		{2, 6, "2345", screenPos{Y: 0, X: 2}, screenPos{Y: 1, X: 0}},
		{41, 66, "56789d\ne123456789e\nf12345", screenPos{Y: 6, X: 5}, screenPos{Y: 11, X: 0}},
	} {
		// printLines(e)

		gtx.Constraints = layout.Exact(image.Pt(100, 100))
		if got := selected(tst.start, tst.end); got != tst.selection {
			t.Errorf("Test %d pt1: Expected %q, got %q", n, tst.selection, got)
			continue
		}

		// Constrain the editor to roughly 6 columns wide and redraw
		gtx.Constraints = layout.Exact(image.Pt(36, 36))
		// Keep existing selection
		gtx.Queue = nil
		e.Layout(gtx, cache, font, fontSize)

		if e.caret.end.lineCol != tst.startPos || e.caret.start.lineCol != tst.endPos {
			t.Errorf("Test %d pt2: Expected %#v, %#v; got %#v, %#v",
				n,
				e.caret.end.lineCol, e.caret.start.lineCol,
				tst.startPos, tst.endPos)
			continue
		}

		// printLines(e)
	}
}

// Verify that an existing selection is dismissed when you press arrow keys.
func TestSelectMove(t *testing.T) {
	e := new(Editor)
	e.SetText(`0123456789`)

	gtx := layout.Context{Ops: new(op.Ops)}
	cache := text.NewCache(gofont.Collection())
	font := text.Font{}
	fontSize := unit.Px(10)

	// Layout once to populate e.lines and get focus.
	gtx.Queue = newQueue(key.FocusEvent{Focus: true})
	e.Layout(gtx, cache, font, fontSize)

	testKey := func(keyName string) {
		// Select 345
		e.SetCaret(3, 6)
		if expected, got := "345", e.SelectedText(); expected != got {
			t.Errorf("KeyName %s, expected %q, got %q", keyName, expected, got)
		}

		// Press the key
		gtx.Queue = newQueue(key.Event{State: key.Press, Name: keyName})
		e.Layout(gtx, cache, font, fontSize)

		if expected, got := "", e.SelectedText(); expected != got {
			t.Errorf("KeyName %s, expected %q, got %q", keyName, expected, got)
		}
	}

	testKey(key.NameLeftArrow)
	testKey(key.NameRightArrow)
	testKey(key.NameUpArrow)
	testKey(key.NameDownArrow)
}

func textWidth(e *Editor, lineNum, colStart, colEnd int) float32 {
	var w fixed.Int26_6
	advances := e.lines[lineNum].Layout.Advances
	if colEnd > len(advances) {
		colEnd = len(advances)
	}
	for _, adv := range advances[colStart:colEnd] {
		w += adv
	}
	return float32(w.Floor())
}

func textHeight(e *Editor, lineNum int) float32 {
	var h fixed.Int26_6
	for _, line := range e.lines[0:lineNum] {
		h += line.Ascent + line.Descent
	}
	return float32(h.Floor() + 1)
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
	for n, line := range e.lines {
		text := strings.TrimSuffix(line.Layout.Text, "\n")
		fmt.Printf("%d: %s\n", n, text)
	}
}

// sortInts returns a and b sorted such that a2 <= b2.
func sortInts(a, b int) (a2, b2 int) {
	if b < a {
		return b, a
	}
	return a, b
}
