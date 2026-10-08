package widget

import "image/color"

// HoverHighlight tracks hover state and computes a highlight overlay color
// with configurable strength (0..1), base color, and opacity (0..255).
//
// Usage:
//
//	var hl widget.HoverHighlight
//	hl.Color = color.NRGBA{R: 255, G: 255, B: 255}
//	hl.Opacity = 20
//	hl.Strength = 0.15
//
//	// In event handling:
//	hl.SetHover(cursorInsideRect)
//
//	// In drawing:
//	if hl.Active() {
//	    paint.FillShape(gtx.Ops, hl.Result(), clip.Rect(rect).Op(gtx.Ops))
//	}
type HoverHighlight struct {
	// Color is the base highlight color (default: white).
	Color color.NRGBA

	// Opacity is the maximum alpha value (0-255) when fully hovered.
	// The actual alpha is interpolated from 0 to Opacity based on Strength.
	Opacity uint8

	// Strength controls how intense the highlight is (0.0 = invisible, 1.0 = full Opacity).
	// This is the target value when hovered; transitions are handled externally.
	Strength float32

	// HoverDelay is unused for highlights (they appear immediately),
	// but reserved for future animated transitions.

	// internal state
	hovering bool
	hoverAmt float32 // current interpolation 0..1
}

// DefaultHoverHighlight returns a white subtle hover highlight.
func DefaultHoverHighlight() HoverHighlight {
	return HoverHighlight{
		Color:    color.NRGBA{R: 255, G: 255, B: 255},
		Opacity:  20,
		Strength: 0.15,
	}
}

// BlueHoverHighlight returns a blue-tinted hover highlight.
func BlueHoverHighlight() HoverHighlight {
	return HoverHighlight{
		Color:    color.NRGBA{R: 100, G: 160, B: 255},
		Opacity:  25,
		Strength: 0.2,
	}
}

// SetHover notifies the highlight that the cursor is inside (true) or outside (false) the element.
func (h *HoverHighlight) SetHover(inside bool) {
	h.hovering = inside
}

// Update advances the hover interpolation. Call once per frame.
// factor controls transition speed (0..1 per frame, e.g. 0.15 for smooth, 1.0 for instant).
func (h *HoverHighlight) Update(factor float32) {
	if h.hovering {
		h.hoverAmt += (h.Strength - h.hoverAmt) * factor
		if h.hoverAmt < 0.001 {
			h.hoverAmt = 0
		}
	} else {
		h.hoverAmt -= h.hoverAmt * factor
		if h.hoverAmt < 0.001 {
			h.hoverAmt = 0
		}
	}
}

// Active returns true if the highlight has any visible intensity.
func (h *HoverHighlight) Active() bool {
	return h.hoverAmt > 0.001
}

// Result returns the interpolated highlight color with the computed alpha.
// The alpha is: Opacity * hoverAmt / Strength (clamped to 0..Opacity).
func (h HoverHighlight) Result() color.NRGBA {
	if h.hoverAmt < 0.001 {
		return color.NRGBA{} // fully transparent
	}
	strength := h.Strength
	if strength < 0.001 {
		strength = 1
	}
	alpha := float32(h.Opacity) * h.hoverAmt / strength
	if alpha > float32(h.Opacity) {
		alpha = float32(h.Opacity)
	}
	return color.NRGBA{
		R: h.Color.R,
		G: h.Color.G,
		B: h.Color.B,
		A: uint8(alpha),
	}
}

// Amount returns the current interpolation value (0..Strength).
// Useful for custom rendering that needs the raw intensity.
func (h HoverHighlight) Amount() float32 {
	return h.hoverAmt
}

// Reset forces the highlight to zero (e.g. when element becomes invisible).
func (h *HoverHighlight) Reset() {
	h.hovering = false
	h.hoverAmt = 0
}

// HoverGroup manages hover state for a set of indexed elements (e.g. toolbar buttons).
// Only one element can be hovered at a time.
type HoverGroup struct {
	highlights []HoverHighlight
	hovered    int // -1 = none
}

// NewHoverGroup creates a HoverGroup with n elements, all sharing the same style.
func NewHoverGroup(n int, style HoverHighlight) HoverGroup {
	h := make([]HoverHighlight, n)
	for i := range h {
		h[i] = style
	}
	return HoverGroup{highlights: h, hovered: -1}
}

// SetHovered sets which element index is hovered (-1 = none).
func (g *HoverGroup) SetHovered(idx int) {
	g.hovered = idx
	for i := range g.highlights {
		g.highlights[i].SetHover(i == idx)
	}
}

// Update advances all highlights. Call once per frame.
func (g *HoverGroup) Update(factor float32) {
	for i := range g.highlights {
		g.highlights[i].Update(factor)
	}
}

// Get returns the highlight state for element i.
func (g HoverGroup) Get(i int) *HoverHighlight {
	if i < 0 || i >= len(g.highlights) {
		return nil
	}
	return &g.highlights[i]
}

// HoveredIndex returns the currently hovered element index (-1 = none).
func (g HoverGroup) HoveredIndex() int {
	return g.hovered
}
