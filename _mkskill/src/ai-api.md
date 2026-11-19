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

- `SqlExecutor` is satisfied by `*sql.DB`, `*sql.Tx` and `*Dbh` alike —
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
- At this layer there is nothing else to construct: everything but the
  Null is contract, and byte-for-byte it is stdlib underneath.

## API — Dbh and the dialect

```go
type DbType int                       // dbh.SQLite | dbh.MySQL
func ParseDbType(s string) DbType     // "mysql" → MySQL; anything else → SQLite
func (t DbType) String() string       // "sqlite" | "mysql"

type Dbh struct {
    *sql.DB
    DbType DbType
}
func New(conn *sql.DB, dbType DbType) *Dbh
func (d *Dbh) IsMysql() bool
func (d *Dbh) GetDbType() DbType
func (d *Dbh) SqlConcat(a, b string) string        // a || b  |  CONCAT(a, b)
func (d *Dbh) WithTx(fn func(tx *Tx) error) error  // transaction that keeps the dialect

type HasDbType interface { GetDbType() DbType }    // capability probe
func DbTypeOf(sqx SqlExecutor) (DbType, bool)
type SqlExecutorWithDbType interface { SqlExecutor; HasDbType }  // *Dbh, *Tx

type Tx struct { *sql.Tx /* + the dialect, unexported */ }
func (t *Tx) GetDbType() DbType
func (t *Tx) WithTx(fn func(tx *Tx) error) error   // runs fn in the same transaction

func EscapeSqlQuotes(s string) string
func IsUniqueConstraintError(err error, column string) bool
func NullIfZero(v int64) any            // nil when 0, to store NULL
func NullIfEmptyStr(s string) any       // nil when "", to store NULL
```

- `Dbh` embeds `*sql.DB`: it is a `SqlExecutor`, a `Beginner` and a
  `Closeable`. It never opens a connection — `sql.Open` and the driver
  import stay in the caller.
- The dialect is lost across `Begin`: the package-level `WithTx` hands
  fn a plain `*sql.Tx`. When code inside the transaction needs the
  dialect, use the method `d.WithTx`, which hands a `*dbh.Tx`.
- Never pass a `*dbh.Tx` to the package-level `WithTx` — it only
  recognises `*sql.Tx` as an open transaction and would fail with
  "does not support Begin()". Use `tx.WithTx(fn)`, or `tx.Tx` to reach
  the standard transaction.
- Code that receives a bare `SqlExecutor` and needs the dialect probes
  it with `DbTypeOf`; `ok == false` means a plain `*sql.DB`/`*sql.Tx`.
- Code that cannot work without the dialect asks for
  `SqlExecutorWithDbType` in its signature instead of probing: `*Dbh`
  and `*dbh.Tx` satisfy it, plain `*sql.DB`/`*sql.Tx` fail to compile.
- `IsUniqueConstraintError` matches the error text of either dialect
  ("Duplicate entry" / "UNIQUE constraint failed"); an empty column
  matches any UNIQUE violation.
- `NullIfZero` / `NullIfEmptyStr` return `nil` for `0` / `""`, the value
  otherwise. Use them for optional columns (nullable foreign keys)
  instead of writing the `if` by hand; the result goes straight into a
  query argument or the `GenericSave` map.
