// SPDX-License-Identifier: Unlicense OR MIT

// Package key implements key and text events and operations.
package key

import (
	"strings"

	"gioui.org/f32"
	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/op"
)

// Filter matches any [Event] that matches the parameters.
type Filter struct {
	// Focus is the tag that must be focused for the filter to match. It has no effect
	// if it is nil.
	Focus event.Tag
	// Required is the set of modifiers that must be included in events matched.
	Required Modifiers
	// Optional is the set of modifiers that may be included in events matched.
	Optional Modifiers
	// Name of the key to be matched. As a special case, the empty
	// Name matches every key not matched by any other filter.
	Name Name
}

// InputHintOp describes the type of text expected by a tag.
type InputHintOp struct {
	Tag  event.Tag
	Hint InputHint
}

// SoftKeyboardCmd shows or hides the on-screen keyboard, if available.
type SoftKeyboardCmd struct {
	Show bool
}

// SelectionCmd updates the selection for an input handler.
type SelectionCmd struct {
	Tag event.Tag
	Range
	Caret
}

// SnippetCmd updates the content snippet for an input handler.
type SnippetCmd struct {
	Tag event.Tag
	Snippet
}

// Range represents a range of text, such as an editor's selection.
// Start and End are in runes.
type Range struct {
	Start int
	End   int
}

// Snippet represents a snippet of text content used for communicating between
// an editor and an input method.
type Snippet struct {
	Range
	Text string
}

// Caret represents the position of a caret.
type Caret struct {
	// Pos is the intersection point of the caret and its baseline.
	Pos f32.Point
	// Ascent is the length of the caret above its baseline.
	Ascent float32
	// Descent is the length of the caret below its baseline.
	Descent float32
}

// SelectionEvent is generated when an input method changes the selection.
type SelectionEvent Range

// SnippetEvent is generated when the snippet range is updated by an
// input method.
type SnippetEvent Range

// A FocusEvent is generated when a handler gains or loses
// focus.
type FocusEvent struct {
	Focus bool
}

// An Event is generated when a key is pressed. For text input
// use EditEvent.
type Event struct {
	// Name of the key.
	Name Name
	// Modifiers is the set of active modifiers when the key was pressed.
	Modifiers Modifiers
	// State is the state of the key when the event was fired.
	State State
}

// An EditEvent requests an edit by an input method.
type EditEvent struct {
	// Range specifies the range to replace with Text.
	Range Range
	Text  string
}

// FocusFilter matches any [FocusEvent], [EditEvent], [SnippetEvent],
// or [SelectionEvent] with the specified target.
type FocusFilter struct {
	// Target is a tag specified in a previous event.Op.
	Target event.Tag
}

// InputHint changes the on-screen-keyboard type. That hints the
// type of data that might be entered by the user.
type InputHint uint8

const (
	// HintAny hints that any input is expected.
	HintAny InputHint = iota
	// HintText hints that text input is expected. It may activate auto-correction and suggestions.
	HintText
	// HintNumeric hints that numeric input is expected. It may activate shortcuts for 0-9, "." and ",".
	HintNumeric
	// HintEmail hints that email input is expected. It may activate shortcuts for common email characters, such as "@" and ".com".
	HintEmail
	// HintURL hints that URL input is expected. It may activate shortcuts for common URL fragments such as "/" and ".com".
	HintURL
	// HintTelephone hints that telephone number input is expected. It may activate shortcuts for 0-9, "#" and "*".
	HintTelephone
	// HintPassword hints that password input is expected. It may disable autocorrection and enable password autofill.
	HintPassword
)

// State is the state of a key during an event.
type State uint8

const (
	// Press is the state of a pressed key.
	Press State = iota
	// Release is the state of a key that has been released.
	//
	// Note: release events are only implemented on the following platforms:
	// macOS, Linux, Windows, WebAssembly.
	Release
)

// Modifiers
type Modifiers uint32

const (
	// ModCtrl is the ctrl modifier key.
	ModCtrl Modifiers = 1 << iota
	// ModCommand is the command modifier key
	// found on Apple keyboards.
	ModCommand
	// ModShift is the shift modifier key.
	ModShift
	// ModAlt is the alt modifier key, or the option
	// key on Apple keyboards.
	ModAlt
	// ModSuper is the "logo" modifier key, often
	// represented by a Windows logo.
	ModSuper
)

// Name is the identifier for a keyboard key.
//
// For letters, the upper case form is used, via unicode.ToUpper.
// The shift modifier is taken into account, all other
// modifiers are ignored. For example, the "shift-1" and "ctrl-shift-1"
// combinations both give the Name "!" with the US keyboard layout.
type Name string

const (
	// Names for special keys.
	NameLeftArrow      Name = "←"
	NameRightArrow     Name = "→"
	NameUpArrow        Name = "↑"
	NameDownArrow      Name = "↓"
	NameReturn         Name = "⏎"
	NameEnter          Name = "⌤"
	NameEscape         Name = "⎋"
	NameHome           Name = "⇱"
	NameEnd            Name = "⇲"
	NameDeleteBackward Name = "⌫"
	NameDeleteForward  Name = "⌦"
	NamePageUp         Name = "⇞"
	NamePageDown       Name = "⇟"
	NameTab            Name = "Tab"
	NameSpace          Name = "Space"
	NameCtrl           Name = "Ctrl"
	NameShift          Name = "Shift"
	NameAlt            Name = "Alt"
	NameSuper          Name = "Super"
	NameCommand        Name = "⌘"
	NameF1             Name = "F1"
	NameF2             Name = "F2"
	NameF3             Name = "F3"
	NameF4             Name = "F4"
	NameF5             Name = "F5"
	NameF6             Name = "F6"
	NameF7             Name = "F7"
	NameF8             Name = "F8"
	NameF9             Name = "F9"
	NameF10            Name = "F10"
	NameF11            Name = "F11"
	NameF12            Name = "F12"
	NameBack           Name = "Back"
)

type FocusDirection int

const (
	FocusRight FocusDirection = iota
	FocusLeft
	FocusUp
	FocusDown
	FocusForward
	FocusBackward
)

// Contain reports whether m contains all modifiers
// in m2.
func (m Modifiers) Contain(m2 Modifiers) bool {
	return m&m2 == m2
}

// FocusCmd requests to set or clear the keyboard focus.
type FocusCmd struct {
	// Tag is the new focus. The focus is cleared if Tag is nil, or if Tag
	// has no [event.Op] references.
	Tag event.Tag
}

func (h InputHintOp) Add(o *op.Ops) {
	if h.Tag == nil {
		panic("Tag must be non-nil")
	}
	data := ops.Write1(&o.Internal, ops.TypeKeyInputHintLen, h.Tag)
	data[0] = byte(ops.TypeKeyInputHint)
	data[1] = byte(h.Hint)
}

func (EditEvent) ImplementsEvent()      {}
func (Event) ImplementsEvent()          {}
func (FocusEvent) ImplementsEvent()     {}
func (SnippetEvent) ImplementsEvent()   {}
func (SelectionEvent) ImplementsEvent() {}

func (FocusCmd) ImplementsCommand()        {}
func (SoftKeyboardCmd) ImplementsCommand() {}
func (SelectionCmd) ImplementsCommand()    {}
func (SnippetCmd) ImplementsCommand()      {}

func (Filter) ImplementsFilter()      {}
func (FocusFilter) ImplementsFilter() {}

func (m Modifiers) String() string {
	var strs []string
	if m.Contain(ModCtrl) {
		strs = append(strs, string(NameCtrl))
	}
	if m.Contain(ModCommand) {
		strs = append(strs, string(NameCommand))
	}
	if m.Contain(ModShift) {
		strs = append(strs, string(NameShift))
	}
	if m.Contain(ModAlt) {
		strs = append(strs, string(NameAlt))
	}
	if m.Contain(ModSuper) {
		strs = append(strs, string(NameSuper))
	}
	return strings.Join(strs, "-")
}

func (s State) String() string {
	switch s {
	case Press:
		return "Press"
	case Release:
		return "Release"
	default:
		panic("invalid State")
	}
}
