package main

import (
	"fmt"
	"strconv"

	"ToGo4BotPlus/Idea"
	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// ============================================================
// Manage flows (Type B): /ideas, /togos, /tasks.
//
// Unlike the linear add-wizards, managing an item is a small screen-based state
// machine: list -> card -> (edit field -> value) | (delete -> confirm) | toggle.
// FlowState.Screen tracks the current screen; FlowState.Entity selects the
// adapter. All DB work funnels through the existing domain Update/Remove code.
// ============================================================

const (
	manageScreenList     = "list"
	manageScreenCard     = "card"
	manageScreenEditList = "editfield"
	manageScreenEditVal  = "editvalue"
	manageScreenConfirm  = "confirmdelete"
)

// manageFlowEntity maps a manage flow name to its entity key.
var manageFlowEntity = map[string]string{
	"manageIdea": "idea",
	"manageTogo": "togo",
	"manageTask": "task",
}

type manageItem struct {
	ID    uint64
	Label string
}

type editField struct {
	Key     string
	Label   string
	Kind    StepKind // StepText or StepChoice
	Options []FlowOption
}

// ManageEntity abstracts the per-entity operations the manage flow needs.
type ManageEntity interface {
	Label() string
	ListItems(chatID int64) ([]manageItem, error)
	Card(chatID int64, id uint64) (string, bool)
	CanToggle() bool
	Toggle(chatID int64, id uint64) error
	Delete(chatID int64, id uint64) error
	EditFields() []editField
	ApplyEdit(chatID int64, id uint64, field, value string) error
}

func manageEntityFor(entity string) ManageEntity {
	switch entity {
	case "idea":
		return ideaManager{}
	case "togo":
		return togoManager{}
	case "task":
		return taskManager{}
	default:
		return nil
	}
}

// ---------------------- Lifecycle & routing -----------------------------------

func (telegramBot *TelegramBotAPI) startManageFlow(chatID int64, entity string) {
	ent := manageEntityFor(entity)
	if ent == nil {
		telegramBot.SendTextMessage(TelegramResponse{TargetChatId: chatID, TextMsg: "Unknown guided command.", ReplyMarkup: MainKeyboardMenu()})
		return
	}
	state := &FlowState{Entity: entity, Screen: manageScreenList, Data: make(map[string]string)}
	text, kb := telegramBot.buildManageList(chatID, state, ent)
	if id, err := telegramBot.SendTextMessageReturningID(TelegramResponse{TargetChatId: chatID, TextMsg: text, InlineKeyboard: kb}); err == nil {
		state.MessageID = id
	}
	telegramBot.flows.Set(chatID, state)
}

func (telegramBot *TelegramBotAPI) renderManage(chatID int64, state *FlowState, text string, kb *tgbotapi.InlineKeyboardMarkup) {
	telegramBot.EditTextMessage(TelegramResponse{
		TargetChatId:         chatID,
		MessageBeingEditedId: state.MessageID,
		TextMsg:              text,
		InlineKeyboard:       kb,
	})
}

func (telegramBot *TelegramBotAPI) handleManageText(chatID int64, text string, state *FlowState) {
	ent := manageEntityFor(state.Entity)
	if ent == nil {
		telegramBot.flows.Clear(chatID)
		return
	}
	if state.Screen == manageScreenEditVal && state.AwaitText {
		if err := ent.ApplyEdit(chatID, state.ItemID, state.EditField, text); err != nil {
			t, kb := telegramBot.buildManageCard(chatID, state, ent, "⚠️ "+err.Error())
			telegramBot.renderManage(chatID, state, t, kb)
			return
		}
		t, kb := telegramBot.buildManageCard(chatID, state, ent, "✅ Updated.")
		telegramBot.renderManage(chatID, state, t, kb)
		return
	}
	// Any other typed input just re-renders the current screen.
	telegramBot.reRenderManage(chatID, state, ent)
}

func (telegramBot *TelegramBotAPI) handleManageCallback(chatID int64, cb CallbackData, state *FlowState) {
	ent := manageEntityFor(state.Entity)
	if ent == nil {
		telegramBot.flows.Clear(chatID)
		return
	}

	switch state.Screen {
	case manageScreenList:
		switch cb.Action {
		case FlowCancel:
			telegramBot.cancelActiveFlow(chatID)
		case FlowSelect:
			if id, ok := selectedID(state, cb.FlowOpt); ok {
				state.ItemID = id
				t, kb := telegramBot.buildManageCard(chatID, state, ent, "")
				telegramBot.renderManage(chatID, state, t, kb)
			} else {
				telegramBot.reRenderManage(chatID, state, ent)
			}
		default:
			telegramBot.reRenderManage(chatID, state, ent)
		}

	case manageScreenCard:
		switch cb.Action {
		case FlowCancel:
			telegramBot.cancelActiveFlow(chatID)
		case FlowBack:
			t, kb := telegramBot.buildManageList(chatID, state, ent)
			telegramBot.renderManage(chatID, state, t, kb)
		case FlowEdit:
			t, kb := telegramBot.buildEditFieldList(state, ent)
			telegramBot.renderManage(chatID, state, t, kb)
		case FlowDelete:
			t, kb := telegramBot.buildConfirmDelete(chatID, state, ent)
			telegramBot.renderManage(chatID, state, t, kb)
		case FlowToggle:
			if ent.CanToggle() {
				note := "✅ Toggled."
				if err := ent.Toggle(chatID, state.ItemID); err != nil {
					note = "⚠️ " + err.Error()
				}
				t, kb := telegramBot.buildManageCard(chatID, state, ent, note)
				telegramBot.renderManage(chatID, state, t, kb)
			}
		default:
			t, kb := telegramBot.buildManageCard(chatID, state, ent, "")
			telegramBot.renderManage(chatID, state, t, kb)
		}

	case manageScreenEditList:
		switch cb.Action {
		case FlowCancel:
			telegramBot.cancelActiveFlow(chatID)
		case FlowBack:
			t, kb := telegramBot.buildManageCard(chatID, state, ent, "")
			telegramBot.renderManage(chatID, state, t, kb)
		case FlowSelect:
			if cb.FlowOpt >= 0 && cb.FlowOpt < len(state.Options) {
				state.EditField = state.Options[cb.FlowOpt].Value
				t, kb := telegramBot.buildEditValue(state, ent)
				telegramBot.renderManage(chatID, state, t, kb)
			} else {
				telegramBot.reRenderManage(chatID, state, ent)
			}
		default:
			telegramBot.reRenderManage(chatID, state, ent)
		}

	case manageScreenEditVal:
		switch cb.Action {
		case FlowCancel:
			telegramBot.cancelActiveFlow(chatID)
		case FlowBack:
			t, kb := telegramBot.buildEditFieldList(state, ent)
			telegramBot.renderManage(chatID, state, t, kb)
		case FlowSelect:
			if cb.FlowOpt >= 0 && cb.FlowOpt < len(state.Options) {
				value := state.Options[cb.FlowOpt].Value
				note := "✅ Updated."
				if err := ent.ApplyEdit(chatID, state.ItemID, state.EditField, value); err != nil {
					note = "⚠️ " + err.Error()
				}
				t, kb := telegramBot.buildManageCard(chatID, state, ent, note)
				telegramBot.renderManage(chatID, state, t, kb)
			} else {
				telegramBot.reRenderManage(chatID, state, ent)
			}
		default:
			telegramBot.reRenderManage(chatID, state, ent)
		}

	case manageScreenConfirm:
		switch cb.Action {
		case FlowConfirm:
			note := "🗑 Deleted."
			if err := ent.Delete(chatID, state.ItemID); err != nil {
				note = "⚠️ " + err.Error()
			}
			state.ItemID = 0
			t, kb := telegramBot.buildManageList(chatID, state, ent)
			telegramBot.renderManage(chatID, state, note+"\n\n"+t, kb)
		case FlowBack, FlowCancel:
			t, kb := telegramBot.buildManageCard(chatID, state, ent, "")
			telegramBot.renderManage(chatID, state, t, kb)
		default:
			telegramBot.reRenderManage(chatID, state, ent)
		}
	}
}

// reRenderManage re-draws whatever screen the flow is currently on.
func (telegramBot *TelegramBotAPI) reRenderManage(chatID int64, state *FlowState, ent ManageEntity) {
	var text string
	var kb *tgbotapi.InlineKeyboardMarkup
	switch state.Screen {
	case manageScreenCard:
		text, kb = telegramBot.buildManageCard(chatID, state, ent, "")
	case manageScreenEditList:
		text, kb = telegramBot.buildEditFieldList(state, ent)
	case manageScreenEditVal:
		text, kb = telegramBot.buildEditValue(state, ent)
	case manageScreenConfirm:
		text, kb = telegramBot.buildConfirmDelete(chatID, state, ent)
	default:
		text, kb = telegramBot.buildManageList(chatID, state, ent)
	}
	telegramBot.renderManage(chatID, state, text, kb)
}

func selectedID(state *FlowState, idx int) (uint64, bool) {
	if idx < 0 || idx >= len(state.Options) {
		return 0, false
	}
	id, err := strconv.ParseUint(state.Options[idx].Value, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// ---------------------- Screen builders ---------------------------------------

func (telegramBot *TelegramBotAPI) buildManageList(chatID int64, state *FlowState, ent ManageEntity) (string, *tgbotapi.InlineKeyboardMarkup) {
	state.Screen = manageScreenList
	state.AwaitText = false
	state.Options = nil

	items, err := ent.ListItems(chatID)
	if err != nil {
		return fmt.Sprintf("⚠️ Could not load %ss: %s", ent.Label(), err.Error()), cancelOnlyKeyboard()
	}
	if len(items) == 0 {
		return fmt.Sprintf("You have no %ss to manage.", ent.Label()), cancelOnlyKeyboard()
	}

	truncated := false
	opts := make([]FlowOption, 0, len(items))
	for i, it := range items {
		if i >= MaximumInlineMenuItems {
			truncated = true
			break
		}
		opts = append(opts, FlowOption{Label: it.Label, Value: strconv.FormatUint(it.ID, 10)})
	}
	state.Options = opts

	text := fmt.Sprintf("📂 Manage your %ss — pick one:", ent.Label())
	if truncated {
		text += fmt.Sprintf("\n\n(Showing the first %d; refine via commands for the rest.)", MaximumInlineMenuItems)
	}
	return text, optionPickerKeyboard(opts)
}

func (telegramBot *TelegramBotAPI) buildManageCard(chatID int64, state *FlowState, ent ManageEntity, note string) (string, *tgbotapi.InlineKeyboardMarkup) {
	state.Screen = manageScreenCard
	state.AwaitText = false
	state.Options = nil

	card, ok := ent.Card(chatID, state.ItemID)
	if !ok {
		// Item vanished — fall back to the list.
		return telegramBot.buildManageList(chatID, state, ent)
	}
	text := card
	if note != "" {
		text = note + "\n\n" + card
	}

	rows := make([][]tgbotapi.InlineKeyboardButton, 0, 2)
	actionRow := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	edit := (CallbackData{Action: FlowEdit}).Json()
	actionRow = append(actionRow, tgbotapi.InlineKeyboardButton{Text: "✏️ Edit", CallbackData: &edit})
	if ent.CanToggle() {
		toggle := (CallbackData{Action: FlowToggle}).Json()
		actionRow = append(actionRow, tgbotapi.InlineKeyboardButton{Text: "🔄 Toggle done", CallbackData: &toggle})
	}
	del := (CallbackData{Action: FlowDelete}).Json()
	actionRow = append(actionRow, tgbotapi.InlineKeyboardButton{Text: "🗑 Delete", CallbackData: &del})
	rows = append(rows, actionRow)

	back := (CallbackData{Action: FlowBack}).Json()
	cancel := (CallbackData{Action: FlowCancel}).Json()
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		{Text: "⬅️ Back to list", CallbackData: &back},
		{Text: "❌ Close", CallbackData: &cancel},
	})
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	return text, &menu
}

func (telegramBot *TelegramBotAPI) buildEditFieldList(state *FlowState, ent ManageEntity) (string, *tgbotapi.InlineKeyboardMarkup) {
	state.Screen = manageScreenEditList
	state.AwaitText = false

	fields := ent.EditFields()
	opts := make([]FlowOption, 0, len(fields))
	for _, f := range fields {
		opts = append(opts, FlowOption{Label: f.Label, Value: f.Key})
	}
	state.Options = opts

	kb := optionPickerKeyboardWithBack(opts)
	return "What do you want to change?", kb
}

func (telegramBot *TelegramBotAPI) buildEditValue(state *FlowState, ent ManageEntity) (string, *tgbotapi.InlineKeyboardMarkup) {
	state.Screen = manageScreenEditVal

	var field editField
	for _, f := range ent.EditFields() {
		if f.Key == state.EditField {
			field = f
			break
		}
	}

	if field.Kind == StepChoice {
		state.AwaitText = false
		state.Options = field.Options
		return fmt.Sprintf("Pick a new value for %s:", field.Label), optionPickerKeyboardWithBack(field.Options)
	}

	state.AwaitText = true
	state.Options = nil
	back := (CallbackData{Action: FlowBack}).Json()
	cancel := (CallbackData{Action: FlowCancel}).Json()
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{{
		{Text: "⬅️ Back", CallbackData: &back},
		{Text: "❌ Cancel", CallbackData: &cancel},
	}}}
	return fmt.Sprintf("✍️ Type the new %s:", field.Label), &menu
}

func (telegramBot *TelegramBotAPI) buildConfirmDelete(chatID int64, state *FlowState, ent ManageEntity) (string, *tgbotapi.InlineKeyboardMarkup) {
	state.Screen = manageScreenConfirm
	state.AwaitText = false
	state.Options = nil

	card, ok := ent.Card(chatID, state.ItemID)
	if !ok {
		return telegramBot.buildManageList(chatID, state, ent)
	}
	yes := (CallbackData{Action: FlowConfirm}).Json()
	no := (CallbackData{Action: FlowBack}).Json()
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{{
		{Text: "🗑 Yes, delete", CallbackData: &yes},
		{Text: "⬅️ No, keep it", CallbackData: &no},
	}}}
	return fmt.Sprintf("Delete this %s? This cannot be undone.\n\n%s", ent.Label(), card), &menu
}

// ---------------------- Keyboard helpers --------------------------------------

func optionPickerKeyboard(opts []FlowOption) *tgbotapi.InlineKeyboardMarkup {
	rows := optionButtonRows(opts)
	cancel := (CallbackData{Action: FlowCancel}).Json()
	rows = append(rows, []tgbotapi.InlineKeyboardButton{{Text: "❌ Cancel", CallbackData: &cancel}})
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	return &menu
}

func optionPickerKeyboardWithBack(opts []FlowOption) *tgbotapi.InlineKeyboardMarkup {
	rows := optionButtonRows(opts)
	back := (CallbackData{Action: FlowBack}).Json()
	cancel := (CallbackData{Action: FlowCancel}).Json()
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		{Text: "⬅️ Back", CallbackData: &back},
		{Text: "❌ Cancel", CallbackData: &cancel},
	})
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	return &menu
}

func optionButtonRows(opts []FlowOption) [][]tgbotapi.InlineKeyboardButton {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0)
	current := make([]tgbotapi.InlineKeyboardButton, 0, MaximumNumberOfRowItems)
	for idx := range opts {
		label := opts[idx].Label
		if len(label) >= MaximumInlineButtonTextLength {
			label = fmt.Sprintf("%s...", truncateUTF8(label, MaximumInlineButtonTextLength-3))
		}
		data := (CallbackData{Action: FlowSelect, FlowOpt: idx}).Json()
		current = append(current, tgbotapi.InlineKeyboardButton{Text: label, CallbackData: &data})
		if len(current) == MaximumNumberOfRowItems {
			rows = append(rows, current)
			current = make([]tgbotapi.InlineKeyboardButton, 0, MaximumNumberOfRowItems)
		}
	}
	if len(current) > 0 {
		rows = append(rows, current)
	}
	return rows
}

func cancelOnlyKeyboard() *tgbotapi.InlineKeyboardMarkup {
	cancel := (CallbackData{Action: FlowCancel}).Json()
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{{
		{Text: "❌ Close", CallbackData: &cancel},
	}}}
	return &menu
}

func priorityFieldOptions() []FlowOption {
	return []FlowOption{{Label: "🔴 High", Value: "high"}, {Label: "⚪ Normal", Value: "normal"}}
}

func percentFieldOptions() []FlowOption {
	return []FlowOption{
		{Label: "0", Value: "0"}, {Label: "25", Value: "25"}, {Label: "50", Value: "50"},
		{Label: "75", Value: "75"}, {Label: "100", Value: "100"},
	}
}

// ---------------------- Entity adapters ---------------------------------------

type ideaManager struct{}

func (ideaManager) Label() string              { return "idea" }
func (ideaManager) CanToggle() bool            { return false }
func (ideaManager) Toggle(int64, uint64) error { return nil }

func (ideaManager) ListItems(chatID int64) ([]manageItem, error) {
	ideas, err := Idea.Load(chatID, false, false, 0)
	if err != nil {
		return nil, err
	}
	items := make([]manageItem, 0, len(ideas))
	for _, idea := range ideas {
		label := idea.Text
		if idea.IsHighPriority {
			label = "🔴 " + label
		}
		items = append(items, manageItem{ID: idea.Id, Label: label})
	}
	return items, nil
}

func (ideaManager) Card(chatID int64, id uint64) (string, bool) {
	ideas, err := Idea.Load(chatID, false, false, 0)
	if err != nil {
		return "", false
	}
	idea, err := ideas.Get(id)
	if err != nil {
		return "", false
	}
	return idea.ToString(), true
}

func (ideaManager) Delete(chatID int64, id uint64) error {
	ideas, err := Idea.Load(chatID, false, false, 0)
	if err != nil {
		return err
	}
	_, err = ideas.Remove(chatID, id)
	return err
}

func (ideaManager) EditFields() []editField {
	return []editField{
		{Key: "text", Label: "📝 Text", Kind: StepText},
		{Key: "priority", Label: "🚦 Priority", Kind: StepChoice, Options: priorityFieldOptions()},
		{Key: "category", Label: "🏷 Category", Kind: StepText},
	}
}

func (ideaManager) ApplyEdit(chatID int64, id uint64, field, value string) error {
	ideas, err := Idea.Load(chatID, false, false, 0)
	if err != nil {
		return err
	}
	terms := []string{strconv.FormatUint(id, 10)}
	switch field {
	case "text":
		terms = append(terms, IdeaTextFlag, value)
	case "priority":
		if value == "high" {
			terms = append(terms, IdeaHighPriorityFlag)
		} else {
			terms = append(terms, IdeaNormalFlag)
		}
	case "category":
		terms = append(terms, IdeaCategoryFlag, value)
	}
	_, err = ideas.Update(chatID, terms)
	return err
}

type togoManager struct{}

func (togoManager) Label() string   { return "togo" }
func (togoManager) CanToggle() bool { return true }

func (togoManager) ListItems(chatID int64) ([]manageItem, error) {
	togos, err := Togo.Load(chatID, false, false)
	if err != nil && togos == nil {
		return nil, err
	}
	items := make([]manageItem, 0, len(togos))
	for _, togo := range togos {
		label := togo.Title
		if togo.Progress >= 100 {
			label = "✅ " + label
		}
		items = append(items, manageItem{ID: togo.Id, Label: label})
	}
	return items, nil
}

func (togoManager) Card(chatID int64, id uint64) (string, bool) {
	togos, err := Togo.Load(chatID, false, false)
	if err != nil && togos == nil {
		return "", false
	}
	togo, err := togos.Get(id)
	if err != nil {
		return "", false
	}
	return togo.ToString(), true
}

func (togoManager) Toggle(chatID int64, id uint64) error {
	togos, err := Togo.Load(chatID, false, false)
	if err != nil && togos == nil {
		return err
	}
	togo, err := togos.Get(id)
	if err != nil {
		return err
	}
	if togo.Progress < 100 {
		togo.Progress = 100
	} else {
		togo.Progress = 0
	}
	return togo.Update(chatID)
}

func (togoManager) Delete(chatID int64, id uint64) error {
	togos, err := Togo.Load(chatID, false, false)
	if err != nil && togos == nil {
		return err
	}
	_, err = togos.Remove(chatID, id)
	return err
}

func (togoManager) EditFields() []editField {
	return []editField{
		{Key: "progress", Label: "📊 Progress", Kind: StepChoice, Options: percentFieldOptions()},
		{Key: "weight", Label: "⚖️ Weight", Kind: StepText},
		{Key: "description", Label: "📝 Description", Kind: StepText},
		{Key: "extra", Label: "⭐ Extra?", Kind: StepChoice, Options: extraOptions()},
	}
}

func (togoManager) ApplyEdit(chatID int64, id uint64, field, value string) error {
	togos, err := Togo.Load(chatID, false, false)
	if err != nil && togos == nil {
		return err
	}
	terms := togoEditTerms(id, field, value)
	_, err = togos.Update(chatID, terms)
	return err
}

type taskManager struct{}

func (taskManager) Label() string   { return "task" }
func (taskManager) CanToggle() bool { return true }

func (taskManager) ListItems(chatID int64) ([]manageItem, error) {
	tasks, err := Task.Load(chatID, true, true)
	if err != nil && tasks == nil {
		return nil, err
	}
	items := make([]manageItem, 0, len(tasks))
	for _, task := range tasks {
		label := task.Title
		if task.Progress >= 100 {
			label = "✅ " + label
		}
		items = append(items, manageItem{ID: task.Id, Label: label})
	}
	return items, nil
}

func (taskManager) Card(chatID int64, id uint64) (string, bool) {
	tasks, err := Task.Load(chatID, true, true)
	if err != nil && tasks == nil {
		return "", false
	}
	task, err := tasks.Get(id)
	if err != nil {
		return "", false
	}
	return task.ToString(Togo.Today().Time), true
}

func (taskManager) Toggle(chatID int64, id uint64) error {
	tasks, err := Task.Load(chatID, true, true)
	if err != nil && tasks == nil {
		return err
	}
	task, err := tasks.Get(id)
	if err != nil {
		return err
	}
	if task.Progress < 100 {
		task.Progress = 100
	} else {
		task.Progress = 0
	}
	return task.Update(chatID)
}

func (taskManager) Delete(chatID int64, id uint64) error {
	tasks, err := Task.Load(chatID, true, true)
	if err != nil && tasks == nil {
		return err
	}
	_, err = tasks.Remove(chatID, id)
	return err
}

func (taskManager) EditFields() []editField {
	return []editField{
		{Key: "progress", Label: "📊 Progress", Kind: StepChoice, Options: percentFieldOptions()},
		{Key: "weight", Label: "⚖️ Weight", Kind: StepText},
		{Key: "description", Label: "📝 Description", Kind: StepText},
		{Key: "extra", Label: "⭐ Extra?", Kind: StepChoice, Options: extraOptions()},
	}
}

func (taskManager) ApplyEdit(chatID int64, id uint64, field, value string) error {
	tasks, err := Task.Load(chatID, true, true)
	if err != nil && tasks == nil {
		return err
	}
	terms := togoEditTerms(id, field, value)
	_, err = tasks.Update(chatID, terms)
	return err
}

// togoEditTerms builds an update term-slice shared by togo and task edits
// (their flag syntax for these fields is identical).
func togoEditTerms(id uint64, field, value string) []string {
	terms := []string{strconv.FormatUint(id, 10)}
	switch field {
	case "progress":
		terms = append(terms, "+p", value)
	case "weight":
		terms = append(terms, "=", value)
	case "description":
		terms = append(terms, ":", value)
	case "extra":
		if value == "extra" {
			terms = append(terms, "+x")
		} else {
			terms = append(terms, "-x")
		}
	}
	return terms
}
