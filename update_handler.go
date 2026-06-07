package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"ToGo4BotPlus/Article"
	"ToGo4BotPlus/Idea"
	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func (telegramBot *TelegramBotAPI) HandleUpdate(update tgbotapi.Update) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			log.Printf("Panic recovered while processing update: %v", panicErr)
			telegramBot.InformAdmin(fmt.Sprintf("Panic during update processing: %v", panicErr))
		}
	}()

	telegramBot.ensureFlows()
	response := TelegramResponse{TextMsg: HELP_MESSAGE}

	if update.Message != nil {
		chatID := update.Message.Chat.ID

		// 1) A guided-flow slash command (/addIdea, /ideas, /cancel, ...) takes priority.
		if cmd, arg, ok := parseFlowCommand(update.Message.Text); ok {
			telegramBot.handleFlowCommand(chatID, cmd, arg)
			return
		}
		// 2) A reply to an in-progress guided flow.
		if state, active := telegramBot.flows.Get(chatID); active {
			telegramBot.handleFlowText(chatID, update.Message.Text, state)
			return
		}
		// 3) Otherwise the existing stateless command handler.
		telegramBot.handleMessageUpdate(update.Message, &response)
		telegramBot.SendTextMessage(response)
		return
	}

	if update.CallbackQuery != nil {
		// Telegram delivers callbacks with a nil Message when the originating
		// message is too old (~48h) or is an inline_message_id result. Both
		// callback handlers edit that message, so bail out early to avoid a
		// nil dereference (otherwise only caught by the panic recovery above).
		if update.CallbackQuery.Message == nil {
			return
		}
		if cb := LoadCallbackData(update.CallbackQuery.Data); isFlowAction(cb.Action) {
			telegramBot.handleFlowCallback(update.CallbackQuery, cb)
			return
		}
		telegramBot.handleCallbackUpdate(update.CallbackQuery, &response)
		telegramBot.EditTextMessage(response)
	}
}

func (telegramBot *TelegramBotAPI) handleMessageUpdate(message *tgbotapi.Message, response *TelegramResponse) {
	response.ReplyMarkup = MainKeyboardMenu()
	response.TargetChatId = message.Chat.ID
	response.MessageRepliedTo = message.MessageID
	terms := SplitArguments(message.Text)
	numOfTerms := len(terms)

	// Normalize a leading slash command of the form "/cmd@BotName" to "/cmd"
	// (Telegram appends the bot username to commands sent in groups).
	if numOfTerms > 0 && strings.HasPrefix(terms[0], "/") {
		if at := strings.IndexByte(terms[0], '@'); at >= 0 {
			terms[0] = terms[0][:at]
		}
	}

	now := Togo.Today()
	for i := 0; i < numOfTerms; i++ {
		switch terms[i] {
		case "+":
			if i+1 < numOfTerms {
				if togo, err := Togo.Extract(message.Chat.ID, terms[i+1:]); err == nil {
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
				if task, err := Task.Extract(message.Chat.ID, terms[i+1:]); err == nil {
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

			togos, warning := Togo.Load(message.Chat.ID, !allDays, allDays && terms[i+1] == "-a")
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
				telegramBot.SendTextMessage(*response)
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
				tasks, warning := Task.Load(message.Chat.ID, includeInactive, false)
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

				togos, togoWarning := Togo.Load(message.Chat.ID, !allDays, allDays && allDaysToken == "-a")
				tasks, taskWarning := Task.Load(message.Chat.ID, includeInactive, false)

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
				allDays := i+1 < numOfTerms && (terms[i+1] == "+a" || terms[i+1] == "-a")
				togos, warning := Togo.Load(message.Chat.ID, !allDays, allDays && terms[i+1] == "-a")
				if togos == nil {
					if warning != nil {
						log.Println(warning.Error())
						response.TextMsg = warning.Error()
					} else {
						response.TextMsg = "Could not calculate togo progress."
					}
					telegramBot.SendTextMessage(*response)
				} else {
					response.TextMsg = BuildTogoProgressReport(togos, allDays, warning)
				}
			}
		case "$":
			var togos Togo.TogoList
			var err error
			if i+1 < numOfTerms {
				togos, err = Togo.Load(message.Chat.ID, false, false)
				if togos != nil {
					if resp, err := togos.Update(message.Chat.ID, terms[i+1:]); err == nil {
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
				tasks, err := Task.Load(message.Chat.ID, true, true)
				if tasks != nil {
					if resp, err := tasks.Update(message.Chat.ID, terms[i+1:]); err == nil {
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
			tasks, warning := Task.Load(message.Chat.ID, includeInactive, false)
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
			togos, err := Togo.Load(message.Chat.ID, !allDays, allDays && terms[i+1] == "-a")
			if togos != nil {
				if len(togos) >= 1 {
					response.TextMsg = "Here are your togos for today:"
					response.InlineKeyboard = InlineKeyboardMenu(togos, TickTogo, allDays, allDays && terms[i+1] == "-a", 0)
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
			tasks, warning := Task.Load(message.Chat.ID, includeInactive, false)
			if tasks != nil {
				if len(tasks) >= 1 {
					response.TextMsg = "Here are your tasks to tick:"
					response.InlineKeyboard = TaskInlineKeyboardMenu(tasks, TickTask, includeInactive, 0)
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

			if togos, err = Togo.Load(message.Chat.ID, !allDays, allDays && terms[i+1] == "-a"); togos == nil {
				response.TextMsg = err.Error()
				telegramBot.SendTextMessage(*response)
			} else {
				if len(togos) >= 1 {
					response.TextMsg = "Here are your Today's togos:"
					if allDays {
						response.TextMsg = "Here are your ALL togos:"
					}
					if err != nil {
						response.TextMsg = fmt.Sprintln(response.TextMsg, "- - - - - - - - - - - - - - - - - - - - - - - -\n", err.Error())
					}
					response.InlineKeyboard = InlineKeyboardMenu(togos, RemoveTogo, allDays, allDays && terms[i+1] == "-a", 0)
				} else {
					response.TextMsg = "No togos so far..."
				}
			}
		case TaskRemoveCommand, "❌t":
			includeInactive := i+1 < numOfTerms && terms[i+1] == TaskIncludeInactiveToken
			tasks, warning := Task.Load(message.Chat.ID, includeInactive, false)
			if tasks != nil {
				if len(tasks) >= 1 {
					response.TextMsg = "Here are your tasks to remove:"
					response.InlineKeyboard = TaskInlineKeyboardMenu(tasks, RemoveTask, includeInactive, 0)
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
					if err := Task.SetReminderTimes(message.Chat.ID, times); err == nil {
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
			} else if setting, err := Task.GetReminderSetting(message.Chat.ID); err == nil {
				response.TextMsg = fmt.Sprintf("Current task reminder frequency: %d times/day\nAllowed values: %s", setting.RemindersPerDay, allowedTaskReminderValuesText())
			} else {
				response.TextMsg = err.Error()
			}
		case "/db":
			if adminID, err := strconv.Atoi(env["ADMIN_ID"]); err == nil && int64(adminID) == response.TargetChatId {
				msg := tgbotapi.NewDocumentUpload(int64(adminID), "./togos.db")
				if _, err := telegramBot.Send(msg); err != nil {
					response.TextMsg = err.Error()
				} else {
					response.TextMsg = "Successfully sent db!"
				}
			} else {
				response.TextMsg = "get the fuck off my porch!"
			}
		case "/now":
			response.TextMsg = now.Get()
		case "/ideabook", "/favorites":
			scope := ideaScopeAll
			if terms[i] == "/favorites" {
				scope = ideaScopeFav
			}
			ideas, warning := loadIdeasForScope(message.Chat.ID, scope, 0)
			text, kb := renderIdeaList(ideas, scope, 0, 0)
			response.TextMsg = appendWarning(text, warning)
			response.InlineKeyboard = kb
		case "/articlebook":
			articles, warning := loadArticlesForScope(message.Chat.ID, 0)
			text, kb := renderArticleList(articles, 0, 0)
			response.TextMsg = appendWarning(text, warning)
			response.InlineKeyboard = kb
		case ArticleAddCommand:
			if i+1 < numOfTerms {
				if article, err := Article.Extract(message.Chat.ID, terms[i+1:]); err == nil {
					if article.Id, err = article.Save(); err == nil {
						response.TextMsg = fmt.Sprintf("🔗 Article #%d saved.", article.Id)
					} else {
						response.TextMsg = err.Error()
					}
				} else {
					response.TextMsg = err.Error()
				}
			} else {
				response.TextMsg = "You must provide the article title."
			}
		case ArticleListCommand:
			category := ""
			if i+1 < numOfTerms && terms[i+1] == ArticleCategoryToken && i+2 < numOfTerms {
				category = terms[i+2]
			}
			categoryID := int64(0)
			if category != "" {
				resolved, lookupErr := Article.LookupCategoryID(message.Chat.ID, category)
				if lookupErr != nil {
					response.TextMsg = lookupErr.Error()
					break
				}
				if resolved == 0 {
					response.TextMsg = fmt.Sprintf("🔗 No articles found in category %q.", category)
					break
				}
				categoryID = resolved
			}
			articles, warning := Article.Load(message.Chat.ID, categoryID)
			response.TextMsg = BuildArticleListReport(articles, category)
			if warning != nil {
				response.TextMsg = fmt.Sprintf("%s\n\nwarning: %s", response.TextMsg, warning.Error())
			}
		case ArticleUpdateCommand:
			if i+1 < numOfTerms {
				articles, err := Article.Load(message.Chat.ID, 0)
				if resp, updateErr := articles.Update(message.Chat.ID, terms[i+1:]); updateErr == nil {
					response.TextMsg = resp
				} else if err != nil {
					response.TextMsg = err.Error()
				} else {
					response.TextMsg = updateErr.Error()
				}
			} else {
				response.TextMsg = "You must provide the article identifier!"
			}
		case ArticleRemoveCommand:
			articles, warning := Article.Load(message.Chat.ID, 0)
			if len(articles) >= 1 {
				response.TextMsg = "Here are your articles to remove:"
				response.InlineKeyboard = ArticleInlineKeyboardMenu(articles, RemoveArticle, 0)
			} else if warning != nil {
				response.TextMsg = warning.Error()
			} else {
				response.TextMsg = "No articles so far..."
			}
		case TogoTickByIdCommand:
			if i+1 < numOfTerms {
				var id uint64
				if _, err := fmt.Sscan(terms[i+1], &id); err != nil {
					response.TextMsg = "Invalid togo id. Usage: tk  <togo_id>"
					break
				}
				togos, err := Togo.Load(message.Chat.ID, false, false)
				if togos == nil {
					if err != nil {
						response.TextMsg = err.Error()
					} else {
						response.TextMsg = "Could not load togos."
					}
					break
				}
				togo, getErr := togos.Get(id)
				if getErr != nil {
					response.TextMsg = getErr.Error()
					break
				}
				if togo.Progress < 100 {
					togo.Progress = 100
				} else {
					togo.Progress = 0
				}
				if updateErr := togo.Update(message.Chat.ID); updateErr != nil {
					response.TextMsg = updateErr.Error()
					break
				}
				if togo.Progress >= 100 {
					response.TextMsg = fmt.Sprintf("✅ Togo #%d %q ticked.", togo.Id, togo.Title)
				} else {
					response.TextMsg = fmt.Sprintf("Togo #%d %q unticked.", togo.Id, togo.Title)
				}
			} else {
				response.TextMsg = "Usage: tk  <togo_id>"
			}
		case TaskTickByIdCommand:
			if i+1 < numOfTerms {
				var id uint64
				if _, err := fmt.Sscan(terms[i+1], &id); err != nil {
					response.TextMsg = "Invalid task id. Usage: TK  <task_id>"
					break
				}
				tasks, warning := Task.Load(message.Chat.ID, true, true)
				if tasks == nil {
					if warning != nil {
						response.TextMsg = warning.Error()
					} else {
						response.TextMsg = "Could not load tasks."
					}
					break
				}
				task, getErr := tasks.Get(id)
				if getErr != nil {
					response.TextMsg = getErr.Error()
					break
				}
				if task.Progress < 100 {
					task.Progress = 100
				} else {
					task.Progress = 0
				}
				if updateErr := task.Update(message.Chat.ID); updateErr != nil {
					response.TextMsg = updateErr.Error()
					break
				}
				if task.Progress >= 100 {
					response.TextMsg = fmt.Sprintf("✅ Task #%d %q ticked.", task.Id, task.Title)
				} else {
					response.TextMsg = fmt.Sprintf("Task #%d %q unticked.", task.Id, task.Title)
				}
			} else {
				response.TextMsg = "Usage: TK  <task_id>"
			}
		case IdeaAddCommand:
			if i+1 < numOfTerms {
				if idea, err := Idea.Extract(message.Chat.ID, terms[i+1:]); err == nil {
					if idea.Id, err = idea.Save(); err == nil {
						response.TextMsg = fmt.Sprintf("💡 Idea #%d created.", idea.Id)
					} else {
						response.TextMsg = err.Error()
					}
				} else {
					response.TextMsg = err.Error()
				}
			} else {
				response.TextMsg = "You must provide the idea text."
			}
		case IdeaListCommand:
			onlyHigh := false
			category := ""
			if i+1 < numOfTerms {
				switch terms[i+1] {
				case IdeaHighPriorityToken:
					onlyHigh = true
				case IdeaCategoryToken:
					if i+2 < numOfTerms {
						category = terms[i+2]
					}
				}
			}
			categoryID := int64(0)
			if category != "" {
				resolved, lookupErr := Idea.LookupCategoryID(message.Chat.ID, category)
				if lookupErr != nil {
					response.TextMsg = lookupErr.Error()
					break
				}
				if resolved == 0 {
					response.TextMsg = fmt.Sprintf("💡 No ideas found in category %q.", category)
					break
				}
				categoryID = resolved
			}
			ideas, warning := Idea.Load(message.Chat.ID, onlyHigh, false, categoryID)
			if ideas == nil {
				if warning != nil {
					response.TextMsg = warning.Error()
				} else {
					response.TextMsg = "Could not load ideas."
				}
				break
			}
			response.TextMsg = BuildIdeaListReport(ideas, onlyHigh, category)
			if warning != nil {
				response.TextMsg = fmt.Sprintf("%s\n\nwarning: %s", response.TextMsg, warning.Error())
			}
		case IdeaUpdateCommand:
			if i+1 < numOfTerms {
				ideas, err := Idea.Load(message.Chat.ID, false, false, 0)
				if ideas != nil {
					if resp, updateErr := ideas.Update(message.Chat.ID, terms[i+1:]); updateErr == nil {
						response.TextMsg = resp
					} else {
						response.TextMsg = updateErr.Error()
					}
				} else if err != nil {
					response.TextMsg = err.Error()
				} else {
					response.TextMsg = "Could not load ideas for update."
				}
			} else {
				response.TextMsg = "You must provide the idea identifier!"
			}
		case IdeaRemoveCommand:
			ideas, warning := Idea.Load(message.Chat.ID, false, false, 0)
			if ideas != nil {
				if len(ideas) >= 1 {
					response.TextMsg = "Here are your ideas to remove:"
					response.InlineKeyboard = IdeaInlineKeyboardMenu(ideas, RemoveIdea, 0)
				} else {
					response.TextMsg = "No ideas so far..."
				}
				if warning != nil {
					response.TextMsg = fmt.Sprintf("%s\nwarning: %s", response.TextMsg, warning.Error())
				}
			} else if warning != nil {
				response.TextMsg = warning.Error()
			} else {
				response.TextMsg = "Could not load ideas for removing."
			}
		}
	}
}

func (telegramBot *TelegramBotAPI) handleCallbackUpdate(callbackQuery *tgbotapi.CallbackQuery, response *TelegramResponse) {
	response.MessageBeingEditedId = callbackQuery.Message.MessageID
	response.TargetChatId = callbackQuery.Message.Chat.ID
	callbackData := LoadCallbackData(callbackQuery.Data)

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
			telegramBot.SendTextMessage(*response)
		}

		if callbackData.Action == TickTogo {
			togo, err := togos.Get(uint64(callbackData.ID))
			if err != nil {
				response.TextMsg = err.Error()
				break
			}
			if togo.Progress < 100 {
				togo.Progress = 100
			} else {
				togo.Progress = 0
			}
			_ = togo.Update(response.TargetChatId)
			response.InlineKeyboard = InlineKeyboardMenu(togos, TickTogo, callbackData.AllDays, callbackData.JustUndones, callbackData.MenuPage)
			response.TextMsg = "✅ DONE! Now select the next togo you want to tick ..."
		} else {
			updated, err := togos.Remove(response.TargetChatId, uint64(callbackData.ID))
			if err != nil {
				response.TextMsg = err.Error()
				break
			}
			if len(updated) >= 1 {
				response.TextMsg = "❌ DONE! Now select the next togo you want to REMOVE ..."
				response.InlineKeyboard = InlineKeyboardMenu(updated, RemoveTogo, callbackData.AllDays, callbackData.JustUndones, callbackData.MenuPage)
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
			response.InlineKeyboard = TaskInlineKeyboardMenu(updated, TickTask, callbackData.TaskIncludeInactive, callbackData.MenuPage)
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
			response.InlineKeyboard = TaskInlineKeyboardMenu(updated, RemoveTask, callbackData.TaskIncludeInactive, callbackData.MenuPage)
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

	case ShowTogoMenuPage:
		togos, err := Togo.Load(response.TargetChatId, !callbackData.AllDays, callbackData.JustUndones)
		if togos == nil {
			if err != nil {
				response.TextMsg = err.Error()
			} else {
				response.TextMsg = "Could not load togos for this page."
			}
			break
		}
		if len(togos) == 0 {
			response.TextMsg = "No togos to show."
			break
		}
		response.InlineKeyboard = InlineKeyboardMenu(togos, callbackData.MenuAction, callbackData.AllDays, callbackData.JustUndones, callbackData.MenuPage)
		if callbackData.MenuAction == RemoveTogo {
			response.TextMsg = "Select a togo to remove ..."
		} else {
			response.TextMsg = "Select a togo to tick ..."
		}

	case ShowTaskMenuPage:
		tasks, warning := Task.Load(response.TargetChatId, callbackData.TaskIncludeInactive, false)
		if tasks == nil {
			if warning != nil {
				response.TextMsg = warning.Error()
			} else {
				response.TextMsg = "Could not load tasks for this page."
			}
			break
		}
		if len(tasks) == 0 {
			response.TextMsg = "No tasks to show."
			break
		}
		response.InlineKeyboard = TaskInlineKeyboardMenu(tasks, callbackData.MenuAction, callbackData.TaskIncludeInactive, callbackData.MenuPage)
		if callbackData.MenuAction == RemoveTask {
			response.TextMsg = "Select a task to remove ..."
		} else {
			response.TextMsg = "Select a task to tick ..."
		}

	case RemoveIdea:
		ideas, warning := Idea.Load(response.TargetChatId, false, false, 0)
		if ideas == nil {
			if warning != nil {
				response.TextMsg = warning.Error()
			} else {
				response.TextMsg = "Could not load ideas for removal."
			}
			break
		}
		updated, err := ideas.Remove(response.TargetChatId, uint64(callbackData.ID))
		if err != nil {
			response.TextMsg = err.Error()
			break
		}
		if len(updated) >= 1 {
			response.TextMsg = "❌ Idea removed. Pick the next idea to remove ..."
			response.InlineKeyboard = IdeaInlineKeyboardMenu(updated, RemoveIdea, callbackData.MenuPage)
		} else {
			response.TextMsg = "❌ Idea removed. No ideas left in this view."
		}

	case ShowIdeaMenuPage:
		ideas, warning := Idea.Load(response.TargetChatId, false, false, 0)
		if ideas == nil {
			if warning != nil {
				response.TextMsg = warning.Error()
			} else {
				response.TextMsg = "Could not load ideas for this page."
			}
			break
		}
		if len(ideas) == 0 {
			response.TextMsg = "No ideas to show."
			break
		}
		response.InlineKeyboard = IdeaInlineKeyboardMenu(ideas, callbackData.MenuAction, callbackData.MenuPage)
		response.TextMsg = "Select an idea to remove ..."

	case IdeaMenuList, IdeaMenuOpen, IdeaMenuFav, IdeaMenuRemove, IdeaMenuEdit:
		telegramBot.handleIdeaMenuCallback(callbackData, response)

	case RemoveArticle:
		articles, warning := Article.Load(response.TargetChatId, 0)
		if articles == nil {
			if warning != nil {
				response.TextMsg = warning.Error()
			} else {
				response.TextMsg = "Could not load articles for removal."
			}
			break
		}
		updated, err := articles.Remove(response.TargetChatId, uint64(callbackData.ID))
		if err != nil {
			response.TextMsg = err.Error()
			break
		}
		if len(updated) >= 1 {
			response.TextMsg = "🗑 Article removed. Pick the next article to remove ..."
			response.InlineKeyboard = ArticleInlineKeyboardMenu(updated, RemoveArticle, callbackData.MenuPage)
		} else {
			response.TextMsg = "🗑 Article removed. No articles left in this view."
		}

	case ShowArticleMenuPage:
		articles, warning := Article.Load(response.TargetChatId, 0)
		if len(articles) == 0 {
			if warning != nil {
				response.TextMsg = warning.Error()
			} else {
				response.TextMsg = "No articles to show."
			}
			break
		}
		response.InlineKeyboard = ArticleInlineKeyboardMenu(articles, callbackData.MenuAction, callbackData.MenuPage)
		response.TextMsg = "Select an article to remove ..."

	case ArticleMenuList, ArticleMenuOpen, ArticleMenuRemove, ArticleMenuEdit:
		telegramBot.handleArticleMenuCallback(callbackData, response)

	default:
		response.TextMsg = "Unsupported callback action."
	}
}
