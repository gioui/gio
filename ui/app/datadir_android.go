// SPDX-License-Identifier: Unlicense OR MIT

// +build android

package app

import "C"

var dataDir func() (string, error)

//export setDataDir
func setDataDir(cdir *C.char, len C.int) {
	dir := C.GoStringN(cdir, len)
	dataDir = func() (string, error) {
		return dir, nil
	}
}
