---
mkskill:
  pos: 200
  in: ai*
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
sql_helpers.go    EscapeSqlQuotes, IsUniqueConstraintError, NullIfZero, NullIfEmptyStr
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
