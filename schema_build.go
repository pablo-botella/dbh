package dbh

import (
	"fmt"
	"strings"
)

// BuildCreateTable returns the CREATE TABLE statement for the given dialect.
// For SQLite, foreign keys are inlined since the dialect does not support
// ALTER TABLE ADD CONSTRAINT. For MySQL, foreign keys are deferred to
// BuildAlterFKs so the creation order of the tables is irrelevant.
func (t TableMeta) BuildCreateTable(dbType DbType) string {
	mysql := dbType == MySQL

	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS %s (\n", t.Table)

	parts := make([]string, 0, len(t.Fields)+len(t.ExtraColumns)+4)
	pkCols := []string{}
	for _, f := range t.Fields {
		if f.IsPrimaryKey {
			pkCols = append(pkCols, f.Name)
		}
		parts = append(parts, "  "+f.Name+" "+columnTypeDDL(f, dbType)+columnAttrsDDL(f, dbType))
	}
	// Insert ExtraColumns according to their After hint (or append).
	for _, ec := range t.ExtraColumns {
		line := "  " + ec.Name + " " + extraColumnTypeDDL(ec, dbType) + extraColumnAttrsDDL(ec)
		if ec.After == "" {
			parts = append(parts, line)
			continue
		}
		idx := -1
		for i, p := range parts {
			if strings.HasPrefix(strings.TrimSpace(p), ec.After+" ") {
				idx = i
				break
			}
		}
		if idx < 0 {
			parts = append(parts, line)
		} else {
			parts = append(parts[:idx+1], append([]string{line}, parts[idx+1:]...)...)
		}
	}
	if mysql && len(pkCols) > 0 {
		parts = append(parts, "  PRIMARY KEY ("+strings.Join(pkCols, ", ")+")")
	}
	if !mysql {
		for _, fk := range t.ForeignKeys {
			parts = append(parts, "  "+fkInlineClause(fk))
		}
	}

	b.WriteString(strings.Join(parts, ",\n"))
	b.WriteString("\n)")
	if mysql {
		b.WriteString(" ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci")
	}
	return b.String()
}

// BuildIndexes returns the CREATE INDEX statements for the given dialect.
func (t TableMeta) BuildIndexes(dbType DbType) []string {
	out := make([]string, 0, len(t.Indexes))
	for _, ix := range t.Indexes {
		out = append(out, buildCreateIndex(t.Table, ix, dbType))
	}
	return out
}

// BuildAlterFKs returns one ALTER TABLE ADD CONSTRAINT statement per foreign
// key. It only emits for MySQL; SQLite inlines them in CREATE TABLE.
func (t TableMeta) BuildAlterFKs(dbType DbType) []string {
	if dbType != MySQL {
		return nil
	}
	out := make([]string, 0, len(t.ForeignKeys))
	for _, fk := range t.ForeignKeys {
		name := fk.Name
		if name == "" {
			name = "fk_" + t.Table + "_" + strings.Join(fk.Columns, "_")
		}
		out = append(out, fmt.Sprintf(
			"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(%s)",
			t.Table, name,
			strings.Join(fk.Columns, ", "),
			fk.RefTable,
			strings.Join(fk.RefColumns, ", "),
		))
	}
	return out
}

// ColumnTypeDDL returns the SQL type a column of the given type and size gets
// in CREATE TABLE for the dialect. Exposed for schema checkers, which compare
// it with what the database reports.
func ColumnTypeDDL(t ColumnType, maxSize int, dbType DbType) string {
	return columnTypeDDL(TableMetaField{Type: t, MaxSize: maxSize}, dbType)
}

// --- helpers ----------------------------------------------------------------

func columnTypeDDL(f TableMetaField, dbType DbType) string {
	if dbType != MySQL {
		switch f.Type {
		case TypeBigInt, TypeInt, TypeTinyInt:
			return "INTEGER"
		}
		return "TEXT"
	}
	switch f.Type {
	case TypeBigInt:
		return "BIGINT"
	case TypeInt:
		return "INT"
	case TypeTinyInt:
		return "TINYINT"
	case TypeVarchar:
		return fmt.Sprintf("VARCHAR(%d)", f.MaxSize)
	case TypeText:
		return "TEXT"
	case TypeDate:
		return "DATE"
	case TypeTimestamp:
		return "TIMESTAMP NULL"
	case TypeJSON:
		return "JSON"
	}
	return "TEXT"
}

func extraColumnTypeDDL(ec TableMetaExtraColumn, dbType DbType) string {
	f := TableMetaField{Type: ec.Type, MaxSize: ec.MaxSize}
	return columnTypeDDL(f, dbType)
}

func extraColumnAttrsDDL(ec TableMetaExtraColumn) string {
	var parts []string
	if ec.NotNull {
		parts = append(parts, "NOT NULL")
	}
	if ec.Unique {
		parts = append(parts, "UNIQUE")
	}
	if ec.Default != nil {
		parts = append(parts, "DEFAULT "+*ec.Default)
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

func columnAttrsDDL(f TableMetaField, dbType DbType) string {
	mysql := dbType == MySQL

	var parts []string
	if f.NotNull && !f.IsPrimaryKey {
		parts = append(parts, "NOT NULL")
	}
	if f.AutoInc {
		if mysql {
			parts = append(parts, "NOT NULL AUTO_INCREMENT")
		} else {
			parts = append(parts, "PRIMARY KEY AUTOINCREMENT")
		}
	}
	if f.IsPrimaryKey && !f.AutoInc && !mysql {
		parts = append(parts, "PRIMARY KEY")
	}
	if f.Unique {
		parts = append(parts, "UNIQUE")
	}
	if f.Default != nil {
		parts = append(parts, "DEFAULT "+*f.Default)
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

func fkInlineClause(fk TableMetaFK) string {
	return fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s)",
		strings.Join(fk.Columns, ", "), fk.RefTable, strings.Join(fk.RefColumns, ", "))
}

func buildCreateIndex(table string, ix TableMetaIndex, dbType DbType) string {
	name := ix.Name
	if name == "" {
		name = "idx_" + table + "_" + strings.Join(ix.Columns, "_")
	}
	unique := ""
	if ix.Unique {
		unique = "UNIQUE "
	}
	expr := strings.Join(ix.Columns, ", ")
	if dbType != MySQL {
		return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)", unique, name, table, expr)
	}
	return fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)", unique, name, table, expr)
}
