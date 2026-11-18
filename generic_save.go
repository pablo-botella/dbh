package dbh

import (
	"fmt"
	"reflect"
	"strings"
)

// BuildMap returns the column→value map for a model struct, applying every
// per-column transform (trim, case, truncate, validate, zero_to_null, type
// coercion) through GetSafeWriteValue. It lets the Save method of a model be
// a one-liner:
//
//	return m.Meta().GenericSave(sqx, &m.Id, m.Meta().BuildMap(m), fnames...)
//
// m must be a pointer to a struct whose exported field names match
// TableMetaField.GoField for each writable column.
func (t TableMeta) BuildMap(m any) map[string]any {
	v := reflect.ValueOf(m)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	out := make(map[string]any, len(t.Fields))
	for _, f := range t.Fields {
		if f.IsPrimaryKey || f.IsIgnore {
			continue
		}
		fv := v.FieldByName(f.GoField)
		if !fv.IsValid() {
			continue
		}
		raw := normalizeReflectValue(fv)
		sw, _ := t.GetSafeWriteValue(f.Name, raw)
		out[f.Name] = sw.Value
	}
	return out
}

// normalizeReflectValue converts a reflect.Value to a native Go scalar, so
// the type switches in toInt64 and toString recognise it (a named integer
// type becomes int64, for instance).
func normalizeReflectValue(fv reflect.Value) any {
	switch fv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(fv.Uint())
	case reflect.String:
		return fv.String()
	case reflect.Bool:
		return fv.Bool()
	case reflect.Float32, reflect.Float64:
		return fv.Float()
	default:
		return fv.Interface()
	}
}

// GenericSave performs INSERT (when *idRef == -1) or UPDATE on t.Table using
// the values in data. On INSERT it stores the new id in *idRef. The column
// order is taken from t.Fields (writable, non-PK). If fnames[0] is non-empty,
// it overrides the column set with a comma-separated subset (each name must
// exist in data).
//
// The map is expected to come from BuildMap, which already applies the
// column transforms through GetSafeWriteValue.
func (t TableMeta) GenericSave(sqx SqlExecutor, idRef *int64, data map[string]any, fnames ...string) error {
	var cols []string
	if len(fnames) > 0 && fnames[0] != "" {
		for _, c := range strings.Split(fnames[0], ",") {
			cols = append(cols, strings.TrimSpace(c))
		}
	} else {
		cols = t.WritableFieldNames()
	}

	vals := make([]any, len(cols))
	for i, c := range cols {
		v, ok := data[c]
		if !ok {
			return fmt.Errorf("%s: unknown field %q", t.Table, c)
		}
		vals[i] = v
	}

	if *idRef == -1 {
		placeholders := make([]string, len(cols))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		q := "INSERT INTO " + t.Table + " (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
		res, err := sqx.Exec(q, vals...)
		if err != nil {
			return err
		}
		*idRef, _ = res.LastInsertId()
		return nil
	}

	sets := make([]string, len(cols))
	for i, c := range cols {
		sets[i] = c + " = ?"
	}
	vals = append(vals, *idRef)
	q := "UPDATE " + t.Table + " SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	_, err := sqx.Exec(q, vals...)
	return err
}
