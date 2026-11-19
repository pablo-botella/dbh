---
mkskill:
  pos: 42
  in: readme
---

## Dbh: the connection with its dialect

`Dbh` is a `*sql.DB` with a tag that says which database it talks to:

```go
conn, err := sql.Open("mysql", dsn)   // your driver, your DSN
// ...
d := dbh.New(conn, dbh.MySQL)         // or dbh.ParseDbType("mysql")

d.Query("SELECT ...")                 // every *sql.DB method is there
d.IsMysql()                           // true
d.SqlConcat("first", "last")          // CONCAT(first, last)  —  first || last on SQLite
```

It embeds `*sql.DB`, so it is a `SqlExecutor`, a `Beginner` and a
`Closeable` with no extra code, and it goes wherever a connection is
expected. It does not open the connection, choose a driver or hold
configuration: it only makes the dialect travel with the handle instead
of living in a global or an extra parameter.

- `DbType` is `dbh.SQLite` or `dbh.MySQL`. `ParseDbType("mysql")` gives
  `MySQL`; anything else gives `SQLite`. `String()` gives the name back.
- `EscapeSqlQuotes` doubles single quotes.
- `IsUniqueConstraintError(err, "column")` recognises a UNIQUE violation
  on that column in the error message of either dialect; an empty column
  matches any.
- `NullIfZero(n)` and `NullIfEmptyStr(s)` return `nil` for `0` and `""`
  and the value otherwise: pass the result as a query argument, or as a
  value of the `GenericSave` map, to store `NULL` in an optional column.

### The dialect inside a transaction

`Begin` belongs to the standard library: it returns a plain `*sql.Tx`,
and the tag is lost. When the code inside the transaction needs the
dialect, start it from the handle instead:

```go
err := d.WithTx(func(tx *dbh.Tx) error {
	if tx.GetDbType() == dbh.MySQL {
		// ...
	}
	_, err := tx.Exec("UPDATE accounts SET ...")
	return err
})
```

- `Dbh.WithTx` commits on success and rolls back on error, `ErrAbort`
  included — the same rules as the package-level `WithTx`.
- `*dbh.Tx` embeds `*sql.Tx` and is a `SqlExecutor`: pass it to any query
  code. `tx.WithTx(fn)` runs fn in the same transaction, so one function
  works whether it was handed a `*Dbh` or a `*dbh.Tx`.
- Do not hand a `*dbh.Tx` to the package-level `dbh.WithTx`: that one
  only recognises a `*sql.Tx` as an open transaction. Inside a `*dbh.Tx`,
  use the method. To reach the standard transaction, it is the embedded
  field: `tx.Tx`.
