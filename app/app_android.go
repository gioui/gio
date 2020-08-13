// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"gioui.org/app/internal/window"
)

type ViewEvent = window.ViewEvent

// JavaVM returns the global JNI JavaVM.
func JavaVM() uintptr {
	return window.JavaVM()
}

// AppContext returns the global Application context as a JNI
// jobject.
func AppContext() uintptr {
	return window.AppContext()
}
