package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"
	"unicode/utf8"

	"ToGo4BotPlus/Article"
	"ToGo4BotPlus/Idea"
	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	godotenv "github.com/joho/godotenv"
)

type TelegramResponse struct {
	TextMsg              string                         `json:"text,omitempty"`
	TargetChatId         int64                          `json:"chat_id"`
	MessageRepliedTo     int                            `json:"reply_to_message_id,omitempty"`
	MessageBeingEditedId int                            `json:"message_id,omitempty"` // for edit message & etc
	ReplyMarkup          *tgbotapi.ReplyKeyboardMarkup  `json:"reply_markup,omitempty"`
	InlineKeyboard       *tgbotapi.InlineKeyboardMarkup `json:"inline_keyboard,omitempty"`
	// file/photo?
}

type TelegramBotAPI struct {
	*tgbotapi.BotAPI
	flows         *FlowStore     // in-memory guided-flow (Type B) conversation state
	ideaReminders *ReminderStore // in-memory favorite-idea reminder schedule (per owner)
	imports       *importWaitSet // in-memory set of owners awaiting a /import file upload
}

func (telegramBotAPI *TelegramBotAPI) SendTextMessage(response TelegramResponse) {
	msg := tgbotapi.NewMessage(response.TargetChatId, response.TextMsg)
	msg.ReplyToMessageID = response.MessageRepliedTo
	if response.InlineKeyboard != nil {
		msg.ReplyMarkup = response.InlineKeyboard
	} else if response.ReplyMarkup != nil {
		msg.ReplyMarkup = response.ReplyMarkup
	}
	msg.ParseMode = tgbotapi.ModeMarkdown
	if _, err := telegramBotAPI.Send(msg); err != nil {
		// Freeform user content (togo titles, idea text, ...) can contain
		// unbalanced Markdown (* _ ` [ ) which makes Telegram reject the whole
		// message with a 400. Rather than silently dropping it, retry once as
		// plain text so the content still reaches the user.
		plain := tgbotapi.NewMessage(response.TargetChatId, response.TextMsg)
		plain.ReplyToMessageID = response.MessageRepliedTo
		if response.InlineKeyboard != nil {
			plain.ReplyMarkup = response.InlineKeyboard
		} else if response.ReplyMarkup != nil {
			plain.ReplyMarkup = response.ReplyMarkup
		}
		telegramBotAPI.Send(plain)
	}
}

// SendTextMessageReturningID sends a message and returns the resulting Telegram
// message id so guided flows can keep editing that same "wizard" message. It
// deliberately sends as plain text (no Markdown) since flow prompts and summaries
// may interpolate arbitrary user content.
func (telegramBotAPI *TelegramBotAPI) SendTextMessageReturningID(response TelegramResponse) (int, error) {
	msg := tgbotapi.NewMessage(response.TargetChatId, response.TextMsg)
	msg.ReplyToMessageID = response.MessageRepliedTo
	if response.InlineKeyboard != nil {
		msg.ReplyMarkup = response.InlineKeyboard
	} else if response.ReplyMarkup != nil {
		msg.ReplyMarkup = response.ReplyMarkup
	}
	sent, err := telegramBotAPI.Send(msg)
	return sent.MessageID, err
}

// DeleteMessage removes a message (best-effort). Used to clear a user's typed
// input once a guided flow has consumed it, so the conversation isn't littered
// with seemingly-unanswered inputs. In private chats a bot may delete incoming
// user messages; failures (permissions, >48h, already gone) are ignored.
func (telegramBotAPI *TelegramBotAPI) DeleteMessage(chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	telegramBotAPI.Send(tgbotapi.NewDeleteMessage(chatID, messageID))
}

func (telegramBotAPI *TelegramBotAPI) EditTextMessage(response TelegramResponse) {
	msg := tgbotapi.NewEditMessageText(response.TargetChatId, response.MessageBeingEditedId, response.TextMsg)
	if response.InlineKeyboard != nil {
		msg.ReplyMarkup = response.InlineKeyboard
	}
	telegramBotAPI.Send(msg)
}

func NewTelegramBotAPI(token string) (*TelegramBotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	return &TelegramBotAPI{BotAPI: bot, flows: NewFlowStore(), ideaReminders: NewReminderStore()}, err
}

// registerBotCommands publishes the guided-flow slash commands to Telegram's
// native "/" command list. This tgbotapi version predates SetMyCommands, so we
// call the raw endpoint. Best-effort: failures are logged, not fatal.
func (telegramBot *TelegramBotAPI) registerBotCommands() {
	type botCommand struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	// Grouped so the native "/" list reads logically: browse first (favorites sits
	// next to ideas), then add, then remove, then everything else.
	commands := []botCommand{
		// — Browse —
		{"togos", "📋 Browse your togos (interactive)"},
		{"tasks", "📋 Browse your tasks (interactive)"},
		{"ideas", "📋 Browse your ideas (interactive)"},
		{"favorites", "⭐ Browse your favorite ideas"},
		{"articles", "📋 Browse your saved articles (interactive)"},
		// — Add —
		{"addtogo", "➕ Add a togo (guided)"},
		{"addtask", "➕ Add a task (guided)"},
		{"addidea", "➕ Add an idea (guided)"},
		{"addarticle", "➕ Save an article link (guided)"},
		// — Remove —
		{"removetodaytogos", "🗑 Remove today's togos (interactive)"},
		{"removealltogos", "🗑 Remove togos from any day (interactive)"},
		// — Other —
		{"taskreminder", "⏰ Show/Set task reminder frequency"},
		{"tasksperreminder", "🎲 Show/Set how many tasks per reminder (1–10)"},
		{"json", "📦 Export all your data as a JSON file"},
		{"import", "📥 Import data from a /json export file"},
		{"now", "🕒 Show current date/time"},
		{"cancel", "✖️ Cancel the current guided menu"},
		{"help", "❓ Show the full command help"},
		{"start", "🔄 Restart the bot and show the menu"},
	}
	payload, err := json.Marshal(commands)
	if err != nil {
		log.Printf("could not encode bot commands: %v", err)
		return
	}
	params := url.Values{}
	params.Add("commands", string(payload))
	if _, err := telegramBot.MakeRequest("setMyCommands", params); err != nil {
		log.Printf("could not register bot commands: %v", err)
	}
}

// ---------------------- Callback Structs & Functions --------------------------------
type UserAction uint8

const (
	None UserAction = iota
	TickTogo
	UpdateTogo
	RemoveTogo
	TickTask
	RemoveTask
	ShowTaskPage
	ShowTogoMenuPage
	ShowTaskMenuPage
	RemoveIdea
	ShowIdeaMenuPage
	FlowSelect
	FlowCustom
	FlowSkip
	FlowBack
	FlowConfirm
	FlowCancel
	FlowEdit
	FlowDelete
	FlowToggle
	IdeaMenuList   // render a page of the interactive idea browser
	IdeaMenuOpen   // open one idea's detail card in the browser
	IdeaMenuFav    // toggle an idea's favorite flag from the browser
	IdeaMenuRemove // delete an idea from the browser, return to the list
	IdeaMenuEdit   // hand the browser message off to the manage-flow edit screens
	RemoveArticle  // legacy `>x` paginated remove menu
	ShowArticleMenuPage
	ArticleMenuList   // render a page of the interactive article browser
	ArticleMenuOpen   // open one article's detail card in the browser
	ArticleMenuRemove     // delete an article from the browser, return to the list
	ArticleMenuEdit       // hand the browser message off to the manage-flow edit screens
	ArticleMenuToggleRead // flip an article's read flag (from the browser card or a reminder)
	TogoMenuList      // render a page of the interactive togo browser
	TogoMenuOpen      // open one togo's detail card in the browser
	TogoMenuRemove    // delete a togo from the browser, return to the list
	TogoMenuToggle    // toggle a togo's done state from the browser
	TogoMenuEdit      // hand the togo-browser message off to the edit screens
	TaskMenuList      // render a page of the interactive task browser
	TaskMenuOpen      // open one task's detail card in the browser
	TaskMenuRemove    // delete a task from the browser, return to the list
	TaskMenuToggle    // toggle a task's done state from the browser
	TaskMenuEdit      // hand the task-browser message off to the edit screens
	PortTogo          // shift an undone togo's date +1 day from the midnight port-reminder
)

type CallbackData struct {
	Action              UserAction  `json:"A"`
	ID                  int64       `json:"ID,omitempty"`
	Data                interface{} `json:"D,omitempty"`
	AllDays             bool        `json:"AD,omitempty"`
	JustUndones         bool        `json:"JU,omitempty"`
	TaskPage            int         `json:"TP,omitempty"`
	TaskIncludeInactive bool        `json:"TI,omitempty"`
	TaskReminderMode    bool        `json:"TR,omitempty"`
	MenuPage            int         `json:"MP,omitempty"` // current page of a paginated tick/remove inline menu
	MenuAction          UserAction  `json:"MX,omitempty"` // the tick/remove action a menu-navigation button should re-render
	FlowOpt             int         `json:"FO,omitempty"` // selected option index within an active guided flow step
	IdeaScope           int         `json:"IK,omitempty"` // idea-browser scope (all / high / favorites / category)
	IdeaCat             int64       `json:"IC,omitempty"` // category id when the idea-browser scope is "category"
	ArtCat              int64       `json:"AC,omitempty"` // category id filter for the article browser (0 = all)
	ArtReminder         bool        `json:"AR,omitempty"` // toggle-read came from a daily reminder, not the browser card
}

func (callbackData CallbackData) Json() string {
	if res, err := json.Marshal(callbackData); err == nil {
		return string(res)
	} else {
		return fmt.Sprint(err)
	}
}

func LoadCallbackData(jsonString string) (data CallbackData) {
	if err := json.Unmarshal([]byte(jsonString), &data); err != nil {
		log.Printf("Warning: corrupted callback data, json.Unmarshal error: %v", err)
	}
	return
}

// ---------------------- Global Vars --------------------------------
var env map[string]string

// ---------------------- Telegram Response Related Functions ------------------------------
func truncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	for maxBytes > 0 {
		truncated := s[:maxBytes]
		if utf8.ValidString(truncated) {
			return truncated
		}
		maxBytes--
	}

	return ""
}

// packButtonsIntoRows lays a flat list of inline buttons out into rows of at
// most perRow buttons each, so a browser's item buttons fill the message width
// instead of stacking one-per-row.
func packButtonsIntoRows(buttons []tgbotapi.InlineKeyboardButton, perRow int) [][]tgbotapi.InlineKeyboardButton {
	if perRow < 1 {
		perRow = 1
	}
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, (len(buttons)+perRow-1)/perRow)
	for start := 0; start < len(buttons); start += perRow {
		end := start + perRow
		if end > len(buttons) {
			end = len(buttons)
		}
		rows = append(rows, buttons[start:end])
	}
	return rows
}

// menuPageCount returns how many pages of inline buttons are needed for count
// items (always at least 1, so callers can safely index page 0).
func menuPageCount(count int) int {
	pages := (count + MaximumInlineMenuItems - 1) / MaximumInlineMenuItems
	if pages < 1 {
		return 1
	}
	return pages
}

// buildMenuNavRow builds the ⬅️ Prev / page indicator / Next ➡️ row for a
// paginated tick/remove menu. It returns nil when there is only a single page.
// template carries the menu's scope flags (AllDays/JustUndones or
// TaskIncludeInactive); this function fills in the navigation action, the menu
// action to re-render, and the target page for each button.
func buildMenuNavRow(navAction UserAction, menuAction UserAction, page int, totalPages int, template CallbackData) []tgbotapi.InlineKeyboardButton {
	if totalPages <= 1 {
		return nil
	}
	template.Action = navAction
	template.MenuAction = menuAction

	row := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	if page > 0 {
		prev := template
		prev.MenuPage = page - 1
		prevData := prev.Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "⬅️ Prev", CallbackData: &prevData})
	}

	indicator := template
	indicator.MenuPage = page
	indicatorData := indicator.Json()
	row = append(row, tgbotapi.InlineKeyboardButton{Text: fmt.Sprintf("%d/%d", page+1, totalPages), CallbackData: &indicatorData})

	if page < totalPages-1 {
		next := template
		next.MenuPage = page + 1
		nextData := next.Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "Next ➡️", CallbackData: &nextData})
	}
	return row
}

func InlineKeyboardMenu(togos Togo.TogoList, action UserAction, allDays bool, justUndones bool, page int) (inlineKeyboard *tgbotapi.InlineKeyboardMarkup) {
	total := len(togos)
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
	pageItems := togos[start:end]
	count := len(pageItems)

	rowsCount := count / MaximumNumberOfRowItems
	if count%MaximumNumberOfRowItems != 0 {
		rowsCount++
	}

	var menu tgbotapi.InlineKeyboardMarkup
	menu.InlineKeyboard = make([][]tgbotapi.InlineKeyboardButton, 0, rowsCount+1)

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
			var togoTitle string = fmt.Sprint(status, pageItems[k].Title)
			if len(togoTitle) >= MaximumInlineButtonTextLength {
				togoTitle = fmt.Sprint(truncateUTF8(togoTitle, MaximumInlineButtonTextLength-3), "...")
			}
			data := (CallbackData{Action: action, ID: int64(pageItems[k].Id), AllDays: allDays, JustUndones: justUndones, MenuPage: page}).Json()
			buttons = append(buttons, tgbotapi.InlineKeyboardButton{Text: togoTitle, CallbackData: &data})
		}
		menu.InlineKeyboard = append(menu.InlineKeyboard, buttons)
	}

	if navRow := buildMenuNavRow(ShowTogoMenuPage, action, page, totalPages, CallbackData{AllDays: allDays, JustUndones: justUndones}); navRow != nil {
		menu.InlineKeyboard = append(menu.InlineKeyboard, navRow)
	}

	inlineKeyboard = &menu
	return
}

func MainKeyboardMenu() *tgbotapi.ReplyKeyboardMarkup {
	return &tgbotapi.ReplyKeyboardMarkup{ResizeKeyboard: true,
		OneTimeKeyboard: false,
		// Row 1: show togos. Row 2: tick togos + togo progress (merged). Row 3:
		// every task action on one line. Row 4: ideas. Row 5: articles. Togo
		// removal and task-reminder setting moved to slash commands.
		Keyboard: [][]tgbotapi.KeyboardButton{
			{{Text: "#️⃣"}, {Text: "#️⃣  -"}, {Text: "#️⃣  +a"}, {Text: "#️⃣  -a"}},
			{{Text: "✅"}, {Text: "✅  -a"}, {Text: "✅  +a"}, {Text: "%"}, {Text: "%  +a"}},
			{{Text: "~"}, {Text: "~  +i"}, {Text: "%  t"}, {Text: "✅T"}, {Text: "❌T"}},
			{{Text: ";"}, {Text: ";  !"}, {Text: "*x"}},
			{{Text: ">l"}, {Text: ">x"}},
		}}
}

func BuildTogoImportStatsReport(togos Togo.TogoList, allDays bool, justUndones bool, warning error) string {
	scope := "Today"
	if allDays {
		scope = "All days"
	}

	imported := len(togos)
	shown := 0
	done := 0
	pending := 0
	totalProgress := 0

	for _, togo := range togos {
		if justUndones && togo.Progress >= 100 {
			continue
		}
		shown++
		totalProgress += int(togo.Progress)
		if togo.Progress >= 100 {
			done++
		} else {
			pending++
		}
	}

	averageProgress := 0.0
	if shown > 0 {
		averageProgress = float64(totalProgress) / float64(shown)
	}

	report := fmt.Sprintf("%s report\nImported togos: %d\nShown: %d\nDone: %d\nPending: %d\nAverage progress: %.2f%%",
		scope, imported, shown, done, pending, averageProgress)

	if warning != nil {
		report = fmt.Sprintf("%s\nWarning: %s", report, warning.Error())
	} else if shown == 0 {
		report = fmt.Sprintf("%s\nStatus: Nothing to show.", report)
	}

	return report
}

func SplitArguments(statement string) []string {
	result := make([]string, 0)
	numOfSpaces := 0
	segmentStartIndex := 0

	for i := range statement {
		if statement[i] == ' ' {
			numOfSpaces++
		} else if numOfSpaces > 0 {
			if numOfSpaces == NumberOfSeparatorSpaces {
				result = append(result, statement[segmentStartIndex:i-NumberOfSeparatorSpaces])
				segmentStartIndex = i
			}
			numOfSpaces = 0
		}

	}
	result = append(result, statement[segmentStartIndex:])
	return result
}

func (telegramBot *TelegramBotAPI) InformAdmin(news string) {
	if admin_id, err := strconv.Atoi(env["ADMIN_ID"]); err == nil {
		response := TelegramResponse{TextMsg: news, TargetChatId: int64(admin_id)} // default method is sendMessage
		telegramBot.SendTextMessage(response)
	} else {
		log.Println("Cannot get admin id to inform him/her; news is: ", news)
	}
}

func togosDueAtNextMinute(togos Togo.TogoList, now Togo.Date) Togo.TogoList {
	nextMinute := (Togo.Date{Time: now.Add(time.Minute)}).ToLocal()
	due := make(Togo.TogoList, 0)
	for _, togo := range togos {
		if togo.Date.Get() == nextMinute.Get() {
			due = due.Add(&togo)
		}
	}
	return due
}

func (telegramBot *TelegramBotAPI) processNotificationTick(notifiedAboutCorruption *bool, notifiedAboutLoadProblem *bool) {
	if togos, err := Togo.LoadEverybodysToday(); togos != nil {
		*notifiedAboutLoadProblem = false
		if err != nil {
			if !*notifiedAboutCorruption {
				*notifiedAboutCorruption = true

				telegramBot.InformAdmin(fmt.Sprintln(err.Error(), "; this means the notification may encounter some problems on notifying some togos."))
			}
		} else {
			*notifiedAboutCorruption = false
		}

		for _, togo := range togosDueAtNextMinute(togos, Togo.Today()) {
			response := TelegramResponse{TextMsg: togo.ToString(), TargetChatId: togo.OwnerId} // default method is sendMessage
			telegramBot.SendTextMessage(response)
		}
	} else {
		if !*notifiedAboutLoadProblem {
			*notifiedAboutLoadProblem = true
			telegramBot.InformAdmin(err.Error())
		}
	}
}

func (telegramBot *TelegramBotAPI) NotifyRightNowTogos() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	notified_about_curroption := false
	notified_about_load_problem := false
	for range ticker.C {
		telegramBot.processNotificationTick(&notified_about_curroption, &notified_about_load_problem)
		telegramBot.processTaskReminderTick(Togo.Today())
	}
}

func main() {
	var token string

	env = nil
	if envFile, err := godotenv.Read(".env"); err != nil {
		panic(err)
	} else {
		env = envFile
		token = env["TOKEN"]
	}

	bot, err := NewTelegramBotAPI(token)
	if err != nil {
		panic(err)
	}

	// Initialize database (create table, enable WAL mode)
	if err := Togo.InitDatabase(); err != nil {
		panic(err)
	}
	if err := Task.InitDatabase(); err != nil {
		panic(err)
	}
	if err := Idea.InitDatabase(); err != nil {
		panic(err)
	}
	if err := Article.InitDatabase(); err != nil {
		panic(err)
	}

	bot.registerBotCommands()

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates, err := bot.GetUpdatesChan(updateConfig)
	if err != nil {
		panic(err)
	}

	go bot.NotifyRightNowTogos() // run the scheduler that will check which togos are hapening right now, for each user
	go bot.RemindIdeas() // hourly: nudge users about a random batch of their favorite / high-priority ideas
	go bot.RemindArticles()      // daily at ArticleReminderHour: send each user a random saved article
	go bot.RemindPortableTogos() // daily at 00:00: ask each user to port their undone togos to tomorrow
	log.Println("configured.")
	for update := range updates {
		bot.HandleUpdate(update)
	}
}
