package tmeta

import (
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

func pkFromObj(ti *TableInfo, obj interface{}) ([]interface{}, error) {
	var ret []interface{}
	// v := derefValue(reflect.ValueOf(obj))
	for _, kname := range ti.KeySQLFields {
		fv, err := fieldValue(obj, kname)
		if err != nil {
			return nil, err
		}
		ret = append(ret, fv)
	}
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

var ErrNoField = fmt.Errorf("field not found")
var ErrNoTable = fmt.Errorf("table not found for object/type")

type fieldIndexCacheKey struct {
	Type         reflect.Type
	SQLFieldName string
}

var fieldIndexCache = make(map[fieldIndexCacheKey]int, 16)
var fieldIndexMutex sync.Mutex

func fieldIndex(obj interface{}, sqlFieldName string) (out int, rete error) {

	t := derefType(reflect.TypeOf(obj))

	fieldIndexMutex.Lock()
	ret, ok := fieldIndexCache[fieldIndexCacheKey{t, sqlFieldName}]
	fieldIndexMutex.Unlock()
	if ok {
		return ret, nil
	}

	// record result in cache if not error
	defer func() {
		if rete == nil {
			fieldIndexMutex.Lock()
			fieldIndexCache[fieldIndexCacheKey{t, sqlFieldName}] = out
			fieldIndexMutex.Unlock()
		}
	}()

	for j := 0; j < t.NumField(); j++ {
		f := t.Field(j)
		dbName := strings.SplitN(f.Tag.Get("db"), ",", 2)[0]
		// explicitly skip "-" db tags
		if dbName == "-" {
			continue
		}
		if dbName == sqlFieldName {
			return j, nil
		}
		dbName = camelToSnake(f.Name)
		if dbName == sqlFieldName {
			return j, nil
		}
	}

	return -1, ErrNoField

}

func fieldValue(obj interface{}, sqlFieldName string) (interface{}, error) {

	i, err := fieldIndex(obj, sqlFieldName)
	if err != nil {
		return nil, err
	}

	v := derefValue(reflect.ValueOf(obj))
	return v.Field(i).Interface(), nil

}

// Shamelessly borrowed from: https://github.com/fatih/camelcase/blob/master/camelcase.go

// split splits the camelcase word and returns a list of words. It also
// supports digits. Both lower camel case and upper camel case are supported.
// For more info please check: http://en.wikipedia.org/wiki/CamelCase
//
// Examples
//
//   "" =>                     [""]
//   "lowercase" =>            ["lowercase"]
//   "Class" =>                ["Class"]
//   "MyClass" =>              ["My", "Class"]
//   "MyC" =>                  ["My", "C"]
//   "HTML" =>                 ["HTML"]
//   "PDFLoader" =>            ["PDF", "Loader"]
//   "AString" =>              ["A", "String"]
//   "SimpleXMLParser" =>      ["Simple", "XML", "Parser"]
//   "vimRPCPlugin" =>         ["vim", "RPC", "Plugin"]
//   "GL11Version" =>          ["GL", "11", "Version"]
//   "99Bottles" =>            ["99", "Bottles"]
//   "May5" =>                 ["May", "5"]
//   "BFG9000" =>              ["BFG", "9000"]
//   "BöseÜberraschung" =>     ["Böse", "Überraschung"]
//   "Two  spaces" =>          ["Two", "  ", "spaces"]
//   "BadUTF8\xe2\xe2\xa1" =>  ["BadUTF8\xe2\xe2\xa1"]
//
// Splitting rules
//
//  1) If string is not valid UTF-8, return it without splitting as
//     single item array.
//  2) Assign all unicode characters into one of 4 sets: lower case
//     letters, upper case letters, numbers, and all other characters.
//  3) Iterate through characters of string, introducing splits
//     between adjacent characters that belong to different sets.
//  4) Iterate through array of split strings, and if a given string
//     is upper case:
//       if subsequent string is lower case:
//         move last character of upper case string to beginning of
//         lower case string
func split(src string) (entries []string) {
	// don't split invalid utf8
	if !utf8.ValidString(src) {
		return []string{src}
	}
	entries = []string{}
	var runes [][]rune
	lastClass := 0
	class := 0
	// split into fields based on class of unicode character
	for _, r := range src {
		switch true {
		case unicode.IsLower(r):
			class = 1
		case unicode.IsUpper(r):
			class = 2
		case unicode.IsDigit(r):
			class = 3
		default:
			class = 4
		}
		if class == lastClass {
			runes[len(runes)-1] = append(runes[len(runes)-1], r)
		} else {
			runes = append(runes, []rune{r})
		}
		lastClass = class
	}
	// handle upper case -> lower case sequences, e.g.
	// "PDFL", "oader" -> "PDF", "Loader"
	for i := 0; i < len(runes)-1; i++ {
		if unicode.IsUpper(runes[i][0]) && unicode.IsLower(runes[i+1][0]) {
			runes[i+1] = append([]rune{runes[i][len(runes[i])-1]}, runes[i+1]...)
			runes[i] = runes[i][:len(runes[i])-1]
		}
	}
	// construct []string from results
	for _, s := range runes {
		if len(s) > 0 {
			entries = append(entries, string(s))
		}
	}
	return
}

func camelToSnake(s string) string {
	parts := split(s)
	for i := 0; i < len(parts); i++ {
		parts[i] = strings.ToLower(parts[i])
	}
	return strings.Join(parts, "_")
}

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

func structTagToValues(st string) url.Values {

	ret := make(url.Values)

	parts := strings.Split(st, ",")

	for _, part := range parts {
		kvparts := strings.SplitN(part, "=", 2)
		if len(kvparts) < 2 {
			ret.Set(kvparts[0], "")
		} else {
			ret.Set(kvparts[0], kvparts[1])
		}
	}

	return ret
}
