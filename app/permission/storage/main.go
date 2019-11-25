// SPDX-License-Identifier: Unlicense OR MIT

/*
Package storage implements read and write storage permissions
on mobile devices.

Android

The following entries will be added to AndroidManifest.xml:

    <uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE"/>
    <uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAGE"/>

READ_EXTERNAL_STORAGE and WRITE_EXTERNAL_STORAGE are "dangerous" permissions.
See documentation for package gioui.org/app/permission for more information.
*/
package storage
