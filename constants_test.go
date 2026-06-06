package main

import (
	"strings"
	"testing"
)

func TestHelpMessageFormatAndSections(t *testing.T) {
	if HELP_MESSAGE == "" {
		t.Fatal("HELP_MESSAGE should not be empty")
	}
	if !strings.HasPrefix(HELP_MESSAGE, "WTF?\n```") {
		t.Fatalf("HELP_MESSAGE should start with markdown fence, got prefix: %q", HELP_MESSAGE[:min(16, len(HELP_MESSAGE))])
	}
	if !strings.HasSuffix(HELP_MESSAGE, "\n```") {
		t.Fatal("HELP_MESSAGE should end with markdown fence")
	}

	requiredSections := []string{
		"## Commands",
		"## +: New Togo:",
		"## #: Show Togos",
		"## %: Progress Made:",
		"## $: Get / Update a togo",
		"## Tasks (separate from togos):",
	}

	for _, section := range requiredSections {
		if !strings.Contains(HELP_MESSAGE, section) {
			t.Fatalf("HELP_MESSAGE missing section: %q", section)
		}
	}
}

func TestHelpMessageIncludesTaskCommandTokens(t *testing.T) {
	requiredSnippets := []string{
		"=>     " + TaskAddCommand + "     title",
		"=>     " + TaskListCommand + "     [...]",
		"=>     " + TaskUpdateCommand + "     id",
		"=>     " + TaskTickCommand,
		"=>     " + TaskRemoveCommand,
		"=>     " + TaskSettingsCommand,
		"=>     %     " + TaskStatsToken,
		"=>     %     " + TaskBothStatsToken,
		TaskIncludeInactiveToken,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(HELP_MESSAGE, snippet) {
			t.Fatalf("HELP_MESSAGE missing task token snippet: %q", snippet)
		}
	}
}

func TestCommandConstantsAreUniqueAndNonEmpty(t *testing.T) {
	tokens := []string{
		TaskAddCommand,
		TaskListCommand,
		TaskUpdateCommand,
		TaskTickCommand,
		TaskRemoveCommand,
		TaskSettingsCommand,
	}

	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if strings.TrimSpace(token) == "" {
			t.Fatal("command token must not be empty or whitespace")
		}
		if _, exists := seen[token]; exists {
			t.Fatalf("duplicate command token found: %q", token)
		}
		seen[token] = struct{}{}
	}
}

func TestCoreNumericConstantsStayWithinExpectedRanges(t *testing.T) {
	if NumberOfSeparatorSpaces != 2 {
		t.Fatalf("NumberOfSeparatorSpaces must stay 2 for parser behavior, got %d", NumberOfSeparatorSpaces)
	}
	if MaximumNumberOfRowItems < 1 {
		t.Fatalf("MaximumNumberOfRowItems must be positive, got %d", MaximumNumberOfRowItems)
	}
	if MaximumInlineButtonTextLength < 8 {
		t.Fatalf("MaximumInlineButtonTextLength unexpectedly small: %d", MaximumInlineButtonTextLength)
	}
	if MaximumTaskMessageLength <= 0 || MaximumTaskMessageLength > 4096 {
		t.Fatalf("MaximumTaskMessageLength must be in (0, 4096], got %d", MaximumTaskMessageLength)
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
