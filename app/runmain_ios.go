// +build darwin,ios

package app

import "C"

//export gio_runMain
func gio_runMain() {
	runMain()
}
