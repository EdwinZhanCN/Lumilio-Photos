package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixtureFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// beginTxSelector parses a single statement and returns its BeginTx selector.
func beginTxSelector(t *testing.T, statement string) *ast.SelectorExpr {
	t.Helper()
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(
		fileSet,
		"fixture.go",
		"package fixture\n\nfunc f() {\n\t"+statement+"\n}",
		parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("parse %q: %v", statement, err)
	}
	var found *ast.SelectorExpr
	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "BeginTx" {
			found = selector
			return false
		}
		return true
	})
	if found == nil {
		t.Fatalf("no BeginTx call parsed from %q", statement)
	}
	return found
}

func TestAllowedDirectTransactionFile(t *testing.T) {
	for _, allowed := range []string{
		"server/internal/db/catalogtx/capability.go",
		"server/internal/db/catalogtx/driver.go",
		"server/internal/db/catalogtx/recorder_test.go",
		"server/internal/db/migration.go",
		"server/internal/service/asset_service_test.go",
	} {
		if !allowedDirectTransactionFile(allowed) {
			t.Fatalf("legitimate raw transaction boundary rejected: %q", allowed)
		}
	}
	for _, rejected := range []string{
		"server/internal/service/asset_service.go",
		"server/internal/queue/queue_setup.go",
		"server/internal/db/db.go",
		"server/app/app.go",
	} {
		if allowedDirectTransactionFile(rejected) {
			t.Fatalf("production file unexpectedly allowed raw BeginTx: %q", rejected)
		}
	}
}

func TestSanctionedTransactionReceiver(t *testing.T) {
	for _, statement := range []string{
		"writer.BeginTx(ctx, op, nil)",
		"s.writer.BeginTx(ctx, op, nil)",
		"w.Writer.BeginTx(ctx, op, nil)",
		"rm.writer.BeginTx(ctx, op, nil)",
		"s.snapshot.BeginTx(ctx, op)",
		"(s.writer).BeginTx(ctx, op, nil)",
	} {
		if !sanctionedTransactionReceiver(beginTxSelector(t, statement).X) {
			t.Fatalf("catalogtx capability receiver rejected: %q", statement)
		}
	}
	for _, statement := range []string{
		"database.SQL.BeginTx(ctx, nil)",
		"catalog.SQL.BeginTx(ctx, nil)",
		"pool.BeginTx(ctx, nil)",
		"db.BeginTx(ctx, nil)",
		"database.BeginTx(ctx, nil)",
		"beginner.BeginTx(ctx, options)",
	} {
		if sanctionedTransactionReceiver(beginTxSelector(t, statement).X) {
			t.Fatalf("raw pool receiver accepted: %q", statement)
		}
	}
}

func TestCheckSQLiteTransactionInventoryRejectsRawBoundary(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, "server/internal/queue/leaky.go", `package queue

import "database/sql"

func leaky(db *sql.DB) error {
	tx, err := db.BeginTx(nil, nil)
	if err != nil {
		return err
	}
	return tx.Commit()
}
`)
	writeFixtureFile(t, root, "server/internal/db/catalogtx/capability.go", `package catalogtx

import "database/sql"

func begin(pool *sql.DB) error {
	_, err := pool.BeginTx(nil, nil)
	return err
}
`)

	err := checkSQLiteTransactionInventory(root)
	if err == nil {
		t.Fatal("raw BeginTx in production queue code was not rejected")
	}
	if !strings.Contains(err.Error(), "server/internal/queue/leaky.go") {
		t.Fatalf("violation does not name the offending file: %v", err)
	}
	if !strings.Contains(err.Error(), "catalogtx") {
		t.Fatalf("violation does not direct the fix toward catalogtx: %v", err)
	}
}

func TestCheckSQLiteTransactionInventoryAcceptsClosedInventory(t *testing.T) {
	root := t.TempDir()
	writeFixtureFile(t, root, "server/internal/db/catalogtx/capability.go", `package catalogtx

import "database/sql"

func begin(pool *sql.DB) error {
	_, err := pool.BeginTx(nil, nil)
	return err
}
`)
	writeFixtureFile(t, root, "server/internal/db/migration.go", `package db

import "database/sql"

func applyMigrationGroup(database *sql.DB) error {
	_, err := database.BeginTx(nil, nil)
	return err
}
`)
	writeFixtureFile(t, root, "server/internal/queue/worker.go", `package queue

import "server/internal/db/catalogtx"

type worker struct {
	Writer *catalogtx.Writer
}

func (w *worker) work() error {
	_, err := w.Writer.BeginTx(nil, catalogtx.OperationAgentFinishRun, nil)
	return err
}
`)
	writeFixtureFile(t, root, "server/internal/service/holder_test.go", `package service

import "database/sql"

func hold(database *sql.DB) error {
	tx, err := database.BeginTx(nil, nil)
	if err != nil {
		return err
	}
	return tx.Rollback()
}
`)

	if err := checkSQLiteTransactionInventory(root); err != nil {
		t.Fatalf("closed transaction inventory rejected: %v", err)
	}
}
