package Idea

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func withIsolatedIdeaDatabase(t *testing.T) {
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
		t.Fatalf("failed to initialize idea database: %v", err)
	}
}

func TestExtractIdeaEmptyTerms(t *testing.T) {
	if _, err := Extract(1, []string{}); err == nil {
		t.Fatal("expected error for empty idea terms")
	}
}

func TestExtractIdeaWithDefaultsAndFlags(t *testing.T) {
	idea, err := Extract(42, []string{"Build a rocket", "+!", "+c", "Engineering"})
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}
	if idea.OwnerId != 42 {
		t.Fatalf("expected owner id 42, got %d", idea.OwnerId)
	}
	if idea.Text != "Build a rocket" {
		t.Fatalf("expected text, got %q", idea.Text)
	}
	if !idea.IsHighPriority {
		t.Fatal("expected +! to set high priority")
	}
	if idea.Category != "Engineering" {
		t.Fatalf("expected category Engineering, got %q", idea.Category)
	}
}

func TestExtractIdeaDefaultsNormalPriority(t *testing.T) {
	idea, err := Extract(1, []string{"plain idea"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idea.IsHighPriority {
		t.Fatal("expected default priority to be normal")
	}
	if idea.Category != "" {
		t.Fatalf("expected empty default category, got %q", idea.Category)
	}
}

func TestSetFieldsCategoryRequiresValue(t *testing.T) {
	idea := &Idea{Text: "x"}
	if err := idea.setFields([]string{"x", "+c"}); err == nil {
		t.Fatal("expected error when +c has no value")
	}
}

func TestSetFieldsStopsAtCommandBoundary(t *testing.T) {
	idea := &Idea{Text: "x"}
	// "*" is a command boundary; the +! after it must NOT be applied.
	if err := idea.setFields([]string{"x", "*", "+!"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idea.IsHighPriority {
		t.Fatal("expected parsing to stop at command boundary before +!")
	}
}

func TestSaveLoadGetRemoveIdea(t *testing.T) {
	withIsolatedIdeaDatabase(t)
	owner := int64(7)

	idea, err := Extract(owner, []string{"first idea", "+!", "+c", "Tech"})
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	id, err := idea.Save()
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	ideas, err := Load(owner, false, "")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(ideas) != 1 {
		t.Fatalf("expected 1 idea, got %d", len(ideas))
	}
	got, err := ideas.Get(id)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.Text != "first idea" || !got.IsHighPriority || got.Category != "Tech" {
		t.Fatalf("loaded idea mismatch: %+v", *got)
	}

	updated, err := ideas.Remove(owner, id)
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if len(updated) != 0 {
		t.Fatalf("expected empty list after remove, got %d", len(updated))
	}
	after, _ := Load(owner, false, "")
	if len(after) != 0 {
		t.Fatalf("expected 0 ideas in db after remove, got %d", len(after))
	}
}

func TestLoadFiltersByPriorityAndCategory(t *testing.T) {
	withIsolatedIdeaDatabase(t)
	owner := int64(8)

	mk := func(text string, high bool, cat string) {
		flags := []string{text}
		if high {
			flags = append(flags, "+!")
		} else {
			flags = append(flags, "-!")
		}
		if cat != "" {
			flags = append(flags, "+c", cat)
		}
		idea, err := Extract(owner, flags)
		if err != nil {
			t.Fatalf("extract failed: %v", err)
		}
		if _, err := idea.Save(); err != nil {
			t.Fatalf("save failed: %v", err)
		}
	}
	mk("high tech", true, "Tech")
	mk("normal tech", false, "Tech")
	mk("high biz", true, "Business")

	high, err := Load(owner, true, "")
	if err != nil {
		t.Fatalf("load high failed: %v", err)
	}
	if len(high) != 2 {
		t.Fatalf("expected 2 high-priority ideas, got %d", len(high))
	}

	tech, err := Load(owner, false, "Tech")
	if err != nil {
		t.Fatalf("load by category failed: %v", err)
	}
	if len(tech) != 2 {
		t.Fatalf("expected 2 Tech ideas, got %d", len(tech))
	}
}

func TestCategoriesRememberedAndOrderedByUsage(t *testing.T) {
	withIsolatedIdeaDatabase(t)
	owner := int64(9)

	save := func(cat string) {
		idea, _ := Extract(owner, []string{"idea", "+c", cat})
		if _, err := idea.Save(); err != nil {
			t.Fatalf("save failed: %v", err)
		}
	}
	save("Tech")
	save("Tech")
	save("Business")

	cats, err := LoadCategories(owner)
	if err != nil {
		t.Fatalf("load categories failed: %v", err)
	}
	if len(cats) != 2 {
		t.Fatalf("expected 2 unique categories, got %d: %v", len(cats), cats)
	}
	if cats[0] != "Tech" {
		t.Fatalf("expected most-used category Tech first, got %q", cats[0])
	}
}

func TestUpdateIdeaViaTerms(t *testing.T) {
	withIsolatedIdeaDatabase(t)
	owner := int64(10)

	idea, _ := Extract(owner, []string{"old text", "-!"})
	id, err := idea.Save()
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	ideas, _ := Load(owner, false, "")
	msg, err := ideas.Update(owner, []string{
		// id, set new text, mark high priority, set category
		strconv.FormatUint(id, 10), "+t", "new text", "+!", "+c", "Personal",
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !strings.Contains(msg, "new text") {
		t.Fatalf("expected updated text in message, got %q", msg)
	}

	reloaded, _ := Load(owner, false, "")
	got, _ := reloaded.Get(id)
	if got.Text != "new text" || !got.IsHighPriority || got.Category != "Personal" {
		t.Fatalf("update not persisted: %+v", *got)
	}
}
