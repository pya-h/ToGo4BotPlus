package Task

import (
	"strings"
	"testing"
	"time"
)

func TestTaskToStringCoversStatusVariants(t *testing.T) {
	now := time.Date(2026, time.June, 6, 12, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -1)
	future := now.AddDate(0, 0, 1)

	active := Task{Id: 1, Title: "Active", Weight: 1, Progress: 20, StartDate: &past}
	done := Task{Id: 2, Title: "Done", Weight: 1, Progress: 100}
	inactive := Task{Id: 3, Title: "Inactive", Weight: 1, Progress: 30, StartDate: &future}

	if text := active.ToString(now); !strings.Contains(text, "Status: active") {
		t.Fatalf("expected active status in text, got: %s", text)
	}
	if text := done.ToString(now); !strings.Contains(text, "Status: done") {
		t.Fatalf("expected done status in text, got: %s", text)
	}
	if text := inactive.ToString(now); !strings.Contains(text, "Status: inactive") {
		t.Fatalf("expected inactive status in text, got: %s", text)
	}
}

func TestTaskListToStringReturnsOneEntryPerTask(t *testing.T) {
	now := time.Date(2026, time.June, 6, 12, 0, 0, 0, time.UTC)
	tasks := TaskList{
		{Id: 11, Title: "A", Weight: 1, Progress: 10},
		{Id: 12, Title: "B", Weight: 1, Progress: 20},
	}

	lines := tasks.ToString(now)
	if len(lines) != len(tasks) {
		t.Fatalf("expected %d rendered lines, got %d", len(tasks), len(lines))
	}
	if !strings.Contains(lines[0], "Task #11)") || !strings.Contains(lines[1], "Task #12)") {
		t.Fatalf("unexpected rendered tasks output: %#v", lines)
	}
}
