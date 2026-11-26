// SPDX-License-Identifier: Unlicense OR MIT

//go:build ((linux && !android) || freebsd) && !nowayland

package app

import (
	"bytes"
	"os"
	"testing"
	"time"

	syscall "golang.org/x/sys/unix"
)

func TestWriteClipboard(t *testing.T) {
	// Receivers pass blocking or non-blocking pipes; with a slow reader, a
	// single write to a non-blocking pipe comes back short.
	t.Run("blocking", func(t *testing.T) { testWriteClipboard(t, 0) })
	t.Run("non-blocking", func(t *testing.T) { testWriteClipboard(t, syscall.O_NONBLOCK) })
}

func testWriteClipboard(t *testing.T, flags int) {
	var p [2]int
	if err := syscall.Pipe2(p[:], syscall.O_CLOEXEC|flags); err != nil {
		t.Fatal(err)
	}
	r, fd := os.NewFile(uintptr(p[0]), "r"), p[1]
	defer r.Close()
	data := make([]byte, 5<<20)
	for i := range data {
		data[i] = byte(i * 7)
	}
	got := make(chan []byte)
	go func() {
		var buf bytes.Buffer
		chunk := make([]byte, 4096)
		for {
			n, err := r.Read(chunk)
			buf.Write(chunk[:n])
			if err != nil {
				got <- buf.Bytes()
				return
			}
			if buf.Len()%(256<<10) < 4096 {
				time.Sleep(time.Millisecond)
			}
		}
	}()
	if err := writeClipboard(fd, data); err != nil {
		t.Fatal(err)
	}
	if b := <-got; !bytes.Equal(b, data) {
		t.Errorf("read %d bytes, want %d identical bytes", len(b), len(data))
	}
}
