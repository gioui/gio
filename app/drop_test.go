// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileDropEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "example.txt")
	const content = "drop me"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	e := fileDropEvent(path)
	if e.Type != "text/plain" {
		t.Fatalf("unexpected MIME type: %q", e.Type)
	}
	if !strings.HasPrefix(e.URI, "file://") {
		t.Fatalf("unexpected URI: %q", e.URI)
	}
	r := e.Open()
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("unexpected contents: %q", data)
	}
}
