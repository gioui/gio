// SPDX-License-Identifier: Unlicense OR MIT

/*
Package app provides a platform-independent interface to operating system
functionality for running graphical user interfaces.

See https://gioui.org for instructions to set up and run Gio programs.

Windows

Create a new Window by calling NewWindow. On mobile platforms or when Gio
is embedded in another project, NewWindow merely connects with a previously
created window.

A Window is run by receiving events from its Events channel. The most
important event is FrameEvent that prompts an update of the window
contents and state.

For example:

	import "gioui.org/unit"

	w := app.NewWindow()
	for e := range w.Events() {
		if e, ok := e.(app.FrameEvent); ok {
			ops.Reset()
			// Add operations to ops.
			...
			// Completely replace the window contents and state.
			e.Frame(ops)
		}
	}

A program must keep receiving events from the event channel until
DestroyEvent is received.

Main

The Main function must be called from a program's main function, to hand over
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
incoming events to the event handlers declared in the latest frame.
See the gioui.org/io/event package for more information about event handlers.

Permissions

The packages under gioui.org/app/permission should be imported
by a Gio program or by one of its dependencies to indicate that specific
operating-system permissions are required. For example, if a Gio
program requires access to a device's Bluetooth interface, it
should import "gioui.org/app/permission/bluetooth" as follows:

	package main

	import (
		"gioui.org/app"
		_ "gioui.org/app/permission/bluetooth"
	)

	func main() {
		...
	}

Since there are no exported identifiers in the app/permission/bluetooth
package, the import uses the anonymous identifier (_) as the imported
package name.

As a special case, the gogio tool detects when a program directly or
indirectly depends on the "net" package from the Go standard library as an
indication that the program requires network access permissions. If a program
requires network permissions but does not directly or indirectly import
"net", it will be necessary to add the following code somewhere in the
program's source code:

	import (
		...
		_ "net"
	)
*/
package app
