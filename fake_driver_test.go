package dbh

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
)

// fakeDB is a minimal in-memory database/sql driver for the tests: it records
// what it is asked to do and answers queries with one canned row. No real
// database and no third-party driver are involved.
type fakeDB struct {
	mu sync.Mutex

	execs     []string         // statements run through Exec
	execArgs  [][]driver.Value // their arguments
	queries   []string         // statements run through Query
	begins    int
	commits   int
	rollbacks int

	lastInsertID int64          // returned by every Exec result
	execErr      error          // returned by every Exec, if set
	columns      []string       // columns of the canned row
	row          []driver.Value // the canned row; nil means no rows
}

func (f *fakeDB) open() *sql.DB { return sql.OpenDB(fakeConnector{f}) }

type fakeConnector struct{ db *fakeDB }

func (c fakeConnector) Connect(context.Context) (driver.Conn, error) { return &fakeConn{c.db}, nil }
func (c fakeConnector) Driver() driver.Driver                        { return fakeDriver{} }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake driver: use the connector")
}

type fakeConn struct{ db *fakeDB }

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) { return &fakeStmt{c.db, query}, nil }
func (c *fakeConn) Close() error                              { return nil }

func (c *fakeConn) Begin() (driver.Tx, error) {
	c.db.mu.Lock()
	defer c.db.mu.Unlock()
	c.db.begins++
	return &fakeTx{c.db}, nil
}

type fakeTx struct{ db *fakeDB }

func (t *fakeTx) Commit() error {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	t.db.commits++
	return nil
}

func (t *fakeTx) Rollback() error {
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	t.db.rollbacks++
	return nil
}

type fakeStmt struct {
	db    *fakeDB
	query string
}

func (s *fakeStmt) Close() error  { return nil }
func (s *fakeStmt) NumInput() int { return -1 }

func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	if s.db.execErr != nil {
		return nil, s.db.execErr
	}
	s.db.execs = append(s.db.execs, s.query)
	s.db.execArgs = append(s.db.execArgs, args)
	return fakeResult{s.db.lastInsertID}, nil
}

func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.db.mu.Lock()
	defer s.db.mu.Unlock()
	s.db.queries = append(s.db.queries, s.query)
	return &fakeRows{columns: s.db.columns, row: s.db.row}, nil
}

type fakeResult struct{ id int64 }

func (r fakeResult) LastInsertId() (int64, error) { return r.id, nil }
func (r fakeResult) RowsAffected() (int64, error) { return 1, nil }

type fakeRows struct {
	columns []string
	row     []driver.Value
	done    bool
}

func (r *fakeRows) Columns() []string { return r.columns }
func (r *fakeRows) Close() error      { return nil }

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.done || r.row == nil {
		return io.EOF
	}
	copy(dest, r.row)
	r.done = true
	return nil
}
