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

## Capabilities

`SqlExecutor` says what every executor can do; the capability probes say
what some of them can *also* do. Consumers discover a capability with a
plain type assertion — no flags, no registry:

```go
// close whatever you were handed, if it can close at all
c, ok := sqx.(Closeable)
if ok {
	defer c.Close()
}
```

- `Closeable` — `Close() error`. A `*sql.DB` is closeable; a `*sql.Tx` is
  not, so a deferred close like the one above is always safe: the probe
  simply does not fire on a transaction.
- `Beginner` — `Begin() (*sql.Tx, error)`. The example is `WithTx` itself,
  which probes it to start a transaction when handed anything that is not
  already one:

```go
b, ok := sqx.(Beginner)
if !ok {
	return fmt.Errorf("WithTx: %T does not support Begin()", sqx)
}

tx, err := b.Begin()
if err != nil {
	return err
}
```

dbh ships the contracts; the policy — when to close, when to begin —
belongs to the caller.

## WithTx

Run a callback inside a transaction, whatever you were handed:

```go
err := dbh.WithTx(sqx, func(tx *sql.Tx) error {
	if _, err := tx.Exec("UPDATE accounts SET ..."); err != nil {
		return err
	}
	return nil
})
```

- Handed a `*sql.Tx`, fn runs in that existing transaction — no nesting,
  no surprise commits: the owner keeps control.
- Handed anything else, it must be a `Beginner`: `WithTx` begins, commits
  on success and rolls back on error (a rollback failure enriches the
  returned error instead of hiding it).
- Return `ErrAbort` from fn to roll back **on purpose**: the transaction
  is undone without treating it as a real failure — detect it upstream
  with `errors.Is(err, dbh.ErrAbort)` (e.g. show validation warnings
  instead of a 500).

## Install

```
go get github.com/pablo-botella/dbh
```

## License

MIT — see [LICENSE](LICENSE).
