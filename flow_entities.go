package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ToGo4BotPlus/Idea"
	"ToGo4BotPlus/Task"
	"ToGo4BotPlus/Togo"
)

// buildFlows constructs the guided-flow registry once (see lookupFlow). Each
// flow is a linear step sequence whose Commit reuses the existing domain
// Extract/Save/Update code so parsing/validation stays single-sourced.
func buildFlows() map[string]*Flow {
	flows := map[string]*Flow{}
	flows["addIdea"] = newAddIdeaFlow()
	flows["addTogo"] = newAddTogoFlow()
	flows["addTask"] = newAddTaskFlow()
	return flows
}

func nonEmptyText(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("please enter some text")
	}
	return nil
}

func staticOptions(opts ...FlowOption) func(int64) []FlowOption {
	return func(int64) []FlowOption { return opts }
}

func numberOptions(values ...int) func(int64) []FlowOption {
	opts := make([]FlowOption, 0, len(values))
	for _, v := range values {
		s := strconv.Itoa(v)
		opts = append(opts, FlowOption{Label: s, Value: s})
	}
	return func(int64) []FlowOption { return opts }
}

func validatePositiveInt(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n <= 0 {
		return errors.New("enter a positive whole number")
	}
	return nil
}

// validateNonNegativeInt allows 0 (e.g. the togo "day" step where 0 = today),
// unlike validatePositiveInt which rejects it.
func validateNonNegativeInt(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 {
		return errors.New("enter 0 or a positive whole number")
	}
	return nil
}

func validatePercent(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 || n > 100 {
		return errors.New("enter a number between 0 and 100")
	}
	return nil
}

func validateHHMM(value string) error {
	if _, err := time.Parse("15:04", strings.TrimSpace(value)); err != nil {
		return errors.New("use HH:MM (e.g. 14:30)")
	}
	return nil
}

func validateStartDate(value string) error {
	value = strings.TrimSpace(value)
	if _, err := strconv.Atoi(value); err == nil {
		return nil
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return nil
	}
	return errors.New("use a day number (0=today) or YYYY-MM-DD")
}

func extraOptions() []FlowOption {
	return []FlowOption{{Label: "Normal", Value: "normal"}, {Label: "⭐ Extra", Value: "extra"}}
}

func ideaCategoryOptions(chatID int64) []FlowOption {
	cats, err := Idea.LoadCategories(chatID)
	if err != nil {
		return nil
	}
	opts := make([]FlowOption, 0, len(cats))
	for _, c := range cats {
		opts = append(opts, FlowOption{Label: c, Value: c})
	}
	return opts
}

func newAddIdeaFlow() *Flow {
	return &Flow{
		Name: "addIdea",
		Steps: []Step{
			{
				Key:      "text",
				Prompt:   "💡 What's your idea?",
				Kind:     StepText,
				Validate: nonEmptyText,
			},
			{
				Key:    "priority",
				Prompt: "How important is it?",
				Kind:   StepChoice,
				Options: []FlowOption{
					{Label: "🔴 High", Value: "high"},
					{Label: "⚪ Normal", Value: "normal"},
				},
			},
			{
				Key:       "category",
				Prompt:    "Pick a category (or add a custom one).",
				Kind:      StepDynamicChoice,
				Optional:  true,
				OptionsFn: ideaCategoryOptions,
			},
			{
				Prompt: "Review your idea:",
				Kind:   StepConfirm,
			},
		},
		Summary: func(data map[string]string) string {
			priority := "⚪ Normal"
			if data["priority"] == "high" {
				priority = "🔴 High"
			}
			category := data["category"]
			if category == "" {
				category = "—"
			}
			return fmt.Sprintf("Text: %s\nPriority: %s\nCategory: %s", data["text"], priority, category)
		},
		Commit: func(chatID int64, data map[string]string) (string, error) {
			terms := []string{data["text"]}
			if data["priority"] == "high" {
				terms = append(terms, IdeaHighPriorityFlag)
			} else {
				terms = append(terms, IdeaNormalFlag)
			}
			if category := strings.TrimSpace(data["category"]); category != "" {
				terms = append(terms, IdeaCategoryFlag, category)
			}

			idea, err := Idea.Extract(chatID, terms)
			if err != nil {
				return "", err
			}
			id, err := idea.Save()
			if err != nil {
				return "", err
			}
			idea.Id = id
			return fmt.Sprintf("✅ Saved!\n\n%s", idea.ToString()), nil
		},
	}
}

func newAddTogoFlow() *Flow {
	return &Flow{
		Name: "addTogo",
		Steps: []Step{
			{Key: "title", Prompt: "➕ Togo title?", Kind: StepText, Validate: nonEmptyText},
			{Key: "weight", Prompt: "Weight (importance)?", Kind: StepDynamicChoice, Optional: true,
				OptionsFn: numberOptions(1, 2, 3, 5), Validate: validatePositiveInt},
			{Key: "progress", Prompt: "Progress so far (%)?", Kind: StepDynamicChoice, Optional: true,
				OptionsFn: numberOptions(0, 25, 50, 75, 100), Validate: validatePercent},
			{Key: "extra", Prompt: "Is it an extra togo?", Kind: StepChoice, Options: extraOptions()},
			{Key: "day", Prompt: "Schedule in how many days?", Kind: StepDynamicChoice, Optional: true,
				OptionsFn: staticOptions(
					FlowOption{Label: "Today", Value: "0"},
					FlowOption{Label: "Tomorrow", Value: "1"},
					FlowOption{Label: "+2 days", Value: "2"},
					FlowOption{Label: "+7 days", Value: "7"},
				), Validate: validateNonNegativeInt},
			{Key: "time", Prompt: "At what time (HH:MM)?", Kind: StepText, Optional: true, Validate: validateHHMM},
			{Key: "duration", Prompt: "Duration in minutes?", Kind: StepDynamicChoice, Optional: true,
				OptionsFn: numberOptions(15, 30, 60, 90), Validate: validatePositiveInt},
			{Prompt: "Review your togo:", Kind: StepConfirm},
		},
		Summary: func(data map[string]string) string {
			return fmt.Sprintf(
				"Title: %s\nWeight: %s\nProgress: %s\nType: %s\nSchedule: %s\nDuration: %s",
				data["title"], orDash(data["weight"], "1"), orDash(data["progress"], "0"),
				extraLabel(data["extra"]), scheduleLabel(data["day"], data["time"]), orDash(data["duration"], "—"))
		},
		Commit: func(chatID int64, data map[string]string) (string, error) {
			terms := []string{data["title"]}
			if w := data["weight"]; w != "" {
				terms = append(terms, "=", w)
			}
			if p := data["progress"]; p != "" {
				terms = append(terms, "+p", p)
			}
			if data["extra"] == "extra" {
				terms = append(terms, "+x")
			} else {
				terms = append(terms, "-x")
			}
			day, tm := data["day"], data["time"]
			if tm == "" && day != "" {
				tm = "00:00"
			}
			if tm != "" {
				if day != "" {
					terms = append(terms, "@", day, tm)
				} else {
					terms = append(terms, "@", tm)
				}
			}
			if d := data["duration"]; d != "" {
				terms = append(terms, "->", d)
			}

			togo, err := Togo.Extract(chatID, terms)
			if err != nil {
				return "", err
			}
			id, err := togo.Save()
			if err != nil {
				return "", err
			}
			togo.Id = id
			return fmt.Sprintf("✅ Togo #%d saved!\n\n%s", id, togo.ToString()), nil
		},
	}
}

func newAddTaskFlow() *Flow {
	return &Flow{
		Name: "addTask",
		Steps: []Step{
			{Key: "title", Prompt: "^ Task title?", Kind: StepText, Validate: nonEmptyText},
			{Key: "weight", Prompt: "Weight (importance)?", Kind: StepDynamicChoice, Optional: true,
				OptionsFn: numberOptions(1, 2, 3, 5), Validate: validatePositiveInt},
			{Key: "progress", Prompt: "Progress so far (%)?", Kind: StepDynamicChoice, Optional: true,
				OptionsFn: numberOptions(0, 25, 50, 75, 100), Validate: validatePercent},
			{Key: "extra", Prompt: "Is it an extra task?", Kind: StepChoice, Options: extraOptions()},
			{Key: "start", Prompt: "When does it start?", Kind: StepDynamicChoice, Optional: true,
				OptionsFn: staticOptions(
					FlowOption{Label: "Today", Value: "0"},
					FlowOption{Label: "Tomorrow", Value: "1"},
					FlowOption{Label: "+7 days", Value: "7"},
				), Validate: validateStartDate},
			{Prompt: "Review your task:", Kind: StepConfirm},
		},
		Summary: func(data map[string]string) string {
			start := data["start"]
			if start == "" {
				start = "At creation"
			}
			return fmt.Sprintf(
				"Title: %s\nWeight: %s\nProgress: %s\nType: %s\nStart: %s",
				data["title"], orDash(data["weight"], "1"), orDash(data["progress"], "0"),
				extraLabel(data["extra"]), start)
		},
		Commit: func(chatID int64, data map[string]string) (string, error) {
			terms := []string{data["title"]}
			if w := data["weight"]; w != "" {
				terms = append(terms, "=", w)
			}
			if p := data["progress"]; p != "" {
				terms = append(terms, "+p", p)
			}
			if data["extra"] == "extra" {
				terms = append(terms, "+x")
			} else {
				terms = append(terms, "-x")
			}
			if s := data["start"]; s != "" {
				terms = append(terms, "@", s)
			}

			task, err := Task.Extract(chatID, terms)
			if err != nil {
				return "", err
			}
			id, err := task.Save()
			if err != nil {
				return "", err
			}
			task.Id = id
			return fmt.Sprintf("✅ Task #%d saved!", id), nil
		},
	}
}

func orDash(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func extraLabel(value string) string {
	if value == "extra" {
		return "⭐ Extra"
	}
	return "Normal"
}

func scheduleLabel(day, tm string) string {
	if day == "" && tm == "" {
		return "Unscheduled"
	}
	if tm == "" {
		tm = "00:00"
	}
	if day == "" {
		return fmt.Sprintf("Today %s", tm)
	}
	return fmt.Sprintf("+%s day(s) %s", day, tm)
}
