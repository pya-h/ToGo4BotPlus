package main

import (
	"fmt"
	"strings"
	"testing"

	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func sendTextUpdateAndGetLastText(t *testing.T, bot *TelegramBotAPI, transport *recordingTransport, chatID int64, messageID int, text string) string {
	t.Helper()
	before := transport.countEndpoint("sendMessage")

	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: messageID,
		Text:      text,
		Chat:      &tgbotapi.Chat{ID: chatID},
	}})

	after := transport.countEndpoint("sendMessage")
	if after <= before {
		t.Fatalf("expected sendMessage for input %q", text)
	}
	req, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatalf("expected last sendMessage request for input %q", text)
	}
	return req.Values.Get("text")
}

func sendCallbackAndGetEditedText(t *testing.T, bot *TelegramBotAPI, transport *recordingTransport, chatID int64, messageID int, data string) string {
	t.Helper()
	before := transport.countEndpoint("editMessageText")

	bot.HandleUpdate(tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		Data: data,
		Message: &tgbotapi.Message{
			MessageID: messageID,
			Chat:      &tgbotapi.Chat{ID: chatID},
		},
	}})

	after := transport.countEndpoint("editMessageText")
	if after <= before {
		t.Fatalf("expected editMessageText for callback payload %q", data)
	}
	req, ok := transport.lastEndpoint("editMessageText")
	if !ok {
		t.Fatalf("expected last editMessageText request for payload %q", data)
	}
	return req.Values.Get("text")
}

func seedTogo(t *testing.T, ownerID int64, title string, progress uint8) uint64 {
	t.Helper()
	seed := &Togo.Togo{OwnerId: ownerID, Title: title, Weight: 1, Progress: progress, Date: Togo.Today()}
	id, err := seed.Save()
	if err != nil {
		t.Fatalf("failed to seed togo %q: %v", title, err)
	}
	return id
}

func seedTask(t *testing.T, ownerID int64, title string, progress uint8) uint64 {
	t.Helper()
	seed := &Task.Task{OwnerId: ownerID, Title: title, Weight: 1, Progress: progress}
	id, err := seed.Save()
	if err != nil {
		t.Fatalf("failed to seed task %q: %v", title, err)
	}
	return id
}

func TestHandleUpdateUnknownCommandReturnsHelp(t *testing.T) {
	bot, transport := newRecordingBot(t)

	update := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 17,
		Text:      "unknown-command",
		Chat:      &tgbotapi.Chat{ID: 7001},
	}}

	bot.HandleUpdate(update)

	req, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected sendMessage request for text update")
	}
	if req.Values.Get("chat_id") != "7001" {
		t.Fatalf("expected chat_id 7001, got %q", req.Values.Get("chat_id"))
	}
	if req.Values.Get("reply_to_message_id") != "17" {
		t.Fatalf("expected reply_to_message_id 17, got %q", req.Values.Get("reply_to_message_id"))
	}
	if req.Values.Get("text") != HELP_MESSAGE {
		t.Fatal("expected unknown command to return HELP_MESSAGE")
	}
	if !strings.Contains(req.Values.Get("reply_markup"), "keyboard") {
		t.Fatalf("expected reply keyboard markup, got %q", req.Values.Get("reply_markup"))
	}
}

func TestHandleUpdateNowCommand(t *testing.T) {
	bot, transport := newRecordingBot(t)

	update := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 18,
		Text:      "/now",
		Chat:      &tgbotapi.Chat{ID: 7002},
	}}

	bot.HandleUpdate(update)

	req, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected sendMessage request for /now update")
	}
	text := req.Values.Get("text")
	if !strings.Contains(text, "\t") || !strings.Contains(text, ":") {
		t.Fatalf("expected /now response in date-time format, got %q", text)
	}
}

func TestHandleUpdateTaskSettingsUsageOnInvalidInput(t *testing.T) {
	bot, transport := newRecordingBot(t)

	update := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 19,
		Text:      "~s  abc",
		Chat:      &tgbotapi.Chat{ID: 7003},
	}}

	bot.HandleUpdate(update)

	req, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected sendMessage request for invalid task settings input")
	}
	text := req.Values.Get("text")
	if !strings.Contains(text, "Usage: ~s  <times_per_day>") {
		t.Fatalf("expected usage text, got %q", text)
	}
	if !strings.Contains(text, "Allowed values:") {
		t.Fatalf("expected allowed-values text, got %q", text)
	}
}

func TestHandleUpdateTaskSettingsGetAndSet(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(7004)

	getUpdate := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 20,
		Text:      "~s",
		Chat:      &tgbotapi.Chat{ID: chatID},
	}}
	bot.HandleUpdate(getUpdate)

	getReq, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected sendMessage for task settings read")
	}
	if !strings.Contains(getReq.Values.Get("text"), "Current task reminder frequency: 4 times/day") {
		t.Fatalf("expected default reminder frequency in response, got %q", getReq.Values.Get("text"))
	}

	setUpdate := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 21,
		Text:      "~s  6",
		Chat:      &tgbotapi.Chat{ID: chatID},
	}}
	bot.HandleUpdate(setUpdate)

	setReq, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected sendMessage for task settings update")
	}
	if !strings.Contains(setReq.Values.Get("text"), "Task reminders updated to 6 times/day.") {
		t.Fatalf("expected successful reminder update message, got %q", setReq.Values.Get("text"))
	}

	setting, err := Task.GetReminderSetting(chatID)
	if err != nil {
		t.Fatalf("failed loading updated reminder setting: %v", err)
	}
	if setting.RemindersPerDay != 6 {
		t.Fatalf("expected reminders/day to be 6, got %d", setting.RemindersPerDay)
	}
}

func TestHandleUpdateDBUnauthorized(t *testing.T) {
	bot, transport := newRecordingBot(t)
	oldEnv := env
	defer func() {
		env = oldEnv
	}()
	env = map[string]string{"ADMIN_ID": "12345"}

	update := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 22,
		Text:      "/db",
		Chat:      &tgbotapi.Chat{ID: 7005},
	}}

	bot.HandleUpdate(update)

	req, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected sendMessage request for unauthorized /db")
	}
	if req.Values.Get("text") != "get the fuck off my porch!" {
		t.Fatalf("expected unauthorized db message, got %q", req.Values.Get("text"))
	}
}

func TestHandleUpdateCallbackUnsupportedActionEditsMessage(t *testing.T) {
	bot, transport := newRecordingBot(t)
	payload := (CallbackData{Action: None}).Json()
	update := tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		Data: payload,
		Message: &tgbotapi.Message{
			MessageID: 44,
			Chat:      &tgbotapi.Chat{ID: 8001},
		},
	}}

	bot.HandleUpdate(update)

	req, ok := transport.lastEndpoint("editMessageText")
	if !ok {
		t.Fatal("expected editMessageText request for callback update")
	}
	if req.Values.Get("chat_id") != "8001" {
		t.Fatalf("expected chat_id 8001 in edit request, got %q", req.Values.Get("chat_id"))
	}
	if req.Values.Get("message_id") != "44" {
		t.Fatalf("expected message_id 44 in edit request, got %q", req.Values.Get("message_id"))
	}
	if req.Values.Get("text") != "Unsupported callback action." {
		t.Fatalf("expected unsupported-action text, got %q", req.Values.Get("text"))
	}
}

func TestHandleUpdateCallbackShowTaskPageClampsToValidRange(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	ownerID := int64(8002)

	seed := &Task.Task{OwnerId: ownerID, Title: "One task", Weight: 1, Progress: 10}
	if _, err := seed.Save(); err != nil {
		t.Fatalf("failed to seed task for callback test: %v", err)
	}

	payload := (CallbackData{Action: ShowTaskPage, TaskPage: 999}).Json()
	update := tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		Data: payload,
		Message: &tgbotapi.Message{
			MessageID: 45,
			Chat:      &tgbotapi.Chat{ID: ownerID},
		},
	}}

	bot.HandleUpdate(update)

	req, ok := transport.lastEndpoint("editMessageText")
	if !ok {
		t.Fatal("expected editMessageText request for ShowTaskPage callback")
	}
	if !strings.Contains(req.Values.Get("text"), "Page 1/1") {
		t.Fatalf("expected clamped page indicator in response, got %q", req.Values.Get("text"))
	}
}

func TestHandleUpdateRecoversFromPanicsAndNotifiesAdmin(t *testing.T) {
	bot, transport := newRecordingBot(t)
	oldEnv := env
	defer func() {
		env = oldEnv
	}()
	env = map[string]string{"ADMIN_ID": "9001"}

	update := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 99,
		Text:      "/now",
		Chat:      nil,
	}}

	bot.HandleUpdate(update)

	req, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected admin sendMessage after panic recovery")
	}
	if req.Values.Get("chat_id") != "9001" {
		t.Fatalf("expected admin chat_id 9001 for panic notification, got %q", req.Values.Get("chat_id"))
	}
	text := req.Values.Get("text")
	if !strings.Contains(text, "Panic during update processing") {
		t.Fatalf("expected panic notification text, got %q", text)
	}
	if !strings.Contains(text, "runtime error") && !strings.Contains(strings.ToLower(text), "panic") {
		t.Fatalf("expected panic details in admin notification, got %q", text)
	}
}

func TestHandleUpdateCommandMatrix(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	oldEnv := env
	defer func() {
		env = oldEnv
	}()
	env = map[string]string{"ADMIN_ID": "999999"}

	chatID := int64(8100)
	togoID := seedTogo(t, chatID, "matrix-togo", 0)
	taskID := seedTask(t, chatID, "matrix-task", 0)

	cases := []struct {
		input          string
		expectContains string
	}{
		{input: "+", expectContains: "You must provide at least one Parameters!"},
		{input: "^", expectContains: "You must provide at least one task title/parameter."},
		{input: "#", expectContains: "Imported togos:"},
		{input: "%", expectContains: "Progress:"},
		{input: "%  t", expectContains: "Active tasks Progress:"},
		{input: "%  b", expectContains: "Progress:"},
		{input: "$", expectContains: "You must provide the get identifier!"},
		{input: "&", expectContains: "You must provide the task identifier!"},
		{input: "~", expectContains: "Page 1/1"},
		{input: "✅", expectContains: "Here are your togos for today:"},
		{input: "✅T", expectContains: "Here are your tasks to tick:"},
		{input: "❌", expectContains: "Here are your Today's togos:"},
		{input: "❌T", expectContains: "Here are your tasks to remove:"},
		{input: "~s  0", expectContains: "Task reminders are now disabled"},
		{input: "/db", expectContains: "get the fuck off my porch!"},
		{input: "/now", expectContains: "\t"},
		{input: "$  " + fmt.Sprint(togoID), expectContains: "matrix-togo"},
		{input: "&  " + fmt.Sprint(taskID), expectContains: "Task #"},
	}

	for i, tc := range cases {
		text := sendTextUpdateAndGetLastText(t, bot, transport, chatID, 300+i, tc.input)
		if !strings.Contains(text, tc.expectContains) {
			t.Fatalf("input %q expected response to contain %q, got %q", tc.input, tc.expectContains, text)
		}
	}
}

func TestHandleUpdateCallbackMatrix(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	ownerID := int64(8200)

	togoID := seedTogo(t, ownerID, "cb-togo", 0)
	taskTickID := seedTask(t, ownerID, "cb-task-tick", 0)
	taskRemoveID := seedTask(t, ownerID, "cb-task-remove", 0)

	tickTogoText := sendCallbackAndGetEditedText(t, bot, transport, ownerID, 401,
		(CallbackData{Action: TickTogo, ID: int64(togoID), AllDays: true}).Json(),
	)
	if !strings.Contains(tickTogoText, "DONE! Now select the next togo") {
		t.Fatalf("expected TickTogo callback success message, got %q", tickTogoText)
	}

	removeTogoText := sendCallbackAndGetEditedText(t, bot, transport, ownerID, 402,
		(CallbackData{Action: RemoveTogo, ID: int64(togoID), AllDays: true}).Json(),
	)
	if !strings.Contains(removeTogoText, "DONE!") {
		t.Fatalf("expected RemoveTogo callback success message, got %q", removeTogoText)
	}

	tickTaskText := sendCallbackAndGetEditedText(t, bot, transport, ownerID, 403,
		(CallbackData{Action: TickTask, ID: int64(taskTickID), TaskIncludeInactive: true}).Json(),
	)
	if !strings.Contains(tickTaskText, "Task updated") {
		t.Fatalf("expected TickTask callback success message, got %q", tickTaskText)
	}

	removeTaskText := sendCallbackAndGetEditedText(t, bot, transport, ownerID, 404,
		(CallbackData{Action: RemoveTask, ID: int64(taskRemoveID), TaskIncludeInactive: true}).Json(),
	)
	if !strings.Contains(removeTaskText, "Task removed") {
		t.Fatalf("expected RemoveTask callback success message, got %q", removeTaskText)
	}

	_ = seedTask(t, ownerID, "cb-task-page", 0)
	showPageText := sendCallbackAndGetEditedText(t, bot, transport, ownerID, 405,
		(CallbackData{Action: ShowTaskPage, TaskPage: 999, TaskIncludeInactive: true}).Json(),
	)
	if !strings.Contains(showPageText, "Page 1/1") {
		t.Fatalf("expected ShowTaskPage callback to clamp and render page, got %q", showPageText)
	}
}
