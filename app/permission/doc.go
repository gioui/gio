// SPDX-License-Identifier: Unlicense OR MIT

/*
Package permission includes sub-packages that should be imported
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

Android -- Dangerous Permissions

Certain permissions on Android are marked with a protection level of
"dangerous". This means that, in addition to including the relevant Gio
permission packages, your app will need to prompt the user specifically
to request access. This can be done with a java Fragment, installed using
(*app.Window).RegisterFragment(). For more information on dangerous
permissions, see: https://developer.android.com/guide/topics/permissions/overview#dangerous_permissions
*/
package permission
