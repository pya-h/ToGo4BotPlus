package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// maxImportBytes caps the size of an uploaded import file. The export is plain
// text JSON, so this is generous while still rejecting an oversized upload.
const maxImportBytes = 8 << 20 // 8 MiB

// importSummary reports how many records of each kind were created.
type importSummary struct {
	Togos    int
	Tasks    int
	Ideas    int
	Articles int
	Failed   int // records that were present but could not be saved
}

func (s importSummary) total() int { return s.Togos + s.Tasks + s.Ideas + s.Articles }

// importUserData parses a /json export (see userDataExport in json_export.go) and
// recreates every record under ownerID. It is the exact inverse of the export:
//
//   - Additive: every record is inserted as a brand-new row with a fresh id. It
//     never overwrites or deletes, so the same file can be re-imported, or
//     imported into a different account, without primary-key conflicts.
//   - Owner-forced: the stored OwnerId is always set to ownerID, so importing
//     someone else's export simply makes that data yours (a user can never inject
//     rows owned by another telegram id).
//   - Category by name: ideas/articles carry both CategoryId (export-local) and
//     Category (the name). We clear CategoryId and let Save re-resolve the name
//     into this owner's own category, so categories round-trip correctly across
//     accounts.
//   - Read state preserved: Article.Save never writes the read flag, so it is
//     restored explicitly after insert to keep the import faithful.
//
// Per-record failures are tolerated and counted; only a file that isn't valid
// export JSON returns an error.
func importUserData(ownerID int64, data []byte) (importSummary, error) {
	var payload userDataExport
	if err := json.Unmarshal(data, &payload); err != nil {
		return importSummary{}, fmt.Errorf("this doesn't look like a togo4bot JSON export: %w", err)
	}

	var sum importSummary

	for i := range payload.Togos {
		togo := payload.Togos[i]
		togo.OwnerId = ownerID
		if _, err := togo.Save(); err != nil {
			sum.Failed++
			continue
		}
		sum.Togos++
	}

	for i := range payload.Tasks {
		task := payload.Tasks[i]
		task.OwnerId = ownerID
		if _, err := task.Save(); err != nil {
			sum.Failed++
			continue
		}
		sum.Tasks++
	}

	for i := range payload.Ideas {
		idea := payload.Ideas[i]
		idea.OwnerId = ownerID
		idea.CategoryId = 0 // re-resolved from the category name during Save
		if _, err := idea.Save(); err != nil {
			sum.Failed++
			continue
		}
		sum.Ideas++
	}

	for i := range payload.Articles {
		article := payload.Articles[i]
		article.OwnerId = ownerID
		article.CategoryId = 0 // re-resolved from the category name during Save
		wasRead := article.Read
		newID, err := article.Save()
		if err != nil {
			sum.Failed++
			continue
		}
		// Save never persists the read flag, so restore it explicitly to keep the
		// import faithful to the export.
		if wasRead {
			article.Id = newID
			_ = article.SetRead(ownerID, true)
		}
		sum.Articles++
	}

	return sum, nil
}

// importWaitSet tracks owners who ran /import and are expected to upload a file
// next. In-memory and best-effort (like the reminder schedule): on restart the
// user simply re-runs /import.
type importWaitSet struct {
	mu      sync.Mutex
	waiting map[int64]bool
}

func newImportWaitSet() *importWaitSet { return &importWaitSet{waiting: make(map[int64]bool)} }

func (s *importWaitSet) arm(ownerID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.waiting[ownerID] = true
}

// take reports whether ownerID was waiting for an upload and clears the flag.
func (s *importWaitSet) take(ownerID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.waiting[ownerID] {
		delete(s.waiting, ownerID)
		return true
	}
	return false
}

// promptImport asks the user to upload their export file and arms the import
// waiter so the next document they send is ingested.
func (telegramBot *TelegramBotAPI) promptImport(chatID int64, replyTo int) {
	telegramBot.imports.arm(chatID)
	telegramBot.SendTextMessage(TelegramResponse{
		TargetChatId:     chatID,
		MessageRepliedTo: replyTo,
		TextMsg: "📥 Send me your exported `.json` file now and I'll import everything into your account.\n\n" +
			"• Get a file any time with /json.\n" +
			"• Import *adds* to your current data — it never deletes or overwrites.\n" +
			"• Tip: you can also just send the file with the caption /import.",
	})
}

// handleImportDocument downloads an uploaded document and imports its contents
// into chatID's data, replying with a summary.
func (telegramBot *TelegramBotAPI) handleImportDocument(chatID int64, doc *tgbotapi.Document, replyTo int) {
	reply := func(msg string) {
		telegramBot.SendTextMessage(TelegramResponse{
			TargetChatId:     chatID,
			MessageRepliedTo: replyTo,
			TextMsg:          msg,
		})
	}

	if doc.FileSize > maxImportBytes {
		reply("❌ That file is too large to be a togo4bot export.")
		return
	}

	data, err := telegramBot.downloadDocument(doc)
	if err != nil {
		reply(fmt.Sprintf("❌ Couldn't download the file: %v", err))
		return
	}

	summary, err := importUserData(chatID, data)
	if err != nil {
		reply(fmt.Sprintf("❌ %v", err))
		return
	}

	reply(formatImportSummary(summary))
}

// downloadDocument resolves a Telegram file URL and fetches its bytes (capped at
// maxImportBytes).
func (telegramBot *TelegramBotAPI) downloadDocument(doc *tgbotapi.Document) ([]byte, error) {
	url, err := telegramBot.GetFileDirectURL(doc.FileID)
	if err != nil {
		return nil, err
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(io.LimitReader(resp.Body, maxImportBytes))
}

func formatImportSummary(s importSummary) string {
	if s.total() == 0 && s.Failed == 0 {
		return "📥 Nothing to import — the file had no togos, tasks, ideas or articles."
	}
	msg := fmt.Sprintf(
		"✅ Import complete!\n\n• Togos: %d\n• Tasks: %d\n• Ideas: %d\n• Articles: %d",
		s.Togos, s.Tasks, s.Ideas, s.Articles,
	)
	if s.Failed > 0 {
		msg += fmt.Sprintf("\n\n⚠️ %d record(s) couldn't be imported and were skipped.", s.Failed)
	}
	return msg
}
