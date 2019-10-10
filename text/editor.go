// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"image"
	"image/color"
	"math"
	"time"
	"unicode/utf8"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"golang.org/x/image/math/fixed"
)

// Editor implements an editable and scrollable text area.
type Editor struct {
	Font Font
	// Material for drawing the text.
	Material op.MacroOp
	// Hint contains the text displayed to the user when the
	// Editor is empty.
	Hint string
	// Material is used to draw the hint.
	HintMaterial op.MacroOp

	Alignment Alignment
	// SingleLine force the text to stay on a single line.
	// SingleLine also sets the scrolling direction to
	// horizontal.
	SingleLine bool
	// Submit enabled translation of carriage return keys to SubmitEvents.
	// If not enabled, carriage returns are inserted as newlines in the text.
	Submit bool

	scale        int
	font         Font
	blinkStart   time.Time
	focused      bool
	rr           editBuffer
	maxWidth     int
	viewSize     image.Point
	valid        bool
	lines        []Line
	dims         layout.Dimensions
	carWidth     fixed.Int26_6
	requestFocus bool
	caretOn      bool
	caretScroll  bool

	// carXOff is the offset to the current caret
	// position when moving between lines.
	carXOff fixed.Int26_6

	scroller  gesture.Scroll
	scrollOff image.Point

	clicker gesture.Click

	// events is the list of events not yet processed.
	events []event.Event
}

type EditorEvent interface {
	isEditorEvent()
}

// A ChangeEvent is generated for every user change to the text.
type ChangeEvent struct{}

// A SubmitEvent is generated when Submit is set
// and a carriage return key is pressed.
type SubmitEvent struct{}

const (
	blinksPerSecond  = 1
	maxBlinkDuration = 10 * time.Second
)

// Event returns the next available editor event, or false if none are available.
func (e *Editor) Event(gtx *layout.Context) (EditorEvent, bool) {
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
	sdist := e.scroller.Scroll(gtx.Config, gtx.Queue, gtx.Now(), axis)
	var soff int
	if e.SingleLine {
		e.scrollOff.X += sdist
		soff = e.scrollOff.X
	} else {
		e.scrollOff.Y += sdist
		soff = e.scrollOff.Y
	}
	for _, evt := range e.clicker.Events(gtx.Queue) {
		switch {
		case evt.Type == gesture.TypePress && evt.Source == pointer.Mouse,
			evt.Type == gesture.TypeClick && evt.Source == pointer.Touch:
			e.blinkStart = gtx.Now()
			e.moveCoord(gtx, image.Point{
				X: int(math.Round(float64(evt.Position.X))),
				Y: int(math.Round(float64(evt.Position.Y))),
			})
			e.requestFocus = true
			if e.scroller.State() != gesture.StateFlinging {
				e.caretScroll = true
			}
		}
	}
	if (sdist > 0 && soff >= smax) || (sdist < 0 && soff <= smin) {
		e.scroller.Stop()
	}
	e.events = append(e.events, gtx.Queue.Events(e)...)
	return e.editorEvent(gtx)
}

func (e *Editor) editorEvent(gtx *layout.Context) (EditorEvent, bool) {
	for len(e.events) > 0 {
		ke := e.events[0]
		copy(e.events, e.events[1:])
		e.events = e.events[:len(e.events)-1]
		e.blinkStart = gtx.Now()
		switch ke := ke.(type) {
		case key.FocusEvent:
			e.focused = ke.Focus
		case key.Event:
			if !e.focused {
				break
			}
			if e.Submit && ke.Name == key.NameReturn || ke.Name == key.NameEnter {
				if !ke.Modifiers.Contain(key.ModShift) {
					return SubmitEvent{}, true
				}
			}
			if e.command(ke) {
				e.caretScroll = true
				e.scroller.Stop()
			}
		case key.EditEvent:
			e.caretScroll = true
			e.scroller.Stop()
			e.append(ke.Text)
		}
		if e.rr.Changed() {
			return ChangeEvent{}, true
		}
	}
	return nil, false
}

// Focus requests the input focus for the Editor.
func (e *Editor) Focus() {
	e.requestFocus = true
}

// Layout flushes any remaining events and lays out the editor.
func (e *Editor) Layout(gtx *layout.Context, s *Shaper) {
	e.layout(gtx, s, e.Font)
	var stack op.StackOp
	stack.Push(gtx.Ops)
	if e.Len() > 0 {
		paint.ColorOp{Color: color.RGBA{A: 0xff}}.Add(gtx.Ops)
		e.Material.Add(gtx.Ops)
	} else {
		paint.ColorOp{Color: color.RGBA{A: 0xaa}}.Add(gtx.Ops)
		e.HintMaterial.Add(gtx.Ops)
	}
	e.draw(gtx, s, e.Font)
	paint.ColorOp{Color: color.RGBA{A: 0xff}}.Add(gtx.Ops)
	e.Material.Add(gtx.Ops)
	e.drawCaret(gtx)
	stack.Pop()
}

func (e *Editor) layout(gtx *layout.Context, s *Shaper, font Font) {
	for _, ok := e.Event(gtx); ok; _, ok = e.Event(gtx) {
	}
	if e.font != font {
		e.invalidate()
		e.font = font
	}
	// Crude configuration change detection.
	if scale := gtx.Px(unit.Sp(100)); scale != e.scale {
		e.invalidate()
		e.scale = scale
	}
	cs := gtx.Constraints
	e.carWidth = fixed.I(gtx.Px(unit.Dp(1)))

	maxWidth := cs.Width.Max
	if e.SingleLine {
		maxWidth = inf
	}
	if maxWidth != e.maxWidth {
		e.maxWidth = maxWidth
		e.invalidate()
	}

	if !e.valid {
		e.layoutText(gtx, s, font)
		e.valid = true
	}

	e.viewSize = cs.Constrain(e.dims.Size)
	e.adjustScroll()

	if e.caretScroll {
		e.caretScroll = false
		e.scrollToCaret()
	}

	key.InputOp{Key: e, Focus: e.requestFocus}.Add(gtx.Ops)
	e.requestFocus = false
	pointerPadding := gtx.Px(unit.Dp(4))
	r := image.Rectangle{Max: e.viewSize}
	r.Min.X -= pointerPadding
	r.Min.Y -= pointerPadding
	r.Max.X += pointerPadding
	r.Max.X += pointerPadding
	pointer.RectAreaOp{Rect: r}.Add(gtx.Ops)
	e.scroller.Add(gtx.Ops)
	e.clicker.Add(gtx.Ops)
	e.caretOn = false
	if e.focused {
		now := gtx.Now()
		dt := now.Sub(e.blinkStart)
		blinking := dt < maxBlinkDuration
		const timePerBlink = time.Second / blinksPerSecond
		nextBlink := now.Add(timePerBlink/2 - dt%(timePerBlink/2))
		if blinking {
			redraw := op.InvalidateOp{At: nextBlink}
			redraw.Add(gtx.Ops)
		}
		e.caretOn = e.focused && (!blinking || dt%timePerBlink < timePerBlink/2)
	}

	gtx.Dimensions = layout.Dimensions{Size: e.viewSize, Baseline: e.dims.Baseline}
}

func (e *Editor) draw(gtx *layout.Context, s *Shaper, font Font) {
	var stack op.StackOp
	stack.Push(gtx.Ops)
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
	for {
		str, lineOff, ok := it.Next()
		if !ok {
			break
		}
		var stack op.StackOp
		stack.Push(gtx.Ops)
		op.TransformOp{}.Offset(lineOff).Add(gtx.Ops)
		s.Shape(gtx, font, str).Add(gtx.Ops)
		paint.PaintOp{Rect: toRectF(clip).Sub(lineOff)}.Add(gtx.Ops)
		stack.Pop()
	}
	stack.Pop()
}

func (e *Editor) drawCaret(gtx *layout.Context) {
	if !e.caretOn {
		return
	}
	carLine, _, carX, carY := e.layoutCaret()

	var stack op.StackOp
	stack.Push(gtx.Ops)
	carX -= e.carWidth / 2
	carAsc, carDesc := -e.lines[carLine].Bounds.Min.Y, e.lines[carLine].Bounds.Max.Y
	carRect := image.Rectangle{
		Min: image.Point{X: carX.Ceil(), Y: carY - carAsc.Ceil()},
		Max: image.Point{X: carX.Ceil() + e.carWidth.Ceil(), Y: carY + carDesc.Ceil()},
	}
	carRect = carRect.Add(image.Point{
		X: -e.scrollOff.X,
		Y: -e.scrollOff.Y,
	})
	clip := textPadding(e.lines)
	// Account for caret width to each side.
	whalf := (e.carWidth / 2).Ceil()
	if clip.Max.X < whalf {
		clip.Max.X = whalf
	}
	if clip.Min.X > -whalf {
		clip.Min.X = -whalf
	}
	clip.Max = clip.Max.Add(e.viewSize)
	carRect = clip.Intersect(carRect)
	if !carRect.Empty() {
		paint.PaintOp{Rect: toRectF(carRect)}.Add(gtx.Ops)
	}
	stack.Pop()
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
	e.carXOff = 0
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

func (e *Editor) adjustScroll() {
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

func (e *Editor) moveCoord(c unit.Converter, pos image.Point) {
	e.adjustScroll()
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
}

func (e *Editor) layoutText(c unit.Converter, s *Shaper, font Font) {
	txt := e.rr.String()
	if txt == "" {
		txt = e.Hint
	}
	opts := LayoutOptions{SingleLine: e.SingleLine, MaxWidth: e.maxWidth}
	textLayout := s.Layout(c, font, txt, opts)
	lines := textLayout.Lines
	dims := linesDimens(lines)
	for i := 0; i < len(lines)-1; i++ {
		s := lines[i].Text.String
		// To avoid layout flickering while editing, assume a soft newline takes
		// up all available space.
		if len(s) > 0 {
			r, _ := utf8.DecodeLastRuneInString(s)
			if r != '\n' {
				dims.Size.X = e.maxWidth
				break
			}
		}
	}
	e.lines, e.dims = lines, dims
}

func (e *Editor) layoutCaret() (carLine, carCol int, x fixed.Int26_6, y int) {
	e.adjustScroll()
	var idx int
	var prevDesc fixed.Int26_6
loop:
	for carLine = 0; carLine < len(e.lines); carLine++ {
		l := e.lines[carLine]
		y += (prevDesc + l.Ascent).Ceil()
		prevDesc = l.Descent
		if carLine == len(e.lines)-1 || idx+len(l.Text.String) > e.rr.caret {
			str := l.Text.String
			for _, adv := range l.Text.Advances {
				if idx == e.rr.caret {
					break loop
				}
				x += adv
				_, s := utf8.DecodeRuneInString(str)
				idx += s
				str = str[s:]
				carCol++
			}
			break
		}
		idx += len(l.Text.String)
	}
	x += align(e.Alignment, e.lines[carLine].Width, e.viewSize.X)
	return
}

func (e *Editor) invalidate() {
	e.valid = false
}

func (e *Editor) deleteRune() {
	e.rr.deleteRune()
	e.carXOff = 0
	e.invalidate()
}

func (e *Editor) deleteRuneForward() {
	e.rr.deleteRuneForward()
	e.carXOff = 0
	e.invalidate()
}

func (e *Editor) append(s string) {
	if e.SingleLine && s == "\n" {
		return
	}
	e.prepend(s)
	e.rr.caret += len(s)
}

func (e *Editor) prepend(s string) {
	e.rr.prepend(s)
	e.carXOff = 0
	e.invalidate()
}

func (e *Editor) movePages(pages int) {
	_, _, carX, carY := e.layoutCaret()
	y := carY + pages*e.viewSize.Y
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
	e.carXOff = e.moveToLine(carX+e.carXOff, carLine2)
}

func (e *Editor) moveToLine(carX fixed.Int26_6, carLine2 int) fixed.Int26_6 {
	carLine, carCol, _, _ := e.layoutCaret()
	if carLine2 < 0 {
		carLine2 = 0
	}
	if carLine2 >= len(e.lines) {
		carLine2 = len(e.lines) - 1
	}
	// Move to start of line.
	for i := carCol - 1; i >= 0; i-- {
		_, s := e.rr.runeBefore(e.rr.caret)
		e.rr.caret -= s
	}
	if carLine2 != carLine {
		// Move to start of line2.
		if carLine2 > carLine {
			for i := carLine; i < carLine2; i++ {
				e.rr.caret += len(e.lines[i].Text.String)
			}
		} else {
			for i := carLine - 1; i >= carLine2; i-- {
				e.rr.caret -= len(e.lines[i].Text.String)
			}
		}
	}
	l2 := e.lines[carLine2]
	carX2 := align(e.Alignment, l2.Width, e.viewSize.X)
	// Only move past the end of the last line
	end := 0
	if carLine2 < len(e.lines)-1 {
		end = 1
	}
	// Move to rune closest to previous horizontal position.
	for i := 0; i < len(l2.Text.Advances)-end; i++ {
		adv := l2.Text.Advances[i]
		if carX2 >= carX {
			break
		}
		if carX2+adv-carX >= carX-carX2 {
			break
		}
		carX2 += adv
		_, s := e.rr.runeAt(e.rr.caret)
		e.rr.caret += s
	}
	return carX - carX2
}

func (e *Editor) moveLeft() {
	e.rr.moveLeft()
	e.carXOff = 0
}

func (e *Editor) moveRight() {
	e.rr.moveRight()
	e.carXOff = 0
}

func (e *Editor) moveStart() {
	carLine, carCol, x, _ := e.layoutCaret()
	advances := e.lines[carLine].Text.Advances
	for i := carCol - 1; i >= 0; i-- {
		_, s := e.rr.runeBefore(e.rr.caret)
		e.rr.caret -= s
		x -= advances[i]
	}
	e.carXOff = -x
}

func (e *Editor) moveEnd() {
	carLine, carCol, x, _ := e.layoutCaret()
	l := e.lines[carLine]
	// Only move past the end of the last line
	end := 0
	if carLine < len(e.lines)-1 {
		end = 1
	}
	for i := carCol; i < len(l.Text.Advances)-end; i++ {
		adv := l.Text.Advances[i]
		_, s := e.rr.runeAt(e.rr.caret)
		e.rr.caret += s
		x += adv
	}
	a := align(e.Alignment, l.Width, e.viewSize.X)
	e.carXOff = l.Width + a - x
}

func (e *Editor) scrollToCaret() {
	carLine, _, x, y := e.layoutCaret()
	l := e.lines[carLine]
	if e.SingleLine {
		if d := x.Floor() - e.scrollOff.X; d < 0 {
			e.scrollOff.X += d
		}
		if d := x.Ceil() - (e.scrollOff.X + e.viewSize.X); d > 0 {
			e.scrollOff.X += d
		}
	} else {
		miny := y - l.Ascent.Ceil()
		if d := miny - e.scrollOff.Y; d < 0 {
			e.scrollOff.Y += d
		}
		maxy := y + l.Descent.Ceil()
		if d := maxy - (e.scrollOff.Y + e.viewSize.Y); d > 0 {
			e.scrollOff.Y += d
		}
	}
}

func (e *Editor) command(k key.Event) bool {
	switch k.Name {
	case key.NameReturn, key.NameEnter:
		e.append("\n")
	case key.NameDeleteBackward:
		e.deleteRune()
	case key.NameDeleteForward:
		e.deleteRuneForward()
	case key.NameUpArrow:
		line, _, carX, _ := e.layoutCaret()
		e.carXOff = e.moveToLine(carX+e.carXOff, line-1)
	case key.NameDownArrow:
		line, _, carX, _ := e.layoutCaret()
		e.carXOff = e.moveToLine(carX+e.carXOff, line+1)
	case key.NameLeftArrow:
		e.moveLeft()
	case key.NameRightArrow:
		e.moveRight()
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

func (s ChangeEvent) isEditorEvent() {}
func (s SubmitEvent) isEditorEvent() {}
