package handler

import (
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// actionVocabulary is every action and resolution code a storage command or
// Problem can carry. A handler that spells one inside a slice literal, or
// appends one to an action list, is building the list itself.
var actionVocabulary = map[string]bool{
	"verify": true, "rename": true, "detach": true, "reconnect": true, "rescan": true,
	"open": true, "diagnose": true, "relocate": true, "copy": true, "confirm_risk": true,
	"update_location": true, "add_separate": true, "export": true, "delete": true,
}

var (
	sliceLiteralPattern = regexp.MustCompile(`\[\]string\{\s*"([a-z_]+)"`)
	appendActionPattern = regexp.MustCompile(`append\((actions|resolutions|alternatives|blockers)\b[^)]*"([a-z_]+)"`)
)

func handlerActionListViolation(relative, line string) bool {
	if strings.HasSuffix(relative, "_test.go") {
		return false
	}
	code := stripLineComment(line)
	if match := sliceLiteralPattern.FindStringSubmatch(code); match != nil && actionVocabulary[match[1]] {
		return true
	}
	if match := appendActionPattern.FindStringSubmatch(code); match != nil && actionVocabulary[match[2]] {
		return true
	}
	return false
}

// The storage package owns every action and resolution vocabulary (policy
// verdicts, Rejection.Actions, HostActionResolutions, the provisioning
// errors' Actions); a handler projects it and never builds a list itself.
func TestHandlerActionListRule(t *testing.T) {
	type row struct {
		relative, line string
		want           bool
	}
	handlers := []row{
		{"server/internal/api/handler/asset_trash.go", `			actions = []string{"reconnect"}`, true},
		{"server/internal/api/handler/repository_scan_handler.go", `		api.WriteProblem(c, problem.NewRepositoryConflict(err, "x", id, []string{"open"}))`, true},
		{"server/internal/api/handler/host_action_handler.go", `			resolutions = append(resolutions, "update_location")`, true},
		{"server/internal/api/handler/host_action_handler.go", `			AllowedResolutions: storage.HostActionResolutions(conflict.Actions),`, false},
		{"server/internal/api/handler/asset_trash.go", `		api.WriteProblem(c, problem.NewRepositoryConflict(err, rejection.Reason, repositoryID, rejection.Actions()))`, false},
		{"server/internal/api/handler/asset_query_handler.go", `	names := []string{"south", "north", "west", "east"}`, false},
		{"server/internal/api/handler/x_test.go", `	actions := []string{"reconnect"}`, false},
	}
	for _, tc := range handlers {
		if got := handlerActionListViolation(tc.relative, tc.line); got != tc.want {
			t.Errorf("handlerActionListViolation(%s, %q) = %t, want %t", tc.relative, tc.line, got, tc.want)
		}
	}
}

// The rule above, over every handler.
func TestHandlersBuildNoActionLists(t *testing.T) {
	var violations []string
	sourceLines(t, ".", "server/internal/api/handler", func(location, line string) {
		relative := location[:strings.LastIndex(location, ":")]
		if handlerActionListViolation(relative, line) {
			violations = append(violations, location+": "+strings.TrimSpace(line))
		}
	})
	if len(violations) > 0 {
		t.Fatalf("a handler builds an action or resolution list:\n%s", strings.Join(violations, "\n"))
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
