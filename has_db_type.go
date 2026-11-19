package dbh

import "database/sql"

// HasDbType is any executor that knows its database type.
type HasDbType interface {
	GetDbType() DbType
}

// DbTypeOf probes sqx for its database type. The second result is false when
// the executor does not carry one (a plain *sql.DB or *sql.Tx).
func DbTypeOf(sqx SqlExecutor) (DbType, bool) {
	if h, ok := sqx.(HasDbType); ok {
		return h.GetDbType(), true
	}
	return SQLite, false
}

// SqlExecutorWithDbType is an executor that also knows its database type:
// a *Dbh or a *Tx, never a plain *sql.DB or *sql.Tx. Ask for it in the
// signature of code that must emit dialect-specific SQL, and the compiler
// rejects an executor without a dialect instead of a check at run time.
type SqlExecutorWithDbType interface {
	SqlExecutor
	HasDbType
}

// Tx is a transaction that keeps the database type of the Dbh it came from.
//
// A plain *sql.Tx loses the dialect tag: Begin belongs to the standard
// library and knows nothing about it. Tx carries it along, so code running
// inside the transaction can still ask for the dialect.
//
// It embeds *sql.Tx, so it satisfies SqlExecutor. Start one with Dbh.WithTx.
type Tx struct {
	*sql.Tx
	dbType DbType
}

var _ SqlExecutor = (*Tx)(nil)
var _ HasDbType = (*Tx)(nil)
var _ SqlExecutorWithDbType = (*Tx)(nil)
var _ SqlExecutorWithDbType = (*Dbh)(nil)

// GetDbType returns the database type of the Dbh the transaction came from.
func (t *Tx) GetDbType() DbType {
	return t.dbType
}

// WithTx runs fn inside a transaction that keeps the dialect tag: commit on
// success, rollback on error, exactly like the package-level WithTx.
//
// Use this method, not the package-level WithTx, when the code inside the
// transaction needs the database type.
func (d *Dbh) WithTx(fn func(tx *Tx) error) error {
	return WithTx(d.DB, func(tx *sql.Tx) error {
		return fn(&Tx{Tx: tx, dbType: d.DbType})
	})
}

// WithTx runs fn in the transaction itself: it is already open, so there is
// no nesting and no commit here; the owner keeps control. It lets the same
// code run against a *Dbh or a *Tx.
//
// The package-level WithTx does not recognise a *Tx as an open transaction;
// inside one, call this method.
func (t *Tx) WithTx(fn func(tx *Tx) error) error {
	return fn(t)
}
