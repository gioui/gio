// +build android darwin,ios

package app

// Android only supports non-Java programs as c-shared libraries.
// Unfortunately, Go does not run a program's main function in
// library mode. To make Gio programs simpler and uniform, we'll
// link to the main function here and call it from Java.

import _ "unsafe" // for go:linkname

//go:linkname mainMain main.main
func mainMain()

func runMain() {
	go func() {
		// Indirect call, since the linker does not know the address of main when
		// laying down this package.
		fn := mainMain
		fn()
	}()
}
