package tests

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed creating stdout pipe: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed closing stdout writer: %v", err)
	}
	os.Stdout = old

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed reading stdout capture: %v", err)
	}
	return string(data)
}

func TestDateFormattingAndConversions(t *testing.T) {
	base := Date{Time: time.Date(2026, time.June, 6, 14, 5, 0, 0, time.UTC)}

	if got := base.Get(); !strings.Contains(got, "2026-6-6") {
		t.Fatalf("expected date in Get output, got %q", got)
	}
	if got := base.Short(); got != "2026-6-6" {
		t.Fatalf("expected short date 2026-6-6, got %q", got)
	}

	loc1 := base.ToLocal()
	loc2 := base.ToLocal2()
	if loc1.Location() == nil || loc2.Location() == nil {
		t.Fatal("expected local conversion to retain a location")
	}
	if loc2.Year() != base.Year() || loc2.Month() != base.Month() || loc2.Day() != base.Day() {
		t.Fatalf("expected ToLocal2 to keep date components, got %v from %v", loc2, base)
	}
}

func TestNowAndTodayReturnValues(t *testing.T) {
	now := Now()
	if now == nil {
		t.Fatal("Now returned nil")
	}
	today := Today()
	if today.Year() == 0 {
		t.Fatalf("unexpected year from Today: %d", today.Year())
	}
}

func TestMainProducesOutput(t *testing.T) {
	out := captureStdout(t, main)
	if strings.TrimSpace(out) == "" {
		t.Fatal("expected main to print output")
	}
}
