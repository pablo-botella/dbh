package tool

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/pablo-botella/dbh"
)

// model is one struct marked with the directive, already turned into the
// metadata its Meta() method will return.
type model struct {
	Struct string // Go struct name
	Meta   dbh.TableMeta
}

// --- The info JSON -----------------------------------------------------------

// info is the JSON returned by the InfoMethodName method of a model. It is
// decoded strictly: an unknown key is an error, not a silent skip.
type info struct {
	Table        string            `json:"Table"`
	Group        string            `json:"Group"`
	Indexes      []infoIndex       `json:"Indexes"`
	ForeignKeys  []infoFK          `json:"ForeignKeys"`
	Constraints  []string          `json:"Constraints"`
	ExtraColumns []infoExtraColumn `json:"ExtraColumns"`
}

type infoIndex struct {
	Columns []infoIndexColumn `json:"columns"`
	Unique  bool              `json:"unique"`
}

type infoIndexColumn struct {
	Name string `json:"name"`
}

type infoFK struct {
	Columns    []string `json:"columns"`
	RefTable   string   `json:"ref_table"`
	RefColumns []string `json:"ref_columns"`
}

type infoExtraColumn struct {
	Name    string `json:"name"`
	Type    string `json:"type"` // bigint, int, tinyint, varchar(N), text, date, timestamp, json
	After   string `json:"after"`
	NotNull bool   `json:"not_null"`
	Unique  bool   `json:"unique"`
	Default string `json:"default"`
}

// --- Reading the folder ------------------------------------------------------

// parseDir reads every Go source of dir (tests and the given output files
// excluded) and returns the package name and the models it declares.
func parseDir(dir string, skip ...string) (string, []model, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return "", nil, err
	}
	sort.Strings(files)

	skipSet := map[string]bool{}
	for _, s := range skip {
		if s != "" {
			skipSet[filepath.Clean(s)] = true
		}
	}

	var (
		pkg    string
		models []model
	)
	byStruct := map[string]string{}
	byTable := map[string]string{}

	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") || skipSet[filepath.Clean(path)] {
			continue
		}
		name, ms, err := parseFile(fset, path)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", path, err)
		}
		if len(ms) == 0 {
			continue
		}
		if pkg == "" {
			pkg = name
		} else if pkg != name {
			return "", nil, fmt.Errorf("%s: package %s, but other models are in package %s", path, name, pkg)
		}
		for _, m := range ms {
			if prev, dup := byStruct[m.Struct]; dup {
				return "", nil, fmt.Errorf("%s: model %s already declared in %s", path, m.Struct, prev)
			}
			if prev, dup := byTable[m.Meta.Table]; dup {
				return "", nil, fmt.Errorf("%s: table %q already declared in %s", path, m.Meta.Table, prev)
			}
			byStruct[m.Struct] = path
			byTable[m.Meta.Table] = path
			models = append(models, m)
		}
	}
	return pkg, models, nil
}

// parseFile returns the package name of one source file and its models.
func parseFile(fset *token.FileSet, path string) (string, []model, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return "", nil, err
	}

	// The directives say which structs of the file are models.
	var wanted []string
	included := map[string]bool{}
	for _, cg := range f.Comments {
		for _, c := range cg.List {
			name, ok, err := directiveTarget(c.Text)
			if err != nil {
				return "", nil, err
			}
			if ok && !included[name] {
				included[name] = true
				wanted = append(wanted, name)
			}
		}
	}
	if len(wanted) == 0 {
		return f.Name.Name, nil, nil
	}

	structs := map[string]*ast.StructType{}
	infos := map[string]string{}

	for _, decl := range f.Decls {
		switch d := decl.(type) {

		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || !included[ts.Name.Name] {
					continue
				}
				if st, ok := ts.Type.(*ast.StructType); ok {
					structs[ts.Name.Name] = st
				}
			}

		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) == 0 || d.Name.Name != InfoMethodName {
				continue
			}
			recv := receiverTypeName(d.Recv.List[0].Type)
			if !included[recv] {
				continue
			}
			s, ok := returnedStringLiteral(d)
			if !ok {
				return "", nil, fmt.Errorf("%s.%s must return a single string literal", recv, InfoMethodName)
			}
			infos[recv] = s
		}
	}

	var out []model
	for _, name := range wanted {
		st, ok := structs[name]
		if !ok {
			return "", nil, fmt.Errorf("directive names %s, but the file declares no such struct", name)
		}
		raw, ok := infos[name]
		if !ok {
			return "", nil, fmt.Errorf("%s: missing method %s", name, InfoMethodName)
		}
		m, err := buildModel(name, st, raw)
		if err != nil {
			return "", nil, err
		}
		out = append(out, m)
	}
	return f.Name.Name, out, nil
}

// directiveTarget reports whether a comment is the directive and, if so,
// the struct name it carries.
func directiveTarget(comment string) (string, bool, error) {
	text := strings.TrimSpace(comment)
	rest, ok := strings.CutPrefix(text, Directive)
	if !ok {
		return "", false, nil
	}
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false, nil // a longer word that merely starts the same
	}
	fields := strings.Fields(rest)
	if len(fields) != 1 {
		return "", false, fmt.Errorf("%s wants exactly one struct name, got %q", Directive, strings.TrimSpace(rest))
	}
	return fields[0], true, nil
}

func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// returnedStringLiteral extracts the string literal a function returns.
func returnedStringLiteral(fn *ast.FuncDecl) (string, bool) {
	if fn.Body == nil {
		return "", false
	}
	for _, stmt := range fn.Body.List {
		ret, ok := stmt.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		lit, ok := ret.Results[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			return "", false
		}
		return s, true
	}
	return "", false
}

// --- Struct + info → TableMeta -----------------------------------------------

func buildModel(name string, st *ast.StructType, rawInfo string) (model, error) {
	var in info
	dec := json.NewDecoder(bytes.NewReader([]byte(rawInfo)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return model{}, fmt.Errorf("%s.%s: %w", name, InfoMethodName, err)
	}
	if in.Table == "" {
		return model{}, fmt.Errorf("%s.%s: missing Table", name, InfoMethodName)
	}

	fields, err := parseStructFields(st)
	if err != nil {
		return model{}, fmt.Errorf("%s: %w", name, err)
	}

	meta := dbh.TableMeta{
		Table:       in.Table,
		Group:       in.Group,
		Fields:      fields,
		Constraints: in.Constraints,
	}

	for _, ix := range in.Indexes {
		cols := make([]string, 0, len(ix.Columns))
		for _, c := range ix.Columns {
			cols = append(cols, c.Name)
		}
		meta.Indexes = append(meta.Indexes, dbh.TableMetaIndex{Columns: cols, Unique: ix.Unique})
	}

	for _, fk := range in.ForeignKeys {
		meta.ForeignKeys = append(meta.ForeignKeys, dbh.TableMetaFK{
			Columns:    fk.Columns,
			RefTable:   fk.RefTable,
			RefColumns: fk.RefColumns,
		})
	}

	for _, ec := range in.ExtraColumns {
		t, size, err := parseColumnType(ec.Type)
		if err != nil {
			return model{}, fmt.Errorf("%s: extra column %s: %w", name, ec.Name, err)
		}
		col := dbh.TableMetaExtraColumn{
			Name:    ec.Name,
			Type:    t,
			MaxSize: size,
			After:   ec.After,
			NotNull: ec.NotNull,
			Unique:  ec.Unique,
		}
		if ec.Default != "" {
			d := ec.Default
			col.Default = &d
		}
		meta.ExtraColumns = append(meta.ExtraColumns, col)
	}

	return model{Struct: name, Meta: meta}, nil
}

// parseStructFields turns every struct field that carries an sql tag into a
// TableMetaField. Fields without the tag are not columns.
func parseStructFields(st *ast.StructType) ([]dbh.TableMetaField, error) {
	var out []dbh.TableMetaField
	for _, f := range st.Fields.List {
		if f.Tag == nil {
			continue
		}
		raw, err := strconv.Unquote(f.Tag.Value)
		if err != nil {
			continue
		}
		tag := reflect.StructTag(raw)
		sqlTag := tag.Get("sql")
		if sqlTag == "" {
			continue
		}
		goType := exprString(f.Type)

		for _, n := range f.Names {
			fld := dbh.TableMetaField{
				GoField:   n.Name,
				IsPrivate: !ast.IsExported(n.Name),
				IsIgnore:  goType == "dbh.Ignore",
				JSONTag:   tag.Get("json"),
			}
			if err := applySQLTag(sqlTag, &fld); err != nil {
				return nil, fmt.Errorf("field %s: %w", n.Name, err)
			}
			if err := applyWriteTag(tag.Get("write"), &fld); err != nil {
				return nil, fmt.Errorf("field %s: %w", n.Name, err)
			}
			if fld.Name == "" {
				fld.Name = strings.ToLower(n.Name)
			}
			out = append(out, fld)
		}
	}
	return out, nil
}

func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	}
	return fmt.Sprintf("%T", e)
}

// applySQLTag reads the sql tag: space-separated tokens, each one of
//
//	name:X       column name override
//	default:X    DEFAULT clause, verbatim
//	bigint int tinyint text date timestamp json varchar(N)   the type
//	autoinc not_null unique pk                               attributes
func applySQLTag(s string, f *dbh.TableMetaField) error {
	for tok := range strings.FieldsSeq(s) {
		switch {
		case strings.HasPrefix(tok, "name:"):
			f.Name = tok[len("name:"):]
		case strings.HasPrefix(tok, "default:"):
			v := tok[len("default:"):]
			f.Default = &v
		case tok == "autoinc":
			f.AutoInc = true
		case tok == "not_null":
			f.NotNull = true
		case tok == "unique":
			f.Unique = true
		case tok == "pk":
			f.IsPrimaryKey = true
		default:
			t, size, err := parseColumnType(tok)
			if err != nil {
				return fmt.Errorf("sql tag: %w", err)
			}
			f.Type, f.MaxSize = t, size
		}
	}
	return nil
}

// parseColumnType reads a type token: one of the plain names or varchar(N).
func parseColumnType(tok string) (dbh.ColumnType, int, error) {
	if rest, ok := strings.CutPrefix(tok, "varchar("); ok {
		n, err := strconv.Atoi(strings.TrimSuffix(rest, ")"))
		if err != nil || n <= 0 || !strings.HasSuffix(rest, ")") {
			return dbh.TypeUnknown, 0, fmt.Errorf("bad varchar size in %q", tok)
		}
		return dbh.TypeVarchar, n, nil
	}
	switch tok {
	case "bigint":
		return dbh.TypeBigInt, 0, nil
	case "int":
		return dbh.TypeInt, 0, nil
	case "tinyint":
		return dbh.TypeTinyInt, 0, nil
	case "text":
		return dbh.TypeText, 0, nil
	case "date":
		return dbh.TypeDate, 0, nil
	case "timestamp":
		return dbh.TypeTimestamp, 0, nil
	case "json":
		return dbh.TypeJSON, 0, nil
	}
	return dbh.TypeUnknown, 0, fmt.Errorf("unknown token %q", tok)
}

// applyWriteTag reads the write tag: comma-separated transforms applied to
// the value before it is written.
func applyWriteTag(s string, f *dbh.TableMetaField) error {
	for tok := range strings.SplitSeq(s, ",") {
		tok = strings.TrimSpace(tok)
		switch tok {
		case "":
		case "trim":
			f.Trim = dbh.TrimBoth
		case "trim_left":
			f.Trim = dbh.TrimLeft
		case "trim_right":
			f.Trim = dbh.TrimRight
		case "upper":
			f.Case = dbh.CaseUpper
		case "lower":
			f.Case = dbh.CaseLower
		case "truncate":
			f.Overflow = dbh.OverflowTruncate
		case "error_on_overflow":
			f.Overflow = dbh.OverflowError
		case "zero_to_null":
			f.ZeroToNull = true
		case "invalid_to_null":
			f.InvalidToNull = true
		case "validate_json":
			f.ValidateJSON = true
		case "validate_date":
			f.ValidateDate = true
		case "validate_time":
			f.ValidateTime = true
		default:
			return fmt.Errorf("write tag: unknown token %q", tok)
		}
	}
	return nil
}

// --- Dependency order --------------------------------------------------------

// sortByFK returns the models in dependency order: a model comes after every
// model it references through a foreign key. Ties are broken alphabetically
// by struct name, so the output is deterministic. Self-references and
// references to tables outside the set are tolerated.
func sortByFK(models []model) []model {
	byTable := map[string]int{}
	for i, m := range models {
		byTable[m.Meta.Table] = i
	}

	indices := make([]int, len(models))
	for i := range models {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return models[indices[i]].Struct < models[indices[j]].Struct
	})

	visited := make(map[int]bool)
	onStack := make(map[int]bool)
	out := make([]model, 0, len(models))

	var visit func(idx int)
	visit = func(idx int) {
		if visited[idx] || onStack[idx] {
			return
		}
		onStack[idx] = true
		for _, fk := range models[idx].Meta.ForeignKeys {
			if dep, ok := byTable[fk.RefTable]; ok && dep != idx {
				visit(dep)
			}
		}
		onStack[idx] = false
		visited[idx] = true
		out = append(out, models[idx])
	}
	for _, idx := range indices {
		visit(idx)
	}
	return out
}
