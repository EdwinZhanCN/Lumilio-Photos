package main

import (
	"fmt"
	"sort"
	"strings"
)

// groupEdge aggregates package imports between two groups.
type groupEdge struct {
	From, To string
	Count    int
	Declared bool
	Excepted int
}

// checkGroups compares the declared group graph in atlas.yaml with the import
// graph derived from source. Both directions must agree: an undeclared edge is
// an architecture change that needs review, and a declared edge nobody uses is
// a stale claim.
func (a *Atlas) checkGroups() []groupEdge {
	exceptions := map[string]*Exception{}
	usedExceptions := map[string]bool{}
	for index := range a.Config.Exceptions {
		exception := &a.Config.Exceptions[index]
		key := exception.From + " -> " + exception.To
		if strings.TrimSpace(exception.Reason) == "" {
			a.problem("atlas.yaml", "exception %s needs a reason", key)
		}
		if a.Modules[exception.From] == nil || a.Modules[exception.To] == nil {
			a.problem("atlas.yaml", "exception %s names an unknown module", key)
		}
		exceptions[key] = exception
	}
	declared := map[string]bool{}
	for _, group := range a.Config.Groups {
		for _, used := range group.Uses {
			if a.groupByID[used] == nil {
				a.problem("atlas.yaml", "group %s uses unknown group %s", group.ID, used)
			}
			if used == group.ID {
				a.problem("atlas.yaml", "group %s lists itself in uses", group.ID)
			}
			declared[group.ID+" -> "+used] = true
		}
	}

	edges := map[string]*groupEdge{}
	populated := map[string]bool{}
	for _, module := range a.sortedModules() {
		populated[module.Group] = true
		for _, target := range module.Imports {
			other := a.Modules[target]
			if other == nil || other.Group == module.Group || module.Group == "" || other.Group == "" {
				continue
			}
			key := module.Group + " -> " + other.Group
			edge := edges[key]
			if edge == nil {
				edge = &groupEdge{From: module.Group, To: other.Group, Declared: declared[key]}
				edges[key] = edge
			}
			edge.Count++
			if !edge.Declared {
				packageKey := module.ID + " -> " + other.ID
				if exceptions[packageKey] != nil {
					usedExceptions[packageKey] = true
					edge.Excepted++
					continue
				}
				a.problem(module.ID, "imports %s, but group %s does not declare `uses: [%s]`; declare the dependency in docs/atlas/atlas.yaml or remove the import", other.ID, module.Group, other.Group)
			}
		}
	}
	for key := range declared {
		if edges[key] == nil {
			from, to, _ := strings.Cut(key, " -> ")
			a.problem("atlas.yaml", "group %s declares uses %s, but no package import needs it; remove the stale declaration", from, to)
		}
	}
	for key := range exceptions {
		if !usedExceptions[key] {
			a.problem("atlas.yaml", "exception %s is no longer needed; delete it", key)
		}
	}
	for _, group := range a.Config.Groups {
		if !populated[group.ID] {
			a.problem("atlas.yaml", "group %s has no modules", group.ID)
		}
		if strings.TrimSpace(group.Summary) == "" || strings.TrimSpace(group.Title) == "" {
			a.problem("atlas.yaml", "group %s needs a title and summary", group.ID)
		}
	}
	if cycle := declaredCycle(a.Config.Groups); cycle != "" {
		a.problem("atlas.yaml", "declared group dependencies form a cycle: %s", cycle)
	}

	list := make([]groupEdge, 0, len(edges))
	for _, edge := range edges {
		list = append(list, *edge)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].From != list[j].From {
			return list[i].From < list[j].From
		}
		return list[i].To < list[j].To
	})
	return list
}

func declaredCycle(groups []Group) string {
	uses := map[string][]string{}
	for _, group := range groups {
		uses[group.ID] = group.Uses
	}
	const (
		unvisited = iota
		visiting
		done
	)
	state := map[string]int{}
	var stack []string
	var found string
	var visit func(string) bool
	visit = func(id string) bool {
		state[id] = visiting
		stack = append(stack, id)
		for _, next := range uses[id] {
			if state[next] == visiting {
				start := 0
				for index, item := range stack {
					if item == next {
						start = index
					}
				}
				found = strings.Join(append(append([]string{}, stack[start:]...), next), " -> ")
				return true
			}
			if state[next] == unvisited && visit(next) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
		return false
	}
	ids := make([]string, 0, len(uses))
	for id := range uses {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if state[id] == unvisited && visit(id) {
			return found
		}
	}
	return ""
}

// groupLayers assigns each group its depth in the declared DAG (0 = depends on
// nothing), which orders overview diagrams bottom-up.
func groupLayers(groups []Group) map[string]int {
	uses := map[string][]string{}
	for _, group := range groups {
		uses[group.ID] = group.Uses
	}
	layers := map[string]int{}
	var depth func(string, map[string]bool) int
	depth = func(id string, seen map[string]bool) int {
		if layer, ok := layers[id]; ok {
			return layer
		}
		if seen[id] {
			return 0
		}
		seen[id] = true
		best := 0
		for _, next := range uses[id] {
			if layer := depth(next, seen) + 1; layer > best {
				best = layer
			}
		}
		layers[id] = best
		return best
	}
	for _, group := range groups {
		depth(group.ID, map[string]bool{})
	}
	return layers
}

func formatProblems(problems []Problem) string {
	sorted := append([]Problem(nil), problems...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Where < sorted[j].Where })
	var builder strings.Builder
	for _, problem := range sorted {
		fmt.Fprintf(&builder, "  %s: %s\n", problem.Where, problem.Message)
	}
	return builder.String()
}
