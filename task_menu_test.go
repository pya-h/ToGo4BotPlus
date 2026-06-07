package main

import (
	"strings"
	"testing"

	"ToGo4BotPlus/Task"
)

func TestTaskbookCommandRendersList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9800)
	seedTask(t, owner, "Ship the feature", 0)

	req := startFlowGetSend(t, bot, transport, owner, 1000, "/taskbook")
	text := req.Values.Get("text")
	if !strings.Contains(text, "Your tasks") || !strings.Contains(text, "Ship the feature") {
		t.Fatalf("expected task list message, got %q", text)
	}
	if req.Values.Get("reply_markup") == "" {
		t.Fatalf("expected an inline keyboard on the task list")
	}
}

func TestTaskMenuPaginatesAt30(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(9801)
	for i := 0; i < TasksPerMenuPage+1; i++ {
		seedTask(t, owner, "t", 0)
	}
	tasks, _ := loadTasksForBrowse(owner)

	_, kb := renderTaskList(tasks, 0)
	if !strings.Contains(kbText(kb), "1/2") {
		t.Fatalf("expected pagination once above %d tasks, labels were %q", TasksPerMenuPage, kbText(kb))
	}
	for _, row := range kb.InlineKeyboard {
		if len(row) > MaximumNumberOfRowItems {
			t.Fatalf("a row exceeded the max items per row: %d", len(row))
		}
	}
}

func TestTaskMenuOpenShowsDetail(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9802)
	id := seedTask(t, owner, "deep task", 0)

	detail := sendCallbackAndGetEditedText(t, bot, transport, owner, 1010,
		(CallbackData{Action: TaskMenuOpen, ID: int64(id)}).Json())
	if !strings.Contains(detail, "deep task") || !strings.Contains(detail, "1 of 1") {
		t.Fatalf("expected task detail with position, got %q", detail)
	}
}

func TestTaskMenuToggleFlipsDone(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9803)
	id := seedTask(t, owner, "toggle me", 0)

	sendCallbackAndGetEditedText(t, bot, transport, owner, 1020,
		(CallbackData{Action: TaskMenuToggle, ID: int64(id)}).Json())
	tasks, _ := Task.Load(owner, true, true)
	got, _ := tasks.Get(id)
	if got.Progress != 100 {
		t.Fatalf("expected progress 100 after toggle, got %d", got.Progress)
	}
}

func TestTaskMenuRemoveReturnsToList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9804)
	keep := seedTask(t, owner, "keep", 0)
	drop := seedTask(t, owner, "drop", 0)

	edited := sendCallbackAndGetEditedText(t, bot, transport, owner, 1030,
		(CallbackData{Action: TaskMenuRemove, ID: int64(drop)}).Json())
	if !strings.Contains(edited, "Removed") || !strings.Contains(edited, "keep") {
		t.Fatalf("expected removal then remaining list, got %q", edited)
	}
	remaining, _ := Task.Load(owner, true, true)
	if len(remaining) != 1 || remaining[0].Id != keep {
		t.Fatalf("expected only the kept task, got %+v", remaining)
	}
}

func TestTaskMenuEditHandsOffToEditCard(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9805)
	id := seedTask(t, owner, "editable task", 0)

	card := sendCallbackAndGetEditedText(t, bot, transport, owner, 1040,
		(CallbackData{Action: TaskMenuEdit, ID: int64(id)}).Json())
	if !strings.Contains(card, "editable task") || !strings.Contains(card, "Editing") {
		t.Fatalf("expected the edit card, got %q", card)
	}
	state, active := bot.flows.Get(owner)
	if !active || state.Entity != "task" || state.ItemID != id {
		t.Fatalf("expected an active task edit flow, got %+v (active=%v)", state, active)
	}
}
