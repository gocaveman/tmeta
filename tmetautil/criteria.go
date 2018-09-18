package tmetautil

import (
	"bytes"
	"fmt"
	"strings"
)

// Op is one of the supported SQL where operators used with Criteria and Criterion.
type Op string

const (
	EqOp   Op = "="
	NeOp   Op = "<>"
	LtOp   Op = "<"
	LteOp  Op = "<="
	GtOp   Op = ">"
	GteOp  Op = ">="
	LikeOp Op = "like"
	InOp   Op = "in"
)

// Criterion is an individual expression that has a field, an op(erator) and a value.
// It also supports Not for inverting the criterion, and Or can be used to provide
// a list of other expressions to be ORed together.
type Criterion struct {
	Not   bool        `json:"not"`
	Field string      `json:"field"`
	Op    Op          `json:"op"`
	Value interface{} `json:"value"`
	Or    Criteria    `json:"or"`
}

// CheckFieldNames returns an error if it encounters any field which is not in the list provided.
func (c Criterion) CheckFieldNames(fields ...string) error {

	err := c.Or.CheckFieldNames(fields...)
	if err != nil {
		return err
	}

	for _, f := range fields {
		if f == c.Field {
			return nil
		}
	}
	return fmt.Errorf("%q is not a valid field name", c.Field)
}

// ContainsMatch will look for any field in the given set which is "matched", meaning
// it's operator is any of the valid ones except LikeOp, which requires at least likePrefixLen
// characters at the start without a wildcard character ('%'' or '_').  The idea is to
// restrict queries to specific (usually indexed) fields to avoid excessive database load.
func (c Criterion) ContainsMatch(likePrefixLen int, fields ...string) bool {

	// for Or they must all have a match
	for _, ci := range c.Or {
		if !ci.ContainsMatch(likePrefixLen, fields...) {
			return false
		}
	}

	if c.Not {
		return false
	}
	matchField := func(f string) bool {
		for _, fi := range fields {
			if fi == f {
				return true
			}
		}
		return false
	}
	if !matchField(c.Field) {
		return false
	}
	if c.Op == LikeOp {
		s, _ := c.Value.(string)
		if len(s) < likePrefixLen {
			return false
		}
		ilen := len(s)
		if ilen > likePrefixLen {
			ilen = likePrefixLen
		}
		for i := 0; i < ilen; i++ {
			if s[i] == '%' || s[i] == '_' {
				return false
			}
		}
		return true
	}
	return true
}

// SQL converts to a SQL where clause and the corresponding arguments for it.
func (ca Criterion) SQL() (stmt string, args []interface{}, err error) {

	var buf bytes.Buffer

	if ca.Not {
		buf.WriteString("NOT ")
	}

	noOp := false
	switch ca.Op {
	case EqOp, NeOp, LtOp, LteOp, GtOp, GteOp, LikeOp,
		InOp: // IN operator happens to work the same due to dbr's handling of "x IN ?"
		buf.WriteString(ca.Field)
		buf.WriteString(" ")
		buf.WriteString(string(ca.Op))
		buf.WriteString(" ?")
		args = append(args, ca.Value)
	case Op(""):
		noOp = true
	default:
		return "", nil, fmt.Errorf("unknown operator %q", ca.Op)
	}

	if len(ca.Or) > 0 {
		var sl []string
		for _, ci := range ca.Or {
			s, a, err := ci.SQL()
			if err != nil {
				return "", nil, err
			}
			sl = append(sl, s)
			args = append(args, a...)
		}
		if buf.Len() > 0 {
			buf.WriteString(" AND ")
		}
		buf.WriteString("(")
		buf.WriteString(strings.Join(sl, " OR "))
		buf.WriteString(")")
	} else if noOp {
		return "", nil, nil
	}

	return buf.String(), args, nil

}

// Criteria is a list of Criterion.  If used as an Or (see Criterion struct definition) the various Criterions are ORed together, otherwise they are ANDed.
type Criteria []Criterion

// CheckFieldNames returns an error if it encounters any field which is not in the list provided.
func (ca Criteria) CheckFieldNames(fields ...string) error {
	for _, c := range ca {
		err := c.CheckFieldNames(fields...)
		if err != nil {
			return err
		}
	}
	return nil
}

// ContainsMatch calls ContainsMatch on each Criterion.
func (ca Criteria) ContainsMatch(likePrefixLen int, fields ...string) bool {

	// by default it's "and" any as long as one matches we consider the whole thing a match
	for _, ci := range ca {
		if ci.ContainsMatch(likePrefixLen, fields...) {
			return true
		}
	}

	return false

}

// SQL converts to a SQL where clause and the corresponding arguments for it.
func (ca Criteria) SQL() (stmt string, args []interface{}, err error) {

	var slist []string
	for _, c := range ca {
		s, a, err := c.SQL()
		if err != nil {
			return "", nil, err
		}
		slist = append(slist, s)
		args = append(args, a...)
	}

	return strings.Join(slist, " AND "), args, nil

}
