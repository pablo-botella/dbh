package dbh

import (
	"database/sql"
	"errors"
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

// NullSqlExecutor is the do-nothing SqlExecutor: a placeholder for the
// wirings where no database exists, failing with a clear error instead
// of a nil-pointer panic. Embed it to build partial executors that
// implement only what they need.
type NullSqlExecutor struct{}

func (n *NullSqlExecutor) Exec(query string, args ...any) (sql.Result, error) {
	return nil, errors.New("NullSqlExecutor: Exec not implemented")
}
func (n *NullSqlExecutor) QueryRow(query string, args ...any) *sql.Row {
	return &sql.Row{}
}
func (n *NullSqlExecutor) Query(query string, args ...any) (*sql.Rows, error) {
	return nil, errors.New("NullSqlExecutor: Query not implemented")
}
func (n *NullSqlExecutor) Prepare(query string) (*sql.Stmt, error) {
	return nil, errors.New("NullSqlExecutor: Prepare not implemented")
}
