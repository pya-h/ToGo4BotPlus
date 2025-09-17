package ToGo4BotPlus

import (
	// chrono "github.com/gochrono/chrono"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	// _ "github.com/lib/pq" // postgres
)

const DATABASE_NAME string = "./togos.db"

// var taskScheduler chrono.TaskScheduler = chrono.NewDefaultTaskScheduler()

// ---------------------- Date/Time Struct & Date Receivers --------------------------------
type Date struct {
	time.Time
}

func (d *Date) Get() string {
	return fmt.Sprintf("%d-%d-%d\t%d:%d", d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute())
}
func (d *Date) Short() string {

	return fmt.Sprintf("%d-%d-%d", d.Year(), d.Month(), d.Day())
}

// func Now() time.Time {
// if timezone, err := time.LoadLocation("Asia/Tehran"); err == nil {
// 	return time.Now().In(timezone)
// }
// 	return time.Now()
// }

func (date Date) ToLocal() Date {
	if timezone, err := time.LoadLocation("Asia/Tehran"); err == nil {
		return Date{date.In(timezone)}
	}
	return date
}

func Now() Date {
	return Date{time.Now()}
}

func Today() Date {
	return Now().ToLocal()
}

// ---------------------- Togo Struct & Togo Receivers--------------------------------
type Togo struct {
	Id          uint64
	Title       string
	Description string
	Weight      uint16
	Progress    uint8
	Extra       bool
	Date        Date
	Duration    time.Duration
	OwnerId     int64 // telegram id
}

func (togo *Togo) Save() (uint64, error) {
	const CREATE_TABLE_QUERY string = `CREATE TABLE IF NOT EXISTS togos (id INTEGER PRIMARY KEY AUTOINCREMENT, owner_id BIGINT NOT NULL,
	title VARCHAR(64) NOT NULL, description VARCHAR(1024), weight INTEGER, extra INTEGER,
	progress INTEGER, date DATETIME, duration INTEGER)`

	db, err := sql.Open("sqlite3", DATABASE_NAME)

	if err != nil {
		return 0, err
	}
	defer db.Close()
	if _, err := db.Exec(CREATE_TABLE_QUERY); err != nil {
		return 0, err
	}
	extra := 0
	if togo.Extra {
		extra = 1
	}
	if res, err := db.Exec("INSERT INTO togos (owner_id, title, description, weight, extra, progress, date, duration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		togo.OwnerId, togo.Title, togo.Description, togo.Weight, extra, togo.Progress,
		togo.Date.Time, togo.Duration.Minutes()); err != nil {
		return 0, err
	} else if id, e := res.LastInsertId(); e == nil {
		return uint64(id), nil
	}
	return 0, errors.New("bot couldn't save this togo due to unknown reason")
}

func isCommand(term string) bool {
	return term == "+" || term == "%" || term == "#" || term == "$"
}

func (togo *Togo) setFields(terms []string) error {
	numOfTerms := len(terms)
	for i := 1; i < numOfTerms && !isCommand(terms[i]); i++ {
		switch terms[i] {
		case "=", "+w":
			i++
			if _, err := fmt.Sscan(terms[i], &togo.Weight); err != nil {
				return errors.New(fmt.Sprint("Error at Togo:{", togo.Title, "}:", err.Error()))
			}

		case ":", "+d":
			i++
			togo.Description = terms[i]
		case "+x":
			togo.Extra = true
		case "-x":
			togo.Extra = false
		case "+p":
			i++
			if _, err := fmt.Sscan(terms[i], &togo.Progress); err != nil {
				return err
			} else if togo.Progress > 100 {
				togo.Progress = 100
			}
		case "@":
			// im++
			i++
			date := Today()
			var delta int = 0
			if _, err := fmt.Sscan(terms[i], &delta); err == nil {
				i++
			}
			date = Date{date.AddDate(0, 0, delta)}
			temp := strings.Split(terms[i], ":")
			hour := uint8(0)
			min := uint8(0)
			if _, err := fmt.Sscan(temp[0], &hour); err != nil {
				// means that user has entered @ Days
				hour = uint8(0)
			} else if hour >= uint8(24) {
				return errors.New(fmt.Sprint("Error at Togo:{", togo.Title, "}; Hour part of the time must be between 0 and 23"))
			} else if len(temp) > 1 {
				if _, err := fmt.Sscan(temp[1], &min); err != nil {
					return errors.New(fmt.Sprint("Error at Togo:{", togo.Title, "}; Provided time is invalid:", terms[i]))
				} else if min >= uint8(60) {
					return errors.New("minute part must be between 0 and 59")
				}
			}

			if locale, err := time.LoadLocation("Asia/Tehran"); err == nil {
				togo.Date = Date{time.Date(date.Year(), date.Month(), date.Day(), int(hour), int(min), 0, 0, locale)}
			} else {
				togo.Date = Date{time.Date(date.Year(), date.Month(), date.Day(), int(hour), int(min), 0, 0, time.Local)}
			}
		case "->":
			i++
			if _, err := fmt.Sscan(terms[i], &togo.Duration); err != nil {
				return errors.New(fmt.Sprint("Error at Togo:{", togo.Title, "}:", err.Error()))
			} else if togo.Duration > 0 {
				togo.Duration *= time.Minute
			} else {
				return errors.New(fmt.Sprint("Error at Togo:{", togo.Title, "}: Duration must be positive integer!"))
			}
		}

	}
	return nil
}

func (togo *Togo) Update(ownerID int64) error {
	db, err := sql.Open("sqlite3", DATABASE_NAME)

	if err != nil {
		return err
	}
	defer db.Close()

	extra := 0
	if togo.Extra {
		extra = 1
	}
	if _, err := db.Exec("UPDATE togos SET description=?, weight=?, extra=?, progress=?, date=?, duration=? WHERE id=? AND owner_id=?", // TODO: check ownerId? (no need)
		togo.Description, togo.Weight, extra, togo.Progress, togo.Date.Time, togo.Duration.Minutes(), togo.Id, ownerID); err != nil {
		return err
	}
	return nil
}

func (togo *Togo) ToString() string {
	return fmt.Sprintf("Togo #%d) %s:\t%s\nWeight: %d\nExtra: %t\nProgress: %d\nAt: %s, about %.1f minutes",
		togo.Id, togo.Title, togo.Description, togo.Weight, togo.Extra, togo.Progress, togo.Date.Get(), togo.Duration.Minutes())
}

// ---------------------- TogoList Type & Togo Receivers--------------------------------
type TogoList []Togo

func (togos TogoList) ToString() (result []string) {
	//result = "- - - - - - - - - - - - - - - - - - - - - -"
	for i := range togos {
		result = append(result, togos[i].ToString())
	}
	return
}

func (togos TogoList) Add(newTogo *Togo) TogoList {
	return append(togos, *newTogo)
}

func (togos TogoList) ProgressMade() (progress float64, completedInPercent float64, completed uint64, extra uint64, total uint64) {
	totalInPercent := uint64(0)
	for i := range togos {
		progress += float64(togos[i].Progress) * float64(togos[i].Weight)
		if togos[i].Progress == 100 {
			completed++
			completedInPercent += float64(togos[i].Progress) * float64(togos[i].Weight)
		}
		if !togos[i].Extra {
			totalInPercent += uint64(100 * togos[i].Weight)
			total++
		} else {
			extra++
		}
	}
	if totalInPercent > 0 {
		progress *= 100 / float64(totalInPercent) // CHECK IF IT CALCULAFES DECIMAL PART OR NOT
		completedInPercent *= 100 / float64(totalInPercent)
	}
	return
}

func (togos TogoList) Update(chatID int64, terms []string) (string, error) {
	var id uint64
	if _, err := fmt.Sscan(terms[0], &id); err != nil {
		return "", err
	}
	targetIdx := -1
	for i := range togos {
		if togos[i].Id == id {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		return "", errors.New("there is no togo with this Id")
	}
	if len(terms) > 1 && !isCommand(terms[1]) {

		togos[targetIdx].setFields(terms)
		togos[targetIdx].Update(chatID)
	}

	return togos[targetIdx].ToString(), nil
}

func (togos TogoList) RemoveIndex(index int) TogoList {
	return slices.Delete(togos, index, index+1)
}

func (togos TogoList) Remove(ownerID int64, togoID uint64) (TogoList, error) {
	db, err := sql.Open("sqlite3", DATABASE_NAME)

	if err != nil {
		return nil, err
	}
	defer db.Close()

	if _, err := db.Exec("DELETE FROM togos WHERE id=? AND owner_id=?", togoID, ownerID); err != nil {
		return nil, err
	}
	for i := range togos {
		if togos[i].Id == togoID && togos[i].OwnerId == ownerID {
			return togos.RemoveIndex(i), nil
		}
	}
	return nil, errors.New("no such togo found")
}

func (togos TogoList) Get(togoID uint64) (*Togo, error) {
	// TODO: use simple version of FOR
	for i := range togos {
		if togos[i].Id == togoID {
			return &togos[i], nil
		}
	}
	return nil, errors.New("can not find this togo")
}

// ---------------------- Shared Functions --------------------------------
func Load(ownerId int64, justToday bool) (togos TogoList, err error) {
	currupted_rows := 0
	togos = make(TogoList, 0)
	err = nil
	if db, e := sql.Open("sqlite3", DATABASE_NAME); e == nil {
		defer db.Close()
		// ***** BETTER ALGORITHM
		// FIRST GET THE COUNT OF ROWS, then create a slice of that size and then load into that.
		const SELECT_QUERY string = "SELECT id, owner_id, title, description, weight, extra, progress, date, duration FROM togos WHERE owner_id=? ORDER BY date"
		/* if justToday {
			today := Date{time.Now()}
			next := Date{today.AddDate(0, 0, 1)}
			fmt.Println(next.Short())
			SELECT_QUERY = fmt.Sprintf("%s WHERE date >= DATETIME(%s)", SELECT_QUERY, today.Short())//, next.Short())
			fmt.Println(SELECT_QUERY)
		}*/
		rows, e := db.Query(SELECT_QUERY, ownerId)
		if e != nil {
			err = e
			return
		}

		now := Today()
		for rows.Next() {
			var togo Togo
			var date time.Time

			err = rows.Scan(&togo.Id, &togo.OwnerId, &togo.Title, &togo.Description, &togo.Weight, &togo.Extra, &togo.Progress, &date, &togo.Duration)
			if timezone, err := time.LoadLocation("Asia/Tehran"); err == nil {
				togo.Date = Date{date.In(timezone)}
			} else {
				togo.Date = Date{date}
			}
			togo.Duration *= time.Minute
			if err != nil {
				currupted_rows++
				continue
			}
			if togo.Date.Short() == now.Short() || !justToday {
				togos = togos.Add(&togo)
			}
		}
	} else {
		err = e
	}

	if currupted_rows > 0 {
		err = errors.New(fmt.Sprint("bot couldn't read ", currupted_rows, " togos from database because their data seem currupted"))
	}
	return
}

func LoadEverybodysToday() (TogoList, error) {

	togos := make(TogoList, 0)
	currupted_rows := 0
	if db, err := sql.Open("sqlite3", DATABASE_NAME); err == nil {
		defer db.Close()
		const SELECT_QUERY string = "SELECT id, owner_id, title, description, weight, extra, progress, date, duration FROM togos WHERE date BETWEEN ? AND ? ORDER BY date"
		today := Today()

		rows, err := db.Query(SELECT_QUERY, today.Time, today.AddDate(0, 0, 1))
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var togo Togo
			var date time.Time

			err = rows.Scan(&togo.Id, &togo.OwnerId, &togo.Title, &togo.Description, &togo.Weight, &togo.Extra, &togo.Progress, &date, &togo.Duration)
			if err != nil {
				currupted_rows++
				continue
			}

			if timezone, err := time.LoadLocation("Asia/Tehran"); err == nil {
				togo.Date = Date{date.In(timezone)}
			} else {
				togo.Date = Date{date}
			}
			togo.Duration *= time.Minute

			togos = togos.Add(&togo)
		}
	} else {
		return nil, err
	}
	var warning error = nil

	if currupted_rows > 0 {
		warning = errors.New(fmt.Sprint("bot couldn't read ", currupted_rows, " togos from database because their data seem currupted"))
	}
	return togos, warning
}

func Extract(ownerId int64, terms []string) (togo Togo, err error) {
	// setting default values
	if togo.Title = terms[0]; togo.Title == "" {
		togo.Title = "Untitled"
	}
	togo.OwnerId = ownerId
	togo.Weight = 1
	togo.Date = Today()
	err = (&togo).setFields(terms)
	return
}
