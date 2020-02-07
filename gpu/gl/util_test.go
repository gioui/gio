package gl

import (
	"testing"
)

func TestGoString(t *testing.T) {
	tests := [][2]string{
		{"Hello\x00", "Hello"},
		{"\x00", ""},
	}
	for _, test := range tests {
		got := GoString([]byte(test[0]))
		if exp := test[1]; exp != got {
			t.Errorf("expected %q got %q", exp, got)
		}
	}
}
