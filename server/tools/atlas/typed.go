package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// typedPackage is one package as the compiler sees it for the host
// platform: the cgo-processed syntax and the type information go/types
// produced for it. Rules match these objects, never source text.
//
// The map facts (symbols, hashes, line counts, doc links, imports) come from
// the syntactic parse in gofacts.go instead, which reads every non-test file
// regardless of build constraints. That keeps generated files and anchor
// hashes identical on every host; typed facts cover the files the host
// platform builds, which in CI is Linux.
type typedPackage struct {
	fset   *token.FileSet
	types  *types.Package
	info   *types.Info
	syntax []*ast.File
}

const typedLoadMode = packages.NeedName | packages.NeedFiles | packages.NeedImports |
	packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo |
	packages.NeedModule

// loadTyped type-checks every package of one Go module with go/packages and
// returns the module's own packages by import path, plus every type error in
// them or in anything they import. A type error makes rule facts unsound, so
// the caller reports each as a problem.
func loadTyped(root string, module GoModule) (map[string]*typedPackage, []string, error) {
	fset := token.NewFileSet()
	config := &packages.Config{
		Mode: typedLoadMode,
		Dir:  filepath.Join(root, module.Dir),
		Fset: fset,
		Env:  os.Environ(),
	}
	loaded, err := packages.Load(config, "./...")
	if err != nil {
		return nil, nil, fmt.Errorf("type-check module %s: %w", module.Dir, err)
	}
	out := map[string]*typedPackage{}
	var errors []string
	seen := map[string]bool{}
	packages.Visit(loaded, nil, func(pkg *packages.Package) {
		for _, failure := range pkg.Errors {
			message := pkg.PkgPath + ": " + failure.Error()
			if !seen[message] {
				seen[message] = true
				errors = append(errors, message)
			}
		}
	})
	for _, pkg := range loaded {
		if pkg.Module == nil || pkg.Module.Path != module.Path {
			continue
		}
		out[pkg.PkgPath] = &typedPackage{fset: fset, types: pkg.Types, info: pkg.TypesInfo, syntax: pkg.Syntax}
	}
	sort.Strings(errors)
	return out, errors, nil
}

// attachTyped loads typed facts for every Go module and attaches them to the
// mapped packages. A mapped package the host build does not contain, or any
// type error, is a problem: rules over a partial universe would pass for the
// wrong reason.
func (a *Atlas) attachTyped() error {
	for _, module := range a.Config.GoModules {
		typed, errors, err := loadTyped(a.Root, module)
		if err != nil {
			return err
		}
		for _, message := range errors {
			if excludedImport(a, message) {
				continue
			}
			a.problem(module.Dir, "type-check failed: %s (Atlas rules need complete type facts; run through `task atlas:check`, which sets the cgo flags, with the Server's cgo headers installed)", message)
		}
		for _, pkg := range a.goPackages {
			if pkg.module.Path != module.Path {
				continue
			}
			pkg.typed = typed[pkg.importPath]
			if pkg.typed == nil {
				a.problem(pkg.dir, "package is not part of the type-checked build of module %s for this platform", module.Dir)
			}
		}
	}
	return nil
}

// excludedImport reports whether a type error belongs to a package that
// atlas.yaml excludes from the map (developer tools, generated fixtures).
// Their errors cannot reach a mapped package's facts unless a mapped package
// imports them, which the group rule would already reject.
func excludedImport(a *Atlas, message string) bool {
	importPath, _, _ := strings.Cut(message, ": ")
	for _, module := range a.Config.GoModules {
		rest, ok := strings.CutPrefix(importPath, module.Path)
		if !ok || (rest != "" && !strings.HasPrefix(rest, "/")) {
			continue
		}
		return excluded(a.Config.Exclude, module.Dir+rest)
	}
	return false
}
