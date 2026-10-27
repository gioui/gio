// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony

package main

/*
#include <stdlib.h>
*/
import "C"

import "gioui.org/app"

func init() {
	debugLog = func(s string) { app.OHOSLog(s) }
}

// The OHOS shim retains the gpofd entry-point names. This diagnostic app
// ignores the data directory and URI requests.
//
//export gpofd_start
func gpofd_start(dir *C.char) C.int {
	debugLog("gesture-test: gpofd_start")
	return 0
}

//export gpofd_open_uri
func gpofd_open_uri(uri *C.char) C.int { return 0 }

//export gpofd_abi_version
func gpofd_abi_version() C.int { return 2 }
