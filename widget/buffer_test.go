// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestEditBuffer_ReadAt(t *testing.T) {
	type testCase struct {
		name   string
		setup  func() (buf editBuffer, p []byte, offset int64)
		expect func() ([]byte, int, error)
	}

	stubBuf := editBuffer{
		text: []byte("1234"),
	}

	tests := []testCase{
		{
			name: "zero p bytes",
			setup: func() (buf editBuffer, p []byte, offset int64) {
				buf = stubBuf

				return buf, []byte{}, 0
			},

			expect: func() ([]byte, int, error) {
				return []byte{}, 0, nil
			},
		},
		{
			name: "zero offset",
			setup: func() (buf editBuffer, p []byte, offset int64) {
				buf = stubBuf

				return buf, []byte("\x00\x00\x00\x00"), 0
			},

			expect: func() ([]byte, int, error) {
				return []byte("1234"), 4, nil
			},
		},
		{
			name: "non-zero offset",
			setup: func() (buf editBuffer, p []byte, offset int64) {
				buf = stubBuf

				return buf, []byte("\x00\x00\x00"), 1
			},

			expect: func() ([]byte, int, error) {
				return []byte("234"), 3, nil
			},
		},
		{
			name: "non-zero gap start",
			setup: func() (buf editBuffer, p []byte, offset int64) {
				buf = stubBuf
				buf.gapstart = 4

				return buf, []byte("\x00\x00\x00"), 1
			},

			expect: func() ([]byte, int, error) {
				return []byte("234"), 3, nil
			},
		},
		{
			name: "offset greater than len",
			setup: func() (buf editBuffer, p []byte, offset int64) {
				buf = stubBuf

				return buf, []byte("\x00"), 5
			},

			expect: func() ([]byte, int, error) {
				return []byte("\x00"), 0, io.EOF
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(tt *testing.T) {
			buf, set1, set2 := test.setup()
			exp1, exp2, exp3 := test.expect()

			ret1, ret2 := buf.ReadAt(set1, set2)

			if bytes.Compare(set1, exp1) != 0 {
				tt.Errorf("buffer: %s %s", exp1, set1)
			}

			if exp2 != ret1 {
				tt.Errorf("count: %d %d", exp2, ret1)
			}

			if !errors.Is(ret2, exp3) {
				tt.Errorf("error: %+v %+v", exp3, ret2)
			}
		})
	}
}
