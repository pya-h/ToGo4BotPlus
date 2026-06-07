package main

import (
	"strings"
	"testing"

	"ToGo4BotPlus/Article"
)

func seedArticle(t *testing.T, ownerID int64, title, url, category string) uint64 {
	t.Helper()
	a := &Article.Article{OwnerId: ownerID, Title: title, Url: url, Category: category}
	id, err := a.Save()
	if err != nil {
		t.Fatalf("failed to seed article %q: %v", title, err)
	}
	return id
}

func TestArticlebookCommandRendersList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9000)
	seedArticle(t, owner, "The Go Memory Model", "https://go.dev/ref/mem", "Tech")

	req := startFlowGetSend(t, bot, transport, owner, 800, "/articlebook")
	text := req.Values.Get("text")
	if !strings.Contains(text, "Your articles") || !strings.Contains(text, "The Go Memory Model") {
		t.Fatalf("expected article list, got %q", text)
	}
	if req.Values.Get("reply_markup") == "" {
		t.Fatalf("expected an inline keyboard on the article list")
	}
}

func TestRenderArticleListPaginates(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(9001)
	for i := 0; i < 12; i++ {
		seedArticle(t, owner, "a", "http://x", "")
	}
	articles, _ := Article.Load(owner, 0)

	_, kb := renderArticleList(articles, 0, 0)
	if len(kb.InlineKeyboard) != ArticlesPerMenuPage+1 {
		t.Fatalf("expected %d rows on page 0, got %d", ArticlesPerMenuPage+1, len(kb.InlineKeyboard))
	}
	if !strings.Contains(kbText(kb), "1/2") {
		t.Fatalf("expected a 1/2 indicator, got %q", kbText(kb))
	}
	if _, kb1 := renderArticleList(articles, 0, 1); len(kb1.InlineKeyboard) != 2+1 {
		t.Fatalf("expected 3 rows on page 1, got %d", len(kb1.InlineKeyboard))
	}
}

func TestArticleMenuOpenShowsDetailWithUrl(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9002)
	id := seedArticle(t, owner, "Deep dive", "https://example.com/deep", "Research")

	detail := sendCallbackAndGetEditedText(t, bot, transport, owner, 810,
		(CallbackData{Action: ArticleMenuOpen, ID: int64(id)}).Json())
	if !strings.Contains(detail, "Deep dive") || !strings.Contains(detail, "https://example.com/deep") {
		t.Fatalf("expected detail with url, got %q", detail)
	}
	if !strings.Contains(detail, "1 of 1") {
		t.Fatalf("expected position indicator, got %q", detail)
	}
}

func TestArticleMenuRemoveReturnsToList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9003)
	keep := seedArticle(t, owner, "keep", "http://k", "")
	drop := seedArticle(t, owner, "drop", "http://d", "")

	edited := sendCallbackAndGetEditedText(t, bot, transport, owner, 820,
		(CallbackData{Action: ArticleMenuRemove, ID: int64(drop)}).Json())
	if !strings.Contains(edited, "Removed") {
		t.Fatalf("expected removal feedback, got %q", edited)
	}
	remaining, _ := Article.Load(owner, 0)
	if len(remaining) != 1 || remaining[0].Id != keep {
		t.Fatalf("expected only the kept article, got %+v", remaining)
	}
}

func TestArticleMenuRemoveLastClearsKeyboard(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9004)
	id := seedArticle(t, owner, "only", "http://o", "")

	sendCallbackAndGetEditedText(t, bot, transport, owner, 830,
		(CallbackData{Action: ArticleMenuRemove, ID: int64(id)}).Json())
	req, _ := transport.lastEndpoint("editMessageText")
	if !strings.Contains(req.Values.Get("reply_markup"), `"inline_keyboard":[]`) {
		t.Fatalf("expected an empty keyboard after removing the last article, got %q", req.Values.Get("reply_markup"))
	}
}

func TestArticleMenuEditHandsOffToManageFlow(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9005)
	id := seedArticle(t, owner, "editable", "http://e", "")

	card := sendCallbackAndGetEditedText(t, bot, transport, owner, 840,
		(CallbackData{Action: ArticleMenuEdit, ID: int64(id)}).Json())
	if !strings.Contains(card, "editable") || !strings.Contains(card, "Editing") {
		t.Fatalf("expected the manage card, got %q", card)
	}
	state, active := bot.flows.Get(owner)
	if !active || state.Entity != "article" || state.ItemID != id {
		t.Fatalf("expected an active article manage flow, got %+v (active=%v)", state, active)
	}
	fields := sendCallbackAndGetEditedText(t, bot, transport, owner, 840,
		(CallbackData{Action: FlowEdit}).Json())
	if !strings.Contains(fields, "change") {
		t.Fatalf("expected the edit-field list, got %q", fields)
	}
}

func TestArticleListCommandTextReport(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9006)
	seedArticle(t, owner, "Report me", "http://r", "Tech")

	req := startFlowGetSend(t, bot, transport, owner, 850, ">l")
	if !strings.Contains(req.Values.Get("text"), "Report me") {
		t.Fatalf("expected text article report, got %q", req.Values.Get("text"))
	}
}

func TestArticleLegacyRemoveMenuAndCallback(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9007)
	seedArticle(t, owner, "one", "http://1", "")
	drop := seedArticle(t, owner, "two", "http://2", "")

	// `>x` opens the legacy paginated remove menu.
	req := startFlowGetSend(t, bot, transport, owner, 860, ">x")
	if !strings.Contains(req.Values.Get("text"), "remove") {
		t.Fatalf("expected the remove menu prompt, got %q", req.Values.Get("text"))
	}
	// Tapping a button removes that article.
	sendCallbackAndGetEditedText(t, bot, transport, owner, 861,
		(CallbackData{Action: RemoveArticle, ID: int64(drop)}).Json())
	remaining, _ := Article.Load(owner, 0)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 article left after legacy remove, got %d", len(remaining))
	}
}

func TestAddArticleFlowEndToEnd(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9008)

	start := startFlowGetSend(t, bot, transport, owner, 870, "/addArticle")
	if !strings.Contains(start.Values.Get("text"), "Article title?") {
		t.Fatalf("expected title prompt, got %q", start.Values.Get("text"))
	}
	urlPrompt := sendFlowTextGetEdit(t, bot, transport, owner, 871, "Great article")
	if !strings.Contains(urlPrompt, "link") {
		t.Fatalf("expected url prompt, got %q", urlPrompt)
	}
	categoryPrompt := sendFlowTextGetEdit(t, bot, transport, owner, 872, "https://great.example.com")
	if !strings.Contains(categoryPrompt, "category") {
		t.Fatalf("expected category prompt, got %q", categoryPrompt)
	}
	// Skip the category, then confirm.
	confirm := sendCallbackAndGetEditedText(t, bot, transport, owner, 873,
		(CallbackData{Action: FlowSkip}).Json())
	if !strings.Contains(confirm, "Review your article") {
		t.Fatalf("expected confirm screen, got %q", confirm)
	}
	saved := sendCallbackAndGetEditedText(t, bot, transport, owner, 874,
		(CallbackData{Action: FlowConfirm}).Json())
	if !strings.Contains(saved, "Saved") {
		t.Fatalf("expected save confirmation, got %q", saved)
	}

	articles, _ := Article.Load(owner, 0)
	if len(articles) != 1 || articles[0].Title != "Great article" || articles[0].Url != "https://great.example.com" {
		t.Fatalf("expected the saved article, got %+v", articles)
	}
}

func TestProcessArticleReminderSendsOneRandomArticle(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9100)
	seedArticle(t, owner, "Article One", "https://one.example.com", "")
	seedArticle(t, owner, "Article Two", "https://two.example.com", "")
	seedArticle(t, owner, "Article Three", "https://three.example.com", "")

	bot.processArticleReminderTick()

	if got := transport.countEndpoint("sendMessage"); got != 1 {
		t.Fatalf("expected exactly 1 reminder for the owner, sent %d", got)
	}
	last, _ := transport.lastEndpoint("sendMessage")
	text := last.Values.Get("text")
	if !strings.Contains(text, "revisiting") || !strings.Contains(text, "https://") {
		t.Fatalf("expected a reminder with a url, got %q", text)
	}
	// Plain text (no Markdown) so the url isn't mangled.
	if last.Values.Get("parse_mode") != "" {
		t.Fatalf("expected article reminder to be plain text, got parse_mode=%q", last.Values.Get("parse_mode"))
	}
}

func TestProcessArticleReminderIgnoresOwnersWithoutArticles(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	// No articles seeded for anyone.
	bot.processArticleReminderTick()
	if got := transport.countEndpoint("sendMessage"); got != 0 {
		t.Fatalf("expected no reminders when there are no articles, sent %d", got)
	}
}
