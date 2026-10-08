package widget

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
)

// ColorPicker is a reusable HSV color picker with a color wheel,
// brightness slider, and hex/RGB input fields.
type ColorPicker struct {
	hue        float64 // 0–360
	saturation float64 // 0–1
	value      float64 // 0–1 (brightness)
	visible    bool

	// Pointer tags for event routing
	wheelTag  *int
	brightTag *int
	hTag      *int
	sTag      *int
	vTag      *int

	// Drag tracking
	wheelDragging  bool
	brightDragging bool
	hDragging      bool
	sDragging      bool
	vDragging      bool
	wheelCX, wheelCY int
	brightX, brightY, brightH int

	// Hex input
	hexBuf string

	// Mode: 0=HSV, 1=RGB, 2=Hex
	mode int

	// Layout cache (computed once per frame)
	fieldY     int
	fieldH     int
	fieldGap   int
	barX       int
	barW       int
	labelW     int

	// Callback
	OnColorChange func(c color.NRGBA)
}

// NewColorPicker creates a new ColorPicker.
func NewColorPicker() *ColorPicker {
	return &ColorPicker{
		wheelTag:  new(int),
		brightTag: new(int),
		hTag:      new(int),
		sTag:      new(int),
		vTag:      new(int),
		value:     1.0,
	}
}

func (cp *ColorPicker) Visible() bool   { return cp.visible }
func (cp *ColorPicker) Show()            { cp.visible = true }
func (cp *ColorPicker) Hide()            { cp.visible = false }
func (cp *ColorPicker) Toggle()          { cp.visible = !cp.visible }

// SetColor sets the picker's current color.
func (cp *ColorPicker) SetColor(c color.NRGBA) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	cp.hue, cp.saturation, cp.value = rgbToHSV(r, g, b)
	cp.hexBuf = fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// Color returns the current color.
func (cp *ColorPicker) Color() color.NRGBA {
	r, g, b := hsvToRGB(cp.hue, cp.saturation, cp.value)
	return color.NRGBA{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
		A: 255,
	}
}

const (
	PickerW = 280
	PickerH = 290
	WheelR  = 80
)

// Layout renders the color picker at (0,0).
func (cp *ColorPicker) Layout(gtx layout.Context, fillRect func(gtx layout.Context, r image.Rectangle, c color.NRGBA), drawText func(gtx layout.Context, x, y int, txt string, col color.NRGBA)) layout.Dimensions {
	if !cp.visible {
		return layout.Dimensions{}
	}

	dimCol := color.NRGBA{R: 120, G: 120, B: 140, A: 255}
	brightCol := color.NRGBA{R: 220, G: 220, B: 220, A: 255}

	// Background
	panelR := image.Rect(0, 0, PickerW, PickerH)
	fillRect(gtx, panelR, color.NRGBA{R: 30, G: 30, B: 38, A: 245})
	// Border
	bc := color.NRGBA{R: 70, G: 70, B: 85, A: 255}
	fillRect(gtx, image.Rect(0, 0, PickerW, 1), bc)
	fillRect(gtx, image.Rect(PickerW-1, 0, PickerW, PickerH), bc)
	fillRect(gtx, image.Rect(0, PickerH-1, PickerW, PickerH), bc)
	fillRect(gtx, image.Rect(0, 0, 1, PickerH), bc)

	// ── Color wheel ──
	cp.wheelCX = 20 + WheelR
	cp.wheelCY = 10 + WheelR
	cp.drawWheel(gtx, fillRect)

	// ── Brightness slider ──
	sliderX := cp.wheelCX + WheelR + 14
	sliderW := 18
	cp.brightX = sliderX
	cp.brightY = cp.wheelCY - WheelR
	cp.brightH = WheelR * 2
	cp.drawBrightnessSlider(gtx, fillRect, sliderX, cp.brightY, sliderW, cp.brightH)

	// ── Current color preview ──
	previewX := sliderX + sliderW + 8
	previewY := cp.brightY
	previewSize := 36
	previewR := image.Rect(previewX, previewY, previewX+previewSize, previewY+previewSize)
	fillRect(gtx, previewR, cp.Color())
	bw := color.NRGBA{R: 100, G: 100, B: 120, A: 255}
	fillRect(gtx, image.Rect(previewX, previewY, previewX+previewSize, previewY+1), bw)
	fillRect(gtx, image.Rect(previewX+previewSize-1, previewY, previewX+previewSize, previewY+previewSize), bw)
	fillRect(gtx, image.Rect(previewX, previewY+previewSize-1, previewX+previewSize, previewY+previewSize), bw)
	fillRect(gtx, image.Rect(previewX, previewY, previewX+1, previewY+previewSize), bw)

	// Hex value next to preview
	if drawText != nil {
		drawText(gtx, previewX+previewSize+6, previewY+10, cp.hexBuf, brightCol)
	}

	// ── Mode tabs ──
	tabY := cp.wheelCY + WheelR + 14
	tabW := 70
	modes := [3]string{"HSV", "RGB", "Hex"}
	for i, mode := range modes {
		tx := 10 + i*(tabW+2)
		tabR := image.Rect(tx, tabY, tx+tabW, tabY+22)
		bg := color.NRGBA{R: 45, G: 45, B: 58, A: 255}
		if cp.mode == i {
			bg = color.NRGBA{R: 60, G: 80, B: 130, A: 255}
		}
		fillRect(gtx, tabR, bg)
		if drawText != nil {
			txCol := dimCol
			if cp.mode == i {
				txCol = brightCol
			}
			drawText(gtx, tx+tabW/2-10, tabY+4, mode, txCol)
		}
	}

	// ── Input fields ──
	cp.fieldY = tabY + 28
	cp.fieldH = 20
	cp.fieldGap = 4
	cp.labelW = 18
	cp.barX = 10 + cp.labelW + 4
	cp.barW = PickerW - cp.barX - 12

	switch cp.mode {
	case 0: // HSV — draggable bars
		fields := []struct {
			label string
			val   float64
			max   float64
			tag   *int
		}{
			{"H", cp.hue, 360, cp.hTag},
			{"S", cp.saturation, 1, cp.sTag},
			{"V", cp.value, 1, cp.vTag},
		}
		for fi, f := range fields {
			cp.drawSliderField(gtx, fillRect, drawText, fi, f.label, f.val, f.max, f.tag, dimCol, brightCol)
		}
	case 1: // RGB — draggable bars
		r, g, b := hsvToRGB(cp.hue, cp.saturation, cp.value)
		fields := []struct {
			label string
			val   uint8
			tag   *int
		}{
			{"R", uint8(r * 255), cp.hTag},
			{"G", uint8(g * 255), cp.sTag},
			{"B", uint8(b * 255), cp.vTag},
		}
		for fi, f := range fields {
			cp.drawSliderField(gtx, fillRect, drawText, fi, f.label, float64(f.val), 255, f.tag, dimCol, brightCol)
		}
	case 2: // Hex
		fy := cp.fieldY
		hexR := image.Rect(cp.barX, fy, cp.barX+cp.barW, fy+cp.fieldH)
		fillRect(gtx, hexR, color.NRGBA{R: 40, G: 40, B: 52, A: 255})
		if drawText != nil {
			drawText(gtx, cp.barX+4, fy+3, cp.hexBuf, brightCol)
		}
	}

	return layout.Dimensions{Size: image.Pt(PickerW, PickerH)}
}

// drawSliderField draws a labeled slider bar with draggable handle.
func (cp *ColorPicker) drawSliderField(gtx layout.Context,
	fillRect func(gtx layout.Context, r image.Rectangle, c color.NRGBA),
	drawText func(gtx layout.Context, x, y int, txt string, col color.NRGBA),
	fi int, label string, val, max float64, tag *int,
	dimCol, brightCol color.NRGBA,
) {
	fy := cp.fieldY + fi*(cp.fieldH+cp.fieldGap)

	// Label
	labelR := image.Rect(10, fy, 10+cp.labelW, fy+cp.fieldH)
	fillRect(gtx, labelR, color.NRGBA{R: 50, G: 50, B: 65, A: 255})
	if drawText != nil {
		drawText(gtx, 12, fy+3, label, dimCol)
	}

	// Bar background
	barR := image.Rect(cp.barX, fy, cp.barX+cp.barW, fy+cp.fieldH)
	fillRect(gtx, barR, color.NRGBA{R: 40, G: 40, B: 52, A: 255})

	// Fill
	fillW := int(float64(cp.barW) * val / max)
	if fillW > 0 {
		fillRect(gtx, image.Rect(cp.barX, fy, cp.barX+fillW, fy+cp.fieldH),
			color.NRGBA{R: 100, G: 160, B: 240, A: 150})
	}

	// Handle (draggable)
	handleW := 6
	handleX := cp.barX + fillW - handleW/2
	if handleX < cp.barX {
		handleX = cp.barX
	}
	if handleX+handleW > cp.barX+cp.barW {
		handleX = cp.barX + cp.barW - handleW
	}
	handleR := image.Rect(handleX, fy-1, handleX+handleW, fy+cp.fieldH+1)
	fillRect(gtx, handleR, color.NRGBA{R: 220, G: 220, B: 220, A: 255})

	// Register pointer events for this slider bar
	{
		fullBar := image.Rect(cp.barX, fy-2, cp.barX+cp.barW, fy+cp.fieldH+2)
		stack := clip.Rect(fullBar).Push(gtx.Ops)
		event.Op(gtx.Ops, tag)
		stack.Pop()
	}
}

// drawWheel draws the HSV color wheel with denser rendering.
func (cp *ColorPicker) drawWheel(gtx layout.Context, fillRect func(gtx layout.Context, r image.Rectangle, c color.NRGBA)) {
	cx, cy := cp.wheelCX, cp.wheelCY

	// Draw wheel as concentric rings of colored rectangles for density
	for dist := 1; dist < WheelR; dist += 2 {
		radius := float64(dist)
		circumference := 2 * math.Pi * radius
		// More steps for denser rendering
		steps := int(circumference / 2)
		if steps < 12 {
			steps = 12
		}
		for s := 0; s < steps; s++ {
			angle := float64(s) / float64(steps) * 360
			sat := radius / float64(WheelR)
			r, g, b := hsvToRGB(angle, sat, cp.value)
			c := color.NRGBA{
				R: uint8(r * 255),
				G: uint8(g * 255),
				B: uint8(b * 255),
				A: 255,
			}
			x := cx + int(radius*math.Cos(angle*math.Pi/180))
			y := cy + int(radius*math.Sin(angle*math.Pi/180))
			// Draw 2x2 block for smoother look
			fillRect(gtx, image.Rect(x-1, y-1, x+2, y+2), c)
		}
	}

	// Cursor
	cursorDist := cp.saturation * float64(WheelR)
	cursorX := cx + int(cursorDist*math.Cos(cp.hue*math.Pi/180))
	cursorY := cy + int(cursorDist*math.Sin(cp.hue*math.Pi/180))
	// White ring cursor
	for dx := -4; dx <= 4; dx++ {
		for dy := -4; dy <= 4; dy++ {
			d2 := dx*dx + dy*dy
			if d2 >= 13 && d2 <= 20 {
				fillRect(gtx, image.Rect(cursorX+dx, cursorY+dy, cursorX+dx+1, cursorY+dy+1), color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}

	// Register wheel area for pointer events
	{
		wheelBounds := image.Rect(cx-WheelR, cy-WheelR, cx+WheelR, cy+WheelR)
		stack := clip.Rect(wheelBounds).Push(gtx.Ops)
		event.Op(gtx.Ops, cp.wheelTag)
		stack.Pop()
	}
}

// drawBrightnessSlider draws the vertical brightness slider.
func (cp *ColorPicker) drawBrightnessSlider(gtx layout.Context, fillRect func(gtx layout.Context, r image.Rectangle, c color.NRGBA), x, y, w, h int) {
	for i := 0; i < h; i++ {
		v := 1.0 - float64(i)/float64(h)
		r, g, b := hsvToRGB(cp.hue, cp.saturation, v)
		c := color.NRGBA{
			R: uint8(r * 255),
			G: uint8(g * 255),
			B: uint8(b * 255),
			A: 255,
		}
		fillRect(gtx, image.Rect(x, y+i, x+w, y+i+1), c)
	}
	bc := color.NRGBA{R: 80, G: 80, B: 100, A: 255}
	fillRect(gtx, image.Rect(x, y, x+w, y+1), bc)
	fillRect(gtx, image.Rect(x, y+h-1, x+w, y+h), bc)

	// Handle
	handleY := y + int(float64(h)*(1.0-cp.value))
	fillRect(gtx, image.Rect(x-2, handleY-2, x+w+2, handleY+2), color.NRGBA{R: 255, G: 255, B: 255, A: 255})

	// Register slider area
	{
		sliderBounds := image.Rect(x-4, y, x+w+4, y+h)
		stack := clip.Rect(sliderBounds).Push(gtx.Ops)
		event.Op(gtx.Ops, cp.brightTag)
		stack.Pop()
	}
}

// HandleEvents processes pointer events for the color picker.
func (cp *ColorPicker) HandleEvents(gtx layout.Context) {
	if !cp.visible {
		return
	}

	// Wheel events
	wheelFilter := pointer.Filter{
		Target: cp.wheelTag,
		Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
	}
	if ev, ok := gtx.Event(wheelFilter); ok {
		if e, ok := ev.(pointer.Event); ok {
			switch e.Kind {
			case pointer.Press, pointer.Drag:
				cp.wheelDragging = true
				cp.updateWheelFromPos(e.Position.X, e.Position.Y)
			case pointer.Release, pointer.Cancel:
				cp.wheelDragging = false
			}
		}
	}

	// Brightness events
	brightFilter := pointer.Filter{
		Target: cp.brightTag,
		Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
	}
	if ev, ok := gtx.Event(brightFilter); ok {
		if e, ok := ev.(pointer.Event); ok {
			switch e.Kind {
			case pointer.Press, pointer.Drag:
				cp.brightDragging = true
				v := 1.0 - float64((e.Position.Y-float32(cp.brightY))/float32(cp.brightH))
				cp.value = clamp01(v)
				cp.updateHex()
				cp.emitChange()
			case pointer.Release, pointer.Cancel:
				cp.brightDragging = false
			}
		}
	}

	// HSV field sliders (H, S, V)
	cp.handleFieldSlider(gtx, cp.hTag, &cp.hDragging, &cp.hue, 360)
	cp.handleFieldSlider(gtx, cp.sTag, &cp.sDragging, &cp.saturation, 1)
	cp.handleFieldSlider(gtx, cp.vTag, &cp.vDragging, &cp.value, 1)
}

// handleFieldSlider processes pointer events for an HSV/RGB slider bar.
func (cp *ColorPicker) handleFieldSlider(gtx layout.Context, tag *int, dragging *bool, target *float64, maxVal float64) {
	filter := pointer.Filter{
		Target: tag,
		Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
	}
	if ev, ok := gtx.Event(filter); ok {
		if e, ok := ev.(pointer.Event); ok {
			switch e.Kind {
			case pointer.Press, pointer.Drag:
				*dragging = true
				v := float64(e.Position.X-float32(cp.barX)) / float64(cp.barW) * maxVal
				*target = clamp01(v/maxVal) * maxVal
				cp.updateHex()
				cp.emitChange()
			case pointer.Release, pointer.Cancel:
				*dragging = false
			}
		}
	}
}

func (cp *ColorPicker) updateWheelFromPos(px, py float32) {
	dx := px - float32(cp.wheelCX)
	dy := py - float32(cp.wheelCY)
	angle := math.Atan2(float64(dy), float64(dx)) * 180 / math.Pi
	if angle < 0 {
		angle += 360
	}
	dist := math.Sqrt(float64(dx*dx + dy*dy))
	sat := dist / float64(WheelR)
	cp.hue = angle
	cp.saturation = clamp01(sat)
	cp.updateHex()
	cp.emitChange()
}

func (cp *ColorPicker) updateHex() {
	r, g, b := hsvToRGB(cp.hue, cp.saturation, cp.value)
	cp.hexBuf = fmt.Sprintf("#%02x%02x%02x", uint8(r*255), uint8(g*255), uint8(b*255))
}

func (cp *ColorPicker) emitChange() {
	if cp.OnColorChange != nil {
		cp.OnColorChange(cp.Color())
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ── HSV ↔ RGB ──────────────────────────────────────────────────────────────

func hsvToRGB(h, s, v float64) (float64, float64, float64) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return r + m, g + m, b + m
}

func rgbToHSV(r, g, b float64) (h, s, v float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	delta := max - min

	v = max
	if max == 0 {
		s = 0
		return
	}
	s = delta / max

	if delta == 0 {
		h = 0
		return
	}

	switch max {
	case r:
		h = 60 * math.Mod((g-b)/delta, 6)
	case g:
		h = 60 * ((b-r)/delta + 2)
	case b:
		h = 60 * ((r-g)/delta + 4)
	}
	if h < 0 {
		h += 360
	}
	return
}
