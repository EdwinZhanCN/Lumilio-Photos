package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
)

// An arrow is a drawn claim that code at one end reaches the other: a
// sequence message or a data-flow edge. When both ends are go: anchors the
// claim is checked against the typed facts, so a removed call breaks the
// view instead of silently outliving the code.
//
// A hop X -> Y holds when a function body of X references Y: Y itself, a
// method of Y, or an interface method that Y implements (or, for an
// interface Y, a method of a type that implements it). The body of a
// function or method is its declaration; the body of a type is every method
// declared on it; the body of an interface is the methods of every module
// type that implements it. An arrow is the chain from, via..., to, and each
// hop must hold. Replies run the chain backwards (the caller references the
// callee). A data-flow edge holds in either direction, since data moves
// along a call both as arguments and as results.
//
// The arrow's own anchor narrows the check on the callee side: when it is
// a method of the callee end, the last hop must reference that method, so a
// message labelled with one call fails when the code makes another. On the
// caller end or a via hop an anchor only marks where the next call
// happens, which may be inside a callback handed over. An anchor on
// neither end, on an arrow without via, says how the ends meet without a
// call: a type is the message they exchange (a River job's args type), so
// both ends must reference it; a function is the wiring site that connects
// them (it hands one end's method to the other as a func value), so it must
// reference both ends. A failed hop reports the shortest call chain the
// code does have, ready to become via.
type arrow struct {
	view       *View
	from, to   string
	label      string
	anchor     string
	via        []string
	reply      bool
	undirected bool
}

func (a *Atlas) checkArrows() {
	index := a.typed()
	if len(index.mapped) == 0 {
		return
	}
	for _, view := range a.Views {
		if view.Derived {
			continue
		}
		nodes := map[string]string{}
		for _, node := range view.Nodes {
			nodes[node.ID] = node.Anchor
		}
		var arrows []arrow
		switch view.Kind {
		case "sequence":
			arrows = sequenceArrows(view, view.Steps)
		case "dataflow":
			for _, edge := range view.Edges {
				arrows = append(arrows, arrow{view: view, from: edge.From, to: edge.To, label: edge.Label, anchor: edge.Anchor, via: edge.Via, undirected: true})
			}
		default:
			for _, edge := range view.Edges {
				if len(edge.Via) > 0 {
					a.problem(view.Source, "edge %s -> %s: via is only checked in sequence and data-flow views", edge.From, edge.To)
				}
			}
		}
		for _, drawn := range arrows {
			a.checkArrow(index, drawn, nodes[drawn.from], nodes[drawn.to])
		}
	}
}

func sequenceArrows(view *View, steps []Step) []arrow {
	var out []arrow
	for _, step := range steps {
		switch {
		case step.Block != "":
			out = append(out, sequenceArrows(view, step.Steps)...)
			for _, branch := range step.Else {
				out = append(out, sequenceArrows(view, branch.Steps)...)
			}
		case step.Note == "":
			out = append(out, arrow{view: view, from: step.From, to: step.To, label: step.Label, anchor: step.Anchor, via: step.Via, reply: step.Reply})
		}
	}
	return out
}

// arrowEnd is one element of an arrow's chain resolved to typed objects.
type arrowEnd struct {
	anchor string
	obj    types.Object
}

func (a *Atlas) checkArrow(index *typedIndex, drawn arrow, fromAnchor, toAnchor string) {
	name := fmt.Sprintf("%s -> %s (%q)", drawn.from, drawn.to, drawn.label)
	if !a.verifiableEnd(fromAnchor) || !a.verifiableEnd(toAnchor) || drawn.from == drawn.to {
		if len(drawn.via) > 0 {
			a.problem(drawn.view.Source, "arrow %s: via is only checked between two go: or Go mod: participants", name)
		}
		return
	}
	anchors := append(append([]string{fromAnchor}, drawn.via...), toAnchor)
	if drawn.reply {
		for left, right := 0, len(anchors)-1; left < right; left, right = left+1, right-1 {
			anchors[left], anchors[right] = anchors[right], anchors[left]
		}
	}
	chain := make([]arrowEnd, 0, len(anchors))
	for _, anchor := range anchors {
		if !a.verifiableEnd(anchor) {
			a.problem(drawn.view.Source, "arrow %s: via %s must be a go: or Go mod: anchor", name, anchor)
			return
		}
		obj, err := a.typedObject(index, anchor)
		if err != nil {
			a.problem(drawn.view.Source, "arrow %s: %v", name, err)
			return
		}
		chain = append(chain, arrowEnd{anchor: anchor, obj: obj})
	}

	var narrowed types.Object
	narrowAt := -1
	if strings.HasPrefix(drawn.anchor, "go:") {
		obj, err := a.typedObject(index, drawn.anchor)
		if err != nil {
			a.problem(drawn.view.Source, "arrow %s: %v", name, err)
			return
		}
		owned := false
		for position, end := range chain {
			if obj == end.obj || ownedBy(obj, end.obj) {
				owned = true
				// Only the callee end narrows; elsewhere the anchor marks
				// where the next call is made.
				if position == len(chain)-1 {
					narrowed, narrowAt = obj, position
				}
				break
			}
		}
		if !owned {
			if len(drawn.via) > 0 {
				a.problem(drawn.view.Source, "arrow %s: anchor %s is neither end nor a via hop", name, drawn.anchor)
				return
			}
			ends := []arrowEnd{chain[0], chain[len(chain)-1]}
			if _, message := obj.(*types.TypeName); message {
				for _, end := range ends {
					if !index.namesDirectly(index.bodiesOf(end.obj), obj) {
						a.problem(drawn.view.Source, "arrow %s: %s does not reference the message %s that the arrow is anchored at", name, end.anchor, drawn.anchor)
					}
				}
				return
			}
			for _, end := range ends {
				if !index.refersTo(index.bodiesOf(obj), end.obj) {
					a.problem(drawn.view.Source, "arrow %s: the wiring site %s does not reference %s%s", name, drawn.anchor, end.anchor, a.suggestChain(index, obj, end.obj))
				}
			}
			return
		}
	}

	for hop := 0; hop+1 < len(chain); hop++ {
		caller, callee := chain[hop], chain[hop+1]
		bodies := index.bodiesOf(caller.obj)
		target := callee.obj
		if narrowAt == hop+1 {
			target = narrowed
		}
		if index.refersTo(bodies, target) {
			continue
		}
		if drawn.undirected && index.refersTo(index.bodiesOf(callee.obj), caller.obj) {
			continue
		}
		if len(bodies) == 0 && !drawn.undirected {
			a.problem(drawn.view.Source, "arrow %s: %s has no function body to verify; anchor the participant at a function, method, or a type with methods", name, caller.anchor)
			continue
		}
		hint := a.suggestChain(index, caller.obj, target)
		if drawn.undirected && !strings.Contains(hint, "via:") {
			if reverse := a.suggestChain(index, callee.obj, caller.obj); strings.Contains(reverse, "via:") {
				hint = strings.Replace(reverse, "the code reaches it", "from the other end, the code reaches it", 1)
			}
		}
		a.problem(drawn.view.Source, "arrow %s is not in the code: nothing in %s references %s%s", name, caller.anchor, objectName(target), hint)
	}
}

// verifiableEnd reports whether an anchor names Go code: a go: declaration
// or the mod: anchor of a Go package.
func (a *Atlas) verifiableEnd(anchor string) bool {
	if strings.HasPrefix(anchor, "go:") {
		return true
	}
	if !strings.HasPrefix(anchor, "mod:") {
		return false
	}
	resolved, err := a.resolveAnchor(anchor)
	return err == nil && a.Modules[resolved.Module] != nil && a.Modules[resolved.Module].ImportPath != ""
}

// typedObject resolves a go: anchor to its object in the typed facts, and
// a mod: anchor to a package name standing for every declaration of the
// package.
func (a *Atlas) typedObject(index *typedIndex, anchor string) (types.Object, error) {
	resolved, err := a.resolveAnchor(anchor)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(anchor, "mod:") {
		importPath := a.Modules[resolved.Module].ImportPath
		for _, candidate := range a.goPackages {
			if candidate.importPath == importPath && candidate.typed != nil {
				pkg := candidate.typed.types
				return types.NewPkgName(0, pkg, pkg.Name(), pkg), nil
			}
		}
		return nil, fmt.Errorf("%s: package %s has no typed facts", anchor, importPath)
	}
	symbol := resolved.symbol
	if symbol == nil {
		return nil, fmt.Errorf("%s is not a Go declaration", anchor)
	}
	importPath := strings.TrimSuffix(strings.TrimPrefix(symbol.Key, "go:"), "."+symbol.Name)
	// The package's own load, not index.packages: Desktop imports the
	// Server, so a Server package also exists as a copy from Desktop's load
	// whose objects no mapped body uses.
	var pkg *types.Package
	for _, candidate := range a.goPackages {
		if candidate.importPath == importPath && candidate.typed != nil {
			pkg = candidate.typed.types
		}
	}
	if pkg == nil {
		return nil, fmt.Errorf("%s: package %s has no typed facts", anchor, importPath)
	}
	name, member, hasMember := strings.Cut(symbol.Name, ".")
	obj := pkg.Scope().Lookup(name)
	if obj == nil {
		return nil, fmt.Errorf("%s is not declared in the files this platform builds", anchor)
	}
	if !hasMember {
		return obj, nil
	}
	typeName, ok := obj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("%s: %s is not a type", anchor, name)
	}
	found, _, _ := types.LookupFieldOrMethod(types.NewPointer(typeName.Type()), true, pkg, member)
	if found == nil {
		found, _, _ = types.LookupFieldOrMethod(typeName.Type(), true, pkg, member)
	}
	if found == nil {
		return nil, fmt.Errorf("%s: %s has no member %s", anchor, name, member)
	}
	return found, nil
}

// ownedBy reports whether obj is a method declared on the type owner, or,
// for an interface owner, a method of a type that implements it: the
// unexported implementation behind an exported service interface.
func ownedBy(obj, owner types.Object) bool {
	if pkgName, ok := owner.(*types.PkgName); ok {
		return obj.Pkg() == pkgName.Imported()
	}
	fn, ok := obj.(*types.Func)
	typeName, isType := owner.(*types.TypeName)
	if !ok || !isType {
		return false
	}
	named := receiverNamed(fn)
	if named == nil {
		return false
	}
	if named.Origin().Obj() == typeName {
		return true
	}
	iface, isInterface := typeName.Type().Underlying().(*types.Interface)
	if !isInterface || types.IsInterface(named) || iface.NumMethods() == 0 {
		return false
	}
	return types.Implements(named, iface) || types.Implements(types.NewPointer(named), iface)
}

// bodiesOf returns the function bodies that speak for obj: a function's
// own declaration, the methods of a type, every method of each module type
// that implements an interface, or the implementations of an interface
// method.
func (index *typedIndex) bodiesOf(obj types.Object) []*funcBody {
	switch typed := obj.(type) {
	case *types.PkgName:
		var out []*funcBody
		for _, body := range index.funcs {
			if body.obj != nil && body.pkg.typed.types == typed.Imported() {
				out = append(out, body)
			}
		}
		return out
	case *types.Func:
		if isInterfaceMethod(typed) {
			var out []*funcBody
			for _, implementation := range index.implementations(typed) {
				if body := index.byObject[implementation]; body != nil {
					out = append(out, body)
				}
			}
			return out
		}
		if body := index.byObject[typed.Origin()]; body != nil {
			return []*funcBody{body}
		}
	case *types.TypeName:
		var out []*funcBody
		for _, body := range index.funcs {
			if body.obj != nil && ownedBy(body.obj, typed) {
				out = append(out, body)
			}
		}
		return out
	}
	return nil
}

// refersTo reports whether any body references target: the object itself,
// a method of a target type, an interface method the target implements, or
// a method of a type implementing a target interface. A use of a
// func-typed field reaches whatever the module stores in that field, the
// way an interface call reaches its implementations: a worker's Execute
// field filled by server/app with a method value calls that method.
func (index *typedIndex) refersTo(bodies []*funcBody, target types.Object) bool {
	return index.bodiesReach(bodies, target, true)
}

// namesDirectly reports whether a body itself names target, without
// following stored or passed function values: the test for a message,
// which each end's own code must name.
func (index *typedIndex) namesDirectly(bodies []*funcBody, target types.Object) bool {
	return index.bodiesReach(bodies, target, false)
}

func (index *typedIndex) bodiesReach(bodies []*funcBody, target types.Object, follow bool) bool {
	for _, body := range bodies {
		// The signature counts: a worker names its job's args type in its
		// parameters. The receiver does not, or every method would
		// reference its own type.
		var roots []ast.Node
		if body.decl != nil {
			roots = append(roots, body.decl.Type)
		}
		roots = append(roots, body.body)
		for _, root := range roots {
			if index.nodeReaches(body, root, target, 0, follow) {
				return true
			}
		}
	}
	return false
}

func (index *typedIndex) nodeReaches(body *funcBody, root ast.Node, target types.Object, depth int, follow bool) bool {
	info := body.pkg.typed.info
	// A stored or passed function value is followed only where it is
	// called, not where it is merely mentioned or assigned.
	called := map[*ast.Ident]bool{}
	ast.Inspect(root, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			if ident := calleeIdent(call.Fun); ident != nil {
				called[ident] = true
			}
		}
		return true
	})
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if found {
			return false
		}
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		used := info.Uses[ident]
		if used == nil {
			return true
		}
		if index.arrowReaches(used, target) {
			found = true
			return false
		}
		if variable, ok := used.(*types.Var); ok && follow && called[ident] && depth < 3 {
			var values []fieldValue
			switch {
			case variable.IsField():
				values = index.valuesOf(variable)
			case body.obj != nil:
				values = index.argumentsFor(body.obj, variable)
			}
			for _, value := range values {
				switch {
				case value.fn != nil && index.arrowReaches(value.fn, target):
					found = true
				case value.fn != nil && depth < 2 && index.byObject[value.fn] != nil && returnsFunc(value.fn) && index.nodeReaches(index.byObject[value.fn], index.byObject[value.fn].body, target, depth+1, follow):
					found = true
				case value.closure != nil && index.nodeReaches(value.owner, value.closure.Body, target, depth+1, follow):
					found = true
				}
				if found {
					return false
				}
			}
		}
		return true
	})
	return found
}

// valuesOf returns the function values stored in a func-typed field.
func (index *typedIndex) valuesOf(field *types.Var) []fieldValue {
	if index.fieldValues == nil {
		index.fieldValues = map[*types.Var][]fieldValue{}
		index.fieldParams = map[*types.Var][]fieldParam{}
		for _, body := range index.funcs {
			index.collectFieldValues(body)
		}
		index.resolveFieldParams()
	}
	return index.fieldValues[field.Origin()]
}

// resolveFieldParams turns each parameter stored in a field into the
// function values the module passes for it.
func (index *typedIndex) resolveFieldParams() {
	for field, params := range index.fieldParams {
		for _, param := range params {
			index.fieldValues[field] = append(index.fieldValues[field], index.argumentValues(param.fn, param.index)...)
		}
	}
}

// argumentsFor returns the function values call sites pass for a func
// parameter of fn; nil when variable is not one of fn's parameters.
func (index *typedIndex) argumentsFor(fn *types.Func, variable *types.Var) []fieldValue {
	if _, isFunc := variable.Type().Underlying().(*types.Signature); !isFunc {
		return nil
	}
	if cached, ok := index.paramValues[variable]; ok {
		return cached
	}
	if index.paramValues == nil {
		index.paramValues = map[*types.Var][]fieldValue{}
	}
	index.paramValues[variable] = nil
	params := fn.Type().(*types.Signature).Params()
	var values []fieldValue
	for position := range params.Len() {
		if params.At(position) == variable {
			values = index.argumentValues(fn.Origin(), position)
		}
	}
	index.paramValues[variable] = values
	return values
}

// argumentValues returns the function values passed as argument position
// at every call of fn.
func (index *typedIndex) argumentValues(fn *types.Func, position int) []fieldValue {
	if index.callSites == nil {
		index.callSites = map[*types.Func][]callSite{}
		for _, body := range index.funcs {
			info := body.pkg.typed.info
			ast.Inspect(body.body, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					if ident := calleeIdent(call.Fun); ident != nil {
						if callee, ok := info.Uses[ident].(*types.Func); ok {
							index.callSites[callee.Origin()] = append(index.callSites[callee.Origin()], callSite{call: call, owner: body})
						}
					}
				}
				return true
			})
		}
	}
	var values []fieldValue
	for _, site := range index.callSites[fn] {
		if position < len(site.call.Args) {
			if value, ok := index.funcValue(site.owner, site.call.Args[position]); ok {
				values = append(values, value)
			}
		}
	}
	return values
}

// returnsFunc reports whether fn returns a function: a wrapper whose
// result is the value a field or parameter receives.
func returnsFunc(fn *types.Func) bool {
	results := fn.Type().(*types.Signature).Results()
	for result := range results.Variables() {
		if _, ok := result.Type().Underlying().(*types.Signature); ok {
			return true
		}
	}
	return false
}

// funcValue reads a function value: a named function, a method value, a
// closure, or the result of a module wrapper that returns a function.
func (index *typedIndex) funcValue(body *funcBody, expr ast.Expr) (fieldValue, bool) {
	if closure, ok := ast.Unparen(expr).(*ast.FuncLit); ok {
		return fieldValue{closure: closure, owner: body}, true
	}
	if call, ok := ast.Unparen(expr).(*ast.CallExpr); ok {
		if ident := calleeIdent(call.Fun); ident != nil {
			if fn, ok := body.pkg.typed.info.Uses[ident].(*types.Func); ok && returnsFunc(fn) && index.byObject[fn.Origin()] != nil {
				return fieldValue{fn: fn.Origin(), owner: body}, true
			}
		}
		return fieldValue{}, false
	}
	if ident := calleeIdent(expr); ident != nil {
		if fn, ok := body.pkg.typed.info.Uses[ident].(*types.Func); ok {
			return fieldValue{fn: fn.Origin(), owner: body}, true
		}
	}
	return fieldValue{}, false
}

// collectFieldValues records every function value a body stores in a
// func-typed field, through a keyed composite literal or an assignment.
func (index *typedIndex) collectFieldValues(body *funcBody) {
	info := body.pkg.typed.info
	store := func(key ast.Expr, value ast.Expr) {
		var keyIdent *ast.Ident
		switch typed := ast.Unparen(key).(type) {
		case *ast.Ident:
			keyIdent = typed
		case *ast.SelectorExpr:
			keyIdent = typed.Sel
		default:
			return
		}
		field, ok := info.Uses[keyIdent].(*types.Var)
		if !ok || !field.IsField() {
			return
		}
		if _, isFunc := field.Type().Underlying().(*types.Signature); !isFunc {
			return
		}
		if stored, ok := index.funcValue(body, value); ok {
			index.fieldValues[field.Origin()] = append(index.fieldValues[field.Origin()], stored)
			return
		}
		// A parameter stored in the field: a constructor or setter argument.
		ident, ok := ast.Unparen(value).(*ast.Ident)
		if !ok || body.obj == nil {
			return
		}
		param, ok := info.Uses[ident].(*types.Var)
		if !ok {
			return
		}
		params := body.obj.Type().(*types.Signature).Params()
		for position := range params.Len() {
			if params.At(position) == param {
				index.fieldParams[field.Origin()] = append(index.fieldParams[field.Origin()], fieldParam{fn: body.obj.Origin(), index: position})
			}
		}
	}
	ast.Inspect(body.body, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.KeyValueExpr:
			store(typed.Key, typed.Value)
		case *ast.AssignStmt:
			if len(typed.Lhs) == len(typed.Rhs) {
				for position := range typed.Lhs {
					store(typed.Lhs[position], typed.Rhs[position])
				}
			}
		}
		return true
	})
}

func (index *typedIndex) arrowReaches(used, target types.Object) bool {
	if fn, ok := used.(*types.Func); ok {
		used = fn.Origin()
	}
	if typeName, ok := used.(*types.TypeName); ok {
		if named, ok := types.Unalias(typeName.Type()).(*types.Named); ok {
			used = named.Origin().Obj()
		}
	}
	if used == target {
		return true
	}
	if pkgName, ok := target.(*types.PkgName); ok {
		if usedName, ok := used.(*types.PkgName); ok {
			return usedName.Imported() == pkgName.Imported()
		}
		return used.Pkg() == pkgName.Imported() && used.Parent() == pkgName.Imported().Scope()
	}
	usedFunc, isFunc := used.(*types.Func)
	if !isFunc {
		return false
	}
	if typeName, ok := target.(*types.TypeName); ok && constructs(usedFunc, typeName) {
		return true
	}
	switch typed := target.(type) {
	case *types.TypeName:
		if ownedBy(usedFunc, typed) {
			return true
		}
		if isInterfaceMethod(usedFunc) {
			return implementsMethod(typed.Type(), usedFunc)
		}
		if iface, ok := typed.Type().Underlying().(*types.Interface); ok {
			if recv := usedFunc.Type().(*types.Signature).Recv(); recv != nil && hasMethod(iface, usedFunc.Name()) {
				return types.Implements(recv.Type(), iface)
			}
		}
	case *types.Func:
		if usedFunc.Name() != typed.Name() {
			return false
		}
		targetRecv := typed.Type().(*types.Signature).Recv()
		usedRecv := usedFunc.Type().(*types.Signature).Recv()
		if targetRecv == nil || usedRecv == nil {
			return false
		}
		if isInterfaceMethod(usedFunc) && !isInterfaceMethod(typed) {
			return implementsMethod(targetRecv.Type(), usedFunc)
		}
		if isInterfaceMethod(typed) && !isInterfaceMethod(usedFunc) {
			return implementsMethod(usedRecv.Type(), typed)
		}
		// Two interfaces: a narrow consumer interface over the drawn one.
		// The call reaches the target when a module type satisfies both.
		if isInterfaceMethod(typed) && isInterfaceMethod(usedFunc) {
			for _, implementation := range index.implementations(typed) {
				if recv := implementation.Type().(*types.Signature).Recv(); recv != nil && implementsMethod(recv.Type(), usedFunc) {
					return true
				}
			}
		}
	}
	return false
}

// constructs reports whether fn is a function (not a method) that returns
// the named type or a pointer to it: a constructor such as NewScheduler.
func constructs(fn *types.Func, typeName *types.TypeName) bool {
	signature := fn.Type().(*types.Signature)
	if signature.Recv() != nil {
		return false
	}
	for result := range signature.Results().Variables() {
		t := result.Type()
		if pointer, ok := t.(*types.Pointer); ok {
			t = pointer.Elem()
		}
		if named, ok := types.Unalias(t).(*types.Named); ok && named.Origin().Obj() == typeName {
			return true
		}
	}
	return false
}

// implementsMethod reports whether t, or a pointer to t, satisfies the
// interface that declares method; an interface t satisfies it when its
// method set covers it.
func implementsMethod(t types.Type, method *types.Func) bool {
	recv := method.Type().(*types.Signature).Recv()
	if recv == nil {
		return false
	}
	iface, ok := recv.Type().Underlying().(*types.Interface)
	if !ok {
		return false
	}
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	if types.IsInterface(t) {
		// A wider interface satisfies a narrow consumer interface.
		return types.Implements(t, iface)
	}
	return types.Implements(t, iface) || types.Implements(types.NewPointer(t), iface)
}

func hasMethod(iface *types.Interface, name string) bool {
	for method := range iface.Methods() {
		if method.Name() == name {
			return true
		}
	}
	return false
}

func objectName(obj types.Object) string {
	if pkgName, ok := obj.(*types.PkgName); ok {
		return "package " + pkgName.Imported().Path()
	}
	if fn, ok := obj.(*types.Func); ok {
		return funcKey(fn)
	}
	if obj.Pkg() != nil {
		return obj.Pkg().Path() + "." + obj.Name()
	}
	return obj.Name()
}

// suggestChain finds the shortest chain of function references from the
// bodies of from to a body that references target, up to six calls deep,
// and phrases it as the via list that would draw it.
func (a *Atlas) suggestChain(index *typedIndex, from, target types.Object) string {
	type visit struct {
		body *funcBody
		prev *visit
	}
	queue := []*visit{}
	seen := map[*funcBody]bool{}
	for _, body := range index.bodiesOf(from) {
		seen[body] = true
		queue = append(queue, &visit{body: body})
	}
	for depth := 0; depth < 6 && len(queue) > 0; depth++ {
		var next []*visit
		for _, current := range queue {
			if index.refersTo([]*funcBody{current.body}, target) {
				var hops []string
				for step := current; step.prev != nil; step = step.prev {
					hops = append([]string{a.anchorFor(step.body.obj)}, hops...)
				}
				if len(hops) == 0 {
					return ""
				}
				return "; the code reaches it through via: [" + strings.Join(hops, ", ") + "]"
			}
			for _, callee := range index.calledBodies(current.body) {
				if !seen[callee] {
					seen[callee] = true
					next = append(next, &visit{body: callee, prev: current})
				}
			}
		}
		queue = next
	}
	return "; no call chain within six calls reaches it (a func value or a goroutine boundary needs a wiring-site anchor; otherwise redraw the view)"
}

// calledBodies returns the module bodies a body references, resolving an
// interface method to its implementations.
func (index *typedIndex) calledBodies(body *funcBody) []*funcBody {
	var out []*funcBody
	for _, ref := range index.references(body, body.body) {
		if isInterfaceMethod(ref.callee) {
			for _, implementation := range index.implementations(ref.callee) {
				if callee := index.byObject[implementation]; callee != nil {
					out = append(out, callee)
				}
			}
			continue
		}
		if callee := index.byObject[ref.callee]; callee != nil {
			out = append(out, callee)
		}
	}
	info := body.pkg.typed.info
	ast.Inspect(body.body, func(node ast.Node) bool {
		if ident, ok := node.(*ast.Ident); ok {
			if field, ok := info.Uses[ident].(*types.Var); ok && field.IsField() {
				for _, value := range index.valuesOf(field) {
					if value.fn != nil {
						if callee := index.byObject[value.fn]; callee != nil {
							out = append(out, callee)
						}
					} else if value.owner != nil {
						out = append(out, value.owner)
					}
				}
			}
		}
		return true
	})
	return out
}

// anchorFor writes the go: anchor of a module function or method, with the
// short package name when it is unique.
func (a *Atlas) anchorFor(fn *types.Func) string {
	pkg := fn.Pkg().Path()
	if candidates := a.goByName[fn.Pkg().Name()]; len(candidates) == 1 {
		pkg = fn.Pkg().Name()
	}
	if named := receiverNamed(fn); named != nil {
		return "go:" + pkg + "." + named.Obj().Name() + "." + fn.Name()
	}
	return "go:" + pkg + "." + fn.Name()
}
