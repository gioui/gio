// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import "gioui.org/io/pointer"

// ohosCursorNumber maps a Gio cursor to the shim's cursor enum. The numbers
// are part of ABI 2/3 and must match libgpofdohos; 0 is the default arrow.
func ohosCursorNumber(cursor pointer.Cursor) int {
	switch cursor {
	case pointer.CursorText, pointer.CursorVerticalText:
		return 1
	case pointer.CursorPointer:
		return 2
	case pointer.CursorCrosshair:
		return 3
	case pointer.CursorWait:
		return 4
	case pointer.CursorProgress:
		return 5
	case pointer.CursorGrab, pointer.CursorGrabbing:
		return 6
	case pointer.CursorColResize, pointer.CursorEastWestResize, pointer.CursorWestResize, pointer.CursorEastResize:
		return 7
	case pointer.CursorRowResize, pointer.CursorNorthSouthResize, pointer.CursorNorthResize, pointer.CursorSouthResize:
		return 8
	case pointer.CursorNotAllowed:
		return 9
	case pointer.CursorAllScroll:
		return 10
	case pointer.CursorNorthWestResize:
		return 11
	case pointer.CursorNorthEastResize:
		return 12
	case pointer.CursorSouthWestResize:
		return 13
	case pointer.CursorSouthEastResize:
		return 14
	case pointer.CursorNorthEastSouthWestResize:
		return 15
	case pointer.CursorNorthWestSouthEastResize:
		return 16
	case pointer.CursorNone:
		return -1
	}
	return 0
}
