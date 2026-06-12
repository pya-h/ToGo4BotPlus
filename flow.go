package main

import (
	"fmt"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// ============================================================
// Guided-flow engine (Type B menus).
//
// A flow is a linear sequence of steps the bot walks a user through, editing a
// single "wizard" message at each step. Steps collect values either as typed
// text or via inline option buttons. Conversation state lives in memory, keyed
// by chat id (see FlowStore) — fitting this single-instance long-polling bot.
//
// Telegram caps callback_data at 64 bytes, so option buttons never carry their
// value; they carry only an action + an index (FlowOpt) into the options that
// were snapshotted onto the FlowState when the step was rendered. The handler
// resolves the value from that snapshot.
// ============================================================

type StepKind int

const (
	StepText          StepKind = iota // free-text input
	StepChoice                        // pick from static option buttons
	StepDynamicChoice                 // options computed at render time, + Custom
	StepConfirm                       // final review + Save/Cancel
	StepPickItem                      // pick one of the user's existing items (manage flows)
)

type FlowOption struct {
	Label string
	Value string
}

type Step struct {
	Key       string
	Prompt    string
	Label     string // short label shown in the running "answers so far" summary
	Kind      StepKind
	Optional  bool
	Options   []FlowOption                    // static choices (StepChoice)
	OptionsFn func(chatID int64) []FlowOption // dynamic choices (StepDynamicChoice / StepPickItem)
	Validate  func(value string) error        // optional validation for typed/selected values
}

type Flow struct {
	Name    string
	Steps   []Step
	Summary func(data map[string]string) string                        // confirm-screen summary
	Commit  func(chatID int64, data map[string]string) (string, error) // performs the DB action
}

type FlowState struct {
	Flow      string
	Step      int
	Data      map[string]string
	Options   []FlowOption // snapshot of the current step's/screen's selectable options
	AwaitText bool         // current step accepts a typed reply
	MessageID int          // wizard message being edited

	// Manage-flow (list -> card -> edit/delete) fields. Entity is non-empty
	// only for manage flows, which use Screen instead of the linear Step model.
	Entity    string
	Screen    string
	ItemID    uint64
	EditField string
}

// ---------------------- In-memory state store --------------------------------

type FlowStore struct {
	mu     sync.Mutex
	states map[int64]*FlowState
}

func NewFlowStore() *FlowStore {
	return &FlowStore{states: make(map[int64]*FlowState)}
}

func (s *FlowStore) Get(chatID int64) (*FlowState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[chatID]
	return st, ok
}

func (s *FlowStore) Set(chatID int64, st *FlowState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[chatID] = st
}

func (s *FlowStore) Clear(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.states, chatID)
}

func (telegramBot *TelegramBotAPI) ensureFlows() {
	if telegramBot.flows == nil {
		telegramBot.flows = NewFlowStore()
	}
	if telegramBot.imports == nil {
		telegramBot.imports = newImportWaitSet()
	}
}

// ---------------------- Flow command parsing ----------------------------------

// flowCommandToFlowName maps a (lowercased) slash command to a registered flow.
var flowCommandToFlowName = map[string]string{
	"addidea":    "addIdea",
	"addtogo":    "addTogo",
	"addtask":    "addTask",
	"addarticle": "addArticle",
}

// parseFlowCommand detects a e-flow slash command (or /cancel). It returns
// the normalized command, any trailing argument, and whether it matched.
func parseFlowCommand(text string) (cmd string, arg string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "", "", false
	}
	cmd = strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	if at := strings.IndexByte(cmd, '@'); at >= 0 { // strip /cmd@BotName
		cmd = cmd[:at]
	}
	if _, isFlow := flowCommandToFlowName[cmd]; !isFlow && cmd != "cancel" {
		return "", "", false
	}
	arg = strings.TrimSpace(strings.TrimPrefix(text, fields[0]))
	return cmd, arg, true
}

func isFlowAction(action UserAction) bool {
	switch action {
	case FlowSelect, FlowCustom, FlowSkip, FlowBack, FlowConfirm, FlowCancel,
		FlowEdit, FlowDelete, FlowToggle:
		return true
	default:
		return false
	}
}

// ---------------------- Flow lifecycle ----------------------------------------

func (telegramBot *TelegramBotAPI) handleFlowCommand(chatID int64, cmd string, arg string) {
	if cmd == "cancel" {
		telegramBot.cancelActiveFlow(chatID)
		return
	}
	telegramBot.startFlow(chatID, flowCommandToFlowName[cmd], arg)
}

func (telegramBot *TelegramBotAPI) startFlow(chatID int64, flowName string, arg string) {
	flow := lookupFlow(flowName)
	if flow == nil || len(flow.Steps) == 0 {
		telegramBot.SendTextMessage(TelegramResponse{TargetChatId: chatID, TextMsg: "Unknown guided command.", ReplyMarkup: MainKeyboardMenu()})
		return
	}

	state := &FlowState{Flow: flowName, Step: 0, Data: make(map[string]string)}
	if arg != "" {
		state.Data["_arg"] = arg
	}
	telegramBot.prepareStep(chatID, state, flow)

	resp := TelegramResponse{
		TargetChatId:   chatID,
		TextMsg:        stepText(state, flow),
		InlineKeyboard: stepKeyboard(state, flow),
	}
	if id, err := telegramBot.SendTextMessageReturningID(resp); err == nil {
		state.MessageID = id
	}
	telegramBot.flows.Set(chatID, state)
}

func (telegramBot *TelegramBotAPI) cancelActiveFlow(chatID int64) {
	if state, ok := telegramBot.flows.Get(chatID); ok {
		telegramBot.flows.Clear(chatID)
		telegramBot.EditTextMessage(TelegramResponse{
			TargetChatId:         chatID,
			MessageBeingEditedId: state.MessageID,
			TextMsg:              "❌ Cancelled.",
			InlineKeyboard:       emptyInlineKeyboard(),
		})
		return
	}
	telegramBot.SendTextMessage(TelegramResponse{TargetChatId: chatID, TextMsg: "Nothing to cancel.", ReplyMarkup: MainKeyboardMenu()})
}

// prepareStep snapshots the current step's options and whether it awaits text.
func (telegramBot *TelegramBotAPI) prepareStep(chatID int64, state *FlowState, flow *Flow) {
	step := flow.Steps[state.Step]
	state.Options = nil
	state.AwaitText = false
	switch step.Kind {
	case StepText:
		state.AwaitText = true
	case StepChoice:
		state.Options = step.Options
	case StepDynamicChoice, StepPickItem:
		if step.OptionsFn != nil {
			state.Options = step.OptionsFn(chatID)
		}
	}
}

// renderStep re-renders the current step into the wizard message.
func (telegramBot *TelegramBotAPI) renderStep(chatID int64, state *FlowState, flow *Flow) {
	telegramBot.EditTextMessage(TelegramResponse{
		TargetChatId:         chatID,
		MessageBeingEditedId: state.MessageID,
		TextMsg:              stepText(state, flow),
		InlineKeyboard:       stepKeyboard(state, flow),
	})
}

// advance stores a value for the current step and moves to the next one.
func (telegramBot *TelegramBotAPI) advance(chatID int64, state *FlowState, flow *Flow, key string, value string) {
	if key != "" {
		state.Data[key] = value
	}
	state.Step++
	if state.Step >= len(flow.Steps) {
		// No explicit confirm step defined — commit immediately.
		telegramBot.commitFlow(chatID, state, flow)
		return
	}
	telegramBot.prepareStep(chatID, state, flow)
	telegramBot.renderStep(chatID, state, flow)
}

func (telegramBot *TelegramBotAPI) commitFlow(chatID int64, state *FlowState, flow *Flow) {
	telegramBot.flows.Clear(chatID)
	result := "Done."
	if flow.Commit != nil {
		if msg, err := flow.Commit(chatID, state.Data); err == nil {
			result = msg
		} else {
			result = "⚠️ " + err.Error()
		}
	}
	telegramBot.EditTextMessage(TelegramResponse{
		TargetChatId:         chatID,
		MessageBeingEditedId: state.MessageID,
		TextMsg:              result,
		InlineKeyboard:       emptyInlineKeyboard(),
	})
}

// ---------------------- Update routing into the engine ------------------------

func (telegramBot *TelegramBotAPI) handleFlowText(chatID int64, text string, state *FlowState) {
	if state.Entity != "" {
		telegramBot.handleManageText(chatID, text, state)
		return
	}
	flow := lookupFlow(state.Flow)
	if flow == nil {
		telegramBot.flows.Clear(chatID)
		return
	}
	if !state.AwaitText {
		// A choice step is showing; nudge the user back to the buttons.
		telegramBot.renderStep(chatID, state, flow)
		return
	}

	step := flow.Steps[state.Step]
	value := strings.TrimSpace(text)
	if step.Validate != nil {
		if err := step.Validate(value); err != nil {
			telegramBot.EditTextMessage(TelegramResponse{
				TargetChatId:         chatID,
				MessageBeingEditedId: state.MessageID,
				TextMsg:              fmt.Sprintf("⚠️ %s\n\n%s", err.Error(), stepText(state, flow)),
				InlineKeyboard:       stepKeyboard(state, flow),
			})
			return
		}
	}
	telegramBot.advance(chatID, state, flow, step.Key, value)
}

func (telegramBot *TelegramBotAPI) handleFlowCallback(callbackQuery *tgbotapi.CallbackQuery, cb CallbackData) {
	chatID := callbackQuery.Message.Chat.ID
	state, ok := telegramBot.flows.Get(chatID)
	if !ok {
		telegramBot.EditTextMessage(TelegramResponse{
			TargetChatId:         chatID,
			MessageBeingEditedId: callbackQuery.Message.MessageID,
			TextMsg:              "⌛ This guided menu has expired. Start it again from the command list.",
			InlineKeyboard:       emptyInlineKeyboard(),
		})
		return
	}
	// Keep editing the wizard message even if Telegram reported a different id.
	state.MessageID = callbackQuery.Message.MessageID

	if state.Entity != "" {
		telegramBot.handleManageCallback(chatID, cb, state)
		return
	}

	flow := lookupFlow(state.Flow)
	if flow == nil {
		telegramBot.flows.Clear(chatID)
		return
	}
	step := flow.Steps[state.Step]

	switch cb.Action {
	case FlowCancel:
		telegramBot.cancelActiveFlow(chatID)
	case FlowConfirm:
		telegramBot.commitFlow(chatID, state, flow)
	case FlowBack:
		if state.Step > 0 {
			state.Step--
			telegramBot.prepareStep(chatID, state, flow)
			telegramBot.renderStep(chatID, state, flow)
		} else {
			telegramBot.renderStep(chatID, state, flow)
		}
	case FlowSkip:
		if step.Optional {
			telegramBot.advance(chatID, state, flow, step.Key, "")
		} else {
			telegramBot.renderStep(chatID, state, flow)
		}
	case FlowCustom:
		state.AwaitText = true
		telegramBot.EditTextMessage(TelegramResponse{
			TargetChatId:         chatID,
			MessageBeingEditedId: state.MessageID,
			TextMsg:              fmt.Sprintf("%s\n\n✍️ Type a custom value.", step.Prompt),
			InlineKeyboard:       controlOnlyKeyboard(state),
		})
	case FlowSelect:
		if cb.FlowOpt >= 0 && cb.FlowOpt < len(state.Options) {
			telegramBot.advance(chatID, state, flow, step.Key, state.Options[cb.FlowOpt].Value)
		} else {
			telegramBot.renderStep(chatID, state, flow)
		}
	default:
		telegramBot.renderStep(chatID, state, flow)
	}
}

// ---------------------- Rendering helpers -------------------------------------

// flowDivider separates a step's prompt from the running summary of answers
// collected so far. It is a fixed-width rule so the wizard bubble keeps a stable
// minimum width, roughly matching the inline button rows below it.
const flowDivider = "────────────────────"

func stepText(state *FlowState, flow *Flow) string {
	step := flow.Steps[state.Step]
	if step.Kind == StepConfirm {
		summary := ""
		if flow.Summary != nil {
			summary = flow.Summary(state.Data)
		}
		return fmt.Sprintf("%s\n%s\n%s", step.Prompt, flowDivider, summary)
	}

	header := step.Prompt
	if step.Optional {
		header += " (optional)"
	}
	hint := ""
	if state.AwaitText {
		hint = "\n\n✍️ Type your answer below."
	} else if step.Kind == StepChoice || step.Kind == StepDynamicChoice || step.Kind == StepPickItem {
		hint = "\n\n👇 Pick an option below."
	}
	base := fmt.Sprintf("%s\n\nStep %d of %d%s", header, state.Step+1, len(flow.Steps), hint)
	if collected := collectedSummary(state, flow); collected != "" {
		base += fmt.Sprintf("\n%s\n%s", flowDivider, collected)
	}
	return base
}

// collectedSummary renders the "answers so far" block: one "Label: value" line
// per already-answered step (in order), so the user can see their typed input
// echoed back after each step's message is deleted.
func collectedSummary(state *FlowState, flow *Flow) string {
	lines := make([]string, 0, state.Step)
	for s := 0; s < state.Step && s < len(flow.Steps); s++ {
		st := flow.Steps[s]
		if st.Label == "" || st.Kind == StepConfirm {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", st.Label, stepDisplayValue(st, state.Data[st.Key])))
	}
	return strings.Join(lines, "\n")
}

// stepDisplayValue maps a stored value to its human label for the summary: an
// empty (skipped) value shows "—", and a choice value shows its option label
// rather than the raw stored value (e.g. "🔴 High" instead of "high").
func stepDisplayValue(step Step, value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	for _, o := range step.Options {
		if o.Value == value {
			return o.Label
		}
	}
	return value
}

func stepKeyboard(state *FlowState, flow *Flow) *tgbotapi.InlineKeyboardMarkup {
	step := flow.Steps[state.Step]

	if step.Kind == StepConfirm {
		save := (CallbackData{Action: FlowConfirm}).Json()
		cancel := (CallbackData{Action: FlowCancel}).Json()
		row := []tgbotapi.InlineKeyboardButton{
			{Text: "✅ Save", CallbackData: &save},
			{Text: "❌ Cancel", CallbackData: &cancel},
		}
		menu := tgbotapi.NewInlineKeyboardMarkup(row)
		return &menu
	}

	rows := make([][]tgbotapi.InlineKeyboardButton, 0)

	// Option buttons in rows of MaximumNumberOfRowItems.
	current := make([]tgbotapi.InlineKeyboardButton, 0, MaximumNumberOfRowItems)
	for idx := range state.Options {
		label := state.Options[idx].Label
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

	if controls := controlRow(state); len(controls) > 0 {
		rows = append(rows, controls)
	}

	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	return &menu
}

// controlRow builds the Back / Custom / Skip / Cancel control buttons for a step.
func controlRow(state *FlowState) []tgbotapi.InlineKeyboardButton {
	flow := lookupFlow(state.Flow)
	step := flow.Steps[state.Step]

	row := make([]tgbotapi.InlineKeyboardButton, 0, 4)
	if state.Step > 0 {
		back := (CallbackData{Action: FlowBack}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "⬅️ Back", CallbackData: &back})
	}
	if step.Kind == StepDynamicChoice {
		custom := (CallbackData{Action: FlowCustom}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "✏️ Custom", CallbackData: &custom})
	}
	if step.Optional {
		skip := (CallbackData{Action: FlowSkip}).Json()
		row = append(row, tgbotapi.InlineKeyboardButton{Text: "⏭️ Skip", CallbackData: &skip})
	}
	cancel := (CallbackData{Action: FlowCancel}).Json()
	row = append(row, tgbotapi.InlineKeyboardButton{Text: "❌ Cancel", CallbackData: &cancel})
	return row
}

// controlOnlyKeyboard is used while awaiting a custom typed value.
func controlOnlyKeyboard(state *FlowState) *tgbotapi.InlineKeyboardMarkup {
	menu := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{controlRow(state)}}
	return &menu
}

func emptyInlineKeyboard() *tgbotapi.InlineKeyboardMarkup {
	return &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
}

// ---------------------- Flow registry -----------------------------------------

var (
	flowsOnce   sync.Once
	flowsByName map[string]*Flow
)

func lookupFlow(name string) *Flow {
	flowsOnce.Do(func() {
		flowsByName = buildFlows()
	})
	return flowsByName[name]
}
