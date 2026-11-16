package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"
)

// literalsFindings reports every string literal in scope that spells one of
// the rule's values: equal to one, starting with a prefix, or containing a
// fragment. Struct tags are metadata, not values, and are skipped. A finding
// is keyed by the declaration that holds the literal (a function, or a
// package-level const, var, or type) and the literal itself, so a baseline
// entry survives unrelated edits.
func (a *Atlas) literalsFindings(rule Rule) []Finding {
	config := rule.Literals
	if len(config.In) == 0 || len(config.Equal)+len(config.Prefix)+len(config.Contains) == 0 {
		a.ruleProblem(rule, "literals needs in and at least one of equal, prefix, or contains")
		return nil
	}
	index := a.typed()
	seen := map[string]bool{}
	var findings []Finding
	for _, pkg := range a.goPackages {
		if pkg.typed == nil || pkg.typed.info == nil {
			continue
		}
		for _, file := range pkg.typed.syntax {
			for _, decl := range file.Decls {
				for _, owned := range literalOwners(pkg, decl) {
					tags := map[*ast.BasicLit]bool{}
					ast.Inspect(owned.node, func(node ast.Node) bool {
						if field, ok := node.(*ast.Field); ok && field.Tag != nil {
							tags[field.Tag] = true
						}
						return true
					})
					ast.Inspect(owned.node, func(node ast.Node) bool {
						literal, ok := node.(*ast.BasicLit)
						if !ok || literal.Kind != token.STRING || tags[literal] {
							return true
						}
						value, err := strconv.Unquote(literal.Value)
						if err != nil || !config.matches(value) {
							return true
						}
						position, rel := index.position(pkg, literal.Pos())
						if !inScope(config.In, pkg.dir, rel) || inScope(config.Except, pkg.dir, rel) {
							return true
						}
						key := owned.name + " spells " + strconv.Quote(value)
						if seen[key] {
							return true
						}
						seen[key] = true
						findings = append(findings, Finding{Key: key, Where: where(rel, position), Message: fmt.Sprintf("%s spells %q, which code outside %s must not spell", owned.name, value, strings.Join(config.Except, ", "))})
						return true
					})
				}
			}
		}
	}
	return findings
}

func (config *LiteralsRule) matches(value string) bool {
	for _, equal := range config.Equal {
		if value == equal {
			return true
		}
	}
	for _, prefix := range config.Prefix {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	for _, fragment := range config.Contains {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}

type literalOwner struct {
	name string
	node ast.Node
}

// literalOwners names the declarations a top-level declaration holds: the
// function itself, or each spec of a const, var, or type block.
func literalOwners(pkg *goPackage, decl ast.Decl) []literalOwner {
	switch typed := decl.(type) {
	case *ast.FuncDecl:
		name := pkg.importPath + "." + typed.Name.Name
		if def, ok := pkg.typed.info.Defs[typed.Name].(*types.Func); ok {
			name = funcKey(def)
		}
		return []literalOwner{{name: name, node: typed}}
	case *ast.GenDecl:
		var owners []literalOwner
		for _, spec := range typed.Specs {
			switch spec := spec.(type) {
			case *ast.ValueSpec:
				owners = append(owners, literalOwner{name: pkg.importPath + "." + spec.Names[0].Name, node: spec})
			case *ast.TypeSpec:
				owners = append(owners, literalOwner{name: pkg.importPath + "." + spec.Name.Name, node: spec})
			}
		}
		return owners
	}
	return nil
}
