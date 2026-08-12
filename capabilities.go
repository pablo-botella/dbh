package dbh

import (
	"database/sql"
)

// Closeable is any executor that can be closed.
type Closeable interface {
	Close() error
}

// Beginner is any type that can start a transaction.
type Beginner interface {
	Begin() (*sql.Tx, error)
}
