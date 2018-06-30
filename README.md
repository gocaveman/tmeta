# Minimalistic Idiomatic Database "ORM" functionality for Go

## Features
- Structs have SQL table information associated with them - tmeta knows that your `WidgetFactory` struct corresponds to the "widget_factory" table (names configurable, of course).
- Useful struct tags to express things like primary keys and relations.  Sensible defaults but easily configurable.
- Relation support (belongs to, has many, has one, belongs to many, belongs to many IDs)
- Builds queries using [dbr](https://github.com/gocraft/dbr), simple and flexible, avoids the cruft of more complex solutions.  Supports common operations like CRUD, loading relations, and maintaining join tables.
- Operations are succinct and explicit, not too magical, we're not trying to be Hiberate (or GORM for that matter); this is Go.
- Does not require or expect you to embed a special "Model" type, your structs remain simple, no additional dependencies in your model.  (E.g. should not interfere with existing lightweight database packages like [sqlx](https://github.com/jmoiron/sqlx))
- Optimistic locking (version column)
- Date Created/Updated functionality
- Normal underlying DB features like transactions and context support are not hidden from you and easily usable.
- Primary keys can be string/UUID (recommended) or auto-incremented integer.
- We don't do DDL, yes this is a feature.
- Supports SQLite3, MySQL, Postgres

## GoDoc
- tmeta (table/type information) https://godoc.org/github.com/gocaveman/tmeta
- tmetadbr (query building) https://godoc.org/github.com/gocaveman/tmeta/tmetadbr

## Basic CRUD

The `tmeta` package understands your types and `tmetadbr` provides query building using [dbr](https://github.com/gocraft/dbr).

To get set up and concisely run your queries:

```golang
type Widget struct {
	WidgetID string `db:"widget_id" tmeta:"pk"`
	Name     string `db:"name"`
}

// register things
meta := tmeta.NewMeta()
err := meta.Parse(Widget{})

// database connection with dbr
conn, err := dbr.Open(...)

// use a Builder to construct queries
b := tmetadbr.New(conn, meta)

// don't get scared by the "Must" in front of these functions, it applies to the
// query building steps, not to the queries themselves

// create
widgetID := gouuidv6.NewB64().String() // use what you want for an ID, my personal favorite: github.com/bradleypeabody/gouuidv6
_, err = b.MustInsert(&Widget{WidgetID: widgetID,Name: "Widget A"}).Exec()

// read multiple
var widgetList []Widget
// notice you can modify the queries before they are executed - where, limit, order by, etc.
_, err = b.MustSelect(&widgetList).Where("name LIKE ?", `Widget%`).Load(&widgetList)

// read one
var widget Widget
err = b.MustSelectByID(&widget, widgetID).LoadOne(&widget)

// update
widget.Name = "Widget A1"
_, err = b.MustUpdateByID(&widget).Exec()

// delete
_, err = b.MustDeleteByID(Widget{}, widgetID).Exec()
```

Variations of these methods without `Must` are available if you want type errors to be returned instead of panicing.

## Relations

With `tmetadbr` you can also easily generate the SQL to load related records.

### Has Many

A "has many" relation is used by one table where a second one has an ID that points back to it.

```golang
type Author struct {
	AuthorID   string `db:"author_id" tmeta:"pk"`
	NomDePlume string `db:"nom_de_plume"`
	BookList   []Book `db:"-" tmeta:"has_many"`
}

type Book struct {
	BookID   string `db:"book_id" tmeta:"pk"`
	AuthorID string `db:"author_id"`
	Title    string `db:"title"`
}


authorT := b.For(Author{})

_, err = b.MustSelectRelation(&author, "book_list").
	Load(authorT.RelationTargetPtr(&author, "book_list"))

// or you can load directly into the field, less dynamic but shorter and more clear
_, err = b.MustSelectRelation(&author, "book_list").Load(&author.BookList)
```

### Has One

A "has_one" relation is like a "has_many" but is used for a one-to-one relation.

```golang
type Category struct {
	CategoryID   string        `db:"category_id" tmeta:"pk"`
	Name         string        `db:"name"`
	CategoryInfo *CategoryInfo `db:"-" tmeta:"has_one"`
}

type CategoryInfo struct {
	CategoryInfoID string `db:"category_info_id" tmeta:"pk"`
	CategoryID     string `db:"category_id"`
}

categoryT := b.For(Category{})
err = b.MustSelectRelation(&category, "category_info").
	LoadOne(categoryT.RelationTargetPtr(&category, "category_info")))

```

### Belongs To

A "belongs_to" relation is the inverse of a "has_many", it is used when the ID is on the same table as the field being loaded.

```golang
type Author struct {
	AuthorID   string `db:"author_id" tmeta:"pk"`
	NomDePlume string `db:"nom_de_plume"`
}

type Book struct {
	BookID   string  `db:"book_id" tmeta:"pk"`
	AuthorID string  `db:"author_id"`
	Author   *Author `db:"-" tmeta:"belongs_to"`
	Title    string  `db:"title"`
}

bookT := b.For(Book{})
assert.NoError(b.MustSelectRelation(&book, "author").
	LoadOne(bookT.RelationTargetPtr(&book, "author")))
```

### Belongs To Many

A "belongs_to_many" relation is used for a many-to-many relation, using a join table.

```golang
type Book struct {
	BookID string           `db:"book_id" tmeta:"pk"`
	Title string            `db:"title"`
	CategoryList []Category `db:"-" tmeta:"belongs_to_many,join_name=book_category"`
}

// BookCategory is the join table
type BookCategory struct {
	BookID     string `db:"book_id" tmeta:"pk"`
	CategoryID string `db:"category_id" tmeta:"pk"`
}

type Category struct {
	CategoryID string `db:"category_id" tmeta:"pk"`
	Name       string `db:"name"`
}

_, err = b.MustSelectRelation(&book, "category_list").
	Load(bookT.RelationTargetPtr(&book, "category_list"))
```

### Belongs To Many IDs

A "belongs_to_many_ids" relation is similar to a "belongs_to_many" but instead of the field being a slice of structs, it is a slice of IDs, and methods are provided to easily synchronize the join table.

```golang
type Book struct {
	BookID string           `db:"book_id" tmeta:"pk"`
	Title string            `db:"title"`
	CategoryIDList []string `db:"-" tmeta:"belongs_to_many_ids,join_name=book_category"`
}

// BookCategory is the join table
type BookCategory struct {
	BookID     string `db:"book_id" tmeta:"pk"`
	CategoryID string `db:"category_id" tmeta:"pk"`
}

type Category struct {
	CategoryID string `db:"category_id" tmeta:"pk"`
	Name       string `db:"name"`
}

// set the list of IDs in the join
book.CategoryIDList = []string{"category_0001","category_0002"}
// to synchronize we first delete any not in the new set
err = b.ExecOK(b.MustDeleteRelationNotIn(&book, "category_id_list")))
// and then insert (with ignore) the set
err = b.ExecOK(b.MustInsertRelationIgnore(&book, "category_id_list")))

// and you load it like any other relation
book.CategoryIDList = nil
_, err = b.MustSelectRelation(&book, "category_id_list").
	Load(bookT.RelationTargetPtr(&book, "category_id_list"))
```

### Relation Targets

When selecting relations, the query builder needs the object to inspect and the name of the relation.  However, the call to actually execute the select statement also needs to know the "target" field to load into.

The examples above use `SelectRelation` and `RelationTargetPtr` pointer to do this.  This allows you to dynamically load a relation by only knowing it's name.  If you are hard coding for a specific relation you can pass a pointer to the field itself.  `SelectRelationPtr` is also available and returns the pointer when the query is built.  Pick your poison.

## Create and Update Timestamps

Create and update timestamps will be updated at the appropriate time with a simple call to the appropriate method on your struct, if it exists:

```
// called on insert
func (w *Widget) CreateTimeTouch() { ... }

// called on update
func (w *Widget) UpdateTimeTouch() { ... }
```

TODO: Give an example of create and update time fields that work as expected in SQLite3, Postgres and MySQL, both the Go side and the DB column types.

## Optimistic Locking

Optimistic locking means there is a version field on your table and when you perform an update it checks that the version did not change since you selected it earlier.

UpdateByID will generate SQL that checks for the previous version and increments to the next number.  If zero rows are matched, you know the record has been modified since (or deleted).  In this case, the correct thing to do is inform the original caller of the problem so the user can fix it (by refreshing the page, etc.)

ResultWithOneUpdate makes this simple.  Example:

```golang
err = b.ResultWithOneUpdate(b.MustUpdateByID(&theRecord).Exec()))
```

In this case `theRecord` was read earlier, some fields were modified and it's being updated now.  If not exactly one record was updated, `ErrUpdateFailed` will be returned.

## Convience Methods - Must..., Result... and Exec...

Some convenience methods are included on Builder which should reduce unneeded checks for common cases.

- Methods starting with `Must` will panic instead of returning an error, but note that this is only on the query building itself, not on query execution.  Panics should only occur if you give it wrong type information or have incorrect struct tags.  So the panic cases will be due to developer error, not runtime environment or user input, so using `Must` and panicing in this case can be an acceptable tradeoff for the convenience it provides (being able to chain more stuff in a single expression).
- `ResultWithOneUpdate` will check that your query returned exactly one row.
- `ResultWithInsertID` will populate the ID that was inserted into the primary key of your struct.
- `ResultOK` will ignore the result and only return the error (provided for convenient method chaining).
- `ExecOK` will run `Exec`, discard the sql.Result and just return the error.
- `ExecContextOK` is like `ExecOK` but accepts a context.Context also.

## Primary Keys (String/UUIDs or Auto-Increment)

Primary can be strings or auto-increment integers.  We recommend using string UUIDs.  The package [gouuidv6](https://github.com/bradleypeabody/gouuidv6) provides a way to make IDs that are globally unique, sort by creation time and are relatively short.  UUIDs are more resilient architectural changes in long-lived projects (sharding, database synchronization problems, clustered servers, etc.)

Auto-increment works just fine as well.

Multiple primary keys are supported (and necessary for join tables).  Not well tested on non-join tables, buyer beware.

## Recommended Use and "Stores"

TODO: define 'store' and 'model', show example and each method uses a transaction, passes context; one store for entire application or section of application that has related tables (if too large); DefaultMeta is there for convience, but if you need (or might need as your project grows) multiple then tmetadbr.New() is the way to go

## Useful Relation Patterns

- LoadWidgetRelations(relationNames ...string); filter before doing, pass through the ones that are safe; custom names with special where clauses
- SaveJoinRelations(relationNames ...string)
- Eager loading

## Naming Conventions

TODO:  (you can override whatever you want, but some conventions keep things simple) (mention the TableInfo name as being separate from the actual SQL table name, although they are the same by default)
- no plurals in database field names, just use the singlar everywhere so it's the same; for relation names you can use "list" as a suffix, e.g. CategoryIDList
- id fields are the table name with `_id` at the end of it, i.e. "book_id", not just "id"; this means the same logical field has the same name in any table, and it can be derived easily

## Motivation

- Remove or at least minimize fields, table names and other SQL-specific data from store code wherever it's duplicative (reduce maintenance)
- Express relations and provide a means to read and write them
- Allow relations to be expressed on your model objects so they can used in other cases, e.g. it is convenient and productive to express a one-to-many relationship as a field with a slice of structs and this can be serialized using JSON or other mechanism to good effect
- Keep good separation between database query execution and table metadata - we don't execute queries directly, or do ddl, or interfere with your model objects.

## Status

This package is currently on version 0, so there is no official guarantee of API compatibility.  However, quite a bit of thought was put into how the API is organized and breaking changes will not be made lightly. v0.x.y tags will be made as changes are done and can be used with your package versioning tool of choice.  After sufficient experience and feedback is gathered, a v1 release will be made.

## TODO

- Write tests for auto increment
- Write tests for optimistic locking (versions)
- Write tests for MySQL-specifc stuff
- Write tests for Postgres-specifc stuff
- Tag it v0.1.0
