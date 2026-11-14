package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Atlas is the merged, resolved model behind every output.
type Atlas struct {
	Root      string
	ConfigDir string
	Config    *Config
	Modules   map[string]*Module
	Symbols   map[string]*Symbol
	Views     []*View
	Anchors   map[string]*ResolvedAnchor
	Usage     map[string][]string
	Problems  []Problem
	Stale     []Problem
	Lock      map[string]map[string]string
	// Baselines are the reviewed violations each rule tolerates, by finding
	// key. They only shrink: see rules.go.
	Baselines map[string][]string

	viewAnchors map[string]map[string]bool

	goPackages  []*goPackage
	tsByName    map[string][]*Symbol
	goByName    map[string][]string
	goPaths     map[string]*goPackage
	apiRoutes   map[string]bool
	sqlTables   map[string]bool
	groupByID   map[string]*Group
	matched     map[string]bool
	webProblems []Problem

	// Findings holds every rule's findings by rule id, baselined or not.
	Findings map[string][]Finding
	// StaleBaselines are baseline entries whose violation is gone; `atlas
	// lock` drops them, and `atlas check` fails until it has.
	StaleBaselines []Problem
	// groupFindings are the undeclared group imports checkGroups found; the
	// group_uses rule reports them.
	groupFindings []Finding
}

type webFacts struct {
	Modules []struct {
		ID      string   `json:"id"`
		Kind    string   `json:"kind"`
		Layer   string   `json:"layer"`
		Name    string   `json:"name"`
		Files   int      `json:"files"`
		Lines   int      `json:"lines"`
		Imports []string `json:"imports"`
		Summary string   `json:"summary"`
		DocFile *string  `json:"docFile"`
		DocHTML string   `json:"docHtml"`
	} `json:"modules"`
	Symbols []struct {
		Name    string   `json:"name"`
		Module  string   `json:"module"`
		File    string   `json:"file"`
		Line    int      `json:"line"`
		Kind    string   `json:"kind"`
		Hash    string   `json:"hash"`
		Members []string `json:"members"`
	} `json:"symbols"`
	Problems []Problem `json:"problems"`
}

func (a *Atlas) problem(where, format string, args ...any) {
	a.Problems = append(a.Problems, Problem{Where: where, Message: fmt.Sprintf(format, args...)})
}

// loadAtlas reads the declaration in configDir (atlas.yaml, atlas.lock.json,
// views/) relative to root and loads every fact. An empty webFactsPath loads
// no Web modules, which fixtures use.
func loadAtlas(root, configDir, webFactsPath string) (*Atlas, error) {
	a := &Atlas{
		Root:        root,
		ConfigDir:   configDir,
		Modules:     map[string]*Module{},
		Symbols:     map[string]*Symbol{},
		Anchors:     map[string]*ResolvedAnchor{},
		Usage:       map[string][]string{},
		Lock:        map[string]map[string]string{},
		viewAnchors: map[string]map[string]bool{},
		tsByName:    map[string][]*Symbol{},
		goByName:    map[string][]string{},
		goPaths:     map[string]*goPackage{},
		apiRoutes:   map[string]bool{},
		sqlTables:   map[string]bool{},
		groupByID:   map[string]*Group{},
		matched:     map[string]bool{},
		Findings:    map[string][]Finding{},
	}
	configBytes, err := os.ReadFile(filepath.Join(root, configDir, "atlas.yaml"))
	if err != nil {
		return nil, err
	}
	a.Config = &Config{}
	if err := strictYAML(configBytes, a.Config); err != nil {
		return nil, fmt.Errorf("atlas.yaml: %w", err)
	}
	for index := range a.Config.Groups {
		group := &a.Config.Groups[index]
		if a.groupByID[group.ID] != nil {
			a.problem("atlas.yaml", "group %q is declared twice", group.ID)
		}
		a.groupByID[group.ID] = group
	}
	if err := a.loadGo(); err != nil {
		return nil, err
	}
	if webFactsPath != "" {
		if err := a.loadWeb(webFactsPath); err != nil {
			return nil, err
		}
	}
	a.loadContracts()
	a.linkModules()
	if lock, err := os.ReadFile(filepath.Join(root, configDir, "atlas.lock.json")); err == nil {
		var file lockFile
		if err := json.Unmarshal(lock, &file); err != nil {
			return nil, fmt.Errorf("atlas.lock.json: %w", err)
		}
		if file.Views != nil {
			a.Lock = file.Views
		}
		a.Baselines = file.Baselines
	}
	if err := a.loadViews(); err != nil {
		return nil, err
	}
	return a, nil
}

func strictYAML(data []byte, out any) error {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	return decoder.Decode(out)
}

func (a *Atlas) loadGo() error {
	packages, err := loadGoPackages(a.Root, a.Config)
	if err != nil {
		return err
	}
	a.goPackages = packages
	var symbols []Symbol
	for _, pkg := range packages {
		a.goPaths[pkg.importPath] = pkg
		a.goByName[pkg.name] = append(a.goByName[pkg.name], pkg.importPath)
		symbols = append(symbols, pkg.symbols()...)
	}
	for index := range symbols {
		symbol := symbols[index]
		a.Symbols[symbol.Key] = &symbol
	}
	docIndex := newGoDocIndex(packages, symbols)
	moduleOf := func(importPath string) string {
		if pkg := a.goPaths[importPath]; pkg != nil {
			return pkg.dir
		}
		return ""
	}
	for _, pkg := range packages {
		kind := "package"
		if pkg.name == "main" {
			kind = "command"
		}
		module := &Module{
			ID:         pkg.dir,
			Side:       pkg.module.Side,
			Kind:       kind,
			Name:       pkg.name,
			ImportPath: pkg.importPath,
			Group:      a.matchGroup(pkg.dir, pkg.module.Side),
			DocFile:    pkg.docFile,
			Files:      len(pkg.files),
			Lines:      pkg.lines,
		}
		for imported := range pkg.imports {
			if target := a.goPaths[imported]; target != nil {
				module.Imports = append(module.Imports, target.dir)
			}
		}
		a.Modules[module.ID] = module
		where := pkg.dir
		switch {
		case pkg.docFile == "":
			a.problem(where, "missing doc.go: every package documents itself in doc.go with a package comment")
		case strings.TrimSpace(pkg.docText) == "":
			a.problem(pkg.docFile, "doc.go has no package comment")
		}
		if pkg.docDecls {
			a.problem(pkg.docFile, "doc.go must contain only the package comment and package clause")
		}
		for _, stray := range pkg.strayDocs {
			a.problem(stray, "package comment belongs in doc.go; move it there so the package has one canonical description")
		}
		if pkg.docFile != "" {
			for key := range pkg.directives {
				a.problem(pkg.docFile, "unknown directive //atlas:%s; a package joins its group through `match` paths in docs/atlas/atlas.yaml", key)
			}
			for _, message := range docIndex.checkDocLinks(pkg) {
				a.problem(pkg.docFile, "%s", message)
			}
			module.DocHTML, module.Synopsis = docIndex.renderGoDoc(pkg, moduleOf)
		}
	}
	return a.attachTyped()
}

func (a *Atlas) loadWeb(factsPath string) error {
	data, err := os.ReadFile(factsPath)
	if err != nil {
		return fmt.Errorf("read Web facts (run `task atlas` so web/scripts/atlas-facts.ts writes them): %w", err)
	}
	var facts webFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return fmt.Errorf("parse Web facts: %w", err)
	}
	for _, item := range facts.Modules {
		module := &Module{
			ID:       item.ID,
			Side:     "web",
			Kind:     item.Kind,
			Name:     item.Name,
			Synopsis: item.Summary,
			DocHTML:  item.DocHTML,
			Files:    item.Files,
			Lines:    item.Lines,
			Imports:  item.Imports,
		}
		if item.DocFile != nil {
			module.DocFile = *item.DocFile
		}
		module.Group = a.matchGroup(module.ID, "web")
		a.Modules[module.ID] = module
	}
	for _, item := range facts.Symbols {
		symbol := &Symbol{
			Key:      "ts:" + item.Module + "#" + item.Name,
			Name:     item.Name,
			Module:   item.Module,
			File:     item.File,
			Line:     item.Line,
			Kind:     item.Kind,
			Hash:     item.Hash,
			Exported: true,
			Members:  item.Members,
		}
		if existing := a.Symbols[symbol.Key]; existing != nil {
			// Re-exports and overloads share a key; keep the first declaration.
			continue
		}
		a.Symbols[symbol.Key] = symbol
		a.tsByName[symbol.Name] = append(a.tsByName[symbol.Name], symbol)
	}
	for _, problem := range facts.Problems {
		a.Problems = append(a.Problems, problem)
	}
	return nil
}

// matchGroup returns the group whose `match` paths contain a module. Every
// module belongs to exactly one group of its own side; no match, a match on
// the other side, or matches in two groups is a problem.
func (a *Atlas) matchGroup(moduleID, side string) string {
	var found []string
	for _, group := range a.Config.Groups {
		for _, pattern := range group.Match {
			if excluded([]string{pattern}, moduleID) {
				a.matched[group.ID+"\x00"+pattern] = true
				found = appendUnique(found, group.ID)
			}
		}
	}
	switch {
	case len(found) == 0:
		a.problem(moduleID, "no group in docs/atlas/atlas.yaml matches this module; add its path to the `match` list of the group it belongs to")
		return ""
	case len(found) > 1:
		a.problem(moduleID, "groups %s all match this module; a module belongs to exactly one group", strings.Join(found, ", "))
	}
	if group := a.groupByID[found[0]]; group.Side != side {
		a.problem(moduleID, "group %s matches this module, but it belongs to side %q, not %q", group.ID, group.Side, side)
	}
	return found[0]
}

// checkMatches reports `match` patterns that no module needs, so a moved or
// deleted package cannot leave a stale path behind.
func (a *Atlas) checkMatches() {
	for _, group := range a.Config.Groups {
		for _, pattern := range group.Match {
			if !a.matched[group.ID+"\x00"+pattern] {
				a.problem("atlas.yaml", "group %s match %q matches no module; remove it", group.ID, pattern)
			}
		}
	}
}

var (
	createTablePattern = regexp.MustCompile(`(?im)^\s*CREATE\s+(?:VIRTUAL\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?"?([A-Za-z_][A-Za-z0-9_]*)"?`)
	routePattern       = regexp.MustCompile(`^    (/[^:]+):\s*$`)
	methodPattern      = regexp.MustCompile(`^        (get|post|put|patch|delete):\s*$`)
)

// loadContracts indexes the OpenAPI routes and SQLite tables that api: and
// sql: anchors resolve against.
func (a *Atlas) loadContracts() {
	if data, err := os.ReadFile(filepath.Join(a.Root, "server/docs/swagger.yaml")); err == nil {
		inPaths := false
		current := ""
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "paths:") {
				inPaths = true
				continue
			}
			if inPaths && line != "" && !strings.HasPrefix(line, " ") {
				break
			}
			if !inPaths {
				continue
			}
			if match := routePattern.FindStringSubmatch(line); match != nil {
				current = match[1]
			} else if match := methodPattern.FindStringSubmatch(line); match != nil && current != "" {
				a.apiRoutes[strings.ToUpper(match[1])+" "+current] = true
			}
		}
	}
	files, _ := filepath.Glob(filepath.Join(a.Root, "server/migrations/*.sql"))
	queueFiles, _ := filepath.Glob(filepath.Join(a.Root, "server/internal/db/**/*.sql"))
	for _, filename := range append(files, queueFiles...) {
		data, err := os.ReadFile(filename)
		if err != nil {
			continue
		}
		for _, match := range createTablePattern.FindAllStringSubmatch(string(data), -1) {
			a.sqlTables[strings.ToLower(match[1])] = true
		}
	}
}

func (a *Atlas) linkModules() {
	for _, module := range a.Modules {
		sort.Strings(module.Imports)
		for _, target := range module.Imports {
			if other := a.Modules[target]; other != nil {
				other.ImportedBy = append(other.ImportedBy, module.ID)
			}
		}
	}
	for _, module := range a.Modules {
		sort.Strings(module.ImportedBy)
		if module.Imports == nil {
			module.Imports = []string{}
		}
		if module.ImportedBy == nil {
			module.ImportedBy = []string{}
		}
	}
}

func (a *Atlas) sortedModules() []*Module {
	modules := make([]*Module, 0, len(a.Modules))
	for _, module := range a.Modules {
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].ID < modules[j].ID })
	return modules
}

func (a *Atlas) loadViews() error {
	base := filepath.Join(a.Root, a.ConfigDir, "views")
	for _, kind := range viewKinds {
		files, _ := filepath.Glob(filepath.Join(base, kind, "*.yaml"))
		sort.Strings(files)
		for _, filename := range files {
			data, err := os.ReadFile(filename)
			if err != nil {
				return err
			}
			view := &View{}
			rel := filepath.ToSlash(mustRel(a.Root, filename))
			if err := strictYAML(data, view); err != nil {
				a.problem(rel, "invalid view: %v", err)
				continue
			}
			view.Source = rel
			view.Category = kind
			if view.Kind == "" {
				view.Kind = kind
			}
			if view.Kind != kind {
				a.problem(rel, "kind %q does not match its directory %q", view.Kind, kind)
			}
			if expected := strings.TrimSuffix(path.Base(rel), ".yaml"); view.ID != expected {
				a.problem(rel, "id %q must equal the file name %q", view.ID, expected)
			}
			a.Views = append(a.Views, view)
		}
	}
	return nil
}
