package main

import (
	"encoding/json"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func TestBuildUserDataExportStructure(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(5001)
	seedTogo(t, owner, "write tests", 0)
	seedTask(t, owner, "fix the bug", 0)
	seedIdea(t, owner, "new feature idea", false, "")
	seedArticle(t, owner, "Go blog", "https://go.dev/blog", "Tech")

	data, err := buildUserDataExport(owner)
	if err != nil {
		t.Fatalf("buildUserDataExport error: %v", err)
	}

	var export map[string]json.RawMessage
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("output is not valid JSON: %v\ndata: %s", err, data)
	}

	for _, key := range []string{"exported_at", "togos", "tasks", "ideas", "articles"} {
		if _, ok := export[key]; !ok {
			t.Errorf("export missing key %q", key)
		}
	}

	// Each list must have exactly 1 item.
	for key, wantLen := range map[string]int{"togos": 1, "tasks": 1, "ideas": 1, "articles": 1} {
		var items []json.RawMessage
		if err := json.Unmarshal(export[key], &items); err != nil {
			t.Errorf("key %q is not an array: %v", key, err)
			continue
		}
		if len(items) != wantLen {
			t.Errorf("key %q: expected %d item(s), got %d", key, wantLen, len(items))
		}
	}
}

func TestBuildUserDataExportEmptyOwner(t *testing.T) {
	withTempWorkingDir(t, true)
	data, err := buildUserDataExport(int64(5002))
	if err != nil {
		t.Fatalf("buildUserDataExport error: %v", err)
	}
	var export map[string]json.RawMessage
	if err := json.Unmarshal(data, &export); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	// Empty lists must be [] not null.
	for _, key := range []string{"togos", "tasks", "ideas", "articles"} {
		if string(export[key]) == "null" {
			t.Errorf("key %q should be [] for an owner with no data, got null", key)
		}
	}
}

func TestJsonCommandSendsDocument(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(5003)
	seedTogo(t, owner, "exported togo", 0)

	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 41,
		Text:      "/json",
		Chat:      &tgbotapi.Chat{ID: owner},
	}})

	if got := transport.countEndpoint("sendDocument"); got != 1 {
		t.Fatalf("expected exactly 1 sendDocument call, got %d", got)
	}
	// No stray text sendMessage should follow (the file IS the response).
	if got := transport.countEndpoint("sendMessage"); got != 0 {
		t.Fatalf("expected no sendMessage alongside the document, got %d", got)
	}
}

func TestJsonCommandIsolatedByOwner(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, _ := newRecordingBot(t)

	ownerA := int64(5004)
	ownerB := int64(5005)
	seedTogo(t, ownerA, "owner-A togo", 0)
	seedTogo(t, ownerB, "owner-B togo", 0)

	// ownerA requests the export.
	bot.HandleUpdate(tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 42,
		Text:      "/json",
		Chat:      &tgbotapi.Chat{ID: ownerA},
	}})

	// Verify ownerA's export contains only ownerA's data.
	data, err := buildUserDataExport(ownerA)
	if err != nil {
		t.Fatalf("buildUserDataExport error: %v", err)
	}
	if strings.Contains(string(data), "owner-B togo") {
		t.Fatal("ownerA's export must not contain ownerB's data")
	}
	if !strings.Contains(string(data), "owner-A togo") {
		t.Fatal("ownerA's export is missing ownerA's togo")
	}
}

func TestJsonCommandHelpMentionsJson(t *testing.T) {
	if !strings.Contains(HELP_MESSAGE, "/json") {
		t.Fatal("HELP_MESSAGE must mention /json")
	}
}
