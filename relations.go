package tmeta

import "reflect"

// Relation is implemented by the supported types of relations, BelongsTo, HasMany, etc.
type Relation interface {
	RelationName() string
	RelationGoValueField() string
}

// RelationMap is a map of named relations to the relation data itself.
// Values must of one of the supported relation types.
type RelationMap map[string]Relation

// RelationNamed is a simple map lookup.
func (rm RelationMap) RelationNamed(n string) Relation {
	return rm[n]
}

// RelationTargetPtr will find the named relation and use it's RelationGoValueField
// to obtain a pointer to the target field and return it.  If this is not possible
// then nil is returned.  If successful, the returned value will be a pointer to
// the type of the indicated field, e.g. if the field is of type `*Widget`, the
// return value will be of type `**Widget`, for type `[]Widget` it will
// return type `*[]Widget`.
func (rm RelationMap) RelationTargetPtr(o interface{}, n string) interface{} {
	r := rm[n]
	if r == nil {
		return nil
	}
	fn := r.RelationGoValueField()
	f := derefValue(reflect.ValueOf(o)).FieldByName(fn)
	return f.Addr().Interface()
}

// BelongsTo is a relation for a single struct pointer where the
// ID of the linked row is stored on this table.
//
// Example using struct tags:
//
//	type Book struct {
//		// ...
//
//		// This ID points to the row in the "author" table.
//		AuthorID string  `db:"author_id"`
//
//		// This is the "author" relation that can be loaded from it.
//		Author   *Author `db:"-" tmeta:"belongs_to"`
//	}
//
// Full form with all options:
//
//		// The sql_id_field here must match the db struct tag in the field above.
//		Author   *Author `db:"-" tmeta:"belongs_to,relation_name=author,sql_id_field=author_id"`
//
// No options are required except the relation type ("belongs_to").
type BelongsTo struct {
	Name         string
	GoValueField string // e.g. "Author" (of type *Author)
	SQLIDField   string // e.g. "author_id"
}

func (r *BelongsTo) RelationName() string {
	return r.Name
}
func (r *BelongsTo) RelationGoValueField() string {
	return r.GoValueField
}

// HasMany is a relation for a slice where the ID of the linked rows
// are stored on the other table.
//
// Example using struct tags:
//
//	type Publisher struct {
//		// ...
//		BookList []Book `db:"-" tmeta:"has_many"`
//	}
//
//	type Book {
//		// ...
//		PublisherID string `db:"publisher_id"`
//	}
//
// Full form with all options:
//
//		// The sql_other_id_field here must match the ID field in the other table.
//		BookList []Book `db:"-" tmeta:"has_many,relation_name=book_list,sql_other_id_field=publisher_id"`
//
// No options are required except the relation type ("has_many").
type HasMany struct {
	Name            string
	GoValueField    string // e.g. "Books" (of type []Book)
	SQLOtherIDField string // e.g. "author_id" - on the other table
}

func (r *HasMany) RelationName() string {
	return r.Name
}
func (r *HasMany) RelationGoValueField() string {
	return r.GoValueField
}

// HasOne is a relation for a slice where the ID of the linked rows
// are stored on the other table.
//
// Example using struct tags:
//
//	type Category struct {
//		// ...
//		CategoryInfo *CategoryInfo `db:"-" tmeta:"has_one"`
//	}
//
//	type CategoryInfo struct {
//		// ...
//		CategoryID     string    `db:"category_id"` // one to one relation
//	}
//
// Full form with all options:
//
//		CategoryInfo *CategoryInfo `db:"-" tmeta:"has_one,relation_name=category_info,sql_other_id_field=category_id"`
//
// No options are required except the relation type ("has_one").
type HasOne struct {
	Name            string
	GoValueField    string // e.g. "CategoryInfo" (of type *CategoryInfo)
	SQLOtherIDField string // e.g. "category_id" - on the other table
}

func (r *HasOne) RelationName() string {
	return r.Name
}
func (r *HasOne) RelationGoValueField() string {
	return r.GoValueField
}

// BelongsToMany is a relation that uses a join table as a many to many relation.
//
// Example using struct tags:
//
//	type Book struct {
//		// ...
//		CategoryList []Category `db:"-" tmeta:"belongs_to_many,join_name=book_category"`
//	}
//
//	// BookCategory is the join table.
//	type BookCategory struct {
//		BookID     string `db:"book_id" tmeta:"pk"`
//		CategoryID string `db:"category_id" tmeta:"pk"`
//	}
//
//	type Category struct {
//		CategoryID   string        `db:"category_id" tmeta:"pk"`
//		Name         string        `db:"name"`
//	}
//
// Full form with all options:
//
//		CategoryList []Category `db:"-" tmeta:"belongs_to_many,join_name=book_category,sql_id_field=book_id,sql_other_id_field=category_id"`
//
// The join_name option is required.
type BelongsToMany struct {
	Name            string
	GoValueField    string // e.g. "BookLists" (of type []Book)
	JoinName        string // the name of the join table (not necessarily the SQL name, it's Name()), e.g. ""
	SQLIDField      string // SQL ID field on join table corresponding to this side
	SQLOtherIDField string // SQL ID field on join table corresponding to the other side
}

func (r *BelongsToMany) RelationName() string {
	return r.Name
}
func (r *BelongsToMany) RelationGoValueField() string {
	return r.GoValueField
}

// BelongsToManyIDs is a relation that uses a join table as a many to many relation
// but stores the IDs in a slice instead of the instances directly.  Useful for
// easily updating the join table.
//
// Example using struct tags:
//
//	type Book struct {
//		// ...
//		CategoryIDList []string `db:"-" tmeta:"belongs_to_many_ids,join_name=book_category"`
//	}
//
//	// BookCategory is the join table.
//	type BookCategory struct {
//		BookID     string `db:"book_id" tmeta:"pk"`
//		CategoryID string `db:"category_id" tmeta:"pk"`
//	}
//
//	type Category struct {
//		CategoryID   string        `db:"category_id" tmeta:"pk"`
//		Name         string        `db:"name"`
//	}
//
// Full form with all options:
//
//		CategoryIDList []string `db:"-" tmeta:"belongs_to_many_ids,join_name=book_category,sql_id_field=book_id,sql_other_id_field=category_id"`
//
// The join_name option is required.
type BelongsToManyIDs struct {
	Name            string
	GoValueField    string // e.g. "BookIDList" (of type []string)
	JoinName        string // the name of the join table (not necessarily the SQL name, it's Name()), e.g. ""
	SQLIDField      string // SQL ID field on join table corresponding to this side
	SQLOtherIDField string // SQL ID field on join table corresponding to the other side
}

func (r *BelongsToManyIDs) RelationName() string {
	return r.Name
}
func (r *BelongsToManyIDs) RelationGoValueField() string {
	return r.GoValueField
}
