// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import "testing"

func TestCapabilitiesHas(t *testing.T) {
	c := CapabilityPinch | CapabilityTrackpad
	if !c.Has(CapabilityPinch) || !c.Has(CapabilityTrackpad) || c.Has(CapabilitySwipe) {
		t.Fatalf("capabilities = %b", c)
	}
	if (Capabilities(0)).Has(CapabilityPinch) {
		t.Fatal("zero capabilities must not match")
	}
}
