// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"gioui.org/io/transfer"
)

func fileDropEvent(paths []string) transfer.DataEvent {
	var data strings.Builder
	for _, path := range paths {
		path = filepath.ToSlash(path)
		u := url.URL{Scheme: "file"}
		switch {
		case strings.HasPrefix(path, "//"):
			// UNC paths use the server name as the URI authority.
			host, rest, _ := strings.Cut(path[2:], "/")
			u.Host, u.Path = host, "/"+rest
		case len(path) >= 2 && path[1] == ':':
			u.Path = "/" + path
		default:
			u.Path = path
		}
		data.WriteString(u.String())
		data.WriteString("\r\n")
	}
	list := data.String()
	return transfer.DataEvent{
		Type: "text/uri-list",
		Open: func() io.ReadCloser {
			return io.NopCloser(strings.NewReader(list))
		},
	}
}
