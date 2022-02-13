// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"bufio"
	"bytes"
	"image"
	"io"
	"math"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"golang.org/x/image/math/fixed"
)

// Editor implements an editable and scrollable text area.
type Editor struct {
	Alignment text.Alignment
	// SingleLine force the text to stay on a single line.
	// SingleLine also sets the scrolling direction to
	// horizontal.
	SingleLine bool
	// Submit enabled translation of carriage return keys to SubmitEvents.
	// If not enabled, carriage returns are inserted as newlines in the text.
	Submit bool
	// Mask replaces the visual display of each rune in the contents with the given rune.
	// Newline characters are not masked. When non-zero, the unmasked contents
	// are accessed by Len, Text, and SetText.
	Mask rune
	// InputHint specifies the type of on-screen keyboard to be displayed.
	InputHint key.InputHint

	eventKey     int
	font         text.Font
	shaper       text.Shaper
	textSize     fixed.Int26_6
	blinkStart   time.Time
	focused      bool
	rr           editBuffer
	maskReader   maskReader
	lastMask     rune
	maxWidth     int
	viewSize     image.Point
	valid        bool
	lines        []text.Line
	shapes       []line
	dims         layout.Dimensions
	requestFocus bool

	// index tracks combined caret positions at regularly
	// spaced intervals to speed up caret seeking.
	index []combinedPos

	// ime tracks the state relevant to input methods.
	ime struct {
		imeState
		scratch []byte
	}

	caret struct {
		on     bool
		scroll bool
		// xoff is the offset to the current position when moving between lines.
		xoff fixed.Int26_6
		// start is the current caret position in runes, and also the start position of
		// selected text. end is the end position of selected text. If start
		// == end, then there's no selection. Note that it's possible (and
		// common) that the caret (start) is after the end, e.g. after
		// Shift-DownArrow.
		start int
		end   int
	}

	dragging  bool
	dragger   gesture.Drag
	scroller  gesture.Scroll
	scrollOff image.Point

	clicker gesture.Click

	// events is the list of events not yet processed.
	events []EditorEvent
	// prevEvents is the number of events from the previous frame.
	prevEvents int
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

// combinedPos is a point in the editor.
type combinedPos struct {
	// ofs is the offset into the editorBuffer in bytes. The other three fields are based off of this one.
	ofs int
	// runes is the offset in runes.
	runes int

	// lineCol.Y = line (offset into Editor.lines), and X = col (offset into
	// Editor.lines[Y])
	lineCol screenPos

	// Pixel coordinates
	x fixed.Int26_6
	y int
}

type selectionAction int

const (
	selectionExtend selectionAction = iota
	selectionClear
)

func (m *maskReader) Reset(r io.RuneReader, mr rune) {
	m.rr = r
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

type line struct {
	offset         image.Point
	clip           clip.Op
	selected       bool
	selectionYOffs int
	selectionSize  image.Point
}

const (
	blinksPerSecond  = 1
	maxBlinkDuration = 10 * time.Second
)

// Events returns available editor events.
func (e *Editor) Events() []EditorEvent {
	events := e.events
	e.events = nil
	e.prevEvents = 0
	return events
}

func (e *Editor) processEvents(gtx layout.Context) {
	// Flush events from before the previous Layout.
	n := copy(e.events, e.events[e.prevEvents:])
	e.events = e.events[:n]
	e.prevEvents = n

	if e.shaper == nil {
		// Can't process events without a shaper.
		return
	}
	oldStart, oldLen := min(e.caret.start, e.caret.end), e.SelectionLen()
	e.processPointer(gtx)
	e.processKey(gtx)
	// Queue a SelectEvent if the selection changed, including if it went away.
	if newStart, newLen := min(e.caret.start, e.caret.end), e.SelectionLen(); oldStart != newStart || oldLen != newLen {
		e.events = append(e.events, SelectEvent{})
	}
}

func (e *Editor) makeValid() {
	if e.valid {
		return
	}
	e.lines, e.dims = e.layoutText(e.shaper)
	e.valid = true
}

func (e *Editor) processPointer(gtx layout.Context) {
	sbounds := e.scrollBounds()
	var smin, smax int
	var axis gesture.Axis
	if e.SingleLine {
		axis = gesture.Horizontal
		smin, smax = sbounds.Min.X, sbounds.Max.X
	} else {
		axis = gesture.Vertical
		smin, smax = sbounds.Min.Y, sbounds.Max.Y
	}
	sdist := e.scroller.Scroll(gtx.Metric, gtx, gtx.Now, axis)
	var soff int
	if e.SingleLine {
		e.scrollRel(sdist, 0)
		soff = e.scrollOff.X
	} else {
		e.scrollRel(0, sdist)
		soff = e.scrollOff.Y
	}
	for _, evt := range e.clickDragEvents(gtx) {
		switch evt := evt.(type) {
		case gesture.ClickEvent:
			switch {
			case evt.Type == gesture.TypePress && evt.Source == pointer.Mouse,
				evt.Type == gesture.TypeClick:
				prevCaretPos := e.caret.start
				e.blinkStart = gtx.Now
				e.moveCoord(image.Point{
					X: int(math.Round(float64(evt.Position.X))),
					Y: int(math.Round(float64(evt.Position.Y))),
				})
				e.requestFocus = true
				if e.scroller.State() != gesture.StateFlinging {
					e.caret.scroll = true
				}

				if evt.Modifiers == key.ModShift {
					// If they clicked closer to the end, then change the end to
					// where the caret used to be (effectively swapping start & end).
					if abs(e.caret.end-e.caret.start) < abs(e.caret.start-prevCaretPos) {
						e.caret.end = prevCaretPos
					}
				} else {
					e.ClearSelection()
				}
				e.dragging = true

				// Process a double-click.
				if evt.NumClicks == 2 {
					e.moveWord(-1, selectionClear)
					e.moveWord(1, selectionExtend)
					e.dragging = false
				}
			}
		case pointer.Event:
			release := false
			switch {
			case evt.Type == pointer.Release && evt.Source == pointer.Mouse:
				release = true
				fallthrough
			case evt.Type == pointer.Drag && evt.Source == pointer.Mouse:
				if e.dragging {
					e.blinkStart = gtx.Now
					e.moveCoord(image.Point{
						X: int(math.Round(float64(evt.Position.X))),
						Y: int(math.Round(float64(evt.Position.Y))),
					})
					e.caret.scroll = true

					if release {
						e.dragging = false
					}
				}
			}
		}
	}

	if (sdist > 0 && soff >= smax) || (sdist < 0 && soff <= smin) {
		e.scroller.Stop()
	}
}

func (e *Editor) clickDragEvents(gtx layout.Context) []event.Event {
	var combinedEvents []event.Event
	for _, evt := range e.clicker.Events(gtx) {
		combinedEvents = append(combinedEvents, evt)
	}
	for _, evt := range e.dragger.Events(gtx.Metric, gtx, gesture.Both) {
		combinedEvents = append(combinedEvents, evt)
	}
	return combinedEvents
}

func (e *Editor) processKey(gtx layout.Context) {
	if e.rr.Changed() {
		e.events = append(e.events, ChangeEvent{})
	}
	for _, ke := range gtx.Events(&e.eventKey) {
		e.blinkStart = gtx.Now
		switch ke := ke.(type) {
		case key.FocusEvent:
			e.focused = ke.Focus
			// Reset IME state.
			e.ime.imeState = imeState{}
		case key.Event:
			if !e.focused || ke.State != key.Press {
				break
			}
			if e.Submit && (ke.Name == key.NameReturn || ke.Name == key.NameEnter) {
				if !ke.Modifiers.Contain(key.ModShift) {
					e.events = append(e.events, SubmitEvent{
						Text: e.Text(),
					})
					continue
				}
			}
			if e.command(gtx, ke) {
				e.caret.scroll = true
				e.scroller.Stop()
			}
		case key.SnippetEvent:
			e.updateSnippet(gtx, ke.Start, ke.End)
		case key.EditEvent:
			e.caret.scroll = true
			e.scroller.Stop()
			e.replace(ke.Range.Start, ke.Range.End, ke.Text)
			e.caret.xoff = 0
		// Complete a paste event, initiated by Shortcut-V in Editor.command().
		case clipboard.Event:
			e.caret.scroll = true
			e.scroller.Stop()
			e.append(ke.Text)
		case key.SelectionEvent:
			e.caret.scroll = true
			e.scroller.Stop()
			e.caret.start = e.closestPosition(combinedPos{runes: ke.Start}).runes
			e.caret.end = e.closestPosition(combinedPos{runes: ke.End}).runes
		}
		if e.rr.Changed() {
			e.events = append(e.events, ChangeEvent{})
		}
	}
}

func (e *Editor) moveLines(distance int, selAct selectionAction) {
	caretStart := e.closestPosition(combinedPos{runes: e.caret.start})
	x := caretStart.x + e.caret.xoff
	// Seek to line.
	pos := e.closestPosition(combinedPos{lineCol: screenPos{Y: caretStart.lineCol.Y + distance}})
	pos = e.closestPosition(combinedPos{x: x, y: pos.y})
	e.caret.start = pos.runes
	e.caret.xoff = x - caretStart.x
	e.updateSelection(selAct)
}

func (e *Editor) command(gtx layout.Context, k key.Event) bool {
	modSkip := key.ModCtrl
	if runtime.GOOS == "darwin" {
		modSkip = key.ModAlt
	}
	moveByWord := k.Modifiers.Contain(modSkip)
	selAct := selectionClear
	if k.Modifiers.Contain(key.ModShift) {
		selAct = selectionExtend
	}
	switch k.Name {
	case key.NameReturn, key.NameEnter:
		e.append("\n")
	case key.NameDeleteBackward:
		if moveByWord {
			e.deleteWord(-1)
		} else {
			e.Delete(-1)
		}
	case key.NameDeleteForward:
		if moveByWord {
			e.deleteWord(1)
		} else {
			e.Delete(1)
		}
	case key.NameUpArrow:
		e.moveLines(-1, selAct)
	case key.NameDownArrow:
		e.moveLines(+1, selAct)
	case key.NameLeftArrow:
		if moveByWord {
			e.moveWord(-1, selAct)
		} else {
			if selAct == selectionClear {
				e.ClearSelection()
			}
			e.MoveCaret(-1, -1*int(selAct))
		}
	case key.NameRightArrow:
		if moveByWord {
			e.moveWord(1, selAct)
		} else {
			if selAct == selectionClear {
				e.ClearSelection()
			}
			e.MoveCaret(1, int(selAct))
		}
	case key.NamePageUp:
		e.movePages(-1, selAct)
	case key.NamePageDown:
		e.movePages(+1, selAct)
	case key.NameHome:
		e.moveStart(selAct)
	case key.NameEnd:
		e.moveEnd(selAct)
	// Initiate a paste operation, by requesting the clipboard contents; other
	// half is in Editor.processKey() under clipboard.Event.
	case "V":
		if k.Modifiers != key.ModShortcut {
			return false
		}
		clipboard.ReadOp{Tag: &e.eventKey}.Add(gtx.Ops)
	// Copy or Cut selection -- ignored if nothing selected.
	case "C", "X":
		if k.Modifiers != key.ModShortcut {
			return false
		}
		if text := e.SelectedText(); text != "" {
			clipboard.WriteOp{Text: text}.Add(gtx.Ops)
			if k.Name == "X" {
				e.Delete(1)
			}
		}
	// Select all
	case "A":
		if k.Modifiers != key.ModShortcut {
			return false
		}
		e.caret.end = 0
		e.caret.start = e.Len()
	default:
		return false
	}
	return true
}

// Focus requests the input focus for the Editor.
func (e *Editor) Focus() {
	e.requestFocus = true
}

// Focused returns whether the editor is focused or not.
func (e *Editor) Focused() bool {
	return e.focused
}

// Layout lays out the editor. If content is not nil, it is laid out on top.
func (e *Editor) Layout(gtx layout.Context, sh text.Shaper, font text.Font, size unit.Value, content layout.Widget) layout.Dimensions {
	textSize := fixed.I(gtx.Px(size))
	if e.font != font || e.textSize != textSize {
		e.invalidate()
		e.font = font
		e.textSize = textSize
	}
	maxWidth := gtx.Constraints.Max.X
	if e.SingleLine {
		maxWidth = inf
	}
	if maxWidth != e.maxWidth {
		e.maxWidth = maxWidth
		e.invalidate()
	}
	if sh != e.shaper {
		e.shaper = sh
		e.invalidate()
	}
	if e.Mask != e.lastMask {
		e.lastMask = e.Mask
		e.invalidate()
	}

	e.makeValid()
	e.processEvents(gtx)
	e.makeValid()

	if viewSize := gtx.Constraints.Constrain(e.dims.Size); viewSize != e.viewSize {
		e.viewSize = viewSize
		e.invalidate()
	}
	e.makeValid()

	dims := e.layout(gtx, content)

	if e.focused {
		// Notify IME of selection if it changed.
		newSel := e.ime.selection
		newSel.rng = key.Range{
			Start: e.caret.start,
			End:   e.caret.end,
		}
		caretPos, carAsc, carDesc := e.caretInfo()
		newSel.caret = key.Caret{
			Pos:     layout.FPt(caretPos),
			Ascent:  float32(carAsc),
			Descent: float32(carDesc),
		}
		if newSel != e.ime.selection {
			e.ime.selection = newSel
			key.SelectionOp{
				Tag:   &e.eventKey,
				Range: newSel.rng,
				Caret: newSel.caret,
			}.Add(gtx.Ops)
		}

		e.updateSnippet(gtx, e.ime.start, e.ime.end)
	}

	return dims
}

// updateSnippet adds a key.SnippetOp if the snippet content or position
// have changed. off and len are in runes.
func (e *Editor) updateSnippet(gtx layout.Context, start, end int) {
	if start > end {
		start, end = end, start
	}
	imeStart := e.closestPosition(combinedPos{runes: start})
	imeEnd := e.closestPosition(combinedPos{runes: end})
	e.ime.start = imeStart.runes
	e.ime.end = imeEnd.runes
	e.rr.Seek(int64(imeStart.ofs), io.SeekStart)
	n := imeEnd.ofs - imeStart.ofs
	if n > len(e.ime.scratch) {
		e.ime.scratch = make([]byte, n)
	}
	scratch := e.ime.scratch[:n]
	read, _ := e.rr.Read(scratch)
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
	key.SnippetOp{
		Tag:     &e.eventKey,
		Snippet: newSnip,
	}.Add(gtx.Ops)
}

func (e *Editor) layout(gtx layout.Context, content layout.Widget) layout.Dimensions {
	// Adjust scrolling for new viewport and layout.
	e.scrollRel(0, 0)

	if e.caret.scroll {
		e.caret.scroll = false
		e.scrollToCaret()
	}

	off := image.Point{
		X: -e.scrollOff.X,
		Y: -e.scrollOff.Y,
	}
	cl := textPadding(e.lines)
	cl.Max = cl.Max.Add(e.viewSize)
	caretStart := e.closestPosition(combinedPos{runes: e.caret.start})
	caretEnd := e.closestPosition(combinedPos{runes: e.caret.end})
	startSel, endSel := sortPoints(caretStart.lineCol, caretEnd.lineCol)
	it := segmentIterator{
		startSel:  startSel,
		endSel:    endSel,
		Lines:     e.lines,
		Clip:      cl,
		Alignment: e.Alignment,
		Width:     e.viewSize.X,
		Offset:    off,
	}
	e.shapes = e.shapes[:0]
	for {
		layout, off, selected, yOffs, size, ok := it.Next()
		if !ok {
			break
		}
		op := clip.Outline{Path: e.shaper.Shape(e.font, e.textSize, layout)}.Op()
		e.shapes = append(e.shapes, line{off, op, selected, yOffs, size})
	}

	key.InputOp{Tag: &e.eventKey, Hint: e.InputHint}.Add(gtx.Ops)
	if e.requestFocus {
		key.FocusOp{Tag: &e.eventKey}.Add(gtx.Ops)
		key.SoftKeyboardOp{Show: true}.Add(gtx.Ops)
	}
	e.requestFocus = false
	pointerPadding := gtx.Px(unit.Dp(4))
	r := image.Rectangle{Max: e.viewSize}
	r.Min.X -= pointerPadding
	r.Min.Y -= pointerPadding
	r.Max.X += pointerPadding
	r.Max.X += pointerPadding
	defer clip.Rect(r).Push(gtx.Ops).Pop()
	pointer.CursorNameOp{Name: pointer.CursorText}.Add(gtx.Ops)

	var scrollRange image.Rectangle
	if e.SingleLine {
		scrollRange.Min.X = min(-e.scrollOff.X, 0)
		scrollRange.Max.X = max(0, e.dims.Size.X-(e.scrollOff.X+e.viewSize.X))
	} else {
		scrollRange.Min.Y = -e.scrollOff.Y
		scrollRange.Max.Y = max(0, e.dims.Size.Y-(e.scrollOff.Y+e.viewSize.Y))
	}
	e.scroller.Add(gtx.Ops, scrollRange)

	e.clicker.Add(gtx.Ops)
	e.dragger.Add(gtx.Ops)
	e.caret.on = false
	if e.focused {
		now := gtx.Now
		dt := now.Sub(e.blinkStart)
		blinking := dt < maxBlinkDuration
		const timePerBlink = time.Second / blinksPerSecond
		nextBlink := now.Add(timePerBlink/2 - dt%(timePerBlink/2))
		if blinking {
			redraw := op.InvalidateOp{At: nextBlink}
			redraw.Add(gtx.Ops)
		}
		e.caret.on = e.focused && (!blinking || dt%timePerBlink < timePerBlink/2)
	}

	if content != nil {
		content(gtx)
	}
	return layout.Dimensions{Size: e.viewSize, Baseline: e.dims.Baseline}
}

// PaintSelection paints the contrasting background for selected text.
func (e *Editor) PaintSelection(gtx layout.Context) {
	cl := textPadding(e.lines)
	cl.Max = cl.Max.Add(e.viewSize)
	defer clip.Rect(cl).Push(gtx.Ops).Pop()
	for _, shape := range e.shapes {
		if !shape.selected {
			continue
		}
		offset := shape.offset
		offset.Y += shape.selectionYOffs
		t := op.Offset(layout.FPt(offset)).Push(gtx.Ops)
		cl := clip.Rect(image.Rectangle{Max: shape.selectionSize}).Push(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		cl.Pop()
		t.Pop()
	}
}

func (e *Editor) PaintText(gtx layout.Context) {
	cl := textPadding(e.lines)
	cl.Max = cl.Max.Add(e.viewSize)
	defer clip.Rect(cl).Push(gtx.Ops).Pop()
	for _, shape := range e.shapes {
		t := op.Offset(layout.FPt(shape.offset)).Push(gtx.Ops)
		cl := shape.clip.Push(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		cl.Pop()
		t.Pop()
	}
}

func (e *Editor) PaintCaret(gtx layout.Context) {
	if !e.caret.on {
		return
	}
	carWidth2 := gtx.Px(unit.Dp(1)) / 2
	if carWidth2 < 1 {
		carWidth2 = 1
	}
	caretPos, carAsc, carDesc := e.caretInfo()

	carRect := image.Rectangle{
		Min: caretPos.Sub(image.Pt(carWidth2, carAsc)),
		Max: caretPos.Add(image.Pt(carWidth2, carDesc)),
	}
	cl := textPadding(e.lines)
	// Account for caret width to each side.
	if cl.Max.X < carWidth2 {
		cl.Max.X = carWidth2
	}
	if cl.Min.X > -carWidth2 {
		cl.Min.X = -carWidth2
	}
	cl.Max = cl.Max.Add(e.viewSize)
	carRect = cl.Intersect(carRect)
	if !carRect.Empty() {
		defer clip.Rect(carRect).Push(gtx.Ops).Pop()
		paint.PaintOp{}.Add(gtx.Ops)
	}
}

func (e *Editor) caretInfo() (pos image.Point, ascent, descent int) {
	caretStart := e.closestPosition(combinedPos{runes: e.caret.start})
	carX := caretStart.x
	carY := caretStart.y

	ascent = -e.lines[caretStart.lineCol.Y].Bounds.Min.Y.Ceil()
	descent = e.lines[caretStart.lineCol.Y].Bounds.Max.Y.Ceil()
	pos = image.Point{
		X: carX.Round(),
		Y: carY,
	}
	pos = pos.Sub(e.scrollOff)
	return
}

// TODO: copied from package math. Remove when Go 1.18 is minimum.
const (
	intSize = 32 << (^uint(0) >> 63) // 32 or 64
	maxInt  = 1<<(intSize-1) - 1
)

// Len is the length of the editor contents, in runes.
func (e *Editor) Len() int {
	end := e.closestPosition(combinedPos{runes: maxInt})
	return end.runes
}

// Text returns the contents of the editor.
func (e *Editor) Text() string {
	return e.rr.String()
}

// SetText replaces the contents of the editor, clearing any selection first.
func (e *Editor) SetText(s string) {
	e.rr = editBuffer{}
	e.caret.start = 0
	e.caret.end = 0
	e.replace(e.caret.start, e.caret.end, s)
	e.caret.xoff = 0
}

func (e *Editor) scrollBounds() image.Rectangle {
	var b image.Rectangle
	if e.SingleLine {
		if len(e.lines) > 0 {
			b.Min.X = align(e.Alignment, e.lines[0].Width, e.viewSize.X).Floor()
			if b.Min.X > 0 {
				b.Min.X = 0
			}
		}
		b.Max.X = e.dims.Size.X + b.Min.X - e.viewSize.X
	} else {
		b.Max.Y = e.dims.Size.Y - e.viewSize.Y
	}
	return b
}

func (e *Editor) scrollRel(dx, dy int) {
	e.scrollAbs(e.scrollOff.X+dx, e.scrollOff.Y+dy)
}

func (e *Editor) scrollAbs(x, y int) {
	e.scrollOff.X = x
	e.scrollOff.Y = y
	b := e.scrollBounds()
	if e.scrollOff.X > b.Max.X {
		e.scrollOff.X = b.Max.X
	}
	if e.scrollOff.X < b.Min.X {
		e.scrollOff.X = b.Min.X
	}
	if e.scrollOff.Y > b.Max.Y {
		e.scrollOff.Y = b.Max.Y
	}
	if e.scrollOff.Y < b.Min.Y {
		e.scrollOff.Y = b.Min.Y
	}
}

func (e *Editor) moveCoord(pos image.Point) {
	x := fixed.I(pos.X + e.scrollOff.X)
	y := pos.Y + e.scrollOff.Y
	e.caret.start = e.closestPosition(combinedPos{x: x, y: y}).runes
	e.caret.xoff = 0
}

func (e *Editor) layoutText(s text.Shaper) ([]text.Line, layout.Dimensions) {
	e.rr.Reset()
	var r io.Reader = &e.rr
	if e.Mask != 0 {
		e.maskReader.Reset(&e.rr, e.Mask)
		r = &e.maskReader
	}
	var lines []text.Line
	if s != nil {
		lines, _ = s.Layout(e.font, e.textSize, e.maxWidth, r)
	} else {
		lines, _ = nullLayout(r)
	}
	dims := linesDimens(lines)
	for i := 0; i < len(lines)-1; i++ {
		// To avoid layout flickering while editing, assume a soft newline takes
		// up all available space.
		if layout := lines[i].Layout; len(layout.Text) > 0 {
			r := layout.Text[len(layout.Text)-1]
			if r != '\n' {
				dims.Size.X = e.maxWidth
				break
			}
		}
	}
	return lines, dims
}

// CaretPos returns the line & column numbers of the caret.
func (e *Editor) CaretPos() (line, col int) {
	caret := e.closestPosition(combinedPos{runes: e.caret.start})
	return caret.lineCol.Y, caret.lineCol.X
}

// CaretCoords returns the coordinates of the caret, relative to the
// editor itself.
func (e *Editor) CaretCoords() f32.Point {
	caret := e.closestPosition(combinedPos{runes: e.caret.start})
	return f32.Pt(float32(caret.x)/64, float32(caret.y))
}

// indexPosition returns the latest position from the index no later than pos.
func (e *Editor) indexPosition(pos combinedPos) combinedPos {
	e.makeValid()
	// Initialize index with first caret position.
	if len(e.index) == 0 {
		l := e.lines[0]
		e.index = append(e.index, combinedPos{
			x: align(e.Alignment, l.Width, e.viewSize.X),
			y: l.Ascent.Ceil(),
		})
	}
	i := sort.Search(len(e.index), func(i int) bool {
		return e.positionGreaterOrEqual(e.index[i], pos)
	})
	// Return position just before pos, which is guaranteed to be less than or equal to pos.
	if i > 0 {
		i--
	}
	return e.index[i]
}

// positionGreaterOrEqual reports whether p1 >= p2 according to the non-zero fields
// of p2. All fields of p1 must be a consistent and valid.
func (e *Editor) positionGreaterOrEqual(p1, p2 combinedPos) bool {
	l := e.lines[p1.lineCol.Y]
	endCol := len(l.Layout.Advances) - 1
	if lastLine := p1.lineCol.Y == len(e.lines)-1; lastLine {
		endCol++
	}
	eol := p1.lineCol.X == endCol
	switch {
	case p2.runes != 0:
		return p1.runes >= p2.runes
	case p2.lineCol != (screenPos{}):
		if p1.lineCol.Y != p2.lineCol.Y {
			return p1.lineCol.Y > p2.lineCol.Y
		}
		return eol || p1.lineCol.X >= p2.lineCol.X
	case p2.x != 0 || p2.y != 0:
		ly := p1.y + l.Descent.Ceil()
		prevy := p1.y - l.Ascent.Ceil()
		switch {
		case ly < p2.y && p1.lineCol.Y < len(e.lines)-1:
			// p1 is on a line before p2.y.
			return false
		case prevy >= p2.y && p1.lineCol.Y > 0:
			// p1 is on a line after p2.y.
			return true
		}
		if eol {
			return true
		}
		adv := l.Layout.Advances[p1.lineCol.X]
		return p1.x+adv-p2.x >= p2.x-p1.x
	}
	return true
}

// closestPosition takes a position and returns its closest valid position.
// Zero fields of pos and pos.ofs are ignored.
func (e *Editor) closestPosition(pos combinedPos) combinedPos {
	closest := e.indexPosition(pos)
	l := e.lines[closest.lineCol.Y]
	count := 0
	const runesPerIndexEntry = 50
	// Advance next and prev until next is greater than or equal to pos.
	for {
		for ; closest.lineCol.X < len(l.Layout.Advances); closest.lineCol.X++ {
			if count == runesPerIndexEntry {
				e.index = append(e.index, closest)
				count = 0
			}
			count++
			if e.positionGreaterOrEqual(closest, pos) {
				return closest
			}

			adv := l.Layout.Advances[closest.lineCol.X]
			closest.x += adv
			_, s := e.rr.runeAt(closest.ofs)
			closest.ofs += s
			closest.runes++
		}
		if closest.lineCol.Y == len(e.lines)-1 {
			// End of file.
			return closest
		}

		prevDesc := l.Descent
		closest.lineCol.Y++
		closest.lineCol.X = 0
		l = e.lines[closest.lineCol.Y]
		closest.x = align(e.Alignment, l.Width, e.viewSize.X)
		closest.y += (prevDesc + l.Ascent).Ceil()
	}
}

func (e *Editor) invalidate() {
	e.index = e.index[:0]
	e.valid = false
}

// Delete runes from the caret position. The sign of runes specifies the
// direction to delete: positive is forward, negative is backward.
//
// If there is a selection, it is deleted and counts as a single rune.
func (e *Editor) Delete(runes int) {
	if runes == 0 {
		return
	}

	start := e.caret.start
	end := e.caret.end
	if start != end {
		runes -= sign(runes)
	}

	end += runes
	e.replace(start, end, "")
	e.caret.xoff = 0
	e.ClearSelection()
}

// Insert inserts text at the caret, moving the caret forward. If there is a
// selection, Insert overwrites it.
func (e *Editor) Insert(s string) {
	e.append(s)
	e.caret.scroll = true
}

// append inserts s at the cursor, leaving the caret is at the end of s. If
// there is a selection, append overwrites it.
// xxx|yyy + append zzz => xxxzzz|yyy
func (e *Editor) append(s string) {
	e.replace(e.caret.start, e.caret.end, s)
	e.caret.xoff = 0
	start := e.caret.start
	if end := e.caret.end; end < start {
		start = end
	}
	e.caret.start = start + utf8.RuneCountInString(s)
	e.caret.end = e.caret.start
}

// replace the text between start and end with s. Indices are in runes.
func (e *Editor) replace(start, end int, s string) {
	if e.SingleLine {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	if start > end {
		start, end = end, start
	}
	startPos := e.closestPosition(combinedPos{runes: start})
	endPos := e.closestPosition(combinedPos{runes: end})
	e.rr.deleteRunes(startPos.ofs, endPos.runes-startPos.runes)
	e.rr.prepend(startPos.ofs, s)
	newEnd := startPos.runes + utf8.RuneCountInString(s)
	adjust := func(pos int) int {
		switch {
		case newEnd < pos && pos <= endPos.runes:
			pos = newEnd
		case endPos.runes < pos:
			diff := newEnd - endPos.runes
			pos = pos + diff
		}
		return pos
	}
	e.caret.start = adjust(e.caret.start)
	e.caret.end = adjust(e.caret.end)
	e.ime.start = adjust(e.ime.start)
	e.ime.end = adjust(e.ime.end)
	e.invalidate()
}

func (e *Editor) movePages(pages int, selAct selectionAction) {
	caret := e.closestPosition(combinedPos{runes: e.caret.start})
	x := caret.x + e.caret.xoff
	y := caret.y + pages*e.viewSize.Y
	pos := e.closestPosition(combinedPos{x: x, y: y})
	e.caret.start = pos.runes
	e.caret.xoff = x - pos.x
	e.updateSelection(selAct)
}

// MoveCaret moves the caret (aka selection start) and the selection end
// relative to their current positions. Positive distances moves forward,
// negative distances moves backward. Distances are in runes.
func (e *Editor) MoveCaret(startDelta, endDelta int) {
	e.caret.xoff = 0
	e.caret.start = e.closestPosition(combinedPos{runes: e.caret.start + startDelta}).runes
	e.caret.end = e.closestPosition(combinedPos{runes: e.caret.end + endDelta}).runes
}

func (e *Editor) moveStart(selAct selectionAction) {
	caret := e.closestPosition(combinedPos{runes: e.caret.start})
	caret = e.closestPosition(combinedPos{lineCol: screenPos{Y: caret.lineCol.Y}})
	e.caret.start = caret.runes
	e.caret.xoff = -caret.x
	e.updateSelection(selAct)
}

func (e *Editor) moveEnd(selAct selectionAction) {
	caret := e.closestPosition(combinedPos{runes: e.caret.start})
	caret = e.closestPosition(combinedPos{lineCol: screenPos{X: maxInt, Y: caret.lineCol.Y}})
	e.caret.start = caret.runes
	l := e.lines[caret.lineCol.Y]
	a := align(e.Alignment, l.Width, e.viewSize.X)
	e.caret.xoff = l.Width + a - caret.x
	e.updateSelection(selAct)
}

// moveWord moves the caret to the next word in the specified direction.
// Positive is forward, negative is backward.
// Absolute values greater than one will skip that many words.
func (e *Editor) moveWord(distance int, selAct selectionAction) {
	// split the distance information into constituent parts to be
	// used independently.
	words, direction := distance, 1
	if distance < 0 {
		words, direction = distance*-1, -1
	}
	// atEnd if caret is at either side of the buffer.
	caret := e.closestPosition(combinedPos{runes: e.caret.start})
	atEnd := func() bool {
		return caret.ofs == 0 || caret.ofs == e.rr.len()
	}
	// next returns the appropriate rune given the direction.
	next := func() (r rune) {
		if direction < 0 {
			r, _ = e.rr.runeBefore(caret.ofs)
		} else {
			r, _ = e.rr.runeAt(caret.ofs)
		}
		return r
	}
	for ii := 0; ii < words; ii++ {
		for r := next(); unicode.IsSpace(r) && !atEnd(); r = next() {
			e.MoveCaret(direction, 0)
			caret = e.closestPosition(combinedPos{runes: e.caret.start})
		}
		e.MoveCaret(direction, 0)
		caret = e.closestPosition(combinedPos{runes: e.caret.start})
		for r := next(); !unicode.IsSpace(r) && !atEnd(); r = next() {
			e.MoveCaret(direction, 0)
			caret = e.closestPosition(combinedPos{runes: e.caret.start})
		}
	}
	e.updateSelection(selAct)
}

// deleteWord deletes the next word(s) in the specified direction.
// Unlike moveWord, deleteWord treats whitespace as a word itself.
// Positive is forward, negative is backward.
// Absolute values greater than one will delete that many words.
// The selection counts as a single word.
func (e *Editor) deleteWord(distance int) {
	if distance == 0 {
		return
	}

	if e.caret.start != e.caret.end {
		e.Delete(1)
		distance -= sign(distance)
	}
	if distance == 0 {
		return
	}

	// split the distance information into constituent parts to be
	// used independently.
	words, direction := distance, 1
	if distance < 0 {
		words, direction = distance*-1, -1
	}
	// atEnd if offset is at or beyond either side of the buffer.
	caret := e.closestPosition(combinedPos{runes: e.caret.start})
	atEnd := func(offset int) bool {
		idx := caret.ofs + offset*direction
		return idx <= 0 || idx >= e.rr.len()
	}
	// next returns the appropriate rune and length given the direction and offset (in bytes).
	next := func(offset int) (r rune, l int) {
		idx := caret.ofs + offset*direction
		if idx < 0 {
			idx = 0
		} else if idx > e.rr.len() {
			idx = e.rr.len()
		}
		if direction < 0 {
			r, l = e.rr.runeBefore(idx)
		} else {
			r, l = e.rr.runeAt(idx)
		}
		return
	}
	var runes = 1
	_, bytes := e.rr.runeAt(caret.ofs)
	if direction < 0 {
		_, bytes = e.rr.runeBefore(caret.ofs)
	}
	for ii := 0; ii < words; ii++ {
		if r, _ := next(bytes); unicode.IsSpace(r) {
			for r, lg := next(bytes); unicode.IsSpace(r) && !atEnd(bytes); r, lg = next(bytes) {
				runes += 1
				bytes += lg
			}
		} else {
			for r, lg := next(bytes); !unicode.IsSpace(r) && !atEnd(bytes); r, lg = next(bytes) {
				runes += 1
				bytes += lg
			}
		}
	}
	e.Delete(runes * direction)
}

func (e *Editor) scrollToCaret() {
	caret := e.closestPosition(combinedPos{runes: e.caret.start})
	l := e.lines[caret.lineCol.Y]
	if e.SingleLine {
		var dist int
		if d := caret.x.Floor() - e.scrollOff.X; d < 0 {
			dist = d
		} else if d := caret.x.Ceil() - (e.scrollOff.X + e.viewSize.X); d > 0 {
			dist = d
		}
		e.scrollRel(dist, 0)
	} else {
		miny := caret.y - l.Ascent.Ceil()
		maxy := caret.y + l.Descent.Ceil()
		var dist int
		if d := miny - e.scrollOff.Y; d < 0 {
			dist = d
		} else if d := maxy - (e.scrollOff.Y + e.viewSize.Y); d > 0 {
			dist = d
		}
		e.scrollRel(0, dist)
	}
}

// NumLines returns the number of lines in the editor.
func (e *Editor) NumLines() int {
	e.makeValid()
	return len(e.lines)
}

// SelectionLen returns the length of the selection, in runes; it is
// equivalent to utf8.RuneCountInString(e.SelectedText()).
func (e *Editor) SelectionLen() int {
	return abs(e.caret.start - e.caret.end)
}

// Selection returns the start and end of the selection, as rune offsets.
// start can be > end.
func (e *Editor) Selection() (start, end int) {
	return e.caret.start, e.caret.end
}

// SetCaret moves the caret to start, and sets the selection end to end. start
// and end are in runes, and represent offsets into the editor text.
func (e *Editor) SetCaret(start, end int) {
	e.caret.start = e.closestPosition(combinedPos{runes: start}).runes
	e.caret.end = e.closestPosition(combinedPos{runes: end}).runes
	e.caret.scroll = true
	e.scroller.Stop()
}

// SelectedText returns the currently selected text (if any) from the editor.
func (e *Editor) SelectedText() string {
	caretStart := e.closestPosition(combinedPos{runes: e.caret.start})
	caretEnd := e.closestPosition(combinedPos{runes: e.caret.end})
	start := min(caretStart.ofs, caretEnd.ofs)
	end := max(caretStart.ofs, caretEnd.ofs)
	buf := make([]byte, end-start)
	e.rr.Seek(int64(start), io.SeekStart)
	_, err := e.rr.Read(buf)
	if err != nil {
		// The only error that rr.Read can return is EOF, which just means no
		// selection, but we've already made sure that shouldn't happen.
		panic("impossible error because end is before e.rr.Len()")
	}
	return string(buf)
}

func (e *Editor) updateSelection(selAct selectionAction) {
	if selAct == selectionClear {
		e.ClearSelection()
	}
}

// ClearSelection clears the selection, by setting the selection end equal to
// the selection start.
func (e *Editor) ClearSelection() {
	e.caret.end = e.caret.start
}

// WriteTo implements io.WriterTo.
func (e *Editor) WriteTo(w io.Writer) (int64, error) {
	return e.rr.WriteTo(w)
}

// Seek implements io.Seeker.
func (e *Editor) Seek(offset int64, whence int) (int64, error) {
	return e.rr.Seek(0, io.SeekStart)
}

// Read implements io.Reader.
func (e *Editor) Read(p []byte) (int, error) {
	return e.rr.Read(p)
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

// sortPoints returns a and b sorted such that a2 <= b2.
func sortPoints(a, b screenPos) (a2, b2 screenPos) {
	if b.Less(a) {
		return b, a
	}
	return a, b
}

func nullLayout(r io.Reader) ([]text.Line, error) {
	rr := bufio.NewReader(r)
	var rerr error
	var n int
	var buf bytes.Buffer
	for {
		r, _, err := rr.ReadRune()
		if err != nil {
			rerr = err
			break
		}
		n++
		buf.WriteRune(r)
	}
	return []text.Line{
		{
			Layout: text.Layout{
				Text:     buf.String(),
				Advances: make([]fixed.Int26_6, n),
			},
		},
	}, rerr
}

func (s ChangeEvent) isEditorEvent() {}
func (s SubmitEvent) isEditorEvent() {}
func (s SelectEvent) isEditorEvent() {}
