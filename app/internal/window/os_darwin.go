// SPDX-License-Identifier: Unlicense OR MIT

package window

/*
__attribute__ ((visibility ("hidden"))) void gio_wakeupMainThread(void);
*/
import "C"

var mainFuncs = make(chan func(), 1)

// runOnMain runs the function on the main thread.
func runOnMain(f func()) {
	go func() {
		mainFuncs <- f
		C.gio_wakeupMainThread()
	}()
}

//export gio_dispatchMainFuncs
func gio_dispatchMainFuncs() {
	for {
		select {
		case f := <-mainFuncs:
			f()
		default:
			return
		}
	}
}
