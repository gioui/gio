package system

import (
	"strings"

	"gioui.org/io/pointer"
)

// Action is a set of window decoration actions.
type Action uint

const (
	// ActionMinimize minimizes a window.
	ActionMinimize Action = 1 << iota
	// ActionMaximize maximizes a window.
	ActionMaximize
	// ActionUnmaximize restores a maximized window.
	ActionUnmaximize
	// ActionFullscreen makes a window fullscreen.
	ActionFullscreen
	// ActionClose closes a window.
	ActionClose
	// ActionMove moves a window directed by the user.
	ActionMove
	// ActionResizeNorth resizes the top border of a window (directed by the user).
	ActionResizeNorth
	// ActionResizeSouth resizes the bottom border of a window (directed by the user).
	ActionResizeSouth
	// ActionResizeWest resizes the right border of a window (directed by the user).
	ActionResizeWest
	// ActionResizeEast resizes the left border of a window (directed by the user).
	ActionResizeEast
	// ActionResizeNorthWest resizes the top-left corner of a window (directed by the user).
	ActionResizeNorthWest
	// ActionResizeSouthWest resizes the bottom-left corner of a window (directed by the user).
	ActionResizeSouthWest
	// ActionResizeNorthEast resizes the top-right corner of a window (directed by the user).
	ActionResizeNorthEast
	// ActionResizeSouthEast resizes the bottom-right corner of a window (directed by the user).
	ActionResizeSouthEast
)

// CursorName returns the cursor for the action.
// It must be a single action otherwise the default
// cursor is returned.
func (a Action) CursorName() pointer.CursorName {
	switch a {
	case ActionResizeNorthWest:
		return pointer.CursorTopLeftResize
	case ActionResizeSouthEast:
		return pointer.CursorBottomRightResize
	case ActionResizeNorthEast:
		return pointer.CursorTopRightResize
	case ActionResizeSouthWest:
		return pointer.CursorBottomLeftResize
	case ActionResizeWest:
		return pointer.CursorLeftResize
	case ActionResizeEast:
		return pointer.CursorRightResize
	case ActionResizeNorth:
		return pointer.CursorTopResize
	case ActionResizeSouth:
		return pointer.CursorBottomResize
	}
	return pointer.CursorDefault
}

func (a Action) String() string {
	var buf strings.Builder
	for b := Action(1); a != 0; b <<= 1 {
		if a&b != 0 {
			if buf.Len() > 0 {
				buf.WriteByte('|')
			}
			buf.WriteString(b.string())
			a &^= b
		}
	}
	return buf.String()
}

func (a Action) string() string {
	switch a {
	case ActionMinimize:
		return "ActionMinimize"
	case ActionMaximize:
		return "ActionMaximize"
	case ActionUnmaximize:
		return "ActionUnmaximize"
	case ActionClose:
		return "ActionClose"
	case ActionMove:
		return "ActionMove"
	case ActionResizeNorth:
		return "ActionResizeNorth"
	case ActionResizeSouth:
		return "ActionResizeSouth"
	case ActionResizeWest:
		return "ActionResizeWest"
	case ActionResizeEast:
		return "ActionResizeEast"
	case ActionResizeNorthWest:
		return "ActionResizeNorthWest"
	case ActionResizeSouthWest:
		return "ActionResizeSouthWest"
	case ActionResizeNorthEast:
		return "ActionResizeNorthEast"
	case ActionResizeSouthEast:
		return "ActionResizeSouthEast"
	}
	return ""
}
