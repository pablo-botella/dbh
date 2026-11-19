---
mkskill:
  pos: 30
  in: readme
---

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

- `HasDbType` — `GetDbType() DbType`. A `*Dbh` and a `*dbh.Tx` know their
  dialect; a plain `*sql.DB` or `*sql.Tx` does not. `DbTypeOf` wraps the
  probe:

```go
if t, ok := dbh.DbTypeOf(sqx); ok && t == dbh.MySQL {
	// MySQL-only SQL
}
```

- `SqlExecutorWithDbType` — `SqlExecutor` plus `HasDbType` in one
  interface, for code that cannot work without the dialect. Ask for it
  in the signature and the compiler does the checking: a `*Dbh` and a
  `*dbh.Tx` fit, a plain `*sql.DB` or `*sql.Tx` does not compile.

```go
func CreateTables(sqx dbh.SqlExecutorWithDbType) error {
	_, err := sqx.Exec(meta.BuildCreateTable(sqx.GetDbType()))
	return err
}
```

dbh ships the contracts; the policy — when to close, when to begin —
belongs to the caller.
