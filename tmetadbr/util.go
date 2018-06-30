package tmetadbr

import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gocaveman/tmeta"
)

// CreateTimeToucher can be implemented by objects to be notified when their create time should be set to "now".
type CreateTimeToucher interface {
	CreateTimeTouch()
}

// // checks to see if v implements or if it's pointer does and calls if so, returns true if it worked
// func invokeCreateTimeTouch(v interface{}) bool {
// 	t := reflect.TypeOf(v)
// 	if t.Kind() != reflect.Ptr { // get pointer if it's not already
// 		v = reflect.ValueOf(v).Addr().Interface()
// 	}
// 	if ctt, ok := v.(CreateTimeToucher); ok {
// 		ctt.CreateTimeTouch()
// 		return true
// 	}
// 	return false
// }

// UpdateTimeToucher can be implemented by objects to be notified when their update time should be set to "now".
type UpdateTimeToucher interface {
	UpdateTimeTouch()
}

// // checks to see if v implements or if it's pointer does and calls if so, returns true if it worked
// func invokeUpdateTimeTouch(v interface{}) bool {
// }

// UUIDV4Generator implements IDGenerator and will populate string PK fields with a version 4 UUID.
func UUIDV4Generator(meta *tmeta.Meta, obj interface{}) error {

	ti := meta.For(obj)
	if ti == nil {
		return ErrTypeNotRegistered
	}

	// no action if auto increment
	if ti.PKAutoIncr() {
		return nil
	}

	v := reflect.ValueOf(obj)
	for v.Kind() == reflect.Ptr {
		v = v.Elem() // deref
	}

	goPKFields := ti.GoPKFields()
	for _, f := range goPKFields {
		sf, ok := ti.GoType().FieldByNameFunc(func(fn string) bool { return fn == f })
		if !ok {
			return fmt.Errorf("tmetadbr: unable to find Go field %q", f)
		}
		vsf := v.FieldByIndex(sf.Index)
		if vsf.Kind() != reflect.String {
			return fmt.Errorf("unable to populate primary key of type: %T", vsf.Interface())
		}
		u, err := uuidv4()
		if err != nil {
			return err
		}
		vsf.SetString(u)
	}

	return nil
}

func uuidv4() (string, error) {

	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	ret := fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])

	return ret, nil
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

func derefValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return v
}

// like derefType but will take a slice look at it's element type,
// non-slice types are treated the same as by derefType
func elemDerefType(t reflect.Type) reflect.Type {
	t = derefType(t)
	if t.Kind() == reflect.Slice {
		t = derefType(t.Elem())
	}
	return t
}

type errorWithCode struct {
	code int
	msg  string
}

func (ec *errorWithCode) Error() string { return ec.msg }
func (ec *errorWithCode) Code() int     { return ec.code }

// walks all exported fields, including embedded anonymous structs and returns a slice
// of index slices for use with reflect.Type.FieldByIndex
func exportedFieldIndexes(t reflect.Type) (ret [][]int) {

	l := t.NumField()
	for i := 0; i < l; i++ {

		f := t.Field(i)

		// skip unexported fields
		if f.PkgPath != "" {
			continue
		}

		// for anonymous structs, we recurse into them
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			inner := exportedFieldIndexes(f.Type)
			for _, iv := range inner {
				// prepend i
				iv2 := append([]int(nil), i)
				iv2 = append(iv2, iv...)
				ret = append(ret, iv2)
			}
			continue
		}

		// otherwise we add to our result
		ret = append(ret, f.Index)

	}

	return
}

type sqlFieldIndexCacheKey struct {
	T reflect.Type
	F string
}

var sqlFieldIndexCacheMU sync.RWMutex
var sqlFieldIndexCache = make(map[sqlFieldIndexCacheKey][]int)

func sqlFieldIndex(t reflect.Type, sqlFieldName string) []int {

	sqlFieldIndexCacheMU.RLock()
	ret, ok := sqlFieldIndexCache[sqlFieldIndexCacheKey{T: t, F: sqlFieldName}]
	sqlFieldIndexCacheMU.RUnlock()

	if ok {
		return ret
	}

	sqlFieldIndexCacheMU.Lock()
	defer sqlFieldIndexCacheMU.Unlock()

	for _, idx := range exportedFieldIndexes(t) {
		sf := t.FieldByIndex(idx)
		sfdb := strings.SplitN(sf.Tag.Get("db"), ",", 2)[0]
		if sfdb == "" || sfdb == "-" {
			continue
		}
		if sfdb == sqlFieldName {
			ret = idx
			break
		}
	}

	// write result to cache (might be nil, but that's okay to cache also)
	sqlFieldIndexCache[sqlFieldIndexCacheKey{T: t, F: sqlFieldName}] = ret

	return ret
}

func sqlFieldValue(v reflect.Value, sqlFieldName string) interface{} {

	t := v.Type()
	idx := sqlFieldIndex(t, sqlFieldName)
	if idx == nil {
		return nil
	}

	f := v.FieldByIndex(idx)
	return f.Interface()
}

func isZero(x interface{}) bool {
	return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

func stringsAddPrefix(slist []string, prefix string) []string {
	ret := make([]string, 0, len(slist))
	for _, s := range slist {
		ret = append(ret, prefix+s)
	}
	return ret
}

func incrementInteger(v interface{}) (interface{}, error) {

	vv := reflect.ValueOf(v)
	vt := vv.Type()

	switch vt.Kind() {
	case reflect.Int:
		vv.Set(reflect.ValueOf(vv.Interface().(int) + 1))
	case reflect.Uint:
		vv.Set(reflect.ValueOf(vv.Interface().(uint) + 1))
	case reflect.Int32:
		vv.Set(reflect.ValueOf(vv.Interface().(int32) + 1))
	case reflect.Uint32:
		vv.Set(reflect.ValueOf(vv.Interface().(uint32) + 1))
	case reflect.Int64:
		vv.Set(reflect.ValueOf(vv.Interface().(int64) + 1))
	case reflect.Uint64:
		vv.Set(reflect.ValueOf(vv.Interface().(uint64) + 1))
	}

	return nil, fmt.Errorf("%T is not a supported integer type", v)
}

// printEventReceiver writes to anything that implements printer.
// For example a *log.Logger
type printEventReceiver struct {
	printer
}

// printer interface matches log.Print and implementations should behave in a compatible manner.
type printer interface {
	Print(v ...interface{})
}

// newPrintEventReceiver creates an instance that prints to the printer you provide.
// Passing nil will use a log.Logger that writes to os.Stderr.
func newPrintEventReceiver(p printer) *printEventReceiver {
	if p == nil {
		p = log.New(os.Stderr, "", log.LstdFlags)
	}
	return &printEventReceiver{
		printer: p,
	}
}

// Event receives a simple notification when various events occur.
func (r *printEventReceiver) Event(eventName string) {
	r.Print(eventName)
}

// EventKv receives a notification when various events occur along with
// optional key/value data.
func (r *printEventReceiver) EventKv(eventName string, kvs map[string]string) {
	r.Print(eventName, ": ", kvs)
}

// EventErr receives a notification of an error if one occurs.
func (r *printEventReceiver) EventErr(eventName string, err error) error {
	r.Print(eventName, ", err: ", err)
	return err
}

// EventErrKv receives a notification of an error if one occurs along with
// optional key/value data.
func (r *printEventReceiver) EventErrKv(eventName string, err error, kvs map[string]string) error {
	r.Print(eventName, ": ", kvs, ", err: ", err)
	return err
}

// Timing receives the time an event took to happen.
func (r *printEventReceiver) Timing(eventName string, nanoseconds int64) {
	r.Print(eventName, ": timing: ", time.Duration(nanoseconds))
}

// TimingKv receives the time an event took to happen along with optional key/value data.
func (r *printEventReceiver) TimingKv(eventName string, nanoseconds int64, kvs map[string]string) {
	r.Print(eventName, ": ", kvs, ": timing: ", time.Duration(nanoseconds))

}
