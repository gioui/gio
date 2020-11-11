package main

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("RUN_GOGIO") != "" {
		// Allow the end-to-end tests to call the gogio tool without
		// having to build it from scratch, nor having to refactor the
		// main function to avoid using global variables.
		main()
		os.Exit(0) // main already exits, but just in case.
	}
	os.Exit(m.Run())
}
