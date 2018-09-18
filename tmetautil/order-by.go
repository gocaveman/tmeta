package tmetautil

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// OrderBy corresponds to a single field in an ORDER BY SQL clause including it's direction (ascending by default).
type OrderBy struct {
	Field string `json:"field"`
	Desc  bool   `json:"desc,omitempty"`
}

// CheckFieldNames returns true if the field for this OrderBy is not in the list provided.
func (o OrderBy) CheckFieldNames(fields ...string) error {
	for _, f := range fields {
		if f == o.Field {
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid field name", o.Field)
}

// UnmarshalJSON supports normal JSON unmarshaling plus a shorthand of just providing a string to mean the field name sorted ascending.
func (o *OrderBy) UnmarshalJSON(b []byte) error {

	b = bytes.TrimSpace(b)

	// if a single string is provided, we interpret this as: {"field":"THESTRING","desc":false}
	if len(b) > 0 && b[0] == '"' {
		err := json.Unmarshal(b, &o.Field)
		if err != nil {
			return err
		}
		o.Desc = false
		return nil
	}

	m := make(map[string]interface{}, 2)
	err := json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	o.Field, _ = m["field"].(string)
	o.Desc, _ = m["desc"].(bool)

	return nil
}

// OrderByList is a list of OrderBys.
type OrderByList []OrderBy

// CheckFieldNames returns true if any of the fields for this OrderByList are not in the list provided.
func (ol OrderByList) CheckFieldNames(fields ...string) error {
	for _, o := range ol {
		err := o.CheckFieldNames(fields...)
		if err != nil {
			return err
		}
	}
	return nil
}

// UnmarshalJSON supports normal JSON unmarshaling plus a shorthand of just providing either a single object
// instead of an array, or a string to mean the field name sorted ascending.
func (ol *OrderByList) UnmarshalJSON(b []byte) error {

	b = bytes.TrimSpace(b)

	// single value case
	if len(b) > 0 && (b[0] == '{' || b[0] == '"') {
		*ol = make(OrderByList, 1)
		return (&(*ol)[0]).UnmarshalJSON(b)
	}

	ms := make([]map[string]interface{}, 0, 1)
	err := json.Unmarshal(b, &ms)
	if err != nil {
		return err
	}

	*ol = make(OrderByList, len(ms))
	for i, m := range ms {
		(*ol)[i].Field, _ = m["field"].(string)
		(*ol)[i].Desc, _ = m["desc"].(bool)
	}

	return nil
}
