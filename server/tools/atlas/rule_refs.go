package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

// The reference rules: calls, constructs, forbid_types, and forbid_imports.
// Each matches go/types objects of the mapped packages. A target that does
// not resolve is a problem, so a rename cannot silently disable a rule.

func (a *Atlas) ruleProblem(rule Rule, format string, args ...any) {
	a.Problems = append(a.Problems, Problem{Where: "atlas.yaml", Rule: rule.ID, Message: fmt.Sprintf(format, args...)})
}

// resolveTargets validates function targets and returns them as a set.
func (a *Atlas) resolveTargets(rule Rule, targets []string) map[string]bool {
	index := a.typed()
	set := map[string]bool{}
	for _, target := range targets {
		fn := index.lookupFunc(target)
		if fn == nil {
			a.ruleProblem(rule, "target %s does not resolve to a function or method", target)
			continue
		}
		set[funcKey(fn)] = true
	}
	return set
}

func (a *Atlas) callsFindings(rule Rule) []Finding {
	config := rule.Calls
	if len(config.Targets) == 0 || (len(config.Only) == 0) == (len(config.Forbid) == 0) {
		a.ruleProblem(rule, "calls needs targets and exactly one of only or forbid")
		return nil
	}
	targets := a.resolveTargets(rule, config.Targets)
	index := a.typed()
	seen := map[string]bool{}
	var findings []Finding
	for _, fn := range index.funcs {
		for _, ref := range index.references(fn, fn.body) {
			target, ok := index.matchesTarget(ref.callee, targets)
			if !ok {
				continue
			}
			allowed := inScope(config.Only, fn.pkg.dir, ref.file)
			if len(config.Forbid) > 0 {
				allowed = !inScope(config.Forbid, fn.pkg.dir, ref.file)
			}
			if allowed {
				continue
			}
			key := fn.key + " -> " + target
			if seen[key] {
				continue
			}
			seen[key] = true
			findings = append(findings, Finding{Key: key, Where: where(ref.file, ref.pos), Message: fmt.Sprintf("%s refers to %s, which only %s may use", fn.key, target, scopeText(config))})
		}
	}
	return findings
}

func scopeText(config *CallsRule) string {
	if len(config.Only) > 0 {
		return strings.Join(config.Only, ", ")
	}
	return "code outside " + strings.Join(config.Forbid, ", ")
}

func (a *Atlas) constructsFindings(rule Rule) []Finding {
	config := rule.Constructs
	if len(config.Types) == 0 || len(config.Only) == 0 {
		a.ruleProblem(rule, "constructs needs types and only")
		return nil
	}
	index := a.typed()
	targets := map[string]bool{}
	for _, target := range config.Types {
		if !index.validTypeKey(target) || strings.HasPrefix(target, "*") {
			a.ruleProblem(rule, "type %s does not resolve to a named type", target)
			continue
		}
		targets[target] = true
	}
	seen := map[string]bool{}
	var findings []Finding
	for _, fn := range index.funcs {
		info := fn.pkg.typed.info
		ast.Inspect(fn.body, func(node ast.Node) bool {
			var constructed types.Type
			switch typed := node.(type) {
			case *ast.CompositeLit:
				constructed = info.TypeOf(typed)
			case *ast.CallExpr:
				if ident, ok := ast.Unparen(typed.Fun).(*ast.Ident); ok && len(typed.Args) == 1 {
					if builtin, ok := info.Uses[ident].(*types.Builtin); ok && builtin.Name() == "new" {
						constructed = info.TypeOf(typed.Args[0])
					}
				}
			}
			if constructed == nil {
				return true
			}
			key := typeKey(constructed)
			if !targets[key] {
				return true
			}
			position, file := index.position(fn.pkg, node.Pos())
			if inScope(config.Only, fn.pkg.dir, file) {
				return true
			}
			findingKey := fn.key + " constructs " + key
			if !seen[findingKey] {
				seen[findingKey] = true
				findings = append(findings, Finding{Key: findingKey, Where: where(file, position), Message: fmt.Sprintf("%s builds a %s, which only %s may construct", fn.key, key, strings.Join(config.Only, ", "))})
			}
			return true
		})
	}
	return findings
}

func (a *Atlas) forbidTypesFindings(rule Rule) []Finding {
	config := rule.ForbidTypes
	if len(config.Types) == 0 || len(config.In) == 0 {
		a.ruleProblem(rule, "forbid_types needs in and types")
		return nil
	}
	index := a.typed()
	targets := map[string]bool{}
	for _, target := range config.Types {
		if !index.validTypeKey(target) {
			a.ruleProblem(rule, "type %s does not resolve to a named type", target)
			continue
		}
		targets[target] = true
	}
	var findings []Finding
	seen := map[string]bool{}
	for _, pkg := range a.goPackages {
		if pkg.typed == nil || pkg.typed.info == nil {
			continue
		}
		var idents []*ast.Ident
		for ident := range pkg.typed.info.Defs {
			idents = append(idents, ident)
		}
		sort.Slice(idents, func(i, j int) bool { return idents[i].Pos() < idents[j].Pos() })
		report := func(name string, t types.Type, pos token.Pos) {
			position, file := index.position(pkg, pos)
			if !inScope(config.In, pkg.dir, file) {
				return
			}
			held := mentionedType(t, targets)
			if held == "" {
				return
			}
			key := name + " holds " + held
			if seen[key] {
				return
			}
			seen[key] = true
			findings = append(findings, Finding{Key: key, Where: where(file, position), Message: fmt.Sprintf("%s holds %s; code here may own only narrow interfaces, not that capability", name, held)})
		}
		for _, ident := range idents {
			switch obj := pkg.typed.info.Defs[ident].(type) {
			case *types.Func:
				// Parameters and results, named or not.
				report(funcKey(obj), obj.Type(), ident.Pos())
			case *types.Var:
				switch obj.Kind() {
				case types.ParamVar, types.ResultVar, types.RecvVar:
					continue
				}
				report(varKey(pkg, obj), obj.Type(), ident.Pos())
			}
		}
		for _, file := range pkg.typed.syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				if literal, ok := node.(*ast.FuncLit); ok {
					name := pkg.importPath + " func literal"
					if fn := enclosingFuncAt(pkg, literal.Pos()); fn != "" {
						name = fn + " func literal"
					}
					report(name, pkg.typed.info.TypeOf(literal), literal.Pos())
				}
				return true
			})
		}
	}
	return findings
}

// mentionedType returns the first target a type mentions through pointers,
// slices, arrays, maps, channels, function signatures, and unnamed structs.
// It does not look inside other named types: holding a type that wraps a
// capability is that type's own concern.
func mentionedType(t types.Type, targets map[string]bool) string {
	if key := typeKey(t); targets[key] {
		return key
	}
	switch typed := types.Unalias(t).(type) {
	case *types.Pointer:
		return mentionedType(typed.Elem(), targets)
	case *types.Slice:
		return mentionedType(typed.Elem(), targets)
	case *types.Array:
		return mentionedType(typed.Elem(), targets)
	case *types.Chan:
		return mentionedType(typed.Elem(), targets)
	case *types.Map:
		if held := mentionedType(typed.Key(), targets); held != "" {
			return held
		}
		return mentionedType(typed.Elem(), targets)
	case *types.Signature:
		for _, tuple := range []*types.Tuple{typed.Params(), typed.Results()} {
			for i := 0; i < tuple.Len(); i++ {
				if held := mentionedType(tuple.At(i).Type(), targets); held != "" {
					return held
				}
			}
		}
	case *types.Struct:
		for i := 0; i < typed.NumFields(); i++ {
			if held := mentionedType(typed.Field(i).Type(), targets); held != "" {
				return held
			}
		}
	}
	return ""
}

// varKey names a declared variable: a field as pkg.Type.field, anything
// else as its enclosing function plus its name.
func varKey(pkg *goPackage, obj *types.Var) string {
	if obj.IsField() {
		if owner := fieldOwner(pkg, obj); owner != "" {
			return pkg.importPath + "." + owner + "." + obj.Name()
		}
	}
	if obj.Parent() == pkg.typed.types.Scope() {
		return pkg.importPath + "." + obj.Name()
	}
	if fn := enclosingFuncAt(pkg, obj.Pos()); fn != "" {
		return fn + " " + obj.Name()
	}
	return pkg.importPath + " " + obj.Name()
}

func fieldOwner(pkg *goPackage, field *types.Var) string {
	scope := pkg.typed.types.Scope()
	for _, name := range scope.Names() {
		typeName, ok := scope.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if structType, ok := typeName.Type().Underlying().(*types.Struct); ok {
			for i := 0; i < structType.NumFields(); i++ {
				if structType.Field(i) == field {
					return name
				}
			}
		}
	}
	return ""
}

func enclosingFuncAt(pkg *goPackage, pos token.Pos) string {
	for _, file := range pkg.typed.syntax {
		if file.Pos() > pos || pos > file.End() {
			continue
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Pos() <= pos && pos <= fn.End() {
				if def, ok := pkg.typed.info.Defs[fn.Name].(*types.Func); ok {
					return funcKey(def)
				}
			}
		}
	}
	return ""
}

func (a *Atlas) forbidImportsFindings(rule Rule) []Finding {
	config := rule.ForbidImports
	if len(config.From) == 0 || len(config.Imports) == 0 {
		a.ruleProblem(rule, "forbid_imports needs from and imports")
		return nil
	}
	var findings []Finding
	seen := map[string]bool{}
	for _, pkg := range a.goPackages {
		for _, fileRel := range sortedKeys(pkg.files) {
			if !inScope(config.From, pkg.dir, fileRel) {
				continue
			}
			for _, spec := range pkg.files[fileRel].Imports {
				imported := strings.Trim(spec.Path.Value, `"`)
				if !excluded(config.Imports, imported) {
					continue
				}
				key := fileRel + " imports " + imported
				if excluded(config.From, pkg.dir) {
					key = pkg.dir + " imports " + imported
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				line := pkg.fset.Position(spec.Pos()).Line
				findings = append(findings, Finding{Key: key, Where: fmt.Sprintf("%s:%d", fileRel, line), Message: fmt.Sprintf("imports %s, which %s must not import", imported, strings.Join(config.From, ", "))})
			}
		}
	}
	return findings
}
