package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// allowedDirectTransactionFile reports whether a server file may contain a raw
// database/sql transaction boundary. The named catalogtx capability (the
// capability implementation and its observed driver) and the schema migration
// own the only legal raw boundaries in production; tests may hold the writer
// directly to prove availability under contention.
func allowedDirectTransactionFile(relative string) bool {
	if strings.HasSuffix(relative, "_test.go") {
		return true
	}
	if strings.HasPrefix(relative, "server/internal/db/catalogtx/") {
		return true
	}
	return relative == "server/internal/db/migration.go"
}

// sanctionedTransactionReceiver reports whether the BeginTx receiver is the
// catalogtx capability field the codebase uses everywhere (writer, Writer,
// snapshot) rather than a raw *sql.DB or driver.Conn. The writer and snapshot
// fields are the only production paths into named, measured transactions.
func sanctionedTransactionReceiver(expr ast.Expr) bool {
	switch receiver := expr.(type) {
	case *ast.Ident:
		return receiver.Name == "writer"
	case *ast.SelectorExpr:
		switch receiver.Sel.Name {
		case "writer", "Writer", "snapshot":
			return true
		}
	case *ast.ParenExpr:
		return sanctionedTransactionReceiver(receiver.X)
	}
	return false
}

// checkSQLiteTransactionInventory closes the direct-BeginTx inventory: every
// production transaction boundary must be the catalogtx capability, the schema
// migration, or a test. A raw BeginTx outside those boundaries escapes the
// one-writer/query-only-reader architecture, so admission, body, commit, and
// cancellation would stop being measured and bounded.
func checkSQLiteTransactionInventory(root string) error {
	var violations []string
	err := filepath.WalkDir(filepath.Join(root, "server"), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if allowedDirectTransactionFile(relative) {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, relative, source, parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("parse %s: %w", relative, err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "BeginTx" {
				return true
			}
			if sanctionedTransactionReceiver(selector.X) {
				return true
			}
			receiver := string(source[selector.X.Pos()-1 : selector.X.End()-1])
			violations = append(violations, fmt.Sprintf(
				"%s:%d: direct BeginTx on %s; enter transactions through the catalogtx.Writer/Reader named operations so admission, body, commit, and cancellation stay measured and bounded",
				relative, fileSet.Position(call.Pos()).Line, receiver,
			))
			return true
		})
		return nil
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("raw SQLite transaction boundaries outside the catalogtx capability, schema migration, and tests:\n%s", strings.Join(violations, "\n"))
	}
	return nil
}
