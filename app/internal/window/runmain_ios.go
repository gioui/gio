// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,ios

package window

import "C"

//export gio_runMain
func gio_runMain() {
	runMain()
}
