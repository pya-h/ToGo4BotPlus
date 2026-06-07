package Task

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"ToGo4BotPlus/Togo"

	_ "github.com/mattn/go-sqlite3"
)

const DATABASE_NAME string = "./togos.db"

const CREATE_TASKS_TABLE_QUERY string = `CREATE TABLE IF NOT EXISTS tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id BIGINT NOT NULL,
	title VARCHAR(128) NOT NULL,
	description VARCHAR(2048),
	weight INTEGER NOT NULL,
	extra INTEGER NOT NULL,
	progress INTEGER NOT NULL,
	start_date DATETIME,
	created_at DATETIME NOT NULL
)`

const CREATE_TASK_SETTINGS_TABLE_QUERY string = `CREATE TABLE IF NOT EXISTS task_settings (
	owner_id BIGINT PRIMARY KEY,
	reminders_per_day INTEGER NOT NULL DEFAULT 4,
	last_reminder_slot TEXT NOT NULL DEFAULT ''
)`

var allowedReminderTimes = map[int]bool{
	0:  true,
	1:  true,
	2:  true,
	4:  true,
	6:  true,
	8:  true,
	12: true,
	24: true,
}

func AllowedReminderTimes() []int {
	return []int{0, 1, 2, 4, 6, 8, 12, 24}
}

func IsValidReminderTimes(times int) bool {
	return allowedReminderTimes[times]
}

type Task struct {
	Id          uint64
	OwnerId     int64
	Title       string
	Description string
	Weight      uint16
	Progress    uint8
	Extra       bool
	StartDate   *time.Time
	CreatedAt   time.Time
}

type TaskList []Task

type ReminderSetting struct {
	OwnerId          int64
	RemindersPerDay  int
	LastReminderSlot string
}

func (task Task) IsActive(now time.Time) bool {
	if task.StartDate == nil {
		return true
	}
	return !task.StartDate.After(now)
}

func (task *Task) Save() (uint64, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	now := Togo.Today().Time
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.Weight == 0 {
		task.Weight = 1
	}

	extra := 0
	if task.Extra {
		extra = 1
	}

	if res, err := db.Exec(
		"INSERT INTO tasks (owner_id, title, description, weight, extra, progress, start_date, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		task.OwnerId, task.Title, task.Description, task.Weight, extra, task.Progress, task.StartDate, task.CreatedAt,
	); err != nil {
		return 0, err
	} else if id, e := res.LastInsertId(); e == nil {
		if err := EnsureReminderSetting(task.OwnerId); err != nil {
			return 0, err
		}
		return uint64(id), nil
	}

	return 0, errors.New("could not save task due to unknown reason")
}

func isCommand(term string) bool {
	switch term {
	case "+", "%", "#", "#️⃣", "$", "✅", "❌", "/db", "/now", "^", "~", "~s", "&", "✅T", "✅t", "❌T", "❌t", "tk", "TK", "*", ";", ";u", "*x", ">", ">l", ">u", ">x":
		return true
	default:
		return false
	}
}

func normalizeDateToMidnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func parseStartDate(term string) (time.Time, error) {
	now := Togo.Today().Time
	if delta, err := strconv.Atoi(term); err == nil {
		target := normalizeDateToMidnight(now).AddDate(0, 0, delta)
		return target, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02", term, now.Location()); err == nil {
		return normalizeDateToMidnight(parsed), nil
	}
	return time.Time{}, errors.New("start date must be an integer day offset or YYYY-MM-DD")
}

func (task *Task) setFields(terms []string) error {
	numOfTerms := len(terms)
	for i := 1; i < numOfTerms && !isCommand(terms[i]); i++ {
		switch terms[i] {
		case "=", "+w":
			i++
			if i >= numOfTerms {
				return errors.New("weight flag requires a value")
			}
			if _, err := fmt.Sscan(terms[i], &task.Weight); err != nil {
				return errors.New("invalid weight value: " + err.Error())
			}
		case ":", "+d":
			i++
			if i >= numOfTerms {
				return errors.New("description flag requires a value")
			}
			task.Description = terms[i]
		case "+x":
			task.Extra = true
		case "-x":
			task.Extra = false
		case "+p":
			i++
			if i >= numOfTerms {
				return errors.New("progress flag requires a value")
			}
			if _, err := fmt.Sscan(terms[i], &task.Progress); err != nil {
				return errors.New("invalid progress value: " + err.Error())
			}
			if task.Progress > 100 {
				task.Progress = 100
			}
		case "@":
			i++
			if i >= numOfTerms {
				return errors.New("start date flag requires a value")
			}
			parsed, err := parseStartDate(terms[i])
			if err != nil {
				return err
			}
			task.StartDate = &parsed
		case "->":
			return errors.New("duration flag is not supported for tasks")
		}
	}
	return nil
}

func (task *Task) Update(ownerID int64) error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	extra := 0
	if task.Extra {
		extra = 1
	}

	if _, err := db.Exec(
		"UPDATE tasks SET description=?, weight=?, extra=?, progress=?, start_date=? WHERE id=? AND owner_id=?",
		task.Description, task.Weight, extra, task.Progress, task.StartDate, task.Id, ownerID,
	); err != nil {
		return err
	}
	return nil
}

func (task Task) ToString(now time.Time) string {
	status := "active"
	if task.Progress >= 100 {
		status = "done"
	} else if !task.IsActive(now) {
		status = "inactive"
	}

	startAt := "At creation"
	if task.StartDate != nil {
		startAt = task.StartDate.In(now.Location()).Format("2006-01-02")
	}

	return fmt.Sprintf(
		"Task #%d) %s\nStatus: %s\nDescription: %s\nWeight: %d\nExtra: %t\nProgress: %d\nStart: %s",
		task.Id, task.Title, status, task.Description, task.Weight, task.Extra, task.Progress, startAt,
	)
}

func (tasks TaskList) ToString(now time.Time) (result []string) {
	for i := range tasks {
		result = append(result, tasks[i].ToString(now))
	}
	return
}

func (tasks TaskList) Add(newTask *Task) TaskList {
	return append(tasks, *newTask)
}

func (tasks TaskList) ProgressMade() (progress float64, completedInPercent float64, completed uint64, extra uint64, total uint64) {
	totalInPercent := uint64(0)
	for i := range tasks {
		progress += float64(tasks[i].Progress) * float64(tasks[i].Weight)
		if tasks[i].Progress == 100 {
			completed++
			completedInPercent += float64(tasks[i].Progress) * float64(tasks[i].Weight)
		}
		if !tasks[i].Extra {
			totalInPercent += uint64(100 * tasks[i].Weight)
			total++
		} else {
			extra++
		}
	}
	if totalInPercent > 0 {
		progress *= 100 / float64(totalInPercent)
		completedInPercent *= 100 / float64(totalInPercent)
	}
	return
}

func (tasks TaskList) Update(ownerID int64, terms []string) (string, error) {
	if len(terms) == 0 {
		return "", errors.New("task id is required")
	}
	var id uint64
	if _, err := fmt.Sscan(terms[0], &id); err != nil {
		return "", err
	}

	targetIdx := -1
	for i := range tasks {
		if tasks[i].Id == id {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return "", errors.New("there is no task with this id")
	}

	if len(terms) > 1 && !isCommand(terms[1]) {
		if err := tasks[targetIdx].setFields(terms); err != nil {
			return "", err
		}
		if err := tasks[targetIdx].Update(ownerID); err != nil {
			return "", err
		}
	}

	return tasks[targetIdx].ToString(Togo.Today().Time), nil
}

func (tasks TaskList) RemoveIndex(index int) TaskList {
	return slices.Delete(tasks, index, index+1)
}

func (tasks TaskList) Remove(ownerID int64, taskID uint64) (TaskList, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if _, err := db.Exec("DELETE FROM tasks WHERE id=? AND owner_id=?", taskID, ownerID); err != nil {
		return nil, err
	}
	for i := range tasks {
		if tasks[i].Id == taskID && tasks[i].OwnerId == ownerID {
			return tasks.RemoveIndex(i), nil
		}
	}
	return nil, errors.New("no such task found")
}

func (tasks TaskList) Get(taskID uint64) (*Task, error) {
	for i := range tasks {
		if tasks[i].Id == taskID {
			return &tasks[i], nil
		}
	}
	return nil, errors.New("can not find this task")
}

func InitDatabase() error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(CREATE_TASKS_TABLE_QUERY); err != nil {
		return err
	}
	if _, err := db.Exec(CREATE_TASK_SETTINGS_TABLE_QUERY); err != nil {
		return err
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_owner ON tasks(owner_id)"); err != nil {
		return err
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_progress_start ON tasks(owner_id, progress, start_date)"); err != nil {
		return err
	}

	return nil
}

func Load(ownerID int64, includeInactive bool, includeCompleted bool) (tasks TaskList, err error) {
	corruptedRows := 0
	tasks = make(TaskList, 0)
	err = nil

	now := Togo.Today().Time
	if db, e := sql.Open("sqlite3", DATABASE_NAME); e == nil {
		defer db.Close()

		query := "SELECT id, owner_id, title, description, weight, extra, progress, start_date, created_at FROM tasks WHERE owner_id=?"
		args := []interface{}{ownerID}
		if !includeCompleted {
			query += " AND progress < 100"
		}
		if !includeInactive {
			query += " AND (start_date IS NULL OR start_date <= ?)"
			args = append(args, now)
		}
		query += " ORDER BY COALESCE(start_date, created_at), id"

		rows, e := db.Query(query, args...)
		if e != nil {
			err = e
			return
		}
		defer rows.Close()

		for rows.Next() {
			var task Task
			var startDate sql.NullTime
			var createdAt sql.NullTime
			var extra int

			scanErr := rows.Scan(
				&task.Id,
				&task.OwnerId,
				&task.Title,
				&task.Description,
				&task.Weight,
				&extra,
				&task.Progress,
				&startDate,
				&createdAt,
			)
			if scanErr != nil {
				corruptedRows++
				continue
			}

			task.Extra = extra != 0
			if startDate.Valid {
				t := startDate.Time
				task.StartDate = &t
			}
			if createdAt.Valid {
				task.CreatedAt = createdAt.Time
			}
			tasks = tasks.Add(&task)
		}
		if rowErr := rows.Err(); rowErr != nil {
			return tasks, rowErr
		}
	} else {
		err = e
	}

	if corruptedRows > 0 {
		err = errors.New(fmt.Sprint("could not read ", corruptedRows, " tasks from database because some rows seem corrupted"))
	}
	return
}

func LoadActiveOwners(now time.Time) ([]int64, error) {
	owners := make([]int64, 0)
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		"SELECT DISTINCT owner_id FROM tasks WHERE progress < 100 AND (start_date IS NULL OR start_date <= ?) ORDER BY owner_id",
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ownerID int64
		if err := rows.Scan(&ownerID); err != nil {
			return nil, err
		}
		owners = append(owners, ownerID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return owners, nil
}

func EnsureReminderSetting(ownerID int64) error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("INSERT OR IGNORE INTO task_settings (owner_id, reminders_per_day, last_reminder_slot) VALUES (?, 4, '')", ownerID)
	return err
}

func GetReminderSetting(ownerID int64) (ReminderSetting, error) {
	if err := EnsureReminderSetting(ownerID); err != nil {
		return ReminderSetting{}, err
	}

	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return ReminderSetting{}, err
	}
	defer db.Close()

	setting := ReminderSetting{OwnerId: ownerID, RemindersPerDay: 4, LastReminderSlot: ""}
	if err := db.QueryRow("SELECT reminders_per_day, last_reminder_slot FROM task_settings WHERE owner_id=?", ownerID).Scan(&setting.RemindersPerDay, &setting.LastReminderSlot); err != nil {
		return ReminderSetting{}, err
	}
	if !IsValidReminderTimes(setting.RemindersPerDay) {
		setting.RemindersPerDay = 4
	}
	return setting, nil
}

func SetReminderTimes(ownerID int64, times int) error {
	if !IsValidReminderTimes(times) {
		parts := make([]string, 0)
		for _, v := range AllowedReminderTimes() {
			parts = append(parts, fmt.Sprint(v))
		}
		return errors.New("reminder times/day must be one of: " + strings.Join(parts, ", "))
	}

	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(
		"INSERT INTO task_settings (owner_id, reminders_per_day, last_reminder_slot) VALUES (?, ?, '') ON CONFLICT(owner_id) DO UPDATE SET reminders_per_day=excluded.reminders_per_day",
		ownerID,
		times,
	)
	return err
}

func UpdateLastReminderSlot(ownerID int64, slot string) error {
	if err := EnsureReminderSetting(ownerID); err != nil {
		return err
	}
	if slot == "" {
		return errors.New("reminder slot can not be empty")
	}

	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("UPDATE task_settings SET last_reminder_slot=? WHERE owner_id=?", slot, ownerID)
	return err
}

func Extract(ownerID int64, terms []string) (task Task, err error) {
	if len(terms) == 0 {
		return task, errors.New("no task title provided")
	}

	if task.Title = terms[0]; task.Title == "" {
		task.Title = "Untitled"
	}
	task.OwnerId = ownerID
	task.Weight = 1
	task.CreatedAt = Togo.Today().Time
	err = (&task).setFields(terms)
	return
}
