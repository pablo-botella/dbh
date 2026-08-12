---
mkskill:
  pos: 210
  in: ai*
---

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

type NullSqlExecutor struct{}  // do-nothing SqlExecutor, embeddable
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
- `NullSqlExecutor` is the one concrete type: the do-nothing
  `SqlExecutor` (the NullWaaCtx pattern) for wirings without a database —
  `Exec`/`Query`/`Prepare` fail with a clear error instead of a
  nil-pointer panic. Embed it to implement only what you need. Caveat:
  `QueryRow` returns an empty `sql.Row` (stdlib gives no way to build one
  carrying a custom error), so prefer probing the error paths.
- Nothing else to construct: everything but the Null is contract, and
  byte-for-byte it is stdlib underneath.
