// Package tool is the table-metadata generator of dbh as a library: it reads
// the model structs of a Go package, and writes their Meta() methods, the
// model list and the SQL schema scripts.
//
// The dbh-table-meta command is a thin wrapper around Run.
package tool

import (
	"errors"
	"fmt"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/pablo-botella/dbh"
)

const (
	// Directive marks a struct as a model. It goes in a comment of the file
	// that declares the struct, followed by the struct name:
	//
	//	//dbh:generate-table-meta-model User_mdl
	Directive = "//dbh:generate-table-meta-model"

	// InfoMethodName is the method of a model that returns, as a JSON string
	// literal, what the struct tags cannot say: the table name, its group,
	// indexes, foreign keys, constraints and extra columns.
	InfoMethodName = "Dbh_generate_table_meta_model_info"

	// DbhImportPath is the import path written in the generated Go files.
	DbhImportPath = "github.com/pablo-botella/dbh"

	// DefaultListVar is the name of the model list variable when
	// Options.ListVar is empty.
	DefaultListVar = "AllMdls"

	// DefaultDialects is the dialect list when Options.Dialects is empty.
	DefaultDialects = "sqlite,mysql"

	generatedBy = "dbh-table-meta"
)

// Options says what Run reads and what it writes. Every output is optional:
// one is written only when its option is set, and at least one is required.
type Options struct {
	// Dir is the folder with the Go sources of the models. Empty means ".".
	Dir string

	// MetaFile is the Go file that receives the Meta() methods. A relative
	// path is taken from Dir, since the file belongs to the model package.
	MetaFile string

	// ListFile is the Go file that receives the model list, in dependency
	// order. A relative path is taken from Dir.
	ListFile string

	// ListVar is the name of the list variable. Empty means DefaultListVar.
	ListVar string

	// SQLDir is the folder that receives one SQL script per dialect. A
	// relative path is taken from the current directory.
	SQLDir string

	// Dialects is the comma-separated dialect list for the SQL scripts
	// ("sqlite", "mysql"). Empty means DefaultDialects.
	Dialects string

	// Group restricts every output to the models whose group is exactly
	// this one. Empty means every model, grouped or not.
	Group string
}

// Result reports what Run did.
type Result struct {
	Models int      // models written, after the group filter
	Files  []string // files written, in order
}

// Run executes the generator.
func Run(o Options) (*Result, error) {
	if o.MetaFile == "" && o.ListFile == "" && o.SQLDir == "" {
		return nil, errors.New("nothing to generate: set at least one output (meta, list or sql)")
	}

	dir := o.Dir
	if dir == "" {
		dir = "."
	}
	metaFile := inDir(dir, o.MetaFile)
	listFile := inDir(dir, o.ListFile)

	listVar := o.ListVar
	if listVar == "" {
		listVar = DefaultListVar
	}
	if o.ListFile != "" && !token.IsIdentifier(listVar) {
		return nil, fmt.Errorf("list variable %q is not a valid Go identifier", listVar)
	}

	var dialects []dbh.DbType
	if o.SQLDir != "" {
		var err error
		dialects, err = parseDialects(o.Dialects)
		if err != nil {
			return nil, err
		}
	}

	pkg, models, err := parseDir(dir, metaFile, listFile)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("no models found in %s (no %s directive)", dir, Directive)
	}

	models = sortByFK(models)
	if o.Group != "" {
		models = filterGroup(models, o.Group)
		if len(models) == 0 {
			return nil, fmt.Errorf("no models in group %q", o.Group)
		}
	}

	res := &Result{Models: len(models)}

	if metaFile != "" {
		if err := writeMetaGo(metaFile, pkg, models); err != nil {
			return nil, fmt.Errorf("writing %s: %w", metaFile, err)
		}
		res.Files = append(res.Files, metaFile)
	}

	if listFile != "" {
		if err := writeListGo(listFile, pkg, listVar, o.Group, models); err != nil {
			return nil, fmt.Errorf("writing %s: %w", listFile, err)
		}
		res.Files = append(res.Files, listFile)
	}

	for _, d := range dialects {
		path := filepath.Join(o.SQLDir, sqlFileName(o.Group, d))
		if err := writeSQL(path, d, o.Group, models); err != nil {
			return nil, fmt.Errorf("writing %s: %w", path, err)
		}
		res.Files = append(res.Files, path)
	}

	return res, nil
}

// inDir resolves an output Go file against the model folder.
func inDir(dir, file string) string {
	if file == "" || filepath.IsAbs(file) {
		return file
	}
	return filepath.Join(dir, file)
}

// parseDialects turns "sqlite,mysql" into the dialect list, refusing names
// it does not know and dropping repeats.
func parseDialects(s string) ([]dbh.DbType, error) {
	if strings.TrimSpace(s) == "" {
		s = DefaultDialects
	}
	var out []dbh.DbType
	seen := map[dbh.DbType]bool{}
	for name := range strings.SplitSeq(s, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		var d dbh.DbType
		switch name {
		case "sqlite":
			d = dbh.SQLite
		case "mysql":
			d = dbh.MySQL
		case "":
			continue
		default:
			return nil, fmt.Errorf("unknown dialect %q (want sqlite or mysql)", name)
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("empty dialect list")
	}
	return out, nil
}

// filterGroup keeps the models of the given group, preserving their order.
func filterGroup(models []model, group string) []model {
	out := make([]model, 0, len(models))
	for _, m := range models {
		if m.Meta.Group == group {
			out = append(out, m)
		}
	}
	return out
}
