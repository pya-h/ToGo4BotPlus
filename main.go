package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	godotenv "github.com/joho/godotenv"
)

const (
	MaximumInlineButtonTextLength = 24
	MaximumNumberOfRowItems       = 3
	NumberOfSeparatorSpaces       = 2
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
	const HELP_MESSAGE = "WTF?\n```\n" +
		`## Commands
## +: New Togo:
=> +     title     [=  weight]      [+p     progress_till_now]     [:     description]      [+x | -x]     [@  start_date_as_how_many_days_from_now      start_time_as_hh:mm]      [...]

*     Flags order are optional, and Flags and their params must be seperated by 2 SPACES.
*     weight value can also be set by +w flag
*     description value can also be set by +d flag
## #: Show Togos
=>     #     [...]
      
	by default shows today's togos

=>     #     -     [...]
      
	Show incompleted togos.

=>     #     +a  [...] 
      
	Show all togos on any day

=>     #     -a     [...]
      
	Show all togos on any day, which are not completed yet.


## %: Progress Made:
=>     %     [...]
      
	Calculate the progress been made (by default for Today)

=>     %     -      [...]
      
	Calculate the progress been made, just considering the incompleted and ongoing togos.

=>     %     +a      [...]
      
	Calculate the progress been made, considering everything on any day.

=>     %     -a      [...]
      
	Calculate the progress been made considering all incompleted togos on any day.

## $: Get / Update a togo
=> $     id      [...]

     this will get and show a togo (just in today)

=> $     id     [=  weight]      [+p     progress_till_now]     [:     description]      [+x | -x]     [@  start_date_as_how_many_days_from_now      start_time_as_hh:mm]      [...]

## Other Notes:
*     [...] means that Bot supports chaining commands; You can chain any count of any of these commands and bot will do them in queue.
*     Each line can contain multiple command, as many as you want. Like:

=>     +     new_togo      @     1     10:00     +p  85  #  +     next_togo     +x  #   %

*   Extra:
=>        +x: its an extra Togo. its not mandatory but has extra points doing it.
=>        -x: not extra (default)
*   all params between [] are optional.


## Notes:
*   The flag list [& also commands] separator is 2 SPACES. space character will be evaluated as a part of the current flag's param. do not be mistaken.
*   in 'add new togo' syntax, all flags are optional except for the title, meaning that you can simply add new togos even with specifying the title only such as:
=>  +   new togo here
*   use a flag for % and # commands to expand the togos range to ALL.
*   use -a flag for % and # commands, to include All time togos, but only teh ones that are not done.` + "\n```"

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
		// Recover from panic for each update to keep the bot running
		func() {
			defer func() {
				if panicErr := recover(); panicErr != nil {
					log.Printf("Panic recovered while processing update: %v", panicErr)
					bot.InformAdmin(fmt.Sprintf("Panic during update processing: %v", panicErr))
				}
			}()

			response := TelegramResponse{TextMsg: HELP_MESSAGE}

			// ---------------------- Handling Casual Telegram text Messages ------------------------------
			if update.Message != nil { // If we got a message
				response.ReplyMarkup = MainKeyboardMenu() // default keyboard
				response.TargetChatId = update.Message.Chat.ID
				response.MessageRepliedTo = update.Message.MessageID
				terms := SplitArguments(update.Message.Text)

				numOfTerms := len(terms)

				var now Togo.Date = Togo.Today()
				for i := 0; i < numOfTerms; i++ {
					switch terms[i] {
					case "+":
						if i+1 < numOfTerms {
							if togo, err := Togo.Extract(update.Message.Chat.ID, terms[i+1:]); err == nil {
								if togo.Id, err = togo.Save(); err == nil {
									response.TextMsg = fmt.Sprint(now.Get(), ": DONE!")
								} else {
									response.TextMsg = err.Error()
								}
							} else {
								response.TextMsg = err.Error()
							}
						} else {
							response.TextMsg = "You must provide at least one Parameters!"
						}
					case TaskAddCommand:
						if i+1 < numOfTerms {
							if task, err := Task.Extract(update.Message.Chat.ID, terms[i+1:]); err == nil {
								if task.Id, err = task.Save(); err == nil {
									response.TextMsg = fmt.Sprintf("Task #%d created.", task.Id)
								} else {
									response.TextMsg = err.Error()
								}
							} else {
								response.TextMsg = err.Error()
							}
						} else {
							response.TextMsg = "You must provide at least one task title/parameter."
						}
					case "#", "#️⃣":
						justUndones := i+1 < numOfTerms && len(terms[i+1]) > 0 && terms[i+1][0] == '-'
						allDays := i+1 < numOfTerms && (terms[i+1] == "+a" || terms[i+1] == "-a")

						togos, warning := Togo.Load(update.Message.Chat.ID, !allDays, allDays && terms[i+1] == "-a")
						if togos == nil {
							if warning != nil {
								log.Println(warning)
							}
							response.TextMsg = BuildTogoImportStatsReport(nil, allDays, justUndones, warning)
							break
						}

						results := togos.ToString()
						for j := range results {
							if togos[j].Progress >= 100 {
								if justUndones {
									continue
								}
								response.TextMsg = fmt.Sprint("✅ ", results[j])
							} else {
								response.TextMsg = results[j]
							}
							bot.SendTextMessage(response)
						}

						response.TextMsg = BuildTogoImportStatsReport(togos, allDays, justUndones, warning)

					case "%":
						mode := ""
						if i+1 < numOfTerms {
							mode = strings.ToLower(terms[i+1])
						}

						switch mode {
						case TaskStatsToken:
							includeInactive := i+2 < numOfTerms && terms[i+2] == TaskIncludeInactiveToken
							tasks, warning := Task.Load(update.Message.Chat.ID, includeInactive, false)
							if tasks == nil {
								if warning != nil {
									response.TextMsg = warning.Error()
								} else {
									response.TextMsg = "Could not calculate task progress."
								}
								break
							}
							response.TextMsg = BuildTaskProgressReport(tasks, includeInactive, warning)
						case TaskBothStatsToken:
							allDaysToken := ""
							if i+2 < numOfTerms {
								allDaysToken = terms[i+2]
							}
							allDays := allDaysToken == "+a" || allDaysToken == "-a"
							includeInactive := i+3 < numOfTerms && terms[i+3] == TaskIncludeInactiveToken

							togos, togoWarning := Togo.Load(update.Message.Chat.ID, !allDays, allDays && allDaysToken == "-a")
							tasks, taskWarning := Task.Load(update.Message.Chat.ID, includeInactive, false)

							parts := make([]string, 0)
							if togos != nil {
								parts = append(parts, BuildTogoProgressReport(togos, allDays, togoWarning))
							} else if togoWarning != nil {
								parts = append(parts, fmt.Sprintf("Togo progress unavailable: %s", togoWarning.Error()))
							}
							if tasks != nil {
								parts = append(parts, BuildTaskProgressReport(tasks, includeInactive, taskWarning))
							} else if taskWarning != nil {
								parts = append(parts, fmt.Sprintf("Task progress unavailable: %s", taskWarning.Error()))
							}
							if len(parts) == 0 {
								response.TextMsg = "No progress data available."
							} else {
								response.TextMsg = strings.Join(parts, "\n\n")
							}
						default:
							all_days := i+1 < numOfTerms && (terms[i+1] == "+a" || terms[i+1] == "-a")
							togos, warning := Togo.Load(update.Message.Chat.ID, !all_days, all_days && terms[i+1] == "-a")
							if togos == nil {
								if warning != nil {
									log.Println(warning.Error())
									response.TextMsg = warning.Error()
								} else {
									response.TextMsg = "Could not calculate togo progress."
								}
								bot.SendTextMessage(response)
							} else {
								response.TextMsg = BuildTogoProgressReport(togos, all_days, warning)
							}
						}
					case "$":
						var togos Togo.TogoList
						var err error
						// set or update a togo
						if i+1 < numOfTerms {
							togos, err = Togo.Load(update.Message.Chat.ID, false, false)
							if togos != nil {
								if resp, err := togos.Update(update.Message.Chat.ID, terms[i+1:]); err == nil {
									response.TextMsg = resp
								} else {
									response.TextMsg = err.Error()
								}
							} else {
								response.TextMsg = err.Error()
							}

						} else {
							response.TextMsg = "You must provide the get identifier!"
						}
					case TaskUpdateCommand:
						if i+1 < numOfTerms {
							tasks, err := Task.Load(update.Message.Chat.ID, true, true)
							if tasks != nil {
								if resp, err := tasks.Update(update.Message.Chat.ID, terms[i+1:]); err == nil {
									response.TextMsg = resp
								} else {
									response.TextMsg = err.Error()
								}
							} else if err != nil {
								response.TextMsg = err.Error()
							} else {
								response.TextMsg = "Could not load tasks for update."
							}
						} else {
							response.TextMsg = "You must provide the task identifier!"
						}
					case TaskListCommand:
						includeInactive := i+1 < numOfTerms && terms[i+1] == TaskIncludeInactiveToken
						tasks, warning := Task.Load(update.Message.Chat.ID, includeInactive, false)
						if tasks == nil {
							if warning != nil {
								response.TextMsg = warning.Error()
							} else {
								response.TextMsg = "Could not load tasks."
							}
							break
						}

						pages := BuildTaskPages(tasks, includeInactive, false, MaximumTaskMessageLength)
						response.TextMsg = pages[0]
						response.InlineKeyboard = TaskPageNavigationKeyboard(0, len(pages), includeInactive, false)
						if warning != nil {
							if len(response.TextMsg)+len(warning.Error())+12 < MaximumTaskMessageLength {
								response.TextMsg = fmt.Sprintf("%s\n\nwarning: %s", response.TextMsg, warning.Error())
							} else {
								log.Printf("task list warning: %v", warning)
							}
						}
					case "✅":
						allDays := i+1 < numOfTerms && (terms[i+1] == "+a" || terms[i+1] == "-a")
						togos, err := Togo.Load(update.Message.Chat.ID, !allDays, allDays && terms[i+1] == "-a")
						if togos != nil {
							if len(togos) >= 1 {
								response.TextMsg = "Here are your togos for today:"
								response.InlineKeyboard = InlineKeyboardMenu(togos, TickTogo, allDays, allDays && terms[i+1] == "-a")
							} else {
								response.TextMsg = "No togos to tick!"
							}
							if err != nil {
								response.TextMsg = fmt.Sprintln(response.TextMsg, "- - - - - - - - - - - - - - - - - - - - - - - -\nseems: ", err.Error())
							}
						} else {
							response.TextMsg = err.Error()
						}
					case TaskTickCommand, "✅t":
						includeInactive := i+1 < numOfTerms && terms[i+1] == TaskIncludeInactiveToken
						tasks, warning := Task.Load(update.Message.Chat.ID, includeInactive, false)
						if tasks != nil {
							if len(tasks) >= 1 {
								response.TextMsg = "Here are your tasks to tick:"
								response.InlineKeyboard = TaskInlineKeyboardMenu(tasks, TickTask, includeInactive)
							} else {
								response.TextMsg = "No tasks to tick!"
							}
							if warning != nil {
								response.TextMsg = fmt.Sprintf("%s\nwarning: %s", response.TextMsg, warning.Error())
							}
						} else if warning != nil {
							response.TextMsg = warning.Error()
						} else {
							response.TextMsg = "Could not load tasks for ticking."
						}
					case "❌":
						var togos Togo.TogoList
						var err error
						allDays := i+1 < numOfTerms && (terms[i+1] == "+a" || terms[i+1] == "-a")

						if togos, err = Togo.Load(update.Message.Chat.ID, !allDays, allDays && terms[i+1] == "-a"); togos == nil {
							response.TextMsg = err.Error()
							bot.SendTextMessage(response)
						} else {
							if len(togos) >= 1 {
								response.TextMsg = "Here are your Today's togos:"
								if allDays {
									response.TextMsg = "Here are your ALL togos:"
								}
								if err != nil {
									response.TextMsg = fmt.Sprintln(response.TextMsg, "- - - - - - - - - - - - - - - - - - - - - - - -\n", err.Error())
								}
								response.InlineKeyboard = InlineKeyboardMenu(togos, RemoveTogo, allDays, allDays && terms[i+1] == "-a")
							} else {
								response.TextMsg = "No togos so far..."
							}
						}
					case TaskRemoveCommand, "❌t":
						includeInactive := i+1 < numOfTerms && terms[i+1] == TaskIncludeInactiveToken
						tasks, warning := Task.Load(update.Message.Chat.ID, includeInactive, false)
						if tasks != nil {
							if len(tasks) >= 1 {
								response.TextMsg = "Here are your tasks to remove:"
								response.InlineKeyboard = TaskInlineKeyboardMenu(tasks, RemoveTask, includeInactive)
							} else {
								response.TextMsg = "No tasks so far..."
							}
							if warning != nil {
								response.TextMsg = fmt.Sprintf("%s\nwarning: %s", response.TextMsg, warning.Error())
							}
						} else if warning != nil {
							response.TextMsg = warning.Error()
						} else {
							response.TextMsg = "Could not load tasks for removing."
						}
					case TaskSettingsCommand:
						if i+1 < numOfTerms {
							if times, err := strconv.Atoi(terms[i+1]); err == nil {
								if err := Task.SetReminderTimes(update.Message.Chat.ID, times); err == nil {
									if times == 0 {
										response.TextMsg = "Task reminders are now disabled (0 times/day)."
									} else {
										response.TextMsg = fmt.Sprintf("Task reminders updated to %d times/day.", times)
									}
								} else {
									response.TextMsg = err.Error()
								}
							} else {
								response.TextMsg = fmt.Sprintf("Usage: ~s  <times_per_day>\nAllowed values: %s", allowedTaskReminderValuesText())
							}
						} else if setting, err := Task.GetReminderSetting(update.Message.Chat.ID); err == nil {
							response.TextMsg = fmt.Sprintf("Current task reminder frequency: %d times/day\nAllowed values: %s", setting.RemindersPerDay, allowedTaskReminderValuesText())
						} else {
							response.TextMsg = err.Error()
						}
					case "/db":
						if adminId, err := strconv.Atoi(env["ADMIN_ID"]); err == nil && int64(adminId) == response.TargetChatId {
							msg := tgbotapi.NewDocumentUpload(int64(adminId), "./togos.db")
							if _, err := bot.Send(msg); err != nil {
								response.TextMsg = err.Error()
							} else {
								response.TextMsg = "Successfully sent db!"
							}
						} else {
							response.TextMsg = "get the fuck off my porch!"
						}
					case "/now":
						response.TextMsg = now.Get()

					}

				}
				bot.SendTextMessage(response)

			} else if update.CallbackQuery != nil {
				response.MessageBeingEditedId = update.CallbackQuery.Message.MessageID
				response.TargetChatId = update.CallbackQuery.Message.Chat.ID
				callbackData := LoadCallbackData(update.CallbackQuery.Data)

				switch callbackData.Action {
				case TickTogo, RemoveTogo:
					togos, err := Togo.Load(response.TargetChatId, !callbackData.AllDays, callbackData.JustUndones)
					if togos == nil {
						if err != nil {
							response.TextMsg = err.Error()
						} else {
							response.TextMsg = "Could not load togos for callback action."
						}
						break
					}

					if err != nil {
						response.TextMsg = err.Error()
						bot.SendTextMessage(response)
					}

					if callbackData.Action == TickTogo {
						togo, err := togos.Get(uint64(callbackData.ID))
						if err != nil {
							response.TextMsg = err.Error()
							break
						}
						if (*togo).Progress < 100 {
							(*togo).Progress = 100
						} else {
							(*togo).Progress = 0
						}
						_ = (*togo).Update(response.TargetChatId)
						response.InlineKeyboard = InlineKeyboardMenu(togos, TickTogo, callbackData.AllDays, callbackData.JustUndones)
						response.TextMsg = "✅ DONE! Now select the next togo you want to tick ..."
					} else {
						updated, err := togos.Remove(response.TargetChatId, uint64(callbackData.ID))
						if err != nil {
							response.TextMsg = err.Error()
							break
						}
						if len(updated) >= 1 {
							response.TextMsg = "❌ DONE! Now select the next togo you want to REMOVE ..."
							response.InlineKeyboard = InlineKeyboardMenu(updated, RemoveTogo, callbackData.AllDays, callbackData.JustUndones)
						} else {
							response.TextMsg = "❌ DONE! All removed."
						}
					}

				case TickTask:
					tasks, warning := Task.Load(response.TargetChatId, callbackData.TaskIncludeInactive, false)
					if tasks == nil {
						if warning != nil {
							response.TextMsg = warning.Error()
						} else {
							response.TextMsg = "Could not load tasks for ticking."
						}
						break
					}

					task, err := tasks.Get(uint64(callbackData.ID))
					if err != nil {
						response.TextMsg = err.Error()
						break
					}
					if task.Progress < 100 {
						task.Progress = 100
					} else {
						task.Progress = 0
					}
					if err := task.Update(response.TargetChatId); err != nil {
						response.TextMsg = err.Error()
						break
					}

					updated, warn2 := Task.Load(response.TargetChatId, callbackData.TaskIncludeInactive, false)
					if updated == nil {
						if warn2 != nil {
							response.TextMsg = warn2.Error()
						} else {
							response.TextMsg = "Task updated, but refresh failed."
						}
						break
					}
					if len(updated) >= 1 {
						response.TextMsg = "✅ Task updated. Pick the next task to tick ..."
						response.InlineKeyboard = TaskInlineKeyboardMenu(updated, TickTask, callbackData.TaskIncludeInactive)
					} else {
						response.TextMsg = "✅ Task updated. No remaining tasks in this view."
					}
					if warn2 != nil {
						response.TextMsg = fmt.Sprintf("%s\nwarning: %s", response.TextMsg, warn2.Error())
					}

				case RemoveTask:
					tasks, warning := Task.Load(response.TargetChatId, callbackData.TaskIncludeInactive, false)
					if tasks == nil {
						if warning != nil {
							response.TextMsg = warning.Error()
						} else {
							response.TextMsg = "Could not load tasks for removal."
						}
						break
					}

					updated, err := tasks.Remove(response.TargetChatId, uint64(callbackData.ID))
					if err != nil {
						response.TextMsg = err.Error()
						break
					}
					if len(updated) >= 1 {
						response.TextMsg = "❌ Task removed. Pick the next task to remove ..."
						response.InlineKeyboard = TaskInlineKeyboardMenu(updated, RemoveTask, callbackData.TaskIncludeInactive)
					} else {
						response.TextMsg = "❌ Task removed. All removed in this view."
					}

				case ShowTaskPage:
					tasks, warning := Task.Load(response.TargetChatId, callbackData.TaskIncludeInactive, false)
					if tasks == nil {
						if warning != nil {
							response.TextMsg = warning.Error()
						} else {
							response.TextMsg = "Could not load tasks page."
						}
						break
					}

					pages := BuildTaskPages(tasks, callbackData.TaskIncludeInactive, callbackData.TaskReminderMode, MaximumTaskMessageLength)
					page := callbackData.TaskPage
					if page < 0 {
						page = 0
					}
					if page >= len(pages) {
						page = len(pages) - 1
					}
					response.TextMsg = pages[page]
					response.InlineKeyboard = TaskPageNavigationKeyboard(page, len(pages), callbackData.TaskIncludeInactive, callbackData.TaskReminderMode)
					if warning != nil {
						response.TextMsg = fmt.Sprintf("%s\n\nwarning: %s", response.TextMsg, warning.Error())
					}

				default:
					response.TextMsg = "Unsupported callback action."
				}

				bot.EditTextMessage(response)
			}
		}() // Close panic recovery anonymous function
	}
}
