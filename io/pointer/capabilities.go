// SPDX-License-Identifier: Unlicense OR MIT

package pointer

// Capabilities describes native pointer/gesture support for a window.
// A zero value means no native semantic gesture support.
type Capabilities uint32

const (
	// CapabilityPinch means native pinch events are available.
	CapabilityPinch Capabilities = 1 << iota
	// CapabilityRotate means native rotate events are available.
	CapabilityRotate
	// CapabilitySwipe means native swipe events are available.
	CapabilitySwipe
	// CapabilityHold means native hold events are available.
	CapabilityHold
	// CapabilityTrackpad means the backend can distinguish trackpad input.
	CapabilityTrackpad
	// CapabilityTouch means touch input is available.
	CapabilityTouch
	// CapabilityPen means pen input is available.
	CapabilityPen
)

// Has reports whether all bits in want are present in c.
func (c Capabilities) Has(want Capabilities) bool {
	return c&want == want
}
