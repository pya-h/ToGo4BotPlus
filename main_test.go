package main

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"ToGo4BotPlus/Idea"
	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type capturedRequest struct {
	Endpoint string
	Values   url.Values
}

type recordingTransport struct {
	requests []capturedRequest
	// failParseModeOnce, when set, makes the next Markdown sendMessage return an
	// API error (as Telegram does for unbalanced entities) and then clears
	// itself, so tests can exercise the plain-text retry in SendTextMessage.
	failParseModeOnce bool
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := ""
	if req.Body != nil {
		data, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		body = string(data)
	}
	values, err := url.ParseQuery(body)
	if err != nil {
		return nil, err
	}

	endpoint := path.Base(req.URL.Path)
	rt.requests = append(rt.requests, capturedRequest{Endpoint: endpoint, Values: values})

	if endpoint == "sendMessage" && rt.failParseModeOnce && values.Get("parse_mode") != "" {
		rt.failParseModeOnce = false
		responseBody := `{"ok":false,"error_code":400,"description":"Bad Request: can't parse entities"}`
		return &http.Response{
			StatusCode: 400,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Request:    req,
		}, nil
	}

	responseBody := `{"ok":true,"result":{}}`
	switch endpoint {
	case "getMe":
		responseBody = `{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"bot","username":"bot"}}`
	case "sendMessage", "editMessageText":
		responseBody = `{"ok":true,"result":{"message_id":1}}`
	}

	return &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(responseBody)),
		Request:    req,
	}, nil
}

func (rt *recordingTransport) countEndpoint(endpoint string) int {
	count := 0
	for _, req := range rt.requests {
		if req.Endpoint == endpoint {
			count++
		}
	}
	return count
}

func (rt *recordingTransport) lastEndpoint(endpoint string) (capturedRequest, bool) {
	for i := len(rt.requests) - 1; i >= 0; i-- {
		if rt.requests[i].Endpoint == endpoint {
			return rt.requests[i], true
		}
	}
	return capturedRequest{}, false
}

func newRecordingBot(t *testing.T) (*TelegramBotAPI, *recordingTransport) {
	t.Helper()

	transport := &recordingTransport{}
	client := &http.Client{Transport: transport}
	botAPI, err := tgbotapi.NewBotAPIWithClient("test-token", client)
	if err != nil {
		t.Fatalf("failed to create BotAPI with test transport: %v", err)
	}
	return &TelegramBotAPI{BotAPI: botAPI, flows: NewFlowStore()}, transport
}

func withTempWorkingDir(t *testing.T, initDB bool) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	if initDB {
		if err := Togo.InitDatabase(); err != nil {
			t.Fatalf("failed to initialize isolated db: %v", err)
		}
		if err := Task.InitDatabase(); err != nil {
			t.Fatalf("failed to initialize isolated task db: %v", err)
		}
		if err := Idea.InitDatabase(); err != nil {
			t.Fatalf("failed to initialize isolated idea db: %v", err)
		}
	}
}

// ============================================================
// B4: UTF-8 Truncation Tests for Inline Button Titles
// ============================================================

// TestInlineKeyboardMenuTruncatesCorrectly - B4: Button titles with emoji should not be truncated mid-rune
func TestInlineKeyboardMenuTruncatesCorrectly(t *testing.T) {
	// Create a test TogoList with emoji titles
	togos := Togo.TogoList{
		{Id: 1, Title: "🎯 Goal with a long name that should be truncated", Progress: 50},
		{Id: 2, Title: "📝 Write documentation", Progress: 100},
		{Id: 3, Title: "🚀 Deploy to production", Progress: 0},
	}

	// Call InlineKeyboardMenu
	menu := InlineKeyboardMenu(togos, TickTogo, false, false, 0)

	// Verify that the menu was created
	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	// Check that all button texts are valid UTF-8
	for row := range menu.InlineKeyboard {
		for col := range menu.InlineKeyboard[row] {
			buttonText := menu.InlineKeyboard[row][col].Text
			if !utf8.ValidString(buttonText) {
				t.Errorf("Button text is not valid UTF-8: %q", buttonText)
			}
		}
	}
}

// TestInlineKeyboardMenuEmojiPrefix - B4: Completed togo gets checkmark prefix without breaking UTF-8
func TestInlineKeyboardMenuEmojiPrefix(t *testing.T) {
	togos := Togo.TogoList{
		{Id: 1, Title: "Complete Task", Progress: 100}, // Completed, should get ✅ prefix
	}

	menu := InlineKeyboardMenu(togos, TickTogo, false, false, 0)

	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	// Get the button text
	buttonText := menu.InlineKeyboard[0][0].Text
	if !utf8.ValidString(buttonText) {
		t.Errorf("Button text with checkmark prefix is not valid UTF-8: %q", buttonText)
	}

	// Verify it contains the checkmark prefix
	if len(buttonText) < 5 || !strings.HasPrefix(buttonText, "✅ ") {
		t.Errorf("Completed togo should have checkmark prefix, got: %q", buttonText)
	}
}

// TestInlineKeyboardMenuLongTitle - B4: Very long titles should be truncated properly
func TestInlineKeyboardMenuLongTitle(t *testing.T) {
	longTitle := "This is a very long task title that needs to be truncated"
	for i := 0; i < 5; i++ {
		longTitle += " " + longTitle // Keep doubling to ensure it's very long
	}

	togos := Togo.TogoList{
		{Id: 1, Title: longTitle, Progress: 0},
	}

	menu := InlineKeyboardMenu(togos, TickTogo, false, false, 0)

	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	buttonText := menu.InlineKeyboard[0][0].Text
	if !utf8.ValidString(buttonText) {
		t.Errorf("Long title button text is not valid UTF-8: %q", buttonText)
	}

	// Button text should not exceed the maximum length significantly
	// (it should be truncated + "..." which is safe)
	if len(buttonText) > MaximumInlineButtonTextLength+10 {
		t.Errorf("Button text is too long: %d bytes (max expected: %d)", len(buttonText), MaximumInlineButtonTextLength+10)
	}
}

// TestInlineKeyboardMenuAllEmoji - B4: Titles that are purely emoji should work
func TestInlineKeyboardMenuAllEmoji(t *testing.T) {
	togos := Togo.TogoList{
		{Id: 1, Title: "🎯🚀📝🎨🔥", Progress: 0},
	}

	menu := InlineKeyboardMenu(togos, TickTogo, false, false, 0)

	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	buttonText := menu.InlineKeyboard[0][0].Text
	if !utf8.ValidString(buttonText) {
		t.Errorf("Emoji-only title button text is not valid UTF-8: %q", buttonText)
	}
}

// TestInlineKeyboardMenuMultibyteCharacters - B4: Multi-byte UTF-8 characters should not be split
func TestInlineKeyboardMenuMultibyteCharacters(t *testing.T) {
	// Include real multi-byte UTF-8 characters.
	togos := Togo.TogoList{
		{Id: 1, Title: "Cafe mañana", Progress: 0},
	}

	menu := InlineKeyboardMenu(togos, TickTogo, false, false, 0)

	if menu == nil {
		t.Fatal("InlineKeyboardMenu returned nil")
	}

	buttonText := menu.InlineKeyboard[0][0].Text
	if !utf8.ValidString(buttonText) {
		t.Errorf("Multi-byte character button text is not valid UTF-8: %q", buttonText)
	}

	// Verify the title is preserved
	if !strings.Contains(buttonText, "mañana") {
		t.Errorf("Button text should contain the task title, got: %q", buttonText)
	}
}

func TestLoadCallbackDataCorruptedJSON(t *testing.T) {
	data := LoadCallbackData("{not-json")
	if data.Action != None {
		t.Fatalf("expected zero-value action for corrupted json, got %v", data.Action)
	}
	if data.ID != 0 {
		t.Fatalf("expected zero-value ID for corrupted json, got %d", data.ID)
	}
}

func TestCallbackDataJsonRoundTrip(t *testing.T) {
	original := CallbackData{Action: TickTogo, ID: 42, AllDays: true, JustUndones: true}
	encoded := original.Json()
	decoded := LoadCallbackData(encoded)

	if decoded.Action != original.Action {
		t.Fatalf("expected action %v, got %v", original.Action, decoded.Action)
	}
	if decoded.ID != original.ID {
		t.Fatalf("expected id %d, got %d", original.ID, decoded.ID)
	}
	if decoded.AllDays != original.AllDays || decoded.JustUndones != original.JustUndones {
		t.Fatalf("expected AllDays=%t JustUndones=%t, got AllDays=%t JustUndones=%t",
			original.AllDays, original.JustUndones, decoded.AllDays, decoded.JustUndones)
	}
}

func TestCallbackDataJsonUnsupportedData(t *testing.T) {
	payload := CallbackData{Action: UpdateTogo, Data: make(chan int)}
	encoded := payload.Json()
	if !strings.Contains(strings.ToLower(encoded), "unsupported") {
		t.Fatalf("expected marshal error text for unsupported data type, got: %s", encoded)
	}
}

func TestMainKeyboardMenuAllDaysTokens(t *testing.T) {
	menu := MainKeyboardMenu()
	if menu == nil {
		t.Fatal("MainKeyboardMenu returned nil")
	}

	foundPlusA := map[string]bool{"✅": false, "%": false, "❌": false}
	foundLegacyA := map[string]bool{"✅": false, "%": false, "❌": false}

	for _, row := range menu.Keyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.Text, "✅") {
				if strings.Contains(btn.Text, "+a") {
					foundPlusA["✅"] = true
				}
				if strings.Contains(btn.Text, "  a") && !strings.Contains(btn.Text, "+a") {
					foundLegacyA["✅"] = true
				}
			}
			if strings.HasPrefix(btn.Text, "%") {
				if strings.Contains(btn.Text, "+a") {
					foundPlusA["%"] = true
				}
				if strings.Contains(btn.Text, "  a") && !strings.Contains(btn.Text, "+a") {
					foundLegacyA["%"] = true
				}
			}
			if strings.HasPrefix(btn.Text, "❌") {
				if strings.Contains(btn.Text, "+a") {
					foundPlusA["❌"] = true
				}
				if strings.Contains(btn.Text, "  a") && !strings.Contains(btn.Text, "+a") {
					foundLegacyA["❌"] = true
				}
			}
		}
	}

	for key, found := range foundPlusA {
		if !found {
			t.Fatalf("expected +a token for %s keyboard row", key)
		}
	}
	for key, found := range foundLegacyA {
		if found {
			t.Fatalf("found legacy 'a' token without plus sign for %s keyboard row", key)
		}
	}
}

// ============================================================
// B5: Panic Recovery Tests
// Note: These tests verify the panic-safety of parsing functions
// The update-loop recovery is covered indirectly via handler safety tests
// ============================================================

// TestSplitArgumentsWithSpecialCharacters - B5: SplitArguments should not panic
func TestSplitArgumentsWithSpecialCharacters(t *testing.T) {
	testCases := []string{
		"+  task  =  5",
		"#  ",
		"%  -",
		"$  123",
		"✅",
		"❌",
		"/db",
		"/now",
		"",
		" ",
		"     ",
		"task  without  proper  spacing",
		"🎯  emoji  task",
	}

	for _, testCase := range testCases {
		result := SplitArguments(testCase)
		if result == nil {
			t.Errorf("SplitArguments panicked on input: %q", testCase)
		}
	}
}

// TestExtractBoundsChecking - B5: Extract should not panic on malformed input
func TestExtractBoundsChecking(t *testing.T) {
	testCases := [][]string{
		{},
		{""},
		{"task"},
		{"task", "="},
		{"task", "@"},
		{"task", "+p"},
		{"task", ":", ""},
		{"task", "->"},
	}

	for _, terms := range testCases {
		_, err := Togo.Extract(123, terms)
		// We expect errors for invalid input, but no panic
		_ = err
	}
}

func TestTruncateUTF8AvoidsPartialLeadingByte(t *testing.T) {
	input := strings.Repeat("a", 20) + "🎯x"
	truncated := truncateUTF8(input, 21)

	if !utf8.ValidString(truncated) {
		t.Fatalf("truncateUTF8 returned invalid utf8: %q", truncated)
	}

	expected := strings.Repeat("a", 20)
	if truncated != expected {
		t.Fatalf("expected %q, got %q", expected, truncated)
	}
}

func TestBuildTogoImportStatsReport(t *testing.T) {
	togos := Togo.TogoList{
		{Id: 1, Title: "Done", Progress: 100},
		{Id: 2, Title: "In Progress", Progress: 50},
		{Id: 3, Title: "Planned", Progress: 0},
	}

	report := BuildTogoImportStatsReport(togos, false, false, nil)
	if !strings.Contains(report, "Imported togos: 3") {
		t.Fatalf("expected imported count in report, got: %s", report)
	}
	if !strings.Contains(report, "Shown: 3") {
		t.Fatalf("expected shown count in report, got: %s", report)
	}
	if !strings.Contains(report, "Done: 1") {
		t.Fatalf("expected done count in report, got: %s", report)
	}
	if !strings.Contains(report, "Pending: 2") {
		t.Fatalf("expected pending count in report, got: %s", report)
	}

	undoneOnlyReport := BuildTogoImportStatsReport(togos, false, true, nil)
	if !strings.Contains(undoneOnlyReport, "Shown: 2") {
		t.Fatalf("expected undone shown count in report, got: %s", undoneOnlyReport)
	}
	if !strings.Contains(undoneOnlyReport, "Done: 0") {
		t.Fatalf("expected undone done count in report, got: %s", undoneOnlyReport)
	}
}

func TestBuildTogoImportStatsReportWarningAndEmpty(t *testing.T) {
	warning := errors.New("db warning")
	report := BuildTogoImportStatsReport(nil, true, false, warning)

	if !strings.Contains(report, "All days report") {
		t.Fatalf("expected all-days scope in report, got: %s", report)
	}
	if !strings.Contains(report, "Imported togos: 0") || !strings.Contains(report, "Shown: 0") {
		t.Fatalf("expected zero counters in report, got: %s", report)
	}
	if !strings.Contains(report, "Warning: db warning") {
		t.Fatalf("expected warning text in report, got: %s", report)
	}

	empty := BuildTogoImportStatsReport(nil, false, false, nil)
	if !strings.Contains(empty, "Status: Nothing to show.") {
		t.Fatalf("expected empty status in report without warning, got: %s", empty)
	}
}

func TestTogosDueAtNextMinute(t *testing.T) {
	now := Togo.Today()
	togos := Togo.TogoList{
		{Id: 1, Title: "due", Date: Togo.Date{Time: now.Add(time.Minute)}},
		{Id: 2, Title: "later", Date: Togo.Date{Time: now.Add(2 * time.Minute)}},
	}

	due := togosDueAtNextMinute(togos, now)
	if len(due) != 1 {
		t.Fatalf("expected exactly one due togo, got %d", len(due))
	}
	if due[0].Id != 1 {
		t.Fatalf("expected due id 1, got %d", due[0].Id)
	}
}

func TestSendTextMessageUsesInlineKeyboardAndMarkdown(t *testing.T) {
	bot, transport := newRecordingBot(t)
	callback := "cb"
	inline := &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{{{Text: "A", CallbackData: &callback}}},
	}
	reply := &tgbotapi.ReplyKeyboardMarkup{
		Keyboard: [][]tgbotapi.KeyboardButton{{{Text: "R"}}},
	}

	bot.SendTextMessage(TelegramResponse{
		TextMsg:          "hello",
		TargetChatId:     321,
		MessageRepliedTo: 9,
		InlineKeyboard:   inline,
		ReplyMarkup:      reply,
	})

	req, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected sendMessage request to be sent")
	}
	if req.Values.Get("chat_id") != "321" {
		t.Fatalf("expected chat_id=321, got %q", req.Values.Get("chat_id"))
	}
	if req.Values.Get("reply_to_message_id") != "9" {
		t.Fatalf("expected reply_to_message_id=9, got %q", req.Values.Get("reply_to_message_id"))
	}
	if req.Values.Get("parse_mode") != tgbotapi.ModeMarkdown {
		t.Fatalf("expected parse_mode=%s, got %q", tgbotapi.ModeMarkdown, req.Values.Get("parse_mode"))
	}
	markup := req.Values.Get("reply_markup")
	if !strings.Contains(markup, "inline_keyboard") {
		t.Fatalf("expected inline keyboard markup, got %q", markup)
	}
	if strings.Contains(markup, "\"keyboard\"") {
		t.Fatalf("expected inline markup precedence over reply keyboard, got %q", markup)
	}
}

func TestEditTextMessageIncludesMessageIDAndInlineMarkup(t *testing.T) {
	bot, transport := newRecordingBot(t)
	callback := "cb"
	inline := &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{{{Text: "A", CallbackData: &callback}}},
	}

	bot.EditTextMessage(TelegramResponse{
		TextMsg:              "updated",
		TargetChatId:         654,
		MessageBeingEditedId: 77,
		InlineKeyboard:       inline,
	})

	req, ok := transport.lastEndpoint("editMessageText")
	if !ok {
		t.Fatal("expected editMessageText request to be sent")
	}
	if req.Values.Get("chat_id") != "654" {
		t.Fatalf("expected chat_id=654, got %q", req.Values.Get("chat_id"))
	}
	if req.Values.Get("message_id") != "77" {
		t.Fatalf("expected message_id=77, got %q", req.Values.Get("message_id"))
	}
	if req.Values.Get("text") != "updated" {
		t.Fatalf("expected edited text, got %q", req.Values.Get("text"))
	}
	if !strings.Contains(req.Values.Get("reply_markup"), "inline_keyboard") {
		t.Fatalf("expected inline keyboard in edit request, got %q", req.Values.Get("reply_markup"))
	}
}

func TestNewTelegramBotAPICreatesBotWithDefaultTransport(t *testing.T) {
	transport := &recordingTransport{}
	oldTransport := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() {
		http.DefaultTransport = oldTransport
	}()

	bot, err := NewTelegramBotAPI("local-test-token")
	if err != nil {
		t.Fatalf("expected NewTelegramBotAPI to succeed with test transport, got: %v", err)
	}
	if bot == nil || bot.BotAPI == nil {
		t.Fatal("expected non-nil bot from NewTelegramBotAPI")
	}
	if transport.countEndpoint("getMe") == 0 {
		t.Fatal("expected getMe request during NewTelegramBotAPI initialization")
	}
}

func TestInformAdminSendsOnlyWithValidAdminID(t *testing.T) {
	bot, transport := newRecordingBot(t)
	oldEnv := env
	defer func() {
		env = oldEnv
	}()

	env = map[string]string{"ADMIN_ID": "777"}
	bot.InformAdmin("news")

	req, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected sendMessage for valid ADMIN_ID")
	}
	if req.Values.Get("chat_id") != "777" {
		t.Fatalf("expected admin chat id 777, got %q", req.Values.Get("chat_id"))
	}
	if req.Values.Get("text") != "news" {
		t.Fatalf("expected admin news text, got %q", req.Values.Get("text"))
	}

	before := transport.countEndpoint("sendMessage")
	env = map[string]string{"ADMIN_ID": "invalid"}
	bot.InformAdmin("should-not-send")
	after := transport.countEndpoint("sendMessage")
	if before != after {
		t.Fatalf("expected no send for invalid ADMIN_ID, send count changed from %d to %d", before, after)
	}
}

func TestProcessNotificationTickSendsDueTogo(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	oldEnv := env
	defer func() {
		env = oldEnv
	}()
	env = map[string]string{"ADMIN_ID": "999"}

	now := Togo.Today()
	if _, err := (&Togo.Togo{Title: "due", OwnerId: 444, Weight: 1, Date: Togo.Date{Time: now.Add(time.Minute)}}).Save(); err != nil {
		t.Fatalf("failed to save due togo: %v", err)
	}
	if _, err := (&Togo.Togo{Title: "later", OwnerId: 444, Weight: 1, Date: Togo.Date{Time: now.Add(3 * time.Minute)}}).Save(); err != nil {
		t.Fatalf("failed to save non-due togo: %v", err)
	}

	notifiedCorruption := false
	notifiedLoad := false
	before := transport.countEndpoint("sendMessage")
	bot.processNotificationTick(&notifiedCorruption, &notifiedLoad)
	after := transport.countEndpoint("sendMessage")

	if after-before != 1 {
		t.Fatalf("expected exactly one sendMessage for due togo, got delta=%d", after-before)
	}
	req, ok := transport.lastEndpoint("sendMessage")
	if !ok {
		t.Fatal("expected final sendMessage request")
	}
	if req.Values.Get("chat_id") != "444" {
		t.Fatalf("expected due togo chat id 444, got %q", req.Values.Get("chat_id"))
	}
	if notifiedCorruption {
		t.Fatal("did not expect corruption notification flag to be set")
	}
	if notifiedLoad {
		t.Fatal("did not expect load-problem notification flag to be set")
	}
}

func TestProcessNotificationTickLoadFailureNotifiesAdminOnce(t *testing.T) {
	withTempWorkingDir(t, false)
	bot, transport := newRecordingBot(t)
	oldEnv := env
	defer func() {
		env = oldEnv
	}()
	env = map[string]string{"ADMIN_ID": "1234"}

	notifiedCorruption := false
	notifiedLoad := false

	before := transport.countEndpoint("sendMessage")
	bot.processNotificationTick(&notifiedCorruption, &notifiedLoad)
	afterFirst := transport.countEndpoint("sendMessage")
	if afterFirst-before != 1 {
		t.Fatalf("expected one admin notification on first load failure, got delta=%d", afterFirst-before)
	}
	if !notifiedLoad {
		t.Fatal("expected load-problem notification flag to be true after first failure")
	}

	bot.processNotificationTick(&notifiedCorruption, &notifiedLoad)
	afterSecond := transport.countEndpoint("sendMessage")
	if afterSecond != afterFirst {
		t.Fatalf("expected no repeated admin notification while failure persists, count changed %d -> %d", afterFirst, afterSecond)
	}
}

func TestBuildTaskPagesAndNavigation(t *testing.T) {
	now := time.Now()
	future := now.AddDate(0, 0, 1)
	tasks := Task.TaskList{
		{Id: 1, Title: "Active A", Weight: 1, Progress: 20},
		{Id: 2, Title: "Active B", Weight: 2, Progress: 0, Description: strings.Repeat("desc ", 20)},
		{Id: 3, Title: "Inactive", Weight: 1, Progress: 10, StartDate: &future},
	}

	pages := BuildTaskPages(tasks, true, false, 220)
	if len(pages) < 2 {
		t.Fatalf("expected multiple pages with constrained max size, got %d", len(pages))
	}

	firstNav := TaskPageNavigationKeyboard(0, len(pages), true, false)
	if firstNav == nil {
		t.Fatal("expected navigation keyboard for first page")
	}
	encodedFirst := firstNav.InlineKeyboard[0][0].CallbackData
	if encodedFirst == nil || !strings.Contains(*encodedFirst, `"TP":1`) {
		t.Fatalf("expected next-page callback for first page, got: %v", encodedFirst)
	}

	midNav := TaskPageNavigationKeyboard(1, len(pages), true, false)
	if midNav == nil || len(midNav.InlineKeyboard[0]) < 2 {
		t.Fatalf("expected prev/next buttons on middle page, got: %+v", midNav)
	}
}

func TestTaskReminderSlotBoundaries(t *testing.T) {
	dueTime := time.Date(2026, time.June, 6, 6, 0, 0, 0, time.Local)
	slot, due := taskReminderSlot(dueTime, 4)
	if !due {
		t.Fatal("expected 06:00 to be due for 4 reminders/day")
	}
	if slot != "2026-06-06-06" {
		t.Fatalf("unexpected slot format: %s", slot)
	}

	notDue := time.Date(2026, time.June, 6, 7, 0, 0, 0, time.Local)
	if _, ok := taskReminderSlot(notDue, 4); ok {
		t.Fatal("expected 07:00 not to be due for 4 reminders/day")
	}

	if _, ok := taskReminderSlot(dueTime, 0); ok {
		t.Fatal("expected disabled reminders (0/day) to never be due")
	}
}

func TestProcessTaskReminderTickSendsAndDeduplicatesSlot(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	oldEnv := env
	defer func() {
		env = oldEnv
	}()
	env = map[string]string{"ADMIN_ID": "555"}

	ownerID := int64(600)
	seed := &Task.Task{OwnerId: ownerID, Title: "Reminder task", Weight: 1, Progress: 20}
	if _, err := seed.Save(); err != nil {
		t.Fatalf("failed to save reminder task: %v", err)
	}
	if err := Task.SetReminderTimes(ownerID, 24); err != nil {
		t.Fatalf("failed to set reminders/day to 24: %v", err)
	}

	now := Togo.Date{Time: time.Date(2026, time.June, 6, 6, 0, 0, 0, time.Local)}
	before := transport.countEndpoint("sendMessage")
	bot.processTaskReminderTick(now)
	afterFirst := transport.countEndpoint("sendMessage")
	if afterFirst-before != 1 {
		t.Fatalf("expected one reminder send on first due slot, got delta=%d", afterFirst-before)
	}

	setting, err := Task.GetReminderSetting(ownerID)
	if err != nil {
		t.Fatalf("failed loading reminder setting after first tick: %v", err)
	}
	if setting.LastReminderSlot != "2026-06-06-06" {
		t.Fatalf("expected last reminder slot updated, got %q", setting.LastReminderSlot)
	}

	bot.processTaskReminderTick(now)
	afterSecond := transport.countEndpoint("sendMessage")
	if afterSecond != afterFirst {
		t.Fatalf("expected no duplicate reminder in same slot, count changed %d -> %d", afterFirst, afterSecond)
	}
}
