// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"sync"

	giofont "gioui.org/font"
	"gioui.org/font/opentype"
	fontapi "github.com/go-text/typesetting/font"
)

// FontRegistry manages loaded fonts and provides glyph measurement.
type FontRegistry struct {
	mu    sync.RWMutex
	faces map[string]*fontEntry
}

type fontEntry struct {
	gioface  giofont.Face    //gio interface
	font     giofont.Font    // metadata
	apiFace  *fontapi.Face   // underlying face for measurement
	gidCache map[rune]uint32 // rune → glyph ID cache
}

// NewFontRegistry creates an empty FontRegistry.
func NewFontRegistry() *FontRegistry {
	return &FontRegistry{
		faces: make(map[string]*fontEntry),
	}
}

// LoadFont loads a font from raw bytes (TTF/OTF/WOFF2).
func (fr *FontRegistry) LoadFont(name string, data []byte) error {
	f, err := opentype.Parse(data)
	if err != nil {
		return err
	}

	fr.mu.Lock()
	defer fr.mu.Unlock()

	fr.faces[name] = &fontEntry{
		gioface: f,
		font:    f.Font(),
		apiFace: f.Face(), // get the underlying typesetting face
		gidCache: make(map[rune]uint32),
	}
	return nil
}

// LoadFontCollection loads a font collection (TTC) from raw bytes.
func (fr *FontRegistry) LoadFontCollection(name string, data []byte) (int, error) {
	faces, err := opentype.ParseCollection(data)
	if err != nil {
		return 0, err
	}

	fr.mu.Lock()
	defer fr.mu.Unlock()

	for i, ff := range faces {
		key := name
		if i > 0 {
			key = name + ":" + string(rune('0'+i))
		}
		fr.faces[key] = &fontEntry{
			gioface: ff.Face,
			font:    ff.Font,
			apiFace: ff.Face.Face(),
			gidCache: make(map[rune]uint32),
		}
	}
	return len(faces), nil
}

// GetFace returns the gio font.Face for the given name.
func (fr *FontRegistry) GetFace(name string) giofont.Face {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	entry, ok := fr.faces[name]
	if !ok {
		return nil
	}
	return entry.gioface
}

// GetAPIFace returns the underlying typesetting face for measurement.
// NOT safe for concurrent use — each goroutine should create its own.
func (fr *FontRegistry) GetAPIFace(name string) *fontapi.Face {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	entry, ok := fr.faces[name]
	if !ok {
		return nil
	}
	// Create a new Face for thread safety
	return fontapi.NewFace(entry.apiFace.Font)
}

// GetFont returns the giofont.Font metadata.
func (fr *FontRegistry) GetFont(name string) giofont.Font {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	if entry, ok := fr.faces[name]; ok {
		return entry.font
	}
	return giofont.Font{}
}

// HasFont returns true if a font is loaded.
func (fr *FontRegistry) HasFont(name string) bool {
	fr.mu.RLock()
	defer fr.mu.RUnlock()
	_, ok := fr.faces[name]
	return ok
}

// FontNames returns names of all loaded fonts.
func (fr *FontRegistry) FontNames() []string {
	fr.mu.RLock()
	defer fr.mu.RUnlock()

	names := make([]string, 0, len(fr.faces))
	for name := range fr.faces {
		names = append(names, name)
	}
	return names
}

// --- Glyph Metrics Cache ---

type gmKey struct {
	Font string
	Char rune
	Size float32
}

type gmValue struct {
	Width  int
	Height int
	Valid  bool
}

// GlyphMetricsCache provides efficient glyph width/height measurement with caching.
type GlyphMetricsCache struct {
	mu       sync.RWMutex
	registry *FontRegistry
	cache    map[gmKey]gmValue
}

// NewGlyphMetricsCache creates a cache backed by the given FontRegistry.
func NewGlyphMetricsCache(registry *FontRegistry) *GlyphMetricsCache {
	return &GlyphMetricsCache{
		registry: registry,
		cache:    make(map[gmKey]gmValue),
	}
}

// MeasureGlyph returns the width and height of a character in pixels.
func (gmc *GlyphMetricsCache) MeasureGlyph(ch rune, fontName string, size float32) (width, height int) {
	if size <= 0 {
		size = 14
	}
	if fontName == "" {
		fontName = "gofont"
	}

	key := gmKey{Font: fontName, Char: ch, Size: size}

	gmc.mu.RLock()
	if v, ok := gmc.cache[key]; ok && v.Valid {
		gmc.mu.RUnlock()
		return v.Width, v.Height
	}
	gmc.mu.RUnlock()

	w, h := gmc.computeGlyph(ch, fontName, size)

	gmc.mu.Lock()
	gmc.cache[key] = gmValue{Width: w, Height: h, Valid: true}
	gmc.mu.Unlock()

	return w, h
}

// MeasureGlyphWithStyle measures a glyph using CharStyle.
func (gmc *GlyphMetricsCache) MeasureGlyphWithStyle(ch rune, style CharStyle, defaultFont string, defaultSize float32) (width, height int) {
	fontName := style.F
	if fontName == "" {
		fontName = defaultFont
	}
	size := style.S
	if size <= 0 {
		size = defaultSize
	}
	return gmc.MeasureGlyph(ch, fontName, size)
}

// computeGlyph measures a glyph using actual font data.
func (gmc *GlyphMetricsCache) computeGlyph(ch rune, fontName string, size float32) (width, height int) {
	face := gmc.registry.GetAPIFace(fontName)
	if face == nil {
		h := int(size)
		if h <= 0 {
			h = 14
		}
		return h * 6 / 10, h
	}

	// Get glyph ID
	gid, ok := face.NominalGlyph(ch)
	if !ok {
		h := int(size)
		return h * 6 / 10, h
	}

	// Get horizontal advance (width in font units)
	advance := face.HorizontalAdvance(gid)

	// Get font extents for height
	extents, hasExtents := face.FontHExtents()

	// Scale from font units to pixels
	ppemX, ppemY := face.Ppem()
	if ppemX == 0 {
		ppemX = 1000
	}
	if ppemY == 0 {
		ppemY = 1000
	}

	scaleX := size / float32(ppemX)
	scaleY := size / float32(ppemY)

	w := int(advance * scaleX)
	if w <= 0 {
		glyphExt, hasGlyphExt := face.GlyphExtents(gid)
		if hasGlyphExt {
			w = int(float32(glyphExt.Width) * scaleX)
		}
		if w <= 0 {
			w = int(size * 6 / 10)
		}
	}

	h := int(size)
	if hasExtents {
		h = int((extents.Ascender - extents.Descender) * scaleY)
		if h <= 0 {
			h = int(size)
		}
	}

	return w, h
}

// ClearCache removes all cached metrics.
func (gmc *GlyphMetricsCache) ClearCache() {
	gmc.mu.Lock()
	defer gmc.mu.Unlock()
	gmc.cache = make(map[gmKey]gmValue)
}

// CacheSize returns the number of cached entries.
func (gmc *GlyphMetricsCache) CacheSize() int {
	gmc.mu.RLock()
	defer gmc.mu.RUnlock()
	return len(gmc.cache)
}

// --- FontExtents helper ---

// FontExtents holds font-wide vertical metrics (normalized to 1em).
type FontExtents struct {
	Ascender  float32
	Descender float32
	LineGap   float32
}

// GetFontExtents returns normalized vertical metrics for a font.
func (gmc *GlyphMetricsCache) GetFontExtents(fontName string) FontExtents {
	if fontName == "" {
		fontName = "gofont"
	}

	face := gmc.registry.GetAPIFace(fontName)
	if face == nil {
		return FontExtents{Ascender: 0.8, Descender: -0.2, LineGap: 0.1}
	}

	ext, ok := face.FontHExtents()
	if !ok {
		return FontExtents{Ascender: 0.8, Descender: -0.2, LineGap: 0.1}
	}

	_, ppemY := face.Ppem()
	if ppemY == 0 {
		ppemY = 1000
	}
	scale := 1.0 / float32(ppemY)

	return FontExtents{
		Ascender:  ext.Ascender * scale,
		Descender: ext.Descender * scale,
		LineGap:   ext.LineGap * scale,
	}
}

// --- GlyphMeasurer implementation ---

// FontMeasurer implements GlyphMeasurer using FontRegistry + GlyphMetricsCache.
type FontMeasurer struct {
	Registry    *FontRegistry
	Cache       *GlyphMetricsCache
	DefaultFont string
	DefaultSize float32
}

// MeasureGlyph implements the GlyphMeasurer interface.
func (fm *FontMeasurer) MeasureGlyph(ch rune, style CharStyle) (width, height int) {
	return fm.Cache.MeasureGlyphWithStyle(ch, style, fm.DefaultFont, fm.DefaultSize)
}

// BoundingBox returns the bounding box of a glyph.
func (fm *FontMeasurer) BoundingBox(ch rune, style CharStyle) image.Rectangle {
	w, h := fm.MeasureGlyph(ch, style)
	return image.Rect(0, 0, w, h)
}
