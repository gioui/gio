# Gio

Gio implements portable immediate mode GUI programs in Go. Gio programs run on all the major platforms:
iOS/tvOS, Android, Linux (Wayland), macOS, Windows and browsers (Webassembly/WebGL).

Gio includes an efficient vector renderer based on the Pathfinder project (https://github.com/pcwalton/pathfinder).
Text and other shapes are rendered using only their outlines without baking them into texture images,
to support efficient animations, transformed drawing and pixel resolution independence.

[![GoDoc](https://godoc.org/gioui.org/ui?status.svg)](https://godoc.org/gioui.org/ui)

## Quickstart

Gio is designed to work with very few dependencies. It depends only on the platform libraries for
window management, input and GPU drawing.

For Linux you need Wayland and the wayland, xkbcommon, GLES, EGL development packages. On Fedora 28 and newer,
install the dependencies with the command

	$ sudo dnf install wayland-devel libxkbcommon-devel mesa-libGLES-devel mesa-libEGL-devel

On Ubuntu 18.04 and newer, use

	$ sudo apt install libwayland-dev libxkbcommon-dev libgles2-mesa-dev libegl1-mesa-dev

Note that Gio does not run with the NVIDIA proprietary driver.

Xcode is required for macOS, iOS, tvOS.

For Windows you need the ANGLE drivers for emulating OpenGL ES. You can build ANGLE yourself or use
[a prebuilt version](https://drive.google.com/file/d/1k2950mHNtR2iwhweHS1rJ7reChTa3rki/view?usp=sharing).

With [Go 1.12](https://golang.org/dl/) or newer,

	$ export GO111MODULE=on
	$ go run gioui.org/apps/hello

should display a simple message in a window.

The command

	$ go run gioui.org/apps/gophers

runs a simple (nonsense) demo that displays Go contributors fetched from GitHub.

If you run into quota issues, supply a
[Github token](https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line)
with the `-token` flag:

	$ go run gioui.org/apps/gophers -token <github token>

## Android

For Android you need the Android SDK with the NDK installed. Point the ANDROID_HOME to the SDK root
directory.

To build a Gio program as an .aar package, use the gio tool. For example,

	$ export ANDROID_HOME=...
	$ go run gioui.org/cmd/gio -target android gioui.org/apps/gophers

produces gophers.aar, ready to use in an Android project. To run
a Gio program on an Android device or emulator, use -buildmode=exe:

	$ go run gioui.org/cmd/gio -buildmode exe -target android gioui.org/apps/gophers

Install the apk to a running emulator or attached device with adb:

	$ adb install gophers.apk

The gio tool passes command line arguments to os.Args at runtime:

	$ go run gioui.org/cmd/gio -buildmode exe -target android gioui.org/apps/gophers -token <github token>

## iOS/tvOS

To build a Gio program for iOS or tvOS you need a macOS machine with Xcode installed.

The gio tool can produce a framework ready to include in an Xcode project. For example,

	$ go run gioui.org/cmd/gio -target ios gioui.org/apps/gophers

outputs Gophers.framework with the demo program built for iOS. For tvOS, use `-target tvos`:

	$ go run gioui.org/cmd/gio -target tvos gioui.org/apps/gophers

Building for tvOS requires (the not yet released) Go 1.13.

To run a Gio program on an iOS device, use -buildmode=exe:

	$ go run gioui.org/cmd/gio -buildmode exe -target ios -appid <bundle-id> gioui.org/apps/gophers

where <bundle-id> is a valid bundle identifier previously provisioned in Xcode for your device.

Use the Window=>Devices and Simulators option on Xcode to install the ipa file to the device.
If you have [ideviceinstaller](https://github.com/libimobiledevice/ideviceinstaller) installed,
you can install the app directly to your device:

	$ ideviceinstaller -i gophers.ipa

To run a program on a running simulator, use the -o flag with a .app directory:

	$ go run gioui.org/cmd/gio/ -o gophers.app -buildmode=exe -target ios gioui.org/apps/gophers

Install the app with simctl:

	$ xcrun simctl install booted gophers.app

## Webassembly/WebGL

To run a Gio program in a browser with WebAssembly and WebGL support, use the Go webassembly
driver and add a <div id="giowindow"> element to a HTML page. The gio tool can also output
a directory ready to view in a browser:

	$ go get github.com/shurcooL/goexec
	$ go run gioui.org/cmd/gio -target js gioui.org/apps/gophers
	$ goexec 'http.ListenAndServe(":8080", http.FileServer(http.Dir("gophers")))'

Open http://localhost:8080 in a browser to run the app.

## Issues

File bugs and TODOs through the the [issue tracker](https://todo.sr.ht/~eliasnaur/gio) or send an email
to [~eliasnaur/gio@todo.sr.ht](mailto:~eliasnaur/gio-dev@lists.sr.ht). For general discussion, use the
mailing list: [~eliasnaur/gio-dev@lists.sr.ht](mailto:~eliasnaur/gio-dev@lists.sr.ht).

## License

Dual-licensed under MIT or the [UNLICENSE](http://unlicense.org).

## Contributing

Post discussion and patches to the [mailing list](https://lists.sr.ht/~eliasnaur/gio-dev). Send your
message to [~eliasnaur/gio-dev@lists.sr.ht](mailto:~eliasnaur/gio-dev@lists.sr.ht); no Sourcehut account
is required and you can post without being subscribed.

Commit messages follow [the Go project style](https://golang.org/doc/contribute.html#commit_messages):
the first line is prefixed with the package and a short summary. The rest of the message provides context
for the change and what it does. See
[an example](https://git.sr.ht/~eliasnaur/gio/commit/abb9d291e954f3b80384046d7d4487e1ead6bd6a).
Add `Fixes ~eliasnaur/gio#nnn` or `Updates ~eliasnaur/gio#nnn` if the change fixes or updates an existing
issue.

The `git send-email` command is a convenient way to send patches to the mailing list. See
[git-send-email.io](https://git-send-email.io) for a thorough setup guide.

With `git send-email` configured, you can clone the project and set it up for submitting your changes:

	$ git clone https://git.sr.ht/~eliasnaur/gio
	$ cd gio
	$ git config sendemail.to '~eliasnaur/gio-dev@lists.sr.ht'
	$ git config sendemail.annotate yes

Configure your name and email address if you have not done so already:

	$ git config --global user.email "you@example.com"
	$ git config --global user.name "Your Name"

Contributors must agree to the [developer certificate of origin](https://developercertificate.org/),
to ensure their work is compatible with the MIT and the UNLICENSE. Sign your commits with Signed-off-by
statements to show your agreement. The `git commit --signoff` (or `-s`) command signs a commit with
your name and email address.

Whenever you want to submit your work for review, use `git send-email` with the base revision of your
changes. For example, to submit the most recent commit use

	$ git send-email HEAD^
