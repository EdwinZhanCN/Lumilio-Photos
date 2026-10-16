package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The structural writer budget of the repository scan index (#222,
// invariant 6): a catalog write transaction holds the single SQLite writer,
// so nothing slow or unbounded may run inside one. Every function literal
// passed to WithTx, and every function whose name ends in Tx (the convention
// for code that runs inside a caller's transaction), must not perform
// filesystem I/O, hash content, sleep, or run a repository-wide COUNT; and
// the batch cap MaxBatchRows must not exceed 256 rows.
const (
	scanIndexDirectory  = "server/internal/storage/scan"
	scanIndexBatchLimit = 256
)

// transactionForbiddenPackages lists imported packages whose every call is
// filesystem I/O or hashing, by import path.
var transactionForbiddenPackages = map[string]string{
	"os":                         "filesystem I/O",
	"io":                         "I/O",
	"io/ioutil":                  "filesystem I/O",
	"io/fs":                      "filesystem I/O",
	"server/internal/utils/hash": "content hashing",
	"github.com/zeebo/blake3":    "content hashing",
}

// transactionForbiddenFunctions lists package-qualified functions that
// sleep, wait, or walk the disk, by import path.
var transactionForbiddenFunctions = map[string]map[string]string{
	"time":          {"Sleep": "sleep", "After": "sleep", "Tick": "sleep", "NewTimer": "sleep", "NewTicker": "sleep"},
	"path/filepath": {"Walk": "filesystem I/O", "WalkDir": "filesystem I/O", "Glob": "filesystem I/O"},
}

// transactionForbiddenMethods lists method names that reach the disk or
// hash content whatever their receiver: the repository filesystem's
// observation and probe methods, os.Root and os.File methods, and the hash
// helpers.
var transactionForbiddenMethods = map[string]string{
	"InspectMedia": "filesystem I/O", "ListUserMediaDirectory": "filesystem I/O",
	"ProbeUserMediaAbsence": "filesystem I/O", "VerifyIdentity": "filesystem I/O",
	"OpenContext": "filesystem I/O", "Open": "filesystem I/O", "OpenFile": "filesystem I/O",
	"Lstat": "filesystem I/O", "Stat": "filesystem I/O", "ReadDir": "filesystem I/O",
	"ReadFile": "filesystem I/O", "WriteFile": "filesystem I/O", "Rename": "filesystem I/O",
	"Remove": "filesystem I/O", "RemoveAll": "filesystem I/O", "Mkdir": "filesystem I/O",
	"MkdirAll": "filesystem I/O", "Chtimes": "filesystem I/O", "Sync": "filesystem I/O",
	"CalculateReaderHash": "content hashing", "CalculateQuickHash": "content hashing",
	"CalculateLayeredBLAKE3": "content hashing", "CalculateLayeredBLAKE3Reader": "content hashing",
	"CalculateFileHash": "content hashing",
}

// transactionAllowedCounts are the only Count queries a transaction may
// run: each is bounded by a unique key, not by the repository.
var transactionAllowedCounts = map[string]bool{
	"CountRepositoryEntriesWithTrashID": true, // one trash ID, unique per trashed file
}

func checkScanIndexWriterBudget(root string) error {
	violations, err := scanScanIndexWriterTransactionBodies(root)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("scan index writer transactions must not perform filesystem I/O, hashing, sleeping, or repository-wide counts (#222 invariant 6):\n%s",
			strings.Join(violations, "\n"))
	}
	return nil
}

// scanScanIndexWriterTransactionBodies reports every forbidden call inside a
// transaction body of the scan index package, and a MaxBatchRows above the
// budget, as "path:line:reason".
func scanScanIndexWriterTransactionBodies(root string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(scanIndexDirectory)), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		files := token.NewFileSet()
		parsed, err := parser.ParseFile(files, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", relative, err)
		}
		violations = append(violations, scanIndexFileViolations(files, parsed, relative)...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan scan-index writer transaction bodies: %w", err)
	}
	return violations, nil
}

func scanIndexFileViolations(files *token.FileSet, parsed *ast.File, relative string) []string {
	var violations []string
	report := func(node ast.Node, reason string) {
		violations = append(violations, fmt.Sprintf("%s:%d:%s", relative, files.Position(node.Pos()).Line, reason))
	}
	// Local names of the imports whose calls are forbidden.
	packageReasons := map[string]string{}
	functionReasons := map[string]map[string]string{}
	for _, imported := range parsed.Imports {
		importPath := strings.Trim(imported.Path.Value, `"`)
		local := importPath[strings.LastIndex(importPath, "/")+1:]
		if imported.Name != nil {
			local = imported.Name.Name
		}
		if reason, ok := transactionForbiddenPackages[importPath]; ok {
			packageReasons[local] = reason
		}
		if functions, ok := transactionForbiddenFunctions[importPath]; ok {
			functionReasons[local] = functions
		}
	}
	inspectBody := func(body *ast.BlockStmt, context string) {
		if body == nil {
			return
		}
		ast.Inspect(body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			name := selector.Sel.Name
			if qualifier, ok := selector.X.(*ast.Ident); ok {
				if reason, ok := packageReasons[qualifier.Name]; ok {
					report(call, fmt.Sprintf("%s calls %s.%s (%s) inside a writer transaction", context, qualifier.Name, name, reason))
					return true
				}
				if reason, ok := functionReasons[qualifier.Name][name]; ok {
					report(call, fmt.Sprintf("%s calls %s.%s (%s) inside a writer transaction", context, qualifier.Name, name, reason))
					return true
				}
			}
			if reason, ok := transactionForbiddenMethods[name]; ok {
				report(call, fmt.Sprintf("%s calls %s (%s) inside a writer transaction", context, name, reason))
				return true
			}
			if strings.HasPrefix(name, "Count") && !transactionAllowedCounts[name] {
				report(call, fmt.Sprintf("%s runs the count query %s inside a writer transaction", context, name))
			}
			return true
		})
	}
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncDecl:
			if strings.HasSuffix(typed.Name.Name, "Tx") {
				inspectBody(typed.Body, typed.Name.Name)
			}
		case *ast.CallExpr:
			selector, ok := typed.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "WithTx" || len(typed.Args) == 0 {
				return true
			}
			if literal, ok := typed.Args[len(typed.Args)-1].(*ast.FuncLit); ok {
				inspectBody(literal.Body, "WithTx body")
			}
		case *ast.ValueSpec:
			for index, name := range typed.Names {
				if name.Name != "MaxBatchRows" || index >= len(typed.Values) {
					continue
				}
				literal, ok := typed.Values[index].(*ast.BasicLit)
				if !ok {
					report(typed, "MaxBatchRows must be a literal row count")
					continue
				}
				if value, err := strconv.Atoi(literal.Value); err != nil || value > scanIndexBatchLimit {
					report(typed, fmt.Sprintf("MaxBatchRows = %s exceeds the %d-row writer batch budget", literal.Value, scanIndexBatchLimit))
				}
			}
		}
		return true
	})
	return violations
}
