// SPDX-License-Identifier: Unlicense OR MIT

package main

const mainUsage = `The gogio command builds and packages Gio (gioui.org) programs.

Usage:

	gogio -target <target> [flags] <package> [run arguments]

The gogio tool builds and packages Gio programs for platforms where additional
metadata or support files are required.

The package argument specifies an import path or a single Go source file to
package. Any run arguments are appended to os.Args at runtime.

Compiled Java class files from jar files in the package directory are
included in Android builds.

The mandatory -target flag selects the target platform: ios or android for the
mobile platforms, tvos for Apple's tvOS, js for WebAssembly/WebGL.

The -arch flag specifies a comma separated list of GOARCHs to include. The
default is all supported architectures.

The -o flag specifies an output file or directory, depending on the target.

The -buildmode flag selects the build mode. Two build modes are available, exe
and archive. Buildmode exe outputs an .ipa file for iOS or tvOS, an .apk file
for Android or a directory with the WebAssembly module and support files for
a browser.

The -ldflags and -tags flags pass extra linker flags and tags to the go tool.

As a special case for iOS or tvOS, specifying a path that ends with ".app"
will output an app directory suitable for a simulator.

The other buildmode is archive, which will output an .aar library for Android
or a .framework for iOS and tvOS.

The -icon flag specifies a path to a PNG image to use as app icon on iOS and Android.
If left unspecified, the appicon.png file from the main package is used
(if it exists).

The -appid flag specifies the package name for Android or the bundle id for
iOS and tvOS. A bundle id must be provisioned through Xcode before the gogio
tool can use it.

The -version flag specifies the integer version code for Android and the last
component of the 1.0.X version for iOS and tvOS.

For Android builds the -minsdk flag specify the minimum SDK level. For example,
use -minsdk 22 to target Android 5.1 (Lollipop) and later.

For Windows builds the -minsdk flag specify the minimum OS version. For example,
use -mindk 10 to target Windows 10 only, -minsdk 6 for Windows Vista and later.

The -work flag prints the path to the working directory and suppress
its deletion.

The -x flag will print all the external commands executed by the gogio tool.
`
