package Idea

import (
	"os"
	"slices"
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

	ideas, err := Load(owner, false, false, 0)
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
	after, _ := Load(owner, false, false, 0)
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

	high, err := Load(owner, true, false, 0)
	if err != nil {
		t.Fatalf("load high failed: %v", err)
	}
	if len(high) != 2 {
		t.Fatalf("expected 2 high-priority ideas, got %d", len(high))
	}

	techID, err := LookupCategoryID(owner, "Tech")
	if err != nil || techID == 0 {
		t.Fatalf("expected to resolve Tech category id, got id=%d err=%v", techID, err)
	}
	tech, err := Load(owner, false, false, techID)
	if err != nil {
		t.Fatalf("load by category failed: %v", err)
	}
	if len(tech) != 2 {
		t.Fatalf("expected 2 Tech ideas, got %d", len(tech))
	}

	// An unknown category resolves to id 0 (no match).
	if missing, _ := LookupCategoryID(owner, "Nonexistent"); missing != 0 {
		t.Fatalf("expected unknown category to resolve to 0, got %d", missing)
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

	ideas, _ := Load(owner, false, false, 0)
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

	reloaded, _ := Load(owner, false, false, 0)
	got, _ := reloaded.Get(id)
	if got.Text != "new text" || !got.IsHighPriority || got.Category != "Personal" {
		t.Fatalf("update not persisted: %+v", *got)
	}
}

func TestIdeaHeader(t *testing.T) {
	cases := []struct{ in, want string }{
		{"single line", "single line"},
		{"  hi there  ", "hi there"},
		{"first line\nsecond line", "first line"},
		{"l1\nl2\nl3", "l1"},
		{"", "(empty)"},
		{"   ", "(empty)"},
	}
	for _, c := range cases {
		if got := (Idea{Text: c.in}).Header(); got != c.want {
			t.Fatalf("Header(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIdeaListToString(t *testing.T) {
	ideas := IdeaList{{Id: 1, Text: "alpha"}, {Id: 2, Text: "beta", IsHighPriority: true}}
	lines := ideas.ToString()
	if len(lines) != 2 {
		t.Fatalf("expected 2 rendered lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "alpha") || !strings.Contains(lines[1], "beta") {
		t.Fatalf("rendered lines mismatch: %v", lines)
	}
}

func TestIdeaListIndex(t *testing.T) {
	ideas := IdeaList{{Id: 5}, {Id: 9}, {Id: 2}}
	if got := ideas.Index(9); got != 1 {
		t.Fatalf("Index(9) = %d, want 1", got)
	}
	if got := ideas.Index(2); got != 2 {
		t.Fatalf("Index(2) = %d, want 2", got)
	}
	if got := ideas.Index(404); got != -1 {
		t.Fatalf("Index(404) = %d, want -1", got)
	}
}

func TestRegisterCategoryPublicWrapper(t *testing.T) {
	withIsolatedIdeaDatabase(t)
	owner := int64(41)

	id1, err := RegisterCategory(owner, "Tech")
	if err != nil || id1 == 0 {
		t.Fatalf("expected a category id, got id=%d err=%v", id1, err)
	}
	id2, err := RegisterCategory(owner, "Tech")
	if err != nil || id2 != id1 {
		t.Fatalf("expected the same id on re-register, got id1=%d id2=%d err=%v", id1, id2, err)
	}
	if blank, err := RegisterCategory(owner, "   "); err != nil || blank != 0 {
		t.Fatalf("expected blank category to yield id 0, got %d err=%v", blank, err)
	}
}

func TestToggleFavoriteAndFavoritesFilter(t *testing.T) {
	withIsolatedIdeaDatabase(t)
	owner := int64(11)

	idea, _ := Extract(owner, []string{"a favorite candidate"})
	id, err := idea.Save()
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if favs, _ := Load(owner, false, true, 0); len(favs) != 0 {
		t.Fatalf("expected 0 favorites before toggling, got %d", len(favs))
	}

	on, err := ToggleFavorite(owner, id)
	if err != nil || !on {
		t.Fatalf("expected toggle to favorite=true, got %v err=%v", on, err)
	}
	if favs, _ := Load(owner, false, true, 0); len(favs) != 1 || !favs[0].IsFavorite {
		t.Fatalf("expected 1 favorite after toggle, got %+v", favs)
	}

	off, err := ToggleFavorite(owner, id)
	if err != nil || off {
		t.Fatalf("expected toggle to favorite=false, got %v err=%v", off, err)
	}
	if favs, _ := Load(owner, false, true, 0); len(favs) != 0 {
		t.Fatalf("expected 0 favorites after second toggle, got %d", len(favs))
	}

	if _, err := ToggleFavorite(owner, 99999); err == nil {
		t.Fatal("expected error toggling a non-existent idea")
	}
}

func TestLoadRemindableOwnersReturnsFavoriteOrHighPriorityOwners(t *testing.T) {
	withIsolatedIdeaDatabase(t)

	withFav := int64(21)
	withHigh := int64(22)
	plainOnly := int64(23)

	favIdea, _ := Extract(withFav, []string{"keep me"})
	favID, _ := favIdea.Save()
	if _, err := ToggleFavorite(withFav, favID); err != nil {
		t.Fatalf("toggle failed: %v", err)
	}
	highIdea, _ := Extract(withHigh, []string{"urgent", "+!"})
	if _, err := highIdea.Save(); err != nil {
		t.Fatalf("save high-priority failed: %v", err)
	}
	plain, _ := Extract(plainOnly, []string{"not special"})
	if _, err := plain.Save(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	owners, err := LoadRemindableOwners()
	if err != nil {
		t.Fatalf("LoadRemindableOwners failed: %v", err)
	}
	if len(owners) != 2 {
		t.Fatalf("expected 2 remindable owners (fav + high), got %v", owners)
	}
	if !slices.Contains(owners, withFav) || !slices.Contains(owners, withHigh) {
		t.Fatalf("expected owners %d and %d, got %v", withFav, withHigh, owners)
	}
	if slices.Contains(owners, plainOnly) {
		t.Fatalf("plain-only owner %d should be excluded, got %v", plainOnly, owners)
	}
}

func TestLoadRemindablePoolIsFavoriteOrHighPriority(t *testing.T) {
	withIsolatedIdeaDatabase(t)
	owner := int64(41)

	fav, _ := Extract(owner, []string{"favorited"})
	favID, _ := fav.Save()
	if _, err := ToggleFavorite(owner, favID); err != nil {
		t.Fatalf("toggle failed: %v", err)
	}
	high, _ := Extract(owner, []string{"high", "+!"})
	if _, err := high.Save(); err != nil {
		t.Fatalf("save high failed: %v", err)
	}
	both, _ := Extract(owner, []string{"both", "+!"})
	bothID, _ := both.Save()
	if _, err := ToggleFavorite(owner, bothID); err != nil {
		t.Fatalf("toggle both failed: %v", err)
	}
	plain, _ := Extract(owner, []string{"plain"})
	if _, err := plain.Save(); err != nil {
		t.Fatalf("save plain failed: %v", err)
	}

	pool, err := LoadRemindable(owner)
	if err != nil {
		t.Fatalf("LoadRemindable failed: %v", err)
	}
	if len(pool) != 3 {
		t.Fatalf("expected 3 remindable ideas (fav, high, both), got %d", len(pool))
	}
	for i := range pool {
		if !pool[i].IsFavorite && !pool[i].IsHighPriority {
			t.Fatalf("remindable pool contains a plain idea: %+v", pool[i])
		}
	}
}

func TestCategoryIdStableOnTextEditAndChangesOnCategoryEdit(t *testing.T) {
	withIsolatedIdeaDatabase(t)
	owner := int64(31)

	idea, _ := Extract(owner, []string{"original", "+c", "Tech"})
	id, _ := idea.Save()
	techID := idea.CategoryId
	if techID == 0 {
		t.Fatal("expected a non-zero category id after save")
	}

	// Editing only the text must keep the same category_id.
	ideas, _ := Load(owner, false, false, 0)
	if _, err := ideas.Update(owner, []string{strconv.FormatUint(id, 10), "+t", "edited"}); err != nil {
		t.Fatalf("text update failed: %v", err)
	}
	reloaded, _ := Load(owner, false, false, 0)
	got, _ := reloaded.Get(id)
	if got.CategoryId != techID || got.Category != "Tech" {
		t.Fatalf("expected category to stay Tech(id=%d), got id=%d name=%q", techID, got.CategoryId, got.Category)
	}

	// Changing the category must point at a different id and resolve the name.
	if _, err := reloaded.Update(owner, []string{strconv.FormatUint(id, 10), "+c", "Business"}); err != nil {
		t.Fatalf("category update failed: %v", err)
	}
	again, _ := Load(owner, false, false, 0)
	got2, _ := again.Get(id)
	if got2.CategoryId == techID || got2.Category != "Business" {
		t.Fatalf("expected a new Business category id, got id=%d name=%q", got2.CategoryId, got2.Category)
	}
}
