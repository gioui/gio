// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"encoding/json"
	"unicode/utf8"
)

// CharStyle holds per-character style properties.
// Zero values mean "use default" (inherited from zone/document defaults).
type CharStyle struct {
	B    bool    // bold
	I    bool    // italic
	U    bool    // underline
	S    float32 // font size (0 = use default)
	F    string  // font reference path ("", = use default)
	C    string  // text color hex "#rrggbb" ("", = use default)
	BGC  string  // background color hex
	LH   float32 // line height multiplier (0 = default 1.25)
	A    string  // paragraph alignment: "", "center", "right", "justify"
}

// StyledRun is a contiguous run of characters sharing the same style.
type StyledRun struct {
	Text  string
	Style CharStyle
}

// FormattedBuffer holds styled text as a sequence of StyledRuns.
// Each character position maps to exactly one run's characters.
// Operations maintain run coalescing: adjacent runs with identical styles are merged.
type FormattedBuffer struct {
	runs []StyledRun
}

// NewFormattedBuffer creates an empty FormattedBuffer.
func NewFormattedBuffer() *FormattedBuffer {
	return &FormattedBuffer{}
}

// NewFormattedBufferFromText creates a FormattedBuffer from plain text with a default style.
func NewFormattedBufferFromText(text string, defaultStyle CharStyle) *FormattedBuffer {
	if text == "" {
		return &FormattedBuffer{}
	}
	return &FormattedBuffer{
		runs: []StyledRun{{Text: text, Style: defaultStyle}},
	}
}

// Text returns the full text content (all runs concatenated).
func (fb *FormattedBuffer) Text() string {
	totalLen := 0
	for _, r := range fb.runs {
		totalLen += len(r.Text)
	}
	buf := make([]byte, 0, totalLen)
	for _, r := range fb.runs {
		buf = append(buf, r.Text...)
	}
	return string(buf)
}

// Runes returns the full text as runes.
func (fb *FormattedBuffer) Runes() []rune {
	return []rune(fb.Text())
}

// Len returns the total number of runes.
func (fb *FormattedBuffer) Len() int {
	n := 0
	for _, r := range fb.runs {
		n += utf8.RuneCountInString(r.Text)
	}
	return n
}

// Runs returns the underlying styled runs (read-only access).
func (fb *FormattedBuffer) Runs() []StyledRun {
	return fb.runs
}

// StyleAt returns the style of the character at the given rune index.
// Returns the default style if the index is out of bounds.
func (fb *FormattedBuffer) StyleAt(runeIdx int) CharStyle {
	pos := 0
	for _, r := range fb.runs {
		runCount := utf8.RuneCountInString(r.Text)
		if runeIdx < pos+runCount {
			return r.Style
		}
		pos += runCount
	}
	return CharStyle{}
}

// SetStyle applies a style to a range of characters [from, to) (rune indices).
func (fb *FormattedBuffer) SetStyle(from, to int, style CharStyle) {
	if from >= to || from < 0 {
		return
	}
	if to > fb.Len() {
		to = fb.Len()
	}

	// Split at 'from' if needed, then split at 'to' if needed
	fb.splitAt(from)
	fb.splitAt(to)

	// Apply style to runs in range
	pos := 0
	for i := range fb.runs {
		runCount := utf8.RuneCountInString(fb.runs[i].Text)
		runStart := pos
		runEnd := pos + runCount

		if runStart >= to {
			break
		}
		if runEnd <= from {
			pos = runEnd
			continue
		}

		// This run overlaps [from, to) — apply style
		fb.runs[i].Style = style
		pos = runEnd
	}

	fb.coalesce()
}

// GetStyleRange returns styles for characters in [from, to).
// Returns a slice of CharStyle, one per rune.
func (fb *FormattedBuffer) GetStyleRange(from, to int) []CharStyle {
	if from >= to || from < 0 {
		return nil
	}
	if to > fb.Len() {
		to = fb.Len()
	}

	result := make([]CharStyle, 0, to-from)
	pos := 0
	for _, r := range fb.runs {
		runCount := utf8.RuneCountInString(r.Text)
		runEnd := pos + runCount

		if runEnd <= from || pos >= to {
			pos = runEnd
			continue
		}

		// Overlap
		start := from - pos
		if start < 0 {
			start = 0
		}
		end := to - pos
		if end > runCount {
			end = runCount
		}

		for i := start; i < end; i++ {
			result = append(result, r.Style)
		}
		pos = runEnd
	}
	return result
}

// InsertText inserts text at the given rune position with the given style.
func (fb *FormattedBuffer) InsertText(at int, text string, style CharStyle) {
	if text == "" {
		return
	}
	if at < 0 {
		at = 0
	}
	if at > fb.Len() {
		at = fb.Len()
	}

	fb.splitAt(at)

	// Insert new run
	newRun := StyledRun{Text: text, Style: style}
	if at == 0 {
		fb.runs = append([]StyledRun{newRun}, fb.runs...)
	} else if at >= fb.Len() {
		fb.runs = append(fb.runs, newRun)
	} else {
		// Find the run at position 'at' and insert before it
		pos := 0
		for i, r := range fb.runs {
			runCount := utf8.RuneCountInString(r.Text)
			if pos+runCount >= at {
				// Insert at the beginning of this run
				fb.runs = append(fb.runs[:i+1], fb.runs[i:]...)
				fb.runs[i] = newRun
				break
			}
			pos += runCount
		}
	}

	fb.coalesce()
}

// DeleteRange deletes characters in [from, to) (rune indices).
func (fb *FormattedBuffer) DeleteRange(from, to int) {
	if from >= to || from < 0 {
		return
	}
	if to > fb.Len() {
		to = fb.Len()
	}

	fb.splitAt(from)
	fb.splitAt(to)

	pos := 0
	newRuns := make([]StyledRun, 0, len(fb.runs))
	for _, r := range fb.runs {
		runCount := utf8.RuneCountInString(r.Text)
		runEnd := pos + runCount

		if runEnd <= from || pos >= to {
			// Fully outside range — keep
			newRuns = append(newRuns, r)
		} else if runStart := pos; runStart >= from && runEnd <= to {
			// Fully inside range — skip (delete)
		} else {
			// Partially overlapping — trim
			runes := []rune(r.Text)
			var kept []rune
			for i, ch := range runes {
				absIdx := pos + i
				if absIdx < from || absIdx >= to {
					kept = append(kept, ch)
				}
			}
			if len(kept) > 0 {
				newRuns = append(newRuns, StyledRun{Text: string(kept), Style: r.Style})
			}
		}
		pos = runEnd
	}

	fb.runs = newRuns
	fb.coalesce()
}

// ReplaceRange replaces characters in [from, to) with text styled by style.
func (fb *FormattedBuffer) ReplaceRange(from, to int, text string, style CharStyle) {
	fb.DeleteRange(from, to)
	fb.InsertText(from, text, style)
}

// splitAt splits the run containing rune position 'at' into two runs.
// After splitAt(n), the character at position n starts a new run.
func (fb *FormattedBuffer) splitAt(at int) {
	if at <= 0 || at >= fb.Len() {
		return
	}

	pos := 0
	for i, r := range fb.runs {
		runCount := utf8.RuneCountInString(r.Text)
		runEnd := pos + runCount

		if at >= pos && at < runEnd {
			splitPoint := at - pos
			runes := []rune(r.Text)

			left := string(runes[:splitPoint])
			right := string(runes[splitPoint:])

			if left != "" && right != "" {
				// Replace this run with two runs
				fb.runs[i] = StyledRun{Text: left, Style: r.Style}
				fb.runs = append(fb.runs[:i+1], append([]StyledRun{{Text: right, Style: r.Style}}, fb.runs[i+1:]...)...)
			}
			// If one side is empty, no split needed
			return
		}
		pos = runEnd
	}
}

// coalesce merges adjacent runs with identical styles.
func (fb *FormattedBuffer) coalesce() {
	if len(fb.runs) <= 1 {
		return
	}
	merged := make([]StyledRun, 0, len(fb.runs))
	merged = append(merged, fb.runs[0])

	for i := 1; i < len(fb.runs); i++ {
		last := &merged[len(merged)-1]
		if stylesEqual(last.Style, fb.runs[i].Style) {
			last.Text += fb.runs[i].Text
		} else {
			merged = append(merged, fb.runs[i])
		}
	}
	fb.runs = merged
}

// stylesEqual returns true if two CharStyles are identical.
func stylesEqual(a, b CharStyle) bool {
	return a.B == b.B && a.I == b.I && a.U == b.U &&
		a.S == b.S && a.F == b.F && a.C == b.C &&
		a.BGC == b.BGC && a.LH == b.LH && a.A == b.A
}

// --- Serialization for JSON persistence ---

// BufferRunJSON is the JSON representation of a single styled run.
type BufferRunJSON struct {
	T   string    `json:"t"`             // text
	B   bool      `json:"b,omitempty"`   // bold
	I   bool      `json:"i,omitempty"`   // italic
	U   bool      `json:"u,omitempty"`   // underline
	S   float32   `json:"s,omitempty"`   // font size
	F   string    `json:"f,omitempty"`   // font ref
	C   string    `json:"c,omitempty"`   // text color
	BGC string    `json:"bgc,omitempty"` // bg color
	LH  float32   `json:"lh,omitempty"`  // line height
	A   string    `json:"a,omitempty"`   // alignment
}

// MarshalJSON serializes the FormattedBuffer to JSON.
func (fb *FormattedBuffer) MarshalJSON() ([]byte, error) {
	entries := make([]BufferRunJSON, 0, len(fb.runs))
	for _, r := range fb.runs {
		entry := BufferRunJSON{
			T:   r.Text,
			B:   r.Style.B,
			I:   r.Style.I,
			U:   r.Style.U,
			S:   r.Style.S,
			F:   r.Style.F,
			C:   r.Style.C,
			BGC: r.Style.BGC,
			LH:  r.Style.LH,
			A:   r.Style.A,
		}
		entries = append(entries, entry)
	}
	return json.Marshal(entries)
}

// UnmarshalJSON deserializes a FormattedBuffer from JSON.
func (fb *FormattedBuffer) UnmarshalJSON(data []byte) error {
	var entries []BufferRunJSON
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	fb.runs = make([]StyledRun, 0, len(entries))
	for _, e := range entries {
		fb.runs = append(fb.runs, StyledRun{
			Text: e.T,
			Style: CharStyle{
				B:   e.B,
				I:   e.I,
				U:   e.U,
				S:   e.S,
				F:   e.F,
				C:   e.C,
				BGC: e.BGC,
				LH:  e.LH,
				A:   e.A,
			},
		})
	}
	return nil
}

// ToJSON returns a JSON byte slice of the buffer content.
func (fb *FormattedBuffer) ToJSON() ([]byte, error) {
	return json.Marshal(fb)
}

// FromJSON creates a FormattedBuffer from JSON.
func FromJSON(data []byte) (*FormattedBuffer, error) {
	fb := &FormattedBuffer{}
	if err := fb.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return fb, nil
}
