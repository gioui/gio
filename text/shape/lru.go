// SPDX-License-Identifier: Unlicense OR MIT

package shape

import (
	"gioui.org/op"
	"gioui.org/text"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type layoutCache struct {
	m          map[layoutKey]*layout
	head, tail *layout
}

type pathCache struct {
	m          map[pathKey]*path
	head, tail *path
}

type layout struct {
	next, prev *layout
	key        layoutKey
	layout     *text.Layout
}

type path struct {
	next, prev *path
	key        pathKey
	val        op.MacroOp
}

type layoutKey struct {
	f    *sfnt.Font
	ppem fixed.Int26_6
	str  string
	opts text.LayoutOptions
}

type pathKey struct {
	f    *sfnt.Font
	ppem fixed.Int26_6
	str  string
}

const maxSize = 1000

func (l *layoutCache) Get(k layoutKey) (*text.Layout, bool) {
	if lt, ok := l.m[k]; ok {
		l.remove(lt)
		l.insert(lt)
		return lt.layout, true
	}
	return nil, false
}

func (l *layoutCache) Put(k layoutKey, lt *text.Layout) {
	if l.m == nil {
		l.m = make(map[layoutKey]*layout)
		l.head = new(layout)
		l.tail = new(layout)
		l.head.prev = l.tail
		l.tail.next = l.head
	}
	val := &layout{key: k, layout: lt}
	l.m[k] = val
	l.insert(val)
	if len(l.m) > maxSize {
		oldest := l.tail.next
		l.remove(oldest)
		delete(l.m, oldest.key)
	}
}

func (l *layoutCache) remove(lt *layout) {
	lt.next.prev = lt.prev
	lt.prev.next = lt.next
}

func (l *layoutCache) insert(lt *layout) {
	lt.next = l.head
	lt.prev = l.head.prev
	lt.prev.next = lt
	lt.next.prev = lt
}

func (c *pathCache) Get(k pathKey) (op.MacroOp, bool) {
	if v, ok := c.m[k]; ok {
		c.remove(v)
		c.insert(v)
		return v.val, true
	}
	return op.MacroOp{}, false
}

func (c *pathCache) Put(k pathKey, v op.MacroOp) {
	if c.m == nil {
		c.m = make(map[pathKey]*path)
		c.head = new(path)
		c.tail = new(path)
		c.head.prev = c.tail
		c.tail.next = c.head
	}
	val := &path{key: k, val: v}
	c.m[k] = val
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
