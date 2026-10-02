package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/doc"
	"go/doc/comment"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// goPackage is one parsed Go package (non-test files only).
type goPackage struct {
	module     GoModule
	dir        string
	importPath string
	name       string
	files      map[string]*ast.File
	sources    map[string][]byte
	fset       *token.FileSet
	docFile    string
	docText    string
	docDecls   bool
	strayDocs  []string
	directives map[string]string
	imports    map[string]bool
	lines      int
}

var skippedDirNames = map[string]bool{
	"testdata": true, "vendor": true, "node_modules": true, "frontend": true,
	"build": true, "bin": true, "dist": true,
}

func loadGoPackages(root string, cfg *Config) ([]*goPackage, error) {
	var packages []*goPackage
	for _, module := range cfg.GoModules {
		base := filepath.Join(root, module.Dir)
		err := filepath.WalkDir(base, func(current string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if current != base && (strings.HasPrefix(name, ".") || skippedDirNames[name]) {
				return filepath.SkipDir
			}
			rel := filepath.ToSlash(mustRel(root, current))
			if excluded(cfg.Exclude, rel) {
				return filepath.SkipDir
			}
			pkg, err := parseGoPackage(root, module, current)
			if err != nil {
				return err
			}
			if pkg != nil {
				packages = append(packages, pkg)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(packages, func(i, j int) bool { return packages[i].dir < packages[j].dir })
	return packages, nil
}

func excluded(patterns []string, rel string) bool {
	for _, pattern := range patterns {
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
			continue
		}
		if ok, _ := path.Match(pattern, rel); ok {
			return true
		}
	}
	return false
}

func mustRel(root, target string) string {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		panic(err)
	}
	return rel
}

func parseGoPackage(root string, module GoModule, dir string) (*goPackage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	rel := filepath.ToSlash(mustRel(root, dir))
	moduleRel := strings.TrimPrefix(strings.TrimPrefix(rel, module.Dir), "/")
	importPath := module.Path
	if moduleRel != "" {
		importPath = module.Path + "/" + moduleRel
	}
	pkg := &goPackage{
		module:     module,
		dir:        rel,
		importPath: importPath,
		files:      map[string]*ast.File{},
		sources:    map[string][]byte{},
		fset:       token.NewFileSet(),
		directives: map[string]string{},
		imports:    map[string]bool{},
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		filename := filepath.Join(dir, name)
		source, err := os.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		if bytes.Contains(source, []byte("//go:build ignore")) {
			continue
		}
		file, err := parser.ParseFile(pkg.fset, filename, source, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", filename, err)
		}
		fileRel := rel + "/" + name
		pkg.files[fileRel] = file
		pkg.sources[fileRel] = source
		pkg.lines += bytes.Count(source, []byte("\n"))
		if pkg.name == "" {
			pkg.name = file.Name.Name
		}
		for _, spec := range file.Imports {
			pkg.imports[strings.Trim(spec.Path.Value, `"`)] = true
		}
		if name == "doc.go" {
			pkg.docFile = fileRel
			if file.Doc != nil {
				pkg.docText = file.Doc.Text()
				for _, line := range file.Doc.List {
					if directive, ok := strings.CutPrefix(line.Text, "//atlas:"); ok {
						key, value, _ := strings.Cut(directive, " ")
						pkg.directives[key] = strings.TrimSpace(value)
					}
				}
			}
			pkg.docDecls = len(file.Decls) > 0
		} else if file.Doc != nil && strings.TrimSpace(file.Doc.Text()) != "" {
			pkg.strayDocs = append(pkg.strayDocs, fileRel)
		}
	}
	if len(pkg.files) == 0 {
		return nil, nil
	}
	sort.Strings(pkg.strayDocs)
	return pkg, nil
}

func shortHash(source []byte) string {
	sum := sha256.Sum256(source)
	return hex.EncodeToString(sum[:])[:16]
}

func (pkg *goPackage) slice(fileRel string, from, to token.Pos) []byte {
	source := pkg.sources[fileRel]
	start := pkg.fset.Position(from).Offset
	end := pkg.fset.Position(to).Offset
	if start < 0 || end > len(source) || start > end {
		return nil
	}
	return source[start:end]
}

func receiverName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return receiverName(typed.X)
	case *ast.IndexExpr:
		return receiverName(typed.X)
	case *ast.IndexListExpr:
		return receiverName(typed.X)
	case *ast.Ident:
		return typed.Name
	}
	return ""
}

// symbols indexes every top-level declaration, method, struct field, and
// interface method, and records the members of typed constant sets.
func (pkg *goPackage) symbols() []Symbol {
	var out []Symbol
	add := func(fileRel, name, kind string, node ast.Node) {
		position := pkg.fset.Position(node.Pos())
		first, _, _ := strings.Cut(name, ".")
		out = append(out, Symbol{
			Key:      "go:" + pkg.importPath + "." + name,
			Name:     name,
			Module:   pkg.dir,
			File:     fileRel,
			Line:     position.Line,
			Kind:     kind,
			Hash:     shortHash(pkg.slice(fileRel, node.Pos(), node.End())),
			Exported: ast.IsExported(first),
		})
	}
	members := map[string][]string{}
	for fileRel, file := range pkg.files {
		for _, decl := range file.Decls {
			switch typed := decl.(type) {
			case *ast.FuncDecl:
				if typed.Recv != nil && len(typed.Recv.List) > 0 {
					add(fileRel, receiverName(typed.Recv.List[0].Type)+"."+typed.Name.Name, "method", typed)
				} else {
					add(fileRel, typed.Name.Name, "func", typed)
				}
			case *ast.GenDecl:
				var lastType string
				for _, spec := range typed.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						add(fileRel, spec.Name.Name, "type", spec)
						switch body := spec.Type.(type) {
						case *ast.StructType:
							for _, field := range body.Fields.List {
								for _, name := range field.Names {
									add(fileRel, spec.Name.Name+"."+name.Name, "field", field)
								}
							}
						case *ast.InterfaceType:
							for _, method := range body.Methods.List {
								for _, name := range method.Names {
									add(fileRel, spec.Name.Name+"."+name.Name, "method", method)
								}
							}
						}
					case *ast.ValueSpec:
						kind := "var"
						if typed.Tok == token.CONST {
							kind = "const"
							if ident, ok := spec.Type.(*ast.Ident); ok {
								lastType = ident.Name
							} else if spec.Type != nil || len(spec.Values) > 0 {
								lastType = ""
							}
						}
						for _, name := range spec.Names {
							if name.Name == "_" {
								continue
							}
							add(fileRel, name.Name, kind, spec)
							if kind == "const" && lastType != "" {
								members[lastType] = append(members[lastType], name.Name)
							}
						}
					}
				}
			}
		}
	}
	for index := range out {
		if list, ok := members[out[index].Name]; ok && out[index].Kind == "type" {
			sort.Strings(list)
			out[index].Members = list
		}
	}
	return out
}

// goDocIndex resolves Go doc links ([Name], [pkg.Name], [path/pkg.Name.Method]).
type goDocIndex struct {
	byPath  map[string]*goPackage
	byName  map[string][]string
	symbols map[string]map[string]bool
	std     map[string]bool
	stdName map[string]string
}

func newGoDocIndex(packages []*goPackage, symbols []Symbol) *goDocIndex {
	index := &goDocIndex{
		byPath:  map[string]*goPackage{},
		byName:  map[string][]string{},
		symbols: map[string]map[string]bool{},
		std:     map[string]bool{},
		stdName: map[string]string{},
	}
	for _, pkg := range packages {
		index.byPath[pkg.importPath] = pkg
		index.byName[pkg.name] = append(index.byName[pkg.name], pkg.importPath)
		index.symbols[pkg.importPath] = map[string]bool{}
	}
	for _, symbol := range symbols {
		if !strings.HasPrefix(symbol.Key, "go:") {
			continue
		}
		importPath := strings.TrimSuffix(strings.TrimPrefix(symbol.Key, "go:"), "."+symbol.Name)
		if set := index.symbols[importPath]; set != nil {
			set[symbol.Name] = true
		}
	}
	if out, err := exec.Command("go", "list", "std").Output(); err == nil {
		for _, line := range strings.Fields(string(out)) {
			index.std[line] = true
			index.stdName[path.Base(line)] = line
		}
	}
	return index
}

func (index *goDocIndex) lookupPackage(name string) (string, bool) {
	if index.byPath[name] != nil || index.std[name] {
		return name, true
	}
	if paths := index.byName[name]; len(paths) == 1 {
		return paths[0], true
	}
	if len(index.byName[name]) > 1 {
		return name, true
	}
	if full, ok := index.stdName[name]; ok {
		return full, true
	}
	return name, true
}

// checkDocLinks reports doc links in a package comment that do not resolve.
func (index *goDocIndex) checkDocLinks(pkg *goPackage) []string {
	var problems []string
	parser := comment.Parser{
		LookupPackage: index.lookupPackage,
		LookupSym:     func(string, string) bool { return true },
	}
	parsed := parser.Parse(pkg.docText)
	walkDocLinks(parsed, func(link *comment.DocLink) {
		name := link.Name
		if link.Recv != "" {
			name = link.Recv + "." + link.Name
		}
		target := pkg.importPath
		if link.ImportPath != "" && link.Name == "" {
			if index.byPath[link.ImportPath] == nil && !index.std[link.ImportPath] {
				problems = append(problems, fmt.Sprintf("doc link [%s] names an unknown package", link.ImportPath))
			}
			return
		}
		if link.ImportPath != "" {
			target = link.ImportPath
			if index.std[target] {
				return
			}
			if len(index.byName[target]) > 1 {
				problems = append(problems, fmt.Sprintf("doc link [%s.%s] is ambiguous; qualify it with the full import path", target, name))
				return
			}
			if index.byPath[target] == nil {
				problems = append(problems, fmt.Sprintf("doc link [%s.%s] names an unknown package", target, name))
				return
			}
		}
		if !index.symbols[target][name] && !isPredeclared(link) {
			problems = append(problems, fmt.Sprintf("doc link [%s] does not resolve to a declaration in %s", linkText(link), target))
		}
	})
	return problems
}

func isPredeclared(link *comment.DocLink) bool {
	if link.ImportPath != "" || link.Recv != "" {
		return false
	}
	switch link.Name {
	case "error", "any", "string", "bool", "int", "int64", "byte", "rune", "nil", "true", "false", "context":
		return true
	}
	return false
}

func linkText(link *comment.DocLink) string {
	var parts []string
	if link.ImportPath != "" {
		parts = append(parts, link.ImportPath)
	}
	if link.Recv != "" {
		parts = append(parts, link.Recv)
	}
	parts = append(parts, link.Name)
	return strings.Join(parts, ".")
}

func walkDocLinks(parsed *comment.Doc, visit func(*comment.DocLink)) {
	var walkText func([]comment.Text)
	walkText = func(texts []comment.Text) {
		for _, text := range texts {
			switch typed := text.(type) {
			case *comment.DocLink:
				visit(typed)
			case *comment.Link:
				walkText(typed.Text)
			case comment.Italic, comment.Plain:
			}
		}
	}
	for _, block := range parsed.Content {
		switch typed := block.(type) {
		case *comment.Paragraph:
			walkText(typed.Text)
		case *comment.Heading:
			walkText(typed.Text)
		case *comment.List:
			for _, item := range typed.Items {
				for _, content := range item.Content {
					if paragraph, ok := content.(*comment.Paragraph); ok {
						walkText(paragraph.Text)
					}
				}
			}
		}
	}
}

// renderGoDoc renders a package comment to HTML whose doc links navigate the
// Atlas site.
func (index *goDocIndex) renderGoDoc(pkg *goPackage, moduleOf func(string) string) (string, string) {
	parser := comment.Parser{
		LookupPackage: index.lookupPackage,
		LookupSym:     func(string, string) bool { return true },
	}
	parsed := parser.Parse(pkg.docText)
	printer := comment.Printer{
		HeadingLevel: 3,
		DocLinkURL: func(link *comment.DocLink) string {
			target := pkg.importPath
			if link.ImportPath != "" {
				target = link.ImportPath
			}
			module := moduleOf(target)
			if module == "" {
				return "https://pkg.go.dev/" + target
			}
			if link.Name == "" {
				return "#/module/" + module
			}
			name := link.Name
			if link.Recv != "" {
				name = link.Recv + "." + link.Name
			}
			return "#/module/" + module + "?sym=" + name
		},
	}
	return string(printer.HTML(parsed)), new(doc.Package).Synopsis(pkg.docText)
}
