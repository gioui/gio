package widget

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/gpu/headless"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
)

func TestSelectableZeroValue(t *testing.T) {
	var s Selectable
	if s.Text() != "" {
		t.Errorf("expected zero value to have no text, got %q", s.Text())
	}
	if start, end := s.Selection(); start != 0 || end != 0 {
		t.Errorf("expected start=0, end=0, got start=%d, end=%d", start, end)
	}
	if selected := s.SelectedText(); selected != "" {
		t.Errorf("expected selected text to be \"\", got %q", selected)
	}
	s.SetCaret(5, 5)
	if start, end := s.Selection(); start != 0 || end != 0 {
		t.Errorf("expected start=0, end=0, got start=%d, end=%d", start, end)
	}
}

// Verify that an existing selection is dismissed when you press arrow keys.
func TestSelectableMove(t *testing.T) {
	r := new(input.Router)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Locale: english,
		Source: r.Source(),
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fnt := font.Font{}
	fontSize := unit.Sp(10)

	str := `0123456789`

	// Layout once to populate e.lines and get focus.
	s := new(Selectable)

	gtx.Execute(key.FocusCmd{Tag: s})
	s.SetText(str)
	// Set up selection so the Selectable filters for all 4 directional keys.
	s.Layout(gtx, cache, font.Font{}, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)
	s.SetCaret(3, 6)
	s.Layout(gtx, cache, font.Font{}, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)
	s.Layout(gtx, cache, font.Font{}, fontSize, op.CallOp{}, op.CallOp{})
	r.Frame(gtx.Ops)

	for _, keyName := range []key.Name{key.NameLeftArrow, key.NameRightArrow, key.NameUpArrow, key.NameDownArrow} {
		// Select 345
		s.SetCaret(3, 6)
		if start, end := s.Selection(); start != 3 || end != 6 {
			t.Errorf("expected start=%d, end=%d, got start=%d, end=%d", 3, 6, start, end)
		}
		if expected, got := "345", s.SelectedText(); expected != got {
			t.Errorf("KeyName %s, expected %q, got %q", keyName, expected, got)
		}

		// Press the key
		r.Queue(key.Event{State: key.Press, Name: keyName})
		s.SetText(str)
		s.Layout(gtx, cache, fnt, fontSize, op.CallOp{}, op.CallOp{})
		r.Frame(gtx.Ops)

		if expected, got := "", s.SelectedText(); expected != got {
			t.Errorf("KeyName %s, expected %q, got %q", keyName, expected, got)
		}
	}
}

func colorMaterial(ops *op.Ops, c color.NRGBA) op.CallOp {
	m := op.Record(ops)
	paint.ColorOp{Color: c}.Add(ops)
	return m.Stop()
}

// countSelectedColorPixels counts red text pixels.
func countSelectedColorPixels(img *image.RGBA) int {
	count := 0
	for i := 0; i < len(img.Pix); i += 4 {
		r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
		if a > 0x80 && r > 0x60 && g < 0x40 && b < 0x40 {
			count++
		}
	}
	return count
}

func TestSelectableSelectedTextColor(t *testing.T) {
	const (
		width  = 120
		height = 30
	)
	textColor := color.NRGBA{A: 0xff}
	selectionColor := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	selectedColor := color.NRGBA{R: 0xff, A: 0xff}

	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	render := func(selected color.NRGBA) *image.RGBA {
		s := new(Selectable)
		s.PaintSelectionWhenUnfocused = true
		s.SelectedTextColor = selected
		s.SetText("Hello")
		// Shape the text before setting the selection.
		gtx := layout.Context{
			Ops:         new(op.Ops),
			Constraints: layout.Exact(image.Pt(width, height)),
			Locale:      english,
		}
		s.Layout(gtx, shaper, font.Font{}, unit.Sp(16),
			colorMaterial(gtx.Ops, textColor), colorMaterial(gtx.Ops, selectionColor))
		s.SetCaret(0, len("Hello"))
		ops := new(op.Ops)
		gtx = layout.Context{
			Ops:         ops,
			Constraints: layout.Exact(image.Pt(width, height)),
			Locale:      english,
		}
		s.Layout(gtx, shaper, font.Font{}, unit.Sp(16),
			colorMaterial(ops, textColor), colorMaterial(ops, selectionColor))
		window, err := headless.NewWindow(width, height)
		if err != nil {
			t.Skipf("headless renderer unavailable: %v", err)
		}
		defer window.Release()
		if err := window.Frame(ops); err != nil {
			t.Fatal(err)
		}
		pixels := image.NewRGBA(image.Rectangle{Max: image.Pt(width, height)})
		if err := window.Screenshot(pixels); err != nil {
			t.Fatal(err)
		}
		return pixels
	}

	if got := countSelectedColorPixels(render(color.NRGBA{})); got != 0 {
		t.Fatalf("unset SelectedTextColor painted %d selected-colour pixels", got)
	}
	if got := countSelectedColorPixels(render(selectedColor)); got == 0 {
		t.Fatal("SelectedTextColor did not paint the selected glyphs")
	}
}

func TestSelectableConfigurations(t *testing.T) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(300, 300)),
		Locale:      english,
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(gofont.Collection()))
	fontSize := unit.Sp(10)
	font := font.Font{}
	sentence := "\n\n\n\n\n\n\n\n\n\n\n\nthe quick brown fox jumps over the lazy dog"

	for _, alignment := range []text.Alignment{text.Start, text.Middle, text.End} {
		for _, zeroMin := range []bool{true, false} {
			t.Run(fmt.Sprintf("Alignment: %v ZeroMinConstraint: %v", alignment, zeroMin), func(t *testing.T) {
				defer func() {
					if err := recover(); err != nil {
						t.Error(err)
					}
				}()
				if zeroMin {
					gtx.Constraints.Min = image.Point{}
				} else {
					gtx.Constraints.Min = gtx.Constraints.Max
				}
				s := new(Selectable)
				s.Alignment = alignment
				s.SetText(sentence)
				interactiveDims := s.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
				staticDims := Label{Alignment: alignment}.Layout(gtx, cache, font, fontSize, sentence, op.CallOp{})

				if interactiveDims != staticDims {
					t.Errorf("expected consistent dimensions, static returned %#+v, interactive returned %#+v", staticDims, interactiveDims)
				}
			})
		}
	}
}
