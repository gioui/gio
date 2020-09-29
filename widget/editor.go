// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"bufio"
	"image"
	"io"
	"math"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
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

	caret struct {
		on     bool
		scroll bool

		// xoff is the offset to the current caret
		// position when moving between lines.
		xoff fixed.Int26_6

		// line is the caret line position as an index into lines.
		line int
		// col is the caret column measured in runes.
		col int
		// (x, y) are the caret coordinates.
		x fixed.Int26_6
		y int
	}

	scroller  gesture.Scroll
	scrollOff image.Point

	clicker gesture.Click

	// events is the list of events not yet processed.
	events []EditorEvent
	// prevEvents is the number of events from the previous frame.
	prevEvents int
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

type line struct {
	offset f32.Point
	clip   op.CallOp
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
	e.processPointer(gtx)
	e.processKey(gtx)
}

func (e *Editor) makeValid() {
	if e.valid {
		return
	}
	e.lines, e.dims = e.layoutText(e.shaper)
	line, col, x, y := e.layoutCaret()
	e.caret.line = line
	e.caret.col = col
	e.caret.x = x
	e.caret.y = y
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
	for _, evt := range e.clicker.Events(gtx) {
		switch {
		case evt.Type == gesture.TypePress && evt.Source == pointer.Mouse,
			evt.Type == gesture.TypeClick && evt.Source == pointer.Touch:
			e.blinkStart = gtx.Now
			e.moveCoord(image.Point{
				X: int(math.Round(float64(evt.Position.X))),
				Y: int(math.Round(float64(evt.Position.Y))),
			})
			e.requestFocus = true
			if e.scroller.State() != gesture.StateFlinging {
				e.caret.scroll = true
			}
		}
	}
	if (sdist > 0 && soff >= smax) || (sdist < 0 && soff <= smin) {
		e.scroller.Stop()
	}
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
		case key.Event:
			if !e.focused {
				break
			}
			if e.Submit && (ke.Name == key.NameReturn || ke.Name == key.NameEnter) {
				if !ke.Modifiers.Contain(key.ModShift) {
					e.events = append(e.events, SubmitEvent{
						Text: e.Text(),
					})
					return
				}
			}
			if e.command(ke) {
				e.caret.scroll = true
				e.scroller.Stop()
			}
		case key.EditEvent:
			e.caret.scroll = true
			e.scroller.Stop()
			e.append(ke.Text)
		}
		if e.rr.Changed() {
			e.events = append(e.events, ChangeEvent{})
		}
	}
}

func (e *Editor) moveLines(distance int) {
	e.moveToLine(e.caret.x+e.caret.xoff, e.caret.line+distance)
}

func (e *Editor) command(k key.Event) bool {
	modSkip := key.ModCtrl
	if runtime.GOOS == "darwin" {
		modSkip = key.ModAlt
	}
	switch k.Name {
	case key.NameReturn, key.NameEnter:
		e.append("\n")
	case key.NameDeleteBackward:
		if k.Modifiers == modSkip {
			e.deleteWord(-1)
		} else {
			e.Delete(-1)
		}
	case key.NameDeleteForward:
		if k.Modifiers == modSkip {
			e.deleteWord(1)
		} else {
			e.Delete(1)
		}
	case key.NameUpArrow:
		e.moveLines(-1)
	case key.NameDownArrow:
		e.moveLines(+1)
	case key.NameLeftArrow:
		if k.Modifiers == modSkip {
			e.moveWord(-1)
		} else {
			e.Move(-1)
		}
	case key.NameRightArrow:
		if k.Modifiers == modSkip {
			e.moveWord(1)
		} else {
			e.Move(1)
		}
	case key.NamePageUp:
		e.movePages(-1)
	case key.NamePageDown:
		e.movePages(+1)
	case key.NameHome:
		e.moveStart()
	case key.NameEnd:
		e.moveEnd()
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

// Layout lays out the editor.
func (e *Editor) Layout(gtx layout.Context, sh text.Shaper, font text.Font, size unit.Value) layout.Dimensions {
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

	return e.layout(gtx)
}

func (e *Editor) layout(gtx layout.Context) layout.Dimensions {
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
	clip := textPadding(e.lines)
	clip.Max = clip.Max.Add(e.viewSize)
	it := lineIterator{
		Lines:     e.lines,
		Clip:      clip,
		Alignment: e.Alignment,
		Width:     e.viewSize.X,
		Offset:    off,
	}
	e.shapes = e.shapes[:0]
	for {
		_, _, layout, off, ok := it.Next()
		if !ok {
			break
		}
		path := e.shaper.Shape(e.font, e.textSize, layout)
		e.shapes = append(e.shapes, line{off, path})
	}

	key.InputOp{Tag: &e.eventKey, Focus: e.requestFocus}.Add(gtx.Ops)
	e.requestFocus = false
	pointerPadding := gtx.Px(unit.Dp(4))
	r := image.Rectangle{Max: e.viewSize}
	r.Min.X -= pointerPadding
	r.Min.Y -= pointerPadding
	r.Max.X += pointerPadding
	r.Max.X += pointerPadding
	pointer.Rect(r).Add(gtx.Ops)
	e.scroller.Add(gtx.Ops)
	e.clicker.Add(gtx.Ops)
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

	return layout.Dimensions{Size: e.viewSize, Baseline: e.dims.Baseline}
}

func (e *Editor) PaintText(gtx layout.Context) {
	clip := textPadding(e.lines)
	clip.Max = clip.Max.Add(e.viewSize)
	for _, shape := range e.shapes {
		stack := op.Push(gtx.Ops)
		op.Offset(shape.offset).Add(gtx.Ops)
		shape.clip.Add(gtx.Ops)
		paint.PaintOp{Rect: layout.FRect(clip).Sub(shape.offset)}.Add(gtx.Ops)
		stack.Pop()
	}
}

func (e *Editor) PaintCaret(gtx layout.Context) {
	if !e.caret.on {
		return
	}
	e.makeValid()
	carWidth := fixed.I(gtx.Px(unit.Dp(1)))
	carX := e.caret.x
	carY := e.caret.y

	defer op.Push(gtx.Ops).Pop()
	carX -= carWidth / 2
	carAsc, carDesc := -e.lines[e.caret.line].Bounds.Min.Y, e.lines[e.caret.line].Bounds.Max.Y
	carRect := image.Rectangle{
		Min: image.Point{X: carX.Ceil(), Y: carY - carAsc.Ceil()},
		Max: image.Point{X: carX.Ceil() + carWidth.Ceil(), Y: carY + carDesc.Ceil()},
	}
	carRect = carRect.Add(image.Point{
		X: -e.scrollOff.X,
		Y: -e.scrollOff.Y,
	})
	clip := textPadding(e.lines)
	// Account for caret width to each side.
	whalf := (carWidth / 2).Ceil()
	if clip.Max.X < whalf {
		clip.Max.X = whalf
	}
	if clip.Min.X > -whalf {
		clip.Min.X = -whalf
	}
	clip.Max = clip.Max.Add(e.viewSize)
	carRect = clip.Intersect(carRect)
	if !carRect.Empty() {
		paint.PaintOp{Rect: layout.FRect(carRect)}.Add(gtx.Ops)
	}
}

// Len is the length of the editor contents.
func (e *Editor) Len() int {
	return e.rr.len()
}

// Text returns the contents of the editor.
func (e *Editor) Text() string {
	return e.rr.String()
}

// SetText replaces the contents of the editor.
func (e *Editor) SetText(s string) {
	e.rr = editBuffer{}
	e.caret.xoff = 0
	e.prepend(s)
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
	var (
		prevDesc fixed.Int26_6
		carLine  int
		y        int
	)
	for _, l := range e.lines {
		y += (prevDesc + l.Ascent).Ceil()
		prevDesc = l.Descent
		if y+prevDesc.Ceil() >= pos.Y+e.scrollOff.Y {
			break
		}
		carLine++
	}
	x := fixed.I(pos.X + e.scrollOff.X)
	e.moveToLine(x, carLine)
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
		if layout := lines[i].Layout; len(layout) > 0 {
			r := layout[len(layout)-1].Rune
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
	e.makeValid()
	return e.caret.line, e.caret.col
}

// CaretCoords returns the coordinates of the caret, relative to the
// editor itself.
func (e *Editor) CaretCoords() f32.Point {
	e.makeValid()
	return f32.Pt(float32(e.caret.x)/64, float32(e.caret.y))
}

func (e *Editor) layoutCaret() (line, col int, x fixed.Int26_6, y int) {
	var idx int
	var prevDesc fixed.Int26_6
loop:
	for {
		x = 0
		col = 0
		l := e.lines[line]
		y += (prevDesc + l.Ascent).Ceil()
		prevDesc = l.Descent
		for _, g := range l.Layout {
			if idx == e.rr.caret {
				break loop
			}
			x += g.Advance
			_, s := e.rr.runeAt(idx)
			idx += s
			col++
		}
		if line == len(e.lines)-1 || idx > e.rr.caret {
			break
		}
		line++
	}
	x += align(e.Alignment, e.lines[line].Width, e.viewSize.X)
	return
}

func (e *Editor) invalidate() {
	e.valid = false
}

// Delete runes from the caret position. The sign of runes specifies the
// direction to delete: positive is forward, negative is backward.
func (e *Editor) Delete(runes int) {
	e.rr.deleteRunes(runes)
	e.caret.xoff = 0
	e.invalidate()
}

// Insert inserts text at the caret, moving the caret forward.
func (e *Editor) Insert(s string) {
	e.append(s)
	e.caret.scroll = true
	e.invalidate()
}

func (e *Editor) append(s string) {
	if e.SingleLine {
		s = strings.ReplaceAll(s, "\n", "")
	}
	e.prepend(s)
	e.rr.caret += len(s)
}

func (e *Editor) prepend(s string) {
	e.rr.prepend(s)
	e.caret.xoff = 0
	e.invalidate()
}

func (e *Editor) movePages(pages int) {
	e.makeValid()
	y := e.caret.y + pages*e.viewSize.Y
	var (
		prevDesc fixed.Int26_6
		carLine2 int
	)
	y2 := e.lines[0].Ascent.Ceil()
	for i := 1; i < len(e.lines); i++ {
		if y2 >= y {
			break
		}
		l := e.lines[i]
		h := (prevDesc + l.Ascent).Ceil()
		prevDesc = l.Descent
		if y2+h-y >= y-y2 {
			break
		}
		y2 += h
		carLine2++
	}
	e.moveToLine(e.caret.x+e.caret.xoff, carLine2)
}

func (e *Editor) moveToLine(x fixed.Int26_6, line int) {
	e.makeValid()
	if line < 0 {
		line = 0
	}
	if line >= len(e.lines) {
		line = len(e.lines) - 1
	}

	prevDesc := e.lines[line].Descent
	for e.caret.line < line {
		e.moveEnd()
		l := e.lines[e.caret.line]
		_, s := e.rr.runeAt(e.rr.caret)
		e.rr.caret += s
		e.caret.y += (prevDesc + l.Ascent).Ceil()
		e.caret.col = 0
		prevDesc = l.Descent
		e.caret.line++
	}
	for e.caret.line > line {
		e.moveStart()
		l := e.lines[e.caret.line]
		_, s := e.rr.runeBefore(e.rr.caret)
		e.rr.caret -= s
		e.caret.y -= (prevDesc + l.Ascent).Ceil()
		prevDesc = l.Descent
		e.caret.line--
		l = e.lines[e.caret.line]
		e.caret.col = len(l.Layout) - 1
	}

	e.moveStart()
	l := e.lines[line]
	e.caret.x = align(e.Alignment, l.Width, e.viewSize.X)
	// Only move past the end of the last line
	end := 0
	if line < len(e.lines)-1 {
		end = 1
	}
	// Move to rune closest to x.
	for i := 0; i < len(l.Layout)-end; i++ {
		g := l.Layout[i]
		if e.caret.x >= x {
			break
		}
		if e.caret.x+g.Advance-x >= x-e.caret.x {
			break
		}
		e.caret.x += g.Advance
		_, s := e.rr.runeAt(e.rr.caret)
		e.rr.caret += s
		e.caret.col++
	}
	e.caret.xoff = x - e.caret.x
}

// Move the caret: positive distance moves forward, negative distance moves
// backward.
func (e *Editor) Move(distance int) {
	e.makeValid()
	for ; distance < 0 && e.rr.caret > 0; distance++ {
		if e.caret.col == 0 {
			// Move to end of previous line.
			e.moveToLine(fixed.I(e.maxWidth), e.caret.line-1)
			continue
		}
		l := e.lines[e.caret.line].Layout
		_, s := e.rr.runeBefore(e.rr.caret)
		e.rr.caret -= s
		e.caret.col--
		e.caret.x -= l[e.caret.col].Advance
	}
	for ; distance > 0 && e.rr.caret < e.rr.len(); distance-- {
		l := e.lines[e.caret.line].Layout
		// Only move past the end of the last line
		end := 0
		if e.caret.line < len(e.lines)-1 {
			end = 1
		}
		if e.caret.col >= len(l)-end {
			// Move to start of next line.
			e.moveToLine(0, e.caret.line+1)
			continue
		}
		e.caret.x += l[e.caret.col].Advance
		_, s := e.rr.runeAt(e.rr.caret)
		e.rr.caret += s
		e.caret.col++
	}
	e.caret.xoff = 0
}

func (e *Editor) moveStart() {
	e.makeValid()
	layout := e.lines[e.caret.line].Layout
	for i := e.caret.col - 1; i >= 0; i-- {
		_, s := e.rr.runeBefore(e.rr.caret)
		e.rr.caret -= s
		e.caret.x -= layout[i].Advance
	}
	e.caret.col = 0
	e.caret.xoff = -e.caret.x
}

func (e *Editor) moveEnd() {
	e.makeValid()
	l := e.lines[e.caret.line]
	// Only move past the end of the last line
	end := 0
	if e.caret.line < len(e.lines)-1 {
		end = 1
	}
	layout := l.Layout
	for i := e.caret.col; i < len(layout)-end; i++ {
		adv := layout[i].Advance
		_, s := e.rr.runeAt(e.rr.caret)
		e.rr.caret += s
		e.caret.x += adv
		e.caret.col++
	}
	a := align(e.Alignment, l.Width, e.viewSize.X)
	e.caret.xoff = l.Width + a - e.caret.x
}

// moveWord moves the caret to the next word in the specified direction.
// Positive is forward, negative is backward.
// Absolute values greater than one will skip that many words.
func (e *Editor) moveWord(distance int) {
	e.makeValid()
	// split the distance information into constituent parts to be
	// used independently.
	words, direction := distance, 1
	if distance < 0 {
		words, direction = distance*-1, -1
	}
	// atEnd if caret is at either side of the buffer.
	atEnd := func() bool {
		return e.rr.caret == 0 || e.rr.caret == e.rr.len()
	}
	// next returns the appropriate rune given the direction.
	next := func() (r rune) {
		if direction < 0 {
			r, _ = e.rr.runeBefore(e.rr.caret)
		} else {
			r, _ = e.rr.runeAt(e.rr.caret)
		}
		return r
	}
	for ii := 0; ii < words; ii++ {
		for r := next(); unicode.IsSpace(r) && !atEnd(); r = next() {
			e.Move(direction)
		}
		e.Move(direction)
		for r := next(); !unicode.IsSpace(r) && !atEnd(); r = next() {
			e.Move(direction)
		}
	}
}

// deleteWord the next word(s) in the specified direction.
// Unlike moveWord, deleteWord treats whitespace as a word itself.
// Positive is forward, negative is backward.
// Absolute values greater than one will delete that many words.
func (e *Editor) deleteWord(distance int) {
	e.makeValid()
	// split the distance information into constituent parts to be
	// used independently.
	words, direction := distance, 1
	if distance < 0 {
		words, direction = distance*-1, -1
	}
	// atEnd if offset is at or beyond either side of the buffer.
	atEnd := func(offset int) bool {
		idx := e.rr.caret + offset*direction
		return idx <= 0 || idx >= e.rr.len()
	}
	// next returns the appropriate rune given the direction and offset.
	next := func(offset int) (r rune) {
		idx := e.rr.caret + offset*direction
		if idx < 0 {
			idx = 0
		} else if idx > e.rr.len() {
			idx = e.rr.len()
		}
		if direction < 0 {
			r, _ = e.rr.runeBefore(idx)
		} else {
			r, _ = e.rr.runeAt(idx)
		}
		return r
	}
	var runes = 1
	for ii := 0; ii < words; ii++ {
		if r := next(runes); unicode.IsSpace(r) {
			for r := next(runes); unicode.IsSpace(r) && !atEnd(runes); r = next(runes) {
				runes += 1
			}
		} else {
			for r := next(runes); !unicode.IsSpace(r) && !atEnd(runes); r = next(runes) {
				runes += 1
			}
		}
	}
	e.Delete(runes * direction)
}

func (e *Editor) scrollToCaret() {
	e.makeValid()
	l := e.lines[e.caret.line]
	if e.SingleLine {
		var dist int
		if d := e.caret.x.Floor() - e.scrollOff.X; d < 0 {
			dist = d
		} else if d := e.caret.x.Ceil() - (e.scrollOff.X + e.viewSize.X); d > 0 {
			dist = d
		}
		e.scrollRel(dist, 0)
	} else {
		miny := e.caret.y - l.Ascent.Ceil()
		maxy := e.caret.y + l.Descent.Ceil()
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

func nullLayout(r io.Reader) ([]text.Line, error) {
	var layout []text.Glyph
	rr := bufio.NewReader(r)
	var rerr error
	var n int
	for {
		r, s, err := rr.ReadRune()
		n += s
		layout = append(layout, text.Glyph{Rune: r})
		if err != nil {
			rerr = err
			break
		}
	}
	return []text.Line{
		{
			Layout: layout,
			Len:    n,
		},
	}, rerr
}

func (s ChangeEvent) isEditorEvent() {}
func (s SubmitEvent) isEditorEvent() {}
