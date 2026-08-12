// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"image"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/io/event"
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
		state.Selection.Transform = f32.AffineId()
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
	s.Selection.Transform = f32.AffineId()
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

func TestIMERange(t *testing.T) {
	editorStateWithSelection := func(compose, selection key.Range) editorState {
		var state editorState
		state.compose = compose
		state.Selection.Range = selection
		return state
	}
	for _, tc := range []struct {
		name string
		in   editorState
		want key.Range
	}{
		{
			name: "selection fallback",
			in:   editorStateWithSelection(key.Range{Start: -1, End: -1}, key.Range{Start: 2, End: 5}),
			want: key.Range{Start: 2, End: 5},
		},
		{
			name: "composition wins",
			in:   editorStateWithSelection(key.Range{Start: 4, End: 9}, key.Range{Start: 1, End: 1}),
			want: key.Range{Start: 4, End: 9},
		},
		{
			name: "normalize reversed",
			in: editorState{
				compose: key.Range{Start: 8, End: 3},
			},
			want: key.Range{Start: 3, End: 8},
		},
	} {
		if got := imeRange(tc.in); got != tc.want {
			t.Errorf("%s: imeRange() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestShouldCancelComposition(t *testing.T) {
	base := editorState{}
	base.Selection.Range = key.Range{Start: 12, End: 12}
	base.Snippet = key.Snippet{
		Range: key.Range{Start: 12, End: 17},
		Text:  "hello",
	}

	expanded := base
	expanded.Snippet = key.Snippet{
		Range: key.Range{Start: 10, End: 17},
		Text:  "拼音hello",
	}
	if shouldCancelComposition(base, expanded) {
		t.Fatal("expanded but consistent snippet should not cancel composition")
	}

	changedText := base
	changedText.Snippet.Text = "hullo"
	if !shouldCancelComposition(base, changedText) {
		t.Fatal("changed snippet text should cancel composition")
	}

	movedSelection := base
	movedSelection.Selection.Range = key.Range{Start: 13, End: 13}
	if !shouldCancelComposition(base, movedSelection) {
		t.Fatal("changed selection should cancel composition")
	}
}

func TestEditorCompositionBounds(t *testing.T) {
	cache := text.NewShaper(text.WithCollection(gofont.Collection()))
	e := new(widget.Editor)
	e.SetText("hello world")

	var r input.Router
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Source:      r.Source(),
		Constraints: layout.Exact(image.Pt(300, 100)),
	}
	gtx.Execute(key.FocusCmd{Tag: e})

	layoutEditor := func() {
		ops.Reset()
		gtx.Ops = &ops
		gtx.Source = r.Source()
		e.Layout(gtx, cache, font.Font{}, unit.Sp(12), op.CallOp{}, op.CallOp{})
		r.Frame(gtx.Ops)
	}

	layoutEditor()
	r.Queue(key.CompositionEvent{Start: 0, End: 5})
	layoutEditor()
	if bounds := r.EditorState().Selection.CompositionBounds; bounds.Empty() {
		t.Fatalf("expected non-empty composition bounds")
	}

	r.Queue(key.CompositionEvent{Start: -1, End: -1})
	layoutEditor()
	if bounds := r.EditorState().Selection.CompositionBounds; !bounds.Empty() {
		t.Fatalf("expected empty composition bounds, got %v", bounds)
	}
}

type textInputTestDriver struct {
	driver
	events []event.Event
}

func (d *textInputTestDriver) ProcessEvent(e event.Event) {
	d.events = append(d.events, e)
}

func newTextInputTestEditor(state editorState) (*callbacks, *textInputTestDriver) {
	d := new(textInputTestDriver)
	w := &Window{driver: d, imeState: state}
	return &callbacks{w: w}, d
}

func textInputTestState(text string, selection key.Range) editorState {
	var state editorState
	state.Selection.Transform = f32.AffineId()
	state.Selection.Range = selection
	state.Snippet = key.Snippet{
		Range: key.Range{End: utf8.RuneCountInString(text)},
		Text:  text,
	}
	state.compose = key.Range{Start: -1, End: -1}
	return state
}

func TestTextInputAppliesPreedit(t *testing.T) {
	state := textInputTestState("ab", key.Range{Start: 1, End: 1})
	editor, _ := newTextInputTestEditor(state)
	s := textInputState{pending: textInputUpdate{
		preedit:          "拼音",
		preeditSet:       true,
		preeditSelection: key.Range{Start: 0, End: 3},
	}}
	if !s.apply(editor) {
		t.Fatal("done did not report a changed editor state")
	}
	if got, want := editor.EditorState().Snippet.Text, "a拼音b"; got != want {
		t.Fatalf("text after done = %q, want %q", got, want)
	}
}

func TestTextInputAppliesTransactionInProtocolOrder(t *testing.T) {
	state := textInputTestState("abc旧def", key.Range{Start: 4, End: 4})
	state.compose = key.Range{Start: 3, End: 4}
	editor, d := newTextInputTestEditor(state)
	s := textInputState{
		sent: textInputSnapshot{text: "abcdef", cursor: 3, anchor: 3},
		pending: textInputUpdate{
			deleteBefore:     1,
			deleteAfter:      1,
			commit:           "中",
			commitSet:        true,
			preedit:          "かな",
			preeditSet:       true,
			preeditSelection: key.Range{Start: 0, End: len("か")},
		},
	}
	if !s.apply(editor) {
		t.Fatal("transaction did not change editor state")
	}
	if s.pending != (textInputUpdate{}) {
		t.Fatalf("pending state after done = %+v, want zero value", s.pending)
	}
	got := editor.EditorState()
	if want := "ab中かなef"; got.Snippet.Text != want {
		t.Fatalf("text = %q, want %q", got.Snippet.Text, want)
	}
	if want := (key.Range{Start: 3, End: 5}); got.compose != want {
		t.Fatalf("compose = %+v, want %+v", got.compose, want)
	}
	if want := (key.Range{Start: 3, End: 4}); got.Selection.Range != want {
		t.Fatalf("selection = %+v, want %+v", got.Selection.Range, want)
	}
	wantEvents := []event.Event{
		key.EditEvent{Range: key.Range{Start: 3, End: 4}, Text: ""},
		key.SnippetEvent(key.Range{Start: 0, End: 6}),
		key.CompositionEvent(key.Range{Start: -1, End: -1}),
		key.SelectionEvent(key.Range{Start: 3, End: 3}),
		key.EditEvent{Range: key.Range{Start: 2, End: 4}, Text: ""},
		key.SnippetEvent(key.Range{Start: 0, End: 4}),
		key.SelectionEvent(key.Range{Start: 2, End: 2}),
		key.EditEvent{Range: key.Range{Start: 2, End: 2}, Text: "中"},
		key.SnippetEvent(key.Range{Start: 0, End: 5}),
		key.SelectionEvent(key.Range{Start: 3, End: 3}),
		key.EditEvent{Range: key.Range{Start: 3, End: 3}, Text: "かな"},
		key.SnippetEvent(key.Range{Start: 0, End: 7}),
		key.CompositionEvent(key.Range{Start: 3, End: 5}),
		key.SelectionEvent(key.Range{Start: 3, End: 4}),
	}
	if !reflect.DeepEqual(d.events, wantEvents) {
		t.Fatalf("events:\n got: %#v\nwant: %#v", d.events, wantEvents)
	}
}

func TestTextInputNullableEmptyEvents(t *testing.T) {
	t.Run("empty done preserves selection", func(t *testing.T) {
		state := textInputTestState("abc", key.Range{Start: 1, End: 2})
		editor, _ := newTextInputTestEditor(state)
		if (&textInputState{}).apply(editor) {
			t.Fatal("empty done changed an unrelated selection")
		}
		if got := editor.EditorState(); got != state {
			t.Fatalf("state after empty done = %+v, want %+v", got, state)
		}
	})

	for _, tc := range []struct {
		name        string
		pending     textInputUpdate
		wantText    string
		wantCompose key.Range
	}{
		{
			name:        "empty commit",
			pending:     textInputUpdate{commitSet: true},
			wantText:    "ac",
			wantCompose: key.Range{Start: -1, End: -1},
		},
		{
			name:        "empty preedit",
			pending:     textInputUpdate{preeditSet: true},
			wantText:    "ac",
			wantCompose: key.Range{Start: 1, End: 1},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := textInputTestState("abc", key.Range{Start: 1, End: 2})
			editor, _ := newTextInputTestEditor(state)
			s := textInputState{pending: tc.pending}
			if !s.apply(editor) {
				t.Fatal("non-null empty event did not change selected text")
			}
			got := editor.EditorState()
			if got.Snippet.Text != tc.wantText {
				t.Fatalf("text = %q, want %q", got.Snippet.Text, tc.wantText)
			}
			if got.compose != tc.wantCompose {
				t.Fatalf("compose = %+v, want %+v", got.compose, tc.wantCompose)
			}
		})
	}
}

func TestTextInputPreeditReplacement(t *testing.T) {
	t.Run("absent preedit clears old composition", func(t *testing.T) {
		state := textInputTestState("a拼b", key.Range{Start: 2, End: 2})
		state.compose = key.Range{Start: 1, End: 2}
		editor, _ := newTextInputTestEditor(state)
		if !(&textInputState{}).apply(editor) {
			t.Fatal("empty pending preedit did not clear old composition")
		}
		got := editor.EditorState()
		if got.Snippet.Text != "ab" || got.compose != (key.Range{Start: -1, End: -1}) {
			t.Fatalf("state after clear = %+v, want text %q and no composition", got, "ab")
		}
	})

	t.Run("identical preedit needs no refresh", func(t *testing.T) {
		state := textInputTestState("a拼b", key.Range{Start: 2, End: 2})
		state.compose = key.Range{Start: 1, End: 2}
		editor, _ := newTextInputTestEditor(state)
		s := textInputState{pending: textInputUpdate{
			preedit:          "拼",
			preeditSet:       true,
			preeditSelection: key.Range{Start: len("拼"), End: len("拼")},
		}}
		if s.apply(editor) {
			t.Fatal("identical preedit and cursor requested a state refresh")
		}
		if got := editor.EditorState(); got != state {
			t.Fatalf("state after identical preedit = %+v, want %+v", got, state)
		}
	})
}

func TestTextInputDoneSerial(t *testing.T) {
	t.Run("empty stale done does not refresh", func(t *testing.T) {
		state := textInputTestState("ab", key.Range{Start: 1, End: 1})
		editor, _ := newTextInputTestEditor(state)
		s := textInputState{serial: 2}
		if s.applyDone(editor, 1) {
			t.Fatal("empty stale done requested a state refresh")
		}
		if s.dirty {
			t.Fatal("empty stale done left a deferred refresh")
		}
		if !s.stale {
			t.Fatal("mismatched done did not mark the state stale")
		}
	})

	t.Run("stale edit applies without refresh", func(t *testing.T) {
		state := textInputTestState("ab", key.Range{Start: 1, End: 1})
		editor, _ := newTextInputTestEditor(state)
		s := textInputState{
			serial:  2,
			pending: textInputUpdate{commit: "中", commitSet: true},
		}
		if s.applyDone(editor, 1) {
			t.Fatal("stale done allowed a state refresh")
		}
		if !s.dirty {
			t.Fatal("stale edit did not retain a deferred refresh")
		}
		if !s.stale {
			t.Fatal("mismatched done did not mark the state stale")
		}
		if got, want := editor.EditorState().Snippet.Text, "a中b"; got != want {
			t.Fatalf("stale done did not apply edit: got %q, want %q", got, want)
		}
		if !s.applyDone(editor, 2) {
			t.Fatal("matching empty done did not release the deferred refresh")
		}
		if s.stale {
			t.Fatal("matching done did not clear staleness")
		}
	})

	t.Run("matching edit refreshes", func(t *testing.T) {
		state := textInputTestState("ab", key.Range{Start: 1, End: 1})
		editor, _ := newTextInputTestEditor(state)
		s := textInputState{
			serial:  2,
			pending: textInputUpdate{commit: "中", commitSet: true},
		}
		if !s.applyDone(editor, 2) {
			t.Fatal("matching changed done did not request a state refresh")
		}
		if !s.dirty {
			t.Fatal("matching changed done did not retain its refresh")
		}
		if s.stale {
			t.Fatal("matching done marked the state stale")
		}
	})
}

func TestTextInputSurroundingText(t *testing.T) {
	t.Run("removes preedit", func(t *testing.T) {
		state := textInputTestState("前pre後", key.Range{Start: 2, End: 2})
		state.compose = key.Range{Start: 1, End: 4}
		snapshot, status := textInputSurrounding(state)
		if status != surroundingReady {
			t.Fatalf("status = %v, want ready", status)
		}
		if snapshot.text != "前後" || snapshot.cursor != len("前") || snapshot.anchor != len("前") {
			t.Fatalf("snapshot = %+v, want text %q with cursor and anchor %d", snapshot, "前後", len("前"))
		}
	})

	t.Run("windows at UTF-8 boundaries", func(t *testing.T) {
		text := strings.Repeat("界", 2000)
		state := textInputTestState(text, key.Range{Start: 1000, End: 1000})
		snapshot, status := textInputSurrounding(state)
		if status != surroundingReady {
			t.Fatalf("status = %v, want ready", status)
		}
		if len(snapshot.text) > maxSurroundingTextBytes {
			t.Fatalf("surrounding text has %d bytes, maximum is %d", len(snapshot.text), maxSurroundingTextBytes)
		}
		if !utf8.ValidString(snapshot.text) {
			t.Fatal("surrounding text ends inside a UTF-8 code point")
		}
		if snapshot.cursor < 0 || snapshot.cursor > len(snapshot.text) ||
			snapshot.cursor < len(snapshot.text) && !utf8.RuneStart(snapshot.text[snapshot.cursor]) {
			t.Fatalf("cursor %d is not a valid UTF-8 boundary", snapshot.cursor)
		}
		// Windowing must not move the caret.
		caret := snapshot.start + utf8.RuneCountInString(snapshot.text[:snapshot.cursor])
		if want := state.Selection.Start; caret != want {
			t.Fatalf("windowed cursor resolves to rune %d, want %d", caret, want)
		}
	})

	t.Run("maps offsets through a partial snippet", func(t *testing.T) {
		// A snippet covering runes 10..16 of a larger document.
		state := textInputTestState("甲乙選択丙丁", key.Range{Start: 12, End: 14})
		state.Snippet.Range = key.Range{Start: 10, End: 16}
		snapshot, status := textInputSurrounding(state)
		if status != surroundingReady {
			t.Fatalf("status = %v, want ready", status)
		}
		if snapshot.start != 10 {
			t.Fatalf("start = %d, want 10", snapshot.start)
		}
		if snapshot.cursor != len("甲乙") || snapshot.anchor != len("甲乙選択") {
			t.Fatalf("snapshot = %+v, want cursor %d and anchor %d",
				snapshot, len("甲乙"), len("甲乙選択"))
		}
		// Deletions map back to document offsets, not snippet offsets.
		rng, ok := snapshot.convertRange(uint32(len("乙")), uint32(len("丙")))
		if !ok {
			t.Fatal("valid UTF-8 delete range was rejected")
		}
		if want := (key.Range{Start: 11, End: 15}); rng != want {
			t.Fatalf("delete range = %+v, want %+v", rng, want)
		}
	})

	t.Run("preserves logical offsets for reversed bidi selection", func(t *testing.T) {
		state := textInputTestState("aمرحباb", key.Range{Start: 6, End: 2})
		snapshot, status := textInputSurrounding(state)
		if status != surroundingReady {
			t.Fatalf("status = %v, want ready", status)
		}
		// Cursor and anchor are UTF-8 byte offsets in logical order,
		// and their direction is preserved.
		if want := len("aمرحبا"); snapshot.cursor != want {
			t.Fatalf("cursor = %d, want %d", snapshot.cursor, want)
		}
		if want := len("aم"); snapshot.anchor != want {
			t.Fatalf("anchor = %d, want %d", snapshot.anchor, want)
		}
	})

	t.Run("rejects selection larger than limit", func(t *testing.T) {
		text := strings.Repeat("界", 1400)
		state := textInputTestState(text, key.Range{Start: 0, End: 1400})
		if _, status := textInputSurrounding(state); status != surroundingUnavailable {
			t.Fatalf("status = %v, want unavailable", status)
		}
	})
}

func TestTextInputDeleteRangeUsesUTF8Bytes(t *testing.T) {
	text := "甲乙選択丙丁"
	for _, snapshot := range []textInputSnapshot{
		{text: text, start: 10, cursor: len("甲乙"), anchor: len("甲乙選択")},
		{text: text, start: 10, cursor: len("甲乙選択"), anchor: len("甲乙")},
	} {
		rng, ok := snapshot.convertRange(uint32(len("乙")), uint32(len("丙")))
		if !ok {
			t.Fatal("valid UTF-8 delete range was rejected")
		}
		if want := (key.Range{Start: 11, End: 15}); rng != want {
			t.Fatalf("delete range = %+v, want %+v", rng, want)
		}
	}
}
