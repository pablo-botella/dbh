---
mkskill:
  pos: 200
  in: ai*
---

# dbh — agent notes

Go package `github.com/pablo-botella/dbh`: a minimal database virtual
interface. Pure contracts — no concrete handle, no drivers, everything
stdlib. The only function is `WithTx`, which itself works strictly
against the contracts.

Layout — one file per piece:

```
sql_executor.go   SqlExecutor + the compile-time guards + NullSqlExecutor
capabilities.go   Closeable + Beginner (the capability probes)
with_tx.go        WithTx + ErrAbort
```

Driver subpackages are planned (see the repo issues); the root package
stays purely virtual on purpose.
