package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed closing writer: %v", err)
	}
	os.Stdout = old

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed reading stdout: %v", err)
	}
	return string(out)
}

func TestMainPrintsExpectedSlice(t *testing.T) {
	out := captureStdout(t, main)
	if got := strings.TrimSpace(out); got != "[10 20]" {
		t.Fatalf("expected output [10 20], got %q", got)
	}
}
