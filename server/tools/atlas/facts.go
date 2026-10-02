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
	Root     string
	Config   *Config
	Modules  map[string]*Module
	Symbols  map[string]*Symbol
	Views    []*View
	Anchors  map[string]*ResolvedAnchor
	Usage    map[string][]string
	Problems []Problem
	Stale    []Problem
	Lock     map[string]map[string]string

	viewAnchors map[string]map[string]bool

	goPackages  []*goPackage
	tsByName    map[string][]*Symbol
	goByName    map[string][]string
	goPaths     map[string]*goPackage
	apiRoutes   map[string]bool
	sqlTables   map[string]bool
	groupByID   map[string]*Group
	webProblems []Problem
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

func loadAtlas(root, webFactsPath string) (*Atlas, error) {
	a := &Atlas{
		Root:        root,
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
	}
	configBytes, err := os.ReadFile(filepath.Join(root, atlasDir, "atlas.yaml"))
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
	if err := a.loadWeb(webFactsPath); err != nil {
		return nil, err
	}
	a.loadContracts()
	a.linkModules()
	if lock, err := os.ReadFile(filepath.Join(root, atlasDir, "atlas.lock.json")); err == nil {
		if err := json.Unmarshal(lock, &a.Lock); err != nil {
			return nil, fmt.Errorf("atlas.lock.json: %w", err)
		}
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
			Group:      pkg.directives["group"],
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
			a.problem(where, "missing doc.go: every package documents itself in doc.go with a package comment and an //atlas:group directive")
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
			group := pkg.directives["group"]
			switch {
			case group == "":
				a.problem(pkg.docFile, "missing //atlas:group directive")
			case a.groupByID[group] == nil:
				a.problem(pkg.docFile, "//atlas:group %q is not declared in docs/atlas/atlas.yaml", group)
			case a.groupByID[group].Side != pkg.module.Side:
				a.problem(pkg.docFile, "//atlas:group %q belongs to side %q, not %q", group, a.groupByID[group].Side, pkg.module.Side)
			}
			for key := range pkg.directives {
				if key != "group" {
					a.problem(pkg.docFile, "unknown directive //atlas:%s", key)
				}
			}
			for _, message := range docIndex.checkDocLinks(pkg) {
				a.problem(pkg.docFile, "%s", message)
			}
			module.DocHTML, module.Synopsis = docIndex.renderGoDoc(pkg, moduleOf)
		}
	}
	return nil
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
		module.Group = a.webGroup(module.ID)
		if module.Group == "" {
			a.problem(module.ID, "no Web group in docs/atlas/atlas.yaml matches this module")
		}
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

func (a *Atlas) webGroup(moduleID string) string {
	for _, group := range a.Config.Groups {
		if group.Side != "web" {
			continue
		}
		for _, pattern := range group.Match {
			if excluded([]string{pattern}, moduleID) {
				return group.ID
			}
		}
	}
	return ""
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
	base := filepath.Join(a.Root, atlasDir, "views")
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
