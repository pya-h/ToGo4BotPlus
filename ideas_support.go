package main

import (
	"fmt"

	"ToGo4BotPlus/Idea"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// IdeaInlineKeyboardMenu builds a paginated inline keyboard for the given ideas.
// Ideas have no "done" state, so the only item action is removal. Pagination
// reuses the shared menuPageCount/buildMenuNavRow helpers (see main.go). The
// removal menu always operates over the owner's full idea list, so no scope
// flags need to ride along in the callback data.
func IdeaInlineKeyboardMenu(ideas Idea.IdeaList, action UserAction, page int) *tgbotapi.InlineKeyboardMarkup {
	total := len(ideas)
	if total == 0 {
		return nil
	}

	totalPages := menuPageCount(total)
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * MaximumInlineMenuItems
	end := start + MaximumInlineMenuItems
	if end > total {
		end = total
	}
	pageItems := ideas[start:end]
	count := len(pageItems)

	rowsCount := count / MaximumNumberOfRowItems
	if count%MaximumNumberOfRowItems != 0 {
		rowsCount++
	}

	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: make([][]tgbotapi.InlineKeyboardButton, 0, rowsCount+1)}
	for r := 0; r < rowsCount; r++ {
		rowStart := r * MaximumNumberOfRowItems
		rowEnd := rowStart + MaximumNumberOfRowItems
		if rowEnd > count {
			rowEnd = count
		}
		buttons := make([]tgbotapi.InlineKeyboardButton, 0, rowEnd-rowStart)
		for k := rowStart; k < rowEnd; k++ {
			status := ""
			if pageItems[k].IsHighPriority {
				status = "🔴 "
			}
			title := fmt.Sprintf("%s%s", status, pageItems[k].Text)
			if len(title) >= MaximumInlineButtonTextLength {
				title = fmt.Sprintf("%s...", truncateUTF8(title, MaximumInlineButtonTextLength-3))
			}

			data := (CallbackData{Action: action, ID: int64(pageItems[k].Id), MenuPage: page}).Json()
			buttons = append(buttons, tgbotapi.InlineKeyboardButton{Text: title, CallbackData: &data})
		}
		menu.InlineKeyboard = append(menu.InlineKeyboard, buttons)
	}

	if navRow := buildMenuNavRow(ShowIdeaMenuPage, action, page, totalPages, CallbackData{}); navRow != nil {
		menu.InlineKeyboard = append(menu.InlineKeyboard, navRow)
	}

	return &menu
}

// BuildIdeaListReport renders a textual listing of ideas for the `;` command.
func BuildIdeaListReport(ideas Idea.IdeaList, onlyHighPriority bool, category string) string {
	scope := "All ideas"
	if onlyHighPriority {
		scope = "High-priority ideas"
	}
	if category != "" {
		scope = fmt.Sprintf("Ideas in %q", category)
	}

	if len(ideas) == 0 {
		return fmt.Sprintf("💡 %s\n\nNothing here yet.", scope)
	}

	parts := make([]string, 0, len(ideas)+1)
	parts = append(parts, fmt.Sprintf("💡 %s (%d):", scope, len(ideas)))
	for _, entry := range ideas.ToString() {
		parts = append(parts, entry)
	}
	return joinWithBlankLines(parts)
}

func joinWithBlankLines(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\n\n"
		}
		out += p
	}
	return out
}
