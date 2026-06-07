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
	// 10 ideas packed MaximumNumberOfRowItems per row + 1 nav row.
	wantPage0 := (IdeasPerMenuPage+MaximumNumberOfRowItems-1)/MaximumNumberOfRowItems + 1
	if len(kb.InlineKeyboard) != wantPage0 {
		t.Fatalf("expected %d rows on page 0, got %d", wantPage0, len(kb.InlineKeyboard))
	}
	if !strings.Contains(kbText(kb), "1/2") {
		t.Fatalf("expected a 1/2 page indicator, keyboard labels were %q", kbText(kb))
	}

	// Page 1 holds the remaining 2 ideas (one packed row) + nav.
	_, kb2 := renderIdeaList(ideas, ideaScopeAll, 0, 1)
	if len(kb2.InlineKeyboard) != 1+1 {
		t.Fatalf("expected 2 rows on page 1 (2 ideas packed + nav), got %d", len(kb2.InlineKeyboard))
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

func TestLoadIdeasForScopeFilters(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(8200)
	seedIdea(t, owner, "high one", true, "Tech")
	seedIdea(t, owner, "normal tech", false, "Tech")
	seedIdea(t, owner, "normal biz", false, "Business")
	favoriteIdea(t, owner, "starred")

	if got, _ := loadIdeasForScope(owner, ideaScopeHigh, 0); len(got) != 1 || !got[0].IsHighPriority {
		t.Fatalf("high scope should return only the high-priority idea, got %+v", got)
	}
	if got, _ := loadIdeasForScope(owner, ideaScopeFav, 0); len(got) != 1 || !got[0].IsFavorite {
		t.Fatalf("favorites scope should return only the favorite, got %+v", got)
	}
	techID, _ := Idea.LookupCategoryID(owner, "Tech")
	if got, _ := loadIdeasForScope(owner, ideaScopeCategory, techID); len(got) != 2 {
		t.Fatalf("category scope should return the 2 Tech ideas, got %d", len(got))
	}
	if got, _ := loadIdeasForScope(owner, ideaScopeAll, 0); len(got) != 4 {
		t.Fatalf("all scope should return every idea, got %d", len(got))
	}
}

func TestRenderIdeaListScopeTitles(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(8201)
	seedIdea(t, owner, "x", true, "")
	ideas, _ := Idea.Load(owner, false, false, 0)

	for scope, want := range map[int]string{
		ideaScopeAll:      "Your ideas",
		ideaScopeHigh:     "High-priority ideas",
		ideaScopeFav:      "Favorite ideas",
		ideaScopeCategory: "Ideas in category",
	} {
		text, _ := renderIdeaList(ideas, scope, 0, 0)
		if !strings.Contains(text, want) {
			t.Fatalf("scope %d title: expected %q in %q", scope, want, text)
		}
	}
}

func TestRenderIdeaDetailNeighbors(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(8202)
	seedIdea(t, owner, "a", false, "")
	seedIdea(t, owner, "b", false, "")
	seedIdea(t, owner, "c", false, "")
	ideas, _ := Idea.Load(owner, false, false, 0) // ordered, 3 items

	// First item: Menu + Next (no Prev).
	_, kb, ok := renderIdeaDetail(ideas, ideaScopeAll, 0, ideas[0].Id)
	if !ok {
		t.Fatal("expected detail for first idea")
	}
	if labels := kbText(kb); strings.Contains(labels, "Prev") || !strings.Contains(labels, "Next") || !strings.Contains(labels, "Menu") {
		t.Fatalf("first item nav row wrong: %q", labels)
	}

	// Middle item: Prev + Menu + Next.
	_, kb, _ = renderIdeaDetail(ideas, ideaScopeAll, 0, ideas[1].Id)
	if labels := kbText(kb); !strings.Contains(labels, "Prev") || !strings.Contains(labels, "Next") {
		t.Fatalf("middle item should have both Prev and Next: %q", labels)
	}

	// Last item: Prev + Menu (no Next).
	_, kb, _ = renderIdeaDetail(ideas, ideaScopeAll, 0, ideas[2].Id)
	if labels := kbText(kb); !strings.Contains(labels, "Prev") || strings.Contains(labels, "Next") {
		t.Fatalf("last item nav row wrong: %q", labels)
	}

	// Unknown id: not found.
	if _, _, ok := renderIdeaDetail(ideas, ideaScopeAll, 0, 99999); ok {
		t.Fatal("expected ok=false for an unknown idea id")
	}
}

func TestIdeaMenuOpenUnknownIdFallsBackToList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8203)
	seedIdea(t, owner, "real idea", false, "")

	text := sendCallbackAndGetEditedText(t, bot, transport, owner, 760,
		(CallbackData{Action: IdeaMenuOpen, ID: 99999, IdeaScope: ideaScopeAll}).Json())
	if !strings.Contains(text, "Your ideas") {
		t.Fatalf("expected fallback to the list for an unknown id, got %q", text)
	}
}

func TestIdeaMenuUnfavoriteInFavScopeClearsKeyboard(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8204)
	id := favoriteIdea(t, owner, "lonely favorite")

	text := sendCallbackAndGetEditedText(t, bot, transport, owner, 770,
		(CallbackData{Action: IdeaMenuFav, ID: int64(id), IdeaScope: ideaScopeFav}).Json())
	if !strings.Contains(text, "Nothing here yet") {
		t.Fatalf("expected to fall back to an empty favorites list, got %q", text)
	}
	req, _ := transport.lastEndpoint("editMessageText")
	if !strings.Contains(req.Values.Get("reply_markup"), `"inline_keyboard":[]`) {
		t.Fatalf("expected stale buttons to be cleared, reply_markup was %q", req.Values.Get("reply_markup"))
	}
}

func TestIdeaMenuRemoveLastClearsKeyboard(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8205)
	id := seedIdea(t, owner, "the only idea", false, "")

	sendCallbackAndGetEditedText(t, bot, transport, owner, 780,
		(CallbackData{Action: IdeaMenuRemove, ID: int64(id), IdeaScope: ideaScopeAll}).Json())
	req, _ := transport.lastEndpoint("editMessageText")
	if !strings.Contains(req.Values.Get("reply_markup"), `"inline_keyboard":[]`) {
		t.Fatalf("expected an empty keyboard after removing the last idea, got %q", req.Values.Get("reply_markup"))
	}
	if remaining, _ := Idea.Load(owner, false, false, 0); len(remaining) != 0 {
		t.Fatalf("expected 0 ideas after removing the last, got %d", len(remaining))
	}
}

func TestIdeaMenuRemoveUnknownIdSurfacesError(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(8206)
	seedIdea(t, owner, "present idea", false, "")

	text := sendCallbackAndGetEditedText(t, bot, transport, owner, 790,
		(CallbackData{Action: IdeaMenuRemove, ID: 99999, IdeaScope: ideaScopeAll}).Json())
	if !strings.Contains(text, "no such idea") {
		t.Fatalf("expected an error when removing an unknown id, got %q", text)
	}
	if remaining, _ := Idea.Load(owner, false, false, 0); len(remaining) != 1 {
		t.Fatalf("the real idea must survive a failed remove, got %d", len(remaining))
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
