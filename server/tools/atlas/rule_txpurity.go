package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
)

// tx-purity: a transaction holds the single SQLite writer (or a read
// snapshot), so nothing slow or unbounded may run inside one. Its scope is
// defined by types, not names. The roots are every function and function
// literal with a parameter of a transaction type, and the statements after
// a Begin call up to the Commit of what it returned. The scope is the roots
// plus every mapped function statically reachable from them, with interface
// calls resolved to their mapped implementations. A function reached from a
// transaction body runs inside that transaction, whatever its name.

// txNode is one unit of the reachability walk: a function body or a root
// region inside one.
type txNode struct {
	owner *funcBody
	label string
	nodes []ast.Node
	// ended are the spans of a Begin region where the transaction has been
	// rolled back: code there no longer holds the writer.
	ended spans
}

// spans are byte ranges of one file.
type spans struct {
	file   string
	ranges [][2]int
}

func (s spans) contains(offset int, file string) bool {
	if file != s.file {
		return false
	}
	for _, r := range s.ranges {
		if offset >= r[0] && offset < r[1] {
			return true
		}
	}
	return false
}

func (a *Atlas) txPurityFindings(rule Rule) []Finding {
	config := rule.TxPurity
	if len(config.TxTypes) == 0 {
		a.ruleProblem(rule, "tx_purity needs tx_types")
		return nil
	}
	index := a.typed()
	txTypes := map[string]bool{}
	for _, key := range config.TxTypes {
		if !index.validTypeKey(key) {
			a.ruleProblem(rule, "tx type %s does not resolve", key)
		}
		txTypes[key] = true
	}
	begins := a.resolveTargets(rule, config.Begin)
	forbidCalls := a.resolveTargets(rule, config.ForbidCalls)
	allow := a.resolveTargets(rule, config.Allow)
	forbidPackages := map[string]bool{}
	for _, path := range config.ForbidPackages {
		if index.packages[path] == nil {
			a.ruleProblem(rule, "forbidden package %s is not imported by any mapped package", path)
		}
		forbidPackages[path] = true
	}
	countQueries := map[string]bool{}
	if config.ForbidCountQueries {
		for key, statements := range a.sqlFacts().queries {
			for _, statement := range statements {
				if statement.Count {
					countQueries[key] = true
				}
			}
		}
	}
	forbidden := func(callee *types.Func) string {
		key := funcKey(callee)
		if allow[key] {
			return ""
		}
		if forbidCalls[key] {
			return "a forbidden call"
		}
		if countKey, ok := index.matchesTarget(callee, countQueries); ok && !allow[countKey] {
			return "a COUNT query"
		}
		// An interface method is judged by its implementations, which the
		// walk visits: (io.Writer).Write on an in-memory hash is not I/O.
		if isInterfaceMethod(callee) {
			return ""
		}
		pkg := callee.Pkg()
		if named := receiverNamed(callee); named != nil && named.Obj().Pkg() != nil {
			pkg = named.Obj().Pkg()
		}
		if pkg != nil && forbidPackages[pkg.Path()] {
			return "package " + pkg.Path()
		}
		return ""
	}

	// Roots.
	var roots []txNode
	for _, fn := range index.funcs {
		if fn.obj != nil && signatureHasType(fn.obj.Type().(*types.Signature), txTypes) {
			roots = append(roots, txNode{owner: fn, label: fn.key, nodes: []ast.Node{fn.body}})
			continue
		}
		info := fn.pkg.typed.info
		ast.Inspect(fn.body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.FuncLit:
				if signature, ok := info.TypeOf(typed).(*types.Signature); ok && signatureHasType(signature, txTypes) {
					roots = append(roots, txNode{owner: fn, label: fn.key + " (transaction closure)", nodes: []ast.Node{typed.Body}})
					return false
				}
			case *ast.BlockStmt:
				if region, txType := beginRegion(index, fn, typed, begins); len(region) > 0 {
					roots = append(roots, txNode{owner: fn, label: fn.key + " (between Begin and Commit)", nodes: region, ended: rolledBack(fn, region, txType)})
				}
			}
			return true
		})
	}

	// Reachability.
	// parent records how each function was reached, so a finding names
	// the call chain from its transaction root.
	visited := map[*funcBody]bool{}
	parent := map[*funcBody]*funcBody{}
	var queue []txNode
	for _, root := range roots {
		if len(root.nodes) == 1 && root.nodes[0] == root.owner.body {
			visited[root.owner] = true
		}
		queue = append(queue, root)
	}
	chain := func(fn *funcBody) string {
		var path []string
		for step := fn; step != nil && len(path) < 12; step = parent[step] {
			path = append([]string{step.key}, path...)
		}
		return strings.Join(path, " → ")
	}
	var findings []Finding
	seen := map[string]bool{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, node := range current.nodes {
			for _, ref := range index.references(current.owner, node) {
				if current.ended.contains(ref.pos.Offset, ref.file) {
					continue
				}
				if reason := forbidden(ref.callee); reason != "" {
					key := current.owner.key + " calls " + funcKey(ref.callee)
					if !seen[key] {
						seen[key] = true
						findings = append(findings, Finding{Key: key, Where: where(ref.file, ref.pos), Message: fmt.Sprintf("%s calls %s (%s) inside a transaction opened by %s, via %s", current.owner.key, funcKey(ref.callee), reason, current.label, chain(current.owner))})
					}
					continue
				}
				callees := []*types.Func{ref.callee}
				if isInterfaceMethod(ref.callee) {
					callees = index.implementations(ref.callee)
				}
				for _, callee := range callees {
					body := index.byObject[callee]
					if body == nil {
						continue
					}
					if visited[body] {
						continue
					}
					visited[body] = true
					parent[body] = current.owner
					queue = append(queue, txNode{owner: body, label: current.label, nodes: []ast.Node{body.body}})
				}
			}
		}
	}
	return findings
}

func signatureHasType(signature *types.Signature, targets map[string]bool) bool {
	params := signature.Params()
	for i := 0; i < params.Len(); i++ {
		if targets[typeKey(params.At(i).Type())] {
			return true
		}
	}
	return false
}

// beginRegion returns the statements of a block that run inside a
// transaction opened by a Begin call in that block: those after the
// statement that calls Begin, up to and including the first that commits a
// value of the type Begin returns.
func beginRegion(index *typedIndex, fn *funcBody, block *ast.BlockStmt, begins map[string]bool) ([]ast.Node, types.Type) {
	if len(begins) == 0 {
		return nil, nil
	}
	info := fn.pkg.typed.info
	for k, statement := range block.List {
		var txType types.Type
		ast.Inspect(statement, func(node ast.Node) bool {
			switch node.(type) {
			case *ast.FuncLit, *ast.BlockStmt:
				// A Begin in a nested block opens its region there.
				return false
			}
			if call, ok := node.(*ast.CallExpr); ok {
				if ident := calleeIdent(call.Fun); ident != nil {
					if callee, ok := info.Uses[ident].(*types.Func); ok && begins[funcKey(callee)] {
						if results := callee.Type().(*types.Signature).Results(); results.Len() > 0 {
							txType = results.At(0).Type()
						}
					}
				}
			}
			return true
		})
		if txType == nil {
			continue
		}
		var region []ast.Node
		for _, next := range block.List[k+1:] {
			region = append(region, next)
			if commitsType(info, next, txType) {
				break
			}
		}
		return region, txType
	}
	return nil, nil
}

// rolledBack returns the spans of a region that follow a non-deferred
// Rollback of the transaction in the same block: on that path the
// transaction has ended.
func rolledBack(fn *funcBody, region []ast.Node, txType types.Type) spans {
	info := fn.pkg.typed.info
	fset := fn.pkg.typed.fset
	out := spans{file: fn.file}
	for _, node := range region {
		ast.Inspect(node, func(node ast.Node) bool {
			if _, literal := node.(*ast.FuncLit); literal {
				return false
			}
			block, ok := node.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for k, statement := range block.List {
				if k+1 < len(block.List) && callsMethod(info, statement, txType, "Rollback") {
					out.ranges = append(out.ranges, [2]int{fset.Position(block.List[k+1].Pos()).Offset, fset.Position(block.End()).Offset})
					break
				}
			}
			return true
		})
	}
	return out
}

// commitsType reports whether a statement calls Commit on a value of the
// transaction type outside a deferred call.
func commitsType(info *types.Info, statement ast.Stmt, txType types.Type) bool {
	return callsMethod(info, statement, txType, "Commit")
}

// callsMethod reports whether a statement calls a method of a value of the
// transaction type outside a deferred call or a function literal.
func callsMethod(info *types.Info, statement ast.Stmt, txType types.Type, method string) bool {
	if _, deferred := statement.(*ast.DeferStmt); deferred {
		return false
	}
	found := false
	ast.Inspect(statement, func(node ast.Node) bool {
		if _, literal := node.(*ast.FuncLit); literal {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return !found
		}
		selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if !ok {
			return true
		}
		callee, ok := info.Uses[selector.Sel].(*types.Func)
		if ok && callee.Name() == method && types.Identical(info.TypeOf(selector.X), txType) {
			found = true
		}
		return !found
	})
	return found
}

// budgetMeasure returns what a budgets rule counts for a package, and
// whether the package exists inside the rule's scope.
func (a *Atlas) budgetMeasure(rule Rule, dir string) (int, bool) {
	if a.Modules[dir] == nil {
		return 0, false
	}
	if scope := rule.Budgets; (len(scope.In) > 0 && !inScope(scope.In, dir, "")) || inScope(scope.Except, dir, "") {
		return 0, false
	}
	if rule.Budgets.Measure == "sql" {
		return a.sqlFacts().sites[dir], true
	}
	return a.Modules[dir].Lines, true
}

func budgetUnit(rule Rule) string {
	if rule.Budgets.Measure == "sql" {
		return "hand-written SQL call sites"
	}
	return "production lines"
}

// budgetsFindings reports every package over its ceiling: the recorded
// ceiling if it has one, else the default.
func (a *Atlas) budgetsFindings(rule Rule) []Finding {
	switch rule.Budgets.Measure {
	case "", "lines":
		if rule.Budgets.Default <= 0 {
			a.ruleProblem(rule, "budgets needs a positive default")
			return nil
		}
	case "sql":
		if rule.Budgets.Default < 0 {
			a.ruleProblem(rule, "budgets needs a default of zero or more")
			return nil
		}
	default:
		a.ruleProblem(rule, "budgets measure %q is neither lines nor sql", rule.Budgets.Measure)
		return nil
	}
	ceilings := a.Ceilings[rule.ID]
	advice := "split the package or move code out"
	if rule.Budgets.Measure == "sql" {
		advice = "write a sqlc query, or move the SQL to the package that owns the table"
	}
	var findings []Finding
	for _, pkg := range a.goPackages {
		measured, ok := a.budgetMeasure(rule, pkg.dir)
		if !ok {
			continue
		}
		limit := rule.Budgets.Default
		if ceiling, ok := ceilings[pkg.dir]; ok {
			limit = ceiling
		}
		if measured > limit {
			findings = append(findings, Finding{Key: pkg.dir, Where: pkg.dir, Message: fmt.Sprintf("has %d %s, over its ceiling of %d; %s", measured, budgetUnit(rule), limit, advice)})
		}
	}
	for _, dir := range sortedKeys(ceilings) {
		measured, ok := a.budgetMeasure(rule, dir)
		switch {
		case !ok:
			a.StaleBaselines = append(a.StaleBaselines, Problem{Where: "atlas.lock.json", Rule: rule.ID, Message: fmt.Sprintf("ceiling for %s, which no longer exists", dir)})
		case measured < ceilings[dir]:
			a.StaleBaselines = append(a.StaleBaselines, Problem{Where: "atlas.lock.json", Rule: rule.ID, Message: fmt.Sprintf("ceiling %d for %s is above its %d %s; ceilings only fall", ceilings[dir], dir, measured, budgetUnit(rule))})
		}
	}
	return findings
}

// lockedCeilings is what `atlas lock` writes for budgets rules: every
// ceiling lowered to the package's current measure, and dropped once the
// package is back at or under the default. It never raises one.
func (a *Atlas) lockedCeilings() map[string]map[string]int {
	return a.shrunkCeilings(false)
}

func (a *Atlas) shrunkCeilings(existingOnly bool) map[string]map[string]int {
	out := map[string]map[string]int{}
	for _, rule := range a.Config.Rules {
		if _, ok := a.Ceilings[rule.ID]; rule.Budgets == nil || (existingOnly && !ok) {
			continue
		}
		kept := map[string]int{}
		for dir, ceiling := range a.Ceilings[rule.ID] {
			measured, ok := a.budgetMeasure(rule, dir)
			if !ok || measured <= rule.Budgets.Default {
				continue
			}
			kept[dir] = min(ceiling, measured)
		}
		out[rule.ID] = kept
	}
	return out
}

// baselineCeilings records the current measure of every Go package over the
// default, for a budgets rule that has no ceilings yet.
func (a *Atlas) baselineCeilings(rule Rule) (map[string]map[string]int, error) {
	if _, ok := a.Ceilings[rule.ID]; ok {
		return nil, fmt.Errorf("rule %s already has ceilings; they only fall", rule.ID)
	}
	out := a.shrunkCeilings(true)
	kept := map[string]int{}
	for _, pkg := range a.goPackages {
		if measured, ok := a.budgetMeasure(rule, pkg.dir); ok && measured > rule.Budgets.Default {
			kept[pkg.dir] = measured
		}
	}
	out[rule.ID] = kept
	return out, nil
}
