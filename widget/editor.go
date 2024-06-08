// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"bufio"
	"image"
	"io"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/io/system"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
)

// Editor implements an editable and scrollable text area.
type Editor struct {
	// text manages the text buffer and provides shaping and cursor positioning
	// services.
	text textView
	// Alignment controls the alignment of text within the editor.
	Alignment text.Alignment
	// LineHeight determines the gap between baselines of text. If zero, a sensible
	// default will be used.
	LineHeight unit.Sp
	// LineHeightScale is multiplied by LineHeight to determine the final gap
	// between baselines. If zero, a sensible default will be used.
	LineHeightScale float32
	// SingleLine force the text to stay on a single line.
	// SingleLine also sets the scrolling direction to
	// horizontal.
	SingleLine bool
	// ReadOnly controls whether the contents of the editor can be altered by
	// user interaction. If set to true, the editor will allow selecting text
	// and copying it interactively, but not modifying it.
	ReadOnly bool
	// Submit enabled translation of carriage return keys to SubmitEvents.
	// If not enabled, carriage returns are inserted as newlines in the text.
	Submit bool
	// Mask replaces the visual display of each rune in the contents with the given rune.
	// Newline characters are not masked. When non-zero, the unmasked contents
	// are accessed by Len, Text, and SetText.
	Mask rune
	// InputHint specifies the type of on-screen keyboard to be displayed.
	InputHint key.InputHint
	// MaxLen limits the editor content to a maximum length. Zero means no limit.
	MaxLen int
	// Filter is the list of characters allowed in the Editor. If Filter is empty,
	// all characters are allowed.
	Filter string
	// WrapPolicy configures how displayed text will be broken into lines.
	WrapPolicy text.WrapPolicy

	buffer *editBuffer
	// scratch is a byte buffer that is reused to efficiently read portions of text
	// from the textView.
	scratch    []byte
	blinkStart time.Time

	// ime tracks the state relevant to input methods.
	ime struct {
		imeState
		scratch []byte
	}

	dragging    bool
	dragger     gesture.Drag
	scroller    gesture.Scroll
	scrollCaret bool
	showCaret   bool

	clicker gesture.Click

	// history contains undo history.
	history []modification
	// nextHistoryIdx is the index within the history of the next modification. This
	// is only not len(history) immediately after undo operations occur. It is framed as the "next" value
	// to make the zero value consistent.
	nextHistoryIdx int

	pending []EditorEvent
}

type offEntry struct {
	runes int
	bytes int
}

type imeState struct {
	selection struct {
		rng   key.Range
		caret key.Caret
	}
	snippet    key.Snippet
	start, end int
}

type maskReader struct {
	// rr is the underlying reader.
	rr      io.RuneReader
	maskBuf [utf8.UTFMax]byte
	// mask is the utf-8 encoded mask rune.
	mask []byte
	// overflow contains excess mask bytes left over after the last Read call.
	overflow []byte
}

type selectionAction int

const (
	selectionExtend selectionAction = iota
	selectionClear
)

func (m *maskReader) Reset(r io.Reader, mr rune) {
	m.rr = bufio.NewReader(r)
	n := utf8.EncodeRune(m.maskBuf[:], mr)
	m.mask = m.maskBuf[:n]
}

// Read reads from the underlying reader and replaces every
// rune with the mask rune.
func (m *maskReader) Read(b []byte) (n int, err error) {
	for len(b) > 0 {
		var replacement []byte
		if len(m.overflow) > 0 {
			replacement = m.overflow
		} else {
			var r rune
			r, _, err = m.rr.ReadRune()
			if err != nil {
				break
			}
			if r == '\n' {
				replacement = []byte{'\n'}
			} else {
				replacement = m.mask
			}
		}
		nn := copy(b, replacement)
		m.overflow = replacement[nn:]
		n += nn
		b = b[nn:]
	}
	return n, err
}

type EditorEvent interface {
	isEditorEvent()
}

// A ChangeEvent is generated for every user change to the text.
type ChangeEvent struct{}

// A SubmitEvent is generated when Submit is set
// and a carriage return key is pressed.
type SubmitEvent struct {
	Text string
}

// A SelectEvent is generated when the user selects some text, or changes the
// selection (e.g. with a shift-click), including if they remove the
// selection. The selected text is not part of the event, on the theory that
// it could be a relatively expensive operation (for a large editor), most
// applications won't actually care about it, and those that do can call
// Editor.SelectedText() (which can be empty).
type SelectEvent struct{}

const (
	blinksPerSecond  = 1
	maxBlinkDuration = 10 * time.Second
)

func (e *Editor) processEvents(gtx layout.Context) (ev EditorEvent, ok bool) {
	if len(e.pending) > 0 {
		out := e.pending[0]
		e.pending = e.pending[:copy(e.pending, e.pending[1:])]
		return out, true
	}
	selStart, selEnd := e.Selection()
	defer func() {
		afterSelStart, afterSelEnd := e.Selection()
		if selStart != afterSelStart || selEnd != afterSelEnd {
			if ok {
				e.pending = append(e.pending, SelectEvent{})
			} else {
				ev = SelectEvent{}
				ok = true
			}
		}
	}()

	ev, ok = e.processPointer(gtx)
	if ok {
		return ev, ok
	}
	ev, ok = e.processKey(gtx)
	if ok {
		return ev, ok
	}
	return nil, false
}

func (e *Editor) processPointer(gtx layout.Context) (EditorEvent, bool) {
	sbounds := e.text.ScrollBounds()
	var smin, smax int
	var axis gesture.Axis
	if e.SingleLine {
		axis = gesture.Horizontal
		smin, smax = sbounds.Min.X, sbounds.Max.X
	} else {
		axis = gesture.Vertical
		smin, smax = sbounds.Min.Y, sbounds.Max.Y
	}
	var scrollX, scrollY pointer.ScrollRange
	textDims := e.text.FullDimensions()
	visibleDims := e.text.Dimensions()
	if e.SingleLine {
		scrollOffX := e.text.ScrollOff().X
		scrollX.Min = min(-scrollOffX, 0)
		scrollX.Max = max(0, textDims.Size.X-(scrollOffX+visibleDims.Size.X))
	} else {
		scrollOffY := e.text.ScrollOff().Y
		scrollY.Min = -scrollOffY
		scrollY.Max = max(0, textDims.Size.Y-(scrollOffY+visibleDims.Size.Y))
	}
	sdist := e.scroller.Update(gtx.Metric, gtx.Source, gtx.Now, axis, scrollX, scrollY)
	var soff int
	if e.SingleLine {
		e.text.ScrollRel(sdist, 0)
		soff = e.text.ScrollOff().X
	} else {
		e.text.ScrollRel(0, sdist)
		soff = e.text.ScrollOff().Y
	}
	for {
		evt, ok := e.clicker.Update(gtx.Source)
		if !ok {
			break
		}
		ev, ok := e.processPointerEvent(gtx, evt)
		if ok {
			return ev, ok
		}
	}
	for {
		evt, ok := e.dragger.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		ev, ok := e.processPointerEvent(gtx, evt)
		if ok {
			return ev, ok
		}
	}

	if (sdist > 0 && soff >= smax) || (sdist < 0 && soff <= smin) {
		e.scroller.Stop()
	}
	return nil, false
}

func (e *Editor) processPointerEvent(gtx layout.Context, ev event.Event) (EditorEvent, bool) {
	switch evt := ev.(type) {
	case gesture.ClickEvent:
		switch {
		case evt.Kind == gesture.KindPress && evt.Source == pointer.Mouse,
			evt.Kind == gesture.KindClick && evt.Source != pointer.Mouse:
			prevCaretPos, _ := e.text.Selection()
			e.blinkStart = gtx.Now
			e.text.MoveCoord(image.Point{
				X: int(math.Round(float64(evt.Position.X))),
				Y: int(math.Round(float64(evt.Position.Y))),
			})
			gtx.Execute(key.FocusCmd{Tag: e})
			if !e.ReadOnly {
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			}
			if e.scroller.State() != gesture.StateFlinging {
				e.scrollCaret = true
			}

			if evt.Modifiers == key.ModShift {
				start, end := e.text.Selection()
				// If they clicked closer to the end, then change the end to
				// where the caret used to be (effectively swapping start & end).
				if abs(end-start) < abs(start-prevCaretPos) {
					e.text.SetCaret(start, prevCaretPos)
				}
			} else {
				e.text.ClearSelection()
			}
			e.dragging = true

			// Process multi-clicks.
			switch {
			case evt.NumClicks == 2:
				e.text.MoveWord(-1, selectionClear)
				e.text.MoveWord(1, selectionExtend)
				e.dragging = false
			case evt.NumClicks >= 3:
				e.text.MoveLineStart(selectionClear)
				e.text.MoveLineEnd(selectionExtend)
				e.dragging = false
			}
		}
	case pointer.Event:
		release := false
		switch {
		case evt.Kind == pointer.Release && evt.Source == pointer.Mouse:
			release = true
			fallthrough
		case evt.Kind == pointer.Drag && evt.Source == pointer.Mouse:
			if e.dragging {
				e.blinkStart = gtx.Now
				e.text.MoveCoord(image.Point{
					X: int(math.Round(float64(evt.Position.X))),
					Y: int(math.Round(float64(evt.Position.Y))),
				})
				e.scrollCaret = true

				if release {
					e.dragging = false
				}
			}
		}
	}
	return nil, false
}

func condFilter(pred bool, f key.Filter) event.Filter {
	if pred {
		return f
	} else {
		return nil
	}
}

func (e *Editor) processKey(gtx layout.Context) (EditorEvent, bool) {
	if e.text.Changed() {
		return ChangeEvent{}, true
	}
	caret, _ := e.text.Selection()
	atBeginning := caret == 0
	atEnd := caret == e.text.Len()
	if gtx.Locale.Direction.Progression() != system.FromOrigin {
		atEnd, atBeginning = atBeginning, atEnd
	}
	filters := []event.Filter{
		key.FocusFilter{Target: e},
		transfer.TargetFilter{Target: e, Type: "application/text"},
		key.Filter{Focus: e, Name: key.NameEnter, Optional: key.ModShift},
		key.Filter{Focus: e, Name: key.NameReturn, Optional: key.ModShift},

		key.Filter{Focus: e, Name: "Z", Required: key.ModShortcut, Optional: key.ModShift},
		key.Filter{Focus: e, Name: "C", Required: key.ModShortcut},
		key.Filter{Focus: e, Name: "V", Required: key.ModShortcut},
		key.Filter{Focus: e, Name: "X", Required: key.ModShortcut},
		key.Filter{Focus: e, Name: "A", Required: key.ModShortcut},

		key.Filter{Focus: e, Name: key.NameDeleteBackward, Optional: key.ModShortcutAlt | key.ModShift},
		key.Filter{Focus: e, Name: key.NameDeleteForward, Optional: key.ModShortcutAlt | key.ModShift},

		key.Filter{Focus: e, Name: key.NameHome, Optional: key.ModShortcut | key.ModShift},
		key.Filter{Focus: e, Name: key.NameEnd, Optional: key.ModShortcut | key.ModShift},
		key.Filter{Focus: e, Name: key.NamePageDown, Optional: key.ModShift},
		key.Filter{Focus: e, Name: key.NamePageUp, Optional: key.ModShift},
		condFilter(!atBeginning, key.Filter{Focus: e, Name: key.NameLeftArrow, Optional: key.ModShortcutAlt | key.ModShift}),
		condFilter(!atBeginning, key.Filter{Focus: e, Name: key.NameUpArrow, Optional: key.ModShortcutAlt | key.ModShift}),
		condFilter(!atEnd, key.Filter{Focus: e, Name: key.NameRightArrow, Optional: key.ModShortcutAlt | key.ModShift}),
		condFilter(!atEnd, key.Filter{Focus: e, Name: key.NameDownArrow, Optional: key.ModShortcutAlt | key.ModShift}),
	}
	// adjust keeps track of runes dropped because of MaxLen.
	var adjust int
	for {
		ke, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		e.blinkStart = gtx.Now
		switch ke := ke.(type) {
		case key.FocusEvent:
			// Reset IME state.
			e.ime.imeState = imeState{}
			if ke.Focus && !e.ReadOnly {
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			}
		case key.Event:
			if !gtx.Focused(e) || ke.State != key.Press {
				break
			}
			if !e.ReadOnly && e.Submit && (ke.Name == key.NameReturn || ke.Name == key.NameEnter) {
				if !ke.Modifiers.Contain(key.ModShift) {
					e.scratch = e.text.Text(e.scratch)
					return SubmitEvent{
						Text: string(e.scratch),
					}, true
				}
			}
			e.scrollCaret = true
			e.scroller.Stop()
			ev, ok := e.command(gtx, ke)
			if ok {
				return ev, ok
			}
		case key.SnippetEvent:
			e.updateSnippet(gtx, ke.Start, ke.End)
		case key.EditEvent:
			if e.ReadOnly {
				break
			}
			e.scrollCaret = true
			e.scroller.Stop()
			s := ke.Text
			moves := 0
			submit := false
			switch {
			case e.Submit:
				if i := strings.IndexByte(s, '\n'); i != -1 {
					submit = true
					moves += len(s) - i
					s = s[:i]
				}
			case e.SingleLine:
				s = strings.ReplaceAll(s, "\n", " ")
			}
			moves += e.replace(ke.Range.Start, ke.Range.End, s, true)
			adjust += utf8.RuneCountInString(ke.Text) - moves
			// Reset caret xoff.
			e.text.MoveCaret(0, 0)
			if submit {
				e.scratch = e.text.Text(e.scratch)
				submitEvent := SubmitEvent{
					Text: string(e.scratch),
				}
				if e.text.Changed() {
					e.pending = append(e.pending, submitEvent)
					return ChangeEvent{}, true
				}
				return submitEvent, true
			}
		// Complete a paste event, initiated by Shortcut-V in Editor.command().
		case transfer.DataEvent:
			e.scrollCaret = true
			e.scroller.Stop()
			content, err := io.ReadAll(ke.Open())
			if err == nil {
				if e.Insert(string(content)) != 0 {
					return ChangeEvent{}, true
				}
			}
		case key.SelectionEvent:
			e.scrollCaret = true
			e.scroller.Stop()
			ke.Start -= adjust
			ke.End -= adjust
			adjust = 0
			e.text.SetCaret(ke.Start, ke.End)
		}
	}
	if e.text.Changed() {
		return ChangeEvent{}, true
	}
	return nil, false
}

func (e *Editor) command(gtx layout.Context, k key.Event) (EditorEvent, bool) {
	direction := 1
	if gtx.Locale.Direction.Progression() == system.TowardOrigin {
		direction = -1
	}
	moveByWord := k.Modifiers.Contain(key.ModShortcutAlt)
	selAct := selectionClear
	if k.Modifiers.Contain(key.ModShift) {
		selAct = selectionExtend
	}
	if k.Modifiers.Contain(key.ModShortcut) {
		switch k.Name {
		// Initiate a paste operation, by requesting the clipboard contents; other
		// half is in Editor.processKey() under clipboard.Event.
		case "V":
			if !e.ReadOnly {
				gtx.Execute(clipboard.ReadCmd{Tag: e})
			}
		// Copy or Cut selection -- ignored if nothing selected.
		case "C", "X":
			e.scratch = e.text.SelectedText(e.scratch)
			if text := string(e.scratch); text != "" {
				gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
				if k.Name == "X" && !e.ReadOnly {
					if e.Delete(1) != 0 {
						return ChangeEvent{}, true
					}
				}
			}
		// Select all
		case "A":
			e.text.SetCaret(0, e.text.Len())
		case "Z":
			if !e.ReadOnly {
				if k.Modifiers.Contain(key.ModShift) {
					if ev, ok := e.redo(); ok {
						return ev, ok
					}
				} else {
					if ev, ok := e.undo(); ok {
						return ev, ok
					}
				}
			}
		case key.NameHome:
			e.text.MoveTextStart(selAct)
		case key.NameEnd:
			e.text.MoveTextEnd(selAct)
		}
		return nil, false
	}
	switch k.Name {
	case key.NameReturn, key.NameEnter:
		if !e.ReadOnly {
			if e.Insert("\n") != 0 {
				return ChangeEvent{}, true
			}
		}
	case key.NameDeleteBackward:
		if !e.ReadOnly {
			if moveByWord {
				if e.deleteWord(-1) != 0 {
					return ChangeEvent{}, true
				}
			} else {
				if e.Delete(-1) != 0 {
					return ChangeEvent{}, true
				}
			}
		}
	case key.NameDeleteForward:
		if !e.ReadOnly {
			if moveByWord {
				if e.deleteWord(1) != 0 {
					return ChangeEvent{}, true
				}
			} else {
				if e.Delete(1) != 0 {
					return ChangeEvent{}, true
				}
			}
		}
	case key.NameUpArrow:
		e.text.MoveLines(-1, selAct)
	case key.NameDownArrow:
		e.text.MoveLines(+1, selAct)
	case key.NameLeftArrow:
		if moveByWord {
			e.text.MoveWord(-1*direction, selAct)
		} else {
			if selAct == selectionClear {
				e.text.ClearSelection()
			}
			e.text.MoveCaret(-1*direction, -1*direction*int(selAct))
		}
	case key.NameRightArrow:
		if moveByWord {
			e.text.MoveWord(1*direction, selAct)
		} else {
			if selAct == selectionClear {
				e.text.ClearSelection()
			}
			e.text.MoveCaret(1*direction, int(selAct)*direction)
		}
	case key.NamePageUp:
		e.text.MovePages(-1, selAct)
	case key.NamePageDown:
		e.text.MovePages(+1, selAct)
	case key.NameHome:
		e.text.MoveLineStart(selAct)
	case key.NameEnd:
		e.text.MoveLineEnd(selAct)
	}
	return nil, false
}

// initBuffer should be invoked first in every exported function that accesses
// text state. It ensures that the underlying text widget is both ready to use
// and has its fields synced with the editor.
func (e *Editor) initBuffer() {
	if e.buffer == nil {
		e.buffer = new(editBuffer)
		e.text.SetSource(e.buffer)
	}
	e.text.Alignment = e.Alignment
	e.text.LineHeight = e.LineHeight
	e.text.LineHeightScale = e.LineHeightScale
	e.text.SingleLine = e.SingleLine
	e.text.Mask = e.Mask
	e.text.WrapPolicy = e.WrapPolicy
}

// Update the state of the editor in response to input events. Update consumes editor
// input events until there are no remaining events or an editor event is generated.
// To fully update the state of the editor, callers should call Update until it returns
// false.
func (e *Editor) Update(gtx layout.Context) (EditorEvent, bool) {
	e.initBuffer()
	event, ok := e.processEvents(gtx)
	// Notify IME of selection if it changed.
	newSel := e.ime.selection
	start, end := e.text.Selection()
	newSel.rng = key.Range{
		Start: start,
		End:   end,
	}
	caretPos, carAsc, carDesc := e.text.CaretInfo()
	newSel.caret = key.Caret{
		Pos:     layout.FPt(caretPos),
		Ascent:  float32(carAsc),
		Descent: float32(carDesc),
	}
	if newSel != e.ime.selection {
		e.ime.selection = newSel
		gtx.Execute(key.SelectionCmd{Tag: e, Range: newSel.rng, Caret: newSel.caret})
	}

	e.updateSnippet(gtx, e.ime.start, e.ime.end)
	return event, ok
}

// Layout lays out the editor using the provided textMaterial as the paint material
// for the text glyphs+caret and the selectMaterial as the paint material for the
// selection rectangle.
func (e *Editor) Layout(gtx layout.Context, lt *text.Shaper, font font.Font, size unit.Sp, textMaterial, selectMaterial op.CallOp) layout.Dimensions {
	for {
		_, ok := e.Update(gtx)
		if !ok {
			break
		}
	}

	e.text.Layout(gtx, lt, font, size)
	return e.layout(gtx, textMaterial, selectMaterial)
}

// updateSnippet queues a key.SnippetCmd if the snippet content or position
// have changed. off and len are in runes.
func (e *Editor) updateSnippet(gtx layout.Context, start, end int) {
	if start > end {
		start, end = end, start
	}
	length := e.text.Len()
	if start > length {
		start = length
	}
	if end > length {
		end = length
	}
	e.ime.start = start
	e.ime.end = end
	startOff := e.text.ByteOffset(start)
	endOff := e.text.ByteOffset(end)
	n := endOff - startOff
	if n > int64(len(e.ime.scratch)) {
		e.ime.scratch = make([]byte, n)
	}
	scratch := e.ime.scratch[:n]
	read, _ := e.text.ReadAt(scratch, startOff)
	if read != len(scratch) {
		panic("e.rr.Read truncated data")
	}
	newSnip := key.Snippet{
		Range: key.Range{
			Start: e.ime.start,
			End:   e.ime.end,
		},
		Text: e.ime.snippet.Text,
	}
	if string(scratch) != newSnip.Text {
		newSnip.Text = string(scratch)
	}
	if newSnip == e.ime.snippet {
		return
	}
	e.ime.snippet = newSnip
	gtx.Execute(key.SnippetCmd{Tag: e, Snippet: newSnip})
}

func (e *Editor) layout(gtx layout.Context, textMaterial, selectMaterial op.CallOp) layout.Dimensions {
	// Adjust scrolling for new viewport and layout.
	e.text.ScrollRel(0, 0)

	if e.scrollCaret {
		e.scrollCaret = false
		e.text.ScrollToCaret()
	}
	visibleDims := e.text.Dimensions()

	defer clip.Rect(image.Rectangle{Max: visibleDims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorText.Add(gtx.Ops)
	event.Op(gtx.Ops, e)
	key.InputHintOp{Tag: e, Hint: e.InputHint}.Add(gtx.Ops)

	e.scroller.Add(gtx.Ops)

	e.clicker.Add(gtx.Ops)
	e.dragger.Add(gtx.Ops)
	e.showCaret = false
	if gtx.Focused(e) {
		now := gtx.Now
		dt := now.Sub(e.blinkStart)
		blinking := dt < maxBlinkDuration
		const timePerBlink = time.Second / blinksPerSecond
		nextBlink := now.Add(timePerBlink/2 - dt%(timePerBlink/2))
		if blinking {
			gtx.Execute(op.InvalidateCmd{At: nextBlink})
		}
		e.showCaret = !blinking || dt%timePerBlink < timePerBlink/2
	}
	semantic.Editor.Add(gtx.Ops)
	if e.Len() > 0 {
		e.paintSelection(gtx, selectMaterial)
		e.paintText(gtx, textMaterial)
	}
	if gtx.Enabled() {
		e.paintCaret(gtx, textMaterial)
	}
	return visibleDims
}

// paintSelection paints the contrasting background for selected text using the provided
// material to set the painting material for the selection.
func (e *Editor) paintSelection(gtx layout.Context, material op.CallOp) {
	e.initBuffer()
	if !gtx.Focused(e) {
		return
	}
	e.text.PaintSelection(gtx, material)
}

// paintText paints the text glyphs using the provided material to set the fill of the
// glyphs.
func (e *Editor) paintText(gtx layout.Context, material op.CallOp) {
	e.initBuffer()
	e.text.PaintText(gtx, material)
}

// paintCaret paints the text glyphs using the provided material to set the fill material
// of the caret rectangle.
func (e *Editor) paintCaret(gtx layout.Context, material op.CallOp) {
	e.initBuffer()
	if !e.showCaret || e.ReadOnly {
		return
	}
	e.text.PaintCaret(gtx, material)
}

// Len is the length of the editor contents, in runes.
func (e *Editor) Len() int {
	e.initBuffer()
	return e.text.Len()
}

// Text returns the contents of the editor.
func (e *Editor) Text() string {
	e.initBuffer()
	e.scratch = e.text.Text(e.scratch)
	return string(e.scratch)
}

func (e *Editor) SetText(s string) {
	e.initBuffer()
	if e.SingleLine {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	e.replace(0, e.text.Len(), s, true)
	// Reset xoff and move the caret to the beginning.
	e.SetCaret(0, 0)
}

// CaretPos returns the line & column numbers of the caret.
func (e *Editor) CaretPos() (line, col int) {
	e.initBuffer()
	return e.text.CaretPos()
}

// CaretCoords returns the coordinates of the caret, relative to the
// editor itself.
func (e *Editor) CaretCoords() f32.Point {
	e.initBuffer()
	return e.text.CaretCoords()
}

// Delete runes from the caret position. The sign of the argument specifies the
// direction to delete: positive is forward, negative is backward.
//
// If there is a selection, it is deleted and counts as a single grapheme
// cluster.
func (e *Editor) Delete(graphemeClusters int) (deletedRunes int) {
	e.initBuffer()
	if graphemeClusters == 0 {
		return 0
	}

	start, end := e.text.Selection()
	if start != end {
		graphemeClusters -= sign(graphemeClusters)
	}

	// Move caret by the target quantity of clusters.
	e.text.MoveCaret(0, graphemeClusters)
	// Get the new rune offsets of the selection.
	start, end = e.text.Selection()
	e.replace(start, end, "", true)
	// Reset xoff.
	e.text.MoveCaret(0, 0)
	e.ClearSelection()
	return end - start
}

func (e *Editor) Insert(s string) (insertedRunes int) {
	e.initBuffer()
	if e.SingleLine {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	start, end := e.text.Selection()
	moves := e.replace(start, end, s, true)
	if end < start {
		start = end
	}
	// Reset xoff.
	e.text.MoveCaret(0, 0)
	e.SetCaret(start+moves, start+moves)
	e.scrollCaret = true
	return moves
}

// modification represents a change to the contents of the editor buffer.
// It contains the necessary information to both apply the change and
// reverse it, and is useful for implementing undo/redo.
type modification struct {
	// StartRune is the inclusive index of the first rune
	// modified.
	StartRune int
	// ApplyContent is the data inserted at StartRune to
	// apply this operation. It overwrites len([]rune(ReverseContent)) runes.
	ApplyContent string
	// ReverseContent is the data inserted at StartRune to
	// apply this operation. It overwrites len([]rune(ApplyContent)) runes.
	ReverseContent string
}

// undo applies the modification at e.history[e.historyIdx] and decrements
// e.historyIdx.
func (e *Editor) undo() (EditorEvent, bool) {
	e.initBuffer()
	if len(e.history) < 1 || e.nextHistoryIdx == 0 {
		return nil, false
	}
	mod := e.history[e.nextHistoryIdx-1]
	replaceEnd := mod.StartRune + utf8.RuneCountInString(mod.ApplyContent)
	e.replace(mod.StartRune, replaceEnd, mod.ReverseContent, false)
	caretEnd := mod.StartRune + utf8.RuneCountInString(mod.ReverseContent)
	e.SetCaret(caretEnd, mod.StartRune)
	e.nextHistoryIdx--
	return ChangeEvent{}, true
}

// redo applies the modification at e.history[e.historyIdx] and increments
// e.historyIdx.
func (e *Editor) redo() (EditorEvent, bool) {
	e.initBuffer()
	if len(e.history) < 1 || e.nextHistoryIdx == len(e.history) {
		return nil, false
	}
	mod := e.history[e.nextHistoryIdx]
	end := mod.StartRune + utf8.RuneCountInString(mod.ReverseContent)
	e.replace(mod.StartRune, end, mod.ApplyContent, false)
	caretEnd := mod.StartRune + utf8.RuneCountInString(mod.ApplyContent)
	e.SetCaret(caretEnd, mod.StartRune)
	e.nextHistoryIdx++
	return ChangeEvent{}, true
}

// replace the text between start and end with s. Indices are in runes.
// It returns the number of runes inserted.
// addHistory controls whether this modification is recorded in the undo
// history. replace can modify text in positions unrelated to the cursor
// position.
func (e *Editor) replace(start, end int, s string, addHistory bool) int {
	length := e.text.Len()
	if start > end {
		start, end = end, start
	}
	start = min(start, length)
	end = min(end, length)
	replaceSize := end - start
	el := e.Len()
	var sc int
	idx := 0
	for idx < len(s) {
		if e.MaxLen > 0 && el-replaceSize+sc >= e.MaxLen {
			s = s[:idx]
			break
		}
		_, n := utf8.DecodeRuneInString(s[idx:])
		if e.Filter != "" && !strings.Contains(e.Filter, s[idx:idx+n]) {
			s = s[:idx] + s[idx+n:]
			continue
		}
		idx += n
		sc++
	}

	if addHistory {
		deleted := make([]rune, 0, replaceSize)
		readPos := e.text.ByteOffset(start)
		for i := 0; i < replaceSize; i++ {
			ru, s, _ := e.text.ReadRuneAt(int64(readPos))
			readPos += int64(s)
			deleted = append(deleted, ru)
		}
		if e.nextHistoryIdx < len(e.history) {
			e.history = e.history[:e.nextHistoryIdx]
		}
		e.history = append(e.history, modification{
			StartRune:      start,
			ApplyContent:   s,
			ReverseContent: string(deleted),
		})
		e.nextHistoryIdx++
	}

	sc = e.text.Replace(start, end, s)
	newEnd := start + sc
	adjust := func(pos int) int {
		switch {
		case newEnd < pos && pos <= end:
			pos = newEnd
		case end < pos:
			diff := newEnd - end
			pos = pos + diff
		}
		return pos
	}
	e.ime.start = adjust(e.ime.start)
	e.ime.end = adjust(e.ime.end)
	return sc
}

// MoveCaret moves the caret (aka selection start) and the selection end
// relative to their current positions. Positive distances moves forward,
// negative distances moves backward. Distances are in grapheme clusters,
// which closely match what users perceive as "characters" even when the
// characters are multiple code points long.
func (e *Editor) MoveCaret(startDelta, endDelta int) {
	e.initBuffer()
	e.text.MoveCaret(startDelta, endDelta)
}

// deleteWord deletes the next word(s) in the specified direction.
// Unlike moveWord, deleteWord treats whitespace as a word itself.
// Positive is forward, negative is backward.
// Absolute values greater than one will delete that many words.
// The selection counts as a single word.
func (e *Editor) deleteWord(distance int) (deletedRunes int) {
	if distance == 0 {
		return
	}

	start, end := e.text.Selection()
	if start != end {
		deletedRunes = e.Delete(1)
		distance -= sign(distance)
	}
	if distance == 0 {
		return deletedRunes
	}

	// split the distance information into constituent parts to be
	// used independently.
	words, direction := distance, 1
	if distance < 0 {
		words, direction = distance*-1, -1
	}
	caret, _ := e.text.Selection()
	// atEnd if offset is at or beyond either side of the buffer.
	atEnd := func(runes int) bool {
		idx := caret + runes*direction
		return idx <= 0 || idx >= e.Len()
	}
	// next returns the appropriate rune given the direction and offset in runes).
	next := func(runes int) rune {
		idx := caret + runes*direction
		if idx < 0 {
			idx = 0
		} else if idx > e.Len() {
			idx = e.Len()
		}
		off := e.text.ByteOffset(idx)
		var r rune
		if direction < 0 {
			r, _, _ = e.text.ReadRuneBefore(int64(off))
		} else {
			r, _, _ = e.text.ReadRuneAt(int64(off))
		}
		return r
	}
	runes := 1
	for ii := 0; ii < words; ii++ {
		r := next(runes)
		wantSpace := unicode.IsSpace(r)
		for r := next(runes); unicode.IsSpace(r) == wantSpace && !atEnd(runes); r = next(runes) {
			runes += 1
		}
	}
	deletedRunes += e.Delete(runes * direction)
	return deletedRunes
}

// SelectionLen returns the length of the selection, in runes; it is
// equivalent to utf8.RuneCountInString(e.SelectedText()).
func (e *Editor) SelectionLen() int {
	e.initBuffer()
	return e.text.SelectionLen()
}

// Selection returns the start and end of the selection, as rune offsets.
// start can be > end.
func (e *Editor) Selection() (start, end int) {
	e.initBuffer()
	return e.text.Selection()
}

// SetCaret moves the caret to start, and sets the selection end to end. start
// and end are in runes, and represent offsets into the editor text.
func (e *Editor) SetCaret(start, end int) {
	e.initBuffer()
	e.text.SetCaret(start, end)
	e.scrollCaret = true
	e.scroller.Stop()
}

// SelectedText returns the currently selected text (if any) from the editor.
func (e *Editor) SelectedText() string {
	e.initBuffer()
	e.scratch = e.text.SelectedText(e.scratch)
	return string(e.scratch)
}

// ClearSelection clears the selection, by setting the selection end equal to
// the selection start.
func (e *Editor) ClearSelection() {
	e.initBuffer()
	e.text.ClearSelection()
}

// WriteTo implements io.WriterTo.
func (e *Editor) WriteTo(w io.Writer) (int64, error) {
	e.initBuffer()
	return e.text.WriteTo(w)
}

// Seek implements io.Seeker.
func (e *Editor) Seek(offset int64, whence int) (int64, error) {
	e.initBuffer()
	return e.text.Seek(offset, whence)
}

// Read implements io.Reader.
func (e *Editor) Read(p []byte) (int, error) {
	e.initBuffer()
	return e.text.Read(p)
}

// Regions returns visible regions covering the rune range [start,end).
func (e *Editor) Regions(start, end int, regions []Region) []Region {
	e.initBuffer()
	return e.text.Regions(start, end, regions)
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

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

func (s ChangeEvent) isEditorEvent() {}
func (s SubmitEvent) isEditorEvent() {}
func (s SelectEvent) isEditorEvent() {}
