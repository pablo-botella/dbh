---
mkskill:
  pos: 46
  in: readme
---

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
