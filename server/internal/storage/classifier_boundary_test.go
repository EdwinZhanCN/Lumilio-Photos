package storage

import (
	"fmt"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"testing"
)

// storageRiskLiterals is the closed risk vocabulary. Only the classifier
// defines it (storage_assessment.go, plus the continuity risk the native and
// candidate reviews add in host_action.go); the DTO package may cite a code
// in a swag `example:` tag. Anywhere else, a literal is a second classifier.
var storageRiskLiterals = []string{
	`"removable_storage"`, `"network_filesystem"`, `"cloud_sync_directory"`,
	`"non_default_location"`, `"mount_fingerprint_changed"`, `"unavailable_cloud_placeholder"`,
}

var classifierFiles = map[string]bool{
	"server/internal/storage/storage_assessment.go": true,
	"server/internal/storage/host_action.go":        true,
}

func storageClassifierViolation(relative, line string) bool {
	// Tests, this tool, and the generated OpenAPI document may spell a code.
	if strings.HasSuffix(relative, "_test.go") || strings.HasPrefix(relative, "server/tools/") ||
		strings.HasPrefix(relative, "server/docs/") {
		return false
	}
	code := stripLineComment(line)
	if classifierFiles[relative] || strings.HasPrefix(relative, "server/internal/api/dto/") {
		return false
	}
	for _, literal := range storageRiskLiterals {
		if strings.Contains(code, literal) {
			return true
		}
	}
	return false
}

// Risk codes are the classifier's: storage_assessment.go defines them (plus
// the continuity risk the native and candidate reviews add in host_action.go),
// and the DTO package may cite one in a swag example. A literal anywhere else
// in the Server is a second classifier; risks come from ClassifyStorage
// through AssessStorageTarget.
func TestStorageRiskCodesAreSpelledOnlyByTheClassifier(t *testing.T) {
	type row struct {
		relative, line string
		want           bool
	}
	classifier := []row{
		{"server/internal/storage/storage_assessment.go", `	RiskRemovable  StorageRisk = "removable_storage"`, false},
		{"server/internal/storage/host_action.go", `const RiskMountChanged StorageRisk = "mount_fingerprint_changed"`, false},
		{"server/internal/api/dto/storage_dto.go", "	AcknowledgedRisks []string `json:\"acknowledged_risks,omitempty\" example:\"removable_storage\"`", false},
		{"server/internal/storage/repository_candidates.go", `	warnings = append(warnings, "removable_storage")`, true},
		{"server/internal/storage/storage_path_info.go", `	info.RiskWarnings = append(info.RiskWarnings, "network_filesystem")`, true},
		{"server/internal/storage/storage_path_info.go", `	// network_filesystem used to be derived here; see "removable_storage"`, false},
		{"server/internal/storage/x_test.go", `	risks := []string{"removable_storage"}`, false},
		{"server/docs/docs.go", `                            "removable_storage"`, false},
	}
	for _, tc := range classifier {
		if got := storageClassifierViolation(tc.relative, tc.line); got != tc.want {
			t.Errorf("storageClassifierViolation(%s, %q) = %t, want %t", tc.relative, tc.line, got, tc.want)
		}
	}
}

// The rule above, over every Server source file.
func TestServerSpellsNoStorageRiskOutsideTheClassifier(t *testing.T) {
	var violations []string
	sourceLines(t, "../..", "server", func(location, line string) {
		relative := location[:strings.LastIndex(location, ":")]
		if storageClassifierViolation(relative, line) {
			violations = append(violations, location+": "+strings.TrimSpace(line))
		}
	})
	if len(violations) > 0 {
		t.Fatalf("storage risk code spelled outside the classifier:\n%s", strings.Join(violations, "\n"))
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
