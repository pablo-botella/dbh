package dbh

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// SafeWriteValue is the result of TableMeta.GetSafeWriteValue.
// Value is the cleaned and converted value, ready to pass to the driver.
// The other fields report what happened during the transformation.
type SafeWriteValue struct {
	Value      any
	Truncated  bool
	Converted  bool
	OutOfRange bool
	MaxSize    int
}

// GetSafeWriteValue applies the write transforms of the field to val and
// returns the result. It returns an error if the field does not exist on this
// table, or if a transform fails (e.g. an invalid date on a field without
// invalid_to_null).
func (t TableMeta) GetSafeWriteValue(name string, val any) (SafeWriteValue, error) {
	f := t.FieldByName(name)
	if f == nil {
		return SafeWriteValue{}, fmt.Errorf("unknown field: %s.%s", t.Table, name)
	}
	r := SafeWriteValue{Value: val, MaxSize: f.MaxSize}

	switch f.Type {
	case TypeVarchar, TypeText, TypeDate, TypeTimestamp, TypeJSON:
		s := toString(val)

		// Trim
		switch f.Trim {
		case TrimBoth:
			s = strings.TrimSpace(s)
		case TrimLeft:
			s = strings.TrimLeft(s, " \t\r\n")
		case TrimRight:
			s = strings.TrimRight(s, " \t\r\n")
		}

		// Case
		switch f.Case {
		case CaseLower:
			s = strings.ToLower(s)
		case CaseUpper:
			s = strings.ToUpper(s)
		}

		// Overflow (only meaningful for varchar)
		if f.Type == TypeVarchar && f.MaxSize > 0 {
			runes := []rune(s)
			if len(runes) > f.MaxSize {
				switch f.Overflow {
				case OverflowTruncate:
					s = string(runes[:f.MaxSize])
					r.Truncated = true
				case OverflowError:
					return r, fmt.Errorf("%s.%s exceeds max size %d", t.Table, name, f.MaxSize)
				}
			}
		}

		// Validations
		if f.ValidateJSON && s != "" {
			if !json.Valid([]byte(s)) {
				if f.InvalidToNull {
					r.Value = nil
					return r, nil
				}
				return r, fmt.Errorf("%s.%s: invalid JSON", t.Table, name)
			}
		}
		if f.ValidateDate && s != "" {
			if _, err := time.Parse("2006-01-02", s); err != nil {
				if f.InvalidToNull {
					r.Value = nil
					return r, nil
				}
				return r, fmt.Errorf("%s.%s: invalid date (want YYYY-MM-DD): %w", t.Table, name, err)
			}
		}
		if f.ValidateTime && s != "" {
			if _, err := time.Parse(time.RFC3339, s); err != nil {
				if f.InvalidToNull {
					r.Value = nil
					return r, nil
				}
				return r, fmt.Errorf("%s.%s: invalid timestamp (want RFC3339): %w", t.Table, name, err)
			}
		}

		// zero_to_null on a string: empty becomes NULL
		if f.ZeroToNull && s == "" {
			r.Value = nil
			return r, nil
		}
		r.Value = s

	case TypeBigInt, TypeInt, TypeTinyInt:
		// Handle bool input on tinyint columns.
		if f.Type == TypeTinyInt {
			if b, ok := val.(bool); ok {
				r.Converted = true
				if f.ZeroToNull && !b {
					r.Value = nil
					return r, nil
				}
				if b {
					r.Value = int64(1)
				} else {
					r.Value = int64(0)
				}
				return r, nil
			}
		}

		n, conv, err := toInt64(val)
		if err != nil {
			return r, fmt.Errorf("%s.%s: %w", t.Table, name, err)
		}
		r.Converted = conv

		// Range check for int (32-bit)
		if f.Type == TypeInt {
			i := int(n)
			if int64(i) != n {
				r.OutOfRange = true
			}
		}

		// zero_to_null on a numeric: 0 becomes NULL
		if f.ZeroToNull && n == 0 {
			r.Value = nil
			return r, nil
		}
		r.Value = n
	}

	return r, nil
}

// --- helpers -----------------------------------------------------------------

func toString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toInt64(val any) (int64, bool, error) {
	switch v := val.(type) {
	case int64:
		return v, false, nil
	case int:
		return int64(v), false, nil
	case int32:
		return int64(v), true, nil
	case float64:
		return int64(v), true, nil
	case string:
		if v == "" {
			return 0, true, nil
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false, fmt.Errorf("cannot parse %q as int64: %w", v, err)
		}
		return n, true, nil
	case nil:
		return 0, true, nil
	default:
		return 0, false, fmt.Errorf("unsupported type %T for int64 conversion", val)
	}
}
