// SPDX-License-Identifier: Unlicense OR MIT

/*
Package bluetooth implements permissions to access Bluetooth and Bluetooth
Low Energy hardware, including the ability to discover and pair devices.

Android

The following entries will be added to AndroidManifest.xml:

    <uses-permission android:name="android.permission.BLUETOOTH"/>
    <uses-permission android:name="android.permission.BLUETOOTH_ADMIN"/>
    <uses-permission android:name="android.permission.ACCESS_FINE_LOCATION"/>
    <uses-feature android:name="android.hardware.bluetooth" android:required="false"/>
    <uses-feature android:name="android.hardware.bluetooth_le" android:required="false"/>

Note that ACCESS_FINE_LOCATION is required on Android before the Bluetooth
device may be used.
See https://developer.android.com/guide/topics/connectivity/bluetooth.

ACCESS_FINE_LOCATION is a "dangerous" permission. See documentation for
package gioui.org/app/permission for more information.
*/
package bluetooth
