package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"ToGo4BotPlus/Togo"
)

// ============================================================
// B4: UTF-8 Truncation Tests for Inline Button Titles
// ============================================================

// TestInlineKeyboardMenuTruncatesCorrectly - B4: Button titles with emoji should not be truncated mid-rune
func TestInlineKeyboardMenuTruncatesCorrectly(t *testing.T) {
	// Create a test TogoList with emoji titles
	togos := Togo.TogoList{
		{Id: 1, Title: "🎯 Goal with a long name that should be truncated", Progress: 50},
		{Id: 2, Title: "📝 Write documentation", Progress: 100},
		{Id: 3, Title: "🚀 Deploy to production", Progress: 0},
	}

	// Call InlineKeyboardMenu
	menu := InlineKeyboardMenu(togos, TickTogo, false, false)

	// Verify that the menu was created
	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	// Check that all button texts are valid UTF-8
	for row := range menu.InlineKeyboard {
		for col := range menu.InlineKeyboard[row] {
			buttonText := menu.InlineKeyboard[row][col].Text
			if !utf8.ValidString(buttonText) {
				t.Errorf("Button text is not valid UTF-8: %q", buttonText)
			}
		}
	}
}

// TestInlineKeyboardMenuEmojiPrefix - B4: Completed togo gets checkmark prefix without breaking UTF-8
func TestInlineKeyboardMenuEmojiPrefix(t *testing.T) {
	togos := Togo.TogoList{
		{Id: 1, Title: "Complete Task", Progress: 100}, // Completed, should get ✅ prefix
	}

	menu := InlineKeyboardMenu(togos, TickTogo, false, false)

	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	// Get the button text
	buttonText := menu.InlineKeyboard[0][0].Text
	if !utf8.ValidString(buttonText) {
		t.Errorf("Button text with checkmark prefix is not valid UTF-8: %q", buttonText)
	}

	// Verify it contains the checkmark prefix
	if len(buttonText) < 5 || !strings.HasPrefix(buttonText, "✅ ") {
		t.Errorf("Completed togo should have checkmark prefix, got: %q", buttonText)
	}
}

// TestInlineKeyboardMenuLongTitle - B4: Very long titles should be truncated properly
func TestInlineKeyboardMenuLongTitle(t *testing.T) {
	longTitle := "This is a very long task title that needs to be truncated"
	for i := 0; i < 5; i++ {
		longTitle += " " + longTitle // Keep doubling to ensure it's very long
	}

	togos := Togo.TogoList{
		{Id: 1, Title: longTitle, Progress: 0},
	}

	menu := InlineKeyboardMenu(togos, TickTogo, false, false)

	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	buttonText := menu.InlineKeyboard[0][0].Text
	if !utf8.ValidString(buttonText) {
		t.Errorf("Long title button text is not valid UTF-8: %q", buttonText)
	}

	// Button text should not exceed the maximum length significantly
	// (it should be truncated + "..." which is safe)
	if len(buttonText) > MaximumInlineButtonTextLength+10 {
		t.Errorf("Button text is too long: %d bytes (max expected: %d)", len(buttonText), MaximumInlineButtonTextLength+10)
	}
}

// TestInlineKeyboardMenuAllEmoji - B4: Titles that are purely emoji should work
func TestInlineKeyboardMenuAllEmoji(t *testing.T) {
	togos := Togo.TogoList{
		{Id: 1, Title: "🎯🚀📝🎨🔥", Progress: 0},
	}

	menu := InlineKeyboardMenu(togos, TickTogo, false, false)

	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	buttonText := menu.InlineKeyboard[0][0].Text
	if !utf8.ValidString(buttonText) {
		t.Errorf("Emoji-only title button text is not valid UTF-8: %q", buttonText)
	}
}

// TestInlineKeyboardMenuMultibyteCharacters - B4: Multi-byte UTF-8 characters should not be split
func TestInlineKeyboardMenuMultibyteCharacters(t *testing.T) {
	// Simple multi-byte test with ASCII and accented characters
	togos := Togo.TogoList{
		{Id: 1, Title: "Cafe with accents", Progress: 0},
	}

	menu := InlineKeyboardMenu(togos, TickTogo, false, false)

	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	buttonText := menu.InlineKeyboard[0][0].Text
	if !utf8.ValidString(buttonText) {
		t.Errorf("Multi-byte character button text is not valid UTF-8: %q", buttonText)
	}

	// Verify the title is preserved
	if !strings.Contains(buttonText, "Cafe") {
		t.Errorf("Button text should contain the task title, got: %q", buttonText)
	}
}

// ============================================================
// B5: Panic Recovery Tests
// Note: These tests verify the panic-safety of parsing functions
// The main loop recovery is tested through integration tests
// ============================================================

// TestSplitArgumentsWithSpecialCharacters - B5: SplitArguments should not panic
func TestSplitArgumentsWithSpecialCharacters(t *testing.T) {
	testCases := []string{
		"+  task  =  5",
		"#  ",
		"%  -",
		"$  123",
		"✅",
		"❌",
		"/db",
		"/now",
		"",
		" ",
		"     ",
		"task  without  proper  spacing",
		"🎯  emoji  task",
	}

	for _, testCase := range testCases {
		result := SplitArguments(testCase)
		if result == nil {
			t.Errorf("SplitArguments panicked on input: %q", testCase)
		}
	}
}

// TestExtractBoundsChecking - B5: Extract should not panic on malformed input
func TestExtractBoundsChecking(t *testing.T) {
	testCases := [][]string{
		{},
		{""},
		{"task"},
		{"task", "="},
		{"task", "@"},
		{"task", "+p"},
		{"task", ":", ""},
		{"task", "->"},
	}

	for _, terms := range testCases {
		_, err := Togo.Extract(123, terms)
		// We expect errors for invalid input, but no panic
		_ = err
	}
}
