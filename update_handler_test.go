package main

import (
	"fmt"
	"strings"
	"testing"

	"ToGo4BotPlus/Idea"
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

func seedIdea(t *testing.T, ownerID int64, text string, high bool, category string) uint64 {
	t.Helper()
	seed := &Idea.Idea{OwnerId: ownerID, Text: text, IsHighPriority: high, Category: category}
	id, err := seed.Save()
	if err != nil {
		t.Fatalf("failed to seed idea %q: %v", text, err)
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

// countMenuButtons walks an inline keyboard and splits the buttons into item
// buttons (tick/remove) and navigation buttons (page hops).
func countMenuButtons(menu *tgbotapi.InlineKeyboardMarkup) (items int, navButtons int) {
	for _, row := range menu.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == nil {
				continue
			}
			switch LoadCallbackData(*button.CallbackData).Action {
			case ShowTogoMenuPage, ShowTaskMenuPage, ShowIdeaMenuPage:
				navButtons++
			default:
				items++
			}
		}
	}
	return
}

func TestInlineKeyboardMenuPaginatesTogos(t *testing.T) {
	total := MaximumInlineMenuItems + 5
	togos := make(Togo.TogoList, 0, total)
	for i := 0; i < total; i++ {
		togos = append(togos, Togo.Togo{Id: uint64(i + 1), Title: fmt.Sprintf("togo-%d", i+1)})
	}

	first := InlineKeyboardMenu(togos, TickTogo, false, false, 0)
	items, nav := countMenuButtons(first)
	if items != MaximumInlineMenuItems {
		t.Fatalf("page 0 expected %d item buttons, got %d", MaximumInlineMenuItems, items)
	}
	if nav < 1 {
		t.Fatal("page 0 expected navigation buttons for an oversized list")
	}
	// Item buttons on page 0 must carry MenuPage 0 so a tick re-renders the same page.
	if got := LoadCallbackData(*first.InlineKeyboard[0][0].CallbackData); got.MenuPage != 0 {
		t.Fatalf("expected item button to carry MenuPage 0, got %d", got.MenuPage)
	}

	second := InlineKeyboardMenu(togos, TickTogo, false, false, 1)
	items, _ = countMenuButtons(second)
	if items != 5 {
		t.Fatalf("page 1 expected the 5 remaining item buttons, got %d", items)
	}
	if got := LoadCallbackData(*second.InlineKeyboard[0][0].CallbackData); got.MenuPage != 1 {
		t.Fatalf("expected page-1 item button to carry MenuPage 1, got %d", got.MenuPage)
	}

	// Out-of-range pages clamp to the last available page.
	clamped := InlineKeyboardMenu(togos, TickTogo, false, false, 99)
	items, _ = countMenuButtons(clamped)
	if items != 5 {
		t.Fatalf("clamped page expected 5 item buttons (last page), got %d", items)
	}

	// A small list stays single-page with no navigation row.
	small := InlineKeyboardMenu(togos[:2], TickTogo, false, false, 0)
	if _, nav := countMenuButtons(small); nav != 0 {
		t.Fatalf("single-page list should have no navigation buttons, got %d", nav)
	}
}

func TestTaskInlineKeyboardMenuPaginates(t *testing.T) {
	total := MaximumInlineMenuItems + 3
	tasks := make(Task.TaskList, 0, total)
	for i := 0; i < total; i++ {
		tasks = append(tasks, Task.Task{Id: uint64(i + 1), Title: fmt.Sprintf("task-%d", i+1)})
	}

	first := TaskInlineKeyboardMenu(tasks, RemoveTask, false, 0)
	items, nav := countMenuButtons(first)
	if items != MaximumInlineMenuItems {
		t.Fatalf("page 0 expected %d task buttons, got %d", MaximumInlineMenuItems, items)
	}
	if nav < 1 {
		t.Fatal("page 0 expected navigation buttons for an oversized task list")
	}

	last := TaskInlineKeyboardMenu(tasks, RemoveTask, false, 1)
	if items, _ := countMenuButtons(last); items != 3 {
		t.Fatalf("page 1 expected 3 task buttons, got %d", items)
	}
}

func TestHandleUpdateTickTogoByID(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(8300)
	id := seedTogo(t, chatID, "quick-tick-togo", 0)

	text := sendTextUpdateAndGetLastText(t, bot, transport, chatID, 500, fmt.Sprintf("tk  %d", id))
	if !strings.Contains(text, "ticked") || strings.Contains(text, "unticked") {
		t.Fatalf("expected ticked confirmation, got %q", text)
	}

	togos, _ := Togo.Load(chatID, false, false)
	togo, err := togos.Get(id)
	if err != nil {
		t.Fatalf("failed to reload ticked togo: %v", err)
	}
	if togo.Progress != 100 {
		t.Fatalf("expected togo progress 100 after tk, got %d", togo.Progress)
	}

	text = sendTextUpdateAndGetLastText(t, bot, transport, chatID, 501, fmt.Sprintf("tk  %d", id))
	if !strings.Contains(text, "unticked") {
		t.Fatalf("expected unticked confirmation on second tk, got %q", text)
	}
	togos, _ = Togo.Load(chatID, false, false)
	togo, _ = togos.Get(id)
	if togo.Progress != 0 {
		t.Fatalf("expected togo progress 0 after toggle, got %d", togo.Progress)
	}
}

func TestHandleUpdateTickTaskByID(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(8301)
	id := seedTask(t, chatID, "quick-tick-task", 0)

	text := sendTextUpdateAndGetLastText(t, bot, transport, chatID, 510, fmt.Sprintf("TK  %d", id))
	if !strings.Contains(text, "ticked") || strings.Contains(text, "unticked") {
		t.Fatalf("expected ticked confirmation, got %q", text)
	}

	tasks, _ := Task.Load(chatID, true, true)
	task, err := tasks.Get(id)
	if err != nil {
		t.Fatalf("failed to reload ticked task: %v", err)
	}
	if task.Progress != 100 {
		t.Fatalf("expected task progress 100 after TK, got %d", task.Progress)
	}

	text = sendTextUpdateAndGetLastText(t, bot, transport, chatID, 511, fmt.Sprintf("TK  %d", id))
	if !strings.Contains(text, "unticked") {
		t.Fatalf("expected unticked confirmation on second TK, got %q", text)
	}
}

func TestHandleUpdateTickByIDErrorCases(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(8302)

	cases := []struct {
		input          string
		expectContains string
	}{
		{input: "tk", expectContains: "Usage: tk  <togo_id>"},
		{input: "TK", expectContains: "Usage: TK  <task_id>"},
		{input: "tk  abc", expectContains: "Invalid togo id"},
		{input: "TK  abc", expectContains: "Invalid task id"},
		{input: "tk  99999", expectContains: "can not find this togo"},
		{input: "TK  99999", expectContains: "can not find this task"},
	}
	for i, tc := range cases {
		text := sendTextUpdateAndGetLastText(t, bot, transport, chatID, 520+i, tc.input)
		if !strings.Contains(text, tc.expectContains) {
			t.Fatalf("input %q expected response to contain %q, got %q", tc.input, tc.expectContains, text)
		}
	}
}

func TestHandleUpdateShowMenuPageCallbacks(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	ownerID := int64(8303)
	seedTogo(t, ownerID, "page-togo-1", 0)
	seedTogo(t, ownerID, "page-togo-2", 0)
	seedTask(t, ownerID, "page-task-1", 0)
	seedTask(t, ownerID, "page-task-2", 0)

	cases := []struct {
		payload        string
		expectContains string
	}{
		{payload: (CallbackData{Action: ShowTogoMenuPage, MenuAction: TickTogo, MenuPage: 0}).Json(), expectContains: "Select a togo to tick"},
		{payload: (CallbackData{Action: ShowTogoMenuPage, MenuAction: RemoveTogo, MenuPage: 0}).Json(), expectContains: "Select a togo to remove"},
		{payload: (CallbackData{Action: ShowTaskMenuPage, MenuAction: TickTask, MenuPage: 0, TaskIncludeInactive: true}).Json(), expectContains: "Select a task to tick"},
		{payload: (CallbackData{Action: ShowTaskMenuPage, MenuAction: RemoveTask, MenuPage: 0, TaskIncludeInactive: true}).Json(), expectContains: "Select a task to remove"},
	}
	for i, tc := range cases {
		text := sendCallbackAndGetEditedText(t, bot, transport, ownerID, 600+i, tc.payload)
		if !strings.Contains(text, tc.expectContains) {
			t.Fatalf("callback %q expected response to contain %q, got %q", tc.payload, tc.expectContains, text)
		}
	}
}

func TestHandleUpdateIdeaCommandInterface(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(8500)

	// Add a high-priority idea with a category.
	addText := sendTextUpdateAndGetLastText(t, bot, transport, chatID, 700, "*  Build a rocket  +!  +c  Engineering")
	if !strings.Contains(addText, "Idea #") || !strings.Contains(addText, "created") {
		t.Fatalf("expected idea creation confirmation, got %q", addText)
	}

	// Add a normal-priority idea in a different category.
	_ = sendTextUpdateAndGetLastText(t, bot, transport, chatID, 701, "*  Write a poem  +c  Creative")

	// List all ideas.
	allText := sendTextUpdateAndGetLastText(t, bot, transport, chatID, 702, ";")
	if !strings.Contains(allText, "Build a rocket") || !strings.Contains(allText, "Write a poem") {
		t.Fatalf("expected both ideas in listing, got %q", allText)
	}

	// List only high-priority ideas.
	highText := sendTextUpdateAndGetLastText(t, bot, transport, chatID, 703, ";  !")
	if !strings.Contains(highText, "Build a rocket") || strings.Contains(highText, "Write a poem") {
		t.Fatalf("expected only the high-priority idea, got %q", highText)
	}

	// List by category.
	catText := sendTextUpdateAndGetLastText(t, bot, transport, chatID, 704, ";  c  Creative")
	if !strings.Contains(catText, "Write a poem") || strings.Contains(catText, "Build a rocket") {
		t.Fatalf("expected only the Creative idea, got %q", catText)
	}
}

func TestHandleUpdateIdeaUpdateAndRemoveMenu(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(8501)
	id := seedIdea(t, chatID, "old idea", false, "Tech")

	// Update via ;u
	updText := sendTextUpdateAndGetLastText(t, bot, transport, chatID, 710,
		fmt.Sprintf(";u  %d  +t  new idea text  +!", id))
	if !strings.Contains(updText, "new idea text") || !strings.Contains(updText, "High") {
		t.Fatalf("expected updated idea text + high priority, got %q", updText)
	}

	reloaded, _ := Idea.Load(chatID, false, "")
	got, err := reloaded.Get(id)
	if err != nil {
		t.Fatalf("failed to reload idea: %v", err)
	}
	if got.Text != "new idea text" || !got.IsHighPriority {
		t.Fatalf("idea update not persisted: %+v", *got)
	}

	// Remove menu (*x) should produce an inline keyboard.
	before := transport.countEndpoint("sendMessage")
	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 711, Text: "*x", Chat: &tgbotapi.Chat{ID: chatID},
	}})
	if transport.countEndpoint("sendMessage") <= before {
		t.Fatal("expected sendMessage for *x remove menu")
	}
	req, _ := transport.lastEndpoint("sendMessage")
	if !strings.Contains(req.Values.Get("text"), "ideas to remove") {
		t.Fatalf("expected remove-menu prompt, got %q", req.Values.Get("text"))
	}
	if !strings.Contains(req.Values.Get("reply_markup"), "callback_data") {
		t.Fatalf("expected inline keyboard with callback data, got %q", req.Values.Get("reply_markup"))
	}
}

func TestHandleUpdateRemoveIdeaCallback(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(8502)
	id := seedIdea(t, chatID, "removable idea", false, "")
	seedIdea(t, chatID, "surviving idea", false, "")

	text := sendCallbackAndGetEditedText(t, bot, transport, chatID, 720,
		(CallbackData{Action: RemoveIdea, ID: int64(id)}).Json())
	if !strings.Contains(text, "Idea removed") {
		t.Fatalf("expected idea-removed confirmation, got %q", text)
	}

	remaining, _ := Idea.Load(chatID, false, "")
	if len(remaining) != 1 {
		t.Fatalf("expected 1 idea remaining after removal, got %d", len(remaining))
	}

	// Page navigation callback re-renders the remove menu.
	pageText := sendCallbackAndGetEditedText(t, bot, transport, chatID, 721,
		(CallbackData{Action: ShowIdeaMenuPage, MenuAction: RemoveIdea, MenuPage: 0}).Json())
	if !strings.Contains(pageText, "Select an idea to remove") {
		t.Fatalf("expected idea page prompt, got %q", pageText)
	}
}

func TestIdeaInlineKeyboardMenuPaginates(t *testing.T) {
	total := MaximumInlineMenuItems + 4
	ideas := make(Idea.IdeaList, 0, total)
	for i := 0; i < total; i++ {
		ideas = append(ideas, Idea.Idea{Id: uint64(i + 1), Text: fmt.Sprintf("idea-%d", i+1)})
	}

	first := IdeaInlineKeyboardMenu(ideas, RemoveIdea, 0)
	items, nav := countMenuButtons(first)
	if items != MaximumInlineMenuItems {
		t.Fatalf("page 0 expected %d item buttons, got %d", MaximumInlineMenuItems, items)
	}
	if nav < 1 {
		t.Fatal("page 0 expected navigation buttons for an oversized idea list")
	}
	if last := IdeaInlineKeyboardMenu(ideas, RemoveIdea, 1); func() int { i, _ := countMenuButtons(last); return i }() != 4 {
		t.Fatal("page 1 expected the 4 remaining idea buttons")
	}
	if IdeaInlineKeyboardMenu(nil, RemoveIdea, 0) != nil {
		t.Fatal("expected nil keyboard for empty idea list")
	}
}
