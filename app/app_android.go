package app

import (
	"gioui.org/app/internal/window"
)

type Handle window.Handle

// PlatformHandle returns the Android platform-specific Handle.
func PlatformHandle() *Handle {
	return (*Handle)(window.PlatformHandle)
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
