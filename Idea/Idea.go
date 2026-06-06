package Idea

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"ToGo4BotPlus/Togo"

	_ "github.com/mattn/go-sqlite3"
)

const DATABASE_NAME string = "./togos.db"

const CREATE_IDEAS_TABLE_QUERY string = `CREATE TABLE IF NOT EXISTS ideas (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id BIGINT NOT NULL,
	text VARCHAR(2048) NOT NULL,
	is_high_priority INTEGER NOT NULL DEFAULT 0,
	category VARCHAR(128) NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL
)`

const CREATE_IDEA_CATEGORIES_TABLE_QUERY string = `CREATE TABLE IF NOT EXISTS idea_categories (
	owner_id BIGINT NOT NULL,
	name VARCHAR(128) NOT NULL,
	use_count INTEGER NOT NULL DEFAULT 1,
	PRIMARY KEY (owner_id, name)
)`

// MaximumCategorySuggestions caps how many remembered categories are surfaced
// as inline suggestion buttons in the guided flow.
const MaximumCategorySuggestions = 12

type Idea struct {
	Id             uint64
	OwnerId        int64
	Text           string
	IsHighPriority bool
	Category       string
	CreatedAt      Togo.Date
}

type IdeaList []Idea

func (idea Idea) ToString() string {
	priority := "Normal"
	if idea.IsHighPriority {
		priority = "🔴 High"
	}
	category := idea.Category
	if category == "" {
		category = "—"
	}
	return fmt.Sprintf("💡 Idea #%d) %s\nPriority: %s\nCategory: %s", idea.Id, idea.Text, priority, category)
}

func (ideas IdeaList) ToString() (result []string) {
	for i := range ideas {
		result = append(result, ideas[i].ToString())
	}
	return
}

func (ideas IdeaList) Add(newIdea *Idea) IdeaList {
	return append(ideas, *newIdea)
}

// isCommand reports whether term is a top-level command token of any entity.
// setFields stops consuming flags when it reaches one so chained commands keep
// working. The idea flags (+!, -!, +c, +t) are intentionally NOT listed here:
// they are values to consume, not boundaries.
func isCommand(term string) bool {
	switch term {
	case "+", "%", "#", "#️⃣", "$", "✅", "❌", "/db", "/now",
		"^", "~", "~s", "&", "✅T", "✅t", "❌T", "❌t",
		"tk", "TK", "*", ";", ";u", "*x":
		return true
	default:
		return false
	}
}

func (idea *Idea) setFields(terms []string) error {
	numOfTerms := len(terms)
	for i := 1; i < numOfTerms && !isCommand(terms[i]); i++ {
		switch terms[i] {
		case "+!":
			idea.IsHighPriority = true
		case "-!":
			idea.IsHighPriority = false
		case "+c":
			i++
			if i >= numOfTerms {
				return errors.New("category flag (+c) requires a value")
			}
			idea.Category = strings.TrimSpace(terms[i])
		case "+t":
			i++
			if i >= numOfTerms {
				return errors.New("text flag (+t) requires a value")
			}
			idea.Text = terms[i]
		}
	}
	return nil
}

func (idea *Idea) Save() (uint64, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if idea.CreatedAt.IsZero() {
		idea.CreatedAt = Togo.Today()
	}

	priority := 0
	if idea.IsHighPriority {
		priority = 1
	}

	res, err := db.Exec(
		"INSERT INTO ideas (owner_id, text, is_high_priority, category, created_at) VALUES (?, ?, ?, ?, ?)",
		idea.OwnerId, idea.Text, priority, idea.Category, idea.CreatedAt.Time,
	)
	if err != nil {
		return 0, err
	}
	id, e := res.LastInsertId()
	if e != nil {
		return 0, errors.New("could not save idea due to unknown reason")
	}
	if err := RegisterCategory(idea.OwnerId, idea.Category); err != nil {
		return 0, err
	}
	return uint64(id), nil
}

func (idea *Idea) Update(ownerID int64) error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	priority := 0
	if idea.IsHighPriority {
		priority = 1
	}

	if _, err := db.Exec(
		"UPDATE ideas SET text=?, is_high_priority=?, category=? WHERE id=? AND owner_id=?",
		idea.Text, priority, idea.Category, idea.Id, ownerID,
	); err != nil {
		return err
	}
	return RegisterCategory(ownerID, idea.Category)
}

func (ideas IdeaList) Update(ownerID int64, terms []string) (string, error) {
	if len(terms) == 0 {
		return "", errors.New("idea id is required")
	}
	var id uint64
	if _, err := fmt.Sscan(terms[0], &id); err != nil {
		return "", err
	}

	targetIdx := -1
	for i := range ideas {
		if ideas[i].Id == id {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return "", errors.New("there is no idea with this id")
	}

	if len(terms) > 1 && !isCommand(terms[1]) {
		if err := ideas[targetIdx].setFields(terms); err != nil {
			return "", err
		}
		if err := ideas[targetIdx].Update(ownerID); err != nil {
			return "", err
		}
	}

	return ideas[targetIdx].ToString(), nil
}

func (ideas IdeaList) RemoveIndex(index int) IdeaList {
	return slices.Delete(ideas, index, index+1)
}

func (ideas IdeaList) Remove(ownerID int64, ideaID uint64) (IdeaList, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if _, err := db.Exec("DELETE FROM ideas WHERE id=? AND owner_id=?", ideaID, ownerID); err != nil {
		return nil, err
	}
	for i := range ideas {
		if ideas[i].Id == ideaID && ideas[i].OwnerId == ownerID {
			return ideas.RemoveIndex(i), nil
		}
	}
	return nil, errors.New("no such idea found")
}

func (ideas IdeaList) Get(ideaID uint64) (*Idea, error) {
	for i := range ideas {
		if ideas[i].Id == ideaID {
			return &ideas[i], nil
		}
	}
	return nil, errors.New("can not find this idea")
}

func InitDatabase() error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(CREATE_IDEAS_TABLE_QUERY); err != nil {
		return err
	}
	if _, err := db.Exec(CREATE_IDEA_CATEGORIES_TABLE_QUERY); err != nil {
		return err
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_ideas_owner ON ideas(owner_id)"); err != nil {
		return err
	}
	return nil
}

func Load(ownerID int64, onlyHighPriority bool, category string) (ideas IdeaList, err error) {
	corruptedRows := 0
	ideas = make(IdeaList, 0)
	err = nil

	if db, e := sql.Open("sqlite3", DATABASE_NAME); e == nil {
		defer db.Close()

		query := "SELECT id, owner_id, text, is_high_priority, category, created_at FROM ideas WHERE owner_id=?"
		args := []interface{}{ownerID}
		if onlyHighPriority {
			query += " AND is_high_priority=1"
		}
		if category != "" {
			query += " AND category=?"
			args = append(args, category)
		}
		query += " ORDER BY is_high_priority DESC, created_at DESC, id DESC"

		rows, e := db.Query(query, args...)
		if e != nil {
			err = e
			return
		}
		defer rows.Close()

		for rows.Next() {
			var idea Idea
			var priority int
			var createdAt sql.NullTime

			scanErr := rows.Scan(&idea.Id, &idea.OwnerId, &idea.Text, &priority, &idea.Category, &createdAt)
			if scanErr != nil {
				corruptedRows++
				continue
			}
			idea.IsHighPriority = priority != 0
			if createdAt.Valid {
				idea.CreatedAt = Togo.Date{Time: createdAt.Time}
			}
			ideas = ideas.Add(&idea)
		}
		if rowErr := rows.Err(); rowErr != nil {
			return ideas, rowErr
		}
	} else {
		err = e
	}

	if corruptedRows > 0 {
		err = errors.New(fmt.Sprint("could not read ", corruptedRows, " ideas from database because some rows seem corrupted"))
	}
	return
}

// RegisterCategory records a per-owner category usage so it can be suggested
// later. A blank category is a no-op.
func RegisterCategory(ownerID int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(
		"INSERT INTO idea_categories (owner_id, name, use_count) VALUES (?, ?, 1) ON CONFLICT(owner_id, name) DO UPDATE SET use_count = use_count + 1",
		ownerID, name,
	)
	return err
}

// LoadCategories returns an owner's remembered categories, most-used first.
func LoadCategories(ownerID int64) ([]string, error) {
	categories := make([]string, 0)
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		"SELECT name FROM idea_categories WHERE owner_id=? ORDER BY use_count DESC, name LIMIT ?",
		ownerID, MaximumCategorySuggestions,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		categories = append(categories, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}

func Extract(ownerID int64, terms []string) (idea Idea, err error) {
	if len(terms) == 0 {
		return idea, errors.New("no idea text provided")
	}

	if idea.Text = terms[0]; idea.Text == "" {
		idea.Text = "Untitled idea"
	}
	idea.OwnerId = ownerID
	idea.CreatedAt = Togo.Today()
	err = (&idea).setFields(terms)
	return
}
