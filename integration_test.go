package main

import (
	"testing"

	"ToGo4BotPlus/Togo"
)

// ============================================================
// Integration Tests for B1: Bounds Checking in Message Handler
// These tests simulate actual Telegram message flow through main.go
// to ensure trailing flags don't cause panics
// ============================================================

// TestHandlerPanicsOnTrailingWeightFlag - B1: + task = should not panic
func TestHandlerPanicsOnTrailingWeightFlag(t *testing.T) {
	// Simulate user typing: "+ task =" (with double spaces as separator)
	// This should return error, not panic
	input := "+  task  ="
	terms := SplitArguments(input)

	if len(terms) != 3 || terms[0] != "+" {
		t.Fatalf("SplitArguments parsed unexpected terms: %#v", terms)
	}

	// Simulate the main.go handler logic for + command
	if terms[0] == "+" && len(terms) > 1 {
		// This should NOT panic even though terms[2] is "=" with no value after
		togo, err := Togo.Extract(int64(123), terms[1:])
		if err == nil {
			t.Errorf("Extract should return error for trailing weight flag, got nil")
		}
		// If we get here without panic, test passes
		_ = togo
	} else {
		t.Fatalf("Failed to parse input correctly")
	}
}

// TestHandlerPanicsOnTrailingDescriptionFlag - B1: + task : should not panic
func TestHandlerPanicsOnTrailingDescriptionFlag(t *testing.T) {
	input := "+  task  :"
	terms := SplitArguments(input)
	if len(terms) != 3 || terms[0] != "+" {
		t.Fatalf("SplitArguments parsed unexpected terms: %#v", terms)
	}

	if terms[0] == "+" && len(terms) > 1 {
		togo, err := Togo.Extract(int64(123), terms[1:])
		if err == nil {
			t.Errorf("Extract should return error for trailing description flag, got nil")
		}
		_ = togo
	}
}

// TestHandlerPanicsOnTrailingProgressFlag - B1: + task +p should not panic
func TestHandlerPanicsOnTrailingProgressFlag(t *testing.T) {
	input := "+  task  +p"
	terms := SplitArguments(input)
	if len(terms) != 3 || terms[0] != "+" {
		t.Fatalf("SplitArguments parsed unexpected terms: %#v", terms)
	}

	if terms[0] == "+" && len(terms) > 1 {
		togo, err := Togo.Extract(int64(123), terms[1:])
		if err == nil {
			t.Errorf("Extract should return error for trailing progress flag, got nil")
		}
		_ = togo
	}
}

// TestHandlerPanicsOnTrailingDateFlag - B1: + task @ should not panic
func TestHandlerPanicsOnTrailingDateFlag(t *testing.T) {
	input := "+  task  @"
	terms := SplitArguments(input)
	if len(terms) != 3 || terms[0] != "+" {
		t.Fatalf("SplitArguments parsed unexpected terms: %#v", terms)
	}

	if terms[0] == "+" && len(terms) > 1 {
		togo, err := Togo.Extract(int64(123), terms[1:])
		if err == nil {
			t.Errorf("Extract should return error for trailing date flag, got nil")
		}
		_ = togo
	}
}

// TestHandlerPanicsOnDateFlagWithDayOnly - B1: + task @ 1 (no time) should not panic
func TestHandlerPanicsOnDateFlagWithDayOnly(t *testing.T) {
	input := "+  task  @  1"
	terms := SplitArguments(input)
	if len(terms) != 4 || terms[0] != "+" {
		t.Fatalf("SplitArguments parsed unexpected terms: %#v", terms)
	}

	if terms[0] == "+" && len(terms) > 1 {
		togo, err := Togo.Extract(int64(123), terms[1:])
		if err == nil {
			t.Errorf("Extract should return error for incomplete date (day without time), got nil")
		}
		_ = togo
	}
}

// TestHandlerPanicsOnTrailingDurationFlag - B1: + task -> should not panic
func TestHandlerPanicsOnTrailingDurationFlag(t *testing.T) {
	input := "+  task  ->"
	terms := SplitArguments(input)
	if len(terms) != 3 || terms[0] != "+" {
		t.Fatalf("SplitArguments parsed unexpected terms: %#v", terms)
	}

	if terms[0] == "+" && len(terms) > 1 {
		togo, err := Togo.Extract(int64(123), terms[1:])
		if err == nil {
			t.Errorf("Extract should return error for trailing duration flag, got nil")
		}
		_ = togo
	}
}

// TestHandlerPanicsOnCommandAfterPlus - B1: + # should not panic
func TestHandlerPanicsOnCommandAfterPlus(t *testing.T) {
	input := "+  #"
	terms := SplitArguments(input)

	if len(terms) != 2 || terms[0] != "+" {
		t.Fatalf("SplitArguments parsed unexpected terms: %#v", terms)
	}

	if terms[0] == "+" && len(terms) > 1 {
		// Extract gets [#]; it should not panic.
		togo, err := Togo.Extract(int64(123), terms[1:])
		if err != nil {
			t.Fatalf("unexpected error for command-like title: %v", err)
		}
		if togo.Title != "#" {
			t.Fatalf("expected title '#', got %q", togo.Title)
		}
		_ = togo
	}
}

// TestHandlerReturnsErrorNotPanic - B1: Invalid input returns error not panic
func TestHandlerReturnsErrorNotPanic(t *testing.T) {
	testCases := []struct {
		input       string
		expectTerms int
		expectErr   bool
	}{
		{input: "+", expectTerms: 1, expectErr: false},
		{input: "+  task  =", expectTerms: 3, expectErr: true},
		{input: "+  task  :", expectTerms: 3, expectErr: true},
		{input: "+  task  +p", expectTerms: 3, expectErr: true},
		{input: "+  task  @", expectTerms: 3, expectErr: true},
		{input: "+  task  ->", expectTerms: 3, expectErr: true},
	}

	for _, tc := range testCases {
		terms := SplitArguments(tc.input)
		if len(terms) != tc.expectTerms {
			t.Fatalf("SplitArguments(%q) = %#v, expected %d terms", tc.input, terms, tc.expectTerms)
		}
		if terms[0] != "+" {
			t.Fatalf("expected first token '+' for input %q, got %#v", tc.input, terms)
		}

		if len(terms) == 1 {
			continue
		}

		togo, err := Togo.Extract(int64(123), terms[1:])
		if tc.expectErr && err == nil {
			t.Fatalf("expected error for input %q, got togo=%+v", tc.input, togo)
		}
		if !tc.expectErr && err != nil {
			t.Fatalf("unexpected error for input %q: %v", tc.input, err)
		}
	}
}

// TestSplitArgumentsIntegration - B1: Verify SplitArguments works correctly
// NOTE: SplitArguments requires EXACTLY 2 spaces as separator, not 1 or 3+
func TestSplitArgumentsIntegration(t *testing.T) {
	testCases := []struct {
		input    string
		expected int
		desc     string
	}{
		{"+  task  =  5", 4, "trailing weight flag with double spaces"},
		{"+  task  :  desc", 4, "trailing description flag with double spaces"},
		{"+  task  +p  50", 4, "trailing progress flag with double spaces"},
		{"+  task  @  1  10:00", 5, "date flag with day and time"},
		{"+  task  ->  60", 4, "trailing duration flag with double spaces"},
		{"+  #", 2, "plus followed by command"},
		{"+", 1, "plus alone"},
	}

	for _, tc := range testCases {
		terms := SplitArguments(tc.input)
		if len(terms) != tc.expected {
			t.Errorf("SplitArguments(%q) = %d terms, expected %d (%s)",
				tc.input, len(terms), tc.expected, tc.desc)
		}
	}
}

// TestExtractRejectsAllTrailingFlags - B1: Extract should validate all trailing flags
func TestExtractRejectsAllTrailingFlags(t *testing.T) {
	testCases := []struct {
		terms []string
		desc  string
	}{
		{[]string{"task", "="}, "trailing weight flag"},
		{[]string{"task", ":"}, "trailing description flag"},
		{[]string{"task", "+p"}, "trailing progress flag"},
		{[]string{"task", "@"}, "trailing date flag"},
		{[]string{"task", "->"}, "trailing duration flag"},
	}

	for _, tc := range testCases {
		_, err := Togo.Extract(int64(123), tc.terms)
		if err == nil {
			t.Errorf("Extract should reject %s", tc.desc)
		}
	}
}

// TestExtractAcceptsValidInput - B1: Extract should accept valid input
func TestExtractAcceptsValidInput(t *testing.T) {
	validInputs := []struct {
		terms []string
		desc  string
	}{
		{[]string{"My Task"}, "simple task"},
		{[]string{"My Task", "=", "5"}, "task with weight"},
		{[]string{"My Task", ":", "description"}, "task with description"},
		{[]string{"My Task", "+p", "75"}, "task with progress"},
		{[]string{"My Task", "@", "0", "10:30"}, "task with date"},
		{[]string{"My Task", "->", "60"}, "task with duration"},
		{[]string{"My Task", "=", "5", ":", "desc", "+x"}, "task with multiple flags"},
	}

	for _, tc := range validInputs {
		togo, err := Togo.Extract(int64(123), tc.terms)
		if err != nil {
			t.Errorf("Extract should accept %s, got error: %v", tc.desc, err)
		}
		if togo.Title == "" {
			t.Errorf("Extract produced empty title for %s", tc.desc)
		}
	}
}
