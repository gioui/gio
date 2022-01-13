// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"testing"

	"gioui.org/font/gofont"
	"gioui.org/io/key"
	"gioui.org/io/router"
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
		cache := text.NewCache(gofont.Collection())
		e := new(widget.Editor)
		e.Focus()

		var r router.Router
		gtx := layout.Context{Ops: new(op.Ops), Queue: &r}
		// Layout once to register focus.
		e.Layout(gtx, cache, text.Font{}, unit.Px(10), nil)
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
			rng := key.Range{
				Start: int(cmds[1]),
				End:   int(cmds[2]),
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
				state.Selection = rng
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
			e.Layout(gtx, cache, text.Font{}, unit.Px(10), nil)
			r.Frame(gtx.Ops)
			newState := r.EditorState()
			if newState != state.EditorState {
				t.Errorf("IME state: %+v\neditor state: %+v", state.EditorState, newState)
			}
		}
	})
}
