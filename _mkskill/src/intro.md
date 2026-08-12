---
mkskill:
  pos: 10
  in: readme
---

# dbh

Minimal database virtual interface for Go: `SqlExecutor` is the contract
that `*sql.DB` and `*sql.Tx` both satisfy — one signature for code that
must not care whether it runs inside a transaction — plus the capability
probes (`Closeable`, `Beginner`) and `WithTx`, the only function: run a
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

Pure contracts, no drivers pulled in: everything underneath is stdlib.
The one concrete type is `NullSqlExecutor`, the do-nothing placeholder
for wirings without a database — every operation fails with a clear
error instead of a nil-pointer panic.
