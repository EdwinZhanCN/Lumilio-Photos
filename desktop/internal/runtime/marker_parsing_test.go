package runtime

import (
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"testing"
)

func desktopMarkerViolation(relative, line string) bool {
	if strings.HasSuffix(relative, "_test.go") {
		return false
	}
	code := stripLineComment(line)
	// The YAML libraries are the Atlas desktop-marker-parsing rule's.
	return strings.Contains(code, `.lumilioroot`) || strings.Contains(code, `.lumiliorepo`) ||
		strings.Contains(code, `yaml.Unmarshal(`)
}

// The Desktop host reads no portable marker: it asks server/app. A marker
// file name or YAML decoding in Desktop code is a parser coming back; the
// Server's typed readers are the only marker parsers.
func TestDesktopMarkerRule(t *testing.T) {
	type row struct {
		relative, line string
		want           bool
	}
	desktop := []row{
		{"desktop/internal/runtime/runtimeconfig/store.go", `	root, err := loadPortableMarkerID(filepath.Join(candidate, ".lumilioroot"))`, true},
		{"desktop/internal/runtime/runtimeconfig/store.go", `	if err := yaml.Unmarshal(data, &marker); err != nil {`, true},
		{"desktop/internal/runtime/runtimeconfig/store.go", `	// the Server verifies the .lumilioroot identity on restart`, false},
		{"desktop/internal/runtime/runtimeconfig/store.go", `	if _, err := app.ReviewDefaultStorageMove(currentPath, candidatePath); err != nil {`, false},
		{"desktop/internal/runtime/runtimeconfig/store_test.go", `	write(filepath.Join(root, ".lumilioroot"), marker)`, false},
	}
	for _, tc := range desktop {
		if got := desktopMarkerViolation(tc.relative, tc.line); got != tc.want {
			t.Errorf("desktopMarkerViolation(%s, %q) = %t, want %t", tc.relative, tc.line, got, tc.want)
		}
	}
}

// The rule above, over every Desktop source file.
func TestDesktopReadsNoPortableMarker(t *testing.T) {
	var violations []string
	sourceLines(t, "../..", "desktop", func(location, line string) {
		relative := location[:strings.LastIndex(location, ":")]
		if desktopMarkerViolation(relative, line) {
			violations = append(violations, location+": "+strings.TrimSpace(line))
		}
	})
	if len(violations) > 0 {
		t.Fatalf("Desktop code reads a portable marker; ask app.ReviewDefaultStorageMove:\n%s", strings.Join(violations, "\n"))
	}
}

// sourceLines calls visit for every line of every non-test Go file under
// dir, outside testdata, with the file's path from the repository root.
func sourceLines(t *testing.T, dir, fromRoot string, visit func(relative, line string)) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		relative := pathpkg.Join(fromRoot, filepath.ToSlash(rel))
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for number, line := range strings.Split(string(source), "\n") {
			visit(fmt.Sprintf("%s:%d", relative, number+1), line)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func stripLineComment(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		return line[:index]
	}
	return line
}
