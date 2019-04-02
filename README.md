# Gio

Gio implements portable immediate mode GUI programs in Go. Gio programs run on all the major platforms:
iOS/tvOS, Android, Linux (Wayland), macOS and Windows.

Gio includes an efficient vector renderer based on the Pathfinder project (https://github.com/pcwalton/pathfinder).
Text and other shapes are rendered using only their outlines without baking them into texture images,
to support efficient animations, transformed drawing and pixel resolution independence.

[![GoDoc](https://godoc.org/gioui.org/ui?status.svg)](https://godoc.org/gioui.org/ui)

## Quickstart

Gio is designed to work with very few dependencies. It depends only on the platform libraries for
window management, input and GPU drawing.

For Linux you need Wayland and the wayland, xkbcommon, GLES development packages. On Fedora 28 and newer,
install the dependencies with the command

	$ sudo dnf install wayland-devel libxkbcommon-devel mesa-libGLES-devel

On Ubuntu 18.04 and newer, use

	$ sudo apt install libwayland-dev libxkbcommon-dev libgles2-mesa-dev

Xcode is required for macOS, iOS, tvOS.

For Windows you need the ANGLE drivers for emulating OpenGL ES. You can build ANGLE yourself or use
[a prebuilt version](https://drive.google.com/file/d/1k2950mHNtR2iwhweHS1rJ7reChTa3rki/view?usp=sharing).

With Go 1.12 or newer,

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
the demo on an Android device:

	$ git clone https://git.sr.ht/~eliasnaur/gio
	$ cd gio/apps/gophers/android
	$ go run gioui.org/cmd/gio -target android ..
	$ ./gradlew installDebug          # gradlew.bat on Windows

The gio tool passes command line arguments to os.Args at runtime:

	$ go run gioui.org/cmd/gio -target android .. -token <github token>

## iOS/tvOS

To build a Gio program for iOS or tvOS you need a macOS machine with Xcode installed.

The gio tool can produce a framework ready to include in an Xcode project. For example,

	$ go run gioui.org/cmd/gio -target ios gioui.org/apps/gophers

outputs Gophers.framework with the demo program built for iOS. For tvOS, use `-target tvos`:

	$ go run gioui.org/cmd/gio -target tvos gioui.org/apps/gophers

Building for tvOS requires (the not yet released) Go 1.13.

To run the demo on an iOS device, use the sample Xcode project:

	$ git clone https://git.sr.ht/~eliasnaur/gio
	$ cd gio/apps
	$ go run gioui.org/cmd/gio -target ios -o gophers/ios/gophers/Gophers.framework ./gophers
	$ open gophers/ios/gophers.xcodeproj/

You need to provide a valid bundle identifier and set up code signing in Xcode to run the demo
on a device. If you're using Go 1.12 or older, you also need to disable bitcode.

## License

Dual-licensed under MIT or the [UNLICENSE](http://unlicense.org).

## Contributing

Discussion and patches: [~eliasnaur/gio-dev@lists.sr.ht](mailto:~eliasnaur/gio-dev@lists.sr.ht).
[Instructions](https://man.sr.ht/git.sr.ht/send-email.md) for using git-send-email for sending patches.

Contributors must agree to the [developer certificate og origin](https://developercertificate.org/),
to ensure their work is compatible with the MIT and the UNLICENSE. Sign your commits with Signed-off-by
statements to show your agreement. The `git commit --sign` signs a commit with the name and email from
your `user.name` and `user.email` settings.

File bugs and TODOs in the [issue tracker](https://todo.sr.ht/~eliasnaur/gio).
