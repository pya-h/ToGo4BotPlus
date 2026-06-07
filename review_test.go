package main

import (
	"strings"
	"testing"

	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// TestSendTextMessageFallsBackToPlainOnMarkdownError verifies that when Telegram
// rejects a message because the (user-supplied) content contains unbalanced
// Markdown, SendTextMessage transparently retries the same content as plain
// text instead of silently dropping it.
func TestSendTextMessageFallsBackToPlainOnMarkdownError(t *testing.T) {
	bot, transport := newRecordingBot(t)
	transport.failParseModeOnce = true

	bot.SendTextMessage(TelegramResponse{TargetChatId: 42, TextMsg: "buy milk_and *eggs"})

	if got := transport.countEndpoint("sendMessage"); got != 2 {
		t.Fatalf("expected 2 sendMessage attempts (markdown + plain retry), got %d", got)
	}
	var sends []capturedRequest
	for _, r := range transport.requests {
		if r.Endpoint == "sendMessage" {
			sends = append(sends, r)
		}
	}
	if sends[0].Values.Get("parse_mode") == "" {
		t.Fatalf("expected first attempt to use Markdown parse_mode")
	}
	last, _ := transport.lastEndpoint("sendMessage")
	if last.Values.Get("parse_mode") != "" {
		t.Fatalf("expected retry to drop parse_mode, got %q", last.Values.Get("parse_mode"))
	}
	if last.Values.Get("text") != "buy milk_and *eggs" {
		t.Fatalf("expected retry to preserve original text, got %q", last.Values.Get("text"))
	}
}

// TestSendTextMessageNoRetryOnSuccess ensures the happy path is unchanged: a
// successful Markdown send must not trigger a second request.
func TestSendTextMessageNoRetryOnSuccess(t *testing.T) {
	bot, transport := newRecordingBot(t)

	bot.SendTextMessage(TelegramResponse{TargetChatId: 42, TextMsg: "plain hello"})

	if got := transport.countEndpoint("sendMessage"); got != 1 {
		t.Fatalf("expected exactly 1 sendMessage on success, got %d", got)
	}
}

// TestCallbackWithNilMessageDoesNotPanic guards the case where Telegram delivers
// a callback whose originating Message is nil (too old / inline result). The bot
// must ignore it quietly rather than panic into the recovery handler.
func TestCallbackWithNilMessageDoesNotPanic(t *testing.T) {
	bot, transport := newRecordingBot(t)

	// A flow-action callback (routed to handleFlowCallback) ...
	bot.HandleUpdate(tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		Data: (CallbackData{Action: FlowConfirm}).Json(), Message: nil,
	}})
	// ... and a regular menu callback (routed to handleCallbackUpdate).
	bot.HandleUpdate(tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		Data: (CallbackData{Action: RemoveTogo, ID: 1}).Json(), Message: nil,
	}})

	if got := transport.countEndpoint("editMessageText"); got != 0 {
		t.Fatalf("expected no edits for nil-Message callbacks, got %d", got)
	}
	if got := transport.countEndpoint("sendMessage"); got != 0 {
		t.Fatalf("expected no sends for nil-Message callbacks, got %d", got)
	}
}

// TestManageTogoFlowDelete exercises the togo edit-card delete path end-to-end
// (reached from the togo browser's Edit button — a destructive operation).
func TestManageTogoFlowDelete(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9500)
	id := seedTogo(t, chatID, "delete me", 0)

	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1501,
		(CallbackData{Action: TogoMenuEdit, ID: int64(id)}).Json()) // open card
	confirm := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1502,
		(CallbackData{Action: FlowDelete}).Json())
	if !strings.Contains(confirm, "Delete this togo") {
		t.Fatalf("expected delete confirmation, got %q", confirm)
	}
	deleted := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1503,
		(CallbackData{Action: FlowConfirm}).Json())
	if !strings.Contains(deleted, "Deleted") {
		t.Fatalf("expected delete result, got %q", deleted)
	}

	togos, _ := Togo.Load(chatID, false, false)
	if len(togos) != 0 {
		t.Fatalf("expected 0 togos after delete, got %d", len(togos))
	}
}

// TestManageTaskFlowToggle exercises the task edit-card toggle-done path
// end-to-end (reached from the task browser's Edit button).
func TestManageTaskFlowToggle(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9501)
	id := seedTask(t, chatID, "toggle me", 0)

	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1511,
		(CallbackData{Action: TaskMenuEdit, ID: int64(id)}).Json()) // open card
	toggled := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1512,
		(CallbackData{Action: FlowToggle}).Json())
	if !strings.Contains(toggled, "Toggled") {
		t.Fatalf("expected toggle confirmation, got %q", toggled)
	}

	tasks, _ := Task.Load(chatID, true, true)
	got, _ := tasks.Get(id)
	if got.Progress != 100 {
		t.Fatalf("expected progress 100 after toggle, got %d", got.Progress)
	}

	// Toggling again should flip it back to 0.
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1513,
		(CallbackData{Action: FlowToggle}).Json())
	tasks, _ = Task.Load(chatID, true, true)
	got, _ = tasks.Get(id)
	if got.Progress != 0 {
		t.Fatalf("expected progress 0 after second toggle, got %d", got.Progress)
	}
}

// TestValidateNonNegativeInt covers the validator used by the togo "day" step,
// where 0 (today) must be accepted even though weight/duration reject it.
func TestValidateNonNegativeInt(t *testing.T) {
	if err := validateNonNegativeInt("0"); err != nil {
		t.Fatalf("0 should be valid (today), got %v", err)
	}
	if err := validateNonNegativeInt("7"); err != nil {
		t.Fatalf("7 should be valid, got %v", err)
	}
	if err := validateNonNegativeInt("-1"); err == nil {
		t.Fatal("-1 should be rejected")
	}
	if err := validateNonNegativeInt("x"); err == nil {
		t.Fatal("non-numeric should be rejected")
	}
}

// TestAddTogoFlowAcceptsTypedDayZero drives the add-togo wizard to the "day"
// step, picks Custom, and types "0"; it must advance rather than reject, since
// 0 = today is a valid schedule.
func TestAddTogoFlowAcceptsTypedDayZero(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9502)

	startFlowGetSend(t, bot, transport, chatID, 1520, "/addTogo")
	sendFlowTextGetEdit(t, bot, transport, chatID, 1521, "groceries") // title -> weight
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1521,     // skip weight
		(CallbackData{Action: FlowSkip}).Json())
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1521, // skip progress
		(CallbackData{Action: FlowSkip}).Json())
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1521, // extra -> Normal (index 0)
		(CallbackData{Action: FlowSelect, FlowOpt: 0}).Json())
	// Now on the day step. Choose Custom, then type "0".
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1521,
		(CallbackData{Action: FlowCustom}).Json())
	next := sendFlowTextGetEdit(t, bot, transport, chatID, 1522, "0")
	if strings.Contains(next, "positive whole number") || strings.Contains(next, "enter 0 or") {
		t.Fatalf("typed day 0 should be accepted, got rejection: %q", next)
	}
	// Advancing past the day step lands on the time step.
	if !strings.Contains(next, "time") {
		t.Fatalf("expected to advance to the time step after day 0, got %q", next)
	}
}
