package main

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Handwritten Markdown cannot be type-checked, but what it points at can.
// checkDocRefs requires every repository path, Task target, and API route a
// checked document names to exist, so a rename or removal fails the build
// instead of leaving the prose silently wrong. Package doc comments get the
// same check for the routes and Task targets they mention.

var (
	markdownLink  = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	backticked    = regexp.MustCompile("`([^`\n]+)`")
	apiRoute      = regexp.MustCompile(`/api/v1/[A-Za-z0-9_\-/{}:.*]+`)
	taskReference = regexp.MustCompile(`^task ([a-z0-9][a-z0-9:_\-]*)`)
	repoRoots     = []string{"server/", "web/", "desktop/", "site/", "docs/", "deploy/", "wasm/", ".agents/", ".github/"}
)

func (a *Atlas) checkDocRefs() {
	if len(a.Config.Docs) == 0 {
		return
	}
	tasks := loadTaskTargets(a.Root)
	var files []string
	for _, pattern := range a.Config.Docs {
		matches, _ := filepath.Glob(filepath.Join(a.Root, pattern))
		files = append(files, matches...)
	}
	sort.Strings(files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		rel := filepath.ToSlash(mustRel(a.Root, file))
		a.checkMarkdownRefs(rel, stripFences(string(data)), tasks)
	}
	for _, pkg := range a.goPackages {
		for _, message := range a.textRefProblems(pkg.docText, tasks) {
			a.problem(pkg.docFile, "%s", message)
		}
	}
}

// stripFences drops fenced code blocks, whose contents are examples rather
// than references.
func stripFences(text string) string {
	var out strings.Builder
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func (a *Atlas) checkMarkdownRefs(rel, text string, tasks map[string]bool) {
	dir := path.Dir(rel)
	for _, match := range markdownLink.FindAllStringSubmatch(text, -1) {
		target := match[1]
		if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") || strings.HasPrefix(target, "#") {
			continue
		}
		target, _, _ = strings.Cut(target, "#")
		if target == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(a.Root, dir, target)); err != nil {
			a.problem(rel, "link %s points at a file that does not exist", match[1])
		}
	}
	for _, match := range backticked.FindAllStringSubmatch(text, -1) {
		token := strings.TrimSpace(match[1])
		if strings.HasPrefix(token, "task ") {
			if name := taskReference.FindStringSubmatch(token); name != nil && !tasks[name[1]] {
				a.problem(rel, "`%s` names a Task target that does not exist", token)
			}
			continue
		}
		if target := repoPath(token); target != "" && !a.pathOrSymbolExists(target) {
			a.problem(rel, "`%s` names a repository path or symbol that does not exist", token)
		}
	}
	for _, message := range a.textRefProblems(text, nil) {
		a.problem(rel, "%s", message)
	}
}

// textRefProblems checks API routes anywhere in text and, when tasks is not
// nil, backticked Task targets.
func (a *Atlas) textRefProblems(text string, tasks map[string]bool) []string {
	var problems []string
	seen := map[string]bool{}
	for _, route := range apiRoute.FindAllString(text, -1) {
		route = strings.TrimRight(route, ".:,")
		if seen[route] || a.routeExists(route) {
			continue
		}
		seen[route] = true
		problems = append(problems, "API route "+route+" is not in the OpenAPI document")
	}
	if tasks != nil {
		for _, match := range backticked.FindAllStringSubmatch(text, -1) {
			if name := taskReference.FindStringSubmatch(strings.TrimSpace(match[1])); name != nil && !tasks[name[1]] {
				problems = append(problems, "`"+match[1]+"` names a Task target that does not exist")
			}
		}
	}
	return problems
}

// repoPath returns the repository-relative path a token names, or "" when the
// token is not a concrete path under a known root (globs, placeholders, and
// prose are skipped).
func repoPath(token string) string {
	if strings.ContainsAny(token, " *<>{}$…") || strings.Contains(token, "...") || placeholder.MatchString(token) {
		return ""
	}
	clean := strings.TrimSuffix(token, "/")
	clean, _, _ = strings.Cut(clean, "#")
	if at := strings.LastIndex(clean, ":"); at > 0 {
		clean = clean[:at]
	}
	for _, root := range repoRoots {
		if strings.HasPrefix(clean, root) {
			return clean
		}
	}
	return ""
}

// pathOrSymbolExists accepts a file or directory, or the Go form
// dir/pkg.Symbol (for example server/app.Run) when that declaration exists.
func (a *Atlas) pathOrSymbolExists(target string) bool {
	if _, err := os.Stat(filepath.Join(a.Root, target)); err == nil {
		return true
	}
	dir, last := path.Split(target)
	pkg, symbol, ok := strings.Cut(last, ".")
	if !ok || symbol == "" {
		return false
	}
	return a.Symbols["go:"+dir+pkg+"."+symbol] != nil
}

var placeholder = regexp.MustCompile(`YYYY|NNNN|XXXX|<[a-z]`)

var routeParam = regexp.MustCompile(`\{[^}]+\}|:[A-Za-z_]+`)

// routeExists accepts an exact OpenAPI path (parameters in either {id} or :id
// form) or a prefix naming a group of routes, such as /api/v1/storage/*.
func (a *Atlas) routeExists(route string) bool {
	route = strings.TrimRight(route, "*/")
	want := routeParam.ReplaceAllString(route, "{}")
	known := append([]string{}, a.Config.UndocumentedRoutes...)
	for key := range a.apiRoutes {
		_, path, _ := strings.Cut(key, " ")
		known = append(known, path)
	}
	for _, path := range known {
		have := routeParam.ReplaceAllString(path, "{}")
		// An exact route, or a prefix naming a group of routes.
		if have == want || strings.HasPrefix(have, want+"/") {
			return true
		}
	}
	return false
}

// loadTaskTargets reads the root Taskfile and its namespaced includes.
func loadTaskTargets(root string) map[string]bool {
	targets := map[string]bool{}
	type includeSpec struct {
		Taskfile string   `yaml:"taskfile"`
		Aliases  []string `yaml:"aliases"`
	}
	type taskfile struct {
		Includes map[string]includeSpec `yaml:"includes"`
		Tasks    map[string]yaml.Node   `yaml:"tasks"`
	}
	read := func(file string) taskfile {
		var parsed taskfile
		for _, name := range []string{file, strings.Replace(file, "taskfile.yml", "Taskfile.yml", 1)} {
			if data, err := os.ReadFile(filepath.Join(root, name)); err == nil {
				_ = yaml.Unmarshal(data, &parsed)
				return parsed
			}
		}
		return parsed
	}
	main := read("taskfile.yml")
	for name := range main.Tasks {
		targets[name] = true
	}
	for namespace, include := range main.Includes {
		included := read(path.Clean(include.Taskfile))
		for name := range included.Tasks {
			targets[namespace+":"+name] = true
			for _, alias := range include.Aliases {
				targets[alias+":"+name] = true
			}
		}
	}
	return targets
}
