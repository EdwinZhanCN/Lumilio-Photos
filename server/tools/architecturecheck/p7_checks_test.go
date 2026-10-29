package main

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

// Each P7 rule catches a deliberate violation and lets the allowed shape
// through: the one classifier and swag examples may spell a risk code, the
// Desktop may mention a marker in a comment, a handler may project the
// storage vocabulary but not spell it, candidates carry no risks, and a
// vocabulary code without copy is named with the file that lacks it.
func TestP7RulesCatchDeliberateViolations(t *testing.T) {
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
		{"server/app/repository_control.go", `	return info, storage.StorageLocationWarnings(path), nil`, true},
		{"server/internal/storage/storage_path_info.go", `	// network_filesystem used to be derived here; see "removable_storage"`, false},
		{"server/internal/storage/x_test.go", `	risks := []string{"removable_storage"}`, false},
		{"server/docs/docs.go", `                            "removable_storage"`, false},
	}
	for _, tc := range classifier {
		if got := storageClassifierViolation(tc.relative, tc.line); got != tc.want {
			t.Fatalf("storageClassifierViolation(%s, %q) = %t, want %t", tc.relative, tc.line, got, tc.want)
		}
	}
	desktop := []row{
		{"desktop/internal/runtime/runtimeconfig/store.go", `	root, err := loadPortableMarkerID(filepath.Join(candidate, ".lumilioroot"))`, true},
		{"desktop/internal/runtime/runtimeconfig/store.go", `	if err := yaml.Unmarshal(data, &marker); err != nil {`, true},
		{"desktop/internal/runtime/runtimeconfig/store.go", `	"gopkg.in/yaml.v3"`, true},
		{"desktop/internal/runtime/runtimeconfig/store.go", `	// the Server verifies the .lumilioroot identity on restart`, false},
		{"desktop/internal/runtime/runtimeconfig/store.go", `	if _, err := app.ReviewDefaultStorageMove(currentPath, candidatePath); err != nil {`, false},
		{"desktop/internal/runtime/runtimeconfig/store_test.go", `	write(filepath.Join(root, ".lumilioroot"), marker)`, false},
	}
	for _, tc := range desktop {
		if got := desktopMarkerViolation(tc.relative, tc.line); got != tc.want {
			t.Fatalf("desktopMarkerViolation(%s, %q) = %t, want %t", tc.relative, tc.line, got, tc.want)
		}
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
			t.Fatalf("handlerActionListViolation(%s, %q) = %t, want %t", tc.relative, tc.line, got, tc.want)
		}
	}
	candidates := []row{
		{"server/internal/storage/repository_candidates.go", `		candidate.RiskWarnings = repositoryCandidateRiskWarnings(storageLocation, path, pathInfo)`, true},
		{"server/internal/storage/repository_candidates.go", `	if len(warnings) > 0 && !request.RiskConfirmation {`, true},
		{"server/internal/storage/repo_manager.go", `	RiskWarnings   []string`, true},
		{"server/internal/api/dto/repository_scan_dto.go", "	RiskConfirmation bool `json:\"risk_confirmation,omitempty\"`", true},
		{"server/internal/storage/repository_candidates.go", `	request, err = rm.admitCandidateTarget(ctx, request, lifecycleKindOpenRepository, &storageLocation, directoryName, path)`, false},
		{"server/internal/storage/repository_candidates.go", `	request.RiskConfirmation = len(acknowledged) > 0`, false},
		{"server/internal/storage/host_action.go", `	RiskWarnings []string ` + "`json:\"risk_warnings,omitempty\"`", false},
	}
	for _, tc := range candidates {
		if got := candidateRiskViolation(tc.relative, tc.line); got != tc.want {
			t.Fatalf("candidateRiskViolation(%s, %q) = %t, want %t", tc.relative, tc.line, got, tc.want)
		}
	}
	web := []row{
		{"web/src/features/repositories/flows/storage-panel/RepositoryCandidateModal.tsx", `  {(candidate.risk_warnings ?? []).map((risk) => (`, true},
		{"web/src/features/repositories/api/useRepositoryCandidates.ts", `        body: { directory_name: directoryName, risk_confirmation: riskConfirmation },`, true},
		{"web/src/features/repositories/flows/storage-panel/NativeHostActionModal.tsx", `        ? (action.result.conflict.risk_warnings ?? [])`, false},
		{"web/src/features/repositories/flows/storage-panel/StoragePanel.spec.tsx", `        risk_warnings: ["removable_storage"],`, false},
	}
	for _, tc := range web {
		if got := webCandidateRiskViolation(tc.relative, tc.line); got != tc.want {
			t.Fatalf("webCandidateRiskViolation(%s, %q) = %t, want %t", tc.relative, tc.line, got, tc.want)
		}
	}
}

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
