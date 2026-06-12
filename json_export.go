package main

import (
	"encoding/json"
	"fmt"

	"ToGo4BotPlus/Article"
	"ToGo4BotPlus/Idea"
	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type userDataExport struct {
	ExportedAt string              `json:"exported_at"`
	Togos      Togo.TogoList       `json:"togos"`
	Tasks      Task.TaskList       `json:"tasks"`
	Ideas      Idea.IdeaList       `json:"ideas"`
	Articles   Article.ArticleList `json:"articles"`
}

// buildUserDataExport loads every entity for the given owner and serialises it
// as indented JSON. Partial DB failures are silently treated as empty lists so
// the export always returns valid JSON.
func buildUserDataExport(ownerID int64) ([]byte, error) {
	now := Togo.Today()
	export := userDataExport{
		ExportedAt: now.Get(),
	}

	if togos, _ := Togo.Load(ownerID, false, false); togos != nil {
		export.Togos = togos
	} else {
		export.Togos = Togo.TogoList{}
	}
	if tasks, _ := Task.Load(ownerID, true, true); tasks != nil {
		export.Tasks = tasks
	} else {
		export.Tasks = Task.TaskList{}
	}
	if ideas, _ := Idea.Load(ownerID, false, false, 0); ideas != nil {
		export.Ideas = ideas
	} else {
		export.Ideas = Idea.IdeaList{}
	}
	if articles, _ := Article.Load(ownerID, 0); articles != nil {
		export.Articles = articles
	} else {
		export.Articles = Article.ArticleList{}
	}

	return json.MarshalIndent(export, "", "  ")
}

// sendUserDataJSON builds the full JSON export for the user and delivers it as
// a downloadable .json document. Errors during send fall back to a text reply.
func (telegramBot *TelegramBotAPI) sendUserDataJSON(chatID int64, replyTo int) {
	data, err := buildUserDataExport(chatID)
	if err != nil {
		telegramBot.SendTextMessage(TelegramResponse{
			TargetChatId:     chatID,
			MessageRepliedTo: replyTo,
			TextMsg:          fmt.Sprintf("❌ Failed to build export: %v", err),
		})
		return
	}

	filename := fmt.Sprintf("togo4bot_%d.json", chatID)
	msg := tgbotapi.NewDocumentUpload(chatID, tgbotapi.FileBytes{
		Name:  filename,
		Bytes: data,
	})
	msg.ReplyToMessageID = replyTo
	msg.Caption = "📦 Your full data export — togos, tasks, ideas and articles."

	if _, err := telegramBot.Send(msg); err != nil {
		telegramBot.SendTextMessage(TelegramResponse{
			TargetChatId:     chatID,
			MessageRepliedTo: replyTo,
			TextMsg:          fmt.Sprintf("❌ Export ready but failed to send: %v", err),
		})
	}
}
