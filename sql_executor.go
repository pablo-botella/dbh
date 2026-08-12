package dbh

import (
	"database/sql"
)

// SqlExecutor abstracts *sql.DB and *sql.Tx so that functions
// can accept either of them uniformly.
type SqlExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Prepare(query string) (*sql.Stmt, error)
}

var _ SqlExecutor = (*sql.DB)(nil)
var _ SqlExecutor = (*sql.Tx)(nil)
var _ SqlExecutor = (*sql.Tx)(nil)
