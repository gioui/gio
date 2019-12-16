// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

const bufferDebug = false

// editBuffer implements a gap buffer for text editing.
type editBuffer struct {
	// caret is the caret position in bytes.
	caret int
	// pos is the byte position for Read and ReadRune.
	pos int

	// The gap start and end in bytes.
	gapstart, gapend int
	text             []byte

	// changed tracks whether the buffer content
	// has changed since the last call to Changed.
	changed bool
}

const minSpace = 5

func (e *editBuffer) Changed() bool {
	c := e.changed
	e.changed = false
	return c
}

func (e *editBuffer) deleteRunes(runes int) {
	e.moveGap(0)
	for ; runes < 0 && e.gapstart > 0; runes++ {
		_, s := utf8.DecodeLastRune(e.text[:e.gapstart])
		e.gapstart -= s
		e.caret -= s
		e.changed = e.changed || s > 0
	}
	for ; runes > 0 && e.gapend < len(e.text); runes-- {
		_, s := utf8.DecodeRune(e.text[e.gapend:])
		e.gapend += s
		e.changed = e.changed || s > 0
	}
	e.dump()
}

// moveGap moves the gap to the caret position. After returning,
// the gap is guaranteed to be at least space bytes long.
func (e *editBuffer) moveGap(space int) {
	if e.gapLen() < space {
		if space < minSpace {
			space = minSpace
		}
		txt := make([]byte, e.len()+space)
		// Expand to capacity.
		txt = txt[:cap(txt)]
		gaplen := len(txt) - e.len()
		if e.caret > e.gapstart {
			copy(txt, e.text[:e.gapstart])
			copy(txt[e.caret+gaplen:], e.text[e.caret:])
			copy(txt[e.gapstart:], e.text[e.gapend:e.caret+e.gapLen()])
		} else {
			copy(txt, e.text[:e.caret])
			copy(txt[e.gapstart+gaplen:], e.text[e.gapend:])
			copy(txt[e.caret+gaplen:], e.text[e.caret:e.gapstart])
		}
		e.text = txt
		e.gapstart = e.caret
		e.gapend = e.gapstart + gaplen
	} else {
		if e.caret > e.gapstart {
			copy(e.text[e.gapstart:], e.text[e.gapend:e.caret+e.gapLen()])
		} else {
			copy(e.text[e.caret+e.gapLen():], e.text[e.caret:e.gapstart])
		}
		l := e.gapLen()
		e.gapstart = e.caret
		e.gapend = e.gapstart + l
	}
	e.dump()
}

func (e *editBuffer) len() int {
	return len(e.text) - e.gapLen()
}

func (e *editBuffer) gapLen() int {
	return e.gapend - e.gapstart
}

func (e *editBuffer) Read(p []byte) (int, error) {
	if e.pos == e.len() {
		return 0, io.EOF
	}
	var n int
	if e.pos < e.gapstart {
		n += copy(p, e.text[e.pos:e.gapstart])
		p = p[n:]
	}
	n += copy(p, e.text[e.gapend:])
	e.pos += n
	return n, nil
}

func (e *editBuffer) ReadRune() (rune, int, error) {
	if e.pos == e.len() {
		return 0, 0, io.EOF
	}
	r, s := e.runeAt(e.pos)
	e.pos += s
	return r, s, nil
}

func (e *editBuffer) String() string {
	var b strings.Builder
	b.Grow(e.len())
	b.Write(e.text[:e.gapstart])
	b.Write(e.text[e.gapend:])
	return b.String()
}

func (e *editBuffer) prepend(s string) {
	e.moveGap(len(s))
	copy(e.text[e.caret:], s)
	e.gapstart += len(s)
	e.changed = e.changed || len(s) > 0
	e.dump()
}

func (e *editBuffer) dump() {
	if bufferDebug {
		fmt.Printf("len(e.text) %d e.len() %d e.gapstart %d e.gapend %d e.caret %d txt:\n'%+x'<-%d->'%+x'\n", len(e.text), e.len(), e.gapstart, e.gapend, e.caret, e.text[:e.gapstart], e.gapLen(), e.text[e.gapend:])
	}
}

func (e *editBuffer) move(runes int) {
	for ; runes < 0 && e.caret > 0; runes++ {
		_, s := e.runeBefore(e.caret)
		e.caret -= s
	}
	for ; runes > 0 && e.caret < len(e.text); runes-- {
		_, s := e.runeAt(e.caret)
		e.caret += s
	}
	e.dump()
}

func (e *editBuffer) runeBefore(idx int) (rune, int) {
	if idx > e.gapstart {
		idx += e.gapLen()
	}
	return utf8.DecodeLastRune(e.text[:idx])
}

func (e *editBuffer) runeAt(idx int) (rune, int) {
	if idx >= e.gapstart {
		idx += e.gapLen()
	}
	return utf8.DecodeRune(e.text[idx:])
}
