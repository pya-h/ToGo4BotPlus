package main

import (
	"fmt"

	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// ============================================================
// Interactive togo browser (the single rich togo menu, opened with /togos).
//
// Like the idea/article browsers it is STATELESS: every button encodes the page
// and (in the detail view) the togo id directly in its callback_data, so it
// survives restarts. Togos have a done state, so the detail view offers a Toggle
// in addition to Remove/Edit. Editing is the one stateful action: it hands the
// message off to the existing edit screens (see TogoMenuEdit).
// ============================================================

const TogosPerMenuPage = 30 // togos shown per browser page before pagination kicks in

// loadTogosForBrowse loads every one of the owner's togos (all days), matching
// the scope the old manage flow used.
func loadTogosForBrowse(ownerID int64) (Togo.TogoList, error) {
	return Togo.Load(ownerID, false, false)
}

// togoListLine renders one message line: "#id [✅] title".
func togoListLine(togo Togo.Togo) string {
	status := ""
	if togo.Progress >= 100 {
		status = "✅ "
	}
	return fmt.Sprintf("#%d %s%s", togo.Id, status, togo.Title)
}

// togoButtonRows builds one inline button per togo ("#id: title"), packed several
// to a row, each opening that togo's detail view.
func togoButtonRows(togos Togo.TogoList) [][]tgbotapi.InlineKeyboardButton {
	buttons := make([]tgbotapi.InlineKeyboardButton, 0, len(togos))
	for i := range togos {
		status := ""
		if togos[i].Progress >= 100 {
			status = "✅ "
		}
		label := fmt.Sprintf("#%d: %s%s", togos[i].Id, status, togos[i].Title)
		if len(label) >= MaximumInlineButtonTextLength {
			label = fmt.Sprintf("%s...", truncateUTF8(label, MaximumInlineButtonTextLength-3))
		}
		data := (CallbackData{Action: TogoMenuOpen, ID: int64(togos[i].Id)}).Json()
		buttons = append(buttons, tgbotapi.InlineKeyboardButton{Text: label, CallbackData: &data})
	}
	return packButtonsIntoRows(buttons, MaximumNumberOfRowItems)
}

// renderTogoList builds the list view (message text + paginated inline menu).
func renderTogoList(togos Togo.TogoList, page int) (string, *tgbotapi.InlineKeyboardMarkup) {
	total := len(togos)
	title := "➕ Your togos"
	if total == 0 {
		return fmt.Sprintf("%s\n\nNothing here yet.", title), nil
	}

	totalPages := (total + TogosPerMenuPage - 1) / TogosPerMenuPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * TogosPerMenuPage
	end := start + TogosPerMenuPage
	if end > total {
		end = total
	}
	pageItems := togos[start:end]

	text := fmt.Sprintf("%s (%d):\n", title, total)
	for i := range pageItems {
		text += "\n" + togoListLine(pageItems[i])
	}

	rows := togoButtonRows(pageItems)
	if nav := togoListNavRow(page, totalPages); nav != nil {
		rows = append(rows, nav)
	}
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	return text, &menu
}

// togoListNavRow builds the ⬅️ Prev / page / Next ➡️ row (nil for a single page).
func togoListNavRow(page int, totalPages int) []tgbotapi.InlineKeyboardButton {
	if totalPages <= 1 {
		return nil
	}
	row := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	if page > 0 {
		prev := (CallbackData{Action: TogoMenuList, MenuPage: page - 1}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prev})
	}
	indicator := (CallbackData{Action: TogoMenuList, MenuPage: page}).Json()
	row = append(row, tgbotapi.InlineKeyboardButton{Text: fmt.Sprintf("%d/%d", page+1, totalPages), CallbackData: &indicator})
	if page < totalPages-1 {
		next := (CallbackData{Action: TogoMenuList, MenuPage: page + 1}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &next})
	}
	return row
}

// renderTogoDetail builds the detail view for one togo. Returns ok=false when the
// togo is no longer in the list so the caller can fall back to the list.
func renderTogoDetail(togos Togo.TogoList, togoID uint64) (string, *tgbotapi.InlineKeyboardMarkup, bool) {
	idx := togos.Index(togoID)
	if idx < 0 {
		return "", nil, false
	}
	togo := togos[idx]
	page := idx / TogosPerMenuPage

	toggleLabel := "✅ Mark done"
	if togo.Progress >= 100 {
		toggleLabel = "↩️ Mark undone"
	}
	remove := (CallbackData{Action: TogoMenuRemove, ID: int64(togo.Id), MenuPage: page}).Json()
	toggle := (CallbackData{Action: TogoMenuToggle, ID: int64(togo.Id), MenuPage: page}).Json()
	edit := (CallbackData{Action: TogoMenuEdit, ID: int64(togo.Id)}).Json()
	actionRow := []tgbotapi.InlineKeyboardButton{
		{Text: "🗑 Remove", CallbackData: &remove},
		{Text: toggleLabel, CallbackData: &toggle},
		{Text: "✏️ Edit", CallbackData: &edit},
	}

	navRow := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	if idx > 0 {
		prev := (CallbackData{Action: TogoMenuOpen, ID: int64(togos[idx-1].Id)}).Json()
		navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prev})
	}
	menu := (CallbackData{Action: TogoMenuList, MenuPage: page}).Json()
	navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "🔙 Menu", CallbackData: &menu})
	if idx < len(togos)-1 {
		next := (CallbackData{Action: TogoMenuOpen, ID: int64(togos[idx+1].Id)}).Json()
		navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &next})
	}

	text := fmt.Sprintf("%s\n\n(%d of %d)", togo.ToString(), idx+1, len(togos))
	kb := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{actionRow, navRow}}
	return text, &kb, true
}

// handleTogoMenuCallback drives the stateless togo browser.
func (telegramBot *TelegramBotAPI) handleTogoMenuCallback(cb CallbackData, response *TelegramResponse) {
	owner := response.TargetChatId

	switch cb.Action {
	case TogoMenuList:
		togos, warning := loadTogosForBrowse(owner)
		text, kb := renderTogoList(togos, cb.MenuPage)
		response.TextMsg = appendWarning(text, warning)
		response.InlineKeyboard = orEmptyKeyboard(kb)

	case TogoMenuOpen:
		togos, warning := loadTogosForBrowse(owner)
		response.TextMsg, response.InlineKeyboard = togoDetailOrList(togos, uint64(cb.ID), cb.MenuPage, warning)

	case TogoMenuToggle:
		togos, _ := loadTogosForBrowse(owner)
		if togo, err := togos.Get(uint64(cb.ID)); err == nil {
			if togo.Progress < 100 {
				togo.Progress = 100
			} else {
				togo.Progress = 0
			}
			if updateErr := togo.Update(owner); updateErr != nil {
				response.TextMsg = updateErr.Error()
				return
			}
		}
		togos, warning := loadTogosForBrowse(owner)
		response.TextMsg, response.InlineKeyboard = togoDetailOrList(togos, uint64(cb.ID), cb.MenuPage, warning)

	case TogoMenuRemove:
		before, _ := loadTogosForBrowse(owner)
		updated, err := before.Remove(owner, uint64(cb.ID))
		if err != nil {
			after, _ := loadTogosForBrowse(owner)
			if len(after) == len(before) {
				response.TextMsg = err.Error()
				return
			}
			updated = after
		}
		text, kb := renderTogoList(updated, cb.MenuPage)
		response.TextMsg = "🗑 Removed.\n\n" + text
		response.InlineKeyboard = orEmptyKeyboard(kb)

	case TogoMenuEdit:
		telegramBot.enterTogoEditFromMenu(owner, cb, response)
	}
}

// togoDetailOrList renders the togo's detail view, or falls back to the list when
// the togo is gone. It always returns a non-nil keyboard so an edit clears any
// stale buttons.
func togoDetailOrList(togos Togo.TogoList, togoID uint64, page int, warning error) (string, *tgbotapi.InlineKeyboardMarkup) {
	if text, kb, ok := renderTogoDetail(togos, togoID); ok {
		return appendWarning(text, warning), orEmptyKeyboard(kb)
	}
	text, kb := renderTogoList(togos, page)
	return appendWarning(text, warning), orEmptyKeyboard(kb)
}

// enterTogoEditFromMenu turns the browser message into the edit card for the
// selected togo (so typed/tapped edits route through the shared edit engine).
func (telegramBot *TelegramBotAPI) enterTogoEditFromMenu(owner int64, cb CallbackData, response *TelegramResponse) {
	telegramBot.enterEditFromMenu(owner, "togo", uint64(cb.ID), response)
}
