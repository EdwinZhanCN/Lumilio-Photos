package main

// This repository-level guard intentionally lives in Go so it can run on every
// supported development host without a POSIX shell or awk/grep pipeline.

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	queryConstPattern  = regexp.MustCompile("^const [[:alnum:]_]+ = " + string(rune(96)))
	numberedParameter  = regexp.MustCompile("\\?[[:digit:]]+")
	rawReference       = regexp.MustCompile("PhotoSpecificMetadata|dbtypes|is_raw.*(metadata|exif)|legacyBrowseStateMigration")
	stackListExclusion = regexp.MustCompile("useAssetsList\\.ts|schema\\.d\\.ts")
)

func main() {
	root, err := gitRoot()
	if err != nil {
		fail(err)
	}
	if err := checkSQLiteSQL(root); err != nil {
		fail(err)
	}
	fmt.Println("SQLite SQL checks passed")

	if err := checkBrowseArchitecture(root); err != nil {
		fail(err)
	}
	fmt.Println("Browse architecture checks passed")

	if err := checkDesktopArchitecture(root); err != nil {
		fail(err)
	}
	fmt.Println("Desktop architecture checks passed")

	if err := checkRepositoryPathArchitecture(root); err != nil {
		fail(err)
	}
	fmt.Println("Repository path architecture checks passed")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func gitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

func gitGrep(root, pattern string, paths ...string) (string, error) {
	args := []string{"grep", "-n", "-E", pattern, "--"}
	args = append(args, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return "", nil
	}
	return "", fmt.Errorf("git grep %q: %w: %s", pattern, err, strings.TrimSpace(string(output)))
}

func gitFiles(root string, pathspec string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--", pathspec)
	cmd.Dir = root
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", pathspec, err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func checkSQLiteSQL(root string) error {
	matches, err := gitGrep(
		root,
		"sqlc\\.(arg|narg|slice|embed)|@[[:alpha:]_][[:alnum:]_]*",
		"server/internal/db/repo/*.sql.go",
	)
	if err != nil {
		return err
	}
	if matches != "" {
		return fmt.Errorf(
			"SQLite SQL check found an unresolved sqlc parameter macro in generated SQL:\n%s",
			strings.TrimSpace(matches),
		)
	}

	matches, err = gitGrep(
		root,
		"randomblob[[:space:]]*\\(",
		"server/internal/db/repo/queries/*.sql",
	)
	if err != nil {
		return err
	}
	if matches != "" {
		return fmt.Errorf(
			"SQLite SQL check found database-side randomness in an application query:\n%s\nGenerate UUIDs and security-sensitive random values in Go and pass them explicitly.",
			strings.TrimSpace(matches),
		)
	}

	mixed, err := mixedSliceQueries(root)
	if err != nil {
		return err
	}
	if len(mixed) > 0 {
		return fmt.Errorf(
			"SQLite SQL check found a generated sqlc slice query mixed with fixed parameters:\n%s\nUse a JSON1 list parameter instead; sqlc numbered placeholders are invalid after dynamic slice expansion.",
			strings.Join(mixed, "\n"),
		)
	}
	return nil
}

func mixedSliceQueries(root string) ([]string, error) {
	files, err := gitFiles(root, "server/internal/db/repo/*.sql.go")
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, relative := range files {
		file, err := os.Open(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			return nil, fmt.Errorf("open generated query %s: %w", relative, err)
		}
		scanner := bufio.NewScanner(file)
		inQuery := false
		queryStart := 0
		hasSlice := false
		hasNumbered := false
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if queryConstPattern.MatchString(line) {
				inQuery = true
				queryStart = lineNumber
				hasSlice = false
				hasNumbered = false
			}
			if inQuery && strings.Contains(line, "/*SLICE:") && strings.Contains(line, "*/") {
				hasSlice = true
			}
			if inQuery && numberedParameter.MatchString(line) {
				hasNumbered = true
			}
			if inQuery && line == string(rune(96)) {
				if hasSlice && hasNumbered {
					violations = append(violations, fmt.Sprintf("%s:%d", relative, queryStart))
				}
				inQuery = false
			}
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("read generated query %s: %w", relative, err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close generated query %s: %w", relative, err)
		}
	}
	return violations, nil
}

func checkBrowseArchitecture(root string) error {
	matches, err := gitGrep(
		root,
		"IsRaw|filter\\.raw[^[:alnum:]_]|(^|[^[:alnum:]_])raw\\?:[[:space:]]*boolean|(^|[^[:alnum:]_])raw:[[:space:]]*(true|false)|params\\.(get|set|has|delete)\\(\"raw\"",
		"server/internal/api",
		"server/internal/service",
		"server/internal/agent",
		"web/src/features/assets",
		"web/src/features/search",
		"web/src/features/studio",
	)
	if err != nil {
		return err
	}
	if filtered := excludeLines(matches, rawReference); filtered != "" {
		return fmt.Errorf(
			"Browse architecture check found a retired RAW filter reference:\n%s\nBrowse/filter/search work on media items: use media_item.composition, not a raw boolean.",
			filtered,
		)
	}

	matches, err = gitGrep(root, "StackMode", "server/internal/service/asset_search_fused.go")
	if err != nil {
		return err
	}
	if matches != "" {
		return fmt.Errorf(
			"Browse architecture check found stack_mode in the search pipeline:\n%s\nSearch results are never stack-collapsed.",
			strings.TrimSpace(matches),
		)
	}

	matches, err = gitGrep(root, "stack_mode", "web/src")
	if err != nil {
		return err
	}
	if filtered := excludeLines(matches, stackListExclusion); filtered != "" {
		return fmt.Errorf(
			"Browse architecture check found stack_mode outside the browse list request:\n%s\nOnly useAssetsList.ts may send stack_mode; search rejects it with 400.",
			filtered,
		)
	}

	matches, err = gitGrep(root, "json:\"(results_)?total_assets", "server/internal/api/dto")
	if err != nil {
		return err
	}
	if matches != "" {
		return fmt.Errorf(
			"Browse architecture check found a retired total_assets browse count:\n%s\nUse total_visible / total_media_items / total_files.",
			strings.TrimSpace(matches),
		)
	}
	return nil
}

func excludeLines(matches string, excluded *regexp.Regexp) string {
	var kept []string
	for _, line := range strings.Split(strings.TrimSpace(matches), "\n") {
		if line != "" && !excluded.MatchString(line) {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func checkDesktopArchitecture(root string) error {
	imports, err := scanGoLines(root, "desktop", func(relative, line string) bool {
		return strings.Contains(line, "\"server/") &&
			!strings.HasPrefix(relative, "desktop/internal/runtime/")
	})
	if err != nil {
		return err
	}
	if len(imports) > 0 {
		return fmt.Errorf(
			"Desktop architecture check found a server import outside internal/runtime:\n%s",
			strings.Join(imports, "\n"),
		)
	}

	serverImports, err := scanGoLines(root, "server", func(_ string, line string) bool {
		return strings.HasPrefix(strings.TrimSpace(line), "\"desktop/")
	})
	if err != nil {
		return err
	}
	if len(serverImports) > 0 {
		return errors.New("Desktop architecture check found a server -> desktop import.")
	}
	return nil
}

func checkRepositoryPathArchitecture(root string) error {
	violations, err := scanGoLines(root, "server", func(relative, line string) bool {
		if strings.HasSuffix(relative, "_test.go") || strings.HasPrefix(relative, "server/internal/storage/") {
			return false
		}
		if !strings.Contains(line, "filepath.Join(") {
			return false
		}
		return strings.Contains(line, "repository.Path") ||
			strings.Contains(line, "repo.Path") ||
			strings.Contains(line, "repoPath") ||
			strings.Contains(line, "repositoryPath")
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf(
			"Repository path architecture check found a direct repository-root join outside storage:\n%s\nUse RepositoryFS; native tools may use its explicit local-path adapter.",
			strings.Join(violations, "\n"),
		)
	}
	return nil
}

func scanGoLines(root, directory string, match func(relative, line string) bool) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(file)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if match(relative, line) {
				matches = append(matches, fmt.Sprintf("%s:%d:%s", relative, lineNumber, line))
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return scanErr
		}
		return closeErr
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s Go files: %w", directory, err)
	}
	return matches, nil
}
