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
	// ActionRaise requests that the platform bring this window to the top of all open windows.
	// Some platforms do not allow this except under certain circumstances, such as when
	// a window from the same application already has focus. If the platform does not
	// support it, this method will do nothing.
	ActionRaise
	// ActionCenter centers the window on the screen.
	// It is ignored in Fullscreen mode and on Wayland.
	ActionCenter
	// ActionClose closes a window.
	// Only applicable on macOS, Windows, X11 and Wayland.
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

// Cursor returns the cursor for the action.
// It must be a single action otherwise the default
// cursor is returned.
func (a Action) Cursor() pointer.Cursor {
	switch a {
	case ActionResizeNorthWest:
		return pointer.CursorNorthWestResize
	case ActionResizeSouthEast:
		return pointer.CursorSouthEastResize
	case ActionResizeNorthEast:
		return pointer.CursorNorthEastResize
	case ActionResizeSouthWest:
		return pointer.CursorSouthWestResize
	case ActionResizeWest:
		return pointer.CursorWestResize
	case ActionResizeEast:
		return pointer.CursorEastResize
	case ActionResizeNorth:
		return pointer.CursorNorthResize
	case ActionResizeSouth:
		return pointer.CursorSouthResize
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
