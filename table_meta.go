package dbh

import "reflect"

// --- Table metadata ----------------------------------------------------------

// TableMeta describes a table: its columns, indexes and foreign keys. The
// Build* methods emit its DDL for either dialect, and GenericSave / Query
// read and write rows from it.
//
// It is normally produced by the dbh-table-meta generator from a model
// struct and its tags, not written by hand.
type TableMeta struct {
	Table string

	// Group is a free label chosen by the application to classify its
	// tables (create some of them earlier, generate them apart, ...).
	// dbh gives it no meaning.
	Group string

	Fields       []TableMetaField
	ExtraColumns []TableMetaExtraColumn // schema-only, no Go struct field

	// Constraints is carried as declared by the model. The Build* methods
	// do not emit it.
	Constraints []string

	Indexes     []TableMetaIndex
	ForeignKeys []TableMetaFK
}

// TableMetaExtraColumn declares a column that exists in the database but has
// no Go struct field (e.g. blobs accessed through custom methods). It is
// emitted in CREATE TABLE and ignored by BuildMap, GenericSave and Query.
type TableMetaExtraColumn struct {
	Name    string
	Type    ColumnType
	MaxSize int    // for varchar
	After   string // optional placement hint: name of the preceding column
	NotNull bool
	Unique  bool
	Default *string
}

// TableMetaIndex describes a CREATE INDEX statement.
type TableMetaIndex struct {
	Name    string   // optional; if empty, auto: idx_<table>_<col1>_<col2>...
	Columns []string // ordered list of column names
	Unique  bool
}

// TableMetaFK describes a FOREIGN KEY constraint.
type TableMetaFK struct {
	Name       string   // optional; if empty, auto: fk_<table>_<col1>_<col2>...
	Columns    []string // local columns
	RefTable   string
	RefColumns []string
}

// HasMeta is implemented by model structs that carry table metadata. Generic
// helpers use it to iterate over every model of an application.
type HasMeta interface {
	Meta() TableMeta
}

// FieldByName returns the field with the given column name, or nil.
func (t TableMeta) FieldByName(name string) *TableMetaField {
	for i := range t.Fields {
		if t.Fields[i].Name == name {
			return &t.Fields[i]
		}
	}
	return nil
}

// WritableFieldNames returns the column names of the writable fields
// (primary keys and Ignore fields excluded), in declaration order.
func (t TableMeta) WritableFieldNames() []string {
	out := make([]string, 0, len(t.Fields))
	for _, f := range t.Fields {
		if f.IsPrimaryKey || f.IsIgnore {
			continue
		}
		out = append(out, f.Name)
	}
	return out
}

// --- Markers -----------------------------------------------------------------

// Ignore marks a struct field as a schema-only column. The generator emits the
// column in CREATE TABLE; the generic load and save skip it. Access is through
// model-specific methods.
type Ignore struct{}

// --- TableMetaField ----------------------------------------------------------

// TableMetaField describes one column derived from a struct field and its tags.
type TableMetaField struct {

	// --- Identity ---
	Name      string
	GoField   string
	GoType    reflect.Type
	IsPrivate bool
	IsIgnore  bool

	// --- SQL type ---
	Type    ColumnType
	MaxSize int

	// --- Column attributes ---
	AutoInc      bool
	NotNull      bool
	Unique       bool
	IsPrimaryKey bool
	Default      *string

	// --- Nullable strategy (derived from the Go type) ---
	Nullable NullableMode

	// --- Pre-write transforms ---
	Trim     TrimMode
	Case     CaseMode
	Overflow OverflowMode

	// --- Conversions to NULL ---
	ZeroToNull    bool
	EmptyToNull   bool
	InvalidToNull bool

	// --- Pre-write validations ---
	ValidateJSON bool
	ValidateDate bool
	ValidateTime bool

	// --- Load behavior ---
	LoadSkip bool

	// --- Tag pass-through ---
	JSONTag string
}

// --- Enums -------------------------------------------------------------------

// ColumnType is the dialect-neutral type of a column.
type ColumnType int

const (
	TypeUnknown ColumnType = iota
	TypeBigInt
	TypeInt
	TypeTinyInt
	TypeVarchar
	TypeText
	TypeDate
	TypeTimestamp
	TypeJSON
)

// NullableMode says how a field represents NULL on the Go side.
type NullableMode int

const (
	NullableNone NullableMode = iota
	NullablePointer
	NullableSqlNull
	NullableTagBased
)

// TrimMode says which side of a string value is trimmed before writing.
type TrimMode int

const (
	TrimNone TrimMode = iota
	TrimBoth
	TrimLeft
	TrimRight
)

// CaseMode says how the case of a string value is changed before writing.
type CaseMode int

const (
	CaseNone CaseMode = iota
	CaseUpper
	CaseLower
)

// OverflowMode says what happens to a varchar value longer than MaxSize.
type OverflowMode int

const (
	OverflowNone OverflowMode = iota
	OverflowTruncate
	OverflowError
)
