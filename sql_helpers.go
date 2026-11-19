package dbh

import "strings"

// EscapeSqlQuotes escapes single quotes for SQL by doubling them.
func EscapeSqlQuotes(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// IsUniqueConstraintError returns true when err corresponds to a UNIQUE
// constraint violation on the given column. It works for both MySQL and
// SQLite by inspecting the error message. An empty column matches any
// UNIQUE violation.
//
//	MySQL:  "Error 1062 (23000): Duplicate entry '...' for key 'table.column'"
//	SQLite: "UNIQUE constraint failed: table.column"
func IsUniqueConstraintError(err error, column string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	isUnique := strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "UNIQUE constraint failed")
	if !isUnique {
		return false
	}
	if column == "" {
		return true
	}
	return strings.Contains(msg, column)
}

// NullIfZero returns nil when v is 0 and v otherwise. Pass the result as
// a query argument or as a value of the GenericSave map to store NULL
// instead of 0, typically in an optional foreign key column.
func NullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// NullIfEmptyStr returns nil when s is the empty string and s otherwise,
// to store NULL instead of an empty text.
func NullIfEmptyStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
