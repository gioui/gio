// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"bytes"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/transfer"
)

// fileDropType is the MIME type for file URI lists.
const fileDropType = "text/uri-list"

type platformDragDriver interface {
	StartDrag(data map[string][]byte)
	SetDropTypes(types []string)
}

func platformDrop(pos f32.Point, mime string, data []byte) transfer.DataEvent {
	return transfer.DataEvent{
		ExternalDrop: true,
		Position:     pos,
		Type:         mime,
		Open:         func() io.ReadCloser { return io.NopCloser(bytes.NewReader(data)) },
	}
}

func fileDropEvent(pos f32.Point, paths []string) transfer.DataEvent {
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
	return platformDrop(pos, fileDropType, []byte(data.String()))
}
