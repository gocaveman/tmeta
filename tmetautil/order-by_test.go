package tmetautil

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrderBy(t *testing.T) {

	assert := assert.New(t)

	o := OrderBy{}
	assert.NoError(json.Unmarshal([]byte(`"f1"`), &o))
	assert.Equal("f1", o.Field)
	assert.False(o.Desc)

	o = OrderBy{}
	assert.NoError(json.Unmarshal([]byte(`{"field":"f1","desc":true}`), &o))
	assert.Equal("f1", o.Field)
	assert.True(o.Desc)

	ol := make(OrderByList, 0, 1)
	assert.NoError(json.Unmarshal([]byte(`"f1"`), &ol))
	assert.Len(ol, 1)
	assert.Equal("f1", ol[0].Field)
	assert.False(ol[0].Desc)

	ol = make(OrderByList, 0, 1)
	assert.NoError(json.Unmarshal([]byte(`{"field":"f1","desc":true}`), &ol))
	assert.Len(ol, 1)
	assert.Equal("f1", ol[0].Field)
	assert.True(ol[0].Desc)

	ol = make(OrderByList, 0, 1)
	assert.NoError(json.Unmarshal([]byte(`[{"field":"f1","desc":true}]`), &ol))
	assert.Len(ol, 1)
	assert.Equal("f1", ol[0].Field)
	assert.True(ol[0].Desc)

	ol = make(OrderByList, 0, 1)
	assert.NoError(json.Unmarshal([]byte(`[{"field":"f1","desc":true},{"field":"f2"}]`), &ol))
	assert.Len(ol, 2)
	assert.Equal("f1", ol[0].Field)
	assert.Equal("f2", ol[1].Field)
	assert.True(ol[0].Desc)
	assert.False(ol[1].Desc)

	assert.NoError(ol.CheckFieldNames("f1", "f2", "f3"))
	assert.Error(ol.CheckFieldNames("f5", "f2"))

}
