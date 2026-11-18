// Package dbh is a minimal database handle: SqlExecutor is the contract that
// *sql.DB, *sql.Tx and *Dbh all satisfy, Dbh couples a connection with its
// dialect tag, and TableMeta describes a table so its DDL, its inserts and
// its updates come from one place. No drivers; everything underneath is
// stdlib.
package dbh

// The docs are composed from _mkskill/ — edit the sources there, then:
//go:generate go run github.com/pablo-botella/mkskill/cmd/mkskill@latest -q build
//go:generate go run github.com/pablo-botella/mkskill/cmd/mkskill@latest -q -vbuild
