package main

import (
	"testing"
)

type expval struct {
	in, out string
}

func TestAppID(t *testing.T) {
	t.Parallel()

	tests := []expval{
		{"example", "localhost.example"},
		{"example.com", "com.example"},
		{"www.example.com", "com.example.www"},
		{"examplecom/app", "examplecom.app"},
		{"example.com/app", "com.example.app"},
		{"www.example.com/app", "com.example.www.app"},
		{"www.en.example.com/app", "com.example.en.www.app"},
		{"example.com/dir/app", "com.example.app"},
		{"example.com/dir.ext/app", "com.example.app"},
		{"example.com/dir/app.ext", "com.example.app.ext"},
		{"example-com.net/dir/app", "net.example_com.app"},
	}

	for i, test := range tests {
		got := appIDFromPackage(test.in)
		if exp := test.out; got != exp {
			t.Errorf("(%d): expected '%s', got '%s'", i, exp, got)
		}
	}
}
