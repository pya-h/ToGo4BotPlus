package main

import (
	"strings"
	"testing"

	"ToGo4BotPlus/Togo"
)

func TestTogosCommandRendersList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9700)
	seedTogo(t, owner, "Write the report", 0)

	req := startFlowGetSend(t, bot, transport, owner, 900, "/togos")
	text := req.Values.Get("text")
	if !strings.Contains(text, "Your togos") || !strings.Contains(text, "Write the report") {
		t.Fatalf("expected togo list message, got %q", text)
	}
	if req.Values.Get("reply_markup") == "" {
		t.Fatalf("expected an inline keyboard on the togo list")
	}
}

func TestTogoButtonRowsArePacked(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(9701)
	for i := 0; i < 6; i++ {
		seedTogo(t, owner, "t", 0)
	}
	togos, _ := loadTogosForBrowse(owner)

	_, kb := renderTogoList(togos, 0)
	// 6 togos packed MaximumNumberOfRowItems per row, single page (no nav).
	wantRows := (6 + MaximumNumberOfRowItems - 1) / MaximumNumberOfRowItems
	if len(kb.InlineKeyboard) != wantRows {
		t.Fatalf("expected %d packed rows, got %d", wantRows, len(kb.InlineKeyboard))
	}
	for _, row := range kb.InlineKeyboard {
		if len(row) > MaximumNumberOfRowItems {
			t.Fatalf("a row exceeded the max items per row: %d", len(row))
		}
	}
}

func TestTogoMenuPaginatesAt30(t *testing.T) {
	withTempWorkingDir(t, true)
	owner := int64(9702)
	for i := 0; i < TogosPerMenuPage+1; i++ {
		seedTogo(t, owner, "t", 0)
	}
	togos, _ := loadTogosForBrowse(owner)

	_, kb := renderTogoList(togos, 0)
	if !strings.Contains(kbText(kb), "1/2") {
		t.Fatalf("expected pagination once above %d togos, labels were %q", TogosPerMenuPage, kbText(kb))
	}
}

func TestTogoMenuOpenShowsDetail(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9703)
	id := seedTogo(t, owner, "deep togo", 0)

	detail := sendCallbackAndGetEditedText(t, bot, transport, owner, 910,
		(CallbackData{Action: TogoMenuOpen, ID: int64(id)}).Json())
	if !strings.Contains(detail, "deep togo") || !strings.Contains(detail, "1 of 1") {
		t.Fatalf("expected togo detail with position, got %q", detail)
	}
}

func TestTogoMenuToggleFlipsDone(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9704)
	id := seedTogo(t, owner, "toggle me", 0)

	sendCallbackAndGetEditedText(t, bot, transport, owner, 920,
		(CallbackData{Action: TogoMenuToggle, ID: int64(id)}).Json())
	togos, _ := Togo.Load(owner, false, false)
	got, _ := togos.Get(id)
	if got.Progress != 100 {
		t.Fatalf("expected progress 100 after toggle, got %d", got.Progress)
	}

	// Toggling again flips it back.
	sendCallbackAndGetEditedText(t, bot, transport, owner, 921,
		(CallbackData{Action: TogoMenuToggle, ID: int64(id)}).Json())
	togos, _ = Togo.Load(owner, false, false)
	got, _ = togos.Get(id)
	if got.Progress != 0 {
		t.Fatalf("expected progress 0 after second toggle, got %d", got.Progress)
	}
}

func TestTogoMenuRemoveReturnsToList(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9705)
	keep := seedTogo(t, owner, "keep", 0)
	drop := seedTogo(t, owner, "drop", 0)

	edited := sendCallbackAndGetEditedText(t, bot, transport, owner, 930,
		(CallbackData{Action: TogoMenuRemove, ID: int64(drop)}).Json())
	if !strings.Contains(edited, "Removed") || !strings.Contains(edited, "keep") {
		t.Fatalf("expected removal then remaining list, got %q", edited)
	}
	remaining, _ := Togo.Load(owner, false, false)
	if len(remaining) != 1 || remaining[0].Id != keep {
		t.Fatalf("expected only the kept togo, got %+v", remaining)
	}
}

func TestTogoMenuEditHandsOffToEditCard(t *testing.T) {
	withTempWorkingDir(t, true)
	bot, transport := newRecordingBot(t)
	owner := int64(9706)
	id := seedTogo(t, owner, "editable togo", 0)

	card := sendCallbackAndGetEditedText(t, bot, transport, owner, 940,
		(CallbackData{Action: TogoMenuEdit, ID: int64(id)}).Json())
	if !strings.Contains(card, "editable togo") || !strings.Contains(card, "Editing") {
		t.Fatalf("expected the edit card, got %q", card)
	}
	state, active := bot.flows.Get(owner)
	if !active || state.Entity != "togo" || state.ItemID != id {
		t.Fatalf("expected an active togo edit flow, got %+v (active=%v)", state, active)
	}
	fields := sendCallbackAndGetEditedText(t, bot, transport, owner, 940,
		(CallbackData{Action: FlowEdit}).Json())
	if !strings.Contains(fields, "change") {
		t.Fatalf("expected the edit-field list, got %q", fields)
	}
}
