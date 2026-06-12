package main

import (
	"strings"
	"testing"

	"ToGo4BotPlus/Article"
	"ToGo4BotPlus/Idea"
	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// markArticleRead flips an article's read flag, used to verify the import
// restores read state.
func markArticleRead(t *testing.T, ownerID int64, title string) {
	t.Helper()
	articles, err := Article.Load(ownerID, 0)
	if err != nil {
		t.Fatalf("load articles to mark read: %v", err)
	}
	for i := range articles {
		if articles[i].Title == title {
			if err := articles[i].SetRead(ownerID, true); err != nil {
				t.Fatalf("set read: %v", err)
			}
			return
		}
	}
	t.Fatalf("article %q not found to mark read", title)
}

// TestImportUserDataRoundTrip is the core guarantee: a /json export of one owner
// imported into a different owner reproduces every record faithfully — counts,
// owner reassignment, categories (resolved by name), favorites/high-priority
// flags and article read state.
func TestImportUserDataRoundTrip(t *testing.T) {
	withTempWorkingDir(t, true)
	src := int64(7001)
	dst := int64(7002)

	seedTogo(t, src, "ship release", 30)
	seedTask(t, src, "write docs", 0)
	seedIdea(t, src, "rewrite parser", true, "Eng") // high priority + category
	favoriteIdea(t, src, "buy milk")                // favorite, no category
	seedArticle(t, src, "Go notes", "https://go.dev", "Tech")
	seedArticle(t, src, "already read", "https://read.me", "Tech")
	markArticleRead(t, src, "already read")

	data, err := buildUserDataExport(src)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	sum, err := importUserData(dst, data)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if sum.Togos != 1 || sum.Tasks != 1 || sum.Ideas != 2 || sum.Articles != 2 || sum.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", sum)
	}

	// Togo fidelity.
	togos, _ := Togo.Load(dst, false, false)
	if len(togos) != 1 || togos[0].Title != "ship release" || togos[0].Progress != 30 || togos[0].OwnerId != dst {
		t.Fatalf("togo not round-tripped: %+v", togos)
	}

	// Task fidelity.
	tasks, _ := Task.Load(dst, true, true)
	if len(tasks) != 1 || tasks[0].Title != "write docs" || tasks[0].OwnerId != dst {
		t.Fatalf("task not round-tripped: %+v", tasks)
	}

	// Idea fidelity: high-priority+category and favorite preserved.
	ideas, _ := Idea.Load(dst, false, false, 0)
	if len(ideas) != 2 {
		t.Fatalf("expected 2 ideas, got %d", len(ideas))
	}
	var parser, milk *Idea.Idea
	for i := range ideas {
		switch ideas[i].Text {
		case "rewrite parser":
			parser = &ideas[i]
		case "buy milk":
			milk = &ideas[i]
		}
	}
	if parser == nil || !parser.IsHighPriority || parser.Category != "Eng" || parser.OwnerId != dst {
		t.Fatalf("high-priority idea not round-tripped: %+v", parser)
	}
	if milk == nil || !milk.IsFavorite {
		t.Fatalf("favorite idea not round-tripped: %+v", milk)
	}

	// Article fidelity: category and read state preserved.
	articles, _ := Article.Load(dst, 0)
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}
	for i := range articles {
		if articles[i].OwnerId != dst {
			t.Fatalf("article owner not reassigned: %+v", articles[i])
		}
		if articles[i].Category != "Tech" {
			t.Fatalf("article category not round-tripped: %+v", articles[i])
		}
		wantRead := articles[i].Title == "already read"
		if articles[i].Read != wantRead {
			t.Fatalf("article %q read=%v, want %v", articles[i].Title, articles[i].Read, wantRead)
		}
	}
}

// TestImportUserDataIsAdditive: importing the same file twice yields two copies,
// never an id conflict.
func TestImportUserDataIsAdditive(t *testing.T) {
	withTempWorkingDir(t, true)
	src := int64(7101)
	dst := int64(7102)
	seedTogo(t, src, "dup me", 0)

	data, err := buildUserDataExport(src)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := importUserData(dst, data); err != nil {
		t.Fatalf("import 1: %v", err)
	}
	if _, err := importUserData(dst, data); err != nil {
		t.Fatalf("import 2: %v", err)
	}
	togos, _ := Togo.Load(dst, false, false)
	if len(togos) != 2 {
		t.Fatalf("expected 2 togos after importing twice, got %d", len(togos))
	}
}

// TestImportUserDataRejectsGarbage: a non-export file is rejected with an error,
// not a partial import.
func TestImportUserDataRejectsGarbage(t *testing.T) {
	withTempWorkingDir(t, true)
	if _, err := importUserData(7201, []byte("this is not json")); err == nil {
		t.Fatal("expected an error importing non-JSON data")
	}
}

// TestImportUserDataEmptyExport: an export with empty lists imports cleanly with
// zero counts.
func TestImportUserDataEmptyExport(t *testing.T) {
	withTempWorkingDir(t, true)
	data, err := buildUserDataExport(7301) // owner with no data
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	sum, err := importUserData(7302, data)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if sum.total() != 0 || sum.Failed != 0 {
		t.Fatalf("expected an empty import, got %+v", sum)
	}
}

// TestImportCommandArmsWaiterAndPrompts: /import replies with instructions and
// arms the upload waiter.
func TestImportCommandArmsWaiterAndPrompts(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(7401)

	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 11,
		Text:      "/import",
		Chat:      &tgbotapi.Chat{ID: owner},
	}})

	if got := transport.countEndpoint("sendMessage"); got != 1 {
		t.Fatalf("expected one prompt message, got %d", got)
	}
	last, _ := transport.lastEndpoint("sendMessage")
	if !strings.Contains(strings.ToLower(last.Values.Get("text")), "send me your exported") {
		t.Fatalf("prompt missing upload instruction, got %q", last.Values.Get("text"))
	}
	// The waiter must now be armed for this owner.
	if !bot.imports.take(owner) {
		t.Fatal("/import did not arm the upload waiter")
	}
}

// TestDocumentWithoutIntentIsNudged: a stray document (no /import, no caption)
// must not be imported — the user is nudged toward the command instead.
func TestDocumentWithoutIntentIsNudged(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(7501)

	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 12,
		Chat:      &tgbotapi.Chat{ID: owner},
		Document:  &tgbotapi.Document{FileID: "abc", FileSize: 10},
	}})

	if got := transport.countEndpoint("sendMessage"); got != 1 {
		t.Fatalf("expected one nudge message, got %d", got)
	}
	last, _ := transport.lastEndpoint("sendMessage")
	if !strings.Contains(strings.ToLower(last.Values.Get("text")), "/import") {
		t.Fatalf("nudge should mention /import, got %q", last.Values.Get("text"))
	}
	// No file fetch should have been attempted (getFile endpoint untouched).
	if got := transport.countEndpoint("getFile"); got != 0 {
		t.Fatalf("a stray document must not trigger a download, getFile calls=%d", got)
	}
}

// TestStartClearsImportWaiter: /start resets a pending import wait so a later
// unrelated upload isn't swallowed.
func TestStartClearsImportWaiter(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, _ := newRecordingBot(t)
	owner := int64(7601)

	bot.ensureFlows()
	bot.imports.arm(owner)

	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 13,
		Text:      "/start",
		Chat:      &tgbotapi.Chat{ID: owner},
	}})

	if bot.imports.take(owner) {
		t.Fatal("/start should have cleared the pending import wait")
	}
}
