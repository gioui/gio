// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"testing"
)

func TestSourceString(t *testing.T) {
	for _, tc := range []struct {
		src  Source
		want string
	}{
		{Mouse, "Mouse"},
		{Touch, "Touch"},
		{Trackpad, "Trackpad"},
		{Pen, "Pen"},
	} {
		if got := tc.src.String(); got != tc.want {
			t.Errorf("got %q; want %q", got, tc.want)
		}
	}
}

func TestTypeString(t *testing.T) {
	for _, tc := range []struct {
		typ Kind
		res string
	}{
		{Cancel, "Cancel"},
		{Press, "Press"},
		{Release, "Release"},
		{Move, "Move"},
		{Drag, "Drag"},
		{Enter, "Enter"},
		{Leave, "Leave"},
		{Scroll, "Scroll"},
		{Pinch, "Pinch"},
		{Rotate, "Rotate"},
		{Swipe, "Swipe"},
		{Hold, "Hold"},
		{Pinch | Rotate, "Pinch|Rotate"},
		{Enter | Leave, "Enter|Leave"},
		{Press | Release, "Press|Release"},
		{Enter | Leave | Press | Release, "Press|Release|Enter|Leave"},
		{Move | Scroll, "Move|Scroll"},
	} {
		t.Run(tc.res, func(t *testing.T) {
			if want, got := tc.res, tc.typ.String(); want != got {
				t.Errorf("got %q; want %q", got, want)
			}
		})
	}
}
