package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"
	"unicode/utf8"

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
	telegramBotAPI.Send(msg)

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
	return &TelegramBotAPI{BotAPI: bot}, err
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

func InlineKeyboardMenu(togos Togo.TogoList, action UserAction, allDays bool, justUndones bool) (inlineKeyboard *tgbotapi.InlineKeyboardMarkup) {
	var (
		count     = len(togos)
		col       = 0
		row       = 0
		rowsCount = int(count / MaximumNumberOfRowItems)
	) // calculate the number of rows needed
	if count%MaximumNumberOfRowItems != 0 {
		rowsCount++
	}
	var menu tgbotapi.InlineKeyboardMarkup
	menu.InlineKeyboard = make([][]tgbotapi.InlineKeyboardButton, rowsCount)

	for i := range togos {
		if col == 0 {
			// calculting the number of column needed in each row
			if row < rowsCount-1 {
				menu.InlineKeyboard[row] = make([]tgbotapi.InlineKeyboardButton, MaximumNumberOfRowItems)
			} else {
				menu.InlineKeyboard[row] = make([]tgbotapi.InlineKeyboardButton, count-row*MaximumNumberOfRowItems)
			}
			row++
		}
		status := ""
		if togos[i].Progress >= 100 {
			status = "✅ "
		}
		var togoTitle string = fmt.Sprint(status, togos[i].Title)
		if len(togoTitle) >= MaximumInlineButtonTextLength {
			togoTitle = fmt.Sprint(truncateUTF8(togoTitle, MaximumInlineButtonTextLength-3), "...")
		}
		data := (CallbackData{Action: action, ID: int64(togos[i].Id), AllDays: allDays, JustUndones: justUndones}).Json()
		menu.InlineKeyboard[row-1][col] = tgbotapi.InlineKeyboardButton{Text: togoTitle,
			CallbackData: &data}
		col = (col + 1) % MaximumNumberOfRowItems
	}
	inlineKeyboard = &menu
	return
}

func MainKeyboardMenu() *tgbotapi.ReplyKeyboardMarkup {
	return &tgbotapi.ReplyKeyboardMarkup{ResizeKeyboard: true,
		OneTimeKeyboard: false,
		Keyboard: [][]tgbotapi.KeyboardButton{{tgbotapi.KeyboardButton{Text: "#️⃣"}, tgbotapi.KeyboardButton{Text: "#️⃣  -"}, tgbotapi.KeyboardButton{Text: "#️⃣  +a"}, tgbotapi.KeyboardButton{Text: "#️⃣  -a"}},
			{tgbotapi.KeyboardButton{Text: "✅"}, tgbotapi.KeyboardButton{Text: "✅  -a"}, tgbotapi.KeyboardButton{Text: "✅  +a"}},
			{tgbotapi.KeyboardButton{Text: "%"}, tgbotapi.KeyboardButton{Text: "%  +a"}},
			{tgbotapi.KeyboardButton{Text: "❌"}, tgbotapi.KeyboardButton{Text: "❌  -a"}, tgbotapi.KeyboardButton{Text: "❌  +a"}},
			{tgbotapi.KeyboardButton{Text: "~"}, tgbotapi.KeyboardButton{Text: "~  +i"}, tgbotapi.KeyboardButton{Text: "%  t"}},
			{tgbotapi.KeyboardButton{Text: "✅T"}, tgbotapi.KeyboardButton{Text: "❌T"}, tgbotapi.KeyboardButton{Text: "~s  4"}},
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

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates, err := bot.GetUpdatesChan(updateConfig)
	if err != nil {
		panic(err)
	}

	go bot.NotifyRightNowTogos() // run the scheduler that will check which togos are hapening right now, for each user
	log.Println("configured.")
	for update := range updates {
		bot.HandleUpdate(update)
	}
}
