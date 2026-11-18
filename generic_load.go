package dbh

import (
	"strings"
)

// QueryById returns the row matching id as a column→value map, using
//
//	SELECT id, COALESCE(col, defval), ... FROM table WHERE id = ?
//
// derived from the metadata. Values come back as int64 for the numeric
// columns and string for the rest; each model hydrates its struct from the
// map. Pass field names to restrict the SELECT to a subset; leave empty for
// every column.
func (t TableMeta) QueryById(sqx SqlExecutor, id int64, fields ...string) (map[string]any, error) {
	return t.Query(sqx, "id = ?", []any{id}, fields...)
}

// Query is the more flexible sibling of QueryById: pass a WHERE clause
// (without the leading "WHERE" keyword), its args as a slice, and an optional
// field-name list to restrict the SELECT.
func (t TableMeta) Query(sqx SqlExecutor, where string, args []any, fields ...string) (map[string]any, error) {
	wanted := map[string]struct{}{}
	for _, f := range fields {
		wanted[f] = struct{}{}
	}
	cols := make([]string, 0, len(t.Fields))
	names := make([]string, 0, len(t.Fields))
	for _, f := range t.Fields {
		if f.IsIgnore {
			continue
		}
		if len(wanted) > 0 {
			if _, ok := wanted[f.Name]; !ok {
				continue
			}
		}
		names = append(names, f.Name)
		if f.IsPrimaryKey {
			cols = append(cols, f.Name)
		} else {
			cols = append(cols, "COALESCE("+f.Name+","+coalesceDefault(f.Type)+")")
		}
	}
	q := "SELECT " + strings.Join(cols, ", ") + " FROM " + t.Table + " WHERE " + where

	scanTargets := make([]any, len(names))
	raw := make([]typedScan, len(names))
	for i, n := range names {
		f := t.FieldByName(n)
		raw[i].setupFor(f.Type)
		scanTargets[i] = raw[i].targetPtr()
	}

	if err := sqx.QueryRow(q, args...).Scan(scanTargets...); err != nil {
		return nil, err
	}

	out := make(map[string]any, len(names))
	for i, n := range names {
		out[n] = raw[i].value()
	}
	return out, nil
}

// coalesceDefault returns the literal used in COALESCE for the given column
// type: 0 for numerics, the empty string for strings, dates and JSON.
func coalesceDefault(t ColumnType) string {
	switch t {
	case TypeBigInt, TypeInt, TypeTinyInt:
		return "0"
	default:
		return "''"
	}
}

// typedScan holds a typed receiver for one column, so Scan gets the right
// type and the map ends up with int64 or string instead of raw driver bytes.
type typedScan struct {
	kind ColumnType
	i64  int64
	str  string
}

func (s *typedScan) setupFor(t ColumnType) { s.kind = t }

func (s *typedScan) targetPtr() any {
	switch s.kind {
	case TypeBigInt, TypeInt, TypeTinyInt:
		return &s.i64
	default:
		return &s.str
	}
}

func (s *typedScan) value() any {
	switch s.kind {
	case TypeBigInt, TypeInt, TypeTinyInt:
		return s.i64
	default:
		return s.str
	}
}
