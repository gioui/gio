// SPDX-License-Identifier: Unlicense OR MIT

/*
Package wakelock implements permission to acquire locks that keep the system
from suspending.

# Android

The following entries will be added to AndroidManifest.xml:

	<uses-permission android:name="android.permission.WAKE_LOCK"/>
*/
package wakelock
