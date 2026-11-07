// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"

	"gioui.org/io/transfer"
)

func fileDropEvent(path string) transfer.DataEvent {
	typ := mime.TypeByExtension(filepath.Ext(path))
	if typ == "" {
		typ = "application/octet-stream"
	} else if mediaType, _, err := mime.ParseMediaType(typ); err == nil {
		typ = mediaType
	}
	return transfer.DataEvent{
		Type: typ,
		URI:  (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String(),
		Open: func() io.ReadCloser {
			file, err := os.Open(path)
			if err != nil {
				return errorReadCloser{err: err}
			}
			return file
		},
	}
}

type errorReadCloser struct {
	err error
}

func (e errorReadCloser) Read([]byte) (int, error) { return 0, e.err }
func (errorReadCloser) Close() error               { return nil }
