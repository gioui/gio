// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"image"
	"image/color"
	"math"
	"time"
	"unicode/utf8"

	"gioui.org/ui"
	"gioui.org/ui/gesture"
	"gioui.org/ui/input"
	"gioui.org/ui/key"
	"gioui.org/ui/layout"
	"gioui.org/ui/paint"
	"gioui.org/ui/pointer"

	"golang.org/x/image/math/fixed"
)

// Editor implements an editable and scrollable text area.
type Editor struct {
	Face      Face
	Alignment Alignment
	// SingleLine force the text to stay on a single line.
	// SingleLine also sets the scrolling direction to
	// horizontal.
	SingleLine bool
	// Submit enabled translation of carriage return keys to SubmitEvents.
	// If not enabled, carriage returns are inserted as newlines in the text.
	Submit bool

	// Material for drawing the text.
	Material ui.MacroOp
	// Hint contains the text displayed to the user when the
	// Editor is empty.
	Hint string
	// Mmaterial is used to draw the hint.
	HintMaterial ui.MacroOp

	oldScale          int
	blinkStart        time.Time
	focused           bool
	rr                editBuffer
	maxWidth          int
	viewSize          image.Point
	valid             bool
	lines             []Line
	dims              layout.Dimens
	padTop, padBottom int
	padLeft, padRight int
	requestFocus      bool

	it lineIterator

	// carXOff is the offset to the current caret
	// position when moving between lines.
	carXOff fixed.Int26_6

	scroller  gesture.Scroll
	scrollOff image.Point

	clicker gesture.Click
}

type EditorEvent interface {
	isEditorEvent()
}

// A ChangeEvent is generated for every user change to the text.
type ChangeEvent struct{}

// A SubmitEvent is generated when and Editor's Submit is set
// and a carriage return key is pressed.
type SubmitEvent struct{}

const (
	blinksPerSecond  = 1
	maxBlinkDuration = 10 * time.Second
)

// Next returns the next available editor event, or false if none are available.
func (e *Editor) Next(cfg ui.Config, queue input.Queue) (EditorEvent, bool) {
	// Crude configuration change detection.
	if scale := cfg.Px(ui.Sp(100)); scale != e.oldScale {
		e.invalidate()
		e.oldScale = scale
	}
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
	sdist := e.scroller.Scroll(cfg, queue, axis)
	var soff int
	if e.SingleLine {
		e.scrollOff.X += sdist
		soff = e.scrollOff.X
	} else {
		e.scrollOff.Y += sdist
		soff = e.scrollOff.Y
	}
	for evt, ok := e.clicker.Next(queue); ok; evt, ok = e.clicker.Next(queue) {
		switch {
		case evt.Type == gesture.TypePress && evt.Source == pointer.Mouse,
			evt.Type == gesture.TypeClick && evt.Source == pointer.Touch:
			e.blinkStart = cfg.Now()
			e.moveCoord(image.Point{
				X: int(math.Round(float64(evt.Position.X))),
				Y: int(math.Round(float64(evt.Position.Y))),
			})
			e.requestFocus = true
			if e.scroller.State() != gesture.StateFlinging {
				e.scrollToCaret(cfg)
			}
		}
	}
	if (sdist > 0 && soff >= smax) || (sdist < 0 && soff <= smin) {
		e.scroller.Stop()
	}
	for ke, ok := queue.Next(e); ok; ke, ok = queue.Next(e) {
		e.blinkStart = cfg.Now()
		switch ke := ke.(type) {
		case key.FocusEvent:
			e.focused = ke.Focus
		case key.Event:
			if !e.focused {
				break
			}
			if e.Submit && (ke.Name == key.NameReturn || ke.Name == key.NameEnter) {
				if !ke.Modifiers.Contain(key.ModShift) {
					return SubmitEvent{}, true
				}
			}
			if e.command(ke) {
				e.scrollToCaret(cfg)
				e.scroller.Stop()
			}
		case key.EditEvent:
			e.scrollToCaret(cfg)
			e.scroller.Stop()
			e.append(ke.Text)
		}
		if e.rr.Changed() {
			return ChangeEvent{}, true
		}
	}
	return nil, false
}

func (e *Editor) caretWidth(c ui.Config) fixed.Int26_6 {
	oneDp := c.Px(ui.Dp(1))
	return fixed.Int26_6(oneDp * 64)
}

// Focus requests the input focus for the Editor.
func (e *Editor) Focus() {
	e.requestFocus = true
}

func (e *Editor) Layout(cfg ui.Config, queue input.Queue, ops *ui.Ops, cs layout.Constraints) layout.Dimens {
	for _, ok := e.Next(cfg, queue); ok; _, ok = e.Next(cfg, queue) {
	}
	twoDp := cfg.Px(ui.Dp(2))
	e.padLeft, e.padRight = twoDp, twoDp
	maxWidth := cs.Width.Max
	if e.SingleLine {
		maxWidth = inf
	}
	if maxWidth != inf {
		maxWidth -= e.padLeft + e.padRight
	}
	if maxWidth != e.maxWidth {
		e.maxWidth = maxWidth
		e.invalidate()
	}

	e.layout()
	lines, size := e.lines, e.dims.Size
	e.viewSize = cs.Constrain(size)

	carLine, _, carX, carY := e.layoutCaret()

	off := image.Point{
		X: -e.scrollOff.X + e.padLeft,
		Y: -e.scrollOff.Y + e.padTop,
	}
	clip := image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: e.viewSize.X, Y: e.viewSize.Y},
	}
	key.InputOp{Key: e, Focus: e.requestFocus}.Add(ops)
	e.requestFocus = false
	e.it = lineIterator{
		Lines:     lines,
		Clip:      clip,
		Alignment: e.Alignment,
		Width:     e.viewWidth(),
		Offset:    off,
	}
	var stack ui.StackOp
	stack.Push(ops)
	// Apply material. Set a default color in case the material is empty.
	if e.rr.len() > 0 {
		paint.ColorOp{Color: color.RGBA{A: 0xff}}.Add(ops)
		e.Material.Add(ops)
	} else {
		paint.ColorOp{Color: color.RGBA{A: 0xaa}}.Add(ops)
		e.HintMaterial.Add(ops)
	}
	for {
		str, lineOff, ok := e.it.Next()
		if !ok {
			break
		}
		var stack ui.StackOp
		stack.Push(ops)
		ui.TransformOp{}.Offset(lineOff).Add(ops)
		e.Face.Path(str).Add(ops)
		paint.PaintOp{Rect: toRectF(clip).Sub(lineOff)}.Add(ops)
		stack.Pop()
	}
	if e.focused {
		now := cfg.Now()
		dt := now.Sub(e.blinkStart)
		blinking := dt < maxBlinkDuration
		const timePerBlink = time.Second / blinksPerSecond
		nextBlink := now.Add(timePerBlink/2 - dt%(timePerBlink/2))
		on := !blinking || dt%timePerBlink < timePerBlink/2
		if on {
			carWidth := e.caretWidth(cfg)
			carX -= carWidth / 2
			carAsc, carDesc := -lines[carLine].Bounds.Min.Y, lines[carLine].Bounds.Max.Y
			carRect := image.Rectangle{
				Min: image.Point{X: carX.Ceil(), Y: carY - carAsc.Ceil()},
				Max: image.Point{X: carX.Ceil() + carWidth.Ceil(), Y: carY + carDesc.Ceil()},
			}
			carRect = carRect.Add(image.Point{
				X: -e.scrollOff.X + e.padLeft,
				Y: -e.scrollOff.Y + e.padTop,
			})
			carRect = clip.Intersect(carRect)
			if !carRect.Empty() {
				paint.ColorOp{Color: color.RGBA{A: 0xff}}.Add(ops)
				e.Material.Add(ops)
				paint.PaintOp{Rect: toRectF(carRect)}.Add(ops)
			}
		}
		if blinking {
			redraw := ui.InvalidateOp{At: nextBlink}
			redraw.Add(ops)
		}
	}
	stack.Pop()

	baseline := e.padTop + e.dims.Baseline
	pointerPadding := cfg.Px(ui.Dp(4))
	r := image.Rectangle{Max: e.viewSize}
	r.Min.X -= pointerPadding
	r.Min.Y -= pointerPadding
	r.Max.X += pointerPadding
	r.Max.X += pointerPadding
	pointer.RectAreaOp{Rect: r}.Add(ops)
	e.scroller.Add(ops)
	e.clicker.Add(ops)
	return layout.Dimens{Size: e.viewSize, Baseline: baseline}
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

func (e *Editor) layout() {
	e.adjustScroll()
	if e.valid {
		return
	}
	e.layoutText()
	e.valid = true
}

func (e *Editor) scrollBounds() image.Rectangle {
	var b image.Rectangle
	if e.SingleLine {
		if len(e.lines) > 0 {
			b.Min.X = align(e.Alignment, e.lines[0].Width, e.viewWidth()).Floor()
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

func (e *Editor) moveCoord(pos image.Point) {
	e.layout()
	var (
		prevDesc fixed.Int26_6
		carLine  int
		y        int
	)
	for _, l := range e.lines {
		y += (prevDesc + l.Ascent).Ceil()
		prevDesc = l.Descent
		if y+prevDesc.Ceil() >= pos.Y+e.scrollOff.Y-e.padTop {
			break
		}
		carLine++
	}
	x := fixed.I(pos.X + e.scrollOff.X - e.padLeft)
	e.moveToLine(x, carLine)
}

func (e *Editor) layoutText() {
	s := e.rr.String()
	if s == "" {
		s = e.Hint
	}
	textLayout := e.Face.Layout(s, LayoutOptions{SingleLine: e.SingleLine, MaxWidth: e.maxWidth})
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
	padTop, padBottom := textPadding(lines)
	dims.Size.Y += padTop + padBottom
	dims.Size.X += e.padLeft + e.padRight
	e.padTop = padTop
	e.padBottom = padBottom
	e.lines, e.dims = lines, dims
}

func (e *Editor) viewWidth() int {
	return e.viewSize.X - e.padLeft - e.padRight
}

func (e *Editor) layoutCaret() (carLine, carCol int, x fixed.Int26_6, y int) {
	e.layout()
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
	x += align(e.Alignment, e.lines[carLine].Width, e.viewWidth())
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
	e.layout()
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
	e.layout()
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
	carX2 := align(e.Alignment, l2.Width, e.viewWidth())
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
	a := align(e.Alignment, l.Width, e.viewWidth())
	e.carXOff = l.Width + a - x
}

func (e *Editor) scrollToCaret(cfg ui.Config) {
	carWidth := e.caretWidth(cfg)
	carLine, _, x, y := e.layoutCaret()
	l := e.lines[carLine]
	if e.SingleLine {
		minx := (x - carWidth/2).Ceil()
		if d := minx - e.scrollOff.X + e.padLeft; d < 0 {
			e.scrollOff.X += d
		}
		maxx := (x + carWidth/2).Ceil()
		if d := maxx - (e.scrollOff.X + e.viewSize.X - e.padRight); d > 0 {
			e.scrollOff.X += d
		}
	} else {
		miny := y + l.Bounds.Min.Y.Floor()
		if d := miny - e.scrollOff.Y + e.padTop; d < 0 {
			e.scrollOff.Y += d
		}
		maxy := y + l.Bounds.Max.Y.Ceil()
		if d := maxy - (e.scrollOff.Y + e.viewSize.Y - e.padBottom); d > 0 {
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
