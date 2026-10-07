package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var viewKinds = []string{"architecture", "sequence", "dataflow", "lifecycle"}

var sides = []struct{ ID, Title string }{
	{"server", "Server"},
	{"desktop", "Desktop"},
	{"web", "Web"},
}

// deriveViews builds the architecture views that are pure functions of the
// import graph and atlas.yaml: one group overview per side and one package
// view per group. Authors never write these.
func (a *Atlas) deriveViews(edges []groupEdge) {
	var derived []*View
	layers := groupLayers(a.Config.Groups)
	for _, side := range sides {
		view := &View{
			ID:        side.ID + "-groups",
			Kind:      "architecture",
			Category:  "architecture",
			Title:     side.Title + " groups",
			Direction: "TB",
			Derived:   true,
			Source:    "derived: docs/atlas/atlas.yaml + " + side.ID + " imports",
		}
		view.Summary = fmt.Sprintf("Every %s group and the dependencies between them, derived from source. Arrows point from importer to imported and a label counts package-level imports. Edges implied by a longer path are hidden; red edges are undeclared in atlas.yaml and orange dashed edges are reviewed exceptions.", side.Title)
		inView := map[string]bool{}
		groups := a.groupsOnSide(side.ID)
		sort.SliceStable(groups, func(i, j int) bool { return layers[groups[i].ID] < layers[groups[j].ID] })
		for _, group := range groups {
			inView[group.ID] = true
			view.Nodes = append(view.Nodes, Node{
				ID:     group.ID,
				Label:  group.Title,
				Anchor: "group:" + group.ID,
				Note:   group.Summary,
			})
		}
		var sideEdges []groupEdge
		for _, edge := range edges {
			if inView[edge.From] {
				sideEdges = append(sideEdges, edge)
			}
		}
		implied := impliedEdges(sideEdges)
		var all strings.Builder
		for _, edge := range sideEdges {
			if !inView[edge.To] {
				inView[edge.To] = true
				other := a.groupByID[edge.To]
				view.Nodes = append(view.Nodes, Node{ID: other.ID, Label: other.Title + " (" + other.Side + ")", Anchor: "group:" + other.ID, Shape: "external"})
			}
			status := "declared"
			style := ""
			switch {
			case !edge.Declared && edge.Excepted == edge.Count:
				status, style = "reviewed exception", "exception"
			case !edge.Declared:
				status, style = "UNDECLARED", "violation"
			}
			fmt.Fprintf(&all, "- `%s` → `%s`: %d package import(s), %s\n", edge.From, edge.To, edge.Count, status)
			if implied[edge.From+" -> "+edge.To] && style == "" {
				continue
			}
			view.Edges = append(view.Edges, Edge{From: edge.From, To: edge.To, Label: fmt.Sprint(edge.Count), Style: style})
		}
		view.Notes = append(view.Notes, Note{
			Title: "Every group dependency",
			Body:  "The diagram hides an edge when a longer path between the same groups already implies it (transitive reduction). This is the complete derived list.\n\n" + all.String(),
		})
		derived = append(derived, view)

		for _, group := range groups {
			derived = append(derived, a.deriveGroupView(group))
		}
	}
	for _, view := range derived {
		for _, node := range view.Nodes {
			_, _ = a.resolveAnchor(node.Anchor)
		}
	}
	// Authored views lead each category; derived group maps follow them.
	a.Views = append(a.Views, derived...)
}

// impliedEdges returns the edges a transitive reduction removes: u -> v is
// implied when v is reachable from u through some other neighbour.
func impliedEdges(edges []groupEdge) map[string]bool {
	// Only declared edges form the acyclic architecture; exceptions and
	// violations may close cycles, so they never imply anything.
	next := map[string][]string{}
	for _, edge := range edges {
		if edge.Declared {
			next[edge.From] = append(next[edge.From], edge.To)
		}
	}
	reachable := func(from, to, skip string) bool {
		seen := map[string]bool{}
		stack := []string{}
		for _, neighbour := range next[from] {
			if neighbour != skip {
				stack = append(stack, neighbour)
			}
		}
		for len(stack) > 0 {
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if current == to {
				return true
			}
			if seen[current] {
				continue
			}
			seen[current] = true
			stack = append(stack, next[current]...)
		}
		return false
	}
	implied := map[string]bool{}
	for _, edge := range edges {
		if edge.Declared && reachable(edge.From, edge.To, edge.To) {
			implied[edge.From+" -> "+edge.To] = true
		}
	}
	return implied
}

func (a *Atlas) groupsOnSide(side string) []*Group {
	var groups []*Group
	for index := range a.Config.Groups {
		if a.Config.Groups[index].Side == side {
			groups = append(groups, &a.Config.Groups[index])
		}
	}
	return groups
}

func (a *Atlas) deriveGroupView(group *Group) *View {
	view := &View{
		ID:        "group-" + group.ID,
		Kind:      "architecture",
		Category:  "architecture",
		Title:     group.Title,
		Summary:   group.Summary,
		Direction: "LR",
		Derived:   true,
		Source:    "derived: //atlas:group " + group.ID + " + imports",
	}
	members := map[string]bool{}
	for _, module := range a.sortedModules() {
		if module.Group == group.ID {
			members[module.ID] = true
		}
	}
	usedBy := map[string]bool{}
	uses := map[string]bool{}
	var internal []groupEdge
	for _, module := range a.sortedModules() {
		for _, target := range module.Imports {
			other := a.Modules[target]
			if other == nil {
				continue
			}
			switch {
			case members[module.ID] && members[target]:
				internal = append(internal, groupEdge{From: module.ID, To: target, Declared: true})
			case members[module.ID]:
				uses[other.Group] = true
			case members[target]:
				usedBy[module.Group] = true
			}
		}
	}
	for _, id := range sortedKeys(usedBy) {
		view.Nodes = append(view.Nodes, Node{ID: "in:" + id, Label: a.groupByID[id].Title, Anchor: "group:" + id, Group: "Used by", Shape: "external"})
	}
	for _, module := range a.sortedModules() {
		if members[module.ID] {
			view.Nodes = append(view.Nodes, Node{ID: module.ID, Label: moduleLabel(module), Anchor: "mod:" + module.ID, Group: group.Title, Note: module.Synopsis})
		}
	}
	for _, id := range sortedKeys(uses) {
		view.Nodes = append(view.Nodes, Node{ID: "out:" + id, Label: a.groupByID[id].Title, Anchor: "group:" + id, Group: "Depends on", Shape: "external"})
	}
	// External dependencies attach to the group as a whole; the package-level
	// detail is on each module page.
	for _, id := range sortedKeys(usedBy) {
		view.Edges = append(view.Edges, Edge{From: "in:" + id, To: "subgraph:" + group.Title, Style: "dashed"})
	}
	implied := impliedEdges(internal)
	for _, edge := range internal {
		if !implied[edge.From+" -> "+edge.To] {
			view.Edges = append(view.Edges, Edge{From: edge.From, To: edge.To})
		}
	}
	for _, id := range sortedKeys(uses) {
		view.Edges = append(view.Edges, Edge{From: "subgraph:" + group.Title, To: "out:" + id, Style: "dashed"})
	}
	return view
}

func moduleLabel(module *Module) string {
	parts := strings.Split(module.ID, "/")
	switch {
	case module.Side == "web":
		return strings.Join(parts[2:], "/")
	case len(parts) > 2 && parts[1] == "internal":
		return strings.Join(parts[2:], "/")
	case len(parts) > 1:
		return strings.Join(parts[1:], "/")
	}
	return module.ID
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

var unsafeID = regexp.MustCompile(`[^A-Za-z0-9_]`)

func mermaidID(id string) string {
	return "n_" + unsafeID.ReplaceAllString(id, "_")
}

// edgeEnd maps an edge endpoint to a node or, for "subgraph:<name>", to the
// subgraph drawn for that node group.
func edgeEnd(id string) string {
	if name, ok := strings.CutPrefix(id, "subgraph:"); ok {
		return mermaidID("g_" + name)
	}
	return mermaidID(id)
}

func mermaidText(text string) string {
	text = strings.TrimSpace(text)
	replacer := strings.NewReplacer(`"`, "#quot;", "\n", "<br/>", "#", "#35;", ";", "#59;")
	return replacer.Replace(text)
}

// mermaid renders a view. Node and edge ids are deterministic (mermaidID) so
// the site can map rendered SVG elements back to view elements.
func (a *Atlas) mermaid(view *View) string {
	switch view.Kind {
	case "sequence":
		return a.sequenceMermaid(view)
	case "lifecycle":
		return a.lifecycleMermaid(view)
	}
	return a.flowMermaid(view)
}

func (a *Atlas) flowMermaid(view *View) string {
	var builder strings.Builder
	direction := view.Direction
	if direction == "" {
		direction = "LR"
	}
	fmt.Fprintf(&builder, "flowchart %s\n", direction)
	// Emit nodes in declaration order; a group's subgraph is written where its
	// first member appears. ELK breaks cycles by this order, so the order an
	// author lists nodes in is the order the diagram reads in.
	byGroup := map[string][]Node{}
	for _, node := range view.Nodes {
		byGroup[node.Group] = append(byGroup[node.Group], node)
	}
	written := map[string]bool{}
	for _, node := range view.Nodes {
		if node.Group == "" {
			fmt.Fprintf(&builder, "    %s\n", flowNode(node))
			continue
		}
		if written[node.Group] {
			continue
		}
		written[node.Group] = true
		fmt.Fprintf(&builder, "    subgraph %s[\"%s\"]\n", mermaidID("g_"+node.Group), mermaidText(node.Group))
		for _, member := range byGroup[node.Group] {
			fmt.Fprintf(&builder, "        %s\n", flowNode(member))
		}
		builder.WriteString("    end\n")
	}
	violations := []int{}
	exceptions := []int{}
	for index, edge := range view.Edges {
		arrow := "-->"
		switch edge.Style {
		case "dashed", "async", "exception":
			arrow = "-.->"
		case "thick":
			arrow = "==>"
		}
		if edge.Style == "violation" {
			violations = append(violations, index)
		}
		if edge.Style == "exception" {
			exceptions = append(exceptions, index)
		}
		if edge.Label != "" {
			fmt.Fprintf(&builder, "    %s %s|\"%s\"| %s\n", edgeEnd(edge.From), arrow, mermaidText(edge.Label), edgeEnd(edge.To))
		} else {
			fmt.Fprintf(&builder, "    %s %s %s\n", edgeEnd(edge.From), arrow, edgeEnd(edge.To))
		}
	}
	for _, index := range violations {
		fmt.Fprintf(&builder, "    linkStyle %d stroke:#d1242f,stroke-width:2px\n", index)
	}
	for _, index := range exceptions {
		fmt.Fprintf(&builder, "    linkStyle %d stroke:#d9480f,stroke-width:1.5px\n", index)
	}
	builder.WriteString("    classDef external stroke-dasharray:4 3\n")
	var external []string
	for _, node := range view.Nodes {
		if node.Shape == "external" || strings.HasPrefix(node.Anchor, "ext:") {
			external = append(external, mermaidID(node.ID))
		}
	}
	if len(external) > 0 {
		fmt.Fprintf(&builder, "    class %s external\n", strings.Join(external, ","))
	}
	return builder.String()
}

func flowNode(node Node) string {
	id := mermaidID(node.ID)
	label := mermaidText(node.Label)
	switch node.Shape {
	case "store":
		return fmt.Sprintf("%s[(\"%s\")]", id, label)
	case "actor":
		return fmt.Sprintf("%s([\"%s\"])", id, label)
	case "queue":
		return fmt.Sprintf("%s[[\"%s\"]]", id, label)
	case "decision":
		return fmt.Sprintf("%s{\"%s\"}", id, label)
	case "external":
		return fmt.Sprintf("%s[/\"%s\"/]", id, label)
	}
	return fmt.Sprintf("%s[\"%s\"]", id, label)
}

// sequenceMermaid numbers messages with autonumber; sequenceRows numbers them
// identically so the anchor table lines up with the diagram.
func (a *Atlas) sequenceMermaid(view *View) string {
	var builder strings.Builder
	builder.WriteString("sequenceDiagram\n    autonumber\n")
	for _, node := range view.Nodes {
		keyword := "participant"
		if node.Shape == "actor" {
			keyword = "actor"
		}
		fmt.Fprintf(&builder, "    %s %s as %s\n", keyword, mermaidID(node.ID), mermaidText(node.Label))
	}
	writeSteps(&builder, view.Steps, "    ")
	return builder.String()
}

func writeSteps(builder *strings.Builder, steps []Step, indent string) {
	for _, step := range steps {
		switch {
		case step.Block != "":
			fmt.Fprintf(builder, "%s%s %s\n", indent, step.Block, mermaidText(step.Label))
			writeSteps(builder, step.Steps, indent+"    ")
			for _, branch := range step.Else {
				keyword := "else"
				if step.Block == "par" {
					keyword = "and"
				}
				fmt.Fprintf(builder, "%s%s %s\n", indent, keyword, mermaidText(branch.Label))
				writeSteps(builder, branch.Steps, indent+"    ")
			}
			fmt.Fprintf(builder, "%send\n", indent)
		case step.Note != "":
			ids := strings.Split(step.Over, ",")
			for index := range ids {
				ids[index] = mermaidID(strings.TrimSpace(ids[index]))
			}
			fmt.Fprintf(builder, "%sNote over %s: %s\n", indent, strings.Join(ids, ","), mermaidText(step.Note))
		default:
			arrow := "->>"
			if step.Reply {
				arrow = "-->>"
			}
			if step.Async {
				arrow = "-)"
			}
			fmt.Fprintf(builder, "%s%s%s%s: %s\n", indent, mermaidID(step.From), arrow, mermaidID(step.To), mermaidText(step.Label))
		}
	}
}

// sequenceRow is one numbered message for the anchor table.
type sequenceRow struct {
	Number int    `json:"number"`
	From   string `json:"from"`
	To     string `json:"to"`
	Label  string `json:"label"`
	Anchor string `json:"anchor,omitempty"`
	Block  string `json:"block,omitempty"`
}

func sequenceRows(view *View) []sequenceRow {
	var rows []sequenceRow
	number := 0
	var walk func([]Step, string)
	walk = func(steps []Step, block string) {
		for _, step := range steps {
			switch {
			case step.Block != "":
				label := step.Block + " " + step.Label
				walk(step.Steps, label)
				for _, branch := range step.Else {
					walk(branch.Steps, "else "+branch.Label)
				}
			case step.Note != "":
			default:
				number++
				rows = append(rows, sequenceRow{Number: number, From: step.From, To: step.To, Label: step.Label, Anchor: step.Anchor, Block: strings.TrimSpace(block)})
			}
		}
	}
	walk(view.Steps, "")
	return rows
}

func (a *Atlas) lifecycleMermaid(view *View) string {
	var builder strings.Builder
	builder.WriteString("stateDiagram-v2\n")
	if view.Direction != "" {
		fmt.Fprintf(&builder, "    direction %s\n", view.Direction)
	}
	for _, node := range view.Nodes {
		fmt.Fprintf(&builder, "    state \"%s\" as %s\n", mermaidText(node.Label), mermaidID(node.ID))
	}
	for _, transition := range view.Transitions {
		from, to := mermaidID(transition.From), mermaidID(transition.To)
		if transition.From == "[*]" {
			from = "[*]"
		}
		if transition.To == "[*]" {
			to = "[*]"
		}
		if transition.Label != "" {
			fmt.Fprintf(&builder, "    %s --> %s: %s\n", from, to, mermaidText(transition.Label))
		} else {
			fmt.Fprintf(&builder, "    %s --> %s\n", from, to)
		}
	}
	return builder.String()
}
