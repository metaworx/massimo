// SPDX-License-Identifier: AGPL-3.0-or-later
// Copyright (C) 2026 The Massimo Authors

package store

// The content and structure rule (docs/server.md, "Encryption in the
// local store", and "What version 1 prepares", item 4): a content column
// may one day hold ciphertext, so no index covers one and no query
// filters on one. The structural columns (identifiers, serial,
// users.name, tokens.hash, times, kind, device) stay plaintext and carry
// the indexes and the WHERE clauses.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// contentColumns are the content columns per table: names, addresses,
// hidden, settings.value, preferences.value, events.data, audit.detail,
// backups.body and note.
var contentColumns = map[string][]string{
	"cubes":       {"name", "address", "proxy_address", "discovered_address", "mac", "hidden"},
	"users":       {"display_name"},
	"roles":       {"name"},
	"tokens":      {"label", "last_used_from"},
	"settings":    {"value"},
	"preferences": {"value"},
	"events":      {"data"},
	"audit":       {"detail", "remote"},
	"backups":     {"body", "note"},
	"commands":    {"request", "result"},
}

func isContent(table, col string) bool {
	for _, c := range contentColumns[table] {
		if strings.EqualFold(c, col) {
			return true
		}
	}
	return false
}

// No index of the migrated schema, the automatic ones of UNIQUE and
// PRIMARY KEY included, covers a content column.
func TestNoIndexOnContent(t *testing.T) {
	s := open(t, t.TempDir())
	rows, err := s.db.Query(`SELECT m.name, m.tbl_name, i.name FROM sqlite_master m JOIN pragma_index_info(m.name) i WHERE m.type = 'index'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var index, table, col string
		if err := rows.Scan(&index, &table, &col); err != nil {
			t.Fatal(err)
		}
		n++
		if isContent(table, col) {
			t.Errorf("index %s covers the content column %s.%s", index, table, col)
		}
	}
	if n == 0 {
		t.Fatal("no index found; the check would pass vacuously")
	}
	// Every table with content columns exists, so the list cannot rot.
	for table, cols := range contentColumns {
		for _, col := range cols {
			var found int
			s.db.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name = ?`, table, col).Scan(&found)
			if found != 1 {
				t.Errorf("content column %s.%s is not in the schema", table, col)
			}
		}
	}
}

var (
	sqlTable  = regexp.MustCompile(`(?is)\b(?:FROM|UPDATE|INTO)\s+([a-z_]+)`)
	sqlWhere  = regexp.MustCompile(`(?is)\bWHERE\b(.*?)(?:\bORDER\s+BY\b|\bGROUP\s+BY\b|\bLIMIT\b|\bRETURNING\b|$)`)
	sqlQuoted = regexp.MustCompile(`'[^']*'`)
	sqlIdent  = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.]*`)
)

// contentFilters returns the content columns a statement filters on.
func contentFilters(stmt string) []string {
	m := sqlTable.FindStringSubmatch(stmt)
	if m == nil {
		return nil
	}
	table := strings.ToLower(m[1])
	if _, known := contentColumns[table]; !known {
		return nil
	}
	var out []string
	for _, w := range sqlWhere.FindAllStringSubmatch(stmt, -1) {
		clause := sqlQuoted.ReplaceAllString(w[1], "")
		for _, id := range sqlIdent.FindAllString(clause, -1) {
			col := id[strings.LastIndex(id, ".")+1:]
			if isContent(table, col) {
				out = append(out, table+"."+col)
			}
		}
	}
	return out
}

// No query of the package and no statement of a migration filters on a
// content column. The queries are the string literals of the package's
// Go files that contain WHERE; a literal joined from pieces is checked
// piece by piece, and the WHERE piece names its table.
func TestNoFilterOnContent(t *testing.T) {
	if got := contentFilters(`SELECT id FROM cubes WHERE name = ? AND serial = ?`); len(got) != 1 || got[0] != "cubes.name" {
		t.Fatalf("the check misses a filter on a content column: %v", got)
	}
	if got := contentFilters(`UPDATE cubes SET name = ? WHERE id = ?`); len(got) != 0 {
		t.Fatalf("the check flags a SET: %v", got)
	}
	checked := 0
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v, err := strconv.Unquote(lit.Value)
			if err != nil || !regexp.MustCompile(`(?i)\bWHERE\b`).MatchString(v) {
				return true
			}
			checked++
			for _, c := range contentFilters(v) {
				t.Errorf("%s: a query filters on the content column %s: %s", fset.Position(lit.Pos()), c, v)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no query found; the check would pass vacuously")
	}
	err = fs.WalkDir(embeddedMigrations(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(embeddedMigrations(), p)
		if err != nil {
			return err
		}
		sql := regexp.MustCompile(`--[^\n]*`).ReplaceAllString(string(b), "")
		for _, stmt := range strings.Split(sql, ";") {
			for _, c := range contentFilters(stmt) {
				t.Errorf("%s: a statement filters on the content column %s", p, c)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
