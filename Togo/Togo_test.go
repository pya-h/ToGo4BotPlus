package Togo

import (
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func withIsolatedDatabase(t *testing.T) {
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
		t.Fatalf("failed to initialize test database: %v", err)
	}
}

// ============================================================
// B1: Bounds Checking Tests for Extract & setFields
// ============================================================

// TestExtractEmptyTerms - B1: Extract should handle empty terms slice
func TestExtractEmptyTerms(t *testing.T) {
	ownerID := int64(123)
	emptyTerms := []string{}

	_, err := Extract(ownerID, emptyTerms)
	if err == nil {
		t.Error("Extract with empty terms should return an error")
	}
}

// TestExtractValidTitle - B1: Extract should work with valid single term
func TestExtractValidTitle(t *testing.T) {
	ownerID := int64(123)
	terms := []string{"My Task"}

	togo, err := Extract(ownerID, terms)
	if err != nil {
		t.Errorf("Extract with valid title should not error: %v", err)
	}
	if togo.Title != "My Task" {
		t.Errorf("Expected title 'My Task', got '%s'", togo.Title)
	}
}

// TestSetFieldsTrailingWeightFlag - B1: setFields should reject trailing weight flag
func TestSetFieldsTrailingWeightFlag(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "="}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with trailing '=' flag should return an error")
	}
}

// TestSetFieldsTrailingDescriptionFlag - B1: setFields should reject trailing description flag
func TestSetFieldsTrailingDescriptionFlag(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", ":"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with trailing ':' flag should return an error")
	}
}

// TestSetFieldsTrailingProgressFlag - B1: setFields should reject trailing progress flag
func TestSetFieldsTrailingProgressFlag(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "+p"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with trailing '+p' flag should return an error")
	}
}

// TestSetFieldsTrailingDateFlag - B1: setFields should reject trailing date flag
func TestSetFieldsTrailingDateFlag(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "@"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with trailing '@' flag should return an error")
	}
}

// TestSetFieldsDateFlagWithDayOnly - B1: setFields should reject date flag with only day
func TestSetFieldsDateFlagWithDayOnly(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "@", "1"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with '@' flag and only day (no time) should return an error")
	}
}

// TestSetFieldsTrailingDurationFlag - B1: setFields should reject trailing duration flag
func TestSetFieldsTrailingDurationFlag(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "->"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with trailing '->' flag should return an error")
	}
}

// TestSetFieldsValidWeight - B1: setFields should handle valid weight
func TestSetFieldsValidWeight(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "=", "5"}

	err := togo.setFields(terms)
	if err != nil {
		t.Errorf("setFields with valid weight should not error: %v", err)
	}
	if togo.Weight != 5 {
		t.Errorf("Expected weight 5, got %d", togo.Weight)
	}
}

// TestSetFieldsValidDescription - B1: setFields should handle valid description
func TestSetFieldsValidDescription(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", ":", "My description"}

	err := togo.setFields(terms)
	if err != nil {
		t.Errorf("setFields with valid description should not error: %v", err)
	}
	if togo.Description != "My description" {
		t.Errorf("Expected description 'My description', got '%s'", togo.Description)
	}
}

// TestSetFieldsValidProgress - B1: setFields should handle valid progress
func TestSetFieldsValidProgress(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "+p", "75"}

	err := togo.setFields(terms)
	if err != nil {
		t.Errorf("setFields with valid progress should not error: %v", err)
	}
	if togo.Progress != 75 {
		t.Errorf("Expected progress 75, got %d", togo.Progress)
	}
}

// TestSetFieldsValidDate - B1: setFields should handle valid date
func TestSetFieldsValidDate(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "@", "0", "10:30"}

	err := togo.setFields(terms)
	if err != nil {
		t.Errorf("setFields with valid date should not error: %v", err)
	}
	if togo.Date.Hour() != 10 || togo.Date.Minute() != 30 {
		t.Errorf("Expected time 10:30, got %d:%d", togo.Date.Hour(), togo.Date.Minute())
	}
}

// TestSetFieldsValidDuration - B1: setFields should handle valid duration
func TestSetFieldsValidDuration(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "->", "60"}

	err := togo.setFields(terms)
	if err != nil {
		t.Errorf("setFields with valid duration should not error: %v", err)
	}
	if togo.Duration.Minutes() != 60 {
		t.Errorf("Expected duration 60 minutes, got %.0f", togo.Duration.Minutes())
	}
}

// TestExtractChainedFlags - B1: Extract should handle multiple valid flags
func TestExtractChainedFlags(t *testing.T) {
	ownerID := int64(123)
	terms := []string{"My Task", "=", "5", ":", "Description", "+x"}

	togo, err := Extract(ownerID, terms)
	if err != nil {
		t.Errorf("Extract with chained valid flags should not error: %v", err)
	}
	if togo.Title != "My Task" || togo.Weight != 5 || togo.Description != "Description" || !togo.Extra {
		t.Errorf("Extract failed to parse chained flags correctly")
	}
}

// TestSetFieldsInvalidWeightValue - B1: setFields should reject non-numeric weight
func TestSetFieldsInvalidWeightValue(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "=", "not_a_number"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with non-numeric weight should return an error")
	}
}

// TestSetFieldsInvalidProgressValue - B1: setFields should reject non-numeric progress
func TestSetFieldsInvalidProgressValue(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "+p", "not_a_number"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with non-numeric progress should return an error")
	}
}

// TestSetFieldsProgressCapAt100 - B1: setFields should cap progress at 100
func TestSetFieldsProgressCapAt100(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "+p", "150"}

	err := togo.setFields(terms)
	if err != nil {
		t.Errorf("setFields with progress > 100 should not error, but cap it: %v", err)
	}
	if togo.Progress != 100 {
		t.Errorf("Expected progress capped at 100, got %d", togo.Progress)
	}
}

// TestSetFieldsInvalidHour - B1: setFields should reject invalid hour
func TestSetFieldsInvalidHour(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "@", "0", "25:00"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with invalid hour (25) should return an error")
	}
}

// TestSetFieldsInvalidMinute - B1: setFields should reject invalid minute
func TestSetFieldsInvalidMinute(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "@", "0", "10:75"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with invalid minute (75) should return an error")
	}
}

// TestSetFieldsInvalidDuration - B1: setFields should reject zero or negative duration
func TestSetFieldsInvalidDuration(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "->", "0"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with zero duration should return an error")
	}
}

// TestSetFieldsInvalidDurationNegative - B1: setFields should reject negative duration
func TestSetFieldsInvalidDurationNegative(t *testing.T) {
	togo := &Togo{Title: "Test Task"}
	terms := []string{"Test Task", "->", "-5"}

	err := togo.setFields(terms)
	if err == nil {
		t.Error("setFields with negative duration should return an error")
	}
}

// TestExtractTrailingFlag - B1: Extract should reject when last flag has no value
func TestExtractTrailingFlag(t *testing.T) {
	ownerID := int64(123)
	terms := []string{"My Task", "="}

	_, err := Extract(ownerID, terms)
	if err == nil {
		t.Error("Extract with trailing weight flag should return an error")
	}
}

// ============================================================
// B3: Command Token Consistency Tests
// ============================================================

// TestLoadWithJustTodayFlag - B3: Load should respect justToday parameter
func TestLoadWithJustTodayFlag(t *testing.T) {
	withIsolatedDatabase(t)

	ownerID := int64(999)
	today := Today()
	tomorrow := Date{today.AddDate(0, 0, 1)}

	if _, err := (&Togo{Title: "today", OwnerId: ownerID, Weight: 1, Date: today}).Save(); err != nil {
		t.Fatalf("failed to save today's togo: %v", err)
	}
	if _, err := (&Togo{Title: "tomorrow", OwnerId: ownerID, Weight: 1, Date: tomorrow}).Save(); err != nil {
		t.Fatalf("failed to save tomorrow's togo: %v", err)
	}

	togos, err := Load(ownerID, true, false)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(togos) != 1 {
		t.Fatalf("expected 1 togo for today, got %d", len(togos))
	}

	for _, togo := range togos {
		if togo.Date.Short() != today.Short() {
			t.Errorf("Load with justToday=true returned togo from %s, expected %s", togo.Date.Short(), today.Short())
		}
	}
}

// TestLoadWithAllDaysFlag - B3: Load should handle all days correctly
func TestLoadWithAllDaysFlag(t *testing.T) {
	withIsolatedDatabase(t)

	ownerID := int64(999)
	today := Today()
	tomorrow := Date{today.AddDate(0, 0, 1)}

	if _, err := (&Togo{Title: "today", OwnerId: ownerID, Weight: 1, Date: today}).Save(); err != nil {
		t.Fatalf("failed to save today's togo: %v", err)
	}
	if _, err := (&Togo{Title: "tomorrow", OwnerId: ownerID, Weight: 1, Date: tomorrow}).Save(); err != nil {
		t.Fatalf("failed to save tomorrow's togo: %v", err)
	}

	togos, err := Load(ownerID, false, false)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(togos) != 2 {
		t.Fatalf("expected 2 togos for all-days load, got %d", len(togos))
	}
}

// ============================================================
// B3: Inconsistent "all days" tokens - Testing Load behavior
// ============================================================

// TestLoadTodayOnly - B3: Load with justToday=true should filter to today
func TestLoadTodayOnly(t *testing.T) {
	withIsolatedDatabase(t)

	togos, err := Load(123, true, false)
	if err != nil {
		t.Fatalf("Load returned error on clean initialized database: %v", err)
	}
	if togos == nil {
		t.Fatal("Load returned nil slice on clean initialized database")
	}
	if len(togos) != 0 {
		t.Fatalf("expected zero togos on clean initialized database, got %d", len(togos))
	}
}

// TestLoadAllDays - B3: Load with justToday=false should return all days
func TestLoadAllDays(t *testing.T) {
	withIsolatedDatabase(t)

	togos, err := Load(123, false, false)
	if err != nil {
		t.Fatalf("Load returned error on clean initialized database: %v", err)
	}
	if togos == nil {
		t.Fatal("Load returned nil slice on clean initialized database")
	}
}

// TestLoadJustUndones - B3: Load with justUndones=true should filter incomplete
func TestLoadJustUndones(t *testing.T) {
	withIsolatedDatabase(t)

	ownerID := int64(123)
	today := Today()

	if _, err := (&Togo{Title: "done", OwnerId: ownerID, Weight: 1, Date: today, Progress: 100}).Save(); err != nil {
		t.Fatalf("failed to save completed togo: %v", err)
	}
	if _, err := (&Togo{Title: "incomplete", OwnerId: ownerID, Weight: 1, Date: today, Progress: 45}).Save(); err != nil {
		t.Fatalf("failed to save incomplete togo: %v", err)
	}

	togos, err := Load(ownerID, true, true)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(togos) != 1 {
		t.Fatalf("expected exactly one undone togo, got %d", len(togos))
	}
	if togos[0].Progress >= 100 {
		t.Fatalf("expected only incomplete togos, got progress=%d", togos[0].Progress)
	}
}

// ============================================================
// B4: UTF-8 Truncation in Inline Button Titles (main.go)
// These tests are in main_test.go as they test the UI layer
// ============================================================

// ============================================================
// B6: SQLite Table Creation and Concurrency Tests
// ============================================================

// TestSaveCreatesTable - B6: Save should create table if it doesn't exist
func TestSaveCreatesTable(t *testing.T) {
	withIsolatedDatabase(t)

	togo := &Togo{
		Title:       "Test Task",
		Description: "Test",
		Weight:      1,
		Progress:    0,
		Extra:       false,
		Date:        Today(),
		Duration:    0,
		OwnerId:     123,
	}

	id, err := togo.Save()
	if err != nil {
		t.Errorf("Save should succeed on first call: %v", err)
	}
	if id == 0 {
		t.Error("Save should return a valid ID")
	}

	loaded, err := Load(123, true, false)
	if err != nil {
		t.Fatalf("Load after Save returned error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 loaded togo after save, got %d", len(loaded))
	}
}

// TestLoadInitializesDatabaseFirst - B6: Load should work even if table doesn't exist yet
func TestLoadInitializesDatabaseFirst(t *testing.T) {
	withIsolatedDatabase(t)

	// Load from clean initialized database should succeed and return an empty list.
	togos, err := Load(456, true, false)
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}
	if togos == nil {
		t.Fatal("Load returned nil togos")
	}
	if len(togos) != 0 {
		t.Fatalf("expected no togos on clean initialized database, got %d", len(togos))
	}
}

func TestUpdatePropagatesSetFieldsError(t *testing.T) {
	togos := TogoList{
		{Id: 1, Title: "Task", OwnerId: 123, Weight: 1, Date: Today()},
	}

	_, err := togos.Update(123, []string{"1", "="})
	if err == nil {
		t.Fatal("expected setFields error to propagate from Update")
	}
}

func TestTogoUpdatePersistsChanges(t *testing.T) {
	withIsolatedDatabase(t)

	ownerID := int64(1001)
	initial := &Togo{
		Title:       "Task",
		Description: "old",
		Weight:      1,
		Progress:    5,
		Extra:       false,
		Date:        Date{Today().Add(1 * time.Hour)},
		Duration:    15 * time.Minute,
		OwnerId:     ownerID,
	}

	id, err := initial.Save()
	if err != nil {
		t.Fatalf("failed to save initial togo: %v", err)
	}

	updated := &Togo{
		Id:          id,
		Title:       initial.Title,
		Description: "new description",
		Weight:      4,
		Progress:    90,
		Extra:       true,
		Date:        Date{initial.Date.Add(2 * time.Hour)},
		Duration:    90 * time.Minute,
	}

	if err := updated.Update(ownerID); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	loaded, err := Load(ownerID, false, false)
	if err != nil {
		t.Fatalf("Load after update returned error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected exactly one togo after update, got %d", len(loaded))
	}

	got := loaded[0]
	if got.Description != "new description" {
		t.Fatalf("expected updated description, got %q", got.Description)
	}
	if got.Weight != 4 {
		t.Fatalf("expected updated weight 4, got %d", got.Weight)
	}
	if got.Progress != 90 {
		t.Fatalf("expected updated progress 90, got %d", got.Progress)
	}
	if !got.Extra {
		t.Fatal("expected updated extra flag to be true")
	}
	if got.Duration != 90*time.Minute {
		t.Fatalf("expected updated duration 90m, got %v", got.Duration)
	}
}

func TestTogoListUpdatePersistsChanges(t *testing.T) {
	withIsolatedDatabase(t)

	ownerID := int64(1002)
	seed := &Togo{Title: "Seed", Weight: 1, Progress: 0, Date: Date{Today().Add(30 * time.Minute)}, OwnerId: ownerID}
	id, err := seed.Save()
	if err != nil {
		t.Fatalf("failed to save seed togo: %v", err)
	}

	togos, err := Load(ownerID, false, false)
	if err != nil {
		t.Fatalf("failed to load seed togos: %v", err)
	}

	resp, err := togos.Update(ownerID, []string{fmt.Sprint(id), "=", "3", "+p", "80", ":", "updated from list"})
	if err != nil {
		t.Fatalf("TogoList.Update returned error: %v", err)
	}
	if !strings.Contains(resp, "Weight: 3") {
		t.Fatalf("expected response to include updated weight, got: %s", resp)
	}

	reloaded, err := Load(ownerID, false, false)
	if err != nil {
		t.Fatalf("failed to reload togos: %v", err)
	}
	if len(reloaded) != 1 {
		t.Fatalf("expected exactly one togo after list update, got %d", len(reloaded))
	}
	if reloaded[0].Weight != 3 || reloaded[0].Progress != 80 || reloaded[0].Description != "updated from list" {
		t.Fatalf("unexpected updated togo values: %+v", reloaded[0])
	}
}

func TestTogoAndListStringRepresentations(t *testing.T) {
	fixed := Date{time.Date(2026, time.January, 2, 8, 30, 0, 0, time.UTC)}
	togo := Togo{
		Id:          7,
		Title:       "Read",
		Description: "Book",
		Weight:      2,
		Extra:       true,
		Progress:    40,
		Date:        fixed,
		Duration:    25 * time.Minute,
	}

	text := togo.ToString()
	if !strings.Contains(text, "Togo #7) Read") {
		t.Fatalf("ToString missing header, got: %s", text)
	}
	if !strings.Contains(text, "Weight: 2") || !strings.Contains(text, "Extra: true") || !strings.Contains(text, "Progress: 40") {
		t.Fatalf("ToString missing expected fields, got: %s", text)
	}

	list := TogoList{togo}
	all := list.ToString()
	if len(all) != 1 {
		t.Fatalf("expected one rendered entry, got %d", len(all))
	}
	if all[0] != text {
		t.Fatalf("expected list rendering to match item rendering\nlist: %q\nitem: %q", all[0], text)
	}
}

func TestTogoListProgressMadeIncludesExtraStats(t *testing.T) {
	togos := TogoList{
		{Title: "A", Weight: 2, Progress: 50, Extra: false},
		{Title: "B", Weight: 1, Progress: 100, Extra: false},
		{Title: "C", Weight: 5, Progress: 20, Extra: true},
	}

	progress, completedPct, completed, extra, total := togos.ProgressMade()

	if math.Abs(progress-100.0) > 0.0001 {
		t.Fatalf("expected weighted progress 100.0, got %.4f", progress)
	}
	if math.Abs(completedPct-33.3333) > 0.05 {
		t.Fatalf("expected completed percentage around 33.33, got %.4f", completedPct)
	}
	if completed != 1 {
		t.Fatalf("expected completed count 1, got %d", completed)
	}
	if extra != 1 {
		t.Fatalf("expected extra count 1, got %d", extra)
	}
	if total != 2 {
		t.Fatalf("expected non-extra total 2, got %d", total)
	}
}

func TestTogoListGetAndRemoveIndex(t *testing.T) {
	togos := TogoList{
		{Id: 10, Title: "A"},
		{Id: 20, Title: "B"},
	}

	got, err := togos.Get(20)
	if err != nil {
		t.Fatalf("expected to find id 20, got error: %v", err)
	}
	if got.Title != "B" {
		t.Fatalf("expected title B, got %q", got.Title)
	}

	if _, err := togos.Get(99); err == nil {
		t.Fatal("expected error for unknown togo id")
	}

	remaining := togos.RemoveIndex(0)
	if len(remaining) != 1 || remaining[0].Id != 20 {
		t.Fatalf("unexpected remove index result: %+v", remaining)
	}
}

func TestTogoListRemoveDeletesFromDatabase(t *testing.T) {
	withIsolatedDatabase(t)

	ownerID := int64(2001)
	first := &Togo{Title: "first", Weight: 1, Date: Date{Today().Add(1 * time.Hour)}, OwnerId: ownerID}
	second := &Togo{Title: "second", Weight: 1, Date: Date{Today().Add(2 * time.Hour)}, OwnerId: ownerID}

	firstID, err := first.Save()
	if err != nil {
		t.Fatalf("failed to save first togo: %v", err)
	}
	secondID, err := second.Save()
	if err != nil {
		t.Fatalf("failed to save second togo: %v", err)
	}

	togos, err := Load(ownerID, false, false)
	if err != nil {
		t.Fatalf("failed to load togos: %v", err)
	}

	remaining, err := togos.Remove(ownerID, firstID)
	if err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Id != secondID {
		t.Fatalf("unexpected remaining togos after remove: %+v", remaining)
	}

	if _, err := remaining.Remove(ownerID, 99999); err == nil {
		t.Fatal("expected error when removing non-existent togo")
	}

	reloaded, err := Load(ownerID, false, false)
	if err != nil {
		t.Fatalf("failed to reload togos after remove: %v", err)
	}
	if len(reloaded) != 1 || reloaded[0].Id != secondID {
		t.Fatalf("database does not match expected state after remove: %+v", reloaded)
	}
}

func TestLoadEverybodysTodayFiltersByDateWindow(t *testing.T) {
	withIsolatedDatabase(t)

	withinWindow := Date{Today().Add(30 * time.Minute)}
	outsideWindow := Date{withinWindow.AddDate(0, 0, 2)}

	idA, err := (&Togo{Title: "a", OwnerId: 1, Weight: 1, Date: withinWindow}).Save()
	if err != nil {
		t.Fatalf("failed to save togo a: %v", err)
	}
	idB, err := (&Togo{Title: "b", OwnerId: 2, Weight: 1, Date: withinWindow}).Save()
	if err != nil {
		t.Fatalf("failed to save togo b: %v", err)
	}
	idOutside, err := (&Togo{Title: "future", OwnerId: 1, Weight: 1, Date: outsideWindow}).Save()
	if err != nil {
		t.Fatalf("failed to save future togo: %v", err)
	}

	togos, warning := LoadEverybodysToday()
	if warning != nil {
		t.Fatalf("expected no warning for valid rows, got: %v", warning)
	}
	if len(togos) != 2 {
		t.Fatalf("expected exactly two togos in today's window, got %d", len(togos))
	}

	seen := map[uint64]bool{}
	for _, togo := range togos {
		seen[togo.Id] = true
	}
	if !seen[idA] || !seen[idB] {
		t.Fatalf("expected both today-window togos, got ids: %+v", seen)
	}
	if seen[idOutside] {
		t.Fatalf("did not expect outside-window togo id %d in results", idOutside)
	}
}
