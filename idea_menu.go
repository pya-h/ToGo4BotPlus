package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"ToGo4BotPlus/Idea"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// ============================================================
// Interactive idea browser (the "rich" idea menu) + favorite-idea reminders.
//
// The browser is intentionally STATELESS: every button encodes the scope, the
// page, and (in the detail view) the idea id directly in its callback_data, so
// it survives bot restarts — exactly like the togo/task tick/remove menus.
// Editing is the one stateful action: it hands the same message off to the
// existing manage-flow card (which needs typed input), see IdeaMenuEdit.
// ============================================================

const (
	IdeasPerMenuPage      = 10 // ideas shown per browser page before pagination kicks in
	IdeaReminderBatchSize = 3  // how many random favorites a reminder surfaces
)

// Idea-browser scopes (stored in CallbackData.IdeaScope).
const (
	ideaScopeAll      = 0
	ideaScopeHigh     = 1
	ideaScopeFav      = 2
	ideaScopeCategory = 3
)

// loadIdeasForScope loads the owner's ideas filtered by the browser scope.
func loadIdeasForScope(ownerID int64, scope int, categoryID int64) (Idea.IdeaList, error) {
	switch scope {
	case ideaScopeHigh:
		return Idea.Load(ownerID, true, false, 0)
	case ideaScopeFav:
		return Idea.Load(ownerID, false, true, 0)
	case ideaScopeCategory:
		return Idea.Load(ownerID, false, false, categoryID)
	default:
		return Idea.Load(ownerID, false, false, 0)
	}
}

func ideaScopeTitle(scope int) string {
	switch scope {
	case ideaScopeHigh:
		return "🔴 High-priority ideas"
	case ideaScopeFav:
		return "❤️ Favorite ideas"
	case ideaScopeCategory:
		return "🏷 Ideas in category"
	default:
		return "💡 Your ideas"
	}
}

func priorityCircle(idea Idea.Idea) string {
	if idea.IsHighPriority {
		return "🔴"
	}
	return "⚪"
}

// ideaListLine renders one message line: "#id [circle] Category: header [❤️]".
func ideaListLine(idea Idea.Idea) string {
	category := idea.Category
	if category == "" {
		category = "—"
	}
	line := fmt.Sprintf("#%d %s %s: %s", idea.Id, priorityCircle(idea), category, idea.Header())
	if idea.IsFavorite {
		line += " ❤️"
	}
	return line
}

// ideaButtonRows builds one inline button per idea ("#id: header"), each opening
// that idea's detail view within the given scope.
func ideaButtonRows(ideas Idea.IdeaList, scope int, categoryID int64) [][]tgbotapi.InlineKeyboardButton {
	buttons := make([]tgbotapi.InlineKeyboardButton, 0, len(ideas))
	for i := range ideas {
		label := fmt.Sprintf("#%d: %s", ideas[i].Id, ideas[i].Header())
		if len(label) >= MaximumInlineButtonTextLength {
			label = fmt.Sprintf("%s...", truncateUTF8(label, MaximumInlineButtonTextLength-3))
		}
		data := (CallbackData{Action: IdeaMenuOpen, ID: int64(ideas[i].Id), IdeaScope: scope, IdeaCat: categoryID}).Json()
		buttons = append(buttons, tgbotapi.InlineKeyboardButton{Text: label, CallbackData: &data})
	}
	return packButtonsIntoRows(buttons, MaximumNumberOfRowItems)
}

// renderIdeaList builds the list view (message text + paginated inline menu).
func renderIdeaList(ideas Idea.IdeaList, scope int, categoryID int64, page int) (string, *tgbotapi.InlineKeyboardMarkup) {
	total := len(ideas)
	title := ideaScopeTitle(scope)
	if total == 0 {
		return fmt.Sprintf("%s\n\nNothing here yet.", title), nil
	}

	totalPages := (total + IdeasPerMenuPage - 1) / IdeasPerMenuPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * IdeasPerMenuPage
	end := start + IdeasPerMenuPage
	if end > total {
		end = total
	}
	pageItems := ideas[start:end]

	text := fmt.Sprintf("%s (%d):\n", title, total)
	for i := range pageItems {
		text += "\n" + ideaListLine(pageItems[i])
	}

	rows := ideaButtonRows(pageItems, scope, categoryID)
	if nav := ideaListNavRow(scope, categoryID, page, totalPages); nav != nil {
		rows = append(rows, nav)
	}
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	return text, &menu
}

// ideaListNavRow builds the ⬅️ Prev / page / Next ➡️ row (nil for a single page).
func ideaListNavRow(scope int, categoryID int64, page int, totalPages int) []tgbotapi.InlineKeyboardButton {
	if totalPages <= 1 {
		return nil
	}
	row := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	if page > 0 {
		prev := (CallbackData{Action: IdeaMenuList, IdeaScope: scope, IdeaCat: categoryID, MenuPage: page - 1}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prev})
	}
	indicator := (CallbackData{Action: IdeaMenuList, IdeaScope: scope, IdeaCat: categoryID, MenuPage: page}).Json()
	row = append(row, tgbotapi.InlineKeyboardButton{Text: fmt.Sprintf("%d/%d", page+1, totalPages), CallbackData: &indicator})
	if page < totalPages-1 {
		next := (CallbackData{Action: IdeaMenuList, IdeaScope: scope, IdeaCat: categoryID, MenuPage: page + 1}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &next})
	}
	return row
}

// renderIdeaDetail builds the detail view for one idea. Returns ok=false when the
// idea is no longer in the (filtered) list so the caller can fall back to the list.
func renderIdeaDetail(ideas Idea.IdeaList, scope int, categoryID int64, ideaID uint64) (string, *tgbotapi.InlineKeyboardMarkup, bool) {
	idx := ideas.Index(ideaID)
	if idx < 0 {
		return "", nil, false
	}
	idea := ideas[idx]
	page := idx / IdeasPerMenuPage

	heartLabel := "❤️ Favorite"
	if idea.IsFavorite {
		heartLabel = "💔 Unfavorite"
	}
	remove := (CallbackData{Action: IdeaMenuRemove, ID: int64(idea.Id), IdeaScope: scope, IdeaCat: categoryID, MenuPage: page}).Json()
	heart := (CallbackData{Action: IdeaMenuFav, ID: int64(idea.Id), IdeaScope: scope, IdeaCat: categoryID, MenuPage: page}).Json()
	edit := (CallbackData{Action: IdeaMenuEdit, ID: int64(idea.Id)}).Json()
	actionRow := []tgbotapi.InlineKeyboardButton{
		{Text: "🗑 Remove", CallbackData: &remove},
		{Text: heartLabel, CallbackData: &heart},
		{Text: "✏️ Edit", CallbackData: &edit},
	}

	navRow := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	if idx > 0 {
		prev := (CallbackData{Action: IdeaMenuOpen, ID: int64(ideas[idx-1].Id), IdeaScope: scope, IdeaCat: categoryID}).Json()
		navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prev})
	}
	menu := (CallbackData{Action: IdeaMenuList, IdeaScope: scope, IdeaCat: categoryID, MenuPage: page}).Json()
	navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "🔙 Menu", CallbackData: &menu})
	if idx < len(ideas)-1 {
		next := (CallbackData{Action: IdeaMenuOpen, ID: int64(ideas[idx+1].Id), IdeaScope: scope, IdeaCat: categoryID}).Json()
		navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &next})
	}

	text := fmt.Sprintf("%s\n\n(%d of %d)", idea.ToString(), idx+1, len(ideas))
	kb := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{actionRow, navRow}}
	return text, &kb, true
}

// renderIdeaReminder renders a fixed batch of favorites (no pagination) using the
// same line format and the same open-on-tap mechanism (favorites scope).
func renderIdeaReminder(ideas Idea.IdeaList) (string, *tgbotapi.InlineKeyboardMarkup) {
	text := "❤️ A few of your favorite ideas to revisit:\n"
	for i := range ideas {
		text += "\n" + ideaListLine(ideas[i])
	}
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: ideaButtonRows(ideas, ideaScopeFav, 0)}
	return text, &menu
}

// ---------------------- Browser callback handlers -----------------------------

// handleIdeaMenuCallback drives the stateless idea browser. It fills response
// with the new message text / inline keyboard; HandleUpdate edits the message.
func (telegramBot *TelegramBotAPI) handleIdeaMenuCallback(cb CallbackData, response *TelegramResponse) {
	owner := response.TargetChatId

	switch cb.Action {
	case IdeaMenuList:
		ideas, warning := loadIdeasForScope(owner, cb.IdeaScope, cb.IdeaCat)
		text, kb := renderIdeaList(ideas, cb.IdeaScope, cb.IdeaCat, cb.MenuPage)
		response.TextMsg = appendWarning(text, warning)
		response.InlineKeyboard = orEmptyKeyboard(kb)

	case IdeaMenuOpen:
		ideas, warning := loadIdeasForScope(owner, cb.IdeaScope, cb.IdeaCat)
		response.TextMsg, response.InlineKeyboard = detailOrList(ideas, cb, uint64(cb.ID), cb.MenuPage, warning)

	case IdeaMenuFav:
		if _, err := Idea.ToggleFavorite(owner, uint64(cb.ID)); err != nil {
			response.TextMsg = err.Error()
			return
		}
		// Falls back to the list when un-favoriting removes the idea from the
		// favorites scope; always returns a non-nil keyboard so stale buttons clear.
		ideas, warning := loadIdeasForScope(owner, cb.IdeaScope, cb.IdeaCat)
		response.TextMsg, response.InlineKeyboard = detailOrList(ideas, cb, uint64(cb.ID), cb.MenuPage, warning)

	case IdeaMenuRemove:
		before, _ := loadIdeasForScope(owner, cb.IdeaScope, cb.IdeaCat)
		updated, err := before.Remove(owner, uint64(cb.ID))
		if err != nil {
			// The DB delete runs before the in-memory lookup, so the row may be
			// gone even when the id isn't in this scope's filtered list. Reload
			// and only surface the error if nothing actually changed.
			after, _ := loadIdeasForScope(owner, cb.IdeaScope, cb.IdeaCat)
			if len(after) == len(before) {
				response.TextMsg = err.Error()
				return
			}
			updated = after
		}
		text, kb := renderIdeaList(updated, cb.IdeaScope, cb.IdeaCat, cb.MenuPage)
		response.TextMsg = "🗑 Removed.\n\n" + text
		response.InlineKeyboard = orEmptyKeyboard(kb)

	case IdeaMenuEdit:
		telegramBot.enterIdeaEditFromMenu(owner, cb, response)
	}
}

// detailOrList renders the idea's detail view, or falls back to the list when the
// idea is no longer in the current scope (e.g. just un-favorited). It always
// returns a non-nil keyboard so an edit clears any stale buttons.
func detailOrList(ideas Idea.IdeaList, cb CallbackData, ideaID uint64, page int, warning error) (string, *tgbotapi.InlineKeyboardMarkup) {
	if text, kb, ok := renderIdeaDetail(ideas, cb.IdeaScope, cb.IdeaCat, ideaID); ok {
		return appendWarning(text, warning), orEmptyKeyboard(kb)
	}
	text, kb := renderIdeaList(ideas, cb.IdeaScope, cb.IdeaCat, page)
	return appendWarning(text, warning), orEmptyKeyboard(kb)
}

// orEmptyKeyboard returns an empty (non-nil) inline keyboard for nil input, so an
// edited message drops stale buttons instead of keeping the previous keyboard.
func orEmptyKeyboard(kb *tgbotapi.InlineKeyboardMarkup) *tgbotapi.InlineKeyboardMarkup {
	if kb == nil {
		return emptyInlineKeyboard()
	}
	return kb
}

// enterIdeaEditFromMenu turns the current browser message into the edit card for
// the selected idea (so subsequent typed/tapped edits route through the shared
// edit engine).
func (telegramBot *TelegramBotAPI) enterIdeaEditFromMenu(owner int64, cb CallbackData, response *TelegramResponse) {
	telegramBot.enterEditFromMenu(owner, "idea", uint64(cb.ID), response)
}

func appendWarning(text string, warning error) string {
	if warning != nil {
		return fmt.Sprintf("%s\n\nwarning: %s", text, warning.Error())
	}
	return text
}

// ---------------------- Favorite-idea reminders -------------------------------

// ReminderStore keeps each owner's next favorite-idea reminder time in memory
// (intentionally not persisted: on restart, the next tick simply recomputes a
// time for each owner, just like guided-flow state).
type ReminderStore struct {
	mu   sync.Mutex
	next map[int64]time.Time
}

func NewReminderStore() *ReminderStore {
	return &ReminderStore{next: make(map[int64]time.Time)}
}

func (s *ReminderStore) Get(ownerID int64) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.next[ownerID]
	return t, ok
}

func (s *ReminderStore) Set(ownerID int64, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next[ownerID] = t
}

// ideaReminderDelta returns the gap until the next reminder: a random 1–30 days.
// It is a package var so tests can make it deterministic.
var ideaReminderDelta = func() time.Duration {
	return time.Duration(rand.Intn(30)+1) * 24 * time.Hour
}

// pickRandomIdeas returns up to n ideas chosen at random (order shuffled).
func pickRandomIdeas(ideas Idea.IdeaList, n int) Idea.IdeaList {
	pool := make(Idea.IdeaList, len(ideas))
	copy(pool, ideas)
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	if len(pool) > n {
		pool = pool[:n]
	}
	return pool
}

// RemindFavoriteIdeas runs the hourly reminder tick forever.
func (telegramBot *TelegramBotAPI) RemindFavoriteIdeas() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		telegramBot.processIdeaReminderTick(Togo.Today().Time)
	}
}

// processIdeaReminderTick is one reminder pass. For every owner with at least one
// favorite idea: if they have no scheduled time yet, just compute one and move on
// (no message); if their time has arrived, send up to IdeaReminderBatchSize random
// favorites and schedule the next reminder a random 1–30 days out.
func (telegramBot *TelegramBotAPI) processIdeaReminderTick(now time.Time) {
	owners, err := Idea.LoadFavoriteOwners()
	if err != nil {
		log.Println("idea reminder tick: could not load favorite owners:", err.Error())
		return
	}
	for _, owner := range owners {
		next, scheduled := telegramBot.ideaReminders.Get(owner)
		if !scheduled {
			telegramBot.ideaReminders.Set(owner, now.Add(ideaReminderDelta()))
			continue
		}
		if now.Before(next) {
			continue
		}
		favorites, loadErr := Idea.Load(owner, false, true, 0)
		if loadErr != nil {
			log.Println("idea reminder tick: load favorites failed:", loadErr.Error())
		}
		batch := pickRandomIdeas(favorites, IdeaReminderBatchSize)
		if len(batch) == 0 {
			// Nothing to send (a hard load error left the list empty, or the
			// favorites were just cleared). Leave the schedule due and retry next
			// tick rather than burning this reminder window.
			continue
		}
		telegramBot.sendIdeaReminder(owner, batch)
		telegramBot.ideaReminders.Set(owner, now.Add(ideaReminderDelta()))
	}
}

func (telegramBot *TelegramBotAPI) sendIdeaReminder(ownerID int64, ideas Idea.IdeaList) {
	text, kb := renderIdeaReminder(ideas)
	telegramBot.SendTextMessage(TelegramResponse{TargetChatId: ownerID, TextMsg: text, InlineKeyboard: kb})
}
