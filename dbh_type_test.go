package dbh

import (
	"database/sql"
	"errors"
	"testing"
)

func TestParseDbTypeAndString(t *testing.T) {
	if ParseDbType("mysql") != MySQL {
		t.Error(`ParseDbType("mysql") != MySQL`)
	}
	if ParseDbType("sqlite") != SQLite {
		t.Error(`ParseDbType("sqlite") != SQLite`)
	}
	if ParseDbType("anything else") != SQLite {
		t.Error("an unknown name must fall back to SQLite")
	}
	if MySQL.String() != "mysql" || SQLite.String() != "sqlite" {
		t.Errorf("String: got %q and %q", MySQL.String(), SQLite.String())
	}
}

func TestDbhBasics(t *testing.T) {
	my := New(nil, MySQL)
	lite := New(nil, SQLite)

	if !my.IsMysql() || lite.IsMysql() {
		t.Error("IsMysql is wrong")
	}
	if my.GetDbType() != MySQL || lite.GetDbType() != SQLite {
		t.Error("GetDbType is wrong")
	}
	if got := my.SqlConcat("a", "b"); got != "CONCAT(a, b)" {
		t.Errorf("mysql SqlConcat: %q", got)
	}
	if got := lite.SqlConcat("a", "b"); got != "a || b" {
		t.Errorf("sqlite SqlConcat: %q", got)
	}
}

func TestDbTypeOf(t *testing.T) {
	fake := &fakeDB{}
	conn := fake.open()
	defer conn.Close()

	if _, ok := DbTypeOf(conn); ok {
		t.Error("a plain *sql.DB must not report a database type")
	}
	got, ok := DbTypeOf(New(conn, MySQL))
	if !ok || got != MySQL {
		t.Errorf("DbTypeOf(*Dbh) = %v, %v", got, ok)
	}
}

func TestDbhWithTxCommits(t *testing.T) {
	fake := &fakeDB{}
	d := New(fake.open(), MySQL)
	defer d.Close()

	err := d.WithTx(func(tx *Tx) error {
		if tx.GetDbType() != MySQL {
			t.Error("the transaction lost the database type")
		}
		if got, ok := DbTypeOf(tx); !ok || got != MySQL {
			t.Errorf("DbTypeOf(*Tx) = %v, %v", got, ok)
		}
		_, err := tx.Exec("UPDATE accounts SET balance = 0")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.begins != 1 || fake.commits != 1 || fake.rollbacks != 0 {
		t.Errorf("begins=%d commits=%d rollbacks=%d", fake.begins, fake.commits, fake.rollbacks)
	}
	if len(fake.execs) != 1 {
		t.Errorf("execs: %v", fake.execs)
	}
}

func TestDbhWithTxRollsBack(t *testing.T) {
	fake := &fakeDB{}
	d := New(fake.open(), SQLite)
	defer d.Close()

	boom := errors.New("boom")
	if err := d.WithTx(func(tx *Tx) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("want boom, got %v", err)
	}
	if err := d.WithTx(func(tx *Tx) error { return ErrAbort }); !errors.Is(err, ErrAbort) {
		t.Fatalf("want ErrAbort, got %v", err)
	}
	if fake.begins != 2 || fake.commits != 0 || fake.rollbacks != 2 {
		t.Errorf("begins=%d commits=%d rollbacks=%d", fake.begins, fake.commits, fake.rollbacks)
	}
}

func TestTxWithTxReusesTheTransaction(t *testing.T) {
	fake := &fakeDB{}
	d := New(fake.open(), MySQL)
	defer d.Close()

	err := d.WithTx(func(outer *Tx) error {
		return outer.WithTx(func(inner *Tx) error {
			if inner != outer {
				t.Error("the nested call must run in the same transaction")
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.begins != 1 || fake.commits != 1 {
		t.Errorf("begins=%d commits=%d", fake.begins, fake.commits)
	}
}

// The published package-level WithTx keeps working with a *Dbh: it is a
// Beginner through the embedded *sql.DB.
func TestPackageWithTxAcceptsDbh(t *testing.T) {
	fake := &fakeDB{}
	d := New(fake.open(), MySQL)
	defer d.Close()

	if err := WithTx(d, func(tx *sql.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if fake.begins != 1 || fake.commits != 1 {
		t.Errorf("begins=%d commits=%d", fake.begins, fake.commits)
	}
}

func TestSqlHelpers(t *testing.T) {
	if got := EscapeSqlQuotes("it's"); got != "it''s" {
		t.Errorf("EscapeSqlQuotes: %q", got)
	}

	mysqlErr := errors.New("Error 1062 (23000): Duplicate entry 'x' for key 'users.name'")
	sqliteErr := errors.New("UNIQUE constraint failed: users.name")
	other := errors.New("connection refused")

	for _, err := range []error{mysqlErr, sqliteErr} {
		if !IsUniqueConstraintError(err, "name") {
			t.Errorf("not detected: %v", err)
		}
		if !IsUniqueConstraintError(err, "") {
			t.Errorf("an empty column must match any UNIQUE violation: %v", err)
		}
		if IsUniqueConstraintError(err, "email") {
			t.Errorf("wrong column matched: %v", err)
		}
	}
	if IsUniqueConstraintError(other, "") || IsUniqueConstraintError(nil, "") {
		t.Error("false positive")
	}
}

// needsDialect stands for code that asks for the dialect in its signature.
func needsDialect(sqx SqlExecutorWithDbType) DbType { return sqx.GetDbType() }

func TestSqlExecutorWithDbType(t *testing.T) {
	d := New(nil, MySQL)
	if got := needsDialect(d); got != MySQL {
		t.Errorf("*Dbh through SqlExecutorWithDbType: %v", got)
	}
	tx := &Tx{dbType: MySQL}
	if got := needsDialect(tx); got != MySQL {
		t.Errorf("*Tx through SqlExecutorWithDbType: %v", got)
	}
}
