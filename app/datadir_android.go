// SPDX-License-Identifier: Unlicense OR MIT

// +build android

package app

import "C"

import (
	"os"
	"path/filepath"
	"sync"

	"gioui.org/app/internal/window"
)

var (
	dataDirOnce sync.Once
	dataPath    string
)

func dataDir() (string, error) {
	dataDirOnce.Do(func() {
		dataPath = window.GetDataDir()
		// Set XDG_CACHE_HOME to make os.UserCacheDir work.
		cachePath := filepath.Join(dataPath, "cache")
		os.Setenv("XDG_CACHE_HOME", cachePath)
	})
	return dataPath, nil
}
