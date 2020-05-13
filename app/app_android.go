// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"gioui.org/app/internal/window"
)

// JavaVM returns the global JNI JavaVM.
func JavaVM() uintptr {
	return window.JavaVM()
}

// AppContext returns the global Application context as a JNI
// jobject.
func AppContext() uintptr {
	return window.AppContext()
}

// androidDriver is an interface that allows the Window's run method
// to call the RegisterFragment method of the Android window driver.
type androidDriver interface {
	RegisterFragment(string)
}

// RegisterFragment constructs a Java instance of the specified class
// and registers it as a Fragment in the Context in which the View was
// created.
func (w *Window) RegisterFragment(del string) {
	go func() {
		w.driverFuncs <- func() {
			d := w.driver.(androidDriver)
			d.RegisterFragment(del)
		}
	}()
}
