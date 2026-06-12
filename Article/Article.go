package Article

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

// An article is a saved link: a title, an optional category (by id, in its own
// table, mirroring ideas) and the url the user wants to revisit.
const CREATE_ARTICLES_TABLE_QUERY string = `CREATE TABLE IF NOT EXISTS articles (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id BIGINT NOT NULL,
	title VARCHAR(512) NOT NULL,
	category_id INTEGER NOT NULL DEFAULT 0,
	url VARCHAR(2048) NOT NULL DEFAULT '',
	read INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL
)`

const CREATE_ARTICLE_CATEGORIES_TABLE_QUERY string = `CREATE TABLE IF NOT EXISTS article_categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	owner_id BIGINT NOT NULL,
	name VARCHAR(128) NOT NULL,
	use_count INTEGER NOT NULL DEFAULT 1,
	UNIQUE (owner_id, name)
)`

// MaximumCategorySuggestions caps how many remembered categories are surfaced
// as inline suggestion buttons in the guided flow.
const MaximumCategorySuggestions = 12

type Article struct {
	Id         uint64
	OwnerId    int64
	Title      string
	CategoryId int64
	Category   string // display name (resolved from category_id on load)
	Url        string
	Read       bool // whether the user has marked this article as read
	CreatedAt  Togo.Date
}

type ArticleList []Article

// Category is a per-owner category with its row id (used in callback data).
type Category struct {
	Id   int64
	Name string
}

// Header returns the first line of an article's title, for compact rendering.
func (article Article) Header() string {
	title := strings.TrimSpace(article.Title)
	if idx := strings.IndexByte(title, '\n'); idx >= 0 {
		title = strings.TrimSpace(title[:idx])
	}
	if title == "" {
		return "(untitled)"
	}
	return title
}

func (article Article) ToString() string {
	category := article.Category
	if category == "" {
		category = "—"
	}
	url := article.Url
	if url == "" {
		url = "(no link)"
	}
	status := "📖 Unread"
	if article.Read {
		status = "✅ Read"
	}
	return fmt.Sprintf("🔗 Article #%d) %s\nCategory: %s\nStatus: %s\n%s", article.Id, article.Title, category, status, url)
}

func (articles ArticleList) ToString() (result []string) {
	for i := range articles {
		result = append(result, articles[i].ToString())
	}
	return
}

func (articles ArticleList) Add(newArticle *Article) ArticleList {
	return append(articles, *newArticle)
}

// isCommand reports whether term is a top-level command token of any entity, so
// setFields stops consuming flag values at a chained command boundary. The
// article flags (+t, +u, +c) are values, not boundaries, so are excluded.
func isCommand(term string) bool {
	switch term {
	case "+", "%", "#", "#️⃣", "$", "✅", "❌", "/db", "/now",
		"^", "~", "~s", "~n", "&", "✅T", "✅t", "❌T", "❌t",
		"tk", "TK", "*", ";", ";u", "*x",
		">", ">l", ">u", ">x":
		return true
	default:
		return false
	}
}

func (article *Article) setFields(terms []string) error {
	numOfTerms := len(terms)
	for i := 1; i < numOfTerms && !isCommand(terms[i]); i++ {
		switch terms[i] {
		case "+t":
			i++
			if i >= numOfTerms {
				return errors.New("title flag (+t) requires a value")
			}
			article.Title = terms[i]
		case "+u":
			i++
			if i >= numOfTerms {
				return errors.New("url flag (+u) requires a value")
			}
			article.Url = strings.TrimSpace(terms[i])
		case "+c":
			i++
			if i >= numOfTerms {
				return errors.New("category flag (+c) requires a value")
			}
			article.Category = strings.TrimSpace(terms[i])
		}
	}
	return nil
}

func (article *Article) Save() (uint64, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if article.CreatedAt.IsZero() {
		article.CreatedAt = Togo.Today()
	}

	catID, err := registerCategoryDB(db, article.OwnerId, article.Category)
	if err != nil {
		return 0, err
	}
	article.CategoryId = catID

	res, err := db.Exec(
		"INSERT INTO articles (owner_id, title, category_id, url, created_at) VALUES (?, ?, ?, ?, ?)",
		article.OwnerId, article.Title, article.CategoryId, article.Url, article.CreatedAt.Time,
	)
	if err != nil {
		return 0, err
	}
	id, e := res.LastInsertId()
	if e != nil {
		return 0, errors.New("could not save article due to unknown reason")
	}
	return uint64(id), nil
}

func (article *Article) Update(ownerID int64) error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	// Resolve the category to an id, bumping its usage only when it changed.
	newCatID := int64(0)
	if name := strings.TrimSpace(article.Category); name != "" {
		existingID, lookupErr := lookupCategoryIDDB(db, ownerID, name)
		if lookupErr != nil {
			return lookupErr
		}
		if existingID != 0 && existingID == article.CategoryId {
			newCatID = existingID
		} else {
			newCatID, err = registerCategoryDB(db, ownerID, name)
			if err != nil {
				return err
			}
		}
	}
	article.CategoryId = newCatID

	_, err = db.Exec(
		"UPDATE articles SET title=?, category_id=?, url=? WHERE id=? AND owner_id=?",
		article.Title, article.CategoryId, article.Url, article.Id, ownerID,
	)
	return err
}

func (articles ArticleList) Update(ownerID int64, terms []string) (string, error) {
	if len(terms) == 0 {
		return "", errors.New("article id is required")
	}
	var id uint64
	if _, err := fmt.Sscan(terms[0], &id); err != nil {
		return "", err
	}

	targetIdx := -1
	for i := range articles {
		if articles[i].Id == id {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return "", errors.New("there is no article with this id")
	}

	if len(terms) > 1 && !isCommand(terms[1]) {
		if err := articles[targetIdx].setFields(terms); err != nil {
			return "", err
		}
		if err := articles[targetIdx].Update(ownerID); err != nil {
			return "", err
		}
	}

	return articles[targetIdx].ToString(), nil
}

func (articles ArticleList) RemoveIndex(index int) ArticleList {
	return slices.Delete(articles, index, index+1)
}

func (articles ArticleList) Remove(ownerID int64, articleID uint64) (ArticleList, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if _, err := db.Exec("DELETE FROM articles WHERE id=? AND owner_id=?", articleID, ownerID); err != nil {
		return nil, err
	}
	for i := range articles {
		if articles[i].Id == articleID && articles[i].OwnerId == ownerID {
			return articles.RemoveIndex(i), nil
		}
	}
	return nil, errors.New("no such article found")
}

func (articles ArticleList) Get(articleID uint64) (*Article, error) {
	for i := range articles {
		if articles[i].Id == articleID {
			return &articles[i], nil
		}
	}
	return nil, errors.New("can not find this article")
}

// Index returns the position of articleID within the list, or -1.
func (articles ArticleList) Index(articleID uint64) int {
	for i := range articles {
		if articles[i].Id == articleID {
			return i
		}
	}
	return -1
}

// addColumnIfMissing adds a column to a table when it isn't already present.
// Used for defensive in-place migrations against older databases.
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
			return
		}
	}
	_, _ = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
}

func InitDatabase() error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(CREATE_ARTICLE_CATEGORIES_TABLE_QUERY); err != nil {
		return err
	}
	if _, err := db.Exec(CREATE_ARTICLES_TABLE_QUERY); err != nil {
		return err
	}
	// Older databases predate the read flag; add it in place.
	addColumnIfMissing(db, "articles", "read", "INTEGER NOT NULL DEFAULT 0")
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_articles_owner ON articles(owner_id)"); err != nil {
		return err
	}
	return nil
}

// Load returns an owner's articles, newest first; categoryID == 0 means no
// category filter.
func Load(ownerID int64, categoryID int64) (ArticleList, error) {
	return load(ownerID, categoryID, false)
}

// LoadUnread returns an owner's not-yet-read articles, newest first — used by
// the daily reminder so already-read links are never resurfaced.
func LoadUnread(ownerID int64) (ArticleList, error) {
	return load(ownerID, 0, true)
}

// load is the shared query worker. unreadOnly restricts results to articles the
// user hasn't marked read yet.
func load(ownerID int64, categoryID int64, unreadOnly bool) (articles ArticleList, err error) {
	corruptedRows := 0
	articles = make(ArticleList, 0)
	err = nil

	if db, e := sql.Open("sqlite3", DATABASE_NAME); e == nil {
		defer db.Close()

		query := `SELECT a.id, a.owner_id, a.title, a.category_id, COALESCE(c.name, ''), a.url, a.read, a.created_at
			FROM articles a LEFT JOIN article_categories c ON c.id = a.category_id
			WHERE a.owner_id=?`
		args := []interface{}{ownerID}
		if categoryID != 0 {
			query += " AND a.category_id=?"
			args = append(args, categoryID)
		}
		if unreadOnly {
			query += " AND a.read=0"
		}
		query += " ORDER BY a.created_at DESC, a.id DESC"

		rows, e := db.Query(query, args...)
		if e != nil {
			err = e
			return
		}
		defer rows.Close()

		for rows.Next() {
			var article Article
			var createdAt sql.NullTime

			scanErr := rows.Scan(&article.Id, &article.OwnerId, &article.Title, &article.CategoryId, &article.Category, &article.Url, &article.Read, &createdAt)
			if scanErr != nil {
				corruptedRows++
				continue
			}
			if createdAt.Valid {
				article.CreatedAt = Togo.Date{Time: createdAt.Time}
			}
			articles = articles.Add(&article)
		}
		if rowErr := rows.Err(); rowErr != nil {
			return articles, rowErr
		}
	} else {
		err = e
	}

	if corruptedRows > 0 {
		err = errors.New(fmt.Sprint("could not read ", corruptedRows, " articles from database because some rows seem corrupted"))
	}
	return
}

// SetRead persists the read flag for a single article and updates the receiver.
func (article *Article) SetRead(ownerID int64, read bool) error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec("UPDATE articles SET read=? WHERE id=? AND owner_id=?", read, article.Id, ownerID); err != nil {
		return err
	}
	article.Read = read
	return nil
}

// LoadOwnersWithUnreadArticles returns the distinct owners that have at least
// one unread article — used to drive the daily article reminder tick
// efficiently (owners whose articles are all read are skipped entirely).
func LoadOwnersWithUnreadArticles() ([]int64, error) {
	owners := make([]int64, 0)
	db, err := sql.Open("sqlite3", DATABASE_NAME)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT DISTINCT owner_id FROM articles WHERE read=0")
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
		"INSERT INTO article_categories (owner_id, name, use_count) VALUES (?, ?, 1) ON CONFLICT(owner_id, name) DO UPDATE SET use_count = use_count + 1",
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
	err := db.QueryRow("SELECT id FROM article_categories WHERE owner_id=? AND name=?", ownerID, name).Scan(&id)
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
		"SELECT id, name FROM article_categories WHERE owner_id=? ORDER BY use_count DESC, name LIMIT ?",
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

func Extract(ownerID int64, terms []string) (article Article, err error) {
	if len(terms) == 0 {
		return article, errors.New("no article title provided")
	}

	if article.Title = terms[0]; article.Title == "" {
		article.Title = "Untitled article"
	}
	article.OwnerId = ownerID
	article.CreatedAt = Togo.Today()
	err = (&article).setFields(terms)
	return
}
