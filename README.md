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
- Supports SQLite3, MySQL, Postgres

## Basic CRUD

## Relations

(include snippets of the example structs as we go)

### Has Many

// LOADING NAMED RELATIONS (WITH WHERE...)

### Has One


### Belongs To

### Belongs To Many

### Belongs To Many IDs

// SYNCING JOIN TABLE IDS

### Relation Targets

(SelectRelationPtr vs SelectRelation and RelationTargetPtr)

## Create and Update Timestamps


## Optimistic Locking

(recommend conflict get pushed back up to the UI to be resolved by user - simple case refresh page, more complex case form can try to dynamically pull fresh record and merge - either way the idea is to prevent saves from clobbering each other)

## Convience Methods - Must..., Result... and Exec...

(errors during query building will only be due to wrong type or field information, so using the Must... variations can provide convenience, blah blah)  Basically, incorrect query building is generally going to be due to developer error, not user input, so using Must and panicing in this case can be an acceptable tradeoff for the convenience it provides.

## Primary Keys (String/UUIDs or Auto-Increment)
(consider just using v6 base64 UUIDs)

## Recommended Use and "Stores"

(define 'store' and 'model')
(each method uses a transaction, passes context)
(one store for entire application or section of application that has related tables (if too large))
(DefaultMeta is there for convience, but if you need (or might need as your project grows) multiple then tmetadbr.New() is the way to go)

## Useful Relation Patterns

- LoadWidgetRelations(relationNames ...string)
-- filter before doing, pass through the ones that are safe
-- custom names with special where clauses
- SaveJoinRelations(relationNames ...string)
- Eager loading


## Naming Conventions

(you can override whatever you want, but some conventions keep things simple)
(mention the TableInfo name as being separate from the actual SQL table name, although they are the same by default)
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

- Write at least a one-liner comment for each public function and type (tmeta and tmetadbr)
- Consider making a global DefaultMeta, Parse functions and MustParse - to follow HTTP package pattern of having a simple singleton use, plus the most sophisticated use; document this
- Complete README
- See if tmeta itself needs any more tests, or just cleanup

- Test auto increment
- Test optimistic locking (versions)
- Test MySQL-specifc stuff
- Test Postgres-specifc stuff
- Tag it v0.1.0
