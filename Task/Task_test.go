package Task

import (
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

func withIsolatedTaskDatabase(t *testing.T) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if err := InitDatabase(); err != nil {
		t.Fatalf("failed to initialize task database: %v", err)
	}
}

func TestExtractTaskEmptyTerms(t *testing.T) {
	if _, err := Extract(1, []string{}); err == nil {
		t.Fatal("expected error for empty task terms")
	}
}

func TestExtractTaskWithDefaults(t *testing.T) {
	task, err := Extract(42, []string{"Task title"})
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}
	if task.OwnerId != 42 {
		t.Fatalf("expected owner id 42, got %d", task.OwnerId)
	}
	if task.Title != "Task title" {
		t.Fatalf("expected title Task title, got %q", task.Title)
	}
	if task.Weight != 1 {
		t.Fatalf("expected default weight 1, got %d", task.Weight)
	}
}

func TestTaskSetFieldsUnsupportedDuration(t *testing.T) {
	task := &Task{Title: "X"}
	err := task.setFields([]string{"X", "->", "30"})
	if err == nil {
		t.Fatal("expected duration flag to be rejected for tasks")
	}
}

func TestTaskSetFieldsStartDateByDelta(t *testing.T) {
	task := &Task{Title: "X"}
	if err := task.setFields([]string{"X", "@", "2"}); err != nil {
		t.Fatalf("unexpected start date parse error: %v", err)
	}
	if task.StartDate == nil {
		t.Fatal("expected start date to be set")
	}
	if task.IsActive(time.Now()) {
		t.Fatal("expected future start date task to be inactive")
	}
}

func TestTaskSaveAndLoadFilters(t *testing.T) {
	withIsolatedTaskDatabase(t)

	ownerID := int64(100)
	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)

	active := &Task{OwnerId: ownerID, Title: "active", Weight: 1, Progress: 20}
	inactive := &Task{OwnerId: ownerID, Title: "inactive", Weight: 1, Progress: 10, StartDate: &tomorrow}
	done := &Task{OwnerId: ownerID, Title: "done", Weight: 1, Progress: 100}

	if _, err := active.Save(); err != nil {
		t.Fatalf("failed to save active task: %v", err)
	}
	if _, err := inactive.Save(); err != nil {
		t.Fatalf("failed to save inactive task: %v", err)
	}
	if _, err := done.Save(); err != nil {
		t.Fatalf("failed to save done task: %v", err)
	}

	activeOnly, err := Load(ownerID, false, false)
	if err != nil {
		t.Fatalf("load active only returned error: %v", err)
	}
	if len(activeOnly) != 1 {
		t.Fatalf("expected 1 active incomplete task, got %d", len(activeOnly))
	}

	withInactive, err := Load(ownerID, true, false)
	if err != nil {
		t.Fatalf("load with inactive returned error: %v", err)
	}
	if len(withInactive) != 2 {
		t.Fatalf("expected 2 incomplete tasks with inactive included, got %d", len(withInactive))
	}

	all, err := Load(ownerID, true, true)
	if err != nil {
		t.Fatalf("load all tasks returned error: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 tasks total, got %d", len(all))
	}
}

func TestTaskUpdatePersistsChanges(t *testing.T) {
	withIsolatedTaskDatabase(t)

	ownerID := int64(101)
	task := &Task{OwnerId: ownerID, Title: "task", Weight: 1, Progress: 10}
	id, err := task.Save()
	if err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	task.Id = id
	task.Description = "updated"
	task.Weight = 5
	task.Progress = 90
	task.Extra = true
	if err := task.Update(ownerID); err != nil {
		t.Fatalf("update returned error: %v", err)
	}

	loaded, err := Load(ownerID, true, true)
	if err != nil {
		t.Fatalf("load after update returned error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected one task after update, got %d", len(loaded))
	}
	if loaded[0].Description != "updated" || loaded[0].Weight != 5 || loaded[0].Progress != 90 || !loaded[0].Extra {
		t.Fatalf("unexpected updated values: %+v", loaded[0])
	}
}

func TestTaskListUpdateAndGet(t *testing.T) {
	withIsolatedTaskDatabase(t)

	ownerID := int64(102)
	seed := &Task{OwnerId: ownerID, Title: "seed", Weight: 1}
	id, err := seed.Save()
	if err != nil {
		t.Fatalf("failed to save seed task: %v", err)
	}

	tasks, err := Load(ownerID, true, true)
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	resp, err := tasks.Update(ownerID, []string{fmt.Sprint(id), "=", "3", "+p", "80", ":", "desc"})
	if err != nil {
		t.Fatalf("TaskList.Update returned error: %v", err)
	}
	if resp == "" {
		t.Fatal("expected non-empty update response")
	}

	got, err := tasks.Get(id)
	if err != nil {
		t.Fatalf("expected Get to find task id %d: %v", id, err)
	}
	if got.Id != id {
		t.Fatalf("expected task id %d, got %d", id, got.Id)
	}
	if _, err := tasks.Get(99999); err == nil {
		t.Fatal("expected Get to fail for unknown id")
	}
}

func TestTaskRemove(t *testing.T) {
	withIsolatedTaskDatabase(t)

	ownerID := int64(103)
	a := &Task{OwnerId: ownerID, Title: "a", Weight: 1}
	b := &Task{OwnerId: ownerID, Title: "b", Weight: 1}

	idA, err := a.Save()
	if err != nil {
		t.Fatalf("failed to save a: %v", err)
	}
	idB, err := b.Save()
	if err != nil {
		t.Fatalf("failed to save b: %v", err)
	}

	tasks, err := Load(ownerID, true, true)
	if err != nil {
		t.Fatalf("load returned error: %v", err)
	}

	remaining, err := tasks.Remove(ownerID, idA)
	if err != nil {
		t.Fatalf("remove returned error: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Id != idB {
		t.Fatalf("unexpected remaining tasks after remove: %+v", remaining)
	}
}

func TestTaskProgressMade(t *testing.T) {
	tasks := TaskList{
		{Title: "A", Weight: 2, Progress: 50, Extra: false},
		{Title: "B", Weight: 1, Progress: 100, Extra: false},
		{Title: "C", Weight: 5, Progress: 20, Extra: true},
	}

	progress, completedPct, completed, extra, total := tasks.ProgressMade()
	if math.Abs(progress-100.0) > 0.0001 {
		t.Fatalf("expected weighted progress 100.0, got %.4f", progress)
	}
	if math.Abs(completedPct-33.3333) > 0.05 {
		t.Fatalf("expected completed percentage around 33.33, got %.4f", completedPct)
	}
	if completed != 1 || extra != 1 || total != 2 {
		t.Fatalf("unexpected aggregate counts completed=%d extra=%d total=%d", completed, extra, total)
	}
}

func TestReminderSettings(t *testing.T) {
	withIsolatedTaskDatabase(t)

	ownerID := int64(500)
	setting, err := GetReminderSetting(ownerID)
	if err != nil {
		t.Fatalf("GetReminderSetting returned error: %v", err)
	}
	if setting.RemindersPerDay != 4 {
		t.Fatalf("expected default reminders/day 4, got %d", setting.RemindersPerDay)
	}

	if err := SetReminderTimes(ownerID, 6); err != nil {
		t.Fatalf("SetReminderTimes returned error: %v", err)
	}
	updated, err := GetReminderSetting(ownerID)
	if err != nil {
		t.Fatalf("GetReminderSetting after update returned error: %v", err)
	}
	if updated.RemindersPerDay != 6 {
		t.Fatalf("expected reminders/day 6, got %d", updated.RemindersPerDay)
	}

	if err := SetReminderTimes(ownerID, 5); err == nil {
		t.Fatal("expected SetReminderTimes to reject unsupported value 5")
	}

	if err := UpdateLastReminderSlot(ownerID, "2026-06-06-06"); err != nil {
		t.Fatalf("UpdateLastReminderSlot returned error: %v", err)
	}
	afterSlot, err := GetReminderSetting(ownerID)
	if err != nil {
		t.Fatalf("GetReminderSetting after slot update returned error: %v", err)
	}
	if afterSlot.LastReminderSlot != "2026-06-06-06" {
		t.Fatalf("expected updated last reminder slot, got %q", afterSlot.LastReminderSlot)
	}
}

func TestLoadActiveOwners(t *testing.T) {
	withIsolatedTaskDatabase(t)

	now := time.Now()
	tomorrow := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)

	if _, err := (&Task{OwnerId: 1, Title: "active", Weight: 1, Progress: 10}).Save(); err != nil {
		t.Fatalf("failed to save active task: %v", err)
	}
	if _, err := (&Task{OwnerId: 2, Title: "inactive", Weight: 1, Progress: 10, StartDate: &tomorrow}).Save(); err != nil {
		t.Fatalf("failed to save inactive task: %v", err)
	}
	if _, err := (&Task{OwnerId: 3, Title: "done", Weight: 1, Progress: 100}).Save(); err != nil {
		t.Fatalf("failed to save done task: %v", err)
	}

	owners, err := LoadActiveOwners(time.Now())
	if err != nil {
		t.Fatalf("LoadActiveOwners returned error: %v", err)
	}
	if len(owners) != 1 || owners[0] != 1 {
		t.Fatalf("expected only owner 1 to be active, got %+v", owners)
	}
}
