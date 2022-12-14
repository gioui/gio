// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"io"
	"math"
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
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"

	"golang.org/x/exp/constraints"
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
	// MaxLen limits the editor content to a maximum length. Zero means no limit.
	MaxLen int
	// Filter is the list of characters allowed in the Editor. If Filter is empty,
	// all characters are allowed.
	Filter string

	eventKey     int
	font         text.Font
	shaper       *text.Shaper
	textSize     fixed.Int26_6
	blinkStart   time.Time
	focused      bool
	rr           editBuffer
	maskReader   maskReader
	lastMask     rune
	maxWidth     int
	viewSize     image.Point
	valid        bool
	regions      []region
	dims         layout.Dimensions
	requestFocus bool

	// offIndex is an index of rune index to byte offsets.
	offIndex []offEntry

	index glyphIndex

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

	locale system.Locale

	// history contains undo history.
	history []modification
	// nextHistoryIdx is the index within the history of the next modification. This
	// is only not len(history) immediately after undo operations occur. It is framed as the "next" value
	// to make the zero value consistent.
	nextHistoryIdx int
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

// ReadRune reads a rune from the underlying reader and replaces every
// rune with the mask rune.
func (m *maskReader) ReadRune() (r rune, n int, err error) {
	r, _, err = m.rr.ReadRune()
	if err != nil {
		return
	}
	if r != '\n' {
		r, _ = utf8.DecodeRune(m.mask)
		n = len(m.mask)
	} else {
		n = 1
	}
	return
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
	e.layoutText(e.shaper)
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
				evt.Type == gesture.TypeClick && evt.Source != pointer.Mouse:
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

				// Process multi-clicks.
				switch {
				case evt.NumClicks == 2:
					e.moveWord(-1, selectionClear)
					e.moveWord(1, selectionExtend)
					e.dragging = false
				case evt.NumClicks >= 3:
					e.moveStart(selectionClear)
					e.moveEnd(selectionExtend)
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
	// adjust keeps track of runes dropped because of MaxLen.
	var adjust int
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
			e.command(gtx, ke)
			e.caret.scroll = true
			e.scroller.Stop()
		case key.SnippetEvent:
			e.updateSnippet(gtx, ke.Start, ke.End)
		case key.EditEvent:
			e.caret.scroll = true
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
			e.caret.xoff = 0
			if submit {
				if e.rr.Changed() {
					e.events = append(e.events, ChangeEvent{})
				}
				e.events = append(e.events, SubmitEvent{
					Text: e.Text(),
				})
			}
		// Complete a paste event, initiated by Shortcut-V in Editor.command().
		case clipboard.Event:
			e.caret.scroll = true
			e.scroller.Stop()
			e.append(ke.Text)
		case key.SelectionEvent:
			e.caret.scroll = true
			e.scroller.Stop()
			ke.Start -= adjust
			ke.End -= adjust
			adjust = 0
			e.caret.start = e.closestToRune(ke.Start).runes
			e.caret.end = e.closestToRune(ke.End).runes
		}
	}
	if e.rr.Changed() {
		e.events = append(e.events, ChangeEvent{})
	}
}

func (e *Editor) closestToRune(runeIdx int) combinedPos {
	e.makeValid()
	pos, _ := e.index.closestToRune(runeIdx)
	return pos
}

func (e *Editor) closestToLineCol(line, col int) combinedPos {
	e.makeValid()
	return e.index.closestToLineCol(screenPos{line: line, col: col})
}

func (e *Editor) closestToXY(x fixed.Int26_6, y int) combinedPos {
	e.makeValid()
	return e.index.closestToXY(x, y)
}

func (e *Editor) moveLines(distance int, selAct selectionAction) {
	caretStart := e.closestToRune(e.caret.start)
	x := caretStart.x + e.caret.xoff
	// Seek to line.
	pos := e.closestToLineCol(caretStart.lineCol.line+distance, 0)
	pos = e.closestToXY(x, pos.y)
	e.caret.start = pos.runes
	e.caret.xoff = x - pos.x
	e.updateSelection(selAct)
}

func (e *Editor) command(gtx layout.Context, k key.Event) {
	direction := 1
	if e.locale.Direction.Progression() == system.TowardOrigin {
		direction = -1
	}
	moveByWord := k.Modifiers.Contain(key.ModShortcutAlt)
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
			e.moveWord(-1*direction, selAct)
		} else {
			if selAct == selectionClear {
				e.ClearSelection()
			}
			e.MoveCaret(-1*direction, -1*direction*int(selAct))
		}
	case key.NameRightArrow:
		if moveByWord {
			e.moveWord(1*direction, selAct)
		} else {
			if selAct == selectionClear {
				e.ClearSelection()
			}
			e.MoveCaret(1*direction, int(selAct)*direction)
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
		clipboard.ReadOp{Tag: &e.eventKey}.Add(gtx.Ops)
	// Copy or Cut selection -- ignored if nothing selected.
	case "C", "X":
		if text := e.SelectedText(); text != "" {
			clipboard.WriteOp{Text: text}.Add(gtx.Ops)
			if k.Name == "X" {
				e.Delete(1)
			}
		}
	// Select all
	case "A":
		e.caret.end = 0
		e.caret.start = e.Len()
	case "Z":
		if k.Modifiers.Contain(key.ModShift) {
			e.redo()
		} else {
			e.undo()
		}
	}
}

// Focus requests the input focus for the Editor.
func (e *Editor) Focus() {
	e.requestFocus = true
}

// Focused returns whether the editor is focused or not.
func (e *Editor) Focused() bool {
	return e.focused
}

// calculateViewSize determines the size of the current visible content,
// ensuring that even if there is no text content, some space is reserved
// for the caret.
func (e *Editor) calculateViewSize(gtx layout.Context) image.Point {
	base := e.dims.Size
	if caretWidth := e.caretWidth(gtx); base.X < caretWidth {
		base.X = caretWidth
	}
	return gtx.Constraints.Constrain(base)
}

// Layout lays out the editor. If content is not nil, it is laid out on top.
func (e *Editor) Layout(gtx layout.Context, lt *text.Shaper, font text.Font, size unit.Sp, content layout.Widget) layout.Dimensions {
	if e.locale != gtx.Locale {
		e.locale = gtx.Locale
		e.invalidate()
	}
	textSize := fixed.I(gtx.Sp(size))
	if e.font != font || e.textSize != textSize {
		e.invalidate()
		e.font = font
		e.textSize = textSize
	}
	maxWidth := gtx.Constraints.Max.X
	if e.SingleLine {
		maxWidth = math.MaxInt
	}
	if maxWidth != e.maxWidth {
		e.maxWidth = maxWidth
		e.invalidate()
	}
	if lt != e.shaper {
		e.shaper = lt
		e.invalidate()
	}
	if e.Mask != e.lastMask {
		e.lastMask = e.Mask
		e.invalidate()
	}

	e.makeValid()
	e.processEvents(gtx)
	e.makeValid()

	if viewSize := e.calculateViewSize(gtx); viewSize != e.viewSize {
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
	imeStart := e.closestToRune(start)
	imeEnd := e.closestToRune(end)
	e.ime.start = imeStart.runes
	e.ime.end = imeEnd.runes
	startOff := e.runeOffset(imeStart.runes)
	endOff := e.runeOffset(imeEnd.runes)
	e.rr.Seek(int64(startOff), io.SeekStart)
	n := endOff - startOff
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

	defer clip.Rect(image.Rectangle{Max: e.viewSize}).Push(gtx.Ops).Pop()
	pointer.CursorText.Add(gtx.Ops)
	var keys key.Set
	if e.focused {
		const keyFilterNoLeftUp = "(ShortAlt)-(Shift)-[→,↓]|(Shift)-[⏎,⌤]|(ShortAlt)-(Shift)-[⌫,⌦]|(Shift)-[⇞,⇟,⇱,⇲]|Short-[C,V,X,A]|Short-(Shift)-Z"
		const keyFilterNoRightDown = "(ShortAlt)-(Shift)-[←,↑]|(Shift)-[⏎,⌤]|(ShortAlt)-(Shift)-[⌫,⌦]|(Shift)-[⇞,⇟,⇱,⇲]|Short-[C,V,X,A]|Short-(Shift)-Z"
		const keyFilterNoArrows = "(Shift)-[⏎,⌤]|(ShortAlt)-(Shift)-[⌫,⌦]|(Shift)-[⇞,⇟,⇱,⇲]|Short-[C,V,X,A]|Short-(Shift)-Z"
		const keyFilterAllArrows = "(ShortAlt)-(Shift)-[←,→,↑,↓]|(Shift)-[⏎,⌤]|(ShortAlt)-(Shift)-[⌫,⌦]|(Shift)-[⇞,⇟,⇱,⇲]|Short-[C,V,X,A]|Short-(Shift)-Z"
		caret := e.closestToRune(e.caret.start)
		switch {
		case caret.runes == 0 && caret.runes == e.Len():
			keys = keyFilterNoArrows
		case caret.runes == 0:
			if gtx.Locale.Direction.Progression() == system.FromOrigin {
				keys = keyFilterNoLeftUp
			} else {
				keys = keyFilterNoRightDown
			}
		case caret.runes == e.Len():
			if gtx.Locale.Direction.Progression() == system.FromOrigin {
				keys = keyFilterNoRightDown
			} else {
				keys = keyFilterNoLeftUp
			}
		default:
			keys = keyFilterAllArrows
		}
	}
	key.InputOp{Tag: &e.eventKey, Hint: e.InputHint, Keys: keys}.Add(gtx.Ops)
	if e.requestFocus {
		key.FocusOp{Tag: &e.eventKey}.Add(gtx.Ops)
		key.SoftKeyboardOp{Show: true}.Add(gtx.Ops)
	}
	e.requestFocus = false

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
	if !e.focused {
		return
	}
	localViewport := image.Rectangle{Max: e.viewSize}
	docViewport := image.Rectangle{Max: e.viewSize}.Add(e.scrollOff)
	defer clip.Rect(localViewport).Push(gtx.Ops).Pop()
	e.regions = e.index.locate(docViewport, e.caret.start, e.caret.end, e.regions)
	for _, region := range e.regions {
		area := clip.Rect(region.bounds.Sub(e.scrollOff)).Push(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		area.Pop()
	}
}

func (e *Editor) PaintText(gtx layout.Context) {
	m := op.Record(gtx.Ops)
	viewport := image.Rectangle{
		Min: e.scrollOff,
		Max: e.viewSize.Add(e.scrollOff),
	}
	it := textIterator{viewport: viewport}

	startGlyph := 0
	for _, line := range e.index.lines {
		if line.descent.Ceil()+line.yOff >= viewport.Min.Y {
			break
		}
		startGlyph += line.glyphs
	}
	var glyphs [32]text.Glyph
	line := glyphs[:0]
	for _, g := range e.index.glyphs[startGlyph:] {
		var ok bool
		if line, ok = it.paintGlyph(gtx, e.shaper, g, line); !ok {
			break
		}
	}

	call := m.Stop()
	viewport.Min = viewport.Min.Add(it.padding.Min)
	viewport.Max = viewport.Max.Add(it.padding.Max)
	defer clip.Rect(viewport.Sub(e.scrollOff)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// caretWidth returns the width occupied by the caret for the current
// gtx.
func (e *Editor) caretWidth(gtx layout.Context) int {
	carWidth2 := gtx.Dp(1) / 2
	if carWidth2 < 1 {
		carWidth2 = 1
	}
	return carWidth2
}

func (e *Editor) PaintCaret(gtx layout.Context) {
	if !e.caret.on {
		return
	}
	carWidth2 := e.caretWidth(gtx)
	caretPos, carAsc, carDesc := e.caretInfo()

	carRect := image.Rectangle{
		Min: caretPos.Sub(image.Pt(carWidth2, carAsc)),
		Max: caretPos.Add(image.Pt(carWidth2, carDesc)),
	}
	cl := image.Rectangle{Max: e.viewSize}
	carRect = cl.Intersect(carRect)
	if !carRect.Empty() {
		defer clip.Rect(carRect).Push(gtx.Ops).Pop()
		paint.PaintOp{}.Add(gtx.Ops)
	}
}

func (e *Editor) caretInfo() (pos image.Point, ascent, descent int) {
	caretStart := e.closestToRune(e.caret.start)

	ascent = caretStart.ascent.Ceil()
	descent = caretStart.descent.Ceil()

	pos = image.Point{
		X: caretStart.x.Round(),
		Y: caretStart.y,
	}
	pos = pos.Sub(e.scrollOff)
	return
}

// Len is the length of the editor contents, in runes.
func (e *Editor) Len() int {
	e.makeValid()
	return e.closestToRune(math.MaxInt).runes
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
	if e.SingleLine {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	e.replace(e.caret.start, e.caret.end, s, true)
	e.caret.xoff = 0
}

func (e *Editor) scrollBounds() image.Rectangle {
	var b image.Rectangle
	if e.SingleLine {
		if len(e.index.lines) > 0 {
			line := e.index.lines[0]
			b.Min.X = line.xOff.Floor()
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
	e.caret.start = e.closestToXY(x, y).runes
	e.caret.xoff = 0
}

func (e *Editor) layoutText(lt *text.Shaper) {
	e.rr.Reset()
	var r io.RuneReader = &e.rr
	if e.Mask != 0 {
		e.maskReader.Reset(&e.rr, e.Mask)
		r = &e.maskReader
	}
	e.index = glyphIndex{}
	it := textIterator{viewport: image.Rectangle{Max: image.Point{X: math.MaxInt, Y: math.MaxInt}}}
	if lt != nil {
		lt.Layout(text.Parameters{
			Font:      e.font,
			PxPerEm:   e.textSize,
			Alignment: e.Alignment,
		}, 0, e.maxWidth, e.locale, r)
		for glyph, ok := it.processGlyph(lt.NextGlyph()); ok; glyph, ok = it.processGlyph(lt.NextGlyph()) {
			e.index.Glyph(glyph)
		}
	} else {
		// Make a fake glyph for every rune in the reader.
		for _, _, err := r.ReadRune(); err != io.EOF; _, _, err = r.ReadRune() {
			g, _ := it.processGlyph(text.Glyph{Runes: 1, Flags: text.FlagClusterBreak}, true)
			e.index.Glyph(g)

		}
	}
	dims := layout.Dimensions{Size: it.bounds.Size()}
	dims.Baseline = dims.Size.Y - it.baseline
	e.dims = dims
}

// CaretPos returns the line & column numbers of the caret.
func (e *Editor) CaretPos() (line, col int) {
	pos := e.closestToRune(e.caret.start)
	return pos.lineCol.line, pos.lineCol.col
}

// CaretCoords returns the coordinates of the caret, relative to the
// editor itself.
func (e *Editor) CaretCoords() f32.Point {
	pos := e.closestToRune(e.caret.start)
	return f32.Pt(float32(pos.x)/64-float32(e.scrollOff.X), float32(pos.y-e.scrollOff.Y))
}

// indexRune returns the latest rune index and byte offset no later than r.
func (e *Editor) indexRune(r int) offEntry {
	// Initialize index.
	if len(e.offIndex) == 0 {
		e.offIndex = append(e.offIndex, offEntry{})
	}
	i := sort.Search(len(e.offIndex), func(i int) bool {
		entry := e.offIndex[i]
		return entry.runes >= r
	})
	// Return the entry guaranteed to be less than or equal to r.
	if i > 0 {
		i--
	}
	return e.offIndex[i]
}

// runeOffset returns the byte offset into e.rr of the r'th rune.
// r must be a valid rune index, usually returned by closestPosition.
func (e *Editor) runeOffset(r int) int {
	const runesPerIndexEntry = 50
	entry := e.indexRune(r)
	lastEntry := e.offIndex[len(e.offIndex)-1].runes
	for entry.runes < r {
		if entry.runes > lastEntry && entry.runes%runesPerIndexEntry == runesPerIndexEntry-1 {
			e.offIndex = append(e.offIndex, entry)
		}
		_, s := e.rr.runeAt(entry.bytes)
		entry.bytes += s
		entry.runes++
	}
	return entry.bytes
}

func (e *Editor) invalidate() {
	e.offIndex = e.offIndex[:0]
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
	e.replace(start, end, "", true)
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
	if e.SingleLine {
		s = strings.ReplaceAll(s, "\n", " ")
	}
	moves := e.replace(e.caret.start, e.caret.end, s, true)
	e.caret.xoff = 0
	start := e.caret.start
	if end := e.caret.end; end < start {
		start = end
	}
	e.caret.start = start + moves
	e.caret.end = e.caret.start
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
func (e *Editor) undo() {
	if len(e.history) < 1 || e.nextHistoryIdx == 0 {
		return
	}
	mod := e.history[e.nextHistoryIdx-1]
	replaceEnd := mod.StartRune + utf8.RuneCountInString(mod.ApplyContent)
	e.replace(mod.StartRune, replaceEnd, mod.ReverseContent, false)
	caretEnd := mod.StartRune + utf8.RuneCountInString(mod.ReverseContent)
	e.SetCaret(caretEnd, mod.StartRune)
	e.nextHistoryIdx--
}

// redo applies the modification at e.history[e.historyIdx] and increments
// e.historyIdx.
func (e *Editor) redo() {
	if len(e.history) < 1 || e.nextHistoryIdx == len(e.history) {
		return
	}
	mod := e.history[e.nextHistoryIdx]
	end := mod.StartRune + utf8.RuneCountInString(mod.ReverseContent)
	e.replace(mod.StartRune, end, mod.ApplyContent, false)
	caretEnd := mod.StartRune + utf8.RuneCountInString(mod.ApplyContent)
	e.SetCaret(caretEnd, mod.StartRune)
	e.nextHistoryIdx++
}

// replace the text between start and end with s. Indices are in runes.
// It returns the number of runes inserted.
// addHistory controls whether this modification is recorded in the undo
// history.
func (e *Editor) replace(start, end int, s string, addHistory bool) int {
	if start > end {
		start, end = end, start
	}
	startPos := e.closestToRune(start)
	endPos := e.closestToRune(end)
	startOff := e.runeOffset(startPos.runes)
	replaceSize := endPos.runes - startPos.runes
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
	newEnd := startPos.runes + sc

	if addHistory {
		e.rr.Seek(int64(startOff), 0)
		deleted := make([]rune, 0, replaceSize)
		for i := 0; i < replaceSize; i++ {
			ru, _, _ := e.rr.ReadRune()
			deleted = append(deleted, ru)
		}
		if e.nextHistoryIdx < len(e.history) {
			e.history = e.history[:e.nextHistoryIdx]
		}
		e.history = append(e.history, modification{
			StartRune:      startPos.runes,
			ApplyContent:   s,
			ReverseContent: string(deleted),
		})
		e.nextHistoryIdx++
	}

	e.rr.deleteRunes(startOff, replaceSize)
	e.rr.prepend(startOff, s)
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
	return sc
}

func (e *Editor) movePages(pages int, selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	x := caret.x + e.caret.xoff
	y := caret.y + pages*e.viewSize.Y
	pos := e.closestToXY(x, y)
	e.caret.start = pos.runes
	e.caret.xoff = x - pos.x
	e.updateSelection(selAct)
}

// MoveCaret moves the caret (aka selection start) and the selection end
// relative to their current positions. Positive distances moves forward,
// negative distances moves backward. Distances are in runes.
func (e *Editor) MoveCaret(startDelta, endDelta int) {
	e.caret.xoff = 0
	e.caret.start = e.closestToRune(e.caret.start + startDelta).runes
	e.caret.end = e.closestToRune(e.caret.end + endDelta).runes
}

func (e *Editor) moveStart(selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	caret = e.closestToLineCol(caret.lineCol.line, 0)
	e.caret.start = caret.runes
	e.caret.xoff = -caret.x
	e.updateSelection(selAct)
}

func (e *Editor) moveEnd(selAct selectionAction) {
	caret := e.closestToRune(e.caret.start)
	caret = e.closestToLineCol(caret.lineCol.line, math.MaxInt)
	e.caret.start = caret.runes
	e.caret.xoff = fixed.I(e.maxWidth) - caret.x
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
	caret := e.closestToRune(e.caret.start)
	atEnd := func() bool {
		return caret.runes == 0 || caret.runes == e.Len()
	}
	// next returns the appropriate rune given the direction.
	next := func() (r rune) {
		off := e.runeOffset(caret.runes)
		if direction < 0 {
			r, _ = e.rr.runeBefore(off)
		} else {
			r, _ = e.rr.runeAt(off)
		}
		return r
	}
	for ii := 0; ii < words; ii++ {
		for r := next(); unicode.IsSpace(r) && !atEnd(); r = next() {
			e.MoveCaret(direction, 0)
			caret = e.closestToRune(e.caret.start)
		}
		e.MoveCaret(direction, 0)
		caret = e.closestToRune(e.caret.start)
		for r := next(); !unicode.IsSpace(r) && !atEnd(); r = next() {
			e.MoveCaret(direction, 0)
			caret = e.closestToRune(e.caret.start)
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
	caret := e.closestToRune(e.caret.start)
	atEnd := func(runes int) bool {
		idx := caret.runes + runes*direction
		return idx <= 0 || idx >= e.Len()
	}
	// next returns the appropriate rune given the direction and offset in runes).
	next := func(runes int) rune {
		idx := caret.runes + runes*direction
		if idx < 0 {
			idx = 0
		} else if idx > e.Len() {
			idx = e.Len()
		}
		off := e.runeOffset(idx)
		var r rune
		if direction < 0 {
			r, _ = e.rr.runeBefore(off)
		} else {
			r, _ = e.rr.runeAt(off)
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
	e.Delete(runes * direction)
}

func (e *Editor) scrollToCaret() {
	caret := e.closestToRune(e.caret.start)
	if e.SingleLine {
		var dist int
		if d := caret.x.Floor() - e.scrollOff.X; d < 0 {
			dist = d
		} else if d := caret.x.Ceil() - (e.scrollOff.X + e.viewSize.X); d > 0 {
			dist = d
		}
		e.scrollRel(dist, 0)
	} else {
		miny := caret.y - caret.ascent.Ceil()
		maxy := caret.y + caret.descent.Ceil()
		var dist int
		if d := miny - e.scrollOff.Y; d < 0 {
			dist = d
		} else if d := maxy - (e.scrollOff.Y + e.viewSize.Y); d > 0 {
			dist = d
		}
		e.scrollRel(0, dist)
	}
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
	e.caret.start = e.closestToRune(start).runes
	e.caret.end = e.closestToRune(end).runes
	e.caret.scroll = true
	e.scroller.Stop()
}

// SelectedText returns the currently selected text (if any) from the editor.
func (e *Editor) SelectedText() string {
	startOff := e.runeOffset(e.caret.start)
	endOff := e.runeOffset(e.caret.end)
	start := min(startOff, endOff)
	end := max(startOff, endOff)
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
	return e.rr.Seek(offset, io.SeekStart)
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

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func abs[T constraints.Signed](n T) T {
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
