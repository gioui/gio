// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"io"
	"path/filepath"
	"testing"

	"gioui.org/f32"
)

func TestFileDropEvent(t *testing.T) {
	for _, test := range []struct {
		name  string
		paths []string
		want  string
	}{
		{
			name:  "unix",
			paths: []string{"/tmp/a b.txt", "/tmp/文件#%.png"},
			want:  "file:///tmp/a%20b.txt\r\nfile:///tmp/%E6%96%87%E4%BB%B6%23%25.png\r\n",
		},
		{
			name:  "windows",
			paths: []string{filepath.FromSlash("C:/Users/example/a b.txt")},
			want:  "file:///C:/Users/example/a%20b.txt\r\n",
		},
		{
			name:  "unc",
			paths: []string{filepath.FromSlash("//server/share/a.txt")},
			want:  "file://server/share/a.txt\r\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			pos := f32.Pt(12, 34)
			e := fileDropEvent(pos, test.paths)
			if e.Type != "text/uri-list" || e.Position != pos {
				t.Fatalf("unexpected drop: %#v", e)
			}
			// Replaying the event must not reuse an exhausted reader.
			for range 2 {
				r := e.Open()
				data, err := io.ReadAll(r)
				if err := r.Close(); err != nil {
					t.Fatal(err)
				}
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != test.want {
					t.Fatalf("got %q, want %q", data, test.want)
				}
			}
		})
	}
}
