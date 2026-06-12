package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// ============================================================
// Daily midnight "port your leftovers" reminder.
//
// At 00:00 Asia/Tehran every user that has at least one undone togo (progress
// < 100) dated before today's midnight receives a single message: a prompt
// plus an inline button per undone togo. Tapping a button shifts that togo's
// date by +1 day (same time, next day) and rebuilds the keyboard with whatever
// remains. When the last one is ported the message becomes the success blurb.
//
// The keyboard is stateless — each button only needs the togo's id, and the
// reload uses Togo.LoadUndoneBefore(ownerID, Togo.StartOfToday()) so it
// survives bot restarts.
// ============================================================

// BuildPortReminderMessage formats the midnight prompt. When the togo list is
// empty it returns the "all ported" message + an empty keyboard so the
// previous buttons are cleared on edit.
func BuildPortReminderMessage(togos Togo.TogoList) (string, *tgbotapi.InlineKeyboardMarkup) {
	if len(togos) == 0 {
		return "✅ You've ported all your togos.", emptyInlineKeyboard()
	}

	buttons := make([]tgbotapi.InlineKeyboardButton, 0, len(togos))
	for i := range togos {
		label := togos[i].Title
		if desc := strings.TrimSpace(togos[i].Description); desc != "" {
			label = fmt.Sprintf("%s: %s", togos[i].Title, desc)
		}
		if len(label) >= MaximumInlineButtonTextLength {
			label = fmt.Sprintf("%s...", truncateUTF8(label, MaximumInlineButtonTextLength-3))
		}
		data := (CallbackData{Action: PortTogo, ID: int64(togos[i].Id)}).Json()
		buttons = append(buttons, tgbotapi.InlineKeyboardButton{Text: label, CallbackData: &data})
	}
	rows := packButtonsIntoRows(buttons, MaximumNumberOfRowItems)
	kb := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	return "🌙 These are the togos left from today. Which one do you want to port to tomorrow?", &kb
}

// RemindPortableTogos runs the daily midnight tick forever. It fires once when
// the local (Asia/Tehran) hour rolls over to 0, guarded by lastRun so a long
// uptime can't double-fire within the same calendar day.
func (telegramBot *TelegramBotAPI) RemindPortableTogos() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	lastRun := ""
	for range ticker.C {
		now := Togo.Today()
		today := now.Short()
		if now.Hour() == 0 && lastRun != today {
			lastRun = today
			telegramBot.processPortReminderTick()
		}
	}
}

// processPortReminderTick collects every owner with at least one undone togo
// dated before today's midnight, and sends each of them the port-reminder.
func (telegramBot *TelegramBotAPI) processPortReminderTick() {
	cutoff := Togo.StartOfToday()
	all, err := Togo.LoadEverybodysUndoneBefore(cutoff)
	if err != nil {
		log.Println("port reminder tick: load failed:", err.Error())
	}
	if all == nil {
		return
	}

	grouped := map[int64]Togo.TogoList{}
	for _, t := range all {
		grouped[t.OwnerId] = append(grouped[t.OwnerId], t)
	}
	for owner, togos := range grouped {
		if len(togos) == 0 {
			continue
		}
		text, kb := BuildPortReminderMessage(togos)
		telegramBot.SendTextMessage(TelegramResponse{TargetChatId: owner, TextMsg: text, InlineKeyboard: kb})
	}
}
