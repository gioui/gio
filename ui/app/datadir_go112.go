// SPDX-License-Identifier: Unlicense OR MIT

// +build !android,!go1.13

package app

import "os"

func dataDir() (string, error) {
	// Use a quick workaround until we can require go 1.13.
	return os.UserHomeDir()
}
