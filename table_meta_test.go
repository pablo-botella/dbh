package dbh

import (
	"database/sql/driver"
	"reflect"
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

// ordersMeta is a table that touches every DDL branch: autoinc primary key,
// varchar, a default, unique, an extra column with a placement hint, a plain
// and a unique index, and a foreign key.
func ordersMeta() TableMeta {
	return TableMeta{
		Table: "orders",
		Group: "sales",
		Fields: []TableMetaField{
			{Name: "id", GoField: "Id", Type: TypeBigInt, AutoInc: true, NotNull: true, IsPrimaryKey: true},
			{Name: "id_customer", GoField: "Id_customer", Type: TypeBigInt, NotNull: true, ZeroToNull: true},
			{Name: "code", GoField: "Code", Type: TypeVarchar, MaxSize: 10, Unique: true, Trim: TrimBoth, Case: CaseUpper, Overflow: OverflowTruncate},
			{Name: "status", GoField: "Status", Type: TypeInt, Default: strPtr("0")},
			{Name: "placed_on", GoField: "Placed_on", Type: TypeDate, ValidateDate: true},
			{Name: "notes", GoField: "Notes", Type: TypeText},
		},
		ExtraColumns: []TableMetaExtraColumn{
			{Name: "attachment", Type: TypeText, After: "code"},
		},
		Indexes: []TableMetaIndex{
			{Columns: []string{"id_customer"}},
			{Columns: []string{"code", "status"}, Unique: true},
		},
		ForeignKeys: []TableMetaFK{
			{Columns: []string{"id_customer"}, RefTable: "customers", RefColumns: []string{"id"}},
		},
	}
}

func TestBuildCreateTableSQLite(t *testing.T) {
	want := strings.Join([]string{
		"CREATE TABLE IF NOT EXISTS orders (",
		"  id INTEGER PRIMARY KEY AUTOINCREMENT,",
		"  id_customer INTEGER NOT NULL,",
		"  code TEXT UNIQUE,",
		"  attachment TEXT,",
		"  status INTEGER DEFAULT 0,",
		"  placed_on TEXT,",
		"  notes TEXT,",
		"  FOREIGN KEY (id_customer) REFERENCES customers(id)",
		")",
	}, "\n")
	if got := ordersMeta().BuildCreateTable(SQLite); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBuildCreateTableMySQL(t *testing.T) {
	want := strings.Join([]string{
		"CREATE TABLE IF NOT EXISTS orders (",
		"  id BIGINT NOT NULL AUTO_INCREMENT,",
		"  id_customer BIGINT NOT NULL,",
		"  code VARCHAR(10) UNIQUE,",
		"  attachment TEXT,",
		"  status INT DEFAULT 0,",
		"  placed_on DATE,",
		"  notes TEXT,",
		"  PRIMARY KEY (id)",
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
	}, "\n")
	if got := ordersMeta().BuildCreateTable(MySQL); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBuildIndexes(t *testing.T) {
	gotLite := ordersMeta().BuildIndexes(SQLite)
	wantLite := []string{
		"CREATE INDEX IF NOT EXISTS idx_orders_id_customer ON orders (id_customer)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_code_status ON orders (code, status)",
	}
	if !reflect.DeepEqual(gotLite, wantLite) {
		t.Errorf("sqlite: %q", gotLite)
	}

	gotMy := ordersMeta().BuildIndexes(MySQL)
	wantMy := []string{
		"CREATE INDEX idx_orders_id_customer ON orders (id_customer)",
		"CREATE UNIQUE INDEX idx_orders_code_status ON orders (code, status)",
	}
	if !reflect.DeepEqual(gotMy, wantMy) {
		t.Errorf("mysql: %q", gotMy)
	}
}

func TestBuildAlterFKs(t *testing.T) {
	if got := ordersMeta().BuildAlterFKs(SQLite); got != nil {
		t.Errorf("sqlite inlines its foreign keys, got %q", got)
	}
	got := ordersMeta().BuildAlterFKs(MySQL)
	want := []string{
		"ALTER TABLE orders ADD CONSTRAINT fk_orders_id_customer FOREIGN KEY (id_customer) REFERENCES customers(id)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mysql: %q", got)
	}
}

func TestColumnTypeDDL(t *testing.T) {
	if got := ColumnTypeDDL(TypeVarchar, 35, MySQL); got != "VARCHAR(35)" {
		t.Errorf("mysql varchar: %q", got)
	}
	if got := ColumnTypeDDL(TypeVarchar, 35, SQLite); got != "TEXT" {
		t.Errorf("sqlite varchar: %q", got)
	}
	if got := ColumnTypeDDL(TypeTimestamp, 0, MySQL); got != "TIMESTAMP NULL" {
		t.Errorf("mysql timestamp: %q", got)
	}
	if got := ColumnTypeDDL(TypeTinyInt, 0, SQLite); got != "INTEGER" {
		t.Errorf("sqlite tinyint: %q", got)
	}
}

func TestFieldLookups(t *testing.T) {
	m := ordersMeta()
	if f := m.FieldByName("code"); f == nil || f.MaxSize != 10 {
		t.Errorf("FieldByName(code) = %+v", f)
	}
	if m.FieldByName("missing") != nil {
		t.Error("FieldByName must return nil for an unknown column")
	}
	want := []string{"id_customer", "code", "status", "placed_on", "notes"}
	if got := m.WritableFieldNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("WritableFieldNames: %q", got)
	}
}

func TestGetSafeWriteValue(t *testing.T) {
	m := ordersMeta()

	sw, err := m.GetSafeWriteValue("code", "  abcdefghijklmnop  ")
	if err != nil {
		t.Fatal(err)
	}
	if sw.Value != "ABCDEFGHIJ" || !sw.Truncated || sw.MaxSize != 10 {
		t.Errorf("trim + upper + truncate: %+v", sw)
	}

	sw, _ = m.GetSafeWriteValue("id_customer", int64(0))
	if sw.Value != nil {
		t.Errorf("zero_to_null: %+v", sw)
	}

	sw, err = m.GetSafeWriteValue("status", "7")
	if err != nil || sw.Value != int64(7) || !sw.Converted {
		t.Errorf("string to int: %+v, %v", sw, err)
	}

	if _, err := m.GetSafeWriteValue("placed_on", "not a date"); err == nil {
		t.Error("an invalid date must be an error")
	}
	if _, err := m.GetSafeWriteValue("placed_on", "2026-02-28"); err != nil {
		t.Errorf("a valid date failed: %v", err)
	}
	if _, err := m.GetSafeWriteValue("status", "seven"); err == nil {
		t.Error("a non-numeric string on an int column must be an error")
	}
	if _, err := m.GetSafeWriteValue("missing", 1); err == nil {
		t.Error("an unknown field must be an error")
	}
}

type order struct {
	Id          int64
	Id_customer int64
	Code        string
	Status      int
	Placed_on   string
	Notes       string
}

func TestBuildMap(t *testing.T) {
	o := &order{Id: 5, Id_customer: 0, Code: " ab ", Status: 3, Placed_on: "2026-02-28", Notes: "n"}
	got := ordersMeta().BuildMap(o)
	want := map[string]any{
		"id_customer": nil,
		"code":        "AB",
		"status":      int64(3),
		"placed_on":   "2026-02-28",
		"notes":       "n",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestGenericSaveInsertAndUpdate(t *testing.T) {
	fake := &fakeDB{lastInsertID: 42}
	conn := fake.open()
	defer conn.Close()

	m := ordersMeta()
	o := &order{Id: -1, Id_customer: 9, Code: "x1", Status: 1, Placed_on: "2026-02-28", Notes: "n"}

	if err := m.GenericSave(conn, &o.Id, m.BuildMap(o)); err != nil {
		t.Fatal(err)
	}
	if o.Id != 42 {
		t.Errorf("the new id was not stored: %d", o.Id)
	}
	wantInsert := "INSERT INTO orders (id_customer, code, status, placed_on, notes) VALUES (?, ?, ?, ?, ?)"
	if fake.execs[0] != wantInsert {
		t.Errorf("insert: %q", fake.execs[0])
	}

	o.Notes = "changed"
	if err := m.GenericSave(conn, &o.Id, m.BuildMap(o), "notes, status"); err != nil {
		t.Fatal(err)
	}
	wantUpdate := "UPDATE orders SET notes = ?, status = ? WHERE id = ?"
	if fake.execs[1] != wantUpdate {
		t.Errorf("update: %q", fake.execs[1])
	}
	wantArgs := []driver.Value{"changed", int64(1), int64(42)}
	if !reflect.DeepEqual(fake.execArgs[1], wantArgs) {
		t.Errorf("update args: %#v", fake.execArgs[1])
	}

	if err := m.GenericSave(conn, &o.Id, m.BuildMap(o), "nope"); err == nil {
		t.Error("an unknown field name must be an error")
	}
}

func TestQueryById(t *testing.T) {
	fake := &fakeDB{
		columns: []string{"id", "code", "status"},
		row:     []driver.Value{int64(7), "AB", int64(3)},
	}
	conn := fake.open()
	defer conn.Close()

	got, err := ordersMeta().QueryById(conn, 7, "id", "code", "status")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"id": int64(7), "code": "AB", "status": int64(3)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
	wantSQL := "SELECT id, COALESCE(code,''), COALESCE(status,0) FROM orders WHERE id = ?"
	if fake.queries[0] != wantSQL {
		t.Errorf("query: %q", fake.queries[0])
	}
}
