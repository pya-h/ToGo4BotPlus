package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"ToGo4BotPlus/Article"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// ============================================================
// Interactive article browser (the "rich" article menu) + the daily article
// reminder. This mirrors the idea browser (idea_menu.go) but is simpler:
// articles have no priority or favorite, only a title, a category and a url.
// The browser is stateless (state encoded in callback_data); editing is the one
// stateful action, handed off to the existing manage flow.
// ============================================================

const ArticlesPerMenuPage = 10 // articles shown per browser page before pagination

// loadArticlesForScope loads the owner's articles, optionally filtered by
// category id (0 = all).
func loadArticlesForScope(ownerID int64, categoryID int64) (Article.ArticleList, error) {
	return Article.Load(ownerID, categoryID)
}

// articleListLine renders one message line: "#id Category: header".
func articleListLine(article Article.Article) string {
	category := article.Category
	if category == "" {
		category = "—"
	}
	return fmt.Sprintf("#%d %s: %s", article.Id, category, article.Header())
}

// articleButtonRows builds one inline button per article ("#id: header"), each
// opening that article's detail view.
func articleButtonRows(articles Article.ArticleList, categoryID int64) [][]tgbotapi.InlineKeyboardButton {
	buttons := make([]tgbotapi.InlineKeyboardButton, 0, len(articles))
	for i := range articles {
		label := fmt.Sprintf("#%d: %s", articles[i].Id, articles[i].Header())
		if len(label) >= MaximumInlineButtonTextLength {
			label = fmt.Sprintf("%s...", truncateUTF8(label, MaximumInlineButtonTextLength-3))
		}
		data := (CallbackData{Action: ArticleMenuOpen, ID: int64(articles[i].Id), ArtCat: categoryID}).Json()
		buttons = append(buttons, tgbotapi.InlineKeyboardButton{Text: label, CallbackData: &data})
	}
	return packButtonsIntoRows(buttons, MaximumNumberOfRowItems)
}

// renderArticleList builds the list view (message text + paginated inline menu).
func renderArticleList(articles Article.ArticleList, categoryID int64, page int) (string, *tgbotapi.InlineKeyboardMarkup) {
	total := len(articles)
	title := "🔗 Your articles"
	if total == 0 {
		return fmt.Sprintf("%s\n\nNothing here yet.", title), nil
	}

	totalPages := (total + ArticlesPerMenuPage - 1) / ArticlesPerMenuPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * ArticlesPerMenuPage
	end := start + ArticlesPerMenuPage
	if end > total {
		end = total
	}
	pageItems := articles[start:end]

	text := fmt.Sprintf("%s (%d):\n", title, total)
	for i := range pageItems {
		text += "\n" + articleListLine(pageItems[i])
	}

	rows := articleButtonRows(pageItems, categoryID)
	if nav := articleListNavRow(categoryID, page, totalPages); nav != nil {
		rows = append(rows, nav)
	}
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	return text, &menu
}

// articleListNavRow builds the ⬅️ Prev / page / Next ➡️ row (nil for one page).
func articleListNavRow(categoryID int64, page int, totalPages int) []tgbotapi.InlineKeyboardButton {
	if totalPages <= 1 {
		return nil
	}
	row := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	if page > 0 {
		prev := (CallbackData{Action: ArticleMenuList, ArtCat: categoryID, MenuPage: page - 1}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prev})
	}
	indicator := (CallbackData{Action: ArticleMenuList, ArtCat: categoryID, MenuPage: page}).Json()
	row = append(row, tgbotapi.InlineKeyboardButton{Text: fmt.Sprintf("%d/%d", page+1, totalPages), CallbackData: &indicator})
	if page < totalPages-1 {
		next := (CallbackData{Action: ArticleMenuList, ArtCat: categoryID, MenuPage: page + 1}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &next})
	}
	return row
}

// renderArticleDetail builds the detail view for one article. Returns ok=false
// when the article is no longer in the list so the caller can fall back.
func renderArticleDetail(articles Article.ArticleList, categoryID int64, articleID uint64) (string, *tgbotapi.InlineKeyboardMarkup, bool) {
	idx := articles.Index(articleID)
	if idx < 0 {
		return "", nil, false
	}
	article := articles[idx]
	page := idx / ArticlesPerMenuPage

	remove := (CallbackData{Action: ArticleMenuRemove, ID: int64(article.Id), ArtCat: categoryID, MenuPage: page}).Json()
	edit := (CallbackData{Action: ArticleMenuEdit, ID: int64(article.Id)}).Json()
	actionRow := []tgbotapi.InlineKeyboardButton{
		{Text: "🗑 Remove", CallbackData: &remove},
		{Text: "✏️ Edit", CallbackData: &edit},
	}

	navRow := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	if idx > 0 {
		prev := (CallbackData{Action: ArticleMenuOpen, ID: int64(articles[idx-1].Id), ArtCat: categoryID}).Json()
		navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prev})
	}
	menu := (CallbackData{Action: ArticleMenuList, ArtCat: categoryID, MenuPage: page}).Json()
	navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "🔙 Menu", CallbackData: &menu})
	if idx < len(articles)-1 {
		next := (CallbackData{Action: ArticleMenuOpen, ID: int64(articles[idx+1].Id), ArtCat: categoryID}).Json()
		navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &next})
	}

	text := fmt.Sprintf("%s\n\n(%d of %d)", article.ToString(), idx+1, len(articles))
	kb := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{actionRow, navRow}}
	return text, &kb, true
}

// handleArticleMenuCallback drives the stateless article browser.
func (telegramBot *TelegramBotAPI) handleArticleMenuCallback(cb CallbackData, response *TelegramResponse) {
	owner := response.TargetChatId

	switch cb.Action {
	case ArticleMenuList:
		articles, warning := loadArticlesForScope(owner, cb.ArtCat)
		text, kb := renderArticleList(articles, cb.ArtCat, cb.MenuPage)
		response.TextMsg = appendWarning(text, warning)
		response.InlineKeyboard = orEmptyKeyboard(kb)

	case ArticleMenuOpen:
		articles, warning := loadArticlesForScope(owner, cb.ArtCat)
		response.TextMsg, response.InlineKeyboard = articleDetailOrList(articles, cb, uint64(cb.ID), warning)

	case ArticleMenuRemove:
		before, _ := loadArticlesForScope(owner, cb.ArtCat)
		updated, err := before.Remove(owner, uint64(cb.ID))
		if err != nil {
			after, _ := loadArticlesForScope(owner, cb.ArtCat)
			if len(after) == len(before) {
				response.TextMsg = err.Error()
				return
			}
			updated = after
		}
		text, kb := renderArticleList(updated, cb.ArtCat, cb.MenuPage)
		response.TextMsg = "🗑 Removed.\n\n" + text
		response.InlineKeyboard = orEmptyKeyboard(kb)

	case ArticleMenuEdit:
		telegramBot.enterArticleEditFromMenu(owner, cb, response)
	}
}

// articleDetailOrList renders the detail view or falls back to the list, always
// returning a non-nil keyboard so an edit clears stale buttons.
func articleDetailOrList(articles Article.ArticleList, cb CallbackData, articleID uint64, warning error) (string, *tgbotapi.InlineKeyboardMarkup) {
	if text, kb, ok := renderArticleDetail(articles, cb.ArtCat, articleID); ok {
		return appendWarning(text, warning), orEmptyKeyboard(kb)
	}
	text, kb := renderArticleList(articles, cb.ArtCat, cb.MenuPage)
	return appendWarning(text, warning), orEmptyKeyboard(kb)
}

// enterArticleEditFromMenu turns the browser message into the edit card for the
// selected article (so typed edits route through the shared edit engine).
func (telegramBot *TelegramBotAPI) enterArticleEditFromMenu(owner int64, cb CallbackData, response *TelegramResponse) {
	telegramBot.enterEditFromMenu(owner, "article", uint64(cb.ID), response)
}

// ---------------------- Daily article reminder --------------------------------

// RemindArticles fires the daily article reminder. It checks each minute and
// triggers once when the local (Asia/Tehran) hour matches ArticleReminderHour,
// guarding so it runs at most once per day.
func (telegramBot *TelegramBotAPI) RemindArticles() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	lastRun := ""
	for range ticker.C {
		now := Togo.Today()
		today := now.Short()
		if now.Hour() == ArticleReminderHour && lastRun != today {
			lastRun = today
			telegramBot.processArticleReminderTick()
		}
	}
}

// processArticleReminderTick sends every owner that has at least one article a
// single, randomly chosen one of theirs.
func (telegramBot *TelegramBotAPI) processArticleReminderTick() {
	owners, err := Article.LoadOwnersWithArticles()
	if err != nil {
		log.Println("article reminder tick: could not load owners:", err.Error())
		return
	}
	for _, owner := range owners {
		articles, loadErr := Article.Load(owner, 0)
		if loadErr != nil {
			log.Println("article reminder tick: load articles failed:", loadErr.Error())
		}
		if len(articles) == 0 {
			continue
		}
		pick := articles[rand.Intn(len(articles))]
		telegramBot.sendArticleReminder(owner, pick)
	}
}

// sendArticleReminder sends one article. The url sits on its own line so Telegram
// renders its link preview (and an Instant View for supported sites). It is sent
// as plain text (no Markdown) so urls containing _ * etc. are not mangled and the
// preview resolves correctly.
func (telegramBot *TelegramBotAPI) sendArticleReminder(ownerID int64, article Article.Article) {
	text := fmt.Sprintf("🔗 An article worth revisiting:\n\n%s", article.Title)
	if url := article.Url; url != "" {
		text += "\n" + url
	}
	telegramBot.SendTextMessageReturningID(TelegramResponse{TargetChatId: ownerID, TextMsg: text})
}
