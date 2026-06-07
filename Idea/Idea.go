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

// Categories live in their own table and ideas reference them by id
// (ideas.category_id). This keeps the idea rows compact and lets callback data
// carry a small integer id instead of a free-form category string.
const CREATE_IDEAS_TABLE_QUERY string = `CREATE TABLE IF NOT EXISTS ideas (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id BIGINT NOT NULL,
	text VARCHAR(2048) NOT NULL,
	is_high_priority INTEGER NOT NULL DEFAULT 0,
	is_favorite INTEGER NOT NULL DEFAULT 0,
	category_id INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL
)`

const CREATE_IDEA_CATEGORIES_TABLE_QUERY string = `CREATE TABLE IF NOT EXISTS idea_categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id BIGINT NOT NULL,
	name VARCHAR(128) NOT NULL,
	use_count INTEGER NOT NULL DEFAULT 1,
	UNIQUE (owner_id, name)
)`

// MaximumCategorySuggestions caps how many remembered categories are surfaced
// as inline suggestion buttons in the guided flow.
const MaximumCategorySuggestions = 12

type Idea struct {
	Id             uint64
	OwnerId        int64
	Text           string
	IsHighPriority bool
	IsFavorite     bool
	CategoryId     int64
	Category       string // display name (resolved from category_id on load)
	CreatedAt      Togo.Date
}

type IdeaList []Idea

// Category is a per-owner category with its row id (used in callback data).
type Category struct {
	Id   int64
	Name string
}

// Header returns the first line of an idea's text, for compact menu rendering.
func (idea Idea) Header() string {
	text := strings.TrimSpace(idea.Text)
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		text = strings.TrimSpace(text[:idx])
	}
	if text == "" {
		return "(empty)"
	}
	return text
}

func (idea Idea) ToString() string {
	priority := "Normal"
	if idea.IsHighPriority {
		priority = "🔴 High"
	}
	category := idea.Category
	if category == "" {
		category = "—"
	}
	favorite := "No"
	if idea.IsFavorite {
		favorite = "❤️ Yes"
	}
	return fmt.Sprintf("💡 Idea #%d) %s\nPriority: %s\nCategory: %s\nFavorite: %s", idea.Id, idea.Text, priority, category, favorite)
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
		"tk", "TK", "*", ";", ";u", "*x",
		">", ">l", ">u", ">x":
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

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
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

	// A brand-new idea with a category is a genuine use of that category, so
	// create-or-bump its usage and store the resulting id.
	catID, err := registerCategoryDB(db, idea.OwnerId, idea.Category)
	if err != nil {
		return 0, err
	}
	idea.CategoryId = catID

	res, err := db.Exec(
		"INSERT INTO ideas (owner_id, text, is_high_priority, is_favorite, category_id, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		idea.OwnerId, idea.Text, boolToInt(idea.IsHighPriority), boolToInt(idea.IsFavorite), idea.CategoryId, idea.CreatedAt.Time,
	)
	if err != nil {
		return 0, err
	}
	id, e := res.LastInsertId()
	if e != nil {
		return 0, errors.New("could not save idea due to unknown reason")
	}
	return uint64(id), nil
}

func (idea *Idea) Update(ownerID int64) error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	// Resolve the category to an id, bumping its usage only when it actually
	// changed (so unrelated edits like a text tweak don't inflate use_count).
	newCatID := int64(0)
	if name := strings.TrimSpace(idea.Category); name != "" {
		existingID, lookupErr := lookupCategoryIDDB(db, ownerID, name)
		if lookupErr != nil {
			return lookupErr
		}
		if existingID != 0 && existingID == idea.CategoryId {
			newCatID = existingID
		} else {
			newCatID, err = registerCategoryDB(db, ownerID, name)
			if err != nil {
				return err
			}
		}
	}
	idea.CategoryId = newCatID

	_, err = db.Exec(
		"UPDATE ideas SET text=?, is_high_priority=?, category_id=? WHERE id=? AND owner_id=?",
		idea.Text, boolToInt(idea.IsHighPriority), idea.CategoryId, idea.Id, ownerID,
	)
	return err
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

// ToggleFavorite flips an idea's favorite flag and returns the new state.
func ToggleFavorite(ownerID int64, ideaID uint64) (bool, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return false, err
	}
	defer db.Close()

	res, err := db.Exec("UPDATE ideas SET is_favorite = 1 - is_favorite WHERE id=? AND owner_id=?", ideaID, ownerID)
	if err != nil {
		return false, err
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return false, errors.New("no such idea found")
	}

	var fav int
	if err := db.QueryRow("SELECT is_favorite FROM ideas WHERE id=? AND owner_id=?", ideaID, ownerID).Scan(&fav); err != nil {
		return false, err
	}
	return fav != 0, nil
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

// Index returns the position of ideaID within the (already ordered) list, or -1.
func (ideas IdeaList) Index(ideaID uint64) int {
	for i := range ideas {
		if ideas[i].Id == ideaID {
			return i
		}
	}
	return -1
}

func InitDatabase() error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(CREATE_IDEA_CATEGORIES_TABLE_QUERY); err != nil {
		return err
	}
	if _, err := db.Exec(CREATE_IDEAS_TABLE_QUERY); err != nil {
		return err
	}
	// Defensive migration for any pre-existing ideas table from an earlier schema.
	addColumnIfMissing(db, "ideas", "is_favorite", "INTEGER NOT NULL DEFAULT 0")
	addColumnIfMissing(db, "ideas", "category_id", "INTEGER NOT NULL DEFAULT 0")
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_ideas_owner ON ideas(owner_id)"); err != nil {
		return err
	}
	return nil
}

// addColumnIfMissing adds a column to a table when it isn't already present.
// Errors are intentionally ignored: a duplicate-column ALTER is a no-op for us.
func addColumnIfMissing(db *sql.DB, table, column, definition string) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return
		}
		if name == column {
			return // already present
		}
	}
	_, _ = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
}

// Load returns an owner's ideas, optionally filtered. categoryID == 0 means no
// category filter; pass a real id (see LookupCategoryID) to filter by category.
func Load(ownerID int64, onlyHighPriority bool, onlyFavorites bool, categoryID int64) (ideas IdeaList, err error) {
	corruptedRows := 0
	ideas = make(IdeaList, 0)
	err = nil

	if db, e := sql.Open("sqlite3", DATABASE_NAME); e == nil {
		defer db.Close()

		query := `SELECT i.id, i.owner_id, i.text, i.is_high_priority, i.is_favorite, i.category_id, COALESCE(c.name, ''), i.created_at
			FROM ideas i LEFT JOIN idea_categories c ON c.id = i.category_id
			WHERE i.owner_id=?`
		args := []interface{}{ownerID}
		if onlyHighPriority {
			query += " AND i.is_high_priority=1"
		}
		if onlyFavorites {
			query += " AND i.is_favorite=1"
		}
		if categoryID != 0 {
			query += " AND i.category_id=?"
			args = append(args, categoryID)
		}
		query += " ORDER BY i.is_high_priority DESC, i.created_at DESC, i.id DESC"

		rows, e := db.Query(query, args...)
		if e != nil {
			err = e
			return
		}
		defer rows.Close()

		for rows.Next() {
			var idea Idea
			var priority, favorite int
			var createdAt sql.NullTime

			scanErr := rows.Scan(&idea.Id, &idea.OwnerId, &idea.Text, &priority, &favorite, &idea.CategoryId, &idea.Category, &createdAt)
			if scanErr != nil {
				corruptedRows++
				continue
			}
			idea.IsHighPriority = priority != 0
			idea.IsFavorite = favorite != 0
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

// LoadFavoriteOwners returns the distinct owners that have at least one favorite
// idea — used to drive the favorite-idea reminder tick efficiently.
func LoadFavoriteOwners() ([]int64, error) {
	owners := make([]int64, 0)
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT DISTINCT owner_id FROM ideas WHERE is_favorite=1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var owner int64
		if err := rows.Scan(&owner); err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, rows.Err()
}

// registerCategoryDB creates the category if needed (use_count 1) or bumps its
// usage, returning the category id. A blank name yields id 0 (uncategorized).
func registerCategoryDB(db *sql.DB, ownerID int64, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	if _, err := db.Exec(
		"INSERT INTO idea_categories (owner_id, name, use_count) VALUES (?, ?, 1) ON CONFLICT(owner_id, name) DO UPDATE SET use_count = use_count + 1",
		ownerID, name,
	); err != nil {
		return 0, err
	}
	return lookupCategoryIDDB(db, ownerID, name)
}

func lookupCategoryIDDB(db *sql.DB, ownerID int64, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, nil
	}
	var id int64
	err := db.QueryRow("SELECT id FROM idea_categories WHERE owner_id=? AND name=?", ownerID, name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

// RegisterCategory create-or-bumps a category usage and returns its id.
func RegisterCategory(ownerID int64, name string) (int64, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	return registerCategoryDB(db, ownerID, name)
}

// LookupCategoryID returns the id of an owner's category by name (0 if absent).
func LookupCategoryID(ownerID int64, name string) (int64, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	return lookupCategoryIDDB(db, ownerID, name)
}

// LoadCategories returns an owner's remembered category names, most-used first.
func LoadCategories(ownerID int64) ([]string, error) {
	cats, err := LoadCategoryList(ownerID)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(cats))
	for _, c := range cats {
		names = append(names, c.Name)
	}
	return names, nil
}

// LoadCategoryList returns an owner's remembered categories (id + name),
// most-used first, capped at MaximumCategorySuggestions.
func LoadCategoryList(ownerID int64) ([]Category, error) {
	categories := make([]Category, 0)
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		"SELECT id, name FROM idea_categories WHERE owner_id=? ORDER BY use_count DESC, name LIMIT ?",
		ownerID, MaximumCategorySuggestions,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.Id, &c.Name); err != nil {
			return nil, err
		}
		categories = append(categories, c)
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
