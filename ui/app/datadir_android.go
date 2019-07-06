// SPDX-License-Identifier: Unlicense OR MIT

// +build android

package app

import "C"

var dataDir func() (string, error)

//export setDataDir
func setDataDir(cdir *C.char) {
	dir := C.GoString(cdir)
	dataDir = func() (string, error) {
		return dir, nil
	}
}

