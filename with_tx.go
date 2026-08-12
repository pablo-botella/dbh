package dbh

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrAbort is a sentinel error that callers can return from a WithTx callback to
// signal that the transaction should be rolled back without being treated as a
// real error upstream. The outer caller is expected to detect it with
// errors.Is(err, dbh.ErrAbort) and handle it as a controlled abort (e.g. show
// validation warnings instead of a 500).
var ErrAbort = errors.New("dbh: transaction aborted")

// WithTx runs fn inside a transaction. If sqx is already a *sql.Tx, fn runs
// in that existing transaction. Otherwise sqx must support Begin().
func WithTx(sqx SqlExecutor, fn func(tx *sql.Tx) error) (err error) {
	if tx, ok := sqx.(*sql.Tx); ok {
		return fn(tx)
	}

	b, ok := sqx.(Beginner)
	if !ok {
		return fmt.Errorf("WithTx: %T does not support Begin()", sqx)
	}

	tx, err := b.Begin()
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		rbErr := tx.Rollback()
		if rbErr == nil || errors.Is(rbErr, sql.ErrTxDone) {
			return
		}
		// if there was already an error (from fn or commit), enrich it
		if err != nil {
			err = fmt.Errorf("%w (rollback failed: %v)", err, rbErr)
		} else {
			// rare case: no prior error but the rollback failed
			err = fmt.Errorf("transaction rollback failed: %w", rbErr)
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	committed = true
	return nil
}
