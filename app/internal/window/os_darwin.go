// SPDX-License-Identifier: Unlicense OR MIT

package window

/*
#include <Foundation/Foundation.h>

__attribute__ ((visibility ("hidden"))) void gio_wakeupMainThread(void);
__attribute__ ((visibility ("hidden"))) NSUInteger gio_nsstringLength(CFTypeRef str);
__attribute__ ((visibility ("hidden"))) void gio_nsstringGetCharacters(CFTypeRef str, unichar *chars, NSUInteger loc, NSUInteger length);
*/
import "C"
import (
	"unicode/utf16"
	"unsafe"
)

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

// nsstringToString converts a NSString to a Go string, and
// releases the original string.
func nsstringToString(str C.CFTypeRef) string {
	defer C.CFRelease(str)
	n := C.gio_nsstringLength(str)
	chars := make([]uint16, n)
	C.gio_nsstringGetCharacters(str, (*C.unichar)(unsafe.Pointer(&chars[0])), 0, n)
	utf8 := utf16.Decode(chars)
	return string(utf8)
}
