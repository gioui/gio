// SPDX-License-Identifier: Unlicense OR MIT

// +build android

package clipboard

import "testing"

func TestClipboard(t *testing.T) {
	const want = "Hello, 世界"
	if err := Write(want); err != nil {
		t.Fatal(err)
	}
	got, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("read %q from the clipboard, wanted %q", got, want)
	}
}
