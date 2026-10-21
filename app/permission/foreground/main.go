// SPDX-License-Identifier: Unlicense OR MIT

/*
Package foreground implements permission to keep running while not in front,
as app.StartForeground does.

# Android

The following entries will be added to AndroidManifest.xml:

	<uses-permission android:name="android.permission.FOREGROUND_SERVICE"/>
	<uses-permission android:name="android.permission.FOREGROUND_SERVICE_SPECIAL_USE"/>
	<uses-permission android:name="android.permission.POST_NOTIFICATIONS"/>

and, in the application, the service that holds the process, of foreground
service type specialUse, which Google Play reviews.
*/
package foreground
