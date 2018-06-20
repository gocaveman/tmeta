package tmeta

type Relation interface {
	RelationName() string
}

type RelationMap map[string]Relation

type belongsTo struct {
	name           string
	objFieldGoName string
	idFieldGoName  string
}

func NewBelongsTo(name, objFieldGoName, idFieldGoName string) Relation {
	return &belongsTo{
		name:           name,
		objFieldGoName: objFieldGoName,
		idFieldGoName:  idFieldGoName,
	}
}

func (r *belongsTo) RelationName() string {
	return r.name
}

type hasMany struct {
	name                string
	sliceFieldGoName    string
	otherIDFieldSQLName string
}

func NewHasMany(name, sliceFieldGoName, otherIDFieldSQLName string) Relation {
	return &hasMany{
		name:                name,
		sliceFieldGoName:    sliceFieldGoName,
		otherIDFieldSQLName: otherIDFieldSQLName,
	}
}

func (r *hasMany) RelationName() string {
	return r.name
}
