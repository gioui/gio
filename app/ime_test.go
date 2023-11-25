// SPDX-License-Identifier: Unlicense OR MIT

//go:build go1.18
// +build go1.18

package app

import (
	"testing"
	"unicode/utf8"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

func FuzzIME(f *testing.F) {
	runes := []rune("Hello, 世界! 🤬 علي،الحسنب北查爾斯頓工廠的安全漏洞已")
	f.Add([]byte("20\x0010"))
	f.Add([]byte("80000"))
	f.Add([]byte("2008\"80\r00"))
	f.Add([]byte("20007900002\x02000"))
	f.Add([]byte("20007800002\x02000"))
	f.Add([]byte("200A02000990\x19002\x17\x0200"))
	f.Fuzz(func(t *testing.T, cmds []byte) {
		cache := text.NewShaper(text.WithCollection(gofont.Collection()))
		e := new(widget.Editor)

		var r input.Router
		gtx := layout.Context{Ops: new(op.Ops), Source: r.Source()}
		gtx.Execute(key.FocusCmd{Tag: e})
		// Layout once to register focus.
		e.Layout(gtx, cache, font.Font{}, unit.Sp(10), op.CallOp{}, op.CallOp{})
		r.Frame(gtx.Ops)

		var state editorState
		const (
			cmdReplace = iota
			cmdSelect
			cmdSnip
			maxCmd
		)
		const cmdLen = 5
		for len(cmds) >= cmdLen {
			n := e.Len()
			rng := key.Range{
				Start: int(cmds[1]) % (n + 1),
				End:   int(cmds[2]) % (n + 1),
			}
			switch cmds[0] % cmdLen {
			case cmdReplace:
				rstart := int(cmds[3]) % len(runes)
				rend := int(cmds[4]) % len(runes)
				if rstart > rend {
					rstart, rend = rend, rstart
				}
				replacement := string(runes[rstart:rend])
				state.Replace(rng, replacement)
				r.Queue(key.EditEvent{Range: rng, Text: replacement})
				r.Queue(key.SnippetEvent(state.Snippet.Range))
			case cmdSelect:
				r.Queue(key.SelectionEvent(rng))
				runes := []rune(e.Text())
				if rng.Start < 0 {
					rng.Start = 0
				}
				if rng.End < 0 {
					rng.End = 0
				}
				if rng.Start > len(runes) {
					rng.Start = len(runes)
				}
				if rng.End > len(runes) {
					rng.End = len(runes)
				}
				state.Selection.Range = rng
			case cmdSnip:
				r.Queue(key.SnippetEvent(rng))
				runes := []rune(e.Text())
				if rng.Start > rng.End {
					rng.Start, rng.End = rng.End, rng.Start
				}
				if rng.Start < 0 {
					rng.Start = 0
				}
				if rng.End < 0 {
					rng.End = 0
				}
				if rng.Start > len(runes) {
					rng.Start = len(runes)
				}
				if rng.End > len(runes) {
					rng.End = len(runes)
				}
				state.Snippet = key.Snippet{
					Range: rng,
					Text:  string(runes[rng.Start:rng.End]),
				}
			}
			cmds = cmds[cmdLen:]
			e.Layout(gtx, cache, font.Font{}, unit.Sp(10), op.CallOp{}, op.CallOp{})
			r.Frame(gtx.Ops)
			newState := r.EditorState()
			// We don't track caret position.
			state.Selection.Caret = newState.Selection.Caret
			// Expanded snippets are ok.
			their, our := newState.Snippet, state.EditorState.Snippet
			beforeLen := 0
			for before := our.Start - their.Start; before > 0; before-- {
				_, n := utf8.DecodeRuneInString(their.Text[beforeLen:])
				beforeLen += n
			}
			afterLen := 0
			for after := their.End - our.End; after > 0; after-- {
				_, n := utf8.DecodeLastRuneInString(their.Text[:len(their.Text)-afterLen])
				afterLen += n
			}
			if beforeLen > 0 {
				our.Text = their.Text[:beforeLen] + our.Text
				our.Start = their.Start
			}
			if afterLen > 0 {
				our.Text = our.Text + their.Text[len(their.Text)-afterLen:]
				our.End = their.End
			}
			state.EditorState.Snippet = our
			if newState != state.EditorState {
				t.Errorf("IME state: %+v\neditor state: %+v", state.EditorState, newState)
			}
		}
	})
}

func TestEditorIndices(t *testing.T) {
	var s editorState
	const str = "Hello, 😀"
	s.Snippet = key.Snippet{
		Text: str,
		Range: key.Range{
			Start: 10,
			End:   utf8.RuneCountInString(str),
		},
	}
	utf16Indices := [...]struct {
		Runes, UTF16 int
	}{
		{0, 0}, {10, 10}, {17, 17}, {18, 19}, {30, 31},
	}
	for _, p := range utf16Indices {
		if want, got := p.UTF16, s.UTF16Index(p.Runes); want != got {
			t.Errorf("UTF16Index(%d) = %d, wanted %d", p.Runes, got, want)
		}
		if want, got := p.Runes, s.RunesIndex(p.UTF16); want != got {
			t.Errorf("RunesIndex(%d) = %d, wanted %d", p.UTF16, got, want)
		}
	}
}
