// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"testing"

	"gioui.org/op"
)

func TestTypeString(t *testing.T) {
	for _, tc := range []struct {
		typ Type
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

func TestTransformChecks(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("cross-macro Pop didn't panic")
		}
	}()
	var ops op.Ops
	area := AreaOp{}.Push(&ops)
	op.Record(&ops)
	area.Pop()
}
