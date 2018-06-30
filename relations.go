package tmeta

import "reflect"

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
