// Provides SQL table metadata, enabling select field lists, easy getters,
// relations when using a query builder like gocraft/dbr.
//
package tmeta

import (
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

func NewMeta( /*driverName string*/ ) *Meta {
	return &Meta{
		tableInfoMap: make(map[reflect.Type]*TableInfo),
		// DriverName:   driverName,
	}
}

type Meta struct {
	tableInfoMap map[reflect.Type]*TableInfo
	// FIXME: delay DriverName until we actually need it - a better abstraction might be some sort of Dialect
	// DriverName   string
}

type TableInfo struct {
	Name            string       // the short name for this table, by convention this is often the SQLTableName but not required
	SQLName         string       // SQL table names
	GoType          reflect.Type // underlying Go type (pointer removed)
	KeySQLFields    []string     // SQL primary key field names
	KeyAutoIncr     bool         // true if keys are auto-incremented by the database
	VersionSQLField string       // name of version col, empty disables optimistic locking
	// TODO: function to generate new version number (should increment for number or generate nonce for string)
	RelationMap RelationMap
}

func (ti *TableInfo) SetKey(isAutoIncr bool, keySQLFields []string) *TableInfo {
	ti.KeyAutoIncr = isAutoIncr
	ti.KeySQLFields = keySQLFields
	return ti
}

func (ti *TableInfo) AddRelation(relation Relation) *TableInfo {
	if ti.RelationMap == nil {
		ti.RelationMap = make(RelationMap)
	}
	ti.RelationMap[relation.RelationName()] = relation
	return ti
}

func (ti *TableInfo) SetVersionSQLField(versionSQLField string) *TableInfo {
	ti.VersionSQLField = versionSQLField
	return ti
}

func (ti *TableInfo) Fields() []string {
	names := ti.KeysAndFields()
	ret := make([]string, 0, len(names)-len(ti.KeySQLFields))
nameLoop:
	for i := 0; i < len(names); i++ {
		name := names[i]
		for _, keyName := range ti.KeySQLFields {
			if keyName == name {
				continue nameLoop
			}
		}
		ret = append(ret, name)
	}
	return ret
}

func (ti *TableInfo) KeysAndFields() []string {
	numFields := ti.GoType.NumField()
	ret := make([]string, 0, numFields)
	for i := 0; i < numFields; i++ {
		structField := ti.GoType.Field(i)
		sqlFieldName := strings.SplitN(structField.Tag.Get("db"), ",", 2)[0]
		if sqlFieldName == "" || sqlFieldName == "-" {
			continue
		}
		ret = append(ret, sqlFieldName)
	}
	return ret
}

// SetTableInfo assigns the TableInfo for a specific Go type, overwriting any existing value.
// This will remove/overwrite the name associated with that type as well, and will also remove
// any entry with the same name before setting.  This behavior allows overrides where a package
// a default TableInfo can exist for a type but a specific usage requires it to be assigned differently.
func (m *Meta) SetTableInfo(ty reflect.Type, ti *TableInfo) {
	delete(m.tableInfoMap, m.typeForName(ti.Name))
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
	// for _, ti := range m.tableInfoMap {
	// 	if ti.Name == name {
	// 		return ti
	// 	}
	// }
	// return nil
}

func (m *Meta) typeForName(name string) reflect.Type {
	for t, ti := range m.tableInfoMap {
		if ti.Name == name {
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

// ParseType will extract TableInfo data from the given type (must be a properly tagged struct).
// The resulting TableInfo will be set as if by SetTableInfo.
func (m *Meta) ParseType(t reflect.Type) error {
	t = derefType(t)

	var ti TableInfo

	ti.Name = camelToSnake(t.Name())
	ti.SQLName = ti.Name

	for _, idx := range exportedFieldIndexes(t) {
		f := t.FieldByIndex(idx)

		// skip fields not tagged with db
		sqlName := strings.Split(f.Tag.Get("db"), ",")[0]
		if sqlName == "" || sqlName == "-" {
			continue
		}

		tag := f.Tag.Get(tmetaTag)
		tagv := structTagToValues(tag)
		if len(tagv["key"]) > 0 {
			ti.KeySQLFields = append(ti.KeySQLFields, sqlName)

			if len(tagv["auto_incr"]) > 0 {
				ti.KeyAutoIncr = true
			}

		}

	}

	m.SetTableInfo(t, &ti)

	return nil
}

// func (m *Meta) AddTable(i interface{}) *TableInfo {
// 	return m.AddTableWithName(i, TableNameMapper(derefType(reflect.TypeOf(i)).Name()))
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
// 		GoType:       t,
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
