package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/ast/astutil"
)

// movecheck is the behaviour evidence of a move session: every function of
// the base and head trees indexed by receiver type and name, with package
// qualifiers dropped and export capitalization folded, so a function that
// only moved packages or was exported compares equal. What remains is the
// list a reviewer reads: functions whose normalized body changed (each
// needs a reason), renamed ones (same body, new name), and real additions
// and removals.
type movedFunc struct {
	key  string // receiver type and name, capitalization folded
	name string // as written: Recv.Name or Name
	file string
	body string // normalized signature and body
	// statements counts the body's top-level statements; an empty body
	// matches too many others to prove a rename.
	statements int
}

type moveResult struct {
	moved, changed, renamed [][2]movedFunc
	added, removed          []movedFunc
	unchanged               int
}

func runMovecheck(root string, config *Config, base string) (string, error) {
	var dirs []string
	for _, module := range config.GoModules {
		dirs = append(dirs, module.Dir)
	}
	baseFiles, err := gitTreeFiles(root, base, dirs)
	if err != nil {
		return "", err
	}
	headFiles, err := workingTreeFiles(root, dirs)
	if err != nil {
		return "", err
	}
	baseFuncs, err := indexFuncs(baseFiles)
	if err != nil {
		return "", fmt.Errorf("%s: %w", base, err)
	}
	headFuncs, err := indexFuncs(headFiles)
	if err != nil {
		return "", fmt.Errorf("working tree: %w", err)
	}
	return compareFuncs(baseFuncs, headFuncs).render(base), nil
}

func moveCandidate(file string) bool {
	return strings.HasSuffix(file, ".go") && !strings.Contains("/"+file, "/testdata/") && !strings.Contains("/"+file, "/vendor/")
}

// gitTreeFiles reads every Go file under dirs at ref.
func gitTreeFiles(root, ref string, dirs []string) (map[string][]byte, error) {
	list, err := exec.Command("git", append([]string{"-C", root, "ls-tree", "-r", "--name-only", ref, "--"}, dirs...)...).Output()
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", ref, err)
	}
	var names []string
	for _, name := range strings.Split(strings.TrimSpace(string(list)), "\n") {
		if moveCandidate(name) {
			names = append(names, name)
		}
	}
	command := exec.Command("git", "-C", root, "cat-file", "--batch")
	var input bytes.Buffer
	for _, name := range names {
		fmt.Fprintf(&input, "%s:%s\n", ref, name)
	}
	command.Stdin = &input
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", ref, err)
	}
	files := map[string][]byte{}
	reader := bufio.NewReader(bytes.NewReader(output))
	for _, name := range names {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		fields := strings.Fields(header)
		if len(fields) != 3 {
			return nil, fmt.Errorf("read %s:%s: %s", ref, name, strings.TrimSpace(header))
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, err
		}
		content := make([]byte, size+1)
		if _, err := io.ReadFull(reader, content); err != nil {
			return nil, err
		}
		files[name] = content[:size]
	}
	return files, nil
}

// workingTreeFiles reads every tracked or new, unignored Go file under dirs.
func workingTreeFiles(root string, dirs []string) (map[string][]byte, error) {
	list, err := exec.Command("git", append([]string{"-C", root, "ls-files", "--cached", "--others", "--exclude-standard", "--"}, dirs...)...).Output()
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	for _, name := range strings.Split(strings.TrimSpace(string(list)), "\n") {
		if !moveCandidate(name) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue // deleted in the working tree, not yet staged
		}
		if err != nil {
			return nil, err
		}
		files[name] = content
	}
	return files, nil
}

func indexFuncs(files map[string][]byte) ([]movedFunc, error) {
	var out []movedFunc
	for _, name := range sortedKeys(files) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, files[name], parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		imports := map[string]bool{}
		for _, spec := range file.Imports {
			importPath, _ := strconv.Unquote(spec.Path.Value)
			local := path.Base(importPath)
			if spec.Name != nil {
				local = spec.Name.Name
			}
			imports[local] = true
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			display := fn.Name.Name
			key := foldExport(fn.Name.Name)
			if fn.Recv != nil && len(fn.Recv.List) > 0 {
				recv := receiverName(fn.Recv.List[0].Type)
				display = recv + "." + fn.Name.Name
				key = foldExport(recv) + "." + key
			}
			statements := 0
			if fn.Body != nil {
				statements = len(fn.Body.List)
			}
			out = append(out, movedFunc{key: key, name: display, file: name, body: normalizedFunc(fset, fn, imports), statements: statements})
		}
	}
	return out, nil
}

// normalizedFunc prints a function's signature and body without comments,
// receiver, or name, with package qualifiers dropped and every identifier's
// export capitalization folded.
func normalizedFunc(fset *token.FileSet, fn *ast.FuncDecl, imports map[string]bool) string {
	var nodes []ast.Node
	nodes = append(nodes, fn.Type)
	if fn.Body != nil {
		nodes = append(nodes, fn.Body)
	}
	var b bytes.Buffer
	for _, node := range nodes {
		normalized := astutil.Apply(node, func(cursor *astutil.Cursor) bool {
			if selector, ok := cursor.Node().(*ast.SelectorExpr); ok {
				if qualifier, ok := selector.X.(*ast.Ident); ok && imports[qualifier.Name] {
					cursor.Replace(&ast.Ident{Name: foldExport(selector.Sel.Name)})
				}
			}
			return true
		}, func(cursor *astutil.Cursor) bool {
			if ident, ok := cursor.Node().(*ast.Ident); ok {
				ident.Name = foldExport(ident.Name)
			}
			return true
		})
		_ = printer.Fprint(&b, fset, normalized)
		b.WriteByte('\n')
	}
	return b.String()
}

func foldExport(name string) string {
	first, size := utf8.DecodeRuneInString(name)
	if first == utf8.RuneError {
		return name
	}
	return string(unicode.ToLower(first)) + name[size:]
}

func compareFuncs(base, head []movedFunc) moveResult {
	group := func(list []movedFunc) map[string][]movedFunc {
		out := map[string][]movedFunc{}
		for _, fn := range list {
			out[fn.key] = append(out[fn.key], fn)
		}
		return out
	}
	baseByKey, headByKey := group(base), group(head)
	var result moveResult
	var removed, added []movedFunc
	pair := func(from, to movedFunc) {
		switch {
		case from.body != to.body:
			result.changed = append(result.changed, [2]movedFunc{from, to})
		case path.Dir(from.file) != path.Dir(to.file) || from.name != to.name:
			result.moved = append(result.moved, [2]movedFunc{from, to})
		default:
			result.unchanged++
		}
	}
	keys := map[string]bool{}
	for key := range baseByKey {
		keys[key] = true
	}
	for key := range headByKey {
		keys[key] = true
	}
	for _, key := range sortedKeys(keys) {
		from, to := append([]movedFunc(nil), baseByKey[key]...), append([]movedFunc(nil), headByKey[key]...)
		// Same package directory first, then identical bodies, then a lone
		// pair, which is a move with changes.
		match := func(same func(a, b movedFunc) bool) {
			for i := 0; i < len(from); i++ {
				for j := 0; j < len(to); j++ {
					if same(from[i], to[j]) {
						pair(from[i], to[j])
						from = append(from[:i], from[i+1:]...)
						to = append(to[:j], to[j+1:]...)
						i--
						break
					}
				}
			}
		}
		match(func(a, b movedFunc) bool { return a.file == b.file })
		match(func(a, b movedFunc) bool { return path.Dir(a.file) == path.Dir(b.file) })
		match(func(a, b movedFunc) bool { return a.body == b.body })
		if len(from) == 1 && len(to) == 1 {
			pair(from[0], to[0])
			from, to = nil, nil
		}
		removed = append(removed, from...)
		added = append(added, to...)
	}
	// A removed and an added function with the same non-empty normalized
	// body, and no other function sharing it, were renamed.
	bodies := map[string]int{}
	for _, fn := range append(append([]movedFunc(nil), removed...), added...) {
		bodies[fn.body]++
	}
	for i := 0; i < len(removed); i++ {
		for j := 0; j < len(added); j++ {
			if removed[i].body == added[j].body && removed[i].statements > 0 && bodies[removed[i].body] == 2 {
				result.renamed = append(result.renamed, [2]movedFunc{removed[i], added[j]})
				removed = append(removed[:i], removed[i+1:]...)
				added = append(added[:j], added[j+1:]...)
				i--
				break
			}
		}
	}
	result.added, result.removed = added, removed
	return result
}

func (result moveResult) render(base string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# movecheck %s → working tree\n\n", base)
	fmt.Fprintf(&b, "%d changed · %d renamed · %d moved unchanged · %d added · %d removed · %d untouched\n",
		len(result.changed), len(result.renamed), len(result.moved), len(result.added), len(result.removed), result.unchanged)
	where := func(from, to movedFunc) string {
		if from.file == to.file {
			return "`" + to.file + "`"
		}
		return "`" + from.file + "` → `" + to.file + "`"
	}
	b.WriteString("\n## Changed bodies (give each a one-line reason)\n\n")
	if len(result.changed) == 0 {
		b.WriteString("None.\n")
	}
	for _, pair := range result.changed {
		name := pair[1].name
		if pair[0].name != pair[1].name {
			name = pair[0].name + " → " + pair[1].name
		}
		fmt.Fprintf(&b, "- `%s` %s — reason:\n", name, where(pair[0], pair[1]))
	}
	if len(result.renamed) > 0 {
		b.WriteString("\n## Renamed, body unchanged\n\n")
		for _, pair := range result.renamed {
			fmt.Fprintf(&b, "- `%s` → `%s` %s\n", pair[0].name, pair[1].name, where(pair[0], pair[1]))
		}
	}
	for _, section := range []struct {
		title string
		list  []movedFunc
	}{{"Added", result.added}, {"Removed", result.removed}} {
		if len(section.list) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n## %s\n\n", section.title)
		for _, fn := range section.list {
			fmt.Fprintf(&b, "- `%s` `%s`\n", fn.name, fn.file)
		}
	}
	if len(result.moved) > 0 {
		fmt.Fprintf(&b, "\n<details><summary>Moved, body unchanged (%d)</summary>\n\n", len(result.moved))
		sort.SliceStable(result.moved, func(i, j int) bool { return result.moved[i][1].file < result.moved[j][1].file })
		for _, pair := range result.moved {
			fmt.Fprintf(&b, "- `%s` %s\n", pair[1].name, where(pair[0], pair[1]))
		}
		b.WriteString("\n</details>\n")
	}
	return b.String()
}
