// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"math"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
)

// RichTextEvent is the interface for events from RichTextEditor.
type RichTextEvent interface {
	isRichTextEvent()
}

// RichTextInputEvent is emitted when text content changes.
type RichTextInputEvent struct {
	Buffer *FormattedBuffer
}

func (RichTextInputEvent) isRichTextEvent() {}

// RichTextCaretEvent is emitted when the caret moves.
type RichTextCaretEvent struct {
	Position int
}

func (RichTextCaretEvent) isRichTextEvent() {}

// RichTextStyleChangedEvent is emitted when the current style or selection style changes.
type RichTextStyleChangedEvent struct {
	Style CharStyle
}

func (RichTextStyleChangedEvent) isRichTextEvent() {}

// TextRenderer is the interface that the host must implement to render content.
type TextRenderer interface {
	// RenderCaret draws the caret at the given position.
	RenderCaret(gtx layout.Context, x, y, height int)

	// RenderSelection draws a selection highlight rectangle.
	RenderSelection(gtx layout.Context, rect image.Rectangle)
}

// RichTextEditor is a widget for editing styled text.
// It handles keyboard input, caret positioning, and style tracking.
// Pointer events (clicks) are handled externally via SetCaretFromClick/SelectFromClick.
type RichTextEditor struct {
	// State
	buffer   FormattedBuffer
	caretPos int
	selStart int
	selEnd   int
	focused  bool
	focusTag event.Tag

	// Current style for new characters
	currentStyle CharStyle

	// Focus request flag (set by Focus(), cleared by Layout())
	requestFocus bool

	// Layout cache
	layout      CaretLayout
	layoutDirty bool

	// Widget bounds (set during Layout)
	bounds image.Rectangle

	// Renderer (set by host before Layout)
	renderer TextRenderer

	// Caret blinking
	lastCaretMove time.Time
	blinkPhase    int // 0 = visible, 1 = hidden

	// Pending events
	pending []RichTextEvent

	// Pointer state is owned by the widget. The host only needs to place the
	// widget over the rendered text; hit-testing and drag selection stay in Gio.
	pointerTag   event.Tag
	dragging     bool
	lastPressAt  time.Time
	lastPressPos image.Point
}

// NewRichTextEditor creates a new RichTextEditor.
func NewRichTextEditor() *RichTextEditor {
	return &RichTextEditor{
		focusTag:    new(event.Tag),
		pointerTag:  new(event.Tag),
	}
}

// SetRenderer sets the text renderer for this widget.
func (rte *RichTextEditor) SetRenderer(renderer TextRenderer) {
	rte.renderer = renderer
}

// Buffer returns the formatted text buffer (read-only).
func (rte *RichTextEditor) Buffer() *FormattedBuffer {
	return &rte.buffer
}

// SetBuffer replaces the entire buffer content.
func (rte *RichTextEditor) SetBuffer(buf FormattedBuffer) {
	rte.buffer = buf
	rte.caretPos = rte.buffer.Len()
	rte.selStart = 0
	rte.selEnd = 0
	rte.layoutDirty = true
}

// Text returns the plain text content.
func (rte *RichTextEditor) Text() string {
	return rte.buffer.Text()
}

// SetText replaces the buffer with plain text using the given default style.
func (rte *RichTextEditor) SetText(text string, defaultStyle CharStyle) {
	rte.buffer = *NewFormattedBufferFromText(text, defaultStyle)
	rte.caretPos = rte.buffer.Len()
	rte.selStart = 0
	rte.selEnd = 0
	rte.layoutDirty = true
}

// CaretPos returns the current caret position (rune index).
func (rte *RichTextEditor) CaretPos() int {
	return rte.caretPos
}

// SetCaret sets the caret position and optionally selection range.
func (rte *RichTextEditor) SetCaret(start, end int) {
	l := rte.buffer.Len()
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if start > l {
		start = l
	}
	if end > l {
		end = l
	}
	rte.caretPos = start
	rte.selStart = start
	rte.selEnd = end
	rte.resetCaretBlink()
}

// Selection returns the selection range (from, to) with from <= to.
func (rte *RichTextEditor) Selection() (int, int) {
	if rte.selStart <= rte.selEnd {
		return rte.selStart, rte.selEnd
	}
	return rte.selEnd, rte.selStart
}

// HasSelection returns true if there's an active selection.
func (rte *RichTextEditor) HasSelection() bool {
	return rte.selStart != rte.selEnd
}

// SelectedText returns the selected text.
func (rte *RichTextEditor) SelectedText() string {
	from, to := rte.Selection()
	runes := rte.buffer.Runes()
	if from >= len(runes) || to > len(runes) || from >= to {
		return ""
	}
	return string(runes[from:to])
}

// CurrentStyle returns the style for newly typed characters.
func (rte *RichTextEditor) CurrentStyle() CharStyle {
	return rte.currentStyle
}

// SetCurrentStyle sets the style for newly typed characters.
func (rte *RichTextEditor) SetCurrentStyle(style CharStyle) {
	rte.currentStyle = style
}

// ApplyStyle applies a full style to the current selection (or at caret if no selection).
func (rte *RichTextEditor) ApplyStyle(style CharStyle) {
	if rte.HasSelection() {
		from, to := rte.Selection()
		rte.buffer.SetStyle(from, to, style)
	}
	rte.currentStyle = style
	rte.layoutDirty = true
	rte.emitStyleChanged()
}

// --- Toggle style methods ---

func (rte *RichTextEditor) ToggleBold() {
	newBold := !rte.currentStyle.B
	rte.applyToSelection(func(s *CharStyle) { s.B = newBold })
	rte.currentStyle.B = newBold
	rte.emitStyleChanged()
}

func (rte *RichTextEditor) ToggleItalic() {
	newItalic := !rte.currentStyle.I
	rte.applyToSelection(func(s *CharStyle) { s.I = newItalic })
	rte.currentStyle.I = newItalic
	rte.emitStyleChanged()
}

func (rte *RichTextEditor) ToggleUnderline() {
	newUnderline := !rte.currentStyle.U
	rte.applyToSelection(func(s *CharStyle) { s.U = newUnderline })
	rte.currentStyle.U = newUnderline
	rte.emitStyleChanged()
}

// --- Set style properties ---

func (rte *RichTextEditor) SetTextColor(hex string) {
	rte.applyToSelection(func(s *CharStyle) { s.C = hex })
	rte.currentStyle.C = hex
	rte.emitStyleChanged()
}

func (rte *RichTextEditor) SetFont(fontRef string) {
	rte.applyToSelection(func(s *CharStyle) { s.F = fontRef })
	rte.currentStyle.F = fontRef
	rte.emitStyleChanged()
}

func (rte *RichTextEditor) SetFontSize(size float32) {
	rte.applyToSelection(func(s *CharStyle) { s.S = size })
	rte.currentStyle.S = size
	rte.layoutDirty = true
	rte.emitStyleChanged()
}

func (rte *RichTextEditor) SetLineHeight(lh float32) {
	rte.applyToSelection(func(s *CharStyle) { s.LH = lh })
	rte.currentStyle.LH = lh
	rte.layoutDirty = true
	rte.emitStyleChanged()
}

func (rte *RichTextEditor) SetAlignment(align string) {
	rte.applyToSelection(func(s *CharStyle) { s.A = align })
	rte.currentStyle.A = align
	rte.layoutDirty = true
	rte.emitStyleChanged()
}

func (rte *RichTextEditor) SetBgColor(hex string) {
	rte.applyToSelection(func(s *CharStyle) { s.BGC = hex })
	rte.currentStyle.BGC = hex
	rte.emitStyleChanged()
}

// --- Query style at caret/selection ---

func (rte *RichTextEditor) StyleAtCaret() CharStyle {
	if rte.buffer.Len() == 0 {
		return rte.currentStyle
	}
	pos := rte.caretPos
	if pos >= rte.buffer.Len() {
		pos = rte.buffer.Len() - 1
	}
	return rte.buffer.StyleAt(pos)
}

func (rte *RichTextEditor) IsBoldAtCaret() bool    { return rte.StyleAtCaret().B }
func (rte *RichTextEditor) IsItalicAtCaret() bool   { return rte.StyleAtCaret().I }
func (rte *RichTextEditor) IsUnderlineAtCaret() bool { return rte.StyleAtCaret().U }
func (rte *RichTextEditor) TextColorAtCaret() string { return rte.StyleAtCaret().C }
func (rte *RichTextEditor) FontAtCaret() string      { return rte.StyleAtCaret().F }
func (rte *RichTextEditor) FontSizeAtCaret() float32  { return rte.StyleAtCaret().S }

// --- CaretLayout integration ---

// ComputeLayout computes per-character positions using the given measurer.
func (rte *RichTextEditor) ComputeLayout(measurer GlyphMeasurer, maxWidth, startX, startY int, defaultStyle CharStyle) {
	rte.layout = ComputeLayout(&rte.buffer, measurer, maxWidth, startX, startY, defaultStyle)
	rte.layoutDirty = false
}

// SetLayoutFromExternal sets the layout from externally computed character positions.
// This is used when the host renders text itself (e.g., drawRichTextSegments) and
// provides accurate per-character screen rects. The rects are in zone-local coordinates.
// It detects line breaks by checking for Y coordinate changes OR newline characters.
func (rte *RichTextEditor) SetLayoutFromExternal(rects []image.Rectangle, chars []rune) {
	if len(rects) == 0 {
		rte.layout = CaretLayout{}
		return
	}
	glyphs := make([]GlyphMetrics, len(rects))
	for i, r := range rects {
		glyphs[i] = GlyphMetrics{
			X:      r.Min.X,
			Y:      r.Min.Y,
			Width:  r.Dx(),
			Height: r.Dy(),
			Line:   0,
		}
	}

	// Detect lines by checking for:
	// 1. Y coordinate changes > 2 pixels (word wrap)
	// 2. Newline characters (explicit line breaks)
	var lines []LineInfo
	curLineStart := 0
	curLineY := glyphs[0].Y
	curLineH := glyphs[0].Height
	curLineWidth := 0

	flushLine := func(endIdx int) {
		lines = append(lines, LineInfo{
			StartIdx: curLineStart,
			EndIdx:   endIdx,
			Y:        curLineY,
			Height:   curLineH,
			Width:    curLineWidth,
		})
	}

	for i, g := range glyphs {
		isNewline := i < len(chars) && (chars[i] == '\n' || chars[i] == '\r')
		isYChange := i > 0 && (g.Y-curLineY > 2 || curLineY-g.Y > 2)

		if isNewline || isYChange {
			// Flush current line (end at current index, not including this glyph)
			flushLine(i)
			curLineStart = i
			curLineY = g.Y
			curLineH = g.Height
			curLineWidth = 0
		}

		glyphs[i].Line = len(lines)
		curLineWidth += g.Width
		if g.Height > curLineH {
			curLineH = g.Height
		}
	}
	// Flush last line
	flushLine(len(glyphs))

	rte.layout.glyphs = glyphs
	rte.layout.lines = lines
	rte.layoutDirty = false
}

// LayoutCache returns the current CaretLayout (may be stale if ComputeLayout not called).
func (rte *RichTextEditor) LayoutCache() CaretLayout { return rte.layout }

// LayoutDirty returns true if the layout needs recomputation.
func (rte *RichTextEditor) LayoutDirty() bool { return rte.layoutDirty }

// MarkLayoutDirty marks the layout as needing recomputation.
func (rte *RichTextEditor) MarkLayoutDirty() { rte.layoutDirty = true }

// CaretCoords returns the screen position of the current caret.
// Returns zero CaretCoords if layout hasn't been computed yet.
func (rte *RichTextEditor) CaretCoords() CaretCoords {
	return rte.layout.CaretCoordsAt(rte.caretPos)
}

// GetCaretCoords returns the caret coordinates (alias for external use).
func (rte *RichTextEditor) GetCaretCoords() CaretCoords {
	return rte.CaretCoords()
}

// SelectionCoords returns screen rects for the current selection.
func (rte *RichTextEditor) SelectionCoords() []image.Rectangle {
	if !rte.HasSelection() {
		return nil
	}
	from, to := rte.Selection()
	var rects []image.Rectangle
	for i := from; i < to && i < rte.layout.Len(); i++ {
		g := rte.layout.GlyphAt(i)
		if g.Width > 0 && g.Height > 0 {
			rects = append(rects, image.Rect(g.X, g.Y, g.X+g.Width, g.Y+g.Height))
		}
	}
	return rects
}

// SetCaretFromClick positions the caret nearest to the given screen coordinates.
// Returns true if the caret was repositioned.
func (rte *RichTextEditor) SetCaretFromClick(screenX, screenY int) bool {
	if rte.layout.Len() == 0 {
		return false
	}
	newPos := rte.layout.HitTest(screenX, screenY)
	rte.caretPos = newPos
	rte.selStart = 0
	rte.selEnd = 0
	rte.resetCaretBlink()
	return true
}

// SelectFromClick starts or extends selection from screen coordinates.
func (rte *RichTextEditor) SelectFromClick(screenX, screenY int) {
	if rte.layout.Len() == 0 {
		return
	}
	newPos := rte.layout.HitTest(screenX, screenY)
	if rte.selStart == rte.selEnd {
		rte.selStart = rte.caretPos
		rte.selEnd = newPos
	} else {
		rte.selEnd = newPos
	}
	rte.caretPos = newPos
	rte.resetCaretBlink()
}

// StartSelection begins a new selection at the given rune position (from a mouse press).
func (rte *RichTextEditor) StartSelection(pos int) {
	if pos < 0 {
		pos = 0
	}
	if pos > rte.buffer.Len() {
		pos = rte.buffer.Len()
	}
	rte.selStart = pos
	rte.selEnd = pos
	rte.caretPos = pos
}

// ExtendSelection extends the selection end to the given rune position (from a mouse drag).
func (rte *RichTextEditor) ExtendSelection(pos int) {
	if pos < 0 {
		pos = 0
	}
	if pos > rte.buffer.Len() {
		pos = rte.buffer.Len()
	}
	rte.selEnd = pos
	rte.caretPos = pos
	rte.resetCaretBlink()
	rte.pending = append(rte.pending, RichTextCaretEvent{Position: pos})
}

// ExtendSelectionFromScreen extends the selection end to the nearest position
// to the given screen coordinates (from a mouse drag).
func (rte *RichTextEditor) ExtendSelectionFromScreen(screenX, screenY int) {
	if rte.layout.Len() == 0 {
		return
	}
	newPos := rte.layout.HitTest(screenX, screenY)
	rte.ExtendSelection(newPos)
}

// TotalHeight returns the total height of all lines in the layout.
func (rte *RichTextEditor) TotalHeight() int { return rte.layout.TotalHeight() }

// LineCount returns the number of lines in the layout.
func (rte *RichTextEditor) LineCount() int { return rte.layout.LineCount() }

// resetCaretBlink resets the caret to visible and restarts the blink timer.
// passNow should be gtx.Now if available, otherwise zero resets to always-visible.
func (rte *RichTextEditor) resetCaretBlink() {
	rte.blinkPhase = 0
	rte.lastCaretMove = time.Now()
}

// --- Internal helpers ---

func (rte *RichTextEditor) emitStyleChanged() {
	rte.pending = append(rte.pending, RichTextStyleChangedEvent{Style: rte.currentStyle})
}

func (rte *RichTextEditor) applyToSelection(mutate func(*CharStyle)) {
	if rte.HasSelection() {
		from, to := rte.Selection()
		base := rte.buffer.StyleAt(from)
		mutate(&base)
		rte.buffer.SetStyle(from, to, base)
	}
	mutate(&rte.currentStyle)
}

// selectWordAt selects the word at the given rune position.
func (rte *RichTextEditor) selectWordAt(pos int) {
	runes := rte.buffer.Runes()
	if len(runes) == 0 || pos < 0 || pos >= len(runes) {
		return
	}

	wordStart := pos
	for wordStart > 0 && !isWordSep(runes[wordStart-1]) {
		wordStart--
	}

	wordEnd := pos
	for wordEnd < len(runes)-1 && !isWordSep(runes[wordEnd+1]) {
		wordEnd++
	}

	if isWordSep(runes[pos]) {
		wordStart = pos
		wordEnd = pos
	}

	rte.selStart = wordStart
	rte.selEnd = wordEnd + 1
	rte.caretPos = wordEnd + 1
}

// isWordSep returns true if the rune is a word separator.
func isWordSep(ch rune) bool {
	switch ch {
	case ' ', '\t', '\n', '\r':
		return true
	case '.', ',', ';', ':', '!', '?':
		return true
	case '(', ')', '[', ']', '{', '}':
		return true
	case '\'', '@', '#', '$':
		return true
	case '&', '*', '+', '=', '|', '^':
		return true
	case '<', '>', '~', '`':
		return true
	case '/', '\\':
		return true
	}
	return false
}

// Bounds returns the widget's bounds from the last Layout call.
func (rte *RichTextEditor) Bounds() image.Rectangle {
	return rte.bounds
}

// FocusTag returns the event tag used for keyboard focus.
func (rte *RichTextEditor) FocusTag() event.Tag {
	return rte.focusTag
}

// PointerTag returns the pointer target registered by Layout. Hosts that need
// to compose the widget with draggable objects may use this tag for explicit
// pointer priority, but normal use requires no event handling by the host.
func (rte *RichTextEditor) PointerTag() event.Tag {
	return rte.pointerTag
}

// --- Widget lifecycle ---

// Layout processes keyboard input and renders caret/selection.
// Text is rendered externally by the host (via drawRichTextSegments or similar).
// Pointer events (clicks) are handled externally by the host via SetCaretFromClick/SelectFromClick.
func (rte *RichTextEditor) Layout(gtx layout.Context) layout.Dimensions {
	// Store bounds
	rte.bounds = image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)

	// Register keyboard input hint — this MUST be done here (inside Layout, inside clip context)
	// for the Gio focus system to recognize this tag as a valid keyboard input target.
	key.InputHintOp{Tag: rte.focusTag, Hint: key.HintAny}.Add(gtx.Ops)
	// Register the focus target itself. InputHintOp only describes the input
	// type; without event.Op keyboard/EditEvents cannot be routed to the tag.
	event.Op(gtx.Ops, rte.focusTag)

	// Register the widget's local pointer area. Pointer.Event.Position is then
	// in the same local coordinate system as CaretLayout, eliminating the
	// screen/world coordinate conversions that used to live in applications.
	event.Op(gtx.Ops, rte.pointerTag)
	pointer.CursorText.Add(gtx.Ops)

	// Request focus if Focus() was called before/during this frame
	if rte.requestFocus {
		gtx.Execute(key.FocusCmd{Tag: rte.focusTag})
		rte.requestFocus = false
	}

	// Process keyboard and pointer events.
	rte.processEvents(gtx)

	// Compute caret blink state: toggle every 500ms
	if !rte.lastCaretMove.IsZero() {
		elapsed := gtx.Now.Sub(rte.lastCaretMove)
		blinkCount := int(elapsed / (500 * time.Millisecond))
		if blinkCount%2 == 0 {
			rte.blinkPhase = 0 // visible
		} else {
			rte.blinkPhase = 1 // hidden
		}
	}

	// Periodic invalidation for caret blinking (500ms intervals)
	nextTick := gtx.Now.Add(500 * time.Millisecond)
	gtx.Execute(op.InvalidateCmd{At: nextTick})

	// Render caret and selection (text is rendered by the host)
	if rte.renderer != nil && rte.layout.Len() > 0 {
		rte.render(gtx)
	}

	return layout.Dimensions{
		Size: rte.bounds.Size(),
	}
}

// render draws the caret and selection highlights.
// Text is NOT rendered here — it's rendered by the host's drawRichTextSegments.
func (rte *RichTextEditor) render(gtx layout.Context) {
	if rte.renderer == nil {
		return
	}

	// Draw selection highlights
	if rte.HasSelection() {
		from, to := rte.Selection()
		for i := from; i < to && i < rte.layout.Len(); i++ {
			g := rte.layout.GlyphAt(i)
			if g.Width > 0 && g.Height > 0 {
				rte.renderer.RenderSelection(gtx, image.Rect(g.X, g.Y, g.X+g.Width, g.Y+g.Height))
			}
		}
	}

	// Draw caret (only when blink phase is visible)
	if rte.blinkPhase == 0 {
		caret := rte.CaretCoords()
		if caret.Height > 0 {
			rte.renderer.RenderCaret(gtx, caret.X, caret.Y, caret.Height)
		}
	}
}

// Focus requests keyboard focus on the next Layout() call.
// This is safe to call before Layout() because it sets a flag
// that Layout() checks when key.InputHintOp is registered.
func (rte *RichTextEditor) Focus(gtx layout.Context) {
	rte.requestFocus = true
	_ = gtx // gtx may not be ready yet; Layout() will use it
}

// Blur releases keyboard focus from this widget.
// Calling Blur with an empty FocusCmd{} tells the focus manager to release all focus,
// which allows other widgets to request focus on the next frame.
func (rte *RichTextEditor) Blur(gtx layout.Context) {
	rte.requestFocus = false
	rte.focused = false
	gtx.Execute(key.FocusCmd{})
}

// Focused returns whether this widget has keyboard focus.
func (rte *RichTextEditor) Focused(gtx layout.Context) bool {
	return gtx.Focused(rte.focusTag)
}

// processEvents processes keyboard and transfer events (called from Layout).
// key.InputHintOp must be registered before this call.
func (rte *RichTextEditor) processEvents(gtx layout.Context) {
	// Build event filters — keyboard and transfer only, NO pointer.
	// Use a single catch-all key.Filter (empty Name) that matches ALL key events
	// for this tag, including characters, arrows, delete, etc. This is critical:
	// individual named filters would MISS character input events (like Cyrillic letters).
	allMods := key.ModShift | key.ModCtrl | key.ModAlt | key.ModSuper | key.ModShortcut | key.ModShortcutAlt
	filters := []event.Filter{
		key.FocusFilter{Target: rte.focusTag},
		key.Filter{Focus: rte.focusTag, Optional: allMods},
		transfer.TargetFilter{Target: rte.focusTag, Type: "application/text"},
		pointer.Filter{Target: rte.pointerTag, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel},
	}
	for {
		evt, ok := gtx.Event(filters...)
		if !ok {
			break
		}

		switch e := evt.(type) {
		case key.FocusEvent:
			rte.focused = e.Focus
			if e.Focus {
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			}

		case pointer.Event:
			rte.processPointerEvent(gtx, e)

		case transfer.DataEvent:
			if e.Open != nil {
				reader := e.Open()
				defer reader.Close()
				buf := make([]byte, 1024*1024)
				n, _ := reader.Read(buf)
				if n > 0 {
					text := string(buf[:n])
					rte.insertText(text)
					rte.pending = append(rte.pending, RichTextInputEvent{Buffer: &rte.buffer})
				}
			}

		case key.Event:
			if e.State == key.Press {
				ev := rte.processKey(gtx, e)
				if ev != nil {
					rte.pending = append(rte.pending, ev)
				}
			}

		case key.EditEvent:
			// EditEvent arrives from the textarea input handler. For regular typing,
			// e.Range is based on the internal Gio editor cursor (always 0), which
			// is wrong. We use rte.caretPos instead for all character input.
			if e.Text != "" {
				rte.insertText(e.Text)
				rte.layoutDirty = true
				rte.pending = append(rte.pending, RichTextInputEvent{Buffer: &rte.buffer})
			}
		}
	}

}

// processPointerEvent owns pointer-to-caret conversion for the widget.
// Coordinates are local to the RichTextEditor bounds and therefore match the
// CaretLayout coordinates exactly.
func (rte *RichTextEditor) processPointerEvent(gtx layout.Context, e pointer.Event) {
	if e.Source != pointer.Mouse {
		return
	}
	pos := func() int {
		return rte.layout.HitTest(int(math.Round(float64(e.Position.X))), int(math.Round(float64(e.Position.Y))))
	}

	switch e.Kind {
	case pointer.Press:
		// For empty layouts, allow placing caret at position 0.
		// This is needed so newly-activated zones accept the first click
		// and gain keyboard focus even before any text is typed.
		var caret int
		p := image.Point{X: int(math.Round(float64(e.Position.X))), Y: int(math.Round(float64(e.Position.Y)))}
		if rte.layout.Len() > 0 {
			caret = pos()
		}
		now := time.Now()
		double := !rte.lastPressAt.IsZero() && now.Sub(rte.lastPressAt) < 300*time.Millisecond &&
			absInt(p.X-rte.lastPressPos.X) < 20 && absInt(p.Y-rte.lastPressPos.Y) < 20
		rte.lastPressAt = now
		rte.lastPressPos = p

		if e.Modifiers&key.ModShift != 0 {
			if !rte.HasSelection() {
				rte.selStart = rte.caretPos
			}
			rte.selEnd = caret
			rte.caretPos = caret
		} else if double {
			rte.caretPos = caret
			rte.selectWordAt(caret)
		} else {
			rte.caretPos = caret
			rte.selStart = 0
			rte.selEnd = 0
		}
		rte.dragging = true
		rte.resetCaretBlink()
		gtx.Execute(key.FocusCmd{Tag: rte.focusTag})
		gtx.Execute(key.SoftKeyboardCmd{Show: true})
		rte.pending = append(rte.pending, RichTextCaretEvent{Position: rte.caretPos})

	case pointer.Drag:
		if rte.dragging {
			rte.ExtendSelection(pos())
		}

	case pointer.Release, pointer.Cancel:
		rte.dragging = false
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Update returns any pending events. Called by the host after Layout().
// Keyboard and pointer events are processed inside Layout() -> processEvents().
func (rte *RichTextEditor) Update(gtx layout.Context) (RichTextEvent, bool) {
	if len(rte.pending) > 0 {
		out := rte.pending[0]
		rte.pending = rte.pending[1:]
		return out, true
	}
	return nil, false
}

// --- Keyboard handling ---

func (rte *RichTextEditor) processKey(gtx layout.Context, e key.Event) RichTextEvent {
	// Any key press resets caret blink to visible
	rte.resetCaretBlink()

	hasShift := e.Modifiers&key.ModShift != 0
	hasShortcut := e.Modifiers&key.ModShortcut != 0

	if hasShortcut {
		return rte.processShortcut(gtx, e)
	}

	switch e.Name {
	case key.NameLeftArrow:
		if hasShift {
			if rte.selStart == rte.selEnd {
				rte.selStart = rte.caretPos
				rte.selEnd = rte.caretPos
			}
			if rte.caretPos > 0 {
				rte.caretPos--
			}
			rte.selEnd = rte.caretPos
		} else {
			if rte.HasSelection() {
				from, _ := rte.Selection()
				rte.caretPos = from
			} else if rte.caretPos > 0 {
				rte.caretPos--
			}
			rte.selStart = 0
			rte.selEnd = 0
		}
		rte.resetCaretBlink()
		return RichTextCaretEvent{Position: rte.caretPos}

	case key.NameRightArrow:
		if hasShift {
			if rte.selStart == rte.selEnd {
				rte.selStart = rte.caretPos
				rte.selEnd = rte.caretPos
			}
			runes := rte.buffer.Runes()
			if rte.caretPos < len(runes) {
				rte.caretPos++
			}
			rte.selEnd = rte.caretPos
		} else {
			if rte.HasSelection() {
				_, to := rte.Selection()
				rte.caretPos = to
			} else {
				runes := rte.buffer.Runes()
				if rte.caretPos < len(runes) {
					rte.caretPos++
				}
			}
			rte.selStart = 0
			rte.selEnd = 0
		}
		rte.resetCaretBlink()
		return RichTextCaretEvent{Position: rte.caretPos}

	case key.NameUpArrow:
		if hasShift {
			if rte.selStart == rte.selEnd {
				rte.selStart = rte.caretPos
				rte.selEnd = rte.caretPos
			}
			rte.caretPos = rte.layout.MoveToLine(rte.caretPos, -1)
			rte.selEnd = rte.caretPos
		} else {
			rte.caretPos = rte.layout.MoveToLine(rte.caretPos, -1)
			rte.selStart = 0
			rte.selEnd = 0
		}
		rte.resetCaretBlink()
		return RichTextCaretEvent{Position: rte.caretPos}

	case key.NameDownArrow:
		if hasShift {
			if rte.selStart == rte.selEnd {
				rte.selStart = rte.caretPos
				rte.selEnd = rte.caretPos
			}
			rte.caretPos = rte.layout.MoveToLine(rte.caretPos, 1)
			rte.selEnd = rte.caretPos
		} else {
			rte.caretPos = rte.layout.MoveToLine(rte.caretPos, 1)
			rte.selStart = 0
			rte.selEnd = 0
		}
		rte.resetCaretBlink()
		return RichTextCaretEvent{Position: rte.caretPos}

	case key.NameHome:
		rte.caretPos = 0
		rte.selStart = 0
		rte.selEnd = 0
		return RichTextCaretEvent{Position: rte.caretPos}

	case key.NameEnd:
		rte.caretPos = rte.buffer.Len()
		rte.selStart = 0
		rte.selEnd = 0
		return RichTextCaretEvent{Position: rte.caretPos}

	case key.NameDeleteBackward:
		if rte.HasSelection() {
			from, to := rte.Selection()
			rte.buffer.DeleteRange(from, to)
			rte.caretPos = from
		} else if rte.caretPos > 0 {
			rte.buffer.DeleteRange(rte.caretPos-1, rte.caretPos)
			rte.caretPos--
		}
		rte.selStart = 0
		rte.selEnd = 0
		rte.layoutDirty = true
		return RichTextInputEvent{Buffer: &rte.buffer}

	case key.NameDeleteForward:
		if rte.HasSelection() {
			from, to := rte.Selection()
			rte.buffer.DeleteRange(from, to)
			rte.caretPos = from
		} else {
			runes := rte.buffer.Runes()
			if rte.caretPos < len(runes) {
				rte.buffer.DeleteRange(rte.caretPos, rte.caretPos+1)
			}
		}
		rte.selStart = 0
		rte.selEnd = 0
		rte.layoutDirty = true
		return RichTextInputEvent{Buffer: &rte.buffer}

	case key.NameReturn, key.NameEnter:
		rte.insertText("\n")
		return RichTextInputEvent{Buffer: &rte.buffer}
	}



	return nil
}

func (rte *RichTextEditor) processShortcut(gtx layout.Context, e key.Event) RichTextEvent {
	rte.resetCaretBlink()

	switch e.Name {
	case "A":
		rte.caretPos = rte.buffer.Len()
		rte.selStart = 0
		rte.selEnd = rte.buffer.Len()
		return RichTextCaretEvent{Position: rte.caretPos}

	case "C":
		if rte.HasSelection() {
			text := rte.SelectedText()
			if text != "" {
				gtx.Execute(clipboard.WriteCmd{
					Type: "application/text",
					Data: &richTextReadCloser{data: text},
				})
			}
		}

	case "X":
		if rte.HasSelection() {
			text := rte.SelectedText()
			if text != "" {
				gtx.Execute(clipboard.WriteCmd{
					Type: "application/text",
					Data: &richTextReadCloser{data: text},
				})
			}
			from, to := rte.Selection()
			rte.buffer.DeleteRange(from, to)
			rte.caretPos = from
			rte.selStart = 0
			rte.selEnd = 0
			rte.layoutDirty = true
			return RichTextInputEvent{Buffer: &rte.buffer}
		}

	case "V":
		gtx.Execute(clipboard.ReadCmd{
			Tag: rte.focusTag,
		})

	case "Z":
		// TODO: undo

	case "Home":
		rte.caretPos = 0
		rte.selStart = 0
		rte.selEnd = 0
		return RichTextCaretEvent{Position: rte.caretPos}

	case "End":
		rte.caretPos = rte.buffer.Len()
		rte.selStart = 0
		rte.selEnd = 0
		return RichTextCaretEvent{Position: rte.caretPos}
	}

	return nil
}

func (rte *RichTextEditor) insertText(text string) {
	rte.resetCaretBlink()
	if rte.HasSelection() {
		from, to := rte.Selection()
		rte.buffer.ReplaceRange(from, to, text, rte.currentStyle)
		rte.caretPos = from + len([]rune(text))
	} else {
		rte.buffer.InsertText(rte.caretPos, text, rte.currentStyle)
		rte.caretPos += len([]rune(text))
	}
	rte.selStart = 0
	rte.selEnd = 0
	rte.layoutDirty = true
}

// richTextReadCloser wraps a string as an io.ReadCloser for clipboard operations.
type richTextReadCloser struct {
	data string
	pos  int
}

func (rc *richTextReadCloser) Read(p []byte) (n int, err error) {
	if rc.pos >= len(rc.data) {
		return 0, nil
	}
	n = copy(p, rc.data[rc.pos:])
	rc.pos += n
	return n, nil
}

func (rc *richTextReadCloser) Close() error { return nil }
