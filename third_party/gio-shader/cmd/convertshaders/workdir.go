// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type WorkDir string

func (wd WorkDir) Dir(path string) WorkDir {
	dirname := filepath.Join(string(wd), path)
	if err := os.Mkdir(dirname, 0755); err != nil {
		if !os.IsExist(err) {
			fmt.Fprintf(os.Stderr, "failed to create %q: %v\n", dirname, err)
		}
	}
	return WorkDir(dirname)
}

func (wd WorkDir) Path(path ...string) (fullpath string) {
	return filepath.Join(string(wd), strings.Join(path, "."))
}

func (wd WorkDir) WriteFile(path string, data []byte) error {
	err := ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("unable to create %v: %w", path, err)
	}
	return nil
}
