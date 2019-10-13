// SPDX-License-Identifier: Unlicense OR MIT

// Package font implements a central font registry.
package font

import (
	"sync"

	"gioui.org/text"
)

var (
	mu          sync.Mutex
	initialized bool
	shaper      = new(text.Shaper)
)

// Default returns a singleton *text.Shaper that contains
// the registered fonts.
func Default() *text.Shaper {
	mu.Lock()
	defer mu.Unlock()
	initialized = true
	return shaper
}

// Register a face. Register panics if Default has been
// called.
func Register(font text.Font, face text.Face) {
	mu.Lock()
	defer mu.Unlock()
	if initialized {
		panic("Register must be called before Default")
	}
	shaper.Register(font, face)
}
