# dbh — agent notes

Go package `github.com/pablo-botella/dbh`: a minimal database virtual
interface. Pure contracts — no concrete handle, no drivers, everything
stdlib. The only function is `WithTx`, which itself works strictly
against the contracts.

Layout — one file per piece:

```
sql_executor.go   SqlExecutor + the compile-time guards
capabilities.go   Closeable + Beginner (the capability probes)
with_tx.go        WithTx + ErrAbort
```

Driver subpackages are planned (see the repo issues); the root package
stays purely virtual on purpose.

## API

```go
type SqlExecutor interface {
    Exec(query string, args ...any) (sql.Result, error)
    QueryRow(query string, args ...any) *sql.Row
    Query(query string, args ...any) (*sql.Rows, error)
    Prepare(query string) (*sql.Stmt, error)
}

type Beginner interface {  // any executor that can start a transaction
    Begin() (*sql.Tx, error)
}

type Closeable interface { // any executor that can be closed
    Close() error
}

func WithTx(sqx SqlExecutor, fn func(tx *sql.Tx) error) error
var ErrAbort error  // sentinel: roll back without treating it as a failure
```

- `SqlExecutor` is satisfied by `*sql.DB` and `*sql.Tx` alike —
  compile-time guards (`var _ SqlExecutor = ...`) keep it that way. Write
  query code against it and the caller decides transaction or not.
- `Beginner` and `Closeable` are capability probes: consumers type-assert
  an executor against them (`sqx.(dbh.Closeable)`) to discover what it
  can do — a `*sql.Tx` is neither, a `*sql.DB` is both. dbh ships the
  contracts; closing policy belongs to the caller.
- `WithTx(sqx, fn)` runs fn inside a transaction: a `*sql.Tx` is reused
  as-is, anything else must be a `Beginner` (probed by assertion) or it
  errors. Commit on success, rollback on error; return `ErrAbort` from fn
  to roll back on purpose without it counting as a failure upstream
  (detect with `errors.Is(err, dbh.ErrAbort)`).
- There is no concrete type in the package: nothing to construct,
  nothing to import beyond the contracts. Byte-for-byte it is stdlib
  underneath.
