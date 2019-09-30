// SPDX-License-Identifier: Unlicense OR MIT

/*
Package app provides a platform-independent interface to operating system
functionality for running graphical user interfaces.

Windows

Create a new Window by calling NewWindow. On mobile platforms or when Gio
is embedded in another project, NewWindow merely connects with a previously
created window.

A Window is run by receiving events from its Events channel. The most
important event is UpdateEvent that prompts an update of the window
contents and state.

For example:

	import "gioui.org/ui"

	w := app.NewWindow()
	for e := range w.Events() {
		if e, ok := e.(app.UpdateEvent); ok {
			ops.Reset()
			// Add operations to ops.
			...
			// Completely replace the window contents and state.
			w.Update(ops)
		}
	}

A program must keep receiving events from the event channel until
DestroyEvent is received.

Main

The Main function must be called from a programs main function, to hand over
control of the main thread to operating systems that need it.

Because Main is also blocking, the event loop of a Window must run in a goroutine.

For example, to display a blank but otherwise functional window:

	package main

	import "gioui.org/app"

	func main() {
		go func() {
			w := app.NewWindow()
			for range w.Events() {
			}
		}()
		app.Main()
	}


Event queue

A Window's Queue method returns an event.Queue implementation that distributes
incoming events to the event handlers declared in the latest call to Update.
See the gioui.org/ui package for more information about event handlers.

*/
package app
