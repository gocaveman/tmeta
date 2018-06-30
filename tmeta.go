// Provides SQL table metadata, enabling select field lists, easy getters,
// relations when using a query builder like gocraft/dbr.
//
package tmeta

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

const tmetaTag = "tmeta"

/*
Some conventions for consistency:
- Names of anything in Go have "Go" in them, names of anything in SQL have "SQL" in them; otherwise
  it's really easy to confuse what sort of name you mean, we use both in different cases.
- Fields (of anything - structs or tables) are called "Field", not "Column" or "Col" or anything else.
- The "Name" field on an object is its logical name, e.g. TableInfo.Name, but we don't refer to fields
  or relations or anything else using "Name", e.g. it's just a "VersionField", not a "VersionFieldName".
- Any time a reflect.Type is passed around it is assumed it has been dereferenced and any pointers
  or interfaces removed.  Public functions should take care to derefType() or derefValue() as needed.
*/

func NewMeta() *Meta {
	return &Meta{
		tableInfoMap: make(map[reflect.Type]*TableInfo),
	}
}

type Meta struct {
	tableInfoMap map[reflect.Type]*TableInfo
	// FIXME: delay DriverName until we actually need it - a better abstraction might be some sort of Dialect
	// DriverName   string
}

type TableInfo struct {
	name        string       // the short name for this table, by convention this is often the SQLTableName but not required
	sqlName     string       // SQL table names
	goType      reflect.Type // underlying Go type (pointer removed)
	sqlPKFields []string     // SQL primary key field names

	// FIXME: not sure if we even need to know this - SQLFields allows the caller to specify if they want
	// the pk fields, which is the main use case - deciding if you need pk fields in the insert, if after
	// writing the tests this never comes up as needed, then just remove

	pkAutoIncr      bool   // true if keys are auto-incremented by the database
	sqlVersionField string // name of version col, empty disables optimistic locking
	// TODO: function to generate new version number (should increment for number or generate nonce for string)
	RelationMap
}

func NewTableInfo(goType reflect.Type) *TableInfo {
	var ti TableInfo
	return ti.SetGoType(goType)
}

func (ti *TableInfo) SetGoType(goType reflect.Type) *TableInfo {
	ti.goType = goType
	if ti.name == "" {
		n := camelToSnake(goType.Name())
		return ti.SetName(n)
	}
	return ti
}

func (ti *TableInfo) SetName(name string) *TableInfo {
	ti.name = name
	if ti.sqlName == "" {
		return ti.SetSQLName(name)
	}
	return ti
}

func (ti *TableInfo) SetSQLName(sqlName string) *TableInfo {
	ti.sqlName = sqlName
	return ti
}

func (ti *TableInfo) Name() string {
	return ti.name
}

func (ti *TableInfo) SQLName() string {
	return ti.sqlName
}

func (ti *TableInfo) GoType() reflect.Type {
	return ti.goType
}

func (ti *TableInfo) SQLPKFields() []string {
	return ti.sqlPKFields
}

func (ti *TableInfo) GoPKFields() []string {
	var ret []string
	for _, pkf := range ti.sqlPKFields {
		idx := sqlFieldIndex(ti.GoType(), pkf)
		sf := ti.GoType().FieldByIndex(idx)
		ret = append(ret, sf.Name)
	}
	return ret
}

func (ti *TableInfo) PKAutoIncr() bool {
	return ti.pkAutoIncr
}

func (ti *TableInfo) SQLVersionField() string {
	return ti.sqlVersionField
}

func (ti *TableInfo) SetSQLPKFields(isAutoIncr bool, sqlPKFields []string) *TableInfo {
	ti.pkAutoIncr = isAutoIncr
	ti.sqlPKFields = sqlPKFields
	return ti
}

func (ti *TableInfo) AddRelation(relation Relation) *TableInfo {
	if ti.RelationMap == nil {
		ti.RelationMap = make(RelationMap)
	}
	ti.RelationMap[relation.RelationName()] = relation
	return ti
}

func (ti *TableInfo) SetSQLVersionField(sqlVersionField string) *TableInfo {
	ti.sqlVersionField = sqlVersionField
	return ti
}

func (ti *TableInfo) IsSQLPKField(sqlName string) bool {
	for _, f := range ti.sqlPKFields {
		if f == sqlName {
			return true
		}
	}
	return false
}

func (ti *TableInfo) SQLFields(withPK bool) []string {
	var ret []string
	idxes := exportedFieldIndexes(ti.goType)

	for _, idx := range idxes {
		sf := ti.goType.FieldByIndex(idx)
		sfdb := strings.SplitN(sf.Tag.Get("db"), ",", 2)[0]
		if sfdb == "" || sfdb == "-" {
			continue
		}
		if !ti.IsSQLPKField(sfdb) || withPK {
			ret = append(ret, sfdb)
		}
	}

	return ret
}

// SQLPKWhere returns a where clause with the primary key fields ANDed together and "?" for placeholders.
// For example: "key1 = ? AND key2 = ?"
func (ti *TableInfo) SQLPKWhere() string {
	var buf bytes.Buffer
	for _, fn := range ti.SQLPKFields() {
		fmt.Fprintf(&buf, " AND %s = ?", fn)
	}
	return strings.TrimPrefix(buf.String(), " AND ")
}

// PKValues scans an object for the pk fields.
// Will panic if pk fields cannot be found (e.g. if `o` is of the wrong type).
func (ti *TableInfo) PKValues(o interface{}) []interface{} {
	v := derefValue(reflect.ValueOf(o))
	ret := make([]interface{}, 0, len(ti.sqlPKFields))
	for _, sfn := range ti.sqlPKFields {
		ret = append(ret, sqlFieldValue(v, sfn))
	}
	return ret
}

// SQLValueMap returns a map of [SQLField]->[Value] for all database fields on this struct.
// If includePks is false then primary key fields are omitted.
func (ti *TableInfo) SQLValueMap(o interface{}, includePks bool) map[string]interface{} {
	v := derefValue(reflect.ValueOf(o))
	t := v.Type()
	idxes := exportedFieldIndexes(t)
	ret := make(map[string]interface{}, len(idxes))
	for _, idx := range idxes {
		sfdb := strings.SplitN(t.FieldByIndex(idx).Tag.Get("db"), ",", 2)[0]
		if sfdb == "" || sfdb == "-" { // skip non-db-tagged fields
			continue
		}
		if !ti.IsSQLPKField(sfdb) || includePks {
			ret[sfdb] = v.FieldByIndex(idx).Interface()
		}
	}
	return ret
}

// 	names := ti.SQLPKAndFields()
// 	ret := make([]string, 0, len(names)-len(ti.sqlPKFields))
// nameLoop:
// 	for i := 0; i < len(names); i++ {
// 		name := names[i]
// 		for _, keyName := range ti.sqlPKFields {
// 			if keyName == name {
// 				continue nameLoop
// 			}
// 		}
// 		ret = append(ret, name)
// 	}
// 	return ret
// }

// func (ti *TableInfo) SQLPKAndFields() []string {
// 	numFields := ti.goType.NumField()
// 	ret := make([]string, 0, numFields)
// 	for i := 0; i < numFields; i++ {
// 		structField := ti.goType.Field(i)
// 		sqlFieldName := strings.SplitN(structField.Tag.Get("db"), ",", 2)[0]
// 		if sqlFieldName == "" || sqlFieldName == "-" {
// 			continue
// 		}
// 		ret = append(ret, sqlFieldName)
// 	}
// 	return ret
// }

// SetTableInfo assigns the TableInfo for a specific Go type, overwriting any existing value.
// This will remove/overwrite the name associated with that type as well, and will also remove
// any entry with the same name before setting.  This behavior allows overrides where a package
// a default TableInfo can exist for a type but a specific usage requires it to be assigned differently.
func (m *Meta) SetTableInfo(ty reflect.Type, ti *TableInfo) {
	delete(m.tableInfoMap, m.typeForName(ti.name))
	m.tableInfoMap[derefType(ty)] = ti
}

// For will return the TableInfo for a struct.  Pointers will be dereferenced.
// Nil will be returned if no such table exists.
func (m *Meta) For(i interface{}) *TableInfo {
	t := derefType(reflect.TypeOf(i))
	return m.ForType(t)
}

// For will return the TableInfo for a struct type.  Pointers will be dereferenced.
// Nil will be returned if no such table exists.
func (m *Meta) ForType(t reflect.Type) *TableInfo {
	return m.tableInfoMap[derefType(t)]
}

// ForName will return the TableInfo with the given name.
// Nil will be returned if no such table exists.
func (m *Meta) ForName(name string) *TableInfo {
	return m.ForType(m.typeForName(name))
}

func (m *Meta) typeForName(name string) reflect.Type {
	for t, ti := range m.tableInfoMap {
		if ti.name == name {
			return t
		}
	}
	return nil
}

// Parse will extract TableInfo data from the type of the value given (must be a properly tagged struct).
// The resulting TableInfo will be set as if by SetTableInfo.
func (m *Meta) Parse(i interface{}) error {
	t := derefType(reflect.TypeOf(i))
	return m.ParseType(t)
}

// ParseTypeNamed works like ParseType but allows you to specify the name rather than having
// it being derived from the name of the Go struct.  This is intended to allow you to override
// an existing type with your own struct.  Example: A package comes with a "Widget" type, named
// "widget", and you make a "CustomWidget" with whatever additional fields (optionally) embedding
// the original "Widget".  You can then call meta.ParseTypeNamed(reflect.TypeOf(CustomWidget{}), "widget")
// to override the original definition.
func (m *Meta) ParseTypeNamed(t reflect.Type, name string) error {

	t = derefType(t)

	var ti TableInfo

	ti.SetName(name)
	ti.SetGoType(t)

	for _, idx := range exportedFieldIndexes(t) {
		f := t.FieldByIndex(idx)

		tag := f.Tag.Get(tmetaTag)
		tagv := structTagToValues(tag)

		// check relations
		if len(tagv["belongs_to"]) > 0 {

			// relation name defaults to snake of Go field name unless specified
			name := tagv.Get("relation_name")
			if name == "" {
				name = camelToSnake(f.Name)
			}

			sqlIDField := tagv.Get("sql_id_field")
			if sqlIDField == "" {
				sqlIDField = camelToSnake(f.Name) + "_id"
			}

			ti.AddRelation(&BelongsTo{
				Name:         name,
				GoValueField: f.Name,     // e.g. "Author" (of type *Author)
				SQLIDField:   sqlIDField, // e.g. "author_id"
			})

		}
		if len(tagv["has_many"]) > 0 {

			name := tagv.Get("relation_name")
			if name == "" {
				name = camelToSnake(f.Name)
			}

			sqlOtherIDField := tagv.Get("sql_other_id_field")
			if sqlOtherIDField == "" {
				// sqlOtherIDField = camelToSnake(elemDerefType(f.Type).Name()) + "_id"
				sqlOtherIDField = ti.Name() + "_id"
			}

			ti.AddRelation(&HasMany{
				Name:            name,
				GoValueField:    f.Name,
				SQLOtherIDField: sqlOtherIDField,
			})

		}
		if len(tagv["has_one"]) > 0 {

			name := tagv.Get("relation_name")
			if name == "" {
				name = camelToSnake(f.Name)
			}

			sqlOtherIDField := tagv.Get("sql_other_id_field")
			if sqlOtherIDField == "" {
				// sqlOtherIDField = camelToSnake(derefType(f.Type).Name()) + "_id"
				sqlOtherIDField = ti.Name() + "_id"
			}

			ti.AddRelation(&HasOne{
				Name:            name,
				GoValueField:    f.Name,
				SQLOtherIDField: sqlOtherIDField,
			})

		}
		if len(tagv["belongs_to_many"]) > 0 {

			name := tagv.Get("relation_name")
			if name == "" {
				name = camelToSnake(f.Name)
			}

			joinName := tagv.Get("join_name")
			if joinName == "" {
				return fmt.Errorf("`join_name` not specified for belongs_to_many relation %q", name)
			}

			sqlIDField := tagv.Get("sql_id_field")
			if sqlIDField == "" {
				// FIXME: would be nice to depend on the type name + "_id" but it should be the
				// same behavior as belongs_to_many_ids and it doesn't have the type name...
				sqlIDField = ti.SQLPKFields()[0]
			}

			sqlOtherIDField := tagv.Get("sql_other_id_field")
			if sqlOtherIDField == "" {
				// we try to guess this based on the join name
				s := strings.Replace(joinName, ti.Name(), "", 1)
				if s == joinName { // if the join name doesn't contain the relation name, can't guess
					return fmt.Errorf("`sql_other_id_field` tag is required for relation %q, unable to guess it's value", name)
				}
				s = strings.Trim(s, "_")
				sqlOtherIDField = s + "_id"
			}

			rel := &BelongsToMany{
				Name:            name,
				GoValueField:    f.Name,
				JoinName:        joinName,
				SQLIDField:      sqlIDField,
				SQLOtherIDField: sqlOtherIDField,
			}
			ti.AddRelation(rel)
		}
		if len(tagv["belongs_to_many_ids"]) > 0 {

			name := tagv.Get("relation_name")
			if name == "" {
				name = camelToSnake(f.Name)
			}

			joinName := tagv.Get("join_name")
			if joinName == "" {
				return fmt.Errorf("`join_name` not specified for belongs_to_many_ids relation %q", name)
			}

			sqlIDField := tagv.Get("sql_id_field")
			if sqlIDField == "" {
				// FIXME: see if we can depend on the type name? instead of pk field name on this table, may not be possible
				sqlIDField = ti.SQLPKFields()[0]
			}

			sqlOtherIDField := tagv.Get("sql_other_id_field")
			if sqlOtherIDField == "" {
				// we try to guess this based on the join name
				s := strings.Replace(joinName, ti.Name(), "", 1)
				if s == joinName { // if the join name doesn't contain the relation name, can't guess
					return fmt.Errorf("`sql_other_id_field` tag is required for relation %q, unable to guess it's value", name)
				}
				s = strings.Trim(s, "_")
				sqlOtherIDField = s + "_id"
			}

			rel := &BelongsToManyIDs{
				Name:            name,
				GoValueField:    f.Name,
				JoinName:        joinName,
				SQLIDField:      sqlIDField,
				SQLOtherIDField: sqlOtherIDField,
			}
			ti.AddRelation(rel)

		}

		// past this point, skip fields not tagged with db
		sqlName := strings.Split(f.Tag.Get("db"), ",")[0]
		if sqlName == "" || sqlName == "-" {
			continue
		}

		// check for primary key
		if len(tagv["pk"]) > 0 {
			ti.sqlPKFields = append(ti.sqlPKFields, sqlName)
			if len(tagv["auto_incr"]) > 0 {
				ti.pkAutoIncr = true
			}
			continue
		}

		// check for version (optimistic locking)
		if len(tagv["version"]) > 0 {
			ti.sqlVersionField = sqlName
			continue
		}

	}

	if len(ti.sqlPKFields) < 1 {
		return fmt.Errorf("no primary key fields found for type %v", t)
	}

	m.SetTableInfo(t, &ti)

	return nil

}

// ParseType will extract TableInfo data from the given type (must be a properly tagged struct).
// The resulting TableInfo will be set as if by SetTableInfo.
func (m *Meta) ParseType(t reflect.Type) error {
	t = derefType(t)

	return m.ParseTypeNamed(t, camelToSnake(t.Name()))
}

// ReplaceSQLNames provides the SQLName of each table to a function and sets the
// table name to the return value.  For example, you can easily prefix all of the
// tables by doing:
// m.ReplaceSQLNames(func(n string) string { return "prefix_" + n })
func (m *Meta) ReplaceSQLNames(namer func(name string) string) {
	for _, ti := range m.tableInfoMap {
		ti.sqlName = namer(ti.sqlName)
	}
}

// func (m *Meta) AddTable(i interface{}) *TableInfo {
// 	return m.AddTableWithName(i, TableNameMapper(derefType(reflect.TypeOf(i)).name()))
// }

// func (m *Meta) AddTableWithName(i interface{}, tableName string) *TableInfo {

// 	t := derefType(reflect.TypeOf(i))
// 	tableInfo := m[t]
// 	if tableInfo != nil {
// 		if tableInfo.SQLTableName != tableName {
// 			panic(fmt.Errorf("attempt to call AddTableWithName with a different name (expected %q, got %q)", tableInfo.SQLTableName, tableName))
// 		}
// 		return tableInfo
// 	}

// 	tableInfo = &TableInfo{
// 		goType:       t,
// 		SQLTableName: tableName,
// 	}

// 	m[t] = tableInfo

// 	return tableInfo

// }

// func (m *Meta) TableFor(i interface{}) *TableInfo {
// 	return m.TableForType(reflect.TypeOf(i))
// }

// func (m *Meta) TableForType(t reflect.Type) *TableInfo {
// 	return m[derefType(t)]
// }
