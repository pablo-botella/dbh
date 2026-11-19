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

## TableMeta: a table described once

A `TableMeta` is a table as data: name, columns, indexes, foreign keys.
A model returns it from `Meta()`, and three things come out of that one
description:

```go
meta := (&User_mdl{}).Meta()

// 1. the schema, for either dialect
meta.BuildCreateTable(dbh.SQLite)   // CREATE TABLE IF NOT EXISTS users (...)
meta.BuildIndexes(dbh.SQLite)       // []string of CREATE INDEX
meta.BuildAlterFKs(dbh.MySQL)       // []string of ALTER TABLE ... ADD CONSTRAINT

// 2. insert or update, with every column cleaned on the way
func (m *User_mdl) Save(sqx dbh.SqlExecutor, fnames ...string) error {
	return m.Meta().GenericSave(sqx, &m.Id, m.Meta().BuildMap(m), fnames...)
}

// 3. a row as a column→value map
row, err := meta.QueryById(sqx, 7)
```

You do not write the `Meta()` by hand: you tag the struct and the
generator (next section) writes it.

```go
//dbh:generate-table-meta-model User_mdl
type User_mdl struct {
	Id       int64  `sql:"bigint autoinc not_null pk" json:"id"`
	Id_team  int64  `sql:"bigint" write:"zero_to_null" json:"id_team"`
	Name     string `sql:"varchar(40) not_null unique" write:"trim,truncate" json:"name"`
	Country  string `sql:"varchar(2) default:'US'" write:"upper"`
	Born_on  string `sql:"date name:birth_date" write:"validate_date,invalid_to_null"`
}

func (m *User_mdl) Dbh_generate_table_meta_model_info() string {
	return `{
		"Table": "users",
		"Group": "accounts",
		"Indexes": [
			{"columns": [{"name": "id_team"}]},
			{"columns": [{"name": "country"}, {"name": "name"}], "unique": true}
		],
		"ForeignKeys": [
			{"columns": ["id_team"], "ref_table": "teams", "ref_columns": ["id"]}
		]
	}`
}
```

### The `sql` tag

Space-separated tokens. A field without the tag is not a column.

| Token | Meaning |
|---|---|
| `bigint` `int` `tinyint` `text` `date` `timestamp` `json` `varchar(N)` | the column type |
| `pk` | primary key |
| `autoinc` | auto-increment |
| `not_null` | NOT NULL |
| `unique` | UNIQUE |
| `default:X` | `DEFAULT X`, verbatim (no spaces in X) |
| `name:X` | column name; the default is the field name in lower case |

A field of type `dbh.Ignore` is a schema-only column: it is created with
the table, and the generic load and save skip it.

### The `write` tag

Comma-separated transforms, applied to the value before it is written
(by `BuildMap`, through `GetSafeWriteValue`):

| Token | Meaning |
|---|---|
| `trim` `trim_left` `trim_right` | strip white space |
| `upper` `lower` | change case |
| `truncate` | cut a varchar longer than N |
| `error_on_overflow` | refuse a varchar longer than N |
| `zero_to_null` | write NULL for `0` or the empty string |
| `validate_date` | must be `YYYY-MM-DD` |
| `validate_time` | must be RFC 3339 |
| `validate_json` | must be valid JSON |
| `invalid_to_null` | a failed validation writes NULL instead of failing |

### The info JSON

What the tags cannot say goes in the JSON string literal returned by
`Dbh_generate_table_meta_model_info()`. An unknown key is an error.

| Key | Meaning |
|---|---|
| `Table` | table name — required |
| `Group` | a free label for the table; see below |
| `Indexes` | `[{"columns": [{"name": "c"}], "unique": false}]` |
| `ForeignKeys` | `[{"columns": ["c"], "ref_table": "t", "ref_columns": ["id"]}]` |
| `ExtraColumns` | columns with no struct field: `[{"name": "c", "type": "text", "after": "other", "not_null": false, "unique": false, "default": ""}]` |
| `Constraints` | a list of strings, carried into `TableMeta.Constraints`; the `Build*` methods do not emit it |

### Groups

`Group` is a label you choose — `"system"`, `"accounts"`, anything. dbh
gives it no meaning: it stores it in `TableMeta.Group`, and the generator
can filter by it. Use it to create some tables before the rest, or to
generate them apart:

```go
for _, m := range AllMdls {
	if m.Meta().Group != "system" {
		continue
	}
	// create this one now
}
```

### What the DDL looks like

| | SQLite | MySQL |
|---|---|---|
| Types | `INTEGER` and `TEXT` only | `BIGINT`, `INT`, `TINYINT`, `VARCHAR(N)`, `TEXT`, `DATE`, `TIMESTAMP NULL`, `JSON` |
| Auto-increment | `PRIMARY KEY AUTOINCREMENT` on the column | `NOT NULL AUTO_INCREMENT` plus a `PRIMARY KEY (...)` line |
| Foreign keys | inside `CREATE TABLE` | apart, from `BuildAlterFKs` |
| Indexes | `CREATE INDEX IF NOT EXISTS` | `CREATE INDEX` |
| Table options | none | `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci` |

SQLite cannot add a foreign key to an existing table, so there they are
inlined. On MySQL, create every table first and apply `BuildAlterFKs`
last: the creation order stops mattering.

### Limits worth knowing

- **It creates, it does not migrate.** `CREATE TABLE IF NOT EXISTS`
  leaves an existing table as it is.
- **Names are not quoted.** A table or column named like a reserved word
  breaks the statement.
- **`GenericSave` wants a single integer key named `id`**: it inserts
  when `*idRef == -1` (and stores the new id there) and updates
  `WHERE id = ?` otherwise.
- **Loading is yours.** `Query` and `QueryById` return one row as a map;
  listing and scanning into structs stay hand-written in each model.

## dbh-table-meta: the generator

`dbh-table-meta` reads the Go sources of a model package — it parses
them, it never runs them — and writes up to three things:

- the Go file with the `Meta()` method of every model;
- the Go file with the model list, in dependency order;
- one SQL script per dialect, with the same statements your application
  would run.

```
go install github.com/pablo-botella/dbh/cmd/dbh-table-meta@latest
```

```
dbh-table-meta [options]

  -dir <folder>       folder with the Go sources of the models   (default ".")
  -meta <file>        write the Meta() methods to this Go file
  -list <file>        write the model list to this Go file
  -list-var <name>    name of the model list variable             (default "AllMdls")
  -sql <folder>       write one SQL script per dialect into this folder
  -dialects <list>    comma-separated SQL dialects                (default "sqlite,mysql")
  -group <name>       only the models of this group; empty means all
```

Each output is written only when its option is given, and at least one
is required — so it generates as much or as little as you ask:

```
dbh-table-meta -meta generated_meta.go -list generated_list.go   # the Go files
dbh-table-meta -sql doc/ddl                                      # only the SQL, both dialects
dbh-table-meta -sql doc/ddl -dialects mysql                      # only the MySQL script
```

Wired to `go generate`, from a file of the model package:

```go
//go:generate dbh-table-meta -meta generated_meta.go -list generated_list.go -sql ../doc/ddl
```

- **Which structs.** Every `.go` file of the folder is read, tests
  excluded; a struct is a model when a `//dbh:generate-table-meta-model
  <Struct>` comment in its file names it.
- **Paths.** `-meta` and `-list` are relative to `-dir`: those files
  belong to the model package. `-sql` is relative to the current
  directory.
- **The list** is ordered so that a model comes after the models its
  foreign keys point to; ties go alphabetically by struct name.
- **The SQL scripts** are named `<dialect>.sql`: `CREATE TABLE` and
  `CREATE INDEX`, in that same order. The MySQL script carries no foreign
  keys: they need every table to exist, so the application applies them
  last with `BuildAlterFKs`.
- **Mistakes stop it.** An unknown token in a tag, an unknown key in the
  info JSON, a model without table name or two models on one table are
  errors, caught while reading — before anything is written.

### Generating by group

`-group` filters **every** output — the Go files and the SQL — to the
models whose `Group` is exactly that name (case matters). Without it,
all the models come out, grouped or not.

```
dbh-table-meta -group system -sql doc/ddl        # doc/ddl/system.sqlite.sql, system.mysql.sql
```

With a group, the scripts are named `<group>.<dialect>.sql`, so several
groups can share a folder.

A package needs the `Meta()` of every model to compile. To generate the
Go files group by group, give each group its own files and its own list
variable — and generate them all:

```
dbh-table-meta -group system   -meta generated_meta_system.go   -list generated_list_system.go   -list-var SystemMdls
dbh-table-meta -group accounts -meta generated_meta_accounts.go -list generated_list_accounts.go -list-var AccountMdls
```

That also gives you one ready-made list per group.

### As a library

The command is a thin wrapper. The generator is the package
`github.com/pablo-botella/dbh/tool`, callable from your own tooling:

```go
res, err := tool.Run(tool.Options{
	Dir:      "internal/model",
	MetaFile: "generated_meta.go",
	SQLDir:   "doc/ddl",
	Group:    "accounts",
})
// res.Files: what was written; res.Models: how many models
```

It needs the model sources, so it is a build-time tool. At run time an
application does not need it: the `Meta()` methods are already compiled
in, and `BuildCreateTable`, `BuildIndexes` and `BuildAlterFKs` produce
the same SQL from them.

## Install

The library:

```
go get github.com/pablo-botella/dbh
```

The table-metadata generator, only if you use `TableMeta`:

```
go install github.com/pablo-botella/dbh/cmd/dbh-table-meta@latest
```

## License

MIT — see [LICENSE](LICENSE).
