---
mkskill:
  pos: 44
  in: readme
---

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
