---
mkskill:
  pos: 220
  in: ai*
---

## API — TableMeta

```go
type TableMeta struct {
    Table        string
    Group        string                 // free label; dbh gives it no meaning
    Fields       []TableMetaField
    ExtraColumns []TableMetaExtraColumn // schema-only, no Go struct field
    Constraints  []string               // carried, NOT emitted by Build*
    Indexes      []TableMetaIndex
    ForeignKeys  []TableMetaFK
}
type HasMeta interface { Meta() TableMeta }
type Ignore struct{}                    // field type: schema-only column

func (t TableMeta) BuildCreateTable(dbType DbType) string
func (t TableMeta) BuildIndexes(dbType DbType) []string
func (t TableMeta) BuildAlterFKs(dbType DbType) []string  // MySQL only; nil on SQLite
func ColumnTypeDDL(t ColumnType, maxSize int, dbType DbType) string

func (t TableMeta) BuildMap(m any) map[string]any
func (t TableMeta) GenericSave(sqx SqlExecutor, idRef *int64, data map[string]any, fnames ...string) error
func (t TableMeta) QueryById(sqx SqlExecutor, id int64, fields ...string) (map[string]any, error)
func (t TableMeta) Query(sqx SqlExecutor, where string, args []any, fields ...string) (map[string]any, error)
func (t TableMeta) GetSafeWriteValue(name string, val any) (SafeWriteValue, error)
func (t TableMeta) FieldByName(name string) *TableMetaField
func (t TableMeta) WritableFieldNames() []string
```

- `Meta()` is **generated**, never hand-written: run `dbh-table-meta`
  after touching a model struct, its tags or its info JSON. Do not edit
  the generated files.
- A model is a struct named by a `//dbh:generate-table-meta-model
  <Struct>` comment in its file, plus a method
  `Dbh_generate_table_meta_model_info() string` returning ONE JSON string
  literal (the generator reads the source; it does not run it).
- `sql` tag tokens (space-separated): a type — `bigint` `int` `tinyint`
  `text` `date` `timestamp` `json` `varchar(N)` — and `pk`, `autoinc`,
  `not_null`, `unique`, `default:X`, `name:X`. No tag = not a column.
- `write` tag tokens (comma-separated): `trim` `trim_left` `trim_right`
  `upper` `lower` `truncate` `error_on_overflow` `zero_to_null`
  `validate_date` `validate_time` `validate_json` `invalid_to_null`.
- Info JSON keys: `Table` (required), `Group`, `Indexes`, `ForeignKeys`,
  `ExtraColumns`, `Constraints`. Unknown keys and unknown tag tokens are
  generator errors, not silent skips.
- The canonical Save is one line:
  `return m.Meta().GenericSave(sqx, &m.Id, m.Meta().BuildMap(m), fnames...)`.
  `*idRef == -1` inserts and stores the new id; otherwise it updates
  `WHERE id = ?`. `fnames[0]` may carry a comma-separated column subset.
- Schema creation order: for every model `BuildCreateTable` then
  `BuildIndexes`; then, on MySQL only, a second pass with
  `BuildAlterFKs` once every table exists. SQLite inlines its foreign
  keys in `CREATE TABLE`.
- `Query`/`QueryById` return ONE row as a map (int64 for numeric
  columns, string for the rest, NULL coalesced to 0 or ""). Lists and
  struct scanning are hand-written per model.
- Limits: it creates, it does not migrate (`CREATE TABLE IF NOT
  EXISTS`); identifiers are not quoted; `GenericSave` assumes a single
  integer key named `id`; the Build* methods take a `DbType`, never a
  dialect string.

## The generator

```
dbh-table-meta [-dir .] [-meta file] [-list file] [-list-var AllMdls]
               [-sql folder] [-dialects sqlite,mysql] [-group name]
```

- Each output is written only when its option is given; at least one of
  `-meta`, `-list`, `-sql` is required.
- `-meta` and `-list` are relative to `-dir`; `-sql` to the current
  directory. SQL files are `<dialect>.sql`, or `<group>.<dialect>.sql`
  with `-group`.
- `-group` filters EVERY output (Go and SQL) by exact, case-sensitive
  match on `TableMeta.Group`; without it every model comes out. When
  generating Go per group, give each group its own `-meta`/`-list` file
  and `-list-var`, and generate all groups — the package needs every
  `Meta()` to compile.
- The MySQL script has no foreign keys by design (applied last by the
  application).
- Same thing from Go: `tool.Run(tool.Options{...})` in
  `github.com/pablo-botella/dbh/tool` returns `*tool.Result{Models,
  Files}`.
- Typical wiring: `//go:generate dbh-table-meta -meta generated_meta.go
  -list generated_list.go` in a file of the model package.
