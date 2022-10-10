// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"encoding/binary"
	"hash/maphash"

	"gioui.org/io/system"
	"gioui.org/op/clip"
	"golang.org/x/image/math/fixed"
)

type layoutCache struct {
	m          map[layoutKey]*layoutElem
	head, tail *layoutElem
}

type pathCache struct {
	seed       maphash.Seed
	m          map[uint64]*path
	head, tail *path
}

type layoutElem struct {
	next, prev *layoutElem
	key        layoutKey
	layout     document
}

type path struct {
	next, prev *path
	key        uint64
	val        clip.PathSpec
	glyphs     []glyphInfo
}

type glyphInfo struct {
	ID GlyphID
	X  fixed.Int26_6
}

type layoutKey struct {
	ppem               fixed.Int26_6
	maxWidth, minWidth int
	maxLines           int
	str                string
	locale             system.Locale
	font               Font
}

type pathKey struct {
	gidHash uint64
}

const maxSize = 1000

func (l *layoutCache) Get(k layoutKey) (document, bool) {
	if lt, ok := l.m[k]; ok {
		l.remove(lt)
		l.insert(lt)
		return lt.layout, true
	}
	return document{}, false
}

func (l *layoutCache) Put(k layoutKey, lt document) {
	if l.m == nil {
		l.m = make(map[layoutKey]*layoutElem)
		l.head = new(layoutElem)
		l.tail = new(layoutElem)
		l.head.prev = l.tail
		l.tail.next = l.head
	}
	val := &layoutElem{key: k, layout: lt}
	l.m[k] = val
	l.insert(val)
	if len(l.m) > maxSize {
		oldest := l.tail.next
		l.remove(oldest)
		delete(l.m, oldest.key)
	}
}

func (l *layoutCache) remove(lt *layoutElem) {
	lt.next.prev = lt.prev
	lt.prev.next = lt.next
}

func (l *layoutCache) insert(lt *layoutElem) {
	lt.next = l.head
	lt.prev = l.head.prev
	lt.prev.next = lt
	lt.next.prev = lt
}

// hashGlyphs computes a hash key based on the ID and X offset of
// every glyph in the slice.
func (c *pathCache) hashGlyphs(gs []Glyph) uint64 {
	if c.seed == (maphash.Seed{}) {
		c.seed = maphash.MakeSeed()
	}
	var h maphash.Hash
	h.SetSeed(c.seed)
	var b [8]byte
	firstX := fixed.Int26_6(0)
	for i, g := range gs {
		if i == 0 {
			firstX = g.X
		}
		// Cache glyph X offsets relative to the first glyph.
		binary.LittleEndian.PutUint32(b[:4], uint32(g.X-firstX))
		h.Write(b[:4])
		binary.LittleEndian.PutUint64(b[:], uint64(g.ID))
		h.Write(b[:])
	}
	sum := h.Sum64()
	return sum
}

func gidsEqual(a []glyphInfo, glyphs []Glyph) bool {
	if len(a) != len(glyphs) {
		return false
	}
	firstX := fixed.Int26_6(0)
	for i := range a {
		if i == 0 {
			firstX = glyphs[i].X
		}
		// Cache glyph X offsets relative to the first glyph.
		if a[i].ID != glyphs[i].ID || a[i].X != (glyphs[i].X-firstX) {
			return false
		}
	}
	return true
}

func (c *pathCache) Get(key uint64, gs []Glyph) (clip.PathSpec, bool) {
	if v, ok := c.m[key]; ok && gidsEqual(v.glyphs, gs) {
		c.remove(v)
		c.insert(v)
		return v.val, true
	}
	return clip.PathSpec{}, false
}

func (c *pathCache) Put(key uint64, glyphs []Glyph, v clip.PathSpec) {
	if c.m == nil {
		c.m = make(map[uint64]*path)
		c.head = new(path)
		c.tail = new(path)
		c.head.prev = c.tail
		c.tail.next = c.head
	}
	gids := make([]glyphInfo, len(glyphs))
	firstX := fixed.I(0)
	for i, glyph := range glyphs {
		if i == 0 {
			firstX = glyph.X
		}
		// Cache glyph X offsets relative to the first glyph.
		gids[i] = glyphInfo{ID: glyph.ID, X: glyph.X - firstX}
	}
	val := &path{key: key, val: v, glyphs: gids}
	c.m[key] = val
	c.insert(val)
	if len(c.m) > maxSize {
		oldest := c.tail.next
		c.remove(oldest)
		delete(c.m, oldest.key)
	}
}

func (c *pathCache) remove(v *path) {
	v.next.prev = v.prev
	v.prev.next = v.next
}

func (c *pathCache) insert(v *path) {
	v.next = c.head
	v.prev = c.head.prev
	v.prev.next = v
	v.next.prev = v
}
