package Article

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func withIsolatedArticleDatabase(t *testing.T) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	if err := InitDatabase(); err != nil {
		t.Fatalf("failed to initialize article database: %v", err)
	}
}

func TestExtractArticleEmptyTerms(t *testing.T) {
	if _, err := Extract(1, []string{}); err == nil {
		t.Fatal("expected error for empty article terms")
	}
}

func TestExtractArticleWithFlags(t *testing.T) {
	article, err := Extract(42, []string{"Cool post", "+u", "https://example.com/a_b", "+c", "Reading"})
	if err != nil {
		t.Fatalf("unexpected extract error: %v", err)
	}
	if article.OwnerId != 42 || article.Title != "Cool post" {
		t.Fatalf("owner/title mismatch: %+v", article)
	}
	if article.Url != "https://example.com/a_b" {
		t.Fatalf("expected url, got %q", article.Url)
	}
	if article.Category != "Reading" {
		t.Fatalf("expected category Reading, got %q", article.Category)
	}
}

func TestSetFieldsRequireValues(t *testing.T) {
	for _, flag := range []string{"+u", "+c", "+t"} {
		a := &Article{Title: "x"}
		if err := a.setFields([]string{"x", flag}); err == nil {
			t.Fatalf("expected error when %s has no value", flag)
		}
	}
}

func TestSetFieldsStopsAtCommandBoundary(t *testing.T) {
	a := &Article{Title: "x"}
	// ">" is a command boundary; the +u after it must NOT be applied.
	if err := a.setFields([]string{"x", ">", "+u", "http://z"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Url != "" {
		t.Fatalf("expected parsing to stop at command boundary, got url %q", a.Url)
	}
}

func TestArticleHeaderAndToString(t *testing.T) {
	if got := (Article{Title: "first\nsecond"}).Header(); got != "first" {
		t.Fatalf("Header multiline = %q, want first", got)
	}
	if got := (Article{Title: "   "}).Header(); got != "(untitled)" {
		t.Fatalf("Header blank = %q, want (untitled)", got)
	}
	s := (Article{Id: 3, Title: "T", Category: "C", Url: "http://u"}).ToString()
	if !strings.Contains(s, "#3") || !strings.Contains(s, "C") || !strings.Contains(s, "http://u") {
		t.Fatalf("ToString missing parts: %q", s)
	}
	if noURL := (Article{Id: 1, Title: "T"}).ToString(); !strings.Contains(noURL, "(no link)") {
		t.Fatalf("expected (no link) for empty url, got %q", noURL)
	}
}

func TestSaveLoadGetRemoveArticle(t *testing.T) {
	withIsolatedArticleDatabase(t)
	owner := int64(7)

	article, err := Extract(owner, []string{"Read me", "+u", "https://go.dev", "+c", "Tech"})
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	id, err := article.Save()
	if err != nil || id == 0 {
		t.Fatalf("save failed: id=%d err=%v", id, err)
	}
	if article.CategoryId == 0 {
		t.Fatal("expected a resolved category id after save")
	}

	articles, err := Load(owner, 0)
	if err != nil || len(articles) != 1 {
		t.Fatalf("load mismatch: n=%d err=%v", len(articles), err)
	}
	got, err := articles.Get(id)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got.Title != "Read me" || got.Url != "https://go.dev" || got.Category != "Tech" {
		t.Fatalf("loaded article mismatch: %+v", *got)
	}
	if articles.Index(id) != 0 || articles.Index(999) != -1 {
		t.Fatalf("Index wrong: %d / %d", articles.Index(id), articles.Index(999))
	}

	updated, err := articles.Remove(owner, id)
	if err != nil || len(updated) != 0 {
		t.Fatalf("remove failed: n=%d err=%v", len(updated), err)
	}
	if after, _ := Load(owner, 0); len(after) != 0 {
		t.Fatalf("expected 0 articles after remove, got %d", len(after))
	}
}

func TestLoadFiltersByCategory(t *testing.T) {
	withIsolatedArticleDatabase(t)
	owner := int64(8)

	mk := func(title, cat string) {
		a, _ := Extract(owner, []string{title, "+c", cat})
		if _, err := a.Save(); err != nil {
			t.Fatalf("save failed: %v", err)
		}
	}
	mk("a", "Tech")
	mk("b", "Tech")
	mk("c", "Life")

	techID, err := LookupCategoryID(owner, "Tech")
	if err != nil || techID == 0 {
		t.Fatalf("resolve Tech id: id=%d err=%v", techID, err)
	}
	if got, _ := Load(owner, techID); len(got) != 2 {
		t.Fatalf("expected 2 Tech articles, got %d", len(got))
	}
	if missing, _ := LookupCategoryID(owner, "Nope"); missing != 0 {
		t.Fatalf("expected unknown category to resolve to 0, got %d", missing)
	}
}

func TestUpdateArticleViaTerms(t *testing.T) {
	withIsolatedArticleDatabase(t)
	owner := int64(9)

	a, _ := Extract(owner, []string{"old title", "+u", "http://old"})
	id, err := a.Save()
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	articles, _ := Load(owner, 0)
	if _, err := articles.Update(owner, []string{
		strconv.FormatUint(id, 10), "+t", "new title", "+u", "http://new", "+c", "Eng",
	}); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	reloaded, _ := Load(owner, 0)
	got, _ := reloaded.Get(id)
	if got.Title != "new title" || got.Url != "http://new" || got.Category != "Eng" {
		t.Fatalf("update not persisted: %+v", *got)
	}
}

func TestLoadOwnersWithUnreadArticles(t *testing.T) {
	withIsolatedArticleDatabase(t)
	withArticle := int64(21)
	without := int64(22)

	a, _ := Extract(withArticle, []string{"saved"})
	id, err := a.Save()
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	owners, err := LoadOwnersWithUnreadArticles()
	if err != nil {
		t.Fatalf("LoadOwnersWithUnreadArticles failed: %v", err)
	}
	if len(owners) != 1 || owners[0] != withArticle {
		t.Fatalf("expected only owner %d, got %v (other=%d)", withArticle, owners, without)
	}

	// Marking the only article read drops the owner from the unread set and
	// from the unread load entirely. (Save returns the id without stamping the
	// struct, so toggle through a loaded copy — as the real callers do.)
	loaded, _ := Load(withArticle, 0)
	target, _ := loaded.Get(id)
	if err := target.SetRead(withArticle, true); err != nil {
		t.Fatalf("SetRead failed: %v", err)
	}
	owners, err = LoadOwnersWithUnreadArticles()
	if err != nil {
		t.Fatalf("LoadOwnersWithUnreadArticles after read failed: %v", err)
	}
	if len(owners) != 0 {
		t.Fatalf("expected no unread owners after marking read, got %v", owners)
	}
	unread, _ := LoadUnread(withArticle)
	if len(unread) != 0 {
		t.Fatalf("expected no unread articles, got %d", len(unread))
	}

	// The article is still loadable (read flag set) and toggles back.
	reloaded, _ := Load(withArticle, 0)
	got, _ := reloaded.Get(id)
	if got == nil || !got.Read {
		t.Fatalf("expected article to be marked read after SetRead")
	}
}

func TestCategoryIdStableOnTitleEdit(t *testing.T) {
	withIsolatedArticleDatabase(t)
	owner := int64(31)

	a, _ := Extract(owner, []string{"t", "+c", "Tech"})
	id, _ := a.Save()
	techID := a.CategoryId

	articles, _ := Load(owner, 0)
	if _, err := articles.Update(owner, []string{strconv.FormatUint(id, 10), "+t", "renamed"}); err != nil {
		t.Fatalf("title update failed: %v", err)
	}
	reloaded, _ := Load(owner, 0)
	got, _ := reloaded.Get(id)
	if got.CategoryId != techID || got.Category != "Tech" {
		t.Fatalf("expected category to stay Tech(id=%d), got id=%d name=%q", techID, got.CategoryId, got.Category)
	}
}
