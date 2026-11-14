package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// splitReport is the analysis behind a package split: which files depend
// on which (type-resolved, through any declaration of the package), and,
// for a proposed cut into clusters of files, the edges between clusters,
// the unexported symbols a cut would have to export, and the cycles that
// would block it.
type splitReport struct {
	pkg   string
	files []string
	lines map[string]int
	// uses[from][to] holds the declarations of file to that file from uses.
	uses map[string]map[string]map[string]bool
}

// clusterFile is the --clusters input: each cluster names the files that
// would move together. Files no cluster names form the remainder.
type clusterFile struct {
	Clusters map[string][]string `yaml:"clusters"`
}

const remainderCluster = "(remainder)"

func runSplitReport(root string, config *Config, target, clustersPath string) (string, error) {
	report, err := loadSplitReport(root, config, target)
	if err != nil {
		return "", err
	}
	clusters := map[string]string{}
	if clustersPath != "" {
		data, err := os.ReadFile(clustersPath)
		if err != nil {
			return "", err
		}
		var parsed clusterFile
		if err := strictYAML(data, &parsed); err != nil {
			return "", fmt.Errorf("%s: %w", clustersPath, err)
		}
		known := map[string]bool{}
		for _, file := range report.files {
			known[file] = true
		}
		for name, files := range parsed.Clusters {
			for _, file := range files {
				if !known[file] {
					return "", fmt.Errorf("cluster %s names %s, which is not a production file of %s", name, file, report.pkg)
				}
				if previous, ok := clusters[file]; ok {
					return "", fmt.Errorf("%s is in clusters %s and %s", file, previous, name)
				}
				clusters[file] = name
			}
		}
	}
	return report.render(clusters), nil
}

func loadSplitReport(root string, config *Config, target string) (*splitReport, error) {
	moduleDir, rel, err := splitTarget(root, config, target)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	loaded, err := packages.Load(&packages.Config{
		// Dependencies are type-checked from source, as the map does: the
		// compiler's export data may be newer than this reader.
		Mode: typedLoadMode,
		Dir:  filepath.Join(root, moduleDir),
		Fset: fset,
		Env:  os.Environ(),
	}, "./"+rel)
	if err != nil {
		return nil, err
	}
	if len(loaded) != 1 {
		return nil, fmt.Errorf("%s matched %d packages", target, len(loaded))
	}
	pkg := loaded[0]
	for _, failure := range pkg.Errors {
		return nil, fmt.Errorf("type-check %s: %v (run through the Task target, which sets the build tags and toolchain)", pkg.PkgPath, failure)
	}
	return newSplitReport(fset, pkg.PkgPath, pkg.Types, pkg.TypesInfo, pkg.Syntax), nil
}

// splitTarget resolves a package directory given relative to the
// repository root or to a Go module (internal/service).
func splitTarget(root string, config *Config, target string) (string, string, error) {
	target = filepath.ToSlash(filepath.Clean(target))
	isDir := func(path string) bool {
		info, err := os.Stat(filepath.Join(root, path))
		return err == nil && info.IsDir()
	}
	for _, module := range config.GoModules {
		if rest, ok := strings.CutPrefix(target, module.Dir+"/"); ok && isDir(target) {
			return module.Dir, rest, nil
		}
	}
	for _, module := range config.GoModules {
		if isDir(module.Dir + "/" + target) {
			return module.Dir, target, nil
		}
	}
	return "", "", fmt.Errorf("no Go package directory %s in the repository or a Go module", target)
}

func newSplitReport(fset *token.FileSet, path string, pkg *types.Package, info *types.Info, syntax []*ast.File) *splitReport {
	report := &splitReport{pkg: path, lines: map[string]int{}, uses: map[string]map[string]map[string]bool{}}
	ours := map[string]bool{}
	for _, file := range syntax {
		name := filepath.Base(fset.Position(file.Pos()).Filename)
		ours[name] = true
		report.files = append(report.files, name)
		report.lines[name] = fset.File(file.Pos()).LineCount()
	}
	sort.Strings(report.files)
	for _, file := range syntax {
		from := filepath.Base(fset.Position(file.Pos()).Filename)
		ast.Inspect(file, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			obj := info.Uses[ident]
			if obj == nil || obj.Pkg() != pkg || !packageDeclaration(obj, pkg) {
				return true
			}
			to := filepath.Base(fset.Position(obj.Pos()).Filename)
			if to == from || !ours[to] {
				return true
			}
			if report.uses[from] == nil {
				report.uses[from] = map[string]map[string]bool{}
			}
			if report.uses[from][to] == nil {
				report.uses[from][to] = map[string]bool{}
			}
			report.uses[from][to][declarationName(obj)] = true
			return true
		})
	}
	return report
}

// packageDeclaration keeps package-level objects, methods, and struct
// fields; local variables and parameters never cross files.
func packageDeclaration(obj types.Object, pkg *types.Package) bool {
	if obj.Parent() == pkg.Scope() {
		return true
	}
	switch typed := obj.(type) {
	case *types.Func:
		return typed.Type().(*types.Signature).Recv() != nil
	case *types.Var:
		return typed.IsField()
	}
	return false
}

// declarationName is Name, Type.Method, or Type.field.
func declarationName(obj types.Object) string {
	if fn, ok := obj.(*types.Func); ok {
		if named := receiverNamed(fn); named != nil {
			return named.Obj().Name() + "." + fn.Name()
		}
	}
	if field, ok := obj.(*types.Var); ok && field.IsField() {
		return field.Origin().Name()
	}
	return obj.Name()
}

func (report *splitReport) render(clusters map[string]string) string {
	clusterOf := func(file string) string {
		if len(clusters) == 0 {
			return file
		}
		if name, ok := clusters[file]; ok {
			return name
		}
		return remainderCluster
	}
	var b strings.Builder
	total := 0
	for _, file := range report.files {
		total += report.lines[file]
	}
	edgeCount := 0
	for _, targets := range report.uses {
		edgeCount += len(targets)
	}
	fmt.Fprintf(&b, "# split-report %s\n\n%d files, %d lines, %d file-to-file edges\n", report.pkg, len(report.files), total, edgeCount)

	// Aggregate by cluster.
	members := map[string][]string{}
	for _, file := range report.files {
		members[clusterOf(file)] = append(members[clusterOf(file)], file)
	}
	graph := map[string]map[string]map[string]bool{}
	for from, targets := range report.uses {
		for to, names := range targets {
			a, z := clusterOf(from), clusterOf(to)
			if a == z {
				continue
			}
			if graph[a] == nil {
				graph[a] = map[string]map[string]bool{}
			}
			if graph[a][z] == nil {
				graph[a][z] = map[string]bool{}
			}
			for name := range names {
				graph[a][z][name] = true
			}
		}
	}
	if len(clusters) > 0 {
		b.WriteString("\n## Clusters\n\n| Cluster | Files | Lines |\n| --- | --- | --- |\n")
		for _, name := range sortedKeys(members) {
			lines := 0
			for _, file := range members[name] {
				lines += report.lines[file]
			}
			fmt.Fprintf(&b, "| %s | %s | %d |\n", name, strings.Join(members[name], " "), lines)
		}
		b.WriteString("\n## Edges between clusters\n\n")
		if len(graph) == 0 {
			b.WriteString("None.\n")
		}
		for _, from := range sortedKeys(graph) {
			for _, to := range sortedKeys(graph[from]) {
				fmt.Fprintf(&b, "- %s → %s: %s\n", from, to, strings.Join(sortedKeys(graph[from][to]), ", "))
			}
		}
		b.WriteString("\n## Unexported symbols crossing clusters\n\nA cut along these clusters exports each of these, or keeps them together.\n\n")
		crossing := 0
		for _, from := range sortedKeys(graph) {
			for _, to := range sortedKeys(graph[from]) {
				var unexported []string
				for _, name := range sortedKeys(graph[from][to]) {
					last := name[strings.LastIndex(name, ".")+1:]
					first := name[:strings.Index(name+".", ".")]
					if !ast.IsExported(last) || !ast.IsExported(first) {
						unexported = append(unexported, name)
					}
				}
				if len(unexported) > 0 {
					crossing += len(unexported)
					fmt.Fprintf(&b, "- %s → %s: %s\n", from, to, strings.Join(unexported, ", "))
				}
			}
		}
		if crossing == 0 {
			b.WriteString("None.\n")
		}
	}
	b.WriteString("\n## Cycles\n\n")
	cycles := stronglyConnected(graph)
	if len(cycles) == 0 {
		if len(clusters) > 0 {
			b.WriteString("None: the clusters can be cut apart in dependency order.\n")
		} else {
			b.WriteString("None between files.\n")
		}
	}
	for _, cycle := range cycles {
		fmt.Fprintf(&b, "- %s\n", strings.Join(cycle, ", "))
	}
	b.WriteString("\n## File-to-file dependencies\n\n")
	for _, from := range sortedKeys(report.uses) {
		for _, to := range sortedKeys(report.uses[from]) {
			fmt.Fprintf(&b, "- %s → %s: %s\n", from, to, strings.Join(sortedKeys(report.uses[from][to]), ", "))
		}
	}
	return b.String()
}

// stronglyConnected returns every strongly connected component with more
// than one member, each sorted, in a stable order (Tarjan).
func stronglyConnected(graph map[string]map[string]map[string]bool) [][]string {
	nodes := map[string]bool{}
	for from, targets := range graph {
		nodes[from] = true
		for to := range targets {
			nodes[to] = true
		}
	}
	index, low := map[string]int{}, map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	var out [][]string
	counter := 0
	var connect func(string)
	connect = func(node string) {
		index[node], low[node] = counter, counter
		counter++
		stack = append(stack, node)
		onStack[node] = true
		for _, next := range sortedKeys(graph[node]) {
			if _, seen := index[next]; !seen {
				connect(next)
				low[node] = min(low[node], low[next])
			} else if onStack[next] {
				low[node] = min(low[node], index[next])
			}
		}
		if low[node] == index[node] {
			var component []string
			for {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[top] = false
				component = append(component, top)
				if top == node {
					break
				}
			}
			if len(component) > 1 {
				sort.Strings(component)
				out = append(out, component)
			}
		}
	}
	for _, node := range sortedKeys(nodes) {
		if _, seen := index[node]; !seen {
			connect(node)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}
