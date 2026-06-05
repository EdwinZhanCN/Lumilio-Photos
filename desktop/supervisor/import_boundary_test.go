package supervisor

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDesktopServerImportBoundary(t *testing.T) {
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve cwd: %v", err)
	}
	desktopRoot := filepath.Dir(wd)
	allowed := map[string]bool{
		"server/app":    true,
		"server/config": true,
	}

	err = filepath.WalkDir(desktopRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			importPath, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if strings.HasPrefix(importPath, "server/") && !allowed[importPath] {
				t.Errorf("desktop import boundary violation in %s: %s", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk desktop imports: %v", err)
	}
}
