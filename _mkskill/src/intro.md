---
mkskill:
  pos: 10
  in: readme
---

# dbh

Minimal database handle for Go: `SqlExecutor` is the contract
that `*sql.DB`, `*sql.Tx` and `*Dbh` all satisfy — one signature for code that
must not care whether it runs inside a transaction — plus the capability
probes (`Closeable`, `Beginner`, `HasDbType`) and `WithTx`: run a
callback inside a transaction, whatever you were handed.

```go
func ListUsers(sqx dbh.SqlExecutor) error {
	rows, err := sqx.Query("SELECT name FROM users")
	// ...
}

ListUsers(db)         // *sql.DB, outside any transaction

tx, _ := db.Begin()
ListUsers(tx)         // the same code inside a transaction
```

On top of the contracts sit two optional layers; code that only wants the
contracts never touches them, and the linker leaves them out:

- **`Dbh`** — a `*sql.DB` with its dialect tag (SQLite or MySQL), so the
  code that needs dialect-specific SQL can ask the handle it was given.
- **`TableMeta`** — a table described as data: its DDL for either
  dialect, a generic insert/update and the pre-write clean-up of every
  column all come from that one description. The `dbh-table-meta`
  generator writes it from your model structs.

No drivers pulled in: everything underneath is stdlib. Opening the
connection is yours. `NullSqlExecutor` is the do-nothing placeholder
for wirings without a database — every operation fails with a clear
error instead of a nil-pointer panic.
