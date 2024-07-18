// SPDX-License-Identifier: Unlicense OR MIT

//go:build !android
// +build !android

package app

// app.Start is a no-op on platforms other than android
func startForeground(title, text string) (stop func(), err error) {
	return func() {}, nil
}
