package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

// The coverage rule reads the codes from the Server source and names every
// presenter file that lacks a literal entry for one of them.
func TestContractEnumCoverageNamesTheMissingCopy(t *testing.T) {
	root := t.TempDir()
	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("server/internal/storage/vocab.go", `package storage
const (
	RiskRemovable StorageRisk = "removable_storage"
	RiskCloudSync StorageRisk = "cloud_sync_directory"
)`)
	write("server/internal/storage/vocab_test.go", `package storage
const RiskTestOnly StorageRisk = "test_only"`)
	write("web/copy.ts", "const LABELS = {\n  removable_storage: (t) => t('a'),\n};\n")
	write("desktop/copy.ts", "const LABELS = {\n  removable_storage: (t) => t('a'),\n  cloud_sync_directory: (t) => t('b'),\n};\n")
	vocabularies := []vocabulary{{
		Name: "storage risk code", Source: "server/internal/storage",
		Pattern: regexp.MustCompile(`StorageRisk\s*=\s*"([a-z_]+)"`),
		Copies:  []copyTarget{{File: "web/copy.ts", Literal: recordKey}, {File: "desktop/copy.ts", Literal: recordKey}},
	}}
	got, err := contractEnumViolations(root, vocabularies)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`storage risk code "cloud_sync_directory" has no copy in web/copy.ts`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("violations = %v, want %v", got, want)
	}
}
