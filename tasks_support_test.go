package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"
)

func TestBuildTaskProgressReportIncludesInactiveExtraAndWarning(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1)
	tasks := Task.TaskList{
		{Id: 1, Title: "done", Weight: 1, Progress: 100},
		{Id: 2, Title: "extra", Weight: 1, Progress: 50, Extra: true},
		{Id: 3, Title: "inactive", Weight: 1, Progress: 0, StartDate: &tomorrow},
	}

	report := BuildTaskProgressReport(tasks, true, errors.New("task warning"))
	if !strings.Contains(report, "Active + inactive tasks Progress:") {
		t.Fatalf("expected includeInactive scope in report, got: %s", report)
	}
	if !strings.Contains(report, "Inactive in list: 1") {
		t.Fatalf("expected one inactive task in report, got: %s", report)
	}
	if !strings.Contains(report, "[+1 extras]") {
		t.Fatalf("expected extras section in report, got: %s", report)
	}
	if !strings.Contains(report, "warning: task warning") {
		t.Fatalf("expected warning in report, got: %s", report)
	}
}

func TestBuildTaskProgressReportDefaultScope(t *testing.T) {
	tasks := Task.TaskList{{Id: 1, Title: "active", Weight: 1, Progress: 30}}
	report := BuildTaskProgressReport(tasks, false, nil)
	if !strings.Contains(report, "Active tasks Progress:") {
		t.Fatalf("expected default active scope in report, got: %s", report)
	}
	if strings.Contains(report, "warning:") {
		t.Fatalf("did not expect warning line without warning, got: %s", report)
	}
}

func TestBuildTogoProgressReportIncludesScopeExtrasAndWarning(t *testing.T) {
	togos := Togo.TogoList{
		{Id: 1, Title: "core", Weight: 1, Progress: 100},
		{Id: 2, Title: "extra", Weight: 1, Progress: 20, Extra: true},
	}

	report := BuildTogoProgressReport(togos, true, errors.New("togo warning"))
	if !strings.Contains(report, "Total Progress:") {
		t.Fatalf("expected all-days scope in report, got: %s", report)
	}
	if !strings.Contains(report, "[+1]") {
		t.Fatalf("expected extra togo count in report, got: %s", report)
	}
	if !strings.Contains(report, "warning: togo warning") {
		t.Fatalf("expected warning in report, got: %s", report)
	}
}

func TestBuildTaskPagesHandlesEmptyReminderMode(t *testing.T) {
	pages := BuildTaskPages(nil, false, true, 20)
	if len(pages) != 1 {
		t.Fatalf("expected single page for empty tasks list, got %d", len(pages))
	}
	if !strings.Contains(pages[0], "Task Reminder") {
		t.Fatalf("expected reminder-mode header, got: %s", pages[0])
	}
	if !strings.Contains(pages[0], "No tasks to show.") {
		t.Fatalf("expected empty-list text, got: %s", pages[0])
	}
}

func TestTaskInlineKeyboardMenuBuildsRowsAndCallbackData(t *testing.T) {
	tasks := Task.TaskList{
		{Id: 1, Title: "Completed", Progress: 100},
		{Id: 2, Title: strings.Repeat("long title ", 12), Progress: 10},
		{Id: 3, Title: "Third", Progress: 0},
		{Id: 4, Title: "Fourth", Progress: 0},
	}

	menu := TaskInlineKeyboardMenu(tasks, TickTask, true, 0)
	if menu == nil {
		t.Fatal("expected inline keyboard for non-empty task list")
	}
	if len(menu.InlineKeyboard) != 2 {
		t.Fatalf("expected 2 rows for 4 items and row size 3, got %d", len(menu.InlineKeyboard))
	}
	if len(menu.InlineKeyboard[0]) != 3 || len(menu.InlineKeyboard[1]) != 1 {
		t.Fatalf("unexpected row sizes: row0=%d row1=%d", len(menu.InlineKeyboard[0]), len(menu.InlineKeyboard[1]))
	}

	firstText := menu.InlineKeyboard[0][0].Text
	if !strings.HasPrefix(firstText, "✅ ") {
		t.Fatalf("expected completed-task prefix on first button, got: %q", firstText)
	}

	truncatedText := menu.InlineKeyboard[0][1].Text
	if !utf8.ValidString(truncatedText) {
		t.Fatalf("expected valid UTF-8 title after truncation, got: %q", truncatedText)
	}
	if !strings.HasSuffix(truncatedText, "...") {
		t.Fatalf("expected truncated title to end with ellipsis, got: %q", truncatedText)
	}

	cb := menu.InlineKeyboard[0][0].CallbackData
	if cb == nil {
		t.Fatal("expected callback data on first task button")
	}
	data := LoadCallbackData(*cb)
	if data.Action != TickTask {
		t.Fatalf("expected callback action TickTask, got %v", data.Action)
	}
	if data.ID != int64(tasks[0].Id) {
		t.Fatalf("expected callback id %d, got %d", tasks[0].Id, data.ID)
	}
	if !data.TaskIncludeInactive {
		t.Fatal("expected callback to carry includeInactive=true")
	}
}

func TestTaskInlineKeyboardMenuReturnsNilForEmptyList(t *testing.T) {
	if menu := TaskInlineKeyboardMenu(nil, TickTask, false, 0); menu != nil {
		t.Fatalf("expected nil keyboard for empty list, got: %+v", menu)
	}
}

func TestAllowedTaskReminderValuesTextMatchesAllowedValues(t *testing.T) {
	vals := Task.AllowedReminderTimes()
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		parts = append(parts, fmt.Sprint(v))
	}
	expected := strings.Join(parts, ", ")
	if got := allowedTaskReminderValuesText(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
