package widget

import (
	"image"
	"image/color"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// Slider is a horizontal draggable bar widget.
// It renders a background, a filled portion, and a draggable handle.
type Slider struct {
	// Value is the current value (0..Max).
	Value float32
	// Max is the maximum value (default 1.0).
	Max float32
	// BarColor is the fill color (default blue).
	BarColor color.NRGBA
	// HandleColor is the handle color (default white).
	HandleColor color.NRGBA
	// TrackColor is the background track color.
	TrackColor color.NRGBA

	// Internal state
	tag     *int
	dragging bool
}

// NewSlider creates a new Slider with default styling.
func NewSlider() *Slider {
	return &Slider{
		Max:         1.0,
		BarColor:    color.NRGBA{R: 100, G: 160, B: 240, A: 180},
		HandleColor: color.NRGBA{R: 220, G: 220, B: 220, A: 255},
		TrackColor:  color.NRGBA{R: 40, G: 40, B: 52, A: 255},
		tag:         new(int),
	}
}

// Layout renders the slider at the given dimensions.
// Returns the slider height (typically 20px).
func (s *Slider) Layout(gtx layout.Context, w, h int, fillRect func(gtx layout.Context, r image.Rectangle, c color.NRGBA)) layout.Dimensions {
	// Background track
	trackR := image.Rect(0, 0, w, h)
	fillRect(gtx, trackR, s.TrackColor)

	// Fill
	if s.Max > 0 {
		fillW := int(float32(w) * s.Value / s.Max)
		if fillW > 0 {
			fillRect(gtx, image.Rect(0, 0, fillW, h), s.BarColor)
		}

		// Handle
		handleW := 6
		handleX := fillW - handleW/2
		if handleX < 0 {
			handleX = 0
		}
		if handleX+handleW > w {
			handleX = w - handleW
		}
		handleR := image.Rect(handleX, -2, handleX+handleW, h+2)
		fillRect(gtx, handleR, s.HandleColor)
	}

	// Register pointer events
	{
		fullR := image.Rect(-4, -4, w+4, h+4)
		stack := clip.Rect(fullR).Push(gtx.Ops)
		event.Op(gtx.Ops, s.tag)
		stack.Pop()
	}

	return layout.Dimensions{Size: image.Pt(w, h)}
}

// Update processes pointer events. Call after Layout.
// Returns true if the value changed.
func (s *Slider) Update(gtx layout.Context, w int) bool {
	filter := pointer.Filter{
		Target: s.tag,
		Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
	}
	ev, ok := gtx.Event(filter)
	if !ok {
		return false
	}
	e, ok := ev.(pointer.Event)
	if !ok {
		return false
	}

	switch e.Kind {
	case pointer.Press, pointer.Drag:
		s.dragging = true
	case pointer.Release, pointer.Cancel:
		s.dragging = false
		return false
	}

	if !s.dragging {
		return false
	}

	oldVal := s.Value
	if w > 0 && s.Max > 0 {
		v := float32(e.Position.X) / float32(w) * s.Max
		if v < 0 {
			v = 0
		}
		if v > s.Max {
			v = s.Max
		}
		s.Value = v
	}

	return s.Value != oldVal
}
