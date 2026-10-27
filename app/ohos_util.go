// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import "gioui.org/io/key"

// ohosInputHint maps a Gio input hint to the shim's ime_config value.
func ohosInputHint(h key.InputHint) int {
	switch h {
	case key.HintText:
		return 1
	case key.HintNumeric:
		return 2
	case key.HintEmail:
		return 3
	case key.HintURL:
		return 4
	case key.HintTelephone:
		return 5
	case key.HintPassword:
		return 6
	}
	return 0
}
