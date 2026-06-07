package main

import (
	"fmt"

	"ToGo4BotPlus/Article"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// ArticleInlineKeyboardMenu builds the paginated inline remove menu for the `>x`
// command, mirroring IdeaInlineKeyboardMenu. Articles have no other per-item
// action here, so the buttons just trigger removal.
func ArticleInlineKeyboardMenu(articles Article.ArticleList, action UserAction, page int) *tgbotapi.InlineKeyboardMarkup {
	total := len(articles)
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
	pageItems := articles[start:end]
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
			title := pageItems[k].Header()
			if len(title) >= MaximumInlineButtonTextLength {
				title = fmt.Sprintf("%s...", truncateUTF8(title, MaximumInlineButtonTextLength-3))
			}
			data := (CallbackData{Action: action, ID: int64(pageItems[k].Id), MenuPage: page}).Json()
			buttons = append(buttons, tgbotapi.InlineKeyboardButton{Text: title, CallbackData: &data})
		}
		menu.InlineKeyboard = append(menu.InlineKeyboard, buttons)
	}

	if navRow := buildMenuNavRow(ShowArticleMenuPage, action, page, totalPages, CallbackData{}); navRow != nil {
		menu.InlineKeyboard = append(menu.InlineKeyboard, navRow)
	}

	return &menu
}

// BuildArticleListReport renders a textual listing of articles for the `>l`
// command.
func BuildArticleListReport(articles Article.ArticleList, category string) string {
	scope := "All articles"
	if category != "" {
		scope = fmt.Sprintf("Articles in %q", category)
	}

	if len(articles) == 0 {
		return fmt.Sprintf("🔗 %s\n\nNothing here yet.", scope)
	}

	parts := make([]string, 0, len(articles)+1)
	parts = append(parts, fmt.Sprintf("🔗 %s (%d):", scope, len(articles)))
	parts = append(parts, articles.ToString()...)
	return joinWithBlankLines(parts)
}
