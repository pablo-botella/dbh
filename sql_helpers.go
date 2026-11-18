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
