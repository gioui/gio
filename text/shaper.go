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

	// LayoutString is like Layout, but for strings.
	LayoutString(font Font, size fixed.Int26_6, maxWidth int, str string) []Line
	// ShapeString is like Shape for lines previously laid out by LayoutString.
	ShapeString(font Font, size fixed.Int26_6, str string, layout []Glyph) op.CallOp

	// Metrics returns the font metrics for font.
	Metrics(font Font, size fixed.Int26_6) font.Metrics
}

// Collection maps Fonts to Faces.
type Collection struct {
	def   Typeface
	faces map[Font]Face
}

// Cache implements cached layout and shaping of text from a set of
// registered fonts.
//
// If a font matches no registered shape, Cache falls back to the
// first registered face.
//
// The LayoutString and ShapeString results are cached and re-used if
// possible.
type Cache struct {
	col   *Collection
	faces map[Font]*faceCache
}

type faceCache struct {
	layoutCache layoutCache
	pathCache   pathCache
}

func (c *Collection) Register(font Font, tf Face) {
	if c.faces == nil {
		c.def = font.Typeface
		c.faces = make(map[Font]Face)
	}
	if font.Weight == 0 {
		font.Weight = Normal
	}
	c.faces[font] = tf
}

// Lookup a font and return the effective font and its
// font face.
func (c *Collection) Lookup(font Font) (Font, Face) {
	var f Face
	font, f = c.faceForStyle(font)
	if f == nil {
		font.Typeface = c.def
		font, f = c.faceForStyle(font)
	}
	return font, f
}

func (c *Collection) faceForStyle(font Font) (Font, Face) {
	tf := c.faces[font]
	if tf == nil {
		font := font
		font.Weight = Normal
		tf = c.faces[font]
	}
	if tf == nil {
		font := font
		font.Style = Regular
		tf = c.faces[font]
	}
	if tf == nil {
		font := font
		font.Style = Regular
		font.Weight = Normal
		tf = c.faces[font]
	}
	return font, tf
}

func NewCache(fonts *Collection) *Cache {
	return &Cache{
		col:   fonts,
		faces: make(map[Font]*faceCache),
	}
}

func (s *Cache) Layout(font Font, size fixed.Int26_6, maxWidth int, txt io.Reader) ([]Line, error) {
	_, face := s.faceForFont(font)
	return face.Layout(size, maxWidth, txt)
}

func (s *Cache) Shape(font Font, size fixed.Int26_6, layout []Glyph) op.CallOp {
	_, face := s.faceForFont(font)
	return face.Shape(size, layout)
}

func (s *Cache) LayoutString(font Font, size fixed.Int26_6, maxWidth int, str string) []Line {
	cache, face := s.faceForFont(font)
	return cache.layout(face, size, maxWidth, str)
}

func (s *Cache) ShapeString(font Font, size fixed.Int26_6, str string, layout []Glyph) op.CallOp {
	cache, face := s.faceForFont(font)
	return cache.shape(face, size, str, layout)
}

func (s *Cache) Metrics(font Font, size fixed.Int26_6) font.Metrics {
	cache, face := s.faceForFont(font)
	return cache.metrics(face, size)
}

func (s *Cache) faceForFont(font Font) (*faceCache, Face) {
	var f Face
	font, f = s.col.Lookup(font)
	cache, exists := s.faces[font]
	if !exists {
		cache = new(faceCache)
		s.faces[font] = cache
	}
	return cache, f
}

func (t *faceCache) layout(face Face, ppem fixed.Int26_6, maxWidth int, str string) []Line {
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
	l, _ := face.Layout(ppem, maxWidth, strings.NewReader(str))
	t.layoutCache.Put(lk, l)
	return l
}

func (t *faceCache) shape(face Face, ppem fixed.Int26_6, str string, layout []Glyph) op.CallOp {
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
	clip := face.Shape(ppem, layout)
	t.pathCache.Put(pk, clip)
	return clip
}

func (t *faceCache) metrics(face Face, ppem fixed.Int26_6) font.Metrics {
	return face.Metrics(ppem)
}
