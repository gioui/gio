package window

var PlatformHandle *Handle

type Handle struct {
	// JVM is the JNI *JVM pointer.
	JVM uintptr
	// Context is a global reference to the application's
	// android.content.Context instance.
	Context uintptr
}
