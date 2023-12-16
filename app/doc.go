// SPDX-License-Identifier: Unlicense OR MIT

/*
Package app provides a platform-independent interface to operating system
functionality for running graphical user interfaces.

See https://gioui.org for instructions to set up and run Gio programs.

# Windows

A Window is run by calling its Event method in a loop. The first time a
method on Window is called, a new GUI window is created and shown. On mobile
platforms or when Gio is embedded in another project, Window merely connects
with a previously created GUI window.

The most important event is [FrameEvent] that prompts an update of the window
contents.

For example:

	w := new(app.Window)
	for {
		e := w.Event()
		if e, ok := e.(app.FrameEvent); ok {
			ops.Reset()
			// Add operations to ops.
			...
			// Completely replace the window contents and state.
			e.Frame(ops)
		}
	}

A program must keep receiving events from the event channel until
[DestroyEvent] is received.

# Main Thread

Some GUI platform need access to the main thread of the program. To avoid a
deadlock on such platforms, at least one Window must have its Event method
called by the main goroutine. It doesn't have to be any particular Window;
even a destroyed Window suffices.

# Permissions

The packages under gioui.org/app/permission should be imported
by a Gio program or by one of its dependencies to indicate that specific
operating-system permissions are required.  Please see documentation for
package gioui.org/app/permission for more information.
*/
package app
