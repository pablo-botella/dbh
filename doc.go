// Package dbh is a minimal database virtual interface — pure contracts,
// no structs, no drivers; everything underneath is stdlib.
package dbh

// The docs are composed from _mkskill/ — edit the sources there, then:
//go:generate go run github.com/pablo-botella/mkskill/cmd/mkskill@latest -q build
//go:generate go run github.com/pablo-botella/mkskill/cmd/mkskill@latest -q -vbuild
