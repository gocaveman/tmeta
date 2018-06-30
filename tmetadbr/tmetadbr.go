// Adapt tmeta to github.com/gocraft/dbr for easy query building.
package tmetadbr

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/gocaveman/tmeta"
	"github.com/gocraft/dbr"
	"github.com/gocraft/dbr/dialect"
)

var (
	// ErrTypeNotRegistered is returned when the type of a variable does not correspond to a Meta entry.
	ErrTypeNotRegistered = errors.New("tmetadbr: type not registered")

	// ErrUpdateFailed is used to indicate the record could not be found or an optimistic locking update failure (version changed since last read)
	ErrUpdateFailed = &errorWithCode{code: 409, msg: "tmetadbr: update failed (not found or version conflict)"}
)

// // IDGenerator is a function that can take an object and create IDs for the
// // primary key(s).  Only needed for non-auto_increment pks and in the usual
// // case it's only needed for strings, since integers are usually auto_increment.
// type IDGenerator func(meta *tmeta.Meta, obj interface{}) error

// // DefaultIDGenerator is used when an IDGenerator is not set on Builder.
// var DefaultIDGenerator = UUIDV4Generator

// Session is implemented by *dbr.Session and *dbr.Tx
type Session interface {
	InsertInto(table string) *dbr.InsertStmt
	Select(column ...string) *dbr.SelectStmt
	Update(table string) *dbr.UpdateStmt
	DeleteFrom(table string) *dbr.DeleteStmt

	InsertBySql(query string, value ...interface{}) *dbr.InsertStmt
	SelectBySql(query string, value ...interface{}) *dbr.SelectStmt
	UpdateBySql(query string, value ...interface{}) *dbr.UpdateStmt
	DeleteBySql(query string, value ...interface{}) *dbr.DeleteStmt
}

var (
	// verify types are compatible at compile time
	_ Session = &dbr.Session{}
	_ Session = &dbr.Tx{}
)

func New(sess Session, meta *tmeta.Meta) *Builder {
	return &Builder{
		Session: sess,
		Meta:    meta,
		// IDGenerator: DefaultIDGenerator,
	}
}

type Builder struct {
	Session Session
	*tmeta.Meta
	// IDGenerator IDGenerator
}

// hack this dialect detection for now, would be nicer to have something more
// formal but this is workable for the time being
func (b *Builder) dbrDialect() dbr.Dialect {
	f := derefValue(reflect.ValueOf(b.Session)).FieldByName("Dialect")
	if !f.IsValid() {
		panic("unable to find Dialect field on Session")
	}
	return f.Interface().(dbr.Dialect)
}

// MustSelect is the same as Select but panics on error.
func (b *Builder) MustSelect(o interface{}) *dbr.SelectStmt {
	ret, err := b.Select(o)
	if err != nil {
		panic(err)
	}
	return ret
}

// Select will build a select statement with the field list of the type provided
// from the appropriate table. If a slice is provided, the table is derived from
// the slice's element type.
func (b *Builder) Select(o interface{}) (*dbr.SelectStmt, error) {

	ti := b.Meta.ForType(elemDerefType(reflect.TypeOf(o)))
	if ti == nil {
		return nil, ErrTypeNotRegistered
	}

	return b.Session.
			Select(ti.SQLFields(true)...).
			From(ti.SQLName()),
		nil
}

// MustSelectByID is the same as SelectByID but panics on error.
func (b *Builder) MustSelectByID(o interface{}, ids ...interface{}) *dbr.SelectStmt {
	ret, err := b.SelectByID(o, ids...)
	if err != nil {
		panic(err)
	}
	return ret
}

// SelectByID will build a select statement on the appropriate table with a where
// clause matching the given primary keys.  If ids is non-zero len it will be used
// as the pk values otherwise the pk values will be extracted from the object provided.
func (b *Builder) SelectByID(o interface{}, ids ...interface{}) (*dbr.SelectStmt, error) {

	ti := b.Meta.ForType(elemDerefType(reflect.TypeOf(o)))
	if ti == nil {
		return nil, ErrTypeNotRegistered
	}

	// fill ids if not provided
	if len(ids) == 0 {
		ids = ti.PKValues(o)
	}

	return b.Session.
			Select(ti.SQLFields(true)...).
			From(ti.SQLName()).
			Where(ti.SQLPKWhere(), ids...),
		nil
}

// MustInsert is the same as Insert but panics on error.
func (b *Builder) MustInsert(o interface{}) *dbr.InsertStmt {
	ret, err := b.Insert(o)
	if err != nil {
		panic(err)
	}
	return ret
}

// Insert generates an insert statement for the object(s) provided.  Slice is supported.
// It also calls CreateTimeTouch on the object(s) if possible.
func (b *Builder) Insert(o interface{}) (*dbr.InsertStmt, error) {

	// NOTE: We don't bother with the version field here, making the initial record
	// version 0 makes a lot of sense and if the caller provides a value there's no
	// reason not to use it.

	ti := b.Meta.ForType(elemDerefType(reflect.TypeOf(o)))
	if ti == nil {
		return nil, ErrTypeNotRegistered
	}

	stmt := b.Session.
		InsertInto(ti.SQLName()).
		Columns(ti.SQLFields(!ti.PKAutoIncr())...)

	ov := derefValue(reflect.ValueOf(o))

	if ov.Kind() == reflect.Slice { // multiple records
		for i := 0; i < ov.Len(); i++ {
			elv := ov.Index(i)
			var el interface{}
			if elv.Kind() != reflect.Ptr { // make sure it's a pointer
				el = elv.Addr().Interface()
			} else {
				el = elv.Interface()
			}
			// touch create time if possible
			if ctt, ok := el.(CreateTimeToucher); ok {
				ctt.CreateTimeTouch()
			}
			// touch update time if possible
			if ctt, ok := el.(UpdateTimeToucher); ok {
				ctt.UpdateTimeTouch()
			}
			stmt = stmt.Record(el)
		}

	} else { // one record
		// touch create time if possible
		if ctt, ok := o.(CreateTimeToucher); ok {
			ctt.CreateTimeTouch()
		}
		// touch update time if possible
		if ctt, ok := o.(UpdateTimeToucher); ok {
			ctt.UpdateTimeTouch()
		}
		stmt = stmt.Record(o)
	}

	return stmt, nil
}

// // InsertNew acts like Insert for any records that have one or more empty
// // primary key values.  Records with non-empty pks are ignored.
// // Note that (nil,nil) is a valid return, indicating that no records are to be inserted.
// func (b *Builder) InsertNew(o interface{}) (*dbr.InsertStmt, error) {

// 	ti := b.Meta.ForType(elemDerefType(reflect.TypeOf(o)))
// 	if ti == nil {
// 		return nil, ErrTypeNotRegistered
// 	}

// 	// returns true if all pk fields are empty value
// 	recIsNew := func(inst interface{}) bool {
// 		inst = derefValue(reflect.ValueOf(inst))
// 		pks := ti.SQLPKFields()
// 		for _, pk := range pks {
// 			fv := sqlFieldValue(reflect.ValueOf(inst), pk)
// 			if !isZero(fv) {
// 				return false
// 			}
// 		}
// 		return true
// 	}

// 	ov := derefValue(reflect.ValueOf(o))

// 	if ov.Kind() == reflect.Slice { // multiple records

// 		// we need to build a fresh slice that omits the "not-new" ones
// 		newSlice := reflect.New(ov.Type())

// 		for i := 0; i < ov.Len(); i++ {
// 			elv := ov.Index(i)
// 			if recIsNew(elv.Interface()) {
// 				newSlice = reflect.Append(newSlice, elv)
// 			}
// 		}

// 		if newSlice.Len() > 0 {
// 			return b.Insert(newSlice)
// 		}

// 	} else { // one record
// 		if recIsNew(o) {
// 			return b.Insert(o)
// 		}
// 	}

// 	return nil, nil
// }

// MustUpdateByID is the same as UpdateByID but panics on error.
func (b *Builder) MustUpdateByID(o interface{}) *dbr.UpdateStmt {
	ret, err := b.UpdateByID(o)
	if err != nil {
		panic(err)
	}
	return ret
}

// UpdateByID creates an update statement for a record using it's primary key,
// taking into account the update time (if UpdateTimeToucher is supported), version field
// (if SQLVersionField is not empty).  If using a version field, its value should be the same
// as it was selected with and this method will attempt to increment it by one.
func (b *Builder) UpdateByID(o interface{}) (*dbr.UpdateStmt, error) {

	// TODO: optimistic locking with version column
	// TODO: date_updated field

	ti := b.Meta.For(o)
	if ti == nil {
		return nil, ErrTypeNotRegistered
	}

	// touch the update time if possible
	po := o
	if reflect.TypeOf(po).Kind() != reflect.Ptr { // make sure it's a pointer
		po = reflect.ValueOf(po).Addr().Interface()
	}
	if ctt, ok := po.(UpdateTimeToucher); ok {
		ctt.UpdateTimeTouch()
	}

	vmap := ti.SQLValueMap(o, false)

	// extract and increment version value
	var curVer interface{}
	if ti.SQLVersionField() != "" {
		curVer := vmap[ti.SQLVersionField()]
		newVer, err := incrementInteger(curVer)
		if err != nil {
			return nil, err
		}
		vmap[ti.SQLVersionField()] = newVer
	}

	ustmt := b.Session.
		Update(ti.SQLName()).
		SetMap(vmap).
		Where(ti.SQLPKWhere(), ti.PKValues(o)...)

	if ti.SQLVersionField() != "" { // optimistic lock prevents updating record with newer version
		ustmt = ustmt.Where(ti.SQLVersionField()+" = ?", curVer)
	}

	return ustmt, nil
}

// // update only if pk is not empty, intended for lists
// func (b *Builder) UpdateExisting(o interface{}) (*dbr.UpdateStmt, error) {
// 	// TODO: optimistic locking with version column
// 	// TODO: date_updated field
// 	panic("not implemented")
// }

// MustDeleteByID is the same as DeleteByID but panics on error.
func (b *Builder) MustDeleteByID(o interface{}, ids ...interface{}) *dbr.DeleteStmt {
	ret, err := b.DeleteByID(o, ids...)
	if err != nil {
		panic(err)
	}
	return ret
}

// DeleteByID make a delete statement with a where clause by the primary key.
// If len(ids)>0 then those values are included as the SQL where clause.
// Otherwise the primary keys are extracted from the object provided
// and, if optimistic locking is enabled for this type, the version number is included
// in the SQL where clause also.
func (b *Builder) DeleteByID(o interface{}, ids ...interface{}) (*dbr.DeleteStmt, error) {

	ti := b.Meta.For(o)
	if ti == nil {
		return nil, ErrTypeNotRegistered
	}

	dstmt := b.Session.DeleteFrom(ti.SQLName())
	// fill ids if not provided
	if len(ids) == 0 {
		ids = ti.PKValues(o)
		// check for version field and add to where clause
		if ti.SQLVersionField() != "" {
			dstmt = dstmt.Where(ti.SQLVersionField()+" = ?",
				sqlFieldValue(reflect.ValueOf(o), ti.SQLVersionField()))
		}
	}

	// main where clause by ID(s)
	dstmt = dstmt.Where(ti.SQLPKWhere(), ids...)

	return dstmt, nil
}

// MustSelectRelation is the same as SelectRelation but will panic on error.
func (b *Builder) MustSelectRelation(o interface{}, relationName string) (stmt *dbr.SelectStmt) {
	var err error
	stmt, err = b.SelectRelation(o, relationName)
	if err != nil {
		panic(err)
	}
	return
}

// SelectRelation is the same as SelectRelationPtr but does not return the field pointer.
func (b *Builder) SelectRelation(o interface{}, relationName string) (stmt *dbr.SelectStmt, reterr error) {
	stmt, _, reterr = b.SelectRelationPtr(o, relationName)
	return
}

// MustSelectRelationPtr is the same as SelectRelationPtr but will panic on error.
func (b *Builder) MustSelectRelationPtr(o interface{}, relationName string) (stmt *dbr.SelectStmt, fieldPtr interface{}) {
	var err error
	stmt, fieldPtr, err = b.SelectRelationPtr(o, relationName)
	if err != nil {
		panic(err)
	}
	return
}

// SelectRelationPtr returns a select statement that will select the relation into the appropriate
// field, based on the name of that relation.  The second return value is a pointer to the field
// which can be passed to stmt.Load() to populate the correct field.
// The object provided must not be a slice.
func (b *Builder) SelectRelationPtr(o interface{}, relationName string) (stmt *dbr.SelectStmt, fieldPtr interface{}, reterr error) {

	ti := b.Meta.For(o)
	if ti == nil {
		return nil, nil, ErrTypeNotRegistered
	}

	rel := ti.RelationNamed(relationName)
	if rel == nil {
		return nil, nil, fmt.Errorf("relation %q not found", relationName)
	}

	vo := derefValue(reflect.ValueOf(o))

	switch r := rel.(type) {

	case *tmeta.BelongsTo:
		gvf := vo.FieldByName(r.GoValueField)
		targetType := derefType(gvf.Type())
		targetTI := b.Meta.ForType(targetType)
		if targetTI == nil {
			return nil, nil, fmt.Errorf("%T is not registered", gvf.Interface())
		}

		stmt = b.Session.
			Select(targetTI.SQLFields(true)...).
			From(targetTI.SQLName()).
			Where(targetTI.SQLPKFields()[0]+" = ?",
				sqlFieldValue(vo, r.SQLIDField))
		fieldPtr = ti.RelationTargetPtr(o, relationName)
		return

	case *tmeta.HasMany:
		gvf := vo.FieldByName(r.GoValueField)
		targetType := elemDerefType(gvf.Type()) // look in the slice for it's struct type
		targetTI := b.Meta.ForType(targetType)
		if targetTI == nil {
			return nil, nil, fmt.Errorf("%T is not registered", gvf.Interface())
		}

		stmt = b.Session.
			Select(targetTI.SQLFields(true)...).
			From(targetTI.SQLName()).
			Where(r.SQLOtherIDField+" = ?", ti.PKValues(o)[0])
		fieldPtr = ti.RelationTargetPtr(o, relationName)
		return

	case *tmeta.HasOne:
		gvf := vo.FieldByName(r.GoValueField)
		targetType := derefType(gvf.Type())
		targetTI := b.Meta.ForType(targetType)
		if targetTI == nil {
			return nil, nil, fmt.Errorf("%T is not registered", gvf.Interface())
		}

		stmt = b.Session.
			Select(targetTI.SQLFields(true)...).
			From(targetTI.SQLName()).
			Where(r.SQLOtherIDField+" = ?", ti.PKValues(o)[0])
		fieldPtr = ti.RelationTargetPtr(o, relationName)
		return

	case *tmeta.BelongsToMany:

		joinTI := b.Meta.ForName(r.JoinName)

		targetType := elemDerefType(vo.FieldByName(r.GoValueField).Type())
		targetTI := b.Meta.ForType(targetType)

		stmt = b.Session.
			Select(
				stringsAddPrefix(targetTI.SQLFields(true), targetTI.SQLName()+".")...,
			).
			From(joinTI.SQLName()).
			Join(targetTI.SQLName(),
				fmt.Sprintf(`%s.%s = %s.%s`,
					joinTI.SQLName(), r.SQLOtherIDField,
					targetTI.SQLName(), targetTI.SQLPKFields()[0],
				)).
			Where(joinTI.SQLName()+"."+r.SQLIDField+" = ?", ti.PKValues(o)[0])
		fieldPtr = ti.RelationTargetPtr(o, relationName)
		return

	case *tmeta.BelongsToManyIDs:

		joinTI := b.Meta.ForName(r.JoinName)
		stmt = b.Session.
			Select(r.SQLOtherIDField).
			From(joinTI.SQLName()).
			Where(r.SQLIDField+" = ?", ti.PKValues(o)[0])
		fieldPtr = ti.RelationTargetPtr(o, relationName)
		return

	}

	return nil, nil, fmt.Errorf("relation %q is not of a suppported type", relationName)
}

// TODO: this one we should probably do
// // AttachRelation will look at the field corresponding to the given relation and will set the ID
// // that links back to `o` appropriately.  The ID field must either be empty, or already be set to the
// // correct value, any other value is an error.
// func (b *Builder) AttachRelation(o interface{}, relationName string) error {
// 	panic("not implemented")
// }

// MustDeleteRelationNotIn is the same as DeleteRelationNotIn but panics on error.
func (b *Builder) MustDeleteRelationNotIn(o interface{}, relationName string) *dbr.DeleteStmt {
	ret, err := b.DeleteRelationNotIn(o, relationName)
	if err != nil {
		panic(err)
	}
	return ret
}

// DeleteRelationNotIn will make a delete statement for the records corresponding
// to the IDs indicated by the given relation.  The relation must be of type
// BelongsToManyIDs.
func (b *Builder) DeleteRelationNotIn(o interface{}, relationName string) (*dbr.DeleteStmt, error) {

	ti := b.Meta.For(o)
	if ti == nil {
		return nil, ErrTypeNotRegistered
	}

	rel := ti.RelationNamed(relationName)
	if rel == nil {
		return nil, fmt.Errorf("relation %q not found", relationName)
	}

	switch relv := rel.(type) {
	case *tmeta.BelongsToManyIDs:

		vo := derefValue(reflect.ValueOf(o))
		sliceV := derefValue(vo.FieldByName(relv.GoValueField))

		joinTI := b.Meta.ForName(relv.JoinName)
		stmt := b.Session.DeleteFrom(joinTI.SQLName())
		stmt = stmt.Where(relv.SQLIDField+" = ?", ti.PKValues(o)[0])

		// if there's something in the slice, we add the NOT IN part,
		// otherwise we delete all of them (with the above existing where stipulation)
		if sliceV.Len() > 0 {
			stmt = stmt.Where(relv.SQLOtherIDField+" NOT IN ?", sliceV.Interface())
		}

		return stmt, nil

	}

	return nil, fmt.Errorf("unsupported relation type %T for DeleteRelationNotIn", rel)
}

// MustInsertRelationIgnore is the same as InsertRelationIgnore but panics on error.
func (b *Builder) MustInsertRelationIgnore(o interface{}, relationName string) *dbr.InsertStmt {
	ret, err := b.InsertRelationIgnore(o, relationName)
	if err != nil {
		panic(err)
	}
	return ret
}

// InsertRelationIgnore will make an insert statement for the records indicated
// by the given relation.  The relation must be of type BelongsToManyIDs.
// Will use InsertBySQL to generate a db-specific "insert with ignore" statement
// (unfortunately Sqlite3, MySQL and Postgres each require different syntaxes to achieve
// the same behavior).
// Note: (nil,nil) is a valid return in cases where the relation is an empty set,
// indicating that no insert is necessary.
func (b *Builder) InsertRelationIgnore(o interface{}, relationName string) (*dbr.InsertStmt, error) {

	ti := b.Meta.For(o)
	if ti == nil {
		return nil, ErrTypeNotRegistered
	}

	rel := ti.RelationNamed(relationName)
	if rel == nil {
		return nil, fmt.Errorf("relation %q not found", relationName)
	}

	switch relv := rel.(type) {
	case *tmeta.BelongsToManyIDs:

		joinTI := b.Meta.ForName(relv.JoinName)

		thisID := ti.PKValues(o)[0] // id for this table

		// get the slice of other ids
		vo := derefValue(reflect.ValueOf(o))
		sliceV := derefValue(vo.FieldByName(relv.GoValueField))

		if sliceV.Len() == 0 {
			return nil, nil
		}

		// build a buffer with the SQL values placeholders, and also the args to pass
		var buf bytes.Buffer
		var args []interface{}
		for i := 0; i < sliceV.Len(); i++ {
			buf.WriteString(`(?,?),`)
			elV := derefValue(sliceV.Index(i))
			args = append(args, thisID)
			args = append(args, elV.Interface())
		}
		var valueStr = strings.TrimSuffix(buf.String(), ",")

		// don't we just love random syntax differences between sql dialects...
		switch b.dbrDialect() {

		case dialect.SQLite3:

			return b.Session.InsertBySql(
					`INSERT OR IGNORE INTO `+joinTI.SQLName()+
						`(`+relv.SQLIDField+`,`+relv.SQLOtherIDField+`)`+
						` VALUES `+valueStr, args...),
				nil

		case dialect.MySQL:

			return b.Session.InsertBySql(
					`INSERT IGNORE INTO `+joinTI.SQLName()+
						`(`+relv.SQLIDField+`,`+relv.SQLOtherIDField+`)`+
						` VALUES `+valueStr, args...),
				nil

		case dialect.PostgreSQL:

			return b.Session.InsertBySql(
					`INSERT INTO `+joinTI.SQLName()+
						`(`+relv.SQLIDField+`,`+relv.SQLOtherIDField+`)`+
						` VALUES `+valueStr+` ON CONFLICT DO NOTHING`, args...),
				nil

		}

		return nil, fmt.Errorf("unknown dialect %#v", b.dbrDialect())

	}

	return nil, fmt.Errorf("unsupported relation type %T for DeleteRelationNotIn", rel)

}

// Execer interface for database things that can be Exec()ed
type Execer interface {
	Exec() (sql.Result, error)
}

// Execer interface for database things that can be ExecContext()ed
type ExecContexter interface {
	ExecContext(ctx context.Context) (sql.Result, error)
}

// ExecOK is an alias for Exec and discard result, just return the error.
// If execer is nil then it's a no-op and nil error is returned.
func (b *Builder) ExecOK(execer Execer) error {
	if execer == nil {
		return nil
	}
	// funky case where interface has pointer type but nil value, annoying but easy enough to check for
	ev := reflect.ValueOf(execer)
	if ev.Kind() == reflect.Ptr && ev.Pointer() == 0 {
		return nil
	}
	_, err := execer.Exec()
	return err
}

// ExecContextOK is an alias for ExecContext and discard result, just return the error.
// If execer is nil then it's a no-op and nil error is returned.
func (b *Builder) ExecContextOK(ctx context.Context, execContexter ExecContexter) error {
	if execContexter == nil {
		return nil
	}
	// funky case where interface has pointer type but nil value, annoying but easy enough to check for
	ev := reflect.ValueOf(execContexter)
	if ev.Kind() == reflect.Ptr && ev.Pointer() == 0 {
		return nil
	}
	_, err := execContexter.ExecContext(ctx)
	return err
}

// ResultOK accepts a result and an error and just returns the error.
// This is just a helper to more easily check "did this query succeed" when
// you don't care about rows affected or last insert ID.
func (b *Builder) ResultOK(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	return nil
}

// ResultWithOneUpdate is a helper to handle the result of an update.
// If err is non-nil it will be returned.  The RowsAffected value is
// checked an if not equal to 1 then ErrUpdateFailed is returned.
// (This call also works for inserts if you don't need the last insert ID,
// in which case use ResultWithInsertID).
//
// Note for MySQL: RowsAffected can return different values based on
// the connection string provided, possibly leading to unexpected behavior
// in cases where an update succeeds but no actual data is changed
// (i.e. updating a record without any version column or timestamps and all
// fields the same).
// See https://github.com/go-sql-driver/mysql#clientfoundrows and
// consider setting clientFoundRows=true to avoid this problem.
func (b *Builder) ResultWithOneUpdate(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrUpdateFailed
	}
	return nil
}

// ResultWithInsertID is a helper to handle the result of an insert.
// If err is non-nil it will be returned.  If the object provided
// has an auto increment primary key, then LastInsertId is used used
// to populate it.
func (b *Builder) ResultWithInsertID(o interface{}, res sql.Result, err error) error {
	if err != nil {
		return err
	}

	ti := b.Meta.For(o)
	if ti == nil {
		return ErrTypeNotRegistered
	}

	if ti.PKAutoIncr() {

		if len(ti.GoPKFields()) != 1 {
			return fmt.Errorf("cannot load last insert ID because number of PK fields for %T is %d, needs to be exactly 1",
				o, len(ti.GoPKFields()))
		}

		vo := reflect.ValueOf(o)
		pkf := vo.FieldByNameFunc(func(n string) bool {
			return n == ti.GoPKFields()[0]
		})

		id, err := res.LastInsertId()
		if err != nil {
			return err
		}

		pkf.SetInt(id)

	}

	return nil

}

// sess.DeleteJoinStringNotIn("book_category", "book_id", bookID, "category_id", categoryIDs...)
// sess.UpsertJoinString("book_category", "book_id", bookID, "category_id", categoryIDs...)

// TODO

// "ATTACHING" DONE
// SYNCING JOIN TABLE IDS DONE
// LOADING NAMED RELATIONS (WITH WHERE...) DONE

// auto increment ids DONE
// optimistic locking DONE
// update with difference... - and can we optimistic lock and merge non-conflicting changes???
