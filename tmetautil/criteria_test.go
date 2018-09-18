package tmetautil

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCriteria(t *testing.T) {

	assert := assert.New(t)

	var ca Criteria

	ca = append(ca, Criterion{
		Field: "thefield",
		Op:    EqOp,
		Value: "Testing",
	})

	// ContainsMatch
	assert.True(ca.ContainsMatch(3, "thefield"))

	s, a, err := ca.SQL()
	assert.NoError(err)
	t.Logf("s: %s", s)
	t.Logf("a: %#v", a)

	// two fields
	ca = Criteria{}
	assert.NoError(json.Unmarshal([]byte(`
[
	{"field":"f1","op":"=","value":"tacos"},
	{"field":"f2","op":">","value":7}
]
`), &ca))
	s, a, err = ca.SQL()
	t.Logf("s=%q, a=%+v, err=%v", s, a, err)
	assert.NoError(err)
	assert.Len(a, 2)
	assert.Equal("tacos", a[0])
	assert.Equal(float64(7), a[1])
	assert.Equal(`f1 = ? AND f2 > ?`, s)

	// like
	ca = Criteria{}
	assert.NoError(json.Unmarshal([]byte(`
[
	{"field":"f1","op":"=","value":"tacos"},
	{"field":"f2","op":">","value":7},
	{"field":"f3","op":"like","value":"ab%"}
]
`), &ca))
	s, a, err = ca.SQL()
	t.Logf("s=%q, a=%+v, err=%v", s, a, err)
	assert.NoError(err)
	assert.Len(a, 3)
	assert.Equal("tacos", a[0])
	assert.Equal(float64(7), a[1])
	assert.Equal("ab%", a[2])
	assert.Equal(`f1 = ? AND f2 > ? AND f3 like ?`, s)

	// in
	ca = Criteria{}
	assert.NoError(json.Unmarshal([]byte(`
[
	{"field":"f1","op":"in","value":["asada","pollo","lengua"]}
]
`), &ca))
	s, a, err = ca.SQL()
	t.Logf("s=%q, a=%+v, err=%v", s, a, err)
	assert.NoError(err)
	assert.Len(a, 1)
	assert.Equal(`f1 in ?`, s)

	// or with 1
	ca = Criteria{}
	assert.NoError(json.Unmarshal([]byte(`
[{"or":[
	{"field":"f1","op":"=","value":"tacos"}
]}]
`), &ca))
	s, a, err = ca.SQL()
	t.Logf("s=%q, a=%+v, err=%v", s, a, err)
	assert.NoError(err)
	assert.Len(a, 1)
	assert.Equal(`(f1 = ?)`, s)

	// or with multiple
	ca = Criteria{}
	assert.NoError(json.Unmarshal([]byte(`
[{"or":[
	{"field":"f1","op":"=","value":"tacos"},
	{"field":"f2","op":">","value":7}
]}]
`), &ca))
	s, a, err = ca.SQL()
	t.Logf("s=%q, a=%+v, err=%v", s, a, err)
	assert.NoError(err)
	assert.Len(a, 2)
	assert.Equal(`(f1 = ? OR f2 > ?)`, s)

	// empty input
	ca = Criteria{}
	s, a, err = ca.SQL()
	t.Logf("s=%q, a=%+v, err=%v", s, a, err)
	assert.NoError(err)
	assert.Len(a, 0)
	assert.Equal(``, s)

}
