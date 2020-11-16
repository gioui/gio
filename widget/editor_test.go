// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"fmt"
	"image"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"
	"unicode"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
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
	e.moveEnd()
	assertCaret(t, e, 0, 3, len("æbc"))
	e.Move(+1)
	assertCaret(t, e, 1, 0, len("æbc\n"))
	e.Move(-1)
	assertCaret(t, e, 0, 3, len("æbc"))
	e.moveLines(+1)
	assertCaret(t, e, 1, 3, len("æbc\naøå"))
	e.moveEnd()
	assertCaret(t, e, 1, 4, len("æbc\naøå•"))
	e.Move(+1)
	assertCaret(t, e, 1, 4, len("æbc\naøå•"))

	// Ensure that password masking does not affect caret behavior
	e.Move(-3)
	assertCaret(t, e, 1, 1, len("æbc\na"))
	e.Mask = '*'
	e.Layout(gtx, cache, font, fontSize)
	assertCaret(t, e, 1, 1, len("æbc\na"))
	e.Move(-3)
	assertCaret(t, e, 0, 2, len("æb"))
	e.Mask = '\U0001F92B'
	e.Layout(gtx, cache, font, fontSize)
	e.moveEnd()
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

type testQueue struct {
	events []event.Event
}

func (q *testQueue) Events(_ event.Tag) []event.Event {
	return q.events
}

// assertCaret asserts that the editor caret is at a particular line
// and column, and that the byte position matches as well.
func assertCaret(t *testing.T, e *Editor, line, col, bytes int) {
	t.Helper()
	gotLine, gotCol := e.CaretPos()
	if gotLine != line || gotCol != col {
		t.Errorf("caret at (%d, %d), expected (%d, %d)", gotLine, gotCol, line, col)
	}
	if bytes != e.rr.caret {
		t.Errorf("caret at buffer position %d, expected %d", e.rr.caret, bytes)
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
			wantLine, wantCol, wantX, wantY := e.layoutCaret()
			wantCoords := f32.Pt(float32(wantX)/64, float32(wantY))
			if wantLine == gotLine && wantCol == gotCol && gotCoords == wantCoords {
				return nil
			}
			return fmt.Errorf("caret (%d,%d) pos %s, want (%d,%d) pos %s", gotLine, gotCol, gotCoords, wantLine, wantCol, wantCoords)
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
				e.Move(int(distance))
			case moveLine:
				e.moveLines(int(distance))
			case movePage:
				e.movePages(int(distance))
			case moveStart:
				e.moveStart()
			case moveEnd:
				e.moveEnd()
			case moveCoord:
				e.moveCoord(image.Pt(int(x), int(y)))
			case moveWord:
				e.moveWord(int(distance))
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
		e.Move(tt.Start)
		e.moveWord(tt.Skip)
		if e.rr.caret != tt.Want {
			t.Fatalf("[%d] moveWord: bad caret position: got %d, want %d", ii, e.rr.caret, tt.Want)
		}
	}
}

func TestEditorDeleteWord(t *testing.T) {
	type Test struct {
		Text   string
		Start  int
		Delete int

		Want   int
		Result string
	}
	tests := []Test{
		{"", 0, 0, 0, ""},
		{"", 0, -1, 0, ""},
		{"", 0, 1, 0, ""},
		{"hello", 0, -1, 0, "hello"},
		{"hello", 0, 1, 0, ""},
		{"hello world", 3, 1, 3, "hel world"},
		{"hello world", 3, -1, 0, "lo world"},
		{"hello world", 8, -1, 6, "hello rld"},
		{"hello world", 8, 1, 8, "hello wo"},
		{"hello    world", 3, 1, 3, "hel    world"},
		{"hello    world", 3, 2, 3, "helworld"},
		{"hello    world", 8, 1, 8, "hello   "},
		{"hello    world", 8, -1, 5, "hello world"},
		{"hello brave new world", 0, 3, 0, " new world"},
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
		e.Move(tt.Start)
		e.deleteWord(tt.Delete)
		if e.rr.caret != tt.Want {
			t.Fatalf("[%d] deleteWord: bad caret position: got %d, want %d", ii, e.rr.caret, tt.Want)
		}
		if e.Text() != tt.Result {
			t.Fatalf("[%d] deleteWord: invalid result: got %q, want %q", ii, e.Text(), tt.Result)
		}
	}
}

func TestEditorNoLayout(t *testing.T) {
	var e Editor
	e.SetText("hi!\n")
	e.Move(1)
}

// Generate generates a value of itself, for testing/quick.
func (editMutation) Generate(rand *rand.Rand, size int) reflect.Value {
	t := editMutation(rand.Intn(int(moveLast)))
	return reflect.ValueOf(t)
}
