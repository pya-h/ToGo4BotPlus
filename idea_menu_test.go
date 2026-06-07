package main

import (
	"strings"
	"testing"
	"time"

	"ToGo4BotPlus/Idea"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// favoriteIdea seeds an idea and marks it favorite, returning its id.
func favoriteIdea(t *testing.T, ownerID int64, text string) uint64 {
	t.Helper()
	id := seedIdea(t, ownerID, text, false, "")
	if _, err := Idea.ToggleFavorite(ownerID, id); err != nil {
		t.Fatalf("failed to favorite idea %q: %v", text, err)
	}
	return id
}

func TestRenderIdeaListPaginatesAbovePageSize(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(8000)
	for i := 0; i < 12; i++ {
		seedIdea(t, owner, "idea", false, "")
	}
	ideas, _ := Idea.Load(owner, false, false, 0)

	_, kb := renderIdeaList(ideas, ideaScopeAll, 0, 0)
	if kb == nil {
		t.Fatal("expected a keyboard for a non-empty list")
	}
	// 10 idea rows + 1 nav row.
	if len(kb.InlineKeyboard) != IdeasPerMenuPage+1 {
		t.Fatalf("expected %d rows on page 0, got %d", IdeasPerMenuPage+1, len(kb.InlineKeyboard))
	}
	if !strings.Contains(kbText(kb), "1/2") {
		t.Fatalf("expected a 1/2 page indicator, keyboard labels were %q", kbText(kb))
	}

	// Page 1 holds the remaining 2 ideas + nav.
	_, kb2 := renderIdeaList(ideas, ideaScopeAll, 0, 1)
	if len(kb2.InlineKeyboard) != 2+1 {
		t.Fatalf("expected 3 rows on page 1 (2 ideas + nav), got %d", len(kb2.InlineKeyboard))
	}
}

func TestRenderIdeaListNoPaginationWhenSmall(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(8001)
	seedIdea(t, owner, "only one", false, "")
	ideas, _ := Idea.Load(owner, false, false, 0)

	_, kb := renderIdeaList(ideas, ideaScopeAll, 0, 0)
	if len(kb.InlineKeyboard) != 1 {
		t.Fatalf("expected exactly 1 row (no nav) for a single idea, got %d", len(kb.InlineKeyboard))
	}
}

func TestIdeabookCommandRendersList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8002)
	seedIdea(t, owner, "Launch a startup", true, "Tech")

	req := startFlowGetSend(t, bot, transport, owner, 700, "/ideabook")
	text := req.Values.Get("text")
	if !strings.Contains(text, "Your ideas") || !strings.Contains(text, "Launch a startup") {
		t.Fatalf("expected idea list message, got %q", text)
	}
	if req.Values.Get("reply_markup") == "" {
		t.Fatalf("expected an inline keyboard on the idea list")
	}
}

func TestIdeaMenuOpenShowsDetail(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8003)
	id := seedIdea(t, owner, "deep idea", true, "Research")

	detail := sendCallbackAndGetEditedText(t, bot, transport, owner, 710,
		(CallbackData{Action: IdeaMenuOpen, ID: int64(id), IdeaScope: ideaScopeAll}).Json())
	if !strings.Contains(detail, "deep idea") || !strings.Contains(detail, "Research") {
		t.Fatalf("expected full idea detail, got %q", detail)
	}
	if !strings.Contains(detail, "1 of 1") {
		t.Fatalf("expected position indicator, got %q", detail)
	}
}

func TestIdeaMenuToggleFavorite(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8004)
	id := seedIdea(t, owner, "maybe favorite", false, "")

	edited := sendCallbackAndGetEditedText(t, bot, transport, owner, 720,
		(CallbackData{Action: IdeaMenuFav, ID: int64(id), IdeaScope: ideaScopeAll}).Json())
	if !strings.Contains(edited, "Favorite: ❤️ Yes") {
		t.Fatalf("expected the detail to show the idea favorited, got %q", edited)
	}
	favs, _ := Idea.Load(owner, false, true, 0)
	if len(favs) != 1 {
		t.Fatalf("expected the idea to be favorited in the db, got %d favorites", len(favs))
	}
}

func TestIdeaMenuRemoveReturnsToList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8005)
	keep := seedIdea(t, owner, "keep me", false, "")
	drop := seedIdea(t, owner, "drop me", false, "")

	edited := sendCallbackAndGetEditedText(t, bot, transport, owner, 730,
		(CallbackData{Action: IdeaMenuRemove, ID: int64(drop), IdeaScope: ideaScopeAll}).Json())
	if !strings.Contains(edited, "Removed") || !strings.Contains(edited, "keep me") {
		t.Fatalf("expected removal then the remaining list, got %q", edited)
	}
	remaining, _ := Idea.Load(owner, false, false, 0)
	if len(remaining) != 1 || remaining[0].Id != keep {
		t.Fatalf("expected only the kept idea to remain, got %+v", remaining)
	}
}

func TestIdeaMenuEditHandsOffToManageFlow(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8006)
	id := seedIdea(t, owner, "editable idea", false, "")

	card := sendCallbackAndGetEditedText(t, bot, transport, owner, 740,
		(CallbackData{Action: IdeaMenuEdit, ID: int64(id)}).Json())
	if !strings.Contains(card, "editable idea") || !strings.Contains(card, "Editing") {
		t.Fatalf("expected the manage card with an editing note, got %q", card)
	}
	state, active := bot.flows.Get(owner)
	if !active || state.Entity != "idea" || state.ItemID != id {
		t.Fatalf("expected an active idea manage flow for id %d, got %+v (active=%v)", id, state, active)
	}
	// The handed-off card must now respond to the manage-flow Edit action.
	fields := sendCallbackAndGetEditedText(t, bot, transport, owner, 740,
		(CallbackData{Action: FlowEdit}).Json())
	if !strings.Contains(fields, "change") {
		t.Fatalf("expected the edit-field list after handoff, got %q", fields)
	}
}

func TestFavoritesCommandShowsOnlyFavorites(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8007)
	seedIdea(t, owner, "ordinary idea", false, "")
	favoriteIdea(t, owner, "starred idea")

	req := startFlowGetSend(t, bot, transport, owner, 750, "/favorites")
	text := req.Values.Get("text")
	if !strings.Contains(text, "Favorite ideas") || !strings.Contains(text, "starred idea") {
		t.Fatalf("expected favorites list with the starred idea, got %q", text)
	}
	if strings.Contains(text, "ordinary idea") {
		t.Fatalf("favorites list must not include non-favorite ideas: %q", text)
	}
}

// ---------------------- Reminder process tests --------------------------------

func TestProcessIdeaReminderSchedulesFirstThenSends(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8100)
	favoriteIdea(t, owner, "remind me")

	// Deterministic 1-day gap for this test.
	restore := ideaReminderDelta
	ideaReminderDelta = func() time.Duration { return 24 * time.Hour }
	t.Cleanup(func() { ideaReminderDelta = restore })

	base := time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC)

	// First tick: must only schedule (no message).
	bot.processIdeaReminderTick(base)
	if got := transport.countEndpoint("sendMessage"); got != 0 {
		t.Fatalf("first tick should not send anything, sent %d", got)
	}
	next, ok := bot.ideaReminders.Get(owner)
	if !ok || !next.Equal(base.Add(24*time.Hour)) {
		t.Fatalf("expected schedule at base+24h, got %v (ok=%v)", next, ok)
	}

	// Not yet due: still nothing.
	bot.processIdeaReminderTick(base.Add(23 * time.Hour))
	if got := transport.countEndpoint("sendMessage"); got != 0 {
		t.Fatalf("should not send before due time, sent %d", got)
	}

	// Past due: send exactly one reminder and reschedule.
	due := base.Add(25 * time.Hour)
	bot.processIdeaReminderTick(due)
	if got := transport.countEndpoint("sendMessage"); got != 1 {
		t.Fatalf("expected exactly 1 reminder at due time, sent %d", got)
	}
	rescheduled, _ := bot.ideaReminders.Get(owner)
	if !rescheduled.Equal(due.Add(24 * time.Hour)) {
		t.Fatalf("expected reschedule at due+24h, got %v", rescheduled)
	}
	last, _ := transport.lastEndpoint("sendMessage")
	if !strings.Contains(last.Values.Get("text"), "remind me") {
		t.Fatalf("expected the reminder to list the favorite, got %q", last.Values.Get("text"))
	}
}

func TestProcessIdeaReminderIgnoresOwnersWithoutFavorites(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8101)
	seedIdea(t, owner, "not a favorite", false, "") // never favorited

	bot.processIdeaReminderTick(time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC))
	if got := transport.countEndpoint("sendMessage"); got != 0 {
		t.Fatalf("owners without favorites must be skipped, sent %d", got)
	}
	if _, ok := bot.ideaReminders.Get(owner); ok {
		t.Fatal("owners without favorites should not get a schedule")
	}
}

func TestReminderBatchLimitedToThree(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8102)
	for i := 0; i < 5; i++ {
		favoriteIdea(t, owner, "fav")
	}

	restore := ideaReminderDelta
	ideaReminderDelta = func() time.Duration { return 24 * time.Hour }
	t.Cleanup(func() { ideaReminderDelta = restore })

	base := time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC)
	bot.processIdeaReminderTick(base)                     // schedule
	bot.processIdeaReminderTick(base.Add(48 * time.Hour)) // due -> send

	if got := transport.countEndpoint("sendMessage"); got != 1 {
		t.Fatalf("expected one reminder message, sent %d", got)
	}
	last, _ := transport.lastEndpoint("sendMessage")
	// Each listed idea is its own "\n#..." line; the batch is capped at 3.
	lines := strings.Count(last.Values.Get("text"), "\n#")
	if lines == 0 || lines > IdeaReminderBatchSize {
		t.Fatalf("expected 1..%d ideas in the reminder, got %d lines: %q", IdeaReminderBatchSize, lines, last.Values.Get("text"))
	}
}

// kbText flattens an inline keyboard's button labels for assertions.
func kbText(kb *tgbotapi.InlineKeyboardMarkup) string {
	var b strings.Builder
	for _, row := range kb.InlineKeyboard {
		for _, btn := range row {
			b.WriteString(btn.Text)
			b.WriteByte(' ')
		}
	}
	return b.String()
}
