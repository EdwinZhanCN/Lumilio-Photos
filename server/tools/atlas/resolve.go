package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Anchor grammar (see docs/atlas/README.md):
//
//	go:<pkg>.<Name>[.<Member>]   pkg is a full import path or a unique package name
//	ts:[<module suffix>#]<Name>  an exported Web declaration
//	api:<METHOD> <path>          an OpenAPI operation
//	sql:<table>                  a catalog table
//	mod:<module id>              a whole Go package or Web module
//	group:<group id>             an atlas.yaml group
//	file:<repo path>             a checked-in file
//	ext:<name>                   something outside this repository
func (a *Atlas) resolveAnchor(anchor string) (*ResolvedAnchor, error) {
	if cached := a.Anchors[anchor]; cached != nil {
		return cached, nil
	}
	kind, ref, ok := strings.Cut(anchor, ":")
	if !ok || strings.TrimSpace(ref) == "" {
		return nil, fmt.Errorf("anchor %q must look like kind:reference", anchor)
	}
	resolved := &ResolvedAnchor{Anchor: anchor, Kind: kind, Label: ref}
	switch kind {
	case "go":
		symbol, err := a.resolveGoSymbol(ref)
		if err != nil {
			return nil, err
		}
		a.fillFromSymbol(resolved, symbol)
	case "ts":
		symbol, err := a.resolveTSSymbol(ref)
		if err != nil {
			return nil, err
		}
		a.fillFromSymbol(resolved, symbol)
	case "api":
		method, route, _ := strings.Cut(ref, " ")
		key := strings.ToUpper(method) + " " + strings.TrimSpace(route)
		if !a.apiRoutes[key] {
			return nil, fmt.Errorf("anchor %q: OpenAPI has no operation %s", anchor, key)
		}
		resolved.Label = key
		resolved.File = "server/docs/swagger.yaml"
	case "sql":
		if !a.sqlTables[strings.ToLower(ref)] {
			return nil, fmt.Errorf("anchor %q: no CREATE TABLE %s in the migrations", anchor, ref)
		}
		resolved.File = "server/migrations/000001_storage_baseline.up.sql"
	case "mod":
		module := a.Modules[ref]
		if module == nil {
			if pkg := a.goPaths[ref]; pkg != nil {
				module = a.Modules[pkg.dir]
			}
		}
		if module == nil {
			return nil, fmt.Errorf("anchor %q: no module %s", anchor, ref)
		}
		resolved.Module = module.ID
		resolved.Label = module.ID
		resolved.File = module.DocFile
	case "group":
		if a.groupByID[ref] == nil {
			return nil, fmt.Errorf("anchor %q: no group %s in atlas.yaml", anchor, ref)
		}
	case "file":
		if _, err := os.Stat(filepath.Join(a.Root, ref)); err != nil {
			return nil, fmt.Errorf("anchor %q: file does not exist", anchor)
		}
		resolved.File = ref
	case "ext":
	default:
		return nil, fmt.Errorf("anchor %q: unknown kind %q", anchor, kind)
	}
	a.Anchors[anchor] = resolved
	return resolved, nil
}

func (a *Atlas) fillFromSymbol(resolved *ResolvedAnchor, symbol *Symbol) {
	resolved.Module = symbol.Module
	resolved.File = symbol.File
	resolved.Line = symbol.Line
	resolved.Hash = symbol.Hash
	resolved.Label = symbol.Name
	resolved.symbol = symbol
}

func (a *Atlas) resolveGoSymbol(ref string) (*Symbol, error) {
	prefix := ""
	last := ref
	if slash := strings.LastIndex(ref, "/"); slash >= 0 {
		prefix, last = ref[:slash+1], ref[slash+1:]
	}
	pkgName, name, ok := strings.Cut(last, ".")
	if !ok {
		return nil, fmt.Errorf("anchor go:%s must name pkg.Symbol", ref)
	}
	var candidates []string
	if prefix != "" {
		candidates = []string{prefix + pkgName}
	} else {
		candidates = a.goByName[pkgName]
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("anchor go:%s: no Go package %s", ref, prefix+pkgName)
	}
	if len(candidates) > 1 {
		return nil, fmt.Errorf("anchor go:%s: package name %s is ambiguous (%s); use the full import path", ref, pkgName, strings.Join(candidates, ", "))
	}
	symbol := a.Symbols["go:"+candidates[0]+"."+name]
	if symbol == nil {
		return nil, fmt.Errorf("anchor go:%s: %s has no declaration %s", ref, candidates[0], name)
	}
	return symbol, nil
}

func (a *Atlas) resolveTSSymbol(ref string) (*Symbol, error) {
	scope, name, scoped := strings.Cut(ref, "#")
	if !scoped {
		name, scope = scope, ""
	}
	var matches []*Symbol
	for _, symbol := range a.tsByName[name] {
		if scope == "" || symbol.Module == scope || strings.HasSuffix(symbol.Module, "/"+scope) {
			matches = append(matches, symbol)
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("anchor ts:%s: no exported Web declaration %s", ref, name)
	case 1:
		return matches[0], nil
	}
	modules := make([]string, 0, len(matches))
	for _, match := range matches {
		modules = append(modules, match.Module)
	}
	sort.Strings(modules)
	return nil, fmt.Errorf("anchor ts:%s is ambiguous (%s); scope it as ts:<module>#%s", ref, strings.Join(modules, ", "), name)
}

// checkViews resolves every anchor in every authored view, validates the view
// structure, enforces lifecycle completeness, and records staleness.
func (a *Atlas) checkViews() {
	seen := map[string]string{}
	for _, view := range a.Views {
		if view.Derived {
			continue
		}
		where := view.Source
		if previous, ok := seen[view.ID]; ok {
			a.problem(where, "view id %q is already used by %s", view.ID, previous)
		}
		seen[view.ID] = where
		if view.Layout != "" && view.Layout != "elk" && view.Layout != "dagre" {
			a.problem(where, "layout %q must be elk or dagre", view.Layout)
		}
		if strings.TrimSpace(view.Title) == "" || strings.TrimSpace(view.Summary) == "" {
			a.problem(where, "view needs a title and a summary")
		}
		nodes := map[string]*Node{}
		for index := range view.Nodes {
			node := &view.Nodes[index]
			if node.ID == "" || node.Label == "" {
				a.problem(where, "every node needs an id and a label")
				continue
			}
			if nodes[node.ID] != nil {
				a.problem(where, "node id %q is declared twice", node.ID)
			}
			nodes[node.ID] = node
			if node.Anchor == "" {
				a.problem(where, "node %q has no anchor; anchor it to code, a contract, or ext:<name>", node.ID)
			}
			a.useAnchor(view, node.Anchor)
		}
		known := func(id string) bool { return nodes[id] != nil || id == "[*]" }
		for _, edge := range view.Edges {
			if !known(edge.From) || !known(edge.To) {
				a.problem(where, "edge %s -> %s references an undeclared node", edge.From, edge.To)
			}
			a.useAnchor(view, edge.Anchor)
		}
		for _, transition := range view.Transitions {
			if !known(transition.From) || !known(transition.To) {
				a.problem(where, "transition %s -> %s references an undeclared state", transition.From, transition.To)
			}
			a.useAnchor(view, transition.Anchor)
		}
		a.checkSteps(view, view.Steps, nodes)
		for _, related := range view.Related {
			found := false
			for _, other := range a.Views {
				if other.ID == related {
					found = true
				}
			}
			if !found {
				a.problem(where, "related view %q does not exist", related)
			}
		}
		switch view.Kind {
		case "sequence":
			if len(view.Steps) == 0 {
				a.problem(where, "a sequence view needs steps")
			}
		case "dataflow", "architecture":
			if len(view.Edges) == 0 {
				a.problem(where, "a %s view needs edges", view.Kind)
			}
		case "lifecycle":
			if len(view.Transitions) == 0 {
				a.problem(where, "a lifecycle view needs transitions")
			}
			a.checkEnum(view)
		}
	}
}

func (a *Atlas) checkSteps(view *View, steps []Step, nodes map[string]*Node) {
	for _, step := range steps {
		switch {
		case step.Block != "":
			switch step.Block {
			case "loop", "alt", "opt", "par", "critical", "break":
			default:
				a.problem(view.Source, "unknown sequence block %q", step.Block)
			}
			a.checkSteps(view, step.Steps, nodes)
			for _, branch := range step.Else {
				a.checkSteps(view, branch.Steps, nodes)
			}
		case step.Note != "":
			for _, id := range strings.Split(step.Over, ",") {
				if nodes[strings.TrimSpace(id)] == nil {
					a.problem(view.Source, "note over %q references an undeclared participant", step.Over)
				}
			}
		default:
			if nodes[step.From] == nil || nodes[step.To] == nil {
				a.problem(view.Source, "message %s -> %s references an undeclared participant", step.From, step.To)
			}
			if step.Label == "" {
				a.problem(view.Source, "message %s -> %s needs a label", step.From, step.To)
			}
		}
		a.useAnchor(view, step.Anchor)
	}
}

func (a *Atlas) useAnchor(view *View, anchor string) {
	if anchor == "" {
		return
	}
	resolved, err := a.resolveAnchor(anchor)
	if err != nil {
		a.problem(view.Source, "%v", err)
		return
	}
	if a.viewAnchors[view.ID] == nil {
		a.viewAnchors[view.ID] = map[string]bool{}
	}
	a.viewAnchors[view.ID][anchor] = true
	a.Usage[anchor] = appendUnique(a.Usage[anchor], view.ID)
	if resolved.Module != "" {
		a.Usage["mod:"+resolved.Module] = appendUnique(a.Usage["mod:"+resolved.Module], view.ID)
	}
	if resolved.Hash == "" {
		return
	}
	locked, ok := a.Lock[view.ID][anchor]
	if !ok {
		resolved.Stale = true
		a.Stale = append(a.Stale, Problem{Where: view.Source, Message: fmt.Sprintf("%s is not verified yet", anchor)})
	} else if locked != resolved.Hash {
		resolved.Stale = true
		a.Stale = append(a.Stale, Problem{Where: view.Source, Message: fmt.Sprintf("%s changed since this view was last verified (%s:%d)", anchor, resolved.File, resolved.Line)})
	}
}

func appendUnique(list []string, value string) []string {
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

// checkEnum requires a lifecycle view with an enum anchor to draw exactly the
// enum's members as states, so adding a state in code breaks the build until
// the diagram learns it.
func (a *Atlas) checkEnum(view *View) {
	if view.Enum == "" {
		return
	}
	resolved, err := a.resolveAnchor(view.Enum)
	if err != nil {
		a.problem(view.Source, "enum: %v", err)
		return
	}
	a.useAnchor(view, view.Enum)
	symbol := resolved.symbol
	if symbol == nil || len(symbol.Members) == 0 {
		a.problem(view.Source, "enum %s has no enumerable members (typed Go consts or a TS string-literal union)", view.Enum)
		return
	}
	drawn := map[string]bool{}
	for _, node := range view.Nodes {
		member := node.Member
		if member == "" && strings.HasPrefix(node.Anchor, "go:") {
			if anchored := a.Anchors[node.Anchor]; anchored != nil {
				member = anchored.Label
			}
		}
		if member != "" {
			drawn[member] = true
		}
	}
	for _, member := range symbol.Members {
		if !drawn[member] {
			a.problem(view.Source, "enum %s member %s is not drawn as a state", view.Enum, member)
		}
		delete(drawn, member)
	}
	for member := range drawn {
		a.problem(view.Source, "state member %s is not a member of %s", member, view.Enum)
	}
}
