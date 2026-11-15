package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
)

// typedIndex is the rule engine's view of the typed facts: every function
// body of the mapped packages, every reference a body makes to a function,
// and the module types that can stand behind an interface method. Rules
// match these objects; nothing here reads source text.
type typedIndex struct {
	root     string
	funcs    []*funcBody
	byObject map[*types.Func]*funcBody
	// packages holds every type-checked package by import path, including
	// dependencies, so rule targets can be resolved and validated.
	packages map[string]*types.Package
	// named holds every named type declared in a mapped package, for
	// resolving interface calls to their module implementations.
	named []*types.TypeName

	// mapped holds the import paths of mapped packages.
	mapped map[string]bool

	refCache    map[*funcBody][]reference
	implCache   map[*types.Func][]*types.Func
	lookupCache map[string]*types.Func

	// fieldValues holds, for each func-typed struct field of a mapped
	// package, the function values and closures the module assigns to it;
	// built on first use by the arrow check.
	fieldValues map[*types.Var][]fieldValue
	// fieldParams holds, for each such field, the parameters stored in it
	// (a constructor or setter argument); the values are the arguments its
	// call sites pass.
	fieldParams map[*types.Var][]fieldParam
	// callSites holds every call of a module function, for following a
	// func value passed as an argument.
	callSites map[*types.Func][]callSite
	// paramValues caches the function values passed for a func parameter.
	paramValues map[*types.Var][]fieldValue
}

// callSite is one call expression and the body it is in.
type callSite struct {
	call  *ast.CallExpr
	owner *funcBody
}

// fieldParam is parameter index of fn, stored in a func-typed field.
type fieldParam struct {
	fn    *types.Func
	index int
}

// fieldValue is one function value stored in a func-typed field: a named
// function or method value, or a closure inside the body that stores it.
type fieldValue struct {
	fn      *types.Func
	closure *ast.FuncLit
	owner   *funcBody
}

// funcBody is one function declaration of a mapped package, or the
// package-level initialization of one file (name "<pkg>.init").
type funcBody struct {
	pkg  *goPackage
	obj  *types.Func
	key  string
	decl *ast.FuncDecl
	body ast.Node
	file string
	pos  token.Position
}

// reference is one use of a function or method inside a body.
type reference struct {
	callee *types.Func
	call   *ast.CallExpr
	pos    token.Position
	file   string
}

func (a *Atlas) typed() *typedIndex {
	if a.typedIdx != nil {
		return a.typedIdx
	}
	index := &typedIndex{
		root: a.Root, byObject: map[*types.Func]*funcBody{}, packages: map[string]*types.Package{},
		refCache: map[*funcBody][]reference{}, implCache: map[*types.Func][]*types.Func{},
		lookupCache: map[string]*types.Func{}, mapped: map[string]bool{},
	}
	for _, pkg := range a.goPackages {
		if pkg.typed == nil || pkg.typed.types == nil {
			continue
		}
		index.mapped[pkg.typed.types.Path()] = true
		var visit func(*types.Package)
		visit = func(p *types.Package) {
			if index.packages[p.Path()] != nil {
				return
			}
			index.packages[p.Path()] = p
			for _, imported := range p.Imports() {
				visit(imported)
			}
		}
		visit(pkg.typed.types)
		scope := pkg.typed.types.Scope()
		for _, name := range scope.Names() {
			if typeName, ok := scope.Lookup(name).(*types.TypeName); ok && !typeName.IsAlias() {
				index.named = append(index.named, typeName)
			}
		}
		for _, file := range pkg.typed.syntax {
			for _, decl := range file.Decls {
				switch typed := decl.(type) {
				case *ast.FuncDecl:
					obj, _ := pkg.typed.info.Defs[typed.Name].(*types.Func)
					if obj == nil || typed.Body == nil {
						continue
					}
					body := &funcBody{pkg: pkg, obj: obj, key: funcKey(obj), decl: typed, body: typed.Body}
					body.pos = pkg.typed.fset.Position(typed.Pos())
					body.file = index.relFile(body.pos.Filename)
					index.funcs = append(index.funcs, body)
					index.byObject[obj] = body
				case *ast.GenDecl:
					if typed.Tok != token.VAR {
						continue
					}
					body := &funcBody{pkg: pkg, key: pkg.importPath + ".init", body: typed}
					body.pos = pkg.typed.fset.Position(typed.Pos())
					body.file = index.relFile(body.pos.Filename)
					index.funcs = append(index.funcs, body)
				}
			}
		}
	}
	sort.SliceStable(index.funcs, func(i, j int) bool { return index.funcs[i].key < index.funcs[j].key })
	a.typedIdx = index
	return index
}

func (index *typedIndex) relFile(filename string) string {
	if rel, err := filepath.Rel(index.root, filename); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(filename)
}

func (index *typedIndex) position(pkg *goPackage, pos token.Pos) (token.Position, string) {
	position := pkg.typed.fset.Position(pos)
	return position, index.relFile(position.Filename)
}

// where renders a finding location.
func where(file string, position token.Position) string {
	return fmt.Sprintf("%s:%d", file, position.Line)
}

// references lists every function or method a node refers to: calls,
// method values, and function values alike, so passing a function along
// cannot hide a use. call is set when the reference is called directly.
func (index *typedIndex) references(fn *funcBody, node ast.Node) []reference {
	if node == fn.body {
		if cached, ok := index.refCache[fn]; ok {
			return cached
		}
		refs := index.collectReferences(fn, node)
		index.refCache[fn] = refs
		return refs
	}
	return index.collectReferences(fn, node)
}

func (index *typedIndex) collectReferences(fn *funcBody, node ast.Node) []reference {
	info := fn.pkg.typed.info
	calls := map[*ast.Ident]*ast.CallExpr{}
	var idents []*ast.Ident
	ast.Inspect(node, func(n ast.Node) bool {
		switch typed := n.(type) {
		case *ast.CallExpr:
			if ident := calleeIdent(typed.Fun); ident != nil {
				calls[ident] = typed
			}
		case *ast.Ident:
			idents = append(idents, typed)
		}
		return true
	})
	var out []reference
	for _, ident := range idents {
		obj, ok := info.Uses[ident].(*types.Func)
		if !ok {
			continue
		}
		position, file := index.position(fn.pkg, ident.Pos())
		out = append(out, reference{callee: obj.Origin(), call: calls[ident], pos: position, file: file})
	}
	return out
}

// calleeIdent returns the identifier that names a call's function:
// F, x.M, pkg.F, or an instantiation F[T].
func calleeIdent(fun ast.Expr) *ast.Ident {
	switch typed := ast.Unparen(fun).(type) {
	case *ast.Ident:
		return typed
	case *ast.SelectorExpr:
		return typed.Sel
	case *ast.IndexExpr:
		return calleeIdent(typed.X)
	case *ast.IndexListExpr:
		return calleeIdent(typed.X)
	}
	return nil
}

// funcKey names a function the way rules do: "pkg/path.Name" for a
// function and "(*pkg/path.Type).Name" or "(pkg/path.Type).Name" for a
// method, with type parameters dropped so generic receivers read like
// ordinary ones.
func funcKey(fn *types.Func) string {
	fn = fn.Origin()
	signature, _ := fn.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		if fn.Pkg() == nil {
			return fn.Name()
		}
		return fn.Pkg().Path() + "." + fn.Name()
	}
	return "(" + typeKey(signature.Recv().Type()) + ")." + fn.Name()
}

// typeKey names a type with full package paths and no type arguments:
// "*database/sql.DB", "server/internal/work/execution.Resources".
func typeKey(t types.Type) string {
	switch typed := t.(type) {
	case *types.Pointer:
		return "*" + typeKey(typed.Elem())
	case *types.Named:
		obj := typed.Obj()
		if obj.Pkg() == nil {
			return obj.Name()
		}
		return obj.Pkg().Path() + "." + obj.Name()
	case *types.Alias:
		return typeKey(types.Unalias(typed))
	}
	return types.TypeString(t, nil)
}

// receiverNamed returns the named type of a method's receiver, or nil.
func receiverNamed(fn *types.Func) *types.Named {
	signature, _ := fn.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return nil
	}
	t := signature.Recv().Type()
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, _ := types.Unalias(t).(*types.Named)
	return named
}

// isInterfaceMethod reports whether fn is an abstract interface method.
func isInterfaceMethod(fn *types.Func) bool {
	named := receiverNamed(fn)
	if named == nil {
		signature, _ := fn.Type().(*types.Signature)
		return signature != nil && signature.Recv() != nil && types.IsInterface(signature.Recv().Type())
	}
	return types.IsInterface(named)
}

// implementations resolves an interface method to the methods of every
// mapped named type that implements the interface (by value or pointer).
// Only interfaces declared in mapped packages are resolved: a library
// interface such as io.Writer is implemented by too many module types for
// class-hierarchy analysis to say anything about one call.
func (index *typedIndex) implementations(method *types.Func) []*types.Func {
	if cached, ok := index.implCache[method]; ok {
		return cached
	}
	if method.Pkg() == nil || !index.mapped[method.Pkg().Path()] {
		index.implCache[method] = nil
		return nil
	}
	out := index.findImplementations(method)
	index.implCache[method] = out
	return out
}

func (index *typedIndex) findImplementations(method *types.Func) []*types.Func {
	signature, _ := method.Type().(*types.Signature)
	if signature == nil || signature.Recv() == nil {
		return nil
	}
	iface, _ := signature.Recv().Type().Underlying().(*types.Interface)
	if iface == nil {
		return nil
	}
	var out []*types.Func
	for _, typeName := range index.named {
		named, ok := typeName.Type().(*types.Named)
		if !ok || types.IsInterface(named) || named.TypeParams().Len() > 0 {
			continue
		}
		var t types.Type = named
		var implementer types.Type
		switch {
		case types.Implements(t, iface):
			implementer = t
		case types.Implements(types.NewPointer(t), iface):
			implementer = types.NewPointer(t)
		default:
			continue
		}
		obj, _, _ := types.LookupFieldOrMethod(implementer, true, method.Pkg(), method.Name())
		if fn, ok := obj.(*types.Func); ok {
			out = append(out, fn.Origin())
		}
	}
	return out
}

// lookupFunc resolves a rule target such as "time.Sleep",
// "(*database/sql.DB).BeginTx", or "(server/internal/x.Iface).Method" in
// the type-checked universe.
func (index *typedIndex) lookupFunc(key string) *types.Func {
	if strings.HasPrefix(key, "(") {
		end := strings.Index(key, ").")
		if end < 0 {
			return nil
		}
		receiver, name := key[1:end], key[end+2:]
		named := index.lookupType(strings.TrimPrefix(receiver, "*"))
		if named == nil {
			return nil
		}
		var t types.Type = named
		if strings.HasPrefix(receiver, "*") {
			t = types.NewPointer(named)
		}
		obj, _, _ := types.LookupFieldOrMethod(t, true, named.(*types.Named).Obj().Pkg(), name)
		fn, _ := obj.(*types.Func)
		return fn
	}
	dot := strings.LastIndex(key, ".")
	if dot < 0 {
		return nil
	}
	pkg := index.packages[key[:dot]]
	if pkg == nil {
		return nil
	}
	fn, _ := pkg.Scope().Lookup(key[dot+1:]).(*types.Func)
	return fn
}

// lookupType resolves "pkg/path.Name" to a named type.
func (index *typedIndex) lookupType(key string) types.Type {
	dot := strings.LastIndex(key, ".")
	if dot < 0 {
		return nil
	}
	pkg := index.packages[key[:dot]]
	if pkg == nil {
		return nil
	}
	typeName, _ := pkg.Scope().Lookup(key[dot+1:]).(*types.TypeName)
	if typeName == nil {
		return nil
	}
	named, _ := types.Unalias(typeName.Type()).(*types.Named)
	if named == nil {
		return nil
	}
	return named
}

// validTypeKey reports whether "*pkg.Name" or "pkg.Name" resolves.
func (index *typedIndex) validTypeKey(key string) bool {
	return index.lookupType(strings.TrimPrefix(key, "*")) != nil
}

// matchesTarget reports whether a referenced function is one of the
// targets: the target itself, or an interface method that a target
// implements, so calling through an interface cannot hide a use.
func (index *typedIndex) matchesTarget(callee *types.Func, targets map[string]bool) (string, bool) {
	key := funcKey(callee)
	if targets[key] {
		return key, true
	}
	if !isInterfaceMethod(callee) {
		return "", false
	}
	signature, _ := callee.Type().(*types.Signature)
	iface, _ := signature.Recv().Type().Underlying().(*types.Interface)
	if iface == nil {
		return "", false
	}
	// A target method whose receiver satisfies the called interface: this
	// covers targets outside the module, such as (*database/sql.DB).BeginTx
	// called through a local interface.
	for _, target := range sortedKeys(targets) {
		fn := index.lookupCached(target)
		if fn == nil || fn.Name() != callee.Name() {
			continue
		}
		recv := fn.Type().(*types.Signature).Recv()
		if recv != nil && types.Implements(recv.Type(), iface) {
			return target, true
		}
	}
	return "", false
}

func (index *typedIndex) lookupCached(key string) *types.Func {
	if fn, ok := index.lookupCache[key]; ok {
		return fn
	}
	fn := index.lookupFunc(key)
	index.lookupCache[key] = fn
	return fn
}

// inScope reports whether a package directory or file matches any of the
// scope patterns: package directories, globs (dir/**), or file paths.
func inScope(patterns []string, pkgDir, file string) bool {
	return excluded(patterns, pkgDir) || (file != "" && excluded(patterns, file))
}
