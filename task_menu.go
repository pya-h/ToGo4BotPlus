package main

import (
	"fmt"

	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// ============================================================
// Interactive task browser (the single rich task menu, opened with /tasks).
//
// Mirrors the togo browser: stateless, with a Toggle in the detail view since
// tasks have a done state. Editing hands the message off to the shared edit
// screens (see TaskMenuEdit). The list includes inactive + completed tasks so
// everything the owner has is reachable.
// ============================================================

const TasksPerMenuPage = 30 // tasks shown per browser page before pagination kicks in

// loadTasksForBrowse loads all of the owner's tasks (active + inactive +
// completed) so the browser shows everything.
func loadTasksForBrowse(ownerID int64) (Task.TaskList, error) {
	return Task.Load(ownerID, true, true)
}

// taskButtonRows builds one inline button per task ("#id: title"), packed several
// to a row, each opening that task's detail view.
func taskButtonRows(tasks Task.TaskList) [][]tgbotapi.InlineKeyboardButton {
	buttons := make([]tgbotapi.InlineKeyboardButton, 0, len(tasks))
	for i := range tasks {
		status := ""
		if tasks[i].Progress >= 100 {
			status = "✅ "
		}
		label := fmt.Sprintf("#%d: %s%s", tasks[i].Id, status, tasks[i].Title)
		if len(label) >= MaximumInlineButtonTextLength {
			label = fmt.Sprintf("%s...", truncateUTF8(label, MaximumInlineButtonTextLength-3))
		}
		data := (CallbackData{Action: TaskMenuOpen, ID: int64(tasks[i].Id)}).Json()
		buttons = append(buttons, tgbotapi.InlineKeyboardButton{Text: label, CallbackData: &data})
	}
	return packButtonsIntoRows(buttons, MaximumNumberOfRowItems)
}

// taskListLine renders one message line: "#id [✅] title".
func taskListLine(task Task.Task) string {
	status := ""
	if task.Progress >= 100 {
		status = "✅ "
	}
	return fmt.Sprintf("#%d %s%s", task.Id, status, task.Title)
}

// renderTaskList builds the list view (message text + paginated inline menu).
func renderTaskList(tasks Task.TaskList, page int) (string, *tgbotapi.InlineKeyboardMarkup) {
	total := len(tasks)
	title := "^ Your tasks"
	if total == 0 {
		return fmt.Sprintf("%s\n\nNothing here yet.", title), nil
	}

	totalPages := (total + TasksPerMenuPage - 1) / TasksPerMenuPage
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 0 {
		page = 0
	}
	if page >= totalPages {
		page = totalPages - 1
	}

	start := page * TasksPerMenuPage
	end := start + TasksPerMenuPage
	if end > total {
		end = total
	}
	pageItems := tasks[start:end]

	text := fmt.Sprintf("%s — all %d so far. Tap any item below to open and manage it:\n", title, total)
	for i := range pageItems {
		text += "\n" + taskListLine(pageItems[i])
	}

	rows := taskButtonRows(pageItems)
	if nav := taskListNavRow(page, totalPages); nav != nil {
		rows = append(rows, nav)
	}
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	return text, &menu
}

// taskListNavRow builds the ⬅️ Prev / page / Next ➡️ row (nil for a single page).
func taskListNavRow(page int, totalPages int) []tgbotapi.InlineKeyboardButton {
	if totalPages <= 1 {
		return nil
	}
	row := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	if page > 0 {
		prev := (CallbackData{Action: TaskMenuList, MenuPage: page - 1}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prev})
	}
	indicator := (CallbackData{Action: TaskMenuList, MenuPage: page}).Json()
	row = append(row, tgbotapi.InlineKeyboardButton{Text: fmt.Sprintf("%d/%d", page+1, totalPages), CallbackData: &indicator})
	if page < totalPages-1 {
		next := (CallbackData{Action: TaskMenuList, MenuPage: page + 1}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &next})
	}
	return row
}

// renderTaskDetail builds the detail view for one task. Returns ok=false when the
// task is no longer in the list so the caller can fall back to the list.
func renderTaskDetail(tasks Task.TaskList, taskID uint64) (string, *tgbotapi.InlineKeyboardMarkup, bool) {
	idx := tasks.Index(taskID)
	if idx < 0 {
		return "", nil, false
	}
	task := tasks[idx]
	page := idx / TasksPerMenuPage

	toggleLabel := "✅ Mark done"
	if task.Progress >= 100 {
		toggleLabel = "↩️ Mark undone"
	}
	remove := (CallbackData{Action: TaskMenuRemove, ID: int64(task.Id), MenuPage: page}).Json()
	toggle := (CallbackData{Action: TaskMenuToggle, ID: int64(task.Id), MenuPage: page}).Json()
	edit := (CallbackData{Action: TaskMenuEdit, ID: int64(task.Id)}).Json()
	actionRow := []tgbotapi.InlineKeyboardButton{
		{Text: "🗑 Remove", CallbackData: &remove},
		{Text: toggleLabel, CallbackData: &toggle},
		{Text: "✏️ Edit", CallbackData: &edit},
	}

	navRow := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	if idx > 0 {
		prev := (CallbackData{Action: TaskMenuOpen, ID: int64(tasks[idx-1].Id)}).Json()
		navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prev})
	}
	menu := (CallbackData{Action: TaskMenuList, MenuPage: page}).Json()
	navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "🔙 Menu", CallbackData: &menu})
	if idx < len(tasks)-1 {
		next := (CallbackData{Action: TaskMenuOpen, ID: int64(tasks[idx+1].Id)}).Json()
		navRow = append(navRow, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &next})
	}

	text := fmt.Sprintf("%s\n\n(%d of %d)", task.ToString(Togo.Today().Time), idx+1, len(tasks))
	kb := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{actionRow, navRow}}
	return text, &kb, true
}

// handleTaskMenuCallback drives the stateless task browser.
func (telegramBot *TelegramBotAPI) handleTaskMenuCallback(cb CallbackData, response *TelegramResponse) {
	owner := response.TargetChatId

	switch cb.Action {
	case TaskMenuList:
		tasks, warning := loadTasksForBrowse(owner)
		text, kb := renderTaskList(tasks, cb.MenuPage)
		response.TextMsg = appendWarning(text, warning)
		response.InlineKeyboard = orEmptyKeyboard(kb)

	case TaskMenuOpen:
		tasks, warning := loadTasksForBrowse(owner)
		response.TextMsg, response.InlineKeyboard = taskDetailOrList(tasks, uint64(cb.ID), cb.MenuPage, warning)

	case TaskMenuToggle:
		tasks, _ := loadTasksForBrowse(owner)
		if task, err := tasks.Get(uint64(cb.ID)); err == nil {
			if task.Progress < 100 {
				task.Progress = 100
			} else {
				task.Progress = 0
			}
			if updateErr := task.Update(owner); updateErr != nil {
				response.TextMsg = updateErr.Error()
				return
			}
		}
		tasks, warning := loadTasksForBrowse(owner)
		response.TextMsg, response.InlineKeyboard = taskDetailOrList(tasks, uint64(cb.ID), cb.MenuPage, warning)

	case TaskMenuRemove:
		before, _ := loadTasksForBrowse(owner)
		updated, err := before.Remove(owner, uint64(cb.ID))
		if err != nil {
			after, _ := loadTasksForBrowse(owner)
			if len(after) == len(before) {
				response.TextMsg = err.Error()
				return
			}
			updated = after
		}
		text, kb := renderTaskList(updated, cb.MenuPage)
		response.TextMsg = "🗑 Removed.\n\n" + text
		response.InlineKeyboard = orEmptyKeyboard(kb)

	case TaskMenuEdit:
		telegramBot.enterTaskEditFromMenu(owner, cb, response)
	}
}

// taskDetailOrList renders the task's detail view, or falls back to the list when
// the task is gone. It always returns a non-nil keyboard so an edit clears any
// stale buttons.
func taskDetailOrList(tasks Task.TaskList, taskID uint64, page int, warning error) (string, *tgbotapi.InlineKeyboardMarkup) {
	if text, kb, ok := renderTaskDetail(tasks, taskID); ok {
		return appendWarning(text, warning), orEmptyKeyboard(kb)
	}
	text, kb := renderTaskList(tasks, page)
	return appendWarning(text, warning), orEmptyKeyboard(kb)
}

// enterTaskEditFromMenu turns the browser message into the edit card for the
// selected task (so typed/tapped edits route through the shared edit engine).
func (telegramBot *TelegramBotAPI) enterTaskEditFromMenu(owner int64, cb CallbackData, response *TelegramResponse) {
	telegramBot.enterEditFromMenu(owner, "task", uint64(cb.ID), response)
}
