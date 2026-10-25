package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// P7 of the storage refactor removed every competing path: one storage
// classifier, no Desktop marker parsing, no handler-built action lists, no
// create risks taken from candidates, and every closed vocabulary the
// contract exposes covered by literal copy. These checks keep them removed.
// Each rule is a pure function over source lines so a test can hand it a
// deliberate violation.

// storageRiskLiterals is the closed risk vocabulary. Only the classifier
// defines it (storage_assessment.go, plus the continuity risk the native and
// candidate reviews add in host_action.go); the DTO package may cite a code
// in a swag `example:` tag. Anywhere else, a literal is a second classifier.
var storageRiskLiterals = []string{
	`"removable_storage"`, `"network_filesystem"`, `"cloud_sync_directory"`,
	`"non_default_location"`, `"mount_fingerprint_changed"`, `"unavailable_cloud_placeholder"`,
}

// retiredClassifierSymbols are the legacy warning paths P7 deleted.
var retiredClassifierSymbols = []string{
	"StoragePlacementWarnings", "StorageLocationWarnings", "repositoryCandidateRiskWarnings",
	"rejectRepositoryCandidateRisk", "inspectHostActionStoragePath",
}

var classifierFiles = map[string]bool{
	"server/internal/storage/storage_assessment.go": true,
	"server/internal/storage/host_action.go":        true,
}

func checkStorageClassifierBoundary(root string) error {
	violations, err := scanGoLines(root, "server", func(relative, line string) bool {
		return storageClassifierViolation(relative, line)
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf(
			"Storage classifier check found a second classifier or a retired warning path:\n%s\nRisks come from storage.ClassifyStorage through AssessStorageTarget; facts stay facts everywhere else.",
			strings.Join(violations, "\n"),
		)
	}
	return nil
}

func storageClassifierViolation(relative, line string) bool {
	// Tests, this tool, and the generated OpenAPI document may spell a code.
	if strings.HasSuffix(relative, "_test.go") || strings.HasPrefix(relative, "server/tools/") ||
		strings.HasPrefix(relative, "server/docs/") {
		return false
	}
	code := stripLineComment(line)
	for _, symbol := range retiredClassifierSymbols {
		if strings.Contains(code, symbol) {
			return true
		}
	}
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

// The Desktop host reads no portable marker: it asks server/app. Any marker
// file name or YAML decoding in Desktop Go code is a parser coming back.
func checkDesktopMarkerParsing(root string) error {
	violations, err := scanGoLines(root, "desktop", func(relative, line string) bool {
		return desktopMarkerViolation(relative, line)
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf(
			"Desktop marker check found Desktop code reading a portable marker:\n%s\nThe Desktop asks app.ReviewDefaultStorageMove; the Server's typed readers are the only marker parsers.",
			strings.Join(violations, "\n"),
		)
	}
	return nil
}

func desktopMarkerViolation(relative, line string) bool {
	if strings.HasSuffix(relative, "_test.go") {
		return false
	}
	code := stripLineComment(line)
	return strings.Contains(code, `.lumilioroot`) || strings.Contains(code, `.lumiliorepo`) ||
		strings.Contains(code, `yaml.Unmarshal(`) || strings.Contains(code, `"gopkg.in/yaml`) ||
		strings.Contains(code, `"github.com/goccy/go-yaml"`)
}

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

func checkHandlerActionLists(root string) error {
	violations, err := scanGoLines(root, "server/internal/api/handler", func(relative, line string) bool {
		return handlerActionListViolation(relative, line)
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf(
			"Handler action-list check found a handler building an action or resolution list:\n%s\nThe storage package owns every action vocabulary (policy verdicts, Rejection.Actions, HostActionResolutions, the provisioning errors' Actions).",
			strings.Join(violations, "\n"),
		)
	}
	return nil
}

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

// Candidates suggest folders. Their risks, and the acknowledgement of those
// risks, come from the open_repository review of the exact folder; a
// candidate that carries risks, or a candidate command that takes a boolean
// confirmation, is the create-risk authority coming back.
func checkCandidateCreateRisks(root string) error {
	serverViolations, err := scanGoLines(root, "server", func(relative, line string) bool {
		return candidateRiskViolation(relative, line)
	})
	if err != nil {
		return err
	}
	webViolations, err := scanTextPaths(root, []string{"web/src/features"}, func(relative, line string) bool {
		return webCandidateRiskViolation(relative, line)
	})
	if err != nil {
		return err
	}
	violations := append(serverViolations, webViolations...)
	if len(violations) > 0 {
		return fmt.Errorf(
			"Candidate risk check found create risks taken from a candidate:\n%s\nRisks come from the open_repository assessment and are acknowledged with its token.",
			strings.Join(violations, "\n"),
		)
	}
	return nil
}

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

func webCandidateRiskViolation(relative, line string) bool {
	if strings.Contains(relative, ".test.") || strings.Contains(relative, ".spec.") {
		return false
	}
	// The native task's storage_risk decision carries its reviewed codes
	// (P3b); every other risk list in a feature is a candidate's.
	if strings.HasSuffix(relative, "NativeHostActionModal.tsx") {
		return false
	}
	return strings.Contains(line, "risk_warnings") || strings.Contains(line, "risk_confirmation") ||
		strings.Contains(line, "candidate.risk")
}

// vocabulary is one closed code set the contract exposes, where the Server
// defines it, and where each side renders it.
type vocabulary struct {
	Name    string
	Source  string
	Pattern *regexp.Regexp
	Copies  []copyTarget
}

// copyTarget is a presenter file and the literal shape a covered code takes
// in it: a record key (`code:`), a switch case (`case "code":`), or a quoted
// literal.
type copyTarget struct {
	File    string
	Literal func(code string) string
}

func recordKey(code string) string  { return code + ":" }
func switchCase(code string) string { return `case "` + code + `":` }

var contractVocabularies = []vocabulary{
	{
		Name:    "storage risk code",
		Source:  "server/internal/storage",
		Pattern: regexp.MustCompile(`StorageRisk\s*=\s*"([a-z_]+)"`),
		Copies: []copyTarget{
			{File: "web/src/features/repositories/model/storageRiskCopy.ts", Literal: recordKey},
			{File: "desktop/frontend/src/features/storage/storageRiskCopy.ts", Literal: recordKey},
		},
	},
	{
		Name:    "repository action blocker",
		Source:  "server/internal/storage",
		Pattern: regexp.MustCompile(`RepositoryActionBlocker\s*=\s*"([a-z_]+)"`),
		Copies:  []copyTarget{{File: "web/src/lib/http-commons/problem.ts", Literal: switchCase}},
	},
	{
		Name:    "repository action alternative",
		Source:  "server/internal/storage",
		Pattern: regexp.MustCompile(`RepositoryActionAlternative\s*=\s*"([a-z_]+)"`),
		Copies:  []copyTarget{{File: "web/src/lib/http-commons/problem.ts", Literal: switchCase}},
	},
	{
		Name:    "repository action",
		Source:  "server/internal/storage",
		Pattern: regexp.MustCompile(`RepositoryAction\s*=\s*"([a-z_]+)"`),
		Copies:  []copyTarget{{File: "web/src/features/repositories/model/storageActionCopy.ts", Literal: recordKey}},
	},
	{
		Name:    "storage location action",
		Source:  "server/internal/storage",
		Pattern: regexp.MustCompile(`StorageLocationAction\s*=\s*"([a-z_]+)"`),
		Copies:  []copyTarget{{File: "web/src/features/repositories/model/storageActionCopy.ts", Literal: recordKey}},
	},
	{
		Name:    "repository candidate classification",
		Source:  "server/internal/storage",
		Pattern: regexp.MustCompile(`RepositoryCandidate[A-Za-z]+\s*=\s*"([a-z_]+)"`),
		Copies:  []copyTarget{{File: "web/src/features/repositories/flows/storage-panel/RepositoryCandidateModal.tsx", Literal: switchCase}},
	},
	{
		Name:    "recovery item class",
		Source:  "server/internal/storage",
		Pattern: regexp.MustCompile(`PrivateClass[A-Za-z]+\s+PrivateStateClass\s*=\s*"([a-z_]+)"`),
		Copies:  []copyTarget{{File: "web/src/features/repositories/flows/storage-panel/recoveryCopy.ts", Literal: recordKey}},
	},
	{
		Name:    "recovery disposition",
		Source:  "server/internal/storage",
		Pattern: regexp.MustCompile(`Disposition[A-Za-z]+\s+RecoveryDisposition\s*=\s*"([a-z_]+)"`),
		Copies:  []copyTarget{{File: "web/src/features/repositories/flows/storage-panel/recoveryCopy.ts", Literal: recordKey}},
	},
}

func checkContractEnumCoverage(root string) error {
	violations, err := contractEnumViolations(root, contractVocabularies)
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf(
			"Contract enum coverage check found a code without literal copy:\n%s\nEvery closed vocabulary the contract exposes has an explicit en/zh presenter entry; a new code lands with its copy.",
			strings.Join(violations, "\n"),
		)
	}
	return nil
}

func contractEnumViolations(root string, vocabularies []vocabulary) ([]string, error) {
	var violations []string
	for _, vocab := range vocabularies {
		codes, err := vocabularyCodes(root, vocab)
		if err != nil {
			return nil, err
		}
		if len(codes) == 0 {
			violations = append(violations, fmt.Sprintf("%s: no codes found under %s", vocab.Name, vocab.Source))
			continue
		}
		for _, target := range vocab.Copies {
			source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(target.File)))
			if err != nil {
				return nil, fmt.Errorf("read %s presenter %s: %w", vocab.Name, target.File, err)
			}
			for _, code := range codes {
				if !strings.Contains(string(source), target.Literal(code)) {
					violations = append(violations, fmt.Sprintf("%s %q has no copy in %s", vocab.Name, code, target.File))
				}
			}
		}
	}
	sort.Strings(violations)
	return violations, nil
}

func vocabularyCodes(root string, vocab vocabulary) ([]string, error) {
	seen := map[string]bool{}
	var codes []string
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(vocab.Source)), func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range vocab.Pattern.FindAllStringSubmatch(string(source), -1) {
			if !seen[match[1]] {
				seen[match[1]] = true
				codes = append(codes, match[1])
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect %s codes: %w", vocab.Name, err)
	}
	sort.Strings(codes)
	return codes, nil
}

func stripLineComment(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		return line[:index]
	}
	return line
}
