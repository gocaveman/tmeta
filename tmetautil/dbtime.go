// Utility stuff that goes along with tmeta but is not core functionality.
package tmetautil

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// NewDBTime returns a DBTime with the time set to now.
func NewDBTime() DBTime {
	return DBTime{Time: time.Now()}
}

// NewDBTimeFor returns a DBTime corresponding to the time you provide.
func NewDBTimeFor(t time.Time) DBTime {
	return DBTime{Time: t}
}

// DBTime is a time value that should work with SQLite3, MySQL and Postgres.
// It supports subsecond precision, works with the database's underlying DATETIME
// field type (TEXT for SQLite3), won't get confused about time zones (stores in UTC),
// scans and JSON encodes/decodes properly.
type DBTime struct {
	time.Time
}

func (t DBTime) Value() (driver.Value, error) {
	if t.IsZero() {
		return nil, nil
	}
	// use UTC to avoid time zone ambiguity
	return t.Time.UTC().Format(`2006-01-02T15:04:05.999999999`), nil
}

func (t *DBTime) Scan(value interface{}) error {

	if value == nil {
		t.Time = time.Time{}
		return nil
	}

	var s string
	switch v := value.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	case time.Time: // mysql driver has an option to parse times
		t.Time = v.Local()
		return nil
	default:
		return fmt.Errorf("DBTime.Scan: unable to scan type %T", value)
	}

	// MySQL uses a space instead of a "T", replace before parsing
	s = strings.Replace(s, " ", "T", 1)

	var err error
	// use UTC to avoid time zone ambiguity
	t.Time, err = time.ParseInLocation(`2006-01-02T15:04:05.999999999`, s, time.UTC)
	// switch to local time zone
	t.Time = t.Time.Local()
	return err
}
