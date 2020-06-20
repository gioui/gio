// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"fmt"
	"image"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"gioui.org/f32"
	"gioui.org/font/gofont"
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

// Generate generates a value of itself, for testing/quick.
func (editMutation) Generate(rand *rand.Rand, size int) reflect.Value {
	t := editMutation(rand.Intn(int(moveLast)))
	return reflect.ValueOf(t)
}
