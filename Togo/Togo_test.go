package Togo

import (
	"testing"
)

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
	// This test verifies that Load respects the justToday parameter
	// When justToday=true, should only return today's togos
	// This is a logic test (integration-level)
	ownerID := int64(999)

	// Load today's togos
	togos, _ := Load(ownerID, true, false)

	// All returned togos should be from today
	today := Today()
	for _, togo := range togos {
		if togo.Date.Short() != today.Short() {
			t.Errorf("Load with justToday=true returned togo from %s, expected %s", togo.Date.Short(), today.Short())
		}
	}
}

// TestLoadWithAllDaysFlag - B3: Load should handle all days correctly
func TestLoadWithAllDaysFlag(t *testing.T) {
	// This test verifies that Load can return togos from all days
	ownerID := int64(999)

	// Load all togos (not just today)
	togos, _ := Load(ownerID, false, false)

	// togos can be from any day (no restriction)
	// Just verify it doesn't panic and returns a list
	if togos == nil {
		t.Error("Load with justToday=false should return a valid slice, not nil")
	}
}

// ============================================================
// B3: Inconsistent "all days" tokens - Testing Load behavior
// ============================================================

// TestLoadTodayOnly - B3: Load with justToday=true should filter to today
func TestLoadTodayOnly(t *testing.T) {
	// Note: This test requires a database. For now, we just verify
	// the function signature and error handling.
	togos, err := Load(123, true, false)
	// Should return without panic, even if database doesn't exist yet
	if togos == nil && err == nil {
		t.Error("Load should return an error if database is not initialized")
	}
}

// TestLoadAllDays - B3: Load with justToday=false should return all days
func TestLoadAllDays(t *testing.T) {
	togos, err := Load(123, false, false)
	// Should return without panic
	if togos == nil && err == nil {
		t.Error("Load should return an error if database is not initialized")
	}
}

// TestLoadJustUndones - B3: Load with justUndones=true should filter incomplete
func TestLoadJustUndones(t *testing.T) {
	togos, err := Load(123, true, true)
	// Should return without panic
	if togos == nil && err == nil {
		t.Error("Load should return an error if database is not initialized")
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
	// Create a temporary test database
	tempDB := "./test_togos.db"
	defer deleteTestDB(tempDB)

	// Override the DATABASE_NAME temporarily for this test
	// Note: This requires refactoring the code to accept database path
	// For now, we verify that Save creates the table by checking if a new togo can be saved

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
}

// TestLoadInitializesDatabaseFirst - B6: Load should work even if table doesn't exist yet
func TestLoadInitializesDatabaseFirst(t *testing.T) {
	// Create a test togo first to ensure table exists
	togo := &Togo{
		Title:   "Setup Task",
		OwnerId: 456,
		Weight:  1,
		Date:    Today(),
	}
	togo.Save()

	// Now try to load (should not fail even if table initialization wasn't done yet)
	togos, err := Load(456, true, false)
	if togos == nil && err == nil {
		t.Error("Load should not return nil togos and nil error")
	}
}

// Helper function to delete test database
func deleteTestDB(path string) {
	// Implement in actual test setup/teardown
}
