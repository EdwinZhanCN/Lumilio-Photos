package api

import (
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"testing"

	"server/internal/api/problem"
)

// Handlers answer failures through the closed Problem registry and
// api.WriteProblem: no legacy responder, no direct non-success JSON write, no
// Problem URI literal outside the registry, and no private error text in a
// public payload.
func TestHandlersAnswerFailuresWithRegisteredProblems(t *testing.T) {
	violations := walkSource(t, ".", "server/internal/api", func(relative, line string) bool {
		if strings.HasSuffix(relative, "_test.go") {
			return false
		}
		for _, retired := range []string{
			"ErrorResponse", "GinError", "GinBadRequest", "GinUnauthorized", "GinForbidden",
			"GinNotFound", "GinInternalError", "HandleError",
		} {
			if strings.Contains(line, retired) {
				return true
			}
		}
		if strings.Contains(line, "c.JSON(") && !successJSONWrite(line) {
			return true
		}
		if strings.Contains(line, problem.Namespace) && relative != "server/internal/api/problem/problem.go" {
			return true
		}
		if strings.Contains(line, ".Error()") && !allowedPrivateErrorUse(relative, line) {
			return true
		}
		return false
	})
	if len(violations) > 0 {
		t.Fatalf("legacy responder, non-success JSON write, unregistered URI literal, or raw error use:\n%s\nUse the closed Problem registry and api.WriteProblem; keep private causes out of public payloads.", strings.Join(violations, "\n"))
	}
}

func successJSONWrite(line string) bool {
	for _, status := range []string{
		"http.StatusOK", "http.StatusCreated", "http.StatusAccepted", "http.StatusNoContent",
		"http.StatusPartialContent", "200", "201", "202", "203", "204", "205", "206",
	} {
		if strings.Contains(line, "c.JSON("+status) {
			return true
		}
	}
	return false
}

func allowedPrivateErrorUse(relative, line string) bool {
	if relative == "server/internal/api/problem/problem.go" && strings.Contains(line, "panic(") {
		return true
	}
	// Upload-session manifests are private recovery state. The public progress
	// DTO deliberately omits this diagnostic and exposes only the stable state.
	return relative == "server/internal/api/handler/asset_upload_handler.go" && strings.Contains(line, "SetSessionError(")
}

// walkSource returns "path:line: text" for each line of the non-test Go
// files under dir that match.
func walkSource(t *testing.T, dir, fromRoot string, match func(relative, line string) bool) []string {
	t.Helper()
	var matches []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return err
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
			if match(relative, line) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", relative, number+1, strings.TrimSpace(line)))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return matches
}
