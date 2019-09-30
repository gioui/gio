// SPDX-License-Identifier: Unlicense OR MIT

// +build android

package app

import "C"
import "sync"

var (
	dataDirOnce sync.Once
	dataDirChan = make(chan string, 1)
	dataPath    string
)

func dataDir() (string, error) {
	dataDirOnce.Do(func() {
		dataPath = <-dataDirChan
	})
	return dataPath, nil
}

func setDataDir(dir string) {
	dataDirChan <- dir
}
