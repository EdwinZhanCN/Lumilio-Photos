package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testAtlas() *Atlas {
	a := &Atlas{
		Config:      &Config{},
		Modules:     map[string]*Module{},
		Symbols:     map[string]*Symbol{},
		Anchors:     map[string]*ResolvedAnchor{},
		Usage:       map[string][]string{},
		Lock:        map[string]map[string]string{},
		viewAnchors: map[string]map[string]bool{},
		tsByName:    map[string][]*Symbol{},
		goByName:    map[string][]string{"phase": {"server/internal/phase"}},
		goPaths:     map[string]*goPackage{},
		apiRoutes:   map[string]bool{"GET /api/v1/things": true},
		sqlTables:   map[string]bool{"things": true},
		groupByID:   map[string]*Group{},
	}
	add := func(symbol Symbol) { a.Symbols[symbol.Key] = &symbol }
	add(Symbol{Key: "go:server/internal/phase.Phase", Name: "Phase", Module: "server/internal/phase", File: "phase.go", Line: 3, Kind: "type", Hash: "t", Members: []string{"PhaseDone", "PhaseNew"}})
	add(Symbol{Key: "go:server/internal/phase.PhaseNew", Name: "PhaseNew", Module: "server/internal/phase", File: "phase.go", Line: 5, Kind: "const", Hash: "n1"})
	add(Symbol{Key: "go:server/internal/phase.PhaseDone", Name: "PhaseDone", Module: "server/internal/phase", File: "phase.go", Line: 6, Kind: "const", Hash: "d1"})
	return a
}

func lifecycleView(states ...string) *View {
	view := &View{ID: "phase", Kind: "lifecycle", Title: "Phase", Summary: "Phase.", Source: "views/lifecycle/phase.yaml", Enum: "go:phase.Phase"}
	for _, state := range states {
		view.Nodes = append(view.Nodes, Node{ID: state, Label: state, Anchor: "go:phase." + state})
	}
	view.Transitions = []Transition{{From: "[*]", To: states[0]}}
	return view
}

func messages(problems []Problem) string {
	var out []string
	for _, problem := range problems {
		out = append(out, problem.Message)
	}
	return strings.Join(out, "\n")
}

func TestLifecycleMustDrawEveryEnumMember(t *testing.T) {
	a := testAtlas()
	a.Views = []*View{lifecycleView("PhaseNew")}
	a.checkViews()
	if got := messages(a.Problems); !strings.Contains(got, "member PhaseDone is not drawn") {
		t.Fatalf("a new enum member must fail the view, got:\n%s", got)
	}

	complete := testAtlas()
	complete.Views = []*View{lifecycleView("PhaseNew", "PhaseDone")}
	complete.checkViews()
	if len(complete.Problems) != 0 {
		t.Fatalf("complete lifecycle should pass, got:\n%s", messages(complete.Problems))
	}
}

func TestChangedAnchorIsStaleUntilLocked(t *testing.T) {
	a := testAtlas()
	a.Views = []*View{lifecycleView("PhaseNew", "PhaseDone")}
	a.Lock = map[string]map[string]string{"phase": {"go:phase.Phase": "t", "go:phase.PhaseNew": "n0", "go:phase.PhaseDone": "d1"}}
	a.checkViews()
	if len(a.Stale) != 1 || !strings.Contains(a.Stale[0].Message, "go:phase.PhaseNew changed") {
		t.Fatalf("only the edited constant should be stale, got:\n%s", messages(a.Stale))
	}
	if got := string(a.lockFile()); !strings.Contains(got, `"go:phase.PhaseNew": "n1"`) {
		t.Fatalf("lock should record the current hash, got:\n%s", got)
	}
}

func TestBrokenAnchorsFail(t *testing.T) {
	a := testAtlas()
	for _, anchor := range []string{"go:phase.Gone", "go:missing.Thing", "api:POST /api/v1/things", "sql:nothing", "kind:x", "nonsense"} {
		if _, err := a.resolveAnchor(anchor); err == nil {
			t.Errorf("anchor %q should not resolve", anchor)
		}
	}
	for _, anchor := range []string{"go:phase.PhaseNew", "go:server/internal/phase.Phase", "api:get /api/v1/things", "sql:things", "ext:Someone"} {
		if _, err := a.resolveAnchor(anchor); err != nil {
			t.Errorf("anchor %q should resolve: %v", anchor, err)
		}
	}
}

func TestImpliedEdgesIgnoreUndeclaredCycles(t *testing.T) {
	edges := []groupEdge{
		{From: "http", To: "service", Declared: true},
		{From: "service", To: "catalog", Declared: true},
		{From: "http", To: "catalog", Declared: true},
		{From: "catalog", To: "http", Declared: false},
	}
	implied := impliedEdges(edges)
	if !implied["http -> catalog"] {
		t.Fatal("http -> catalog is implied by http -> service -> catalog")
	}
	if implied["http -> service"] || implied["service -> catalog"] {
		t.Fatalf("an undeclared back edge must not make declared edges look implied: %v", implied)
	}
}

func TestDeclaredCycleIsReported(t *testing.T) {
	cycle := declaredCycle([]Group{{ID: "a", Uses: []string{"b"}}, {ID: "b", Uses: []string{"a"}}})
	if cycle == "" {
		t.Fatal("a -> b -> a must be reported")
	}
	if declaredCycle([]Group{{ID: "a", Uses: []string{"b"}}, {ID: "b"}}) != "" {
		t.Fatal("an acyclic declaration must pass")
	}
}

func TestDocLinksMustResolve(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("doc.go", "// Package demo uses [Widget], [Widget.Run], [Missing], and [encoding/json].\n//\n//atlas:group foundation\npackage demo\n")
	write("widget.go", "package demo\n\ntype Widget struct{}\n\nfunc (Widget) Run() {}\n")
	pkg, err := parseGoPackage(filepath.Dir(dir), GoModule{Dir: filepath.Base(dir), Path: "demo"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.directives["group"] != "foundation" || strings.Contains(pkg.docText, "atlas:") {
		t.Fatalf("the directive must be parsed and kept out of the doc text: %q %v", pkg.docText, pkg.directives)
	}
	index := newGoDocIndex([]*goPackage{pkg}, pkg.symbols())
	problems := strings.Join(index.checkDocLinks(pkg), "\n")
	if !strings.Contains(problems, "[Missing]") {
		t.Fatalf("an unresolved doc link must be reported, got %q", problems)
	}
	if strings.Contains(problems, "Widget") || strings.Contains(problems, "encoding/json") {
		t.Fatalf("valid links must pass, got %q", problems)
	}
}

func TestMermaidTextEscapesSyntax(t *testing.T) {
	if got := mermaidText(`a "b"; #c`); got != "a #quot;b#quot;#59; #35;c" {
		t.Fatalf("unexpected escape: %q", got)
	}
	if !excluded([]string{"server/tools/**"}, "server/tools/atlas") || excluded([]string{"server/tools/**"}, "server/toolsx") {
		t.Fatal("exclude globs must match whole path segments")
	}
}

func TestDocReferencesMustResolve(t *testing.T) {
	a := testAtlas()
	a.Root = t.TempDir()
	a.Config.UndocumentedRoutes = []string{"/api/v1/health/ready"}
	a.apiRoutes["GET /api/v1/things/{id}"] = true
	if err := os.MkdirAll(filepath.Join(a.Root, "server/internal/phase"), 0o755); err != nil {
		t.Fatal(err)
	}
	text := "Uses `server/internal/phase`, `server/internal/phase.PhaseNew`, `server/gone.go`, " +
		"`server/internal/phase.Missing`, `docs/YYYY-MM-DD.md`, `task real`, `task fake`, " +
		"/api/v1/things/:id, /api/v1/things, /api/v1/health/ready, /api/v1/nope.\n" +
		"```sh\ntask ignored-in-fences\n```\n"
	a.checkMarkdownRefs("doc.md", stripFences(text), map[string]bool{"real": true})
	got := messages(a.Problems)
	for _, want := range []string{"`server/gone.go`", "`server/internal/phase.Missing`", "`task fake`", "/api/v1/nope"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected a problem for %s, got:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"phase.PhaseNew", "YYYY", "task real", "things", "health", "ignored-in-fences"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("%s should resolve, got:\n%s", unwanted, got)
		}
	}
}
