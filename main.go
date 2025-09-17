package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	godotenv "github.com/joho/godotenv"
	Togo "github.com/pya-h/ToGo4BotPlus/Togo"
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

// ---------------------- Telegram Response Struct & Interfaces --------------------------------
type TelegramAPIMethods interface {
	SendTextMessage(response TelegramResponse)
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
)

type CallbackData struct {
	Action      UserAction  `json:"A"`
	ID          int64       `json:"ID,omitempty"`
	Data        interface{} `json:"D,omitempty"`
	AllDays     bool        `json:"AD,omitempty"`
	JustUndones bool        `json:"JU,omitempty"`
}

func (callbackData CallbackData) Json() string {
	if res, err := json.Marshal(callbackData); err == nil {
		return string(res)
	} else {
		return fmt.Sprint(err)
	}
}

func LoadCallbackData(jsonString string) (data CallbackData) {
	json.Unmarshal([]byte(jsonString), &data)
	return
}

// ---------------------- Global Vars --------------------------------
var env map[string]string

// ---------------------- Telegram Response Related Functions ------------------------------
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
			if justUndones {
				continue
			}
			status = "✅ "
		}
		var togoTitle string = fmt.Sprint(status, togos[i].Title)
		if len(togoTitle) >= MaximumInlineButtonTextLength {
			togoTitle = fmt.Sprint(togoTitle[:MaximumInlineButtonTextLength], "...")
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
		Keyboard: [][]tgbotapi.KeyboardButton{{tgbotapi.KeyboardButton{Text: "#"}, tgbotapi.KeyboardButton{Text: "#  -"}, tgbotapi.KeyboardButton{Text: "#  +a"}, tgbotapi.KeyboardButton{Text: "#  -a"}},
			{tgbotapi.KeyboardButton{Text: "%"}, tgbotapi.KeyboardButton{Text: "%  a"}},
			{tgbotapi.KeyboardButton{Text: "❌"}, tgbotapi.KeyboardButton{Text: "❌  -a"}, tgbotapi.KeyboardButton{Text: "❌  a"}},
			{tgbotapi.KeyboardButton{Text: "✅"}, tgbotapi.KeyboardButton{Text: "✅  a"}},
		}}
}

// ---------------------- tgbotapi Related Functions ------------------------------
func GetTgBotApiFunction(update *tgbotapi.Update) func(data string) error {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TOKEN"))
	return func(data string) error {
		if err == nil {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, data)
			// msg.ReplyToMessageID = update.Message.MessageID
			bot.Send(msg)
			return nil
		}
		return err
	}
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

func Log(update *tgbotapi.Update, values []string) {
	sendMessage := GetTgBotApiFunction(update)
	var r string
	for _, v := range values {
		r += v + "\n"
	}
	sendMessage(r)
}

func (telegramBot *TelegramBotAPI) InformAdmin(news string) {
	if admin_id, err := strconv.Atoi(env["ADMIN_ID"]); err == nil {
		response := TelegramResponse{TextMsg: news, TargetChatId: int64(admin_id)} // default method is sendMessage
		telegramBot.SendTextMessage(response)
	} else {
		log.Println("Cannot get admin id to inform him/her; news is: ", news)
	}
}

func (telegramBot *TelegramBotAPI) NotifyRightNowTogos() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	notified_about_curroption := false
	notified_about_load_problem := false
	for range ticker.C {
		if togos, err := Togo.LoadEverybodysToday(); togos != nil {
			notified_about_load_problem = false
			if err != nil {
				if !notified_about_curroption {
					notified_about_curroption = true

					telegramBot.InformAdmin(fmt.Sprintln(err.Error(), "; this means the notification may encounter some problems on notifying some togos."))
				}
			} else {
				notified_about_curroption = false
			}
			nextMinute := (Togo.Date{Time: Togo.Today().Add(time.Minute * time.Duration(1))}).ToLocal()

			for _, togo := range togos {
				if togo.Date.Get() == nextMinute.Get() {
					response := TelegramResponse{TextMsg: togo.ToString(), TargetChatId: togo.OwnerId} // default method is sendMessage
					telegramBot.SendTextMessage(response)
				}
			}
		} else {
			if !notified_about_load_problem {
				notified_about_load_problem = true
				telegramBot.InformAdmin(err.Error())
			}
		}
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

	defer func() {
		err := recover()
		if err != nil {
			log.Println(err)
		}
	}()

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates, err := bot.GetUpdatesChan(updateConfig)
	if err != nil {
		panic(err)
	}

	go bot.NotifyRightNowTogos() // run the scheduler that will check which togos are hapening right now, for each user
	log.Println("configured.")
	for update := range updates {
		response := TelegramResponse{TextMsg: "What?"}

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
					if numOfTerms > 1 {
						var err error
						togo := Togo.Extract(update.Message.Chat.ID, terms[i+1:])
						if togo.Id, err = togo.Save(); err == nil {
							response.TextMsg = fmt.Sprint(now.Get(), ": DONE!")
						} else {
							response.TextMsg = err.Error()
						}
					} else {
						response.TextMsg = "You must provide at least one Parameters!"
					}
				case "#":
					var results []string
					just_undones := i+1 < numOfTerms && terms[i+1][0] == '-'
					all_days := i+1 < numOfTerms && (terms[i+1] == "+a" || terms[i+1] == "-a")

					togos, warning := Togo.Load(update.Message.Chat.ID, !all_days)
					if togos == nil {
						log.Println(warning)
						response.TextMsg = warning.Error()
						bot.SendTextMessage(response)
					}
					results = togos.ToString()
					if len(results) > 0 {
						for i := range results {
							if togos[i].Progress >= 100 {
								if just_undones {
									continue
								}
								response.TextMsg = fmt.Sprint("✅ ", results[i])
							} else {
								response.TextMsg = results[i]
							}
							bot.SendTextMessage(response)
						}
						if warning == nil {
							response.TextMsg = "✅!"
						} else {
							response.TextMsg = warning.Error()
						}
					} else {
						response.TextMsg = "Nothing!"
					}

				case "%":
					var togos Togo.TogoList
					var warning error
					all_days := i+1 < numOfTerms && terms[i+1] == "a"

					togos, warning = Togo.Load(update.Message.Chat.ID, !all_days)
					if togos == nil {
						log.Println(warning.Error())
						response.TextMsg = warning.Error()
						bot.SendTextMessage(response)
					} else {
						progress, completedInPercent, completed, extra, total := togos.ProgressMade()
						scope := "Today's"
						if all_days {
							scope = "Total"
						}
						response.TextMsg = fmt.Sprintf("%s Progress: %3.2f%% \n%3.2f%% Completed\nStatistics: %d / %d\n",
							scope, progress, completedInPercent, completed, total)
						if extra > 0 {
							response.TextMsg = fmt.Sprintf("%s[+%d]\n", response.TextMsg, extra)
						}
						if warning != nil {
							response.TextMsg = fmt.Sprintln(response.TextMsg, "- - - - - - - - - - - - - - - - - - - - - - \nwarning: ", warning.Error())
						}
					}
				case "$":
					var togos Togo.TogoList
					var err error
					// set or update a togo
					if i+1 < numOfTerms {
						togos, err = Togo.Load(update.Message.Chat.ID, false)
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
				case "✅":
					allDays := i+1 < numOfTerms && (terms[i+1] == "a" || terms[i+1] == "-a")
					togos, err := Togo.Load(update.Message.Chat.ID, !allDays)
					if togos != nil {
						if len(togos) >= 1 {
							response.TextMsg = "Here are your togos for today:"
							response.InlineKeyboard = InlineKeyboardMenu(togos, TickTogo, allDays, terms[i+1] == "-a")
						} else {
							response.TextMsg = "No togos to tick!"
						}
						if err != nil {
							response.TextMsg = fmt.Sprintln(response.TextMsg, "- - - - - - - - - - - - - - - - - - - - - - - -\nseems: ", err.Error())
						}
					} else {
						response.TextMsg = err.Error()
					}
				case "❌":
					var togos Togo.TogoList
					var err error
					allDays := i+1 < numOfTerms && (terms[i+1] == "a" || terms[i+1] == "-a")

					if togos, err = Togo.Load(update.Message.Chat.ID, !allDays); togos == nil {
						response.TextMsg = err.Error()
						bot.SendTextMessage(response)
					} else {
						response.TextMsg = "Here are your Today's togos:"
						if allDays {
							response.TextMsg = "Here are your ALL togos:"
						}
						if err != nil {
							response.TextMsg = fmt.Sprintln(response.TextMsg, "- - - - - - - - - - - - - - - - - - - - - - - -\n", err.Error())
						}
						response.InlineKeyboard = InlineKeyboardMenu(togos, RemoveTogo, allDays, terms[i+1] == "-a")
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
			var togos Togo.TogoList
			response.MessageBeingEditedId = update.CallbackQuery.Message.MessageID
			response.TargetChatId = update.CallbackQuery.Message.Chat.ID
			callbackData := LoadCallbackData(update.CallbackQuery.Data)

			var err error
			togos, err = Togo.Load(response.TargetChatId, !callbackData.AllDays)
			if togos != nil {
				if err != nil {
					response.TextMsg = err.Error()
					bot.SendTextMessage(response)
				}
				switch callbackData.Action {
				case TickTogo:
					togo, err := togos.Get(uint64(callbackData.ID))
					if err != nil {
						response.TextMsg = err.Error()
						bot.SendTextMessage(response)
					} else {
						if (*togo).Progress < 100 {
							(*togo).Progress = 100
						} else {
							(*togo).Progress = 0
						}
						(*togo).Update(response.TargetChatId)
						response.InlineKeyboard = InlineKeyboardMenu(togos, TickTogo, false, false)
						response.TextMsg = "✅ DONE! Now select the next togo you want to tick ..."
					}
				case RemoveTogo:
					togos, err := togos.Remove(response.TargetChatId, uint64(callbackData.ID))
					if err == nil {
						if len(togos) >= 1 {
							response.TextMsg = "❌ DONE! Now select the next togo you want to REMOVE ..."
							response.InlineKeyboard = InlineKeyboardMenu(togos, RemoveTogo, callbackData.AllDays, false)
						} else {
							response.TextMsg = "❌ DONE! All removed."
						}

					} else {
						response.TextMsg = err.Error()
						bot.SendTextMessage(response)
					}
				}
			} else {
				response.TextMsg = err.Error()
				bot.SendTextMessage(response)
			}

			bot.EditTextMessage(response)
		}
	}
}
