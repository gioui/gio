// SPDX-License-Identifier: Unlicense OR MIT

/*
Package input implements input routing and tracking of interface
state for a window.

The [Source] is the interface between the window and the widgets
of a user interface and is exposed by [gioui.org/app.FrameEvent]
received from windows.

The [Router] is used by [gioui.org/app.Window] to track window state and route
events from the platform to event handlers. It is otherwise only
useful for using Gio with external window implementations.
*/
package input
