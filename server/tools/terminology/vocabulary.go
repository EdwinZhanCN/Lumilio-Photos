package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

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
			{File: "desktop/internal/panel/text.go", Literal: switchCase},
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
