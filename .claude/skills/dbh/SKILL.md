---
name: dbh
description: "Minimal database handle for Go — Dbh couples a *sql.DB with its dialect tag, and SqlExecutor is the virtual contract satisfied by *sql.DB, *sql.Tx and *Dbh alike, so one signature runs the same queries inside and outside transactions. Use when working with github.com/pablo-botella/dbh or when Go code needs one contract over connections and transactions. Driver subpackages are planned."
---

# dbh — agent notes

Go package `github.com/pablo-botella/dbh`: a minimal database handle.
Three layers, each usable without the next, no drivers, everything
stdlib:

1. **Contracts** — `SqlExecutor`, the capability probes and `WithTx`.
2. **`Dbh`** — a `*sql.DB` with its dialect tag (SQLite or MySQL).
3. **`TableMeta`** — a table described as data: DDL for either dialect,
   generic insert/update, pre-write clean-up. Written by the
   `dbh-table-meta` generator from tagged model structs.

Layout — one file per piece:

```
sql_executor.go   SqlExecutor + the compile-time guards + NullSqlExecutor
capabilities.go   Closeable + Beginner (the capability probes)
with_tx.go        WithTx + ErrAbort
dbh_type.go       DbType, Dbh, New, IsMysql, GetDbType, SqlConcat
has_db_type.go    HasDbType, DbTypeOf, Tx, Dbh.WithTx, Tx.WithTx
sql_helpers.go    EscapeSqlQuotes, IsUniqueConstraintError
table_meta.go     TableMeta and its parts, HasMeta, Ignore, the enums
schema_build.go   BuildCreateTable, BuildIndexes, BuildAlterFKs, ColumnTypeDDL
generic_save.go   BuildMap, GenericSave
generic_load.go   Query, QueryById
safe_write.go     GetSafeWriteValue, SafeWriteValue
tool/             the generator as a library: tool.Run(tool.Options)
cmd/dbh-table-meta/   the command, a thin wrapper around tool.Run
```

The first three files are the published contracts: add next to them,
never change their signatures. Driver subpackages are planned (see the
repo issues); the package itself opens no connection.

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

type Tx struct { *sql.Tx /* + the dialect, unexported */ }
func (t *Tx) GetDbType() DbType
func (t *Tx) WithTx(fn func(tx *Tx) error) error   // runs fn in the same transaction

func EscapeSqlQuotes(s string) string
func IsUniqueConstraintError(err error, column string) bool
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
- `IsUniqueConstraintError` matches the error text of either dialect
  ("Duplicate entry" / "UNIQUE constraint failed"); an empty column
  matches any UNIQUE violation.

## API — TableMeta

```go
type TableMeta struct {
    Table        string
    Group        string                 // free label; dbh gives it no meaning
    Fields       []TableMetaField
    ExtraColumns []TableMetaExtraColumn // schema-only, no Go struct field
    Constraints  []string               // carried, NOT emitted by Build*
    Indexes      []TableMetaIndex
    ForeignKeys  []TableMetaFK
}
type HasMeta interface { Meta() TableMeta }
type Ignore struct{}                    // field type: schema-only column

func (t TableMeta) BuildCreateTable(dbType DbType) string
func (t TableMeta) BuildIndexes(dbType DbType) []string
func (t TableMeta) BuildAlterFKs(dbType DbType) []string  // MySQL only; nil on SQLite
func ColumnTypeDDL(t ColumnType, maxSize int, dbType DbType) string

func (t TableMeta) BuildMap(m any) map[string]any
func (t TableMeta) GenericSave(sqx SqlExecutor, idRef *int64, data map[string]any, fnames ...string) error
func (t TableMeta) QueryById(sqx SqlExecutor, id int64, fields ...string) (map[string]any, error)
func (t TableMeta) Query(sqx SqlExecutor, where string, args []any, fields ...string) (map[string]any, error)
func (t TableMeta) GetSafeWriteValue(name string, val any) (SafeWriteValue, error)
func (t TableMeta) FieldByName(name string) *TableMetaField
func (t TableMeta) WritableFieldNames() []string
```

- `Meta()` is **generated**, never hand-written: run `dbh-table-meta`
  after touching a model struct, its tags or its info JSON. Do not edit
  the generated files.
- A model is a struct named by a `//dbh:generate-table-meta-model
  <Struct>` comment in its file, plus a method
  `Dbh_generate_table_meta_model_info() string` returning ONE JSON string
  literal (the generator reads the source; it does not run it).
- `sql` tag tokens (space-separated): a type — `bigint` `int` `tinyint`
  `text` `date` `timestamp` `json` `varchar(N)` — and `pk`, `autoinc`,
  `not_null`, `unique`, `default:X`, `name:X`. No tag = not a column.
- `write` tag tokens (comma-separated): `trim` `trim_left` `trim_right`
  `upper` `lower` `truncate` `error_on_overflow` `zero_to_null`
  `validate_date` `validate_time` `validate_json` `invalid_to_null`.
- Info JSON keys: `Table` (required), `Group`, `Indexes`, `ForeignKeys`,
  `ExtraColumns`, `Constraints`. Unknown keys and unknown tag tokens are
  generator errors, not silent skips.
- The canonical Save is one line:
  `return m.Meta().GenericSave(sqx, &m.Id, m.Meta().BuildMap(m), fnames...)`.
  `*idRef == -1` inserts and stores the new id; otherwise it updates
  `WHERE id = ?`. `fnames[0]` may carry a comma-separated column subset.
- Schema creation order: for every model `BuildCreateTable` then
  `BuildIndexes`; then, on MySQL only, a second pass with
  `BuildAlterFKs` once every table exists. SQLite inlines its foreign
  keys in `CREATE TABLE`.
- `Query`/`QueryById` return ONE row as a map (int64 for numeric
  columns, string for the rest, NULL coalesced to 0 or ""). Lists and
  struct scanning are hand-written per model.
- Limits: it creates, it does not migrate (`CREATE TABLE IF NOT
  EXISTS`); identifiers are not quoted; `GenericSave` assumes a single
  integer key named `id`; the Build* methods take a `DbType`, never a
  dialect string.

## The generator

```
dbh-table-meta [-dir .] [-meta file] [-list file] [-list-var AllMdls]
               [-sql folder] [-dialects sqlite,mysql] [-group name]
```

- Each output is written only when its option is given; at least one of
  `-meta`, `-list`, `-sql` is required.
- `-meta` and `-list` are relative to `-dir`; `-sql` to the current
  directory. SQL files are `<dialect>.sql`, or `<group>.<dialect>.sql`
  with `-group`.
- `-group` filters EVERY output (Go and SQL) by exact, case-sensitive
  match on `TableMeta.Group`; without it every model comes out. When
  generating Go per group, give each group its own `-meta`/`-list` file
  and `-list-var`, and generate all groups — the package needs every
  `Meta()` to compile.
- The MySQL script has no foreign keys by design (applied last by the
  application).
- Same thing from Go: `tool.Run(tool.Options{...})` in
  `github.com/pablo-botella/dbh/tool` returns `*tool.Result{Models,
  Files}`.
- Typical wiring: `//go:generate dbh-table-meta -meta generated_meta.go
  -list generated_list.go` in a file of the model package.
