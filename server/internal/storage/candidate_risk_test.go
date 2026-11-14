package storage

import (
	"strings"
	"testing"
)

func candidateRiskViolation(relative, line string) bool {
	if strings.HasSuffix(relative, "_test.go") || strings.HasPrefix(relative, "server/tools/") {
		return false
	}
	code := stripLineComment(line)
	switch {
	case relative == "server/internal/storage/repository_candidates.go":
		// The legacy gate compared a candidate's own warnings with the bare
		// flag; the admitted request may still record that risks were
		// acknowledged after the token-bound review.
		return strings.Contains(code, "RiskWarnings") || strings.Contains(code, "!request.RiskConfirmation")
	case relative == "server/internal/storage/repo_manager.go":
		return strings.Contains(code, "RiskWarnings")
	case strings.HasPrefix(relative, "server/internal/api/dto/"):
		return strings.Contains(code, `json:"risk_confirmation`)
	default:
		return false
	}
}

// Candidates suggest folders. Their risks, and the acknowledgement of those
// risks, come from the open_repository review of the exact folder; a
// candidate that carries risks, or a candidate command that takes a boolean
// confirmation, is the create-risk authority coming back.
func TestCandidatesCarryNoCreateRisks(t *testing.T) {
	type row struct {
		relative, line string
		want           bool
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
			t.Errorf("candidateRiskViolation(%s, %q) = %t, want %t", tc.relative, tc.line, got, tc.want)
		}
	}
}

// The rule above, over every Server source file.
func TestServerTakesNoCreateRisksFromCandidates(t *testing.T) {
	var violations []string
	sourceLines(t, "../..", "server", func(location, line string) {
		relative := location[:strings.LastIndex(location, ":")]
		if candidateRiskViolation(relative, line) {
			violations = append(violations, location+": "+strings.TrimSpace(line))
		}
	})
	if len(violations) > 0 {
		t.Fatalf("create risks taken from a candidate; risks come from the open_repository assessment and are acknowledged with its token:\n%s", strings.Join(violations, "\n"))
	}
}
