package dbh

import "database/sql"

// DbType identifies the database dialect.
type DbType int

const (
	SQLite DbType = iota
	MySQL
)

// ParseDbType converts a dialect name ("sqlite", "mysql") to a DbType.
// Anything that is not "mysql" is SQLite.
func ParseDbType(s string) DbType {
	switch s {
	case "mysql":
		return MySQL
	default:
		return SQLite
	}
}

// String returns the dialect name: "sqlite" or "mysql".
func (t DbType) String() string {
	if t == MySQL {
		return "mysql"
	}
	return "sqlite"
}

// Dbh couples a database connection with its dialect tag, so the code that
// needs dialect-specific SQL can ask the handle it was given.
//
// It embeds *sql.DB, so it satisfies SqlExecutor, Beginner and Closeable.
// Opening the connection and choosing a driver belong to the caller.
type Dbh struct {
	*sql.DB
	DbType DbType
}

var _ SqlExecutor = (*Dbh)(nil)
var _ HasDbType = (*Dbh)(nil)

// New creates a Dbh for the given connection and database type.
func New(conn *sql.DB, dbType DbType) *Dbh {
	return &Dbh{DB: conn, DbType: dbType}
}

// IsMysql returns true if the database is MySQL.
func (d *Dbh) IsMysql() bool {
	return d.DbType == MySQL
}

// GetDbType returns the database type of the handle.
func (d *Dbh) GetDbType() DbType {
	return d.DbType
}

// SqlConcat returns a SQL expression that concatenates a and b.
//
//	SQLite:  a || b
//	MySQL:   CONCAT(a, b)
func (d *Dbh) SqlConcat(a, b string) string {
	if d.DbType == MySQL {
		return "CONCAT(" + a + ", " + b + ")"
	}
	return a + " || " + b
}
