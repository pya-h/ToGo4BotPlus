package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var taskReminderLoadProblemNotified bool

func BuildTaskProgressReport(tasks Task.TaskList, includeInactive bool, warning error) string {
	scope := "Active tasks"
	if includeInactive {
		scope = "Active + inactive tasks"
	}

	progress, completedInPercent, completed, extra, total := tasks.ProgressMade()
	inactive := 0
	for _, task := range tasks {
		if !task.IsActive(Togo.Today().Time) {
			inactive++
		}
	}

	report := fmt.Sprintf(
		"%s Progress: %3.2f%%\n%3.2f%% Completed\nStatistics: %d / %d\nInactive in list: %d",
		scope,
		progress,
		completedInPercent,
		completed,
		total,
		inactive,
	)
	if extra > 0 {
		report = fmt.Sprintf("%s\n[+%d extras]", report, extra)
	}
	if warning != nil {
		report = fmt.Sprintf("%s\nwarning: %s", report, warning.Error())
	}
	return report
}

func BuildTogoProgressReport(togos Togo.TogoList, allDays bool, warning error) string {
	progress, completedInPercent, completed, extra, total := togos.ProgressMade()
	scope := "Today's"
	if allDays {
		scope = "Total"
	}
	text := fmt.Sprintf("%s Progress: %3.2f%% \n%3.2f%% Completed\nStatistics: %d / %d\n",
		scope, progress, completedInPercent, completed, total)
	if extra > 0 {
		text = fmt.Sprintf("%s[+%d]\n", text, extra)
	}
	if warning != nil {
		text = fmt.Sprintf("%s- - - - - - - - - - - - - - - - - - - - - - \nwarning: %s", text, warning.Error())
	}
	return text
}

func BuildTaskPages(tasks Task.TaskList, includeInactive bool, reminderMode bool, maxBytes int) []string {
	if maxBytes <= 120 {
		maxBytes = MaximumTaskMessageLength
	}

	now := Togo.Today().Time
	header := "📋 Active Tasks"
	if includeInactive {
		header = "📋 Tasks (active + inactive)"
	}
	if reminderMode {
		header = "⏰ Task Reminder (active tasks)"
	}

	if len(tasks) == 0 {
		return []string{fmt.Sprintf("%s\n\nNo tasks to show.", header)}
	}

	pages := make([]string, 0)
	current := header
	for _, task := range tasks {
		entry := task.ToString(now)
		if len(entry) > maxBytes/2 {
			entry = fmt.Sprintf("%s...", truncateUTF8(entry, maxBytes/2-3))
		}

		candidate := fmt.Sprintf("%s\n\n%s", current, entry)
		if len(candidate) > maxBytes && current != header {
			pages = append(pages, current)
			current = fmt.Sprintf("%s\n\n%s", header, entry)
			if len(current) > maxBytes {
				trimLimit := maxBytes - len(header) - 10
				if trimLimit < 32 {
					trimLimit = 32
				}
				current = fmt.Sprintf("%s\n\n%s...", header, truncateUTF8(entry, trimLimit))
			}
			continue
		}
		current = candidate
	}
	pages = append(pages, current)

	total := len(pages)
	for i := range pages {
		footer := fmt.Sprintf("\n\nPage %d/%d", i+1, total)
		if len(pages[i])+len(footer) > maxBytes {
			trimLimit := maxBytes - len(footer) - 3
			if trimLimit < 32 {
				trimLimit = 32
			}
			pages[i] = fmt.Sprintf("%s...", truncateUTF8(pages[i], trimLimit))
		}
		pages[i] += footer
	}
	return pages
}

func TaskPageNavigationKeyboard(page int, total int, includeInactive bool, reminderMode bool) *tgbotapi.InlineKeyboardMarkup {
	if total <= 1 {
		return nil
	}
	if page < 0 {
		page = 0
	}
	if page >= total {
		page = total - 1
	}

	row := make([]tgbotapi.InlineKeyboardButton, 0)
	if page > 0 {
		prevData := (CallbackData{Action: ShowTaskPage, TaskPage: page - 1, TaskIncludeInactive: includeInactive, TaskReminderMode: reminderMode}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prevData})
	}
	if page < total-1 {
		nextData := (CallbackData{Action: ShowTaskPage, TaskPage: page + 1, TaskIncludeInactive: includeInactive, TaskReminderMode: reminderMode}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &nextData})
	}
	if len(row) == 0 {
		return nil
	}
	menu := tgbotapi.NewInlineKeyboardMarkup(row)
	return &menu
}

func TaskInlineKeyboardMenu(tasks Task.TaskList, action UserAction, includeInactive bool, page int) *tgbotapi.InlineKeyboardMarkup {
	total := len(tasks)
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
	pageItems := tasks[start:end]
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
			if pageItems[k].Progress >= 100 {
				status = "✅ "
			}
			title := fmt.Sprintf("%s%s", status, pageItems[k].Title)
			if len(title) >= MaximumInlineButtonTextLength {
				title = fmt.Sprintf("%s...", truncateUTF8(title, MaximumInlineButtonTextLength-3))
			}

			data := (CallbackData{Action: action, ID: int64(pageItems[k].Id), TaskIncludeInactive: includeInactive, MenuPage: page}).Json()
			buttons = append(buttons, tgbotapi.InlineKeyboardButton{Text: title, CallbackData: &data})
		}
		menu.InlineKeyboard = append(menu.InlineKeyboard, buttons)
	}

	if navRow := buildMenuNavRow(ShowTaskMenuPage, action, page, totalPages, CallbackData{TaskIncludeInactive: includeInactive}); navRow != nil {
		menu.InlineKeyboard = append(menu.InlineKeyboard, navRow)
	}

	return &menu
}

func taskReminderSlot(now time.Time, remindersPerDay int) (string, bool) {
	if !Task.IsValidReminderTimes(remindersPerDay) {
		return "", false
	}
	if remindersPerDay == 0 {
		return "", false
	}
	if now.Minute() != 0 {
		return "", false
	}

	interval := 24 / remindersPerDay
	if interval <= 0 || now.Hour()%interval != 0 {
		return "", false
	}

	return now.Format("2006-01-02-15"), true
}

func allowedTaskReminderValuesText() string {
	vals := Task.AllowedReminderTimes()
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		parts = append(parts, fmt.Sprint(v))
	}
	return strings.Join(parts, ", ")
}

// pickWeightedRandomTasks samples up to n tasks without replacement, weighted by
// each task's Weight (so a higher-weight task is more likely to be surfaced).
// Weight 0 is treated as 1 so a misconfigured task can still be picked.
func pickWeightedRandomTasks(tasks Task.TaskList, n int) Task.TaskList {
	if n <= 0 || len(tasks) == 0 {
		return Task.TaskList{}
	}
	pool := make(Task.TaskList, len(tasks))
	copy(pool, tasks)

	if n > len(pool) {
		n = len(pool)
	}
	picked := make(Task.TaskList, 0, n)

	for len(picked) < n && len(pool) > 0 {
		totalWeight := 0
		for _, t := range pool {
			w := int(t.Weight)
			if w < 1 {
				w = 1
			}
			totalWeight += w
		}
		// totalWeight is always >=1 here (pool non-empty, each weight clamped to >=1).
		target := rand.Intn(totalWeight)
		for i, t := range pool {
			w := int(t.Weight)
			if w < 1 {
				w = 1
			}
			target -= w
			if target < 0 {
				picked = append(picked, t)
				pool = append(pool[:i], pool[i+1:]...)
				break
			}
		}
	}
	return picked
}

// BuildTaskReminderMessage formats the periodic task reminder: a short header
// stating how many ongoing tasks are surfaced, each picked task rendered in
// full, plus an inline button that opens the same view as /tasks.
func BuildTaskReminderMessage(picked Task.TaskList) (string, *tgbotapi.InlineKeyboardMarkup) {
	openMenuData := (CallbackData{Action: TaskMenuList, MenuPage: 0}).Json()
	openMenuButton := []tgbotapi.InlineKeyboardButton{{Text: "📋 Open tasks menu", CallbackData: &openMenuData}}
	kb := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{openMenuButton}}

	if len(picked) == 0 {
		return "⏰ Task reminder — no ongoing tasks right now. Tap the button to open your tasks menu.", &kb
	}

	header := fmt.Sprintf("⏰ Task reminder — here are %d of your ongoing tasks:", len(picked))
	now := Togo.Today().Time
	parts := make([]string, 0, len(picked)+1)
	parts = append(parts, header)
	for i := range picked {
		parts = append(parts, picked[i].ToString(now))
	}
	return strings.Join(parts, "\n\n"), &kb
}

func (telegramBot *TelegramBotAPI) processTaskReminderTick(now Togo.Date) {
	owners, err := Task.LoadActiveOwners(now.Time)
	if err != nil {
		if !taskReminderLoadProblemNotified {
			taskReminderLoadProblemNotified = true
			telegramBot.InformAdmin(fmt.Sprintf("%s failed loading active owners: %v", TaskReminderWarningPrefix, err))
		}
		return
	}
	taskReminderLoadProblemNotified = false

	for _, ownerID := range owners {
		setting, err := Task.GetReminderSetting(ownerID)
		if err != nil {
			log.Printf("%s failed loading setting for owner %d: %v", TaskReminderWarningPrefix, ownerID, err)
			continue
		}

		slot, due := taskReminderSlot(now.Time, setting.RemindersPerDay)
		if !due || slot == setting.LastReminderSlot {
			continue
		}

		tasks, warning := Task.Load(ownerID, false, false)
		if tasks == nil {
			log.Printf("%s failed loading tasks for owner %d: %v", TaskReminderWarningPrefix, ownerID, warning)
			continue
		}
		if len(tasks) == 0 {
			if err := Task.UpdateLastReminderSlot(ownerID, slot); err != nil {
				log.Printf("%s failed updating empty-slot marker for owner %d: %v", TaskReminderWarningPrefix, ownerID, err)
			}
			continue
		}

		picked := pickWeightedRandomTasks(tasks, setting.TasksPerReminder)
		text, kb := BuildTaskReminderMessage(picked)
		telegramBot.SendTextMessage(TelegramResponse{TargetChatId: ownerID, TextMsg: text, InlineKeyboard: kb})

		if warning != nil {
			log.Printf("%s owner %d load warning: %v", TaskReminderWarningPrefix, ownerID, warning)
		}
		if err := Task.UpdateLastReminderSlot(ownerID, slot); err != nil {
			log.Printf("%s failed updating slot for owner %d: %v", TaskReminderWarningPrefix, ownerID, err)
		}
	}
}
