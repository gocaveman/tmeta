package tmeta

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRelationParse(t *testing.T) {

	assert := assert.New(t)

	sess, meta, err := doSetup()
	assert.NoError(err)
	defer sess.Connection.Close()

	bookT := meta.For(&Book{})
	authorRel := bookT.RelationNamed("author")
	assert.NotNil(authorRel)

}

func TestCRUD(t *testing.T) {

	assert := assert.New(t)

	sess, meta, err := doSetup()
	assert.NoError(err)
	defer sess.Connection.Close()

	authorT := meta.For(&Author{})
	assert.Len(authorT.SQLPKFields(), 1)

	author := Author{
		AuthorID:   "author_0001",
		NomDePlume: "Mark Twain",
	}

	// create
	_, err = sess.InsertInto(authorT.SQLName()).
		Columns(authorT.SQLFields(true)...).
		Record(&author).
		Exec()
	assert.NoError(err)

	// read
	var author2 Author
	// t.Logf("where: %s; values=%+v", authorT.SQLPKWhere(), authorT.PKValues(author2))
	err = sess.Select(authorT.SQLFields(true)...).
		From(authorT.SQLName()).
		Where(authorT.SQLPKWhere(), authorT.PKValues(author)...).
		LoadOne(&author2)
	assert.NoError(err)
	assert.Equal("author_0001", author2.AuthorID)
	assert.Equal("Mark Twain", author2.NomDePlume)
	// tmetadbr.SelectByID(sess, authorT, author).LoadOne(&author2)

	// update
	author.NomDePlume = "Samuel Langhorne Clemens"
	_, err = sess.Update(authorT.SQLName()).
		SetMap(authorT.SQLValueMap(author, false)).
		Where(authorT.SQLPKWhere(), authorT.PKValues(author)...).
		Exec()
	assert.NoError(err)

	// read it back and check
	err = sess.Select(authorT.SQLFields(true)...).
		From(authorT.SQLName()).
		Where(authorT.SQLPKWhere(), authorT.PKValues(author)...).
		LoadOne(&author2)
	assert.NoError(err)
	assert.Equal("Samuel Langhorne Clemens", author2.NomDePlume)

	// delete
	_, err = sess.DeleteFrom(authorT.SQLName()).
		Where(authorT.SQLPKWhere(), authorT.PKValues(author)...).
		Exec()
	assert.NoError(err)

}

// "ATTACHING"
// SYNCING JOIN TABLE IDS
// LOADING NAMED RELATIONS (WITH WHERE...)

// func TestHasMany(t *testing.T) {

// 	assert := assert.New(t)

// 	sess, meta, err := doSetup()
// 	assert.NoError(err)
// 	defer sess.Connection.Close()

// 	_, _ = sess, meta

// 	// var author Author
// 	// for _, relName := range []string{"books"} {
// 	// 	rel := meta.For(Author{}).Relation(relName)
// 	// 	ti := rel.TableInfo()
// 	// 	_, err := sess.Select(ti.SQLFields()...).
// 	// 		From(ti.SQLName()).
// 	// 		Where(rel.SQLTargetWhere(&author), author.AuthorID).
// 	// 		Load(rel.TargetPtr(&author))
// 	// 	if err != nil {
// 	// 		t.Fatal(err)
// 	// 	}
// 	// }

// 	// has_many:
// 	// SELECT [book.*] FROM book WHERE author_id = ?

// 	// belongs_to:
// 	// SELECT [author.*] FROM author WHERE book_id = ?

// 	// has_one:
// 	// SELECT [category_info.*] FROM category_info WHERE category_id = ?

// 	// belongs_to_many:
// 	// SELECT [test_book.*] FROM test_book JOIN test_book_category ON test_book.book_id = test_book_category.book_id WHERE category_id = ?

// 	// belongs_to_many_ids:
// 	// SELECT book_id FROM book_category WHERE category_id = ?

// 	// polymorphic variations:
// 	// SELECT link_id FROM category_links WHERE category_id = ? AND link_type = 'category'

// 	authorT := meta.For(Author{})
// 	bookT := meta.For(Book{})
// 	bookCategoryT := meta.For(BookCategory{})
// 	categoryT := meta.For(Category{})
// 	publisherT := meta.For(Publisher{})

// 	_, err = sess.InsertInto(authorT.SQLName()).
// 		Columns(authorT.SQLFields(true)...).
// 		Record(&Author{
// 			AuthorID:   "author_0001",
// 			NomDePlume: "Victor Hugo",
// 		}).
// 		Exec()
// 	assert.NoError(err)

// 	_, err = sess.InsertInto(publisherT.SQLName()).
// 		Columns(publisherT.SQLFields(true)...).
// 		Record(&Publisher{
// 			PublisherID: "author_0001",
// 			CompanyName: "Carleton Publishing Company",
// 		}).
// 		Exec()
// 	assert.NoError(err)

// 	_, err = sess.InsertInto(categoryT.SQLName()).
// 		Columns(categoryT.SQLFields(true)...).
// 		Record(&Category{
// 			CategoryID: "category_0001",
// 			Name:       "Historical Fiction",
// 		}).
// 		Exec()
// 	assert.NoError(err)

// 	_, err = sess.InsertInto(bookT.SQLName()).
// 		Columns(bookT.SQLFields(true)...).
// 		Record(&Book{
// 			BookID:      "book_0001",
// 			Title:       "Les Misérables",
// 			PublisherID: "publisher_0001",
// 			AuthorID:    "author_0001",
// 		}).
// 		Exec()
// 	assert.NoError(err)

// 	log.Printf("fields: %v", bookCategoryT.SQLFields(true))
// 	_, err = sess.InsertInto(bookCategoryT.SQLName()).
// 		Columns(bookCategoryT.SQLFields(true)...).
// 		Record(&BookCategory{
// 			CategoryID: "category_0001",
// 			BookID:     "book_0001",
// 		}).
// 		Exec()
// 	assert.NoError(err)

// 	var bookList []Book
// 	_, err = sess.
// 		Select("test_book.book_id").
// 		From(bookT.SQLName()).
// 		Join(bookCategoryT.SQLName(), "test_book.book_id = test_book_category.book_id").
// 		Where("category_id = ?", "category_0001").
// 		Load(&bookList)
// 	assert.NoError(err)
// 	log.Printf("result1: %+v", bookList)

// 	bookList = nil
// 	_, err = sess.
// 		Select(bookT.SQLFields(true)...).
// 		From(bookT.SQLName()).
// 		Where("book_id IN (SELECT book_id FROM test_book_category WHERE category_id = ?)", "category_0001").
// 		Load(&bookList)
// 	assert.NoError(err)
// 	log.Printf("result2: %+v", bookList)

// }

// TODO
// auto increment ids
// optimistic locking
