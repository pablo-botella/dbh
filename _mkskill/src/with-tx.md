---
mkskill:
  pos: 40
  in: readme
---

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
