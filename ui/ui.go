// SPDX-License-Identifier: Unlicense OR MIT

package ui

import (
	"time"
)

// Config define the essential properties of
// the environment.
type Config interface {
	// Now returns the current animation time.
	Now() time.Time
	// Px converts a Value to pixels.
	Px(v Value) int
}
