// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image/color"
	"sync"

	"gioui.org/font"
	"gioui.org/internal/f32color"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

// labelWidthCache caches MeasureWidth results to avoid repeated text shaping.
// Key: text+size+font, Value: pixel width.
type labelWidthCacheKey struct {
	text    string
	size    unit.Sp
	font    font.Font
}

var (
	labelWidthCacheMu sync.Mutex
	labelWidthCache   = make(map[labelWidthCacheKey]int)
)

// ClearLabelWidthCache removes all cached measurements. Call when fonts change.
func ClearLabelWidthCache() {
	labelWidthCacheMu.Lock()
	labelWidthCache = make(map[labelWidthCacheKey]int)
	labelWidthCacheMu.Unlock()
}

// LabelStyle configures the presentation of text. If the State field is set, the
// label will be laid out as interactive (able to be selected and copied). Otherwise,
// the label will be non-interactive.
type LabelStyle struct {
	// Face defines the text style.
	Font font.Font
	// Color is the text color.
	Color color.NRGBA
	// SelectionColor is the color of the background for selected text.
	SelectionColor color.NRGBA
	// Alignment specify the text alignment.
	Alignment text.Alignment
	// MaxLines limits the number of lines. Zero means no limit.
	MaxLines int
	// WrapPolicy configures how displayed text will be broken into lines.
	WrapPolicy text.WrapPolicy
	// Truncator is the text that will be shown at the end of the final
	// line if MaxLines is exceeded. Defaults to "…" if empty.
	Truncator string
	// Text is the content displayed by the label.
	Text string
	// TextSize determines the size of the text glyphs.
	TextSize unit.Sp
	// LineHeight controls the distance between the baselines of lines of text.
	// If zero, a sensible default will be used.
	LineHeight unit.Sp
	// LineHeightScale applies a scaling factor to the LineHeight. If zero, a
	// sensible default will be used.
	LineHeightScale float32

	// Shaper is the text shaper used to display this labe. This field is automatically
	// set using by all constructor functions. If constructing a LabelStyle literal, you
	// must provide a Shaper or displaying text will panic.
	Shaper *text.Shaper
	// State provides text selection state for the label. If not set, the label cannot
	// be selected or copied interactively.
	State *widget.Selectable
}

func H1(th *Theme, txt string) LabelStyle {
	label := Label(th, th.TextSize*96.0/16.0, txt)
	label.Font.Weight = font.Light
	return label
}

func H2(th *Theme, txt string) LabelStyle {
	label := Label(th, th.TextSize*60.0/16.0, txt)
	label.Font.Weight = font.Light
	return label
}

func H3(th *Theme, txt string) LabelStyle {
	return Label(th, th.TextSize*48.0/16.0, txt)
}

func H4(th *Theme, txt string) LabelStyle {
	return Label(th, th.TextSize*34.0/16.0, txt)
}

func H5(th *Theme, txt string) LabelStyle {
	return Label(th, th.TextSize*24.0/16.0, txt)
}

func H6(th *Theme, txt string) LabelStyle {
	label := Label(th, th.TextSize*20.0/16.0, txt)
	label.Font.Weight = font.Medium
	return label
}

func Subtitle1(th *Theme, txt string) LabelStyle {
	return Label(th, th.TextSize*16.0/16.0, txt)
}

func Subtitle2(th *Theme, txt string) LabelStyle {
	label := Label(th, th.TextSize*14.0/16.0, txt)
	label.Font.Weight = font.Medium
	return label
}

func Body1(th *Theme, txt string) LabelStyle {
	return Label(th, th.TextSize, txt)
}

func Body2(th *Theme, txt string) LabelStyle {
	return Label(th, th.TextSize*14.0/16.0, txt)
}

func Caption(th *Theme, txt string) LabelStyle {
	return Label(th, th.TextSize*12.0/16.0, txt)
}

func Overline(th *Theme, txt string) LabelStyle {
	return Label(th, th.TextSize*10.0/16.0, txt)
}

func Label(th *Theme, size unit.Sp, txt string) LabelStyle {
	l := LabelStyle{
		Text:           txt,
		Color:          th.Palette.Fg,
		SelectionColor: f32color.MulAlpha(th.Palette.ContrastBg, 0x60),
		TextSize:       size,
		Shaper:         th.Shaper,
	}
	l.Font.Typeface = th.Face
	return l
}

func (l LabelStyle) Layout(gtx layout.Context) layout.Dimensions {
	textColorMacro := op.Record(gtx.Ops)
	paint.ColorOp{Color: l.Color}.Add(gtx.Ops)
	textColor := textColorMacro.Stop()
	selectColorMacro := op.Record(gtx.Ops)
	paint.ColorOp{Color: l.SelectionColor}.Add(gtx.Ops)
	selectColor := selectColorMacro.Stop()

	if l.State != nil {
		if l.State.Text() != l.Text {
			l.State.SetText(l.Text)
		}
		l.State.Alignment = l.Alignment
		l.State.MaxLines = l.MaxLines
		l.State.Truncator = l.Truncator
		l.State.WrapPolicy = l.WrapPolicy
		l.State.LineHeight = l.LineHeight
		l.State.LineHeightScale = l.LineHeightScale
		return l.State.Layout(gtx, l.Shaper, l.Font, l.TextSize, textColor, selectColor)
	}
	tl := widget.Label{
		Alignment:       l.Alignment,
		MaxLines:        l.MaxLines,
		Truncator:       l.Truncator,
		WrapPolicy:      l.WrapPolicy,
		LineHeight:      l.LineHeight,
		LineHeightScale: l.LineHeightScale,
	}
	return tl.Layout(gtx, l.Shaper, l.Font, l.TextSize, l.Text, textColor)
}

// MeasureWidth returns the pixel width of txt with this label's font/size settings.
// Results are cached globally — repeated calls with the same text+font+size are free.
// Uses a temporary op.Ops so no rendering ops are produced.
func (l LabelStyle) MeasureWidth(gtx layout.Context, txt string) int {
	k := labelWidthCacheKey{text: txt, size: l.TextSize, font: l.Font}
	labelWidthCacheMu.Lock()
	if w, ok := labelWidthCache[k]; ok {
		labelWidthCacheMu.Unlock()
		return w
	}
	labelWidthCacheMu.Unlock()

	// Measure using a temporary op.Ops (no visible output).
	var tempOps op.Ops
	sub := gtx
	sub.Ops = &tempOps
	sub.Constraints.Min.X = 0
	sub.Constraints.Max.X = 1e9
	sub.Constraints.Min.Y = 0
	sub.Constraints.Max.Y = 1e9

	origText := l.Text
	l.Text = txt
	l.Alignment = text.Start
	l.MaxLines = 0
	l.Truncator = ""
	l.WrapPolicy = text.WrapWords
	l.State = nil // no selection
	textColorMacro := op.Record(sub.Ops)
	paint.ColorOp{Color: l.Color}.Add(sub.Ops)
	textColor := textColorMacro.Stop()
	tl := widget.Label{
		Alignment: l.Alignment,
	}
	dims := tl.Layout(sub, l.Shaper, l.Font, l.TextSize, l.Text, textColor)
	l.Text = origText

	w := dims.Size.X
	labelWidthCacheMu.Lock()
	labelWidthCache[k] = w
	labelWidthCacheMu.Unlock()
	return w
}
