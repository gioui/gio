// SPDX-License-Identifier: Unlicense OR MIT

// +build android

// Package clipboard accesses the system clipboard.
package clipboard

import (
	"gioui.org/app/internal/window"
)

// Read the content of the clipboard as a string.
func Read() (string, error) {
	return window.ReadClipboard()
}

// Write a string to the clipboard.
func Write(s string) error {
	return window.WriteClipboard(s)
}
