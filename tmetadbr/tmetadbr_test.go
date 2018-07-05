package tmetadbr

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gocraft/dbr"
	"github.com/stretchr/testify/assert"
)

func TestCRUD(t *testing.T) {
	assert := assert.New(t)
	sess, meta, err := doSetup("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	b := New(sess, meta)

	// insert one
	_, err = b.MustInsert(&Author{
		AuthorID:   "author_0001",
		NomDePlume: "Barack Obama",
	}).Exec()
	assert.NoError(err)

	// TODO: insert multiple

	// select one
	var author Author
	err = b.MustSelectByID(&author, "author_0001").LoadOne(&author)
	assert.NoError(err)
	assert.Equal(author.NomDePlume, "Barack Obama")

	// select slice
	var authorList []Author
	_, err = b.MustSelect(&authorList).Where("author_id = ?", "author_0001").Load(&authorList)
	assert.NoError(err)
	assert.Len(authorList, 1)
	assert.Equal(authorList[0].NomDePlume, "Barack Obama")

	// update
	author.NomDePlume = "Barack Hussein Obama"
	assert.NoError(b.ResultWithOneUpdate(b.MustUpdateByID(&author).Exec()))

	// delete
	assert.NoError(b.ResultWithOneUpdate(b.MustDeleteByID(Author{}, "author_0001").Exec()))

	// make sure it's gone
	err = b.MustSelectByID(&author, "author_0001").LoadOne(&author)
	assert.Equal(dbr.ErrNotFound, err)
}

func TestTx(t *testing.T) {

	// do a few quick things just to make sure transactions generally work

	assert := assert.New(t)
	sess, meta, err := doSetup("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	tx, err := sess.Begin()
	assert.NoError(err)
	defer tx.RollbackUnlessCommitted()

	b := New(tx, meta)

	_, err = b.MustInsert(&Author{
		AuthorID:   "author_0001",
		NomDePlume: "Donald Trump",
	}).Exec()
	assert.NoError(err)

	var author Author
	err = b.MustSelectByID(&author, "author_0001").LoadOne(&author)
	assert.NoError(err)
	assert.Equal(author.NomDePlume, "Donald Trump")

	assert.NoError(tx.Commit())

}

func TestCRUDVersion(t *testing.T) {
	t.Logf("TODO: TestCRUDVersion")
	t.SkipNow()
}

func TestAutoIncrement(t *testing.T) {
	t.Logf("TODO: TestAutoIncrement")
	t.SkipNow()
	// TODO: make sure last insert id acts like it's supposed to
	// TODO: what about uuid generation? is that even still called?
}

func TestRelationBelongsTo(t *testing.T) {

	assert := assert.New(t)
	sess, meta, err := doSetup("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	b := New(sess, meta)

	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Author{
		AuthorID:   "author_0001",
		NomDePlume: "Albert Einstein",
	}).Exec()))

	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Book{
		BookID:   "book_0001",
		Title:    "The World as I See it",
		AuthorID: "author_0001",
	}).Exec()))

	// pull the book
	var book Book
	assert.NoError(b.MustSelectByID(&book, "book_0001").LoadOne(&book))
	assert.NotEmpty(book.BookID)

	// now load it's "belongs_to" Author
	bookT := b.For(Book{})
	assert.NoError(b.MustSelectRelation(&book, "author").
		LoadOne(bookT.RelationTargetPtr(&book, "author")))
	// NOTE: we could just write ...LoadOne(&book.Author) but this shows how to do it dynamically
	assert.NotNil(book.Author)
	assert.Equal("Albert Einstein", book.Author.NomDePlume)

}

func TestRelationHasMany(t *testing.T) {

	assert := assert.New(t)
	sess, meta, err := doSetup("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	b := New(sess, meta)

	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Author{
		AuthorID:   "author_0001",
		NomDePlume: "Albert Einstein",
	}).Exec()))

	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Book{
		BookID:   "book_0001",
		Title:    "The World as I See it",
		AuthorID: "author_0001",
	}).Exec()))

	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Book{
		BookID:   "book_0002",
		Title:    "Relativity: The Special and the General Theory",
		AuthorID: "author_0001",
	}).Exec()))

	var author Author
	assert.NoError(b.MustSelectByID(&author, "author_0001").LoadOne(&author))

	authorT := b.For(Author{})
	_, err = b.MustSelectRelation(&author, "book_list").
		Load(authorT.RelationTargetPtr(&author, "book_list"))
	assert.NoError(err)
	assert.Len(author.BookList, 2)

}

func TestRelationHasOne(t *testing.T) {

	assert := assert.New(t)
	sess, meta, err := doSetup("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	b := New(sess, meta)

	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Category{
		CategoryID: "category_0001",
		Name:       "Horror",
	}).Exec()))

	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&CategoryInfo{
		CategoryID: "category_0001",
		InfoStuff:  "info stuff value",
	}).Exec()))

	var category Category
	assert.NoError(b.MustSelectByID(&category, "category_0001").LoadOne(&category))

	categoryT := b.For(Category{})
	assert.NoError(b.MustSelectRelation(&category, "category_info").
		LoadOne(categoryT.RelationTargetPtr(&category, "category_info")))
	assert.Equal("info stuff value", category.CategoryInfo.InfoStuff)

}

func TestRelationBelongsToMany(t *testing.T) {

	assert := assert.New(t)
	sess, meta, err := doSetup("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	b := New(sess, meta)

	bookT := b.For(Book{})

	// insert book
	book := Book{
		BookID: "book_0001",
		Title:  "Ender's Game",
	}
	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&book).Exec()))

	// insert two categories
	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Category{
		CategoryID: "category_0001",
		Name:       "Science Fiction",
	}).Exec()))
	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Category{
		CategoryID: "category_0002",
		Name:       "Adventure",
	}).Exec()))

	// link
	book.CategoryIDList = []string{"category_0001", "category_0002"}
	assert.NoError(b.ExecOK(b.MustDeleteRelationNotIn(&book, "category_id_list")))
	assert.NoError(b.ExecOK(b.MustInsertRelationIgnore(&book, "category_id_list")))

	// select and make sure it shows up
	book.CategoryList = nil
	_, err = b.MustSelectRelation(&book, "category_list").
		Load(bookT.RelationTargetPtr(&book, "category_list"))
	assert.NoError(err)
	assert.Len(book.CategoryList, 2)

}

func TestRelationBelongsToManyIDs(t *testing.T) {

	assert := assert.New(t)
	sess, meta, err := doSetup("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	b := New(sess, meta)

	bookT := b.For(Book{})

	// insert book
	book := Book{
		BookID: "book_0001",
		Title:  "Ender's Game",
	}
	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&book).Exec()))

	// insert two categories
	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Category{
		CategoryID: "category_0001",
		Name:       "Science Fiction",
	}).Exec()))
	assert.NoError(b.ResultWithOneUpdate(b.MustInsert(&Category{
		CategoryID: "category_0002",
		Name:       "Adventure",
	}).Exec()))

	// join to one
	book.CategoryIDList = []string{"category_0002"}
	assert.NoError(b.ExecOK(b.MustDeleteRelationNotIn(&book, "category_id_list")))
	assert.NoError(b.ExecOK(b.MustInsertRelationIgnore(&book, "category_id_list")))

	// add another one
	book.CategoryIDList = []string{"category_0001", "category_0002"}
	assert.NoError(b.ExecOK(b.MustDeleteRelationNotIn(&book, "category_id_list")))
	assert.NoError(b.ExecOK(b.MustInsertRelationIgnore(&book, "category_id_list")))

	// removing both (checks the zero element case)
	book.CategoryIDList = nil
	assert.NoError(b.ExecOK(b.MustDeleteRelationNotIn(&book, "category_id_list")))
	assert.NoError(b.ExecOK(b.MustInsertRelationIgnore(&book, "category_id_list")))

	// add the two back
	book.CategoryIDList = []string{"category_0001", "category_0002"}
	assert.NoError(b.ExecOK(b.MustDeleteRelationNotIn(&book, "category_id_list")))
	assert.NoError(b.ExecOK(b.MustInsertRelationIgnore(&book, "category_id_list")))

	// then load the relation and make sure both show up
	book.CategoryIDList = nil
	_, err = b.MustSelectRelation(&book, "category_id_list").
		Load(bookT.RelationTargetPtr(&book, "category_id_list"))
	assert.NoError(err)
	assert.Len(book.CategoryIDList, 2)
	assert.Contains(book.CategoryIDList, "category_0001")
	assert.Contains(book.CategoryIDList, "category_0002")

}

// func NewDBNanoTime() DBNanoTime {
// 	return DBNanoTime{Time: time.Now()}
// }

// type DBNanoTime struct {
// 	time.Time
// }

// func (t DBNanoTime) Value() (driver.Value, error) {
// 	if t.IsZero() {
// 		return nil, nil
// 	}
// 	return driver.Value(t.UnixNano()), nil
// }

// func (t *DBNanoTime) Scan(value interface{}) error {
// 	if value == nil {
// 		t.Time = time.Time{}
// 		return nil
// 	}
// 	switch v := value.(type) {
// 	case int:
// 		t.Time = time.Unix(0, int64(v))
// 	case int64:
// 		t.Time = time.Unix(0, int64(v))
// 	case uint:
// 		t.Time = time.Unix(0, int64(v))
// 	case uint64:
// 		t.Time = time.Unix(0, int64(v))
// 	case float32:
// 		t.Time = time.Unix(0, int64(v))
// 	case float64:
// 		t.Time = time.Unix(0, int64(v))
// 	case string:
// 		var n int64
// 		_, err := fmt.Sscanf(v, "%d", &n)
// 		if err != nil {
// 			return err
// 		}
// 		t.Time = time.Unix(0, n)
// 	case []byte:
// 		var n int64
// 		_, err := fmt.Sscanf(string(v), "%d", &n)
// 		if err != nil {
// 			return err
// 		}
// 		t.Time = time.Unix(0, n)
// 	default:
// 		return fmt.Errorf("DBNanoTime.Scan: unknown type %T", value)
// 	}

// 	return nil
// }

func NewDBTime() DBTime {
	return DBTime{Time: time.Now()}
}

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

// TimeTester uses both DBTime and DBNanoTime
type TimeTester struct {
	TimeTesterID int64  `db:"time_tester_id" tmeta:"pk,auto_incr"`
	Name         string `db:"name"`
	CreateTime   DBTime `db:"create_time"`
	UpdateTime   DBTime `db:"update_time"`
}

func (tt *TimeTester) CreateTimeTouch() { tt.CreateTime = NewDBTime() }
func (tt *TimeTester) UpdateTimeTouch() { tt.UpdateTime = NewDBTime() }

func TestSQLite3(t *testing.T) {

	assert := assert.New(t)
	sess, meta, err := doSetup("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	_, err = sess.Exec(`
CREATE TABLE time_tester (
	time_tester_id INTEGER PRIMARY KEY AUTOINCREMENT,
	name VARCHAR(255),
	create_time TEXT,
	update_time TEXT
)`)
	assert.NoError(err)

	meta.MustParse(TimeTester{})

	b := New(sess, meta)

	timeTester := TimeTester{
		Name: "test1",
	}

	res, err := b.MustInsert(&timeTester).Exec()
	assert.NoError(b.ResultWithInsertID(&timeTester, res, err))

	var timeTester2 TimeTester
	assert.NoError(
		b.MustSelectByID(&timeTester2, timeTester.TimeTesterID).
			LoadOne(&timeTester2))

	t.Logf("timeTester2 = %#v", timeTester2)
	t.Logf("timeTester2 times (%v, %v)",
		timeTester2.CreateTime,
		timeTester2.UpdateTime,
	)

}

func TestMySQL(t *testing.T) {
	if mysqlConnStr == "" {
		t.SkipNow()
	}

	assert := assert.New(t)
	sess, meta, err := doSetup("sqlite3")
	if err != nil {
		t.Fatal(err)
	}

	_, err = sess.Exec(`
CREATE TABLE time_tester (
	time_tester_id INTEGER PRIMARY KEY AUTOINCREMENT,
	name VARCHAR(255),
	create_time DATETIME(6),
	update_time DATETIME(6)
)`)
	assert.NoError(err)

	meta.MustParse(TimeTester{})

	b := New(sess, meta)

	timeTester := TimeTester{
		Name: "test1",
	}

	res, err := b.MustInsert(&timeTester).Exec()
	assert.NoError(b.ResultWithInsertID(&timeTester, res, err))

	var timeTester2 TimeTester
	assert.NoError(
		b.MustSelectByID(&timeTester2, timeTester.TimeTesterID).
			LoadOne(&timeTester2))

	t.Logf("timeTester2 = %#v", timeTester2)
	t.Logf("timeTester2 times (%v, %v)",
		timeTester2.CreateTime,
		timeTester2.UpdateTime,
	)

	t.Logf("TODO: additional MySQL-specific testing")
}

func TestPostgres(t *testing.T) {
	if postgresConnStr == "" {
		t.SkipNow()
	}
	t.Logf("TODO: Postgres-specific testing")
}
