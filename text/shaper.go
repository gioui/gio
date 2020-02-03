// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"io"
	"strings"

	"golang.org/x/image/font"

	"gioui.org/op"
	"golang.org/x/image/math/fixed"
)

// Shaper implements layout and shaping of text.
type Shaper interface {
	// Layout a text according to a set of options.
	Layout(font Font, size fixed.Int26_6, maxWidth int, txt io.Reader) ([]Line, error)
	// Shape a line of text and return a clipping operation for its outline.
	Shape(font Font, size fixed.Int26_6, layout []Glyph) op.CallOp

	// LayoutString is like Layout, but for strings..
	LayoutString(font Font, size fixed.Int26_6, maxWidth int, str string) []Line
	// ShapeString is like Shape for lines previously laid out by LayoutString.
	ShapeString(font Font, size fixed.Int26_6, str string, layout []Glyph) op.CallOp

	// Metrics returns the font metrics for font.
	Metrics(font Font, size fixed.Int26_6) font.Metrics
}

// FontRegistry implements layout and shaping of text from a set of
// registered fonts.
//
// If a font matches no registered shape, FontRegistry falls back to the
// first registered face.
//
// The LayoutString and ShapeString results are cached and re-used if
// possible.
type FontRegistry struct {
	def   Typeface
	faces map[Font]*face
}

type face struct {
	face        Face
	layoutCache layoutCache
	pathCache   pathCache
}

func (s *FontRegistry) Register(font Font, tf Face) {
	if s.faces == nil {
		s.def = font.Typeface
		s.faces = make(map[Font]*face)
	}
	if font.Weight == 0 {
		font.Weight = Normal
	}
	s.faces[font] = &face{
		face: tf,
	}
}

func (s *FontRegistry) Layout(font Font, size fixed.Int26_6, maxWidth int, txt io.Reader) ([]Line, error) {
	tf := s.faceForFont(font)
	return tf.face.Layout(size, maxWidth, txt)
}

func (s *FontRegistry) Shape(font Font, size fixed.Int26_6, layout []Glyph) op.CallOp {
	tf := s.faceForFont(font)
	return tf.face.Shape(size, layout)
}

func (s *FontRegistry) LayoutString(font Font, size fixed.Int26_6, maxWidth int, str string) []Line {
	tf := s.faceForFont(font)
	return tf.layout(size, maxWidth, str)
}

func (s *FontRegistry) ShapeString(font Font, size fixed.Int26_6, str string, layout []Glyph) op.CallOp {
	tf := s.faceForFont(font)
	return tf.shape(size, str, layout)
}

func (s *FontRegistry) Metrics(font Font, size fixed.Int26_6) font.Metrics {
	tf := s.faceForFont(font)
	return tf.metrics(size)
}

func (s *FontRegistry) faceForStyle(font Font) *face {
	tf := s.faces[font]
	if tf == nil {
		font := font
		font.Weight = Normal
		tf = s.faces[font]
	}
	if tf == nil {
		font := font
		font.Style = Regular
		tf = s.faces[font]
	}
	if tf == nil {
		font := font
		font.Style = Regular
		font.Weight = Normal
		tf = s.faces[font]
	}
	return tf
}

func (s *FontRegistry) faceForFont(font Font) *face {
	tf := s.faceForStyle(font)
	if tf == nil {
		font.Typeface = s.def
		tf = s.faceForStyle(font)
	}
	return tf
}

func (t *face) layout(ppem fixed.Int26_6, maxWidth int, str string) []Line {
	if t == nil {
		return nil
	}
	lk := layoutKey{
		ppem:     ppem,
		maxWidth: maxWidth,
		str:      str,
	}
	if l, ok := t.layoutCache.Get(lk); ok {
		return l
	}
	l, _ := t.face.Layout(ppem, maxWidth, strings.NewReader(str))
	t.layoutCache.Put(lk, l)
	return l
}

func (t *face) shape(ppem fixed.Int26_6, str string, layout []Glyph) op.CallOp {
	if t == nil {
		return op.CallOp{}
	}
	pk := pathKey{
		ppem: ppem,
		str:  str,
	}
	if clip, ok := t.pathCache.Get(pk); ok {
		return clip
	}
	clip := t.face.Shape(ppem, layout)
	t.pathCache.Put(pk, clip)
	return clip
}

func (t *face) metrics(ppem fixed.Int26_6) font.Metrics {
	return t.face.Metrics(ppem)
}
