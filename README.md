# Gio

Gio implements portable immediate mode GUI programs in Go. Gio programs run on all the major platforms:
iOS/tvOS, Android, Linux (Wayland), macOS and Windows.

## Quickstart

Gio is designed to work with very few dependencies. It depends only on the platform libraries for
window management, input and GPU drawing.

For Linux you need Wayland and the `wayland-client`, `wayland-egl`, `wayland-cursor`, and `xkbcommon`
development packages.

Xcode is required for macOS and iOS.

For Windows you need the ANGLE drivers for emulating OpenGL ES. You can build ANGLE yourself or use
[mine](https://drive.google.com/file/d/1k2950mHNtR2iwhweHS1rJ7reChTa3rki/view?usp=sharing).

With Go 1.12 or newer,

	$ go run gioui.org/apps/gophers

should display a simple (nonsense) demo.

## Android

For Android you need the Android SDK with the NDK installed. Point the ANDROID_HOME to the SDK root
directory.

To build a Gio program as an .aar package, use the gio tool. For example,

	$ go run gioui.org/cmd/gio -target android gioui.org/apps/gophers

to produce gophers.aar, ready to use in an Android project. To run
the demo on an Android device:

	$ git clone https://git.sr.ht/~eliasnaur/gio
	$ cd gio/apps/gophers/android
	$ go run gioui.org/cmd/gio -target android ..
	$ ./gradlew installDebug          # gradlew.bat on Windows

The gio tool passes command line arguments to os.Args at runtime:

	$ go run gioui.org/cmd/gio -target android .. -token <github token>

## License

Dual-licensed under MIT or the [UNLICENSE](http://unlicense.org).

## Contributing

Discussion and patches: [~eliasnaur/gio-dev@lists.sr.ht](mailto:~eliasnaur/gio-dev@lists.sr.ht).
[Instructions](https://man.sr.ht/git.sr.ht/send-email.md). for using git-send-email for sending patches.

Contributors must agree to the [developer certificate og origin](https://developercertificate.org/),
to ensure their work is compatible with the MIT and the UNLICENSE. Sign your commits with Signed-off-by
statements to show your agreement. For convenience, the `git commit --sign` signs a commit with the
name and email from your `user.name` and `user.email` settings.

Bugs and TODOs go in the [issue tracker](https://todo.sr.ht/~eliasnaur/gio).
