# Minimalistic Idiomatic Database "ORM" Functionality for Go

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
- We don't do DDL, this makes things simpler.  (DDL is necessary but should be separate.)
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

// initialize a session
sess := conn.NewSession(nil)

// use a Builder to construct queries
b := tmetadbr.New(sess, meta)

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

## Table Info

`tmeta` understands basic properties about your tables such as SQL table name, the primary key(s), and the list of fields.  These things can be easily set up once and then don't have to be repated in your code, resulting in code that is safer, more resilient to change, still simple and generally just easier to maintain.

```golang
type Widget struct {
	WidgetID string `db:"widget_id" tmeta:"pk"`
	Name     string `db:"name"`
}

// a Meta instance holds info for a group of tables (usually all your tables)
meta := tmeta.NewMeta()

// parse the struct tags and add the table to your Meta instance
// note that pointers are automatically dereferenced throughout, so for the purpose
// of table metadata a Widget and *Widget are treated the same
err := meta.Parse(Widget{})

// fetch the table name
tableName := meta.For(Widget{}).SQLName()

// you can also set the SQL table name, useful for prefixing tables
meta.For(Widget{}).SetSQLName("demoapp_widget")

// code can fetch table info either by struct type
widgetTI := meta.For(Widget{})
// or by logical name
widgetTI = meta.ForName("widget")

// get a slice of primary key SQL field names (tagged with `tmeta:"pk"`)
pkFieldSlice := widgetTI.SQLPKFields()

// get a slice of all of the SQL fields, including the primary keys
fieldSlice := widgetTI.SQLFields(true)

// or without the pks
fieldSlice = widgetTI.SQLFields(false)
```

Have a look at the [TableInfo](https://godoc.org/github.com/gocaveman/tmeta#TableInfo) godoc for a full list of what's available.

This functionality from `tmeta` is what is used by `tmetadbr` to implement higher level query building.

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
err = b.ExecOK(b.MustDeleteRelationNotIn(&book, "category_id_list"))
// and then insert (with ignore) the set
err = b.ExecOK(b.MustInsertRelationIgnore(&book, "category_id_list"))

// and you load it like any other relation
book.CategoryIDList = nil
_, err = b.MustSelectRelation(&book, "category_id_list").
	Load(bookT.RelationTargetPtr(&book, "category_id_list"))
```

### Relation Targets

When selecting relations, the query builder needs the object to inspect and the name of the relation.  However, the call to actually execute the select statement also needs to know the "target" field to load into.

The examples above use `SelectRelation` and `RelationTargetPtr` pointer to do this.  This allows you to dynamically load a relation by only knowing it's name.  If you are hard coding for a specific relation you can pass a pointer to the field itself.  `SelectRelationPtr` is also available and returns the pointer when the query is built.  Pick your poison.

## Create and Update Timestamps, and Dates in General

Create and update timestamps will be updated at the appropriate time with a simple call to the corresponding method on your struct, if it exists:

```golang
// called on insert
func (w *Widget) CreateTimeTouch() { ... }

// called on update
func (w *Widget) UpdateTimeTouch() { ... }
```

That part is easy.

To get date-time information to work correctly on multiple databases/drivers and make use of Go's `time.Time`, marshal to/from JSON correctly, have subsecond precision, is human readable, and avoid time zone confusion, it is useful to make a custom type that deals with these concerns.

In SQLite3 use `TEXT` as the column type, in Postgres use `DATETIME` and in MySQL use `DATETIME(6)` (requires 5.6.4 or above for subsecond precision).

Here is a recommendation on a type that accomplishes this:

```golang
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
```

You can then use this on your model object for any timestamps you need, including the create and update time:

```golang
type Widget struct {
	// ...
	CreateTime   DBTime `db:"create_time"`
	UpdateTime   DBTime `db:"update_time"`
	// ...
}

func (w *Widget) CreateTimeTouch() { w.CreateTime = NewDBTime() }
func (w *Widget) UpdateTimeTouch() { w.UpdateTime = NewDBTime() }
```

## Optimistic Locking

Optimistic locking means there is a version field on your table and when you perform an update it checks that the version did not change since you selected it earlier.

```golang
type Widget struct {
	WidgetID string `db:"widget_id" tmeta:"pk"`
	Name     string `db:"name"`
	Version  int    `db:"version" tmeta:"version"`
}
```

UpdateByID will generate SQL that checks for the previous version and increments to the next number.  If zero rows are matched, you know the record has been modified since (or deleted).  In this case, the correct thing to do is inform the original caller of the problem so the user can fix it (by refreshing the page, etc.)

ResultWithOneUpdate makes this simple.  Example:

```golang
err = b.ResultWithOneUpdate(b.MustUpdateByID(&theRecord).Exec())
```

In this case `theRecord` was read earlier, some fields were modified and it's being updated now.  If not exactly one record was updated, `ErrUpdateFailed` will be returned.  You should not increment the version number, `UpdateByID` will do that for you (i.e. if you read version 6 from the db, you pass that 6 back into `UpdateByID` and it will do `UPDATE ... SET ... version = 7 ... WHERE ... version = 6 ...`)

## Convience Methods - Must..., Result... and Exec...

Some convenience methods are included on [Builder](https://godoc.org/github.com/gocaveman/tmeta/tmetadbr#Builder) which should reduce unneeded checks for common cases.

- Methods starting with `Must` will panic instead of returning an error, but note that this is only on the query building itself, not on query execution.  Panics should only occur if you give it wrong type information or have incorrect struct tags.  So the panic cases will be due to developer error, not runtime environment or user input, so using `Must` and panicing in this case can be an acceptable tradeoff for the convenience it provides (being able to chain more stuff in a single expression).
- `ResultWithOneUpdate` will check that your query returned exactly one row.
- `ResultWithInsertID` will populate the ID that was inserted into the primary key of your struct.
- `ResultOK` will ignore the result and only return the error (provided for convenient method chaining).
- `ExecOK` will run `Exec`, discard the sql.Result and just return the error.
- `ExecContextOK` is like `ExecOK` but accepts a context.Context also.

## Primary Keys (String/UUIDs or Auto-Increment)

Primary can be strings or auto-increment integers.  We recommend using string UUIDs.  The package [gouuidv6](https://github.com/bradleypeabody/gouuidv6) provides a way to make IDs that are globally unique, sort by creation time and are relatively short.  UUIDs are more resilient to architectural changes in long-lived projects (sharding, database synchronization problems, clustered servers, etc.)

```golang
type Widget struct {
	WidgetID string `db:"widget_id" tmeta:"pk"` // string/UUID primary key
}
```

Auto-increment works just fine as well.

```golang
type Widget struct {
	WidgetID int64 `db:"widget_id" tmeta:"pk,auto_incr"` // auto-increment primary key (db table must be set to provide key values, e.g. "AUTO INCREMENT")
}
```

Multiple primary keys are supported (and necessary for join tables).

```golang
type WidgetCategory struct {
	// both widget_id and category_id are the combined primary key
	WidgetID   string `db:"widget_id" tmeta:"pk"`
	CategoryID string `db:"category_id" tmeta:"pk"`
}
```

The `IDAssigner` interface can be implemented to allow a struct to assign itself a new ID when needed.  `Insert` (and `MustInsert`) will look for an `IDAssign` method and call it if present before performing an insertion.

```golang
func (w *Widget) IDAssign() {
	if w.WidgetID == "" {
		w.WidgetID = gouuidv6.NewB64().String()
	}
}
```

## Recommended Use and "Stores"

Generally `tmeta` doesn't care how it is called.  If you want, you can use it directly in your HTTP handlers to build and run queries.  However, we recommend you construct a "store" that acts as a data access layer on top of your database tables and house the query construction and execution in there.  Your store would accept and return pointers to "model" objects (a struct that corresponds to your table(s)).

This is roughly analogous to the Data Access Object design pattern, although the point is to organize your code so queries can be maintained with in a specific section of the code, not to zealously adhere to this pattern.

Store methods would contain basic CRUD operations, plus any other more complex cases.  Anything that requires it's own transaction goes as a method on the store.  These days, store methods should also accept and use a `context.Context`, this allows cancellation at a higher layer (for example the HTTP client closed the connection) to cause pending database queries to be cancelled.  Fancy.

Your HTTP handlers/controllers, etc. can then use this store to access things.

Here's an example:

```golang
type Store struct {
	*tmeta.Meta
	*dbr.Connection
}

func (s *Store) CreateWidget(ctx context.Context, o *Widget) error {
	tx, err := s.Connection.NewSession(nil).BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.RollbackUnlessCommitted()

	b := tmetadbr.New(tx, s.Meta)

	o.WidgetID = gouuidv6.NewB64().String() // however you want to create your IDs

	err = b.ResultWithOneUpdate(b.MustInsert(o).ExecContext(ctx))
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ...

```

Often there is only one `tmeta.Meta` in your application, but in large apps you can have sections of tables that only need to be aware of each other, in this case just make a new one (`tmeta.NewMeta()`) for each, and each store would have one.

## Useful Relational Patterns

Some fancy things you can do with your relations include:

### Eager Loading

"Eager loading" is a feature of some ORMs, and we can achieve equivalent (less magical) functionality in our Store by simply adding `SelectRelation` calls in the appropriate "read" method:

```golang
func (s *Store) FindWidgetByID(ctx context.Context, widgetID string) (*Widget, error) {
	tx, err := s.Connection.NewSession(nil).BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.RollbackUnlessCommitted()

	b := tmetadbr.New(tx, s.Meta)

	// load the widget
	var widget Widget
	err = b.MustSelectByID(&widget, widgetID).LoadOneContext(ctx, &widget)
	if err != nil {
		return err
	}

	// eager load the "category_list" relation
	_, err = b.MustSelectRelation(&widget, "category_list").
		Load(s.Meta.For(widget).RelationTargetPtr(&widget, "category_list"))
	if err != nil {
		return err
	}

	return tx.Commit()
}
```

### Loading Relations Dynamically

It can be useful to allow other layers to request one or more relations by name.  The store can then load these relations as requested and can also easily implement aliases for useful variations.  The names should be filtered to avoid callers requesting too much data.  Example:

```golang
func (s *Store) LoadWidgetRelations(ctx context.Context, widget *Widget, relationNames ...string) error {
	tx, err := s.Connection.NewSession(nil).BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.RollbackUnlessCommitted()

	b := tmetadbr.New(tx, s.Meta)

	for _, rn := range relationNames {
		switch rn {
			// these relations we just load as-is
		case "category_list", "category_id_list":
			_, err = b.MustSelectRelation(widget, rn).
				Load(s.Meta.For(widget).RelationTargetPtr(widget, rn))
			if err != nil {
				return err
			}

			// it can be useful to provide more magical names with specific queries like this
		case "change_list_last10":
			_, err = b.MustSelectRelation(widget, "change_list").
				Where("change_type = ?", "normal").
				OrderDesc("create_time").
				Limit(10).
				Load(s.Meta.For(widget).RelationTargetPtr(widget, "change_list"))
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unkown (or disallowed) relation %q", rn)
		}
	}

	return tx.Commit()
}
```

### Saving "Belongs to Many IDs" Join Tables

Another common relational pattern is to have a join table that needs to be updated to "match this set of IDs".  If a Widget has a many-to-many join to Category (using a join table), you could easily synchronize the IDs in your update method like so:

```golang
func (s *Store) UpdateWidget(ctx context.Context, widget *Widget) error {
	tx, err := s.Connection.NewSession(nil).BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.RollbackUnlessCommitted()

	b := tmetadbr.New(tx, s.Meta)

	// update widget record
	err = b.ResultWithOneUpdate(b.MustUpdateByID(widget).Exec())
	if err != nil {
		return err
	}

	// to synchronize we first delete any not in the new set
	err = b.ExecOK(b.MustDeleteRelationNotIn(widget, "category_id_list"))
	if err != nil {
		return err
	}
	// and then insert (with ignore) the set
	err = b.ExecOK(b.MustInsertRelationIgnore(widget, "category_id_list"))
	if err != nil {
		return err
	}

	return tx.Commit()
}
```

## Naming Conventions

As a general rule, you can set whatever specific names you want in tmeta.  The "Name" corresponding to a struct is by default it's snake-cased translation of the struct name.  So "WidgetFactory" has a "Name" of "widget_factory".  The "SQLName" is the name of the table in the database, and by default it is the same as Name, but is easily changable.  Any time you reference a table in your code however you should do so using it's Name, and then you can SQLName() to get the actual table name.

The convention encouraged by tmeta is to avoid pluralization pretty much whenever possible.  Translating "Category" into "Categories" and taking into account variations in how pluralization is done in English and in other languages can be non-trivial, is not positive (error prone) and has little benefit.  It's way better to just say "Category" everywhere.

For fields that are a list, the suggested approach is to append "List".  So you get "CategoryList".

This convention makes it a lot easier to match things up, because they have the same exact name everywhere.

Also, rather than having tables with just an "id" column tmeta encourages prefixing the identifier with the logical name of the object, e.g. a "Widget" struct has the name "widget" and it's primary key is "widget_id".  This makes it so that anywhere a widget is referenced it is clearly an identfier for that type.  Otherwise everytime you look at an "id" field you have to figure out what is being identified.

## Motivation

- Remove or at least minimize fields, table names and other SQL-specific data from store code wherever it's duplicative (reduce maintenance)
- Express relations and provide a means to read and write them
- Allow relations to be expressed on your model objects so they can used in other cases, e.g. it is convenient and productive to express a one-to-many relationship as a field with a slice of structs and this can be serialized using JSON or other mechanism to good effect
- Keep good separation between database query execution and table metadata - we don't execute queries directly, or do ddl, or interfere with your model objects.

## Status

This package is currently on version 0, so there is no official guarantee of API compatibility.  However, quite a bit of thought was put into how the API is organized and breaking changes will not be made lightly. v0.x.y tags will be made as changes are done and can be used with your package versioning tool of choice.  After sufficient experience and feedback is gathered, a v1 release will be made.

## TODO

- Make variable names in doc more descriptive.
- Consider adding a struct tag to express a "field that should be inserted but not updated" (useful for create_time, for example), see if there are any others like this and what API additions this would mean
- More testing on auto increment, optimistic locking (versions), MySQL, Postgres
- Repo tag
