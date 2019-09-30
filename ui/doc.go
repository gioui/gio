// SPDX-License-Identifier: Unlicense OR MIT

/*
Package ui defines operations buffers, units and common operations
for GUI programs written with the Gio module.

Units

A Value is a value with a Unit attached.

Device independent pixel, or dp, is the unit for sizes independent of
the underlying display device.

Scaled pixels, or sp, is the unit for text sizes. An sp is like dp with
text scaling applied.

Finally, pixels, or px, is the unit for display dependent pixels. Their
size vary between platforms and displays.

To maintain a constant visual size across platforms and displays, always
use dps or sps to define user interfaces. Only use pixels for derived
values.

*/
package ui
