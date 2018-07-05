package tmetadbr

import (
	"fmt"
	"math/rand"

	"github.com/gocaveman/tmeta"
	"github.com/gocraft/dbr"

	_ "github.com/mattn/go-sqlite3"
)

type Author struct {
	AuthorID   string `db:"author_id" tmeta:"pk"`
	NomDePlume string `db:"nom_de_plume"`

	BookList []Book `db:"-" tmeta:"has_many"`
}

type Publisher struct {
	PublisherID string `db:"publisher_id" tmeta:"pk"`
	CompanyName string `db:"company_name"`
	Version     int64  `db:"version" tmeta:"version"`

	BookList []Book `db:"-" tmeta:"has_many,relation_name=book_list"`
}

type Book struct {
	BookID string `db:"book_id" tmeta:"pk"`

	AuthorID string  `db:"author_id"`
	Author   *Author `db:"-" tmeta:"belongs_to,sql_id_field=author_id"`

	PublisherID string     `db:"publisher_id"`
	Publisher   *Publisher `db:"-" tmeta:"belongs_to"`

	Title string `db:"title"`

	CategoryList []Category `db:"-" tmeta:"belongs_to_many,join_name=book_category"`

	CategoryIDList []string `db:"-" tmeta:"belongs_to_many_ids,join_name=book_category"`
}

type BookCategory struct {
	BookID     string `db:"book_id" tmeta:"pk"`
	CategoryID string `db:"category_id" tmeta:"pk"`
}

type Category struct {
	CategoryID   string        `db:"category_id" tmeta:"pk"`
	Name         string        `db:"name"`
	BookList     []Book        `db:"-" tmeta:"belongs_to_many,join_name=book_category"`
	CategoryInfo *CategoryInfo `db:"-" tmeta:"has_one"`
}

type CategoryInfo struct {
	CategoryInfoID int64     `db:"category_info_id" tmeta:"pk,auto_incr"`
	CategoryID     string    `db:"category_id"`
	InfoStuff      string    `db:"info_stuff"`
	Category       *Category `db:"-" tmeta:"belongs_to"`
}

func doSetup(driver string) (*dbr.Session, *tmeta.Meta, error) {

	var conn *dbr.Connection
	var err error

	switch driver {
	case "sqlite3":
		conn, err = dbr.Open("sqlite3", fmt.Sprintf(`file:tmeta_test%d?mode=memory&cache=shared`, rand.Int31()), nil)
	case "mysql":
		conn, err = dbr.Open("mysql", mysqlConnStr, nil)
	case "postgres":
		conn, err = dbr.Open("postgres", postgresConnStr, nil)
	}
	if err != nil {
		return nil, nil, err
	}

	sess := conn.NewSession(newPrintEventReceiver(nil))

	_, err = sess.Exec(`
CREATE TABLE test_author (
	author_id VARCHAR(64),
	nom_de_plume VARCHAR(255),
	PRIMARY KEY(author_id)
)`)
	if err != nil {
		return nil, nil, err
	}

	_, err = sess.Exec(`
CREATE TABLE test_publisher (
	publisher_id VARCHAR(64),
	company_name VARCHAR(255),
	version INTEGER NOT NULL,
	PRIMARY KEY(publisher_id)
)`)
	if err != nil {
		return nil, nil, err
	}

	_, err = sess.Exec(`
CREATE TABLE test_book (
	book_id VARCHAR(64),
	author_id VARCHAR(64),
	publisher_id VARCHAR(64),
	title VARCHAR(255),
	PRIMARY KEY(book_id)
)`)
	if err != nil {
		return nil, nil, err
	}

	_, err = sess.Exec(`
CREATE TABLE test_book_category (
	book_id VARCHAR(64),
	category_id VARCHAR(64),
	PRIMARY KEY(book_id, category_id)
)`)
	if err != nil {
		return nil, nil, err
	}

	_, err = sess.Exec(`
CREATE TABLE test_category (
	category_id VARCHAR(64),
	name VARCHAR(255),
	PRIMARY KEY(category_id)
)`)
	if err != nil {
		return nil, nil, err
	}

	_, err = sess.Exec(`
CREATE TABLE test_category_info (
	category_info_id INTEGER PRIMARY KEY AUTOINCREMENT,
	category_id VARCHAR(64),
	info_stuff VARCHAR(255)
)`)
	if err != nil {
		return nil, nil, err
	}

	meta := tmeta.NewMeta()
	err = meta.Parse(&Author{})
	if err != nil {
		return nil, nil, err
	}
	err = meta.Parse(&Publisher{})
	if err != nil {
		return nil, nil, err
	}
	err = meta.Parse(&Book{})
	if err != nil {
		return nil, nil, err
	}
	err = meta.Parse(&BookCategory{})
	if err != nil {
		return nil, nil, err
	}
	err = meta.Parse(&Category{})
	if err != nil {
		return nil, nil, err
	}
	err = meta.Parse(&CategoryInfo{})
	if err != nil {
		return nil, nil, err
	}
	meta.ReplaceSQLNames(func(name string) string { return "test_" + name })

	return sess, meta, nil
}
