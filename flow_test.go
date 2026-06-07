package main

import (
	"strings"
	"testing"

	"ToGo4BotPlus/Idea"
	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// startFlowGetSend triggers a flow slash command and returns the new wizard
// message (a fresh sendMessage).
func startFlowGetSend(t *testing.T, bot *TelegramBotAPI, transport *recordingTransport, chatID int64, messageID int, text string) capturedRequest {
	t.Helper()
	before := transport.countEndpoint("sendMessage")
	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: messageID, Text: text, Chat: &tgbotapi.Chat{ID: chatID},
	}})
	if transport.countEndpoint("sendMessage") <= before {
		t.Fatalf("expected sendMessage when starting flow with %q", text)
	}
	req, _ := transport.lastEndpoint("sendMessage")
	return req
}

// sendFlowTextGetEdit feeds a typed reply into an active flow and returns the
// edited wizard message text.
func sendFlowTextGetEdit(t *testing.T, bot *TelegramBotAPI, transport *recordingTransport, chatID int64, messageID int, text string) string {
	t.Helper()
	before := transport.countEndpoint("editMessageText")
	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: messageID, Text: text, Chat: &tgbotapi.Chat{ID: chatID},
	}})
	if transport.countEndpoint("editMessageText") <= before {
		t.Fatalf("expected editMessageText after flow text %q", text)
	}
	req, _ := transport.lastEndpoint("editMessageText")
	return req.Values.Get("text")
}

func TestAddIdeaFlowEndToEndWithSkippedCategory(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9100)

	// Start: step 1 asks for the text.
	start := startFlowGetSend(t, bot, transport, chatID, 900, "/addIdea")
	if !strings.Contains(start.Values.Get("text"), "What's your idea?") {
		t.Fatalf("expected text prompt, got %q", start.Values.Get("text"))
	}

	// Provide the text -> step 2 (priority choice).
	priorityPrompt := sendFlowTextGetEdit(t, bot, transport, chatID, 901, "Launch a startup")
	if !strings.Contains(priorityPrompt, "important") {
		t.Fatalf("expected priority prompt, got %q", priorityPrompt)
	}

	// Select High (option index 0) -> step 3 (category).
	categoryPrompt := sendCallbackAndGetEditedText(t, bot, transport, chatID, 902,
		(CallbackData{Action: FlowSelect, FlowOpt: 0}).Json())
	if !strings.Contains(categoryPrompt, "category") {
		t.Fatalf("expected category prompt, got %q", categoryPrompt)
	}

	// Skip the optional category -> confirm screen with summary.
	confirm := sendCallbackAndGetEditedText(t, bot, transport, chatID, 903,
		(CallbackData{Action: FlowSkip}).Json())
	if !strings.Contains(confirm, "Review your idea") || !strings.Contains(confirm, "Launch a startup") {
		t.Fatalf("expected confirm summary, got %q", confirm)
	}
	if !strings.Contains(confirm, "High") {
		t.Fatalf("expected High priority in summary, got %q", confirm)
	}

	// Save -> commit + result.
	result := sendCallbackAndGetEditedText(t, bot, transport, chatID, 904,
		(CallbackData{Action: FlowConfirm}).Json())
	if !strings.Contains(result, "Saved") {
		t.Fatalf("expected saved confirmation, got %q", result)
	}

	// State cleared and idea persisted.
	if _, active := bot.flows.Get(chatID); active {
		t.Fatal("expected flow state cleared after commit")
	}
	ideas, _ := Idea.Load(chatID, false, false, 0)
	if len(ideas) != 1 {
		t.Fatalf("expected 1 saved idea, got %d", len(ideas))
	}
	if ideas[0].Text != "Launch a startup" || !ideas[0].IsHighPriority || ideas[0].Category != "" {
		t.Fatalf("saved idea mismatch: %+v", ideas[0])
	}
}

func TestAddIdeaFlowWithCustomCategory(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9101)

	startFlowGetSend(t, bot, transport, chatID, 910, "/addidea")
	sendFlowTextGetEdit(t, bot, transport, chatID, 911, "Write a book")
	// Normal priority (index 1) -> category step.
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 912,
		(CallbackData{Action: FlowSelect, FlowOpt: 1}).Json())
	// Choose custom -> awaits typed category.
	customPrompt := sendCallbackAndGetEditedText(t, bot, transport, chatID, 913,
		(CallbackData{Action: FlowCustom}).Json())
	if !strings.Contains(customPrompt, "custom") {
		t.Fatalf("expected custom-entry prompt, got %q", customPrompt)
	}
	// Type the custom category -> confirm.
	confirm := sendFlowTextGetEdit(t, bot, transport, chatID, 914, "Creative")
	if !strings.Contains(confirm, "Creative") {
		t.Fatalf("expected custom category in summary, got %q", confirm)
	}
	// Save.
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 915,
		(CallbackData{Action: FlowConfirm}).Json())

	ideas, _ := Idea.Load(chatID, false, false, 0)
	if len(ideas) != 1 || ideas[0].Category != "Creative" || ideas[0].IsHighPriority {
		t.Fatalf("custom-category idea mismatch: %+v", ideas)
	}
	// The new category should now be a remembered suggestion.
	cats, _ := Idea.LoadCategories(chatID)
	if len(cats) != 1 || cats[0] != "Creative" {
		t.Fatalf("expected Creative remembered, got %v", cats)
	}
}

func TestAddIdeaFlowSuggestsRememberedCategories(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9102)
	seedIdea(t, chatID, "seed idea", false, "Engineering")

	startFlowGetSend(t, bot, transport, chatID, 920, "/addidea")
	sendFlowTextGetEdit(t, bot, transport, chatID, 921, "Another idea")
	categoryStep := sendCallbackAndGetEditedText(t, bot, transport, chatID, 922,
		(CallbackData{Action: FlowSelect, FlowOpt: 0}).Json())
	if !strings.Contains(categoryStep, "category") {
		t.Fatalf("expected category prompt, got %q", categoryStep)
	}
	req, _ := transport.lastEndpoint("editMessageText")
	if !strings.Contains(req.Values.Get("reply_markup"), "Engineering") {
		t.Fatalf("expected remembered category button, got markup %q", req.Values.Get("reply_markup"))
	}
}

func TestFlowCancel(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9103)

	startFlowGetSend(t, bot, transport, chatID, 930, "/addidea")
	if _, active := bot.flows.Get(chatID); !active {
		t.Fatal("expected active flow after start")
	}
	cancelText := sendFlowTextGetEdit(t, bot, transport, chatID, 931, "/cancel")
	if !strings.Contains(cancelText, "Cancelled") {
		t.Fatalf("expected cancellation message, got %q", cancelText)
	}
	if _, active := bot.flows.Get(chatID); active {
		t.Fatal("expected flow cleared after /cancel")
	}
}

func TestFlowCallbackExpiredWhenNoState(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9104)

	text := sendCallbackAndGetEditedText(t, bot, transport, chatID, 940,
		(CallbackData{Action: FlowConfirm}).Json())
	if !strings.Contains(text, "expired") {
		t.Fatalf("expected expired-menu message, got %q", text)
	}
}

func TestParseFlowCommand(t *testing.T) {
	cases := []struct {
		in      string
		wantCmd string
		wantOk  bool
	}{
		{"/addIdea", "addidea", true},
		{"/addidea@MyBot", "addidea", true},
		{"/addtask  5", "addtask", true},
		{"/cancel", "cancel", true},
		{"/now", "", false},
		{"/db", "", false},
		{"/ideabook", "", false}, // browsers are stateless, not guided flows
		{"plain text", "", false},
		{"*  an idea", "", false},
	}
	for _, c := range cases {
		cmd, _, ok := parseFlowCommand(c.in)
		if ok != c.wantOk || (ok && cmd != c.wantCmd) {
			t.Fatalf("parseFlowCommand(%q) = (%q,%v), want (%q,%v)", c.in, cmd, ok, c.wantCmd, c.wantOk)
		}
	}
}

func TestAddTogoFlowEndToEnd(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9200)

	startFlowGetSend(t, bot, transport, chatID, 1000, "/addtogo")
	sendFlowTextGetEdit(t, bot, transport, chatID, 1001, "Buy milk") // -> weight
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1002,
		(CallbackData{Action: FlowSelect, FlowOpt: 1}).Json()) // weight preset "2" -> progress
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1003,
		(CallbackData{Action: FlowSkip}).Json()) // skip progress -> extra
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1004,
		(CallbackData{Action: FlowSelect, FlowOpt: 0}).Json()) // Normal -> day
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1005,
		(CallbackData{Action: FlowSelect, FlowOpt: 0}).Json()) // Today -> time
	confirm := sendFlowTextGetEdit(t, bot, transport, chatID, 1006, "10:30") // time -> duration
	_ = confirm
	confirm = sendCallbackAndGetEditedText(t, bot, transport, chatID, 1007,
		(CallbackData{Action: FlowSkip}).Json()) // skip duration -> confirm
	if !strings.Contains(confirm, "Review your togo") || !strings.Contains(confirm, "Buy milk") {
		t.Fatalf("expected togo confirm summary, got %q", confirm)
	}
	result := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1008,
		(CallbackData{Action: FlowConfirm}).Json())
	if !strings.Contains(result, "saved") {
		t.Fatalf("expected save confirmation, got %q", result)
	}

	togos, _ := Togo.Load(chatID, false, false)
	if len(togos) != 1 {
		t.Fatalf("expected 1 togo, got %d", len(togos))
	}
	if togos[0].Title != "Buy milk" || togos[0].Weight != 2 {
		t.Fatalf("togo mismatch: %+v", togos[0])
	}
}

func TestAddTaskFlowEndToEnd(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9201)

	startFlowGetSend(t, bot, transport, chatID, 1100, "/addtask")
	sendFlowTextGetEdit(t, bot, transport, chatID, 1101, "Finish report") // -> weight
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1102,
		(CallbackData{Action: FlowSkip}).Json()) // skip weight -> progress
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1103,
		(CallbackData{Action: FlowSelect, FlowOpt: 2}).Json()) // progress preset "50" -> extra
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1104,
		(CallbackData{Action: FlowSelect, FlowOpt: 1}).Json()) // Extra -> start
	confirm := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1105,
		(CallbackData{Action: FlowSelect, FlowOpt: 1}).Json()) // Tomorrow -> confirm
	if !strings.Contains(confirm, "Review your task") || !strings.Contains(confirm, "Finish report") {
		t.Fatalf("expected task confirm summary, got %q", confirm)
	}
	result := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1106,
		(CallbackData{Action: FlowConfirm}).Json())
	if !strings.Contains(result, "saved") {
		t.Fatalf("expected save confirmation, got %q", result)
	}

	tasks, _ := Task.Load(chatID, true, true)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Title != "Finish report" || tasks[0].Progress != 50 || !tasks[0].Extra {
		t.Fatalf("task mismatch: %+v", tasks[0])
	}
	if tasks[0].StartDate == nil {
		t.Fatal("expected a start date to be set (Tomorrow)")
	}
}

func TestAddTogoFlowValidationRejectsBadTime(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9202)

	startFlowGetSend(t, bot, transport, chatID, 1200, "/addtogo")
	sendFlowTextGetEdit(t, bot, transport, chatID, 1201, "Workout")
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1202, (CallbackData{Action: FlowSkip}).Json())               // weight
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1203, (CallbackData{Action: FlowSkip}).Json())               // progress
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1204, (CallbackData{Action: FlowSelect, FlowOpt: 0}).Json()) // extra Normal
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1205, (CallbackData{Action: FlowSelect, FlowOpt: 0}).Json()) // day Today
	bad := sendFlowTextGetEdit(t, bot, transport, chatID, 1206, "not-a-time")
	if !strings.Contains(bad, "HH:MM") {
		t.Fatalf("expected time validation error, got %q", bad)
	}
	// Flow should still be active on the same (time) step.
	if state, ok := bot.flows.Get(chatID); !ok || state.Data["time"] != "" {
		t.Fatal("expected to remain on the time step after invalid input")
	}
}

func TestManageIdeaFlowEditAndDelete(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9300)
	id := seedIdea(t, chatID, "manage me", false, "Tech")

	// Editing is reached from the browser's ✏️ Edit button.
	card := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1301,
		(CallbackData{Action: IdeaMenuEdit, ID: int64(id)}).Json())
	if !strings.Contains(card, "manage me") {
		t.Fatalf("expected item card, got %q", card)
	}

	fields := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1302,
		(CallbackData{Action: FlowEdit}).Json())
	if !strings.Contains(fields, "change") {
		t.Fatalf("expected edit-field list, got %q", fields)
	}
	// Field index 1 = priority (text/priority/category order).
	valuePrompt := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1303,
		(CallbackData{Action: FlowSelect, FlowOpt: 1}).Json())
	if !strings.Contains(valuePrompt, "Priority") {
		t.Fatalf("expected priority value prompt, got %q", valuePrompt)
	}
	// Value index 0 = High.
	updated := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1304,
		(CallbackData{Action: FlowSelect, FlowOpt: 0}).Json())
	if !strings.Contains(updated, "Updated") || !strings.Contains(updated, "High") {
		t.Fatalf("expected updated card with High priority, got %q", updated)
	}

	reloaded, _ := Idea.Load(chatID, false, false, 0)
	got, _ := reloaded.Get(id)
	if !got.IsHighPriority {
		t.Fatalf("expected idea to be high priority after edit: %+v", *got)
	}

	confirm := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1305,
		(CallbackData{Action: FlowDelete}).Json())
	if !strings.Contains(confirm, "Delete this idea") {
		t.Fatalf("expected delete confirmation, got %q", confirm)
	}
	deleted := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1306,
		(CallbackData{Action: FlowConfirm}).Json())
	if !strings.Contains(deleted, "Deleted") {
		t.Fatalf("expected delete result, got %q", deleted)
	}
	after, _ := Idea.Load(chatID, false, false, 0)
	if len(after) != 0 {
		t.Fatalf("expected 0 ideas after delete, got %d", len(after))
	}
}

func TestManageTogoFlowToggle(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9301)
	id := seedTogo(t, chatID, "toggle me", 0)

	card := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1311,
		(CallbackData{Action: TogoMenuEdit, ID: int64(id)}).Json())
	if !strings.Contains(card, "toggle me") {
		t.Fatalf("expected togo card, got %q", card)
	}
	toggled := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1312,
		(CallbackData{Action: FlowToggle}).Json())
	if !strings.Contains(toggled, "Toggled") {
		t.Fatalf("expected toggle confirmation, got %q", toggled)
	}

	togos, _ := Togo.Load(chatID, false, false)
	got, _ := togos.Get(id)
	if got.Progress != 100 {
		t.Fatalf("expected progress 100 after toggle, got %d", got.Progress)
	}
}

func TestManageTaskFlowEditWeightViaText(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9302)
	id := seedTask(t, chatID, "edit task", 0)

	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1321,
		(CallbackData{Action: TaskMenuEdit, ID: int64(id)}).Json()) // card
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1322,
		(CallbackData{Action: FlowEdit}).Json()) // field list
	prompt := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1323,
		(CallbackData{Action: FlowSelect, FlowOpt: 1}).Json()) // weight (progress/weight/desc/extra)
	if !strings.Contains(prompt, "Weight") {
		t.Fatalf("expected weight entry prompt, got %q", prompt)
	}
	updated := sendFlowTextGetEdit(t, bot, transport, chatID, 1324, "5")
	if !strings.Contains(updated, "Updated") {
		t.Fatalf("expected updated card, got %q", updated)
	}

	tasks, _ := Task.Load(chatID, true, true)
	got, _ := tasks.Get(id)
	if got.Weight != 5 {
		t.Fatalf("expected weight 5 after edit, got %d", got.Weight)
	}
}

func TestBrowseEmptyList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9303)

	list := startFlowGetSend(t, bot, transport, chatID, 1330, "/togobook")
	if !strings.Contains(list.Values.Get("text"), "Nothing here yet") {
		t.Fatalf("expected empty browser message, got %q", list.Values.Get("text"))
	}
}

func TestRegisterBotCommands(t *testing.T) {
	bot, transport := newRecordingBot(t)
	bot.registerBotCommands()
	req, ok := transport.lastEndpoint("setMyCommands")
	if !ok {
		t.Fatal("expected a setMyCommands request")
	}
	cmds := req.Values.Get("commands")
	for _, want := range []string{"addidea", "addtogo", "addtask", "togobook", "taskbook", "ideabook", "articlebook", "cancel"} {
		if !strings.Contains(cmds, want) {
			t.Fatalf("expected %q in registered commands payload %q", want, cmds)
		}
	}
}

func TestFlowShowsRunningAnswerSummary(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9600)

	startFlowGetSend(t, bot, transport, chatID, 1600, "/addArticle")
	// Providing the title advances to the url step, which must now echo the
	// collected title under a divider.
	urlPrompt := sendFlowTextGetEdit(t, bot, transport, chatID, 1601, "My Great Article")
	if !strings.Contains(urlPrompt, "Title: My Great Article") {
		t.Fatalf("expected the running summary to show the title, got %q", urlPrompt)
	}
	if !strings.Contains(urlPrompt, flowDivider) {
		t.Fatalf("expected a divider above the summary, got %q", urlPrompt)
	}
}

func TestFlowDeletesConsumedUserInput(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9601)
	startFlowGetSend(t, bot, transport, chatID, 1610, "/addArticle")

	before := transport.countEndpoint("deleteMessage")
	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 1611, Text: "Some title", Chat: &tgbotapi.Chat{ID: chatID},
	}})
	if got := transport.countEndpoint("deleteMessage"); got != before+1 {
		t.Fatalf("expected the typed input to be deleted once, delta=%d", got-before)
	}
}

func TestFlowSummaryShowsChoiceLabelNotRawValue(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9602)

	startFlowGetSend(t, bot, transport, chatID, 1620, "/addIdea")
	sendFlowTextGetEdit(t, bot, transport, chatID, 1621, "Launch a startup") // -> priority step
	// Pick High; the category step must echo the human label, not the raw "high".
	cat := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1621,
		(CallbackData{Action: FlowSelect, FlowOpt: 0}).Json())
	if !strings.Contains(cat, "Priority: 🔴 High") {
		t.Fatalf("expected the choice label in the summary, got %q", cat)
	}
	if strings.Contains(cat, "Priority: high") {
		t.Fatalf("summary should not show the raw option value: %q", cat)
	}
}

func TestManageEditDeletesTypedValue(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9603)
	id := seedIdea(t, chatID, "manage me", false, "")

	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1631, (CallbackData{Action: IdeaMenuEdit, ID: int64(id)}).Json()) // card
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1631, (CallbackData{Action: FlowEdit}).Json())                    // field list
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1631, (CallbackData{Action: FlowSelect, FlowOpt: 0}).Json())      // text field -> type

	before := transport.countEndpoint("deleteMessage")
	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 1632, Text: "new manage text", Chat: &tgbotapi.Chat{ID: chatID},
	}})
	if got := transport.countEndpoint("deleteMessage"); got != before+1 {
		t.Fatalf("expected the manage edit text to be deleted, delta=%d", got-before)
	}
}

func TestFlowValidators(t *testing.T) {
	ok := func(err error) bool { return err == nil }
	if !ok(validatePositiveInt("3")) || ok(validatePositiveInt("0")) || ok(validatePositiveInt("x")) {
		t.Fatal("validatePositiveInt behaves unexpectedly")
	}
	if !ok(validatePercent("50")) || ok(validatePercent("150")) || ok(validatePercent("-1")) || ok(validatePercent("x")) {
		t.Fatal("validatePercent behaves unexpectedly")
	}
	if !ok(validateHHMM("14:30")) || ok(validateHHMM("99:99")) || ok(validateHHMM("nope")) {
		t.Fatal("validateHHMM behaves unexpectedly")
	}
	if !ok(validateStartDate("2")) || !ok(validateStartDate("2026-01-02")) || ok(validateStartDate("nope")) {
		t.Fatal("validateStartDate behaves unexpectedly")
	}
	if !ok(nonEmptyText("a")) || ok(nonEmptyText("   ")) {
		t.Fatal("nonEmptyText behaves unexpectedly")
	}
}

func TestManageTogoEditDescriptionAndExtra(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9400)
	id := seedTogo(t, chatID, "edit me", 0)

	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1401, (CallbackData{Action: TogoMenuEdit, ID: int64(id)}).Json()) // card
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1402, (CallbackData{Action: FlowEdit}).Json())                    // fields
	// field index 2 = description (progress/weight/description/extra)
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1403, (CallbackData{Action: FlowSelect, FlowOpt: 2}).Json())
	updated := sendFlowTextGetEdit(t, bot, transport, chatID, 1404, "a fresh description")
	if !strings.Contains(updated, "Updated") || !strings.Contains(updated, "a fresh description") {
		t.Fatalf("expected updated description card, got %q", updated)
	}

	// Now mark it extra via the choice field.
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1405, (CallbackData{Action: FlowEdit}).Json())
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1406, (CallbackData{Action: FlowSelect, FlowOpt: 3}).Json()) // extra
	extraCard := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1407, (CallbackData{Action: FlowSelect, FlowOpt: 1}).Json())
	if !strings.Contains(extraCard, "Updated") {
		t.Fatalf("expected updated extra card, got %q", extraCard)
	}

	togos, _ := Togo.Load(chatID, false, false)
	got, _ := togos.Get(id)
	if got.Description != "a fresh description" || !got.Extra {
		t.Fatalf("togo edit not persisted: %+v", *got)
	}
}

func TestManageTaskDelete(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9401)
	id := seedTask(t, chatID, "delete me", 0)

	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1411, (CallbackData{Action: TaskMenuEdit, ID: int64(id)}).Json())
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1412, (CallbackData{Action: FlowDelete}).Json())
	deleted := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1413, (CallbackData{Action: FlowConfirm}).Json())
	if !strings.Contains(deleted, "Deleted") {
		t.Fatalf("expected delete result, got %q", deleted)
	}
	tasks, _ := Task.Load(chatID, true, true)
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

func TestManageTypingOnCardReRenders(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9402)
	id := seedIdea(t, chatID, "stray text test", false, "")

	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1421, (CallbackData{Action: IdeaMenuEdit, ID: int64(id)}).Json()) // card
	// Typing while on the card screen (not awaiting text) should just re-render the card.
	reRendered := sendFlowTextGetEdit(t, bot, transport, chatID, 1422, "ignored text")
	if !strings.Contains(reRendered, "stray text test") {
		t.Fatalf("expected card re-render on stray text, got %q", reRendered)
	}
}

func TestManageBackNavigation(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	chatID := int64(9403)
	id := seedIdea(t, chatID, "nav idea", false, "")

	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1431, (CallbackData{Action: IdeaMenuEdit, ID: int64(id)}).Json()) // card
	// card -> back -> list
	list := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1432, (CallbackData{Action: FlowBack}).Json())
	if !strings.Contains(list, "Pick one to edit") {
		t.Fatalf("expected back to list, got %q", list)
	}
	// list -> select -> card -> edit -> back -> card
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1433, (CallbackData{Action: FlowSelect, FlowOpt: 0}).Json())
	sendCallbackAndGetEditedText(t, bot, transport, chatID, 1434, (CallbackData{Action: FlowEdit}).Json())
	card := sendCallbackAndGetEditedText(t, bot, transport, chatID, 1435, (CallbackData{Action: FlowBack}).Json())
	if !strings.Contains(card, "nav idea") {
		t.Fatalf("expected back to card, got %q", card)
	}
}
