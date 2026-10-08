package widget

import (
	"image"
	"image/color"
	"time"
)

// Tooltip tracks hover state and computes tooltip position for any trigger element.
// Usage:
//
//	var tip widget.Tooltip
//	// In event handling:
//	tip.Update(gtx, triggerRect, hoverDelay)
//	// In drawing:
//	if tip.Visible() {
//	    pos, bg, fg, border := tip.Style()
//	    drawTooltip(pos, bg, fg, border, tip.Label)
//	}
type Tooltip struct {
	// Label is the text to display in the tooltip.
	Label string

	// Style fields — set before calling Update or use defaults.
	Bg           color.NRGBA
	TextColor    color.NRGBA
	BorderColor  color.NRGBA
	HoverDelay   time.Duration // delay before showing (0 = immediate)
	Pos          TooltipPos    // where to place relative to trigger

	// Internal state
	hovering    bool
	hoverStart  time.Time
	visible     bool
	triggerRect image.Rectangle
	canvasRect  image.Rectangle
}

// TooltipPos specifies where the tooltip appears relative to the trigger.
type TooltipPos int

const (
	TooltipLeft   TooltipPos = iota // left of trigger (default)
	TooltipRight                     // right of trigger
	TooltipTop                       // above trigger
	TooltipBottom                    // below trigger
)

// DefaultTooltipBg returns a dark tooltip background color.
func DefaultTooltipBg() color.NRGBA {
	return color.NRGBA{R: 30, G: 30, B: 40, A: 230}
}

// DefaultTooltipFg returns the tooltip text color.
func DefaultTooltipFg() color.NRGBA {
	return color.NRGBA{R: 220, G: 220, B: 230, A: 255}
}

// DefaultTooltipBorder returns the tooltip border color.
func DefaultTooltipBorder() color.NRGBA {
	return color.NRGBA{R: 80, G: 80, B: 100, A: 120}
}

// SetHover通知 tooltip that the cursor is over the trigger element.
func (t *Tooltip) SetHover(inside bool) {
	if inside && !t.hovering {
		t.hovering = true
		t.hoverStart = time.Now()
	} else if !inside {
		t.hovering = false
		t.visible = false
	}
}

// Update checks hover delay and computes visibility.
// Call this once per frame.
func (t *Tooltip) Update(trigger image.Rectangle, canvas image.Rectangle) {
	t.triggerRect = trigger
	t.canvasRect = canvas

	if t.hovering {
		delay := t.HoverDelay
		if delay == 0 {
			delay = 400 * time.Millisecond
		}
		if time.Since(t.hoverStart) >= delay {
			t.visible = true
		}
	} else {
		t.visible = false
	}
}

// Visible returns true if the tooltip should be drawn.
func (t *Tooltip) Visible() bool {
	return t.visible && t.Label != ""
}

// Position returns the screen rectangle where the tooltip should be drawn,
// along with the background, text, and border colors.
func (t *Tooltip) Position() (rect image.Rectangle, bg, fg, border color.NRGBA) {
	bg = t.Bg
	if bg == (color.NRGBA{}) {
		bg = DefaultTooltipBg()
	}
	fg = t.TextColor
	if fg == (color.NRGBA{}) {
		fg = DefaultTooltipFg()
	}
	border = t.BorderColor
	if border == (color.NRGBA{}) {
		border = DefaultTooltipBorder()
	}

	// Approximate text dimensions (7px per rune at default size, 20px height)
	runeCount := len([]rune(t.Label))
	textW := runeCount*8 + 12
	textH := 22

	// Position based on preference
	var tx, ty int
	switch t.Pos {
	case TooltipRight:
		tx = t.triggerRect.Max.X + 6
		ty = t.triggerRect.Min.Y + (t.triggerRect.Dy()-textH)/2
	case TooltipTop:
		tx = t.triggerRect.Min.X + (t.triggerRect.Dx()-textW)/2
		ty = t.triggerRect.Min.Y - textH - 6
	case TooltipBottom:
		tx = t.triggerRect.Min.X + (t.triggerRect.Dx()-textW)/2
		ty = t.triggerRect.Max.Y + 6
	default: // TooltipLeft
		tx = t.triggerRect.Min.X - textW - 6
		ty = t.triggerRect.Min.Y + (t.triggerRect.Dy()-textH)/2
	}

	// Clamp to canvas
	if tx < t.canvasRect.Min.X {
		tx = t.triggerRect.Max.X + 6
	}
	if tx+textW > t.canvasRect.Max.X {
		tx = t.canvasRect.Max.X - textW
	}
	if ty < t.canvasRect.Min.Y {
		ty = t.canvasRect.Min.Y
	}
	if ty+textH > t.canvasRect.Max.Y {
		ty = t.canvasRect.Max.Y - textH
	}

	rect = image.Rect(tx, ty, tx+textW, ty+textH)
	return
}
