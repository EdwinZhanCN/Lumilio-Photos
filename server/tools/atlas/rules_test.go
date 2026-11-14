package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Every rule in docs/atlas/atlas.yaml is proven by two fixtures under
// testdata/rules/<rule-id>/: violation/ must report exactly the findings in
// its want.txt, and pass/ must load with no problem at all. Each fixture is a
// tiny repository: atlas.yaml (goModules, groups, the rule) beside one or
// more Go modules, loaded and type-checked exactly as the real repository is.

func TestEveryDeclaredRuleHasFixtures(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", atlasDir, "atlas.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var config Config
	if err := strictYAML(data, &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Rules) == 0 {
		t.Fatal("docs/atlas/atlas.yaml declares no rules")
	}
	for _, rule := range config.Rules {
		for _, kind := range []string{"violation", "pass"} {
			if _, err := os.Stat(filepath.Join("testdata", "rules", rule.ID, kind, "atlas.yaml")); err != nil {
				t.Errorf("rule %s has no %s fixture: %v", rule.ID, kind, err)
			}
		}
	}
}

func TestRuleFixtures(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("testdata", "rules", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no rule fixtures: %v", err)
	}
	for _, dir := range dirs {
		ruleID := filepath.Base(dir)
		t.Run(ruleID+"/violation", func(t *testing.T) {
			t.Parallel()
			fixture := filepath.Join(dir, "violation")
			a := loadFixture(t, fixture)
			want := readLines(t, filepath.Join(fixture, "want.txt"))
			if len(want) == 0 {
				t.Fatal("a violation fixture must expect at least one finding")
			}
			if got := findingKeys(a.Findings[ruleID]); strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Fatalf("findings of %s:\n got: %q\nwant: %q", ruleID, got, want)
			}
			for _, problem := range a.Problems {
				if problem.Rule != ruleID {
					t.Errorf("violation fixture has an unrelated problem: %s: %s", problem.Where, problem.Message)
				}
			}
		})
		t.Run(ruleID+"/pass", func(t *testing.T) {
			t.Parallel()
			a := loadFixture(t, filepath.Join(dir, "pass"))
			if _, ok := a.Findings[ruleID]; !ok {
				t.Fatalf("the pass fixture does not declare rule %s", ruleID)
			}
			if len(a.Findings[ruleID]) != 0 || len(a.Problems) != 0 {
				t.Fatalf("pass fixture must be clean, got findings %q and problems:\n%s", findingKeys(a.Findings[ruleID]), formatProblems(a.Problems))
			}
		})
	}
}

func loadFixture(t *testing.T, dir string) *Atlas {
	t.Helper()
	root, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := loadAtlas(root, ".", "")
	if err != nil {
		t.Fatal(err)
	}
	a.evaluate()
	return a
}

func readLines(t *testing.T, file string) []string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			lines = append(lines, line)
		}
	}
	sort.Strings(lines)
	return lines
}

func findingKeys(findings []Finding) []string {
	keys := []string{}
	for _, finding := range findings {
		keys = append(keys, finding.Key)
	}
	sort.Strings(keys)
	return keys
}

func baselineAtlas(baselines map[string][]string, keys ...string) *Atlas {
	a := testAtlas()
	a.Config.Rules = []Rule{{ID: "r", Why: "test", GroupUses: true}}
	a.Baselines = baselines
	for _, key := range keys {
		a.groupFindings = append(a.groupFindings, Finding{Key: key, Where: key, Message: "violates " + key})
	}
	a.runRules()
	return a
}

func TestBaselinesOnlyShrink(t *testing.T) {
	// A baselined finding passes; a new one fails.
	a := baselineAtlas(map[string][]string{"r": {"old"}}, "old", "new")
	if got := messages(a.Problems); got != "violates new" {
		t.Fatalf("only the unbaselined finding may fail, got:\n%s", got)
	}

	// A fixed violation leaves a stale entry: check fails, lock drops it, and
	// lock never adds the new finding.
	a = baselineAtlas(map[string][]string{"r": {"fixed", "old"}}, "old", "new")
	if len(a.StaleBaselines) != 1 || !strings.Contains(a.StaleBaselines[0].Message, `"fixed"`) {
		t.Fatalf("the fixed entry must be stale, got:\n%s", messages(a.StaleBaselines))
	}
	if got := a.lockedBaselines()["r"]; strings.Join(got, ",") != "old" {
		t.Fatalf("lock must keep only entries that still occur, got %q", got)
	}

	// baseline records a new rule's findings once, and never again.
	a = baselineAtlas(nil, "b", "a")
	baselines, err := a.baselineRule("r")
	if err != nil || strings.Join(baselines["r"], ",") != "a,b" {
		t.Fatalf("a new rule's findings must be recorded, got %q, %v", baselines["r"], err)
	}
	a = baselineAtlas(map[string][]string{"r": {}}, "a")
	if _, err := a.baselineRule("r"); err == nil {
		t.Fatal("a rule that already has a baseline must not grow it")
	}

	// A baseline for a rule atlas.yaml no longer declares is stale.
	a = baselineAtlas(map[string][]string{"r": {}, "gone": {"x"}})
	if got := messages(a.StaleBaselines); !strings.Contains(got, "rule gone") {
		t.Fatalf("an undeclared rule's baseline must be stale, got:\n%s", got)
	}
}

func TestRuleDeclarationsAreValidated(t *testing.T) {
	a := testAtlas()
	a.Config.Rules = []Rule{{ID: "a", GroupUses: true}, {ID: "a", Why: "x", GroupUses: true}, {ID: "b", Why: "x"}, {Why: "x"}}
	a.runRules()
	got := messages(a.Problems)
	for _, want := range []string{"rule a needs a one-line `why`", "rule a is declared twice", "rule b must set exactly one kind", "every rule needs an id"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q, got:\n%s", want, got)
		}
	}
}

func TestGroupMembershipIsByMatch(t *testing.T) {
	a := testAtlas()
	a.Config.Groups = []Group{
		{ID: "one", Side: "server", Match: []string{"server/a", "server/shared/**", "server/gone"}},
		{ID: "two", Side: "server", Match: []string{"server/shared/b"}},
		{ID: "web", Side: "web", Match: []string{"server/web"}},
	}
	for index := range a.Config.Groups {
		a.groupByID[a.Config.Groups[index].ID] = &a.Config.Groups[index]
	}
	if got := a.matchGroup("server/a", "server"); got != "one" {
		t.Fatalf("server/a belongs to one, got %q", got)
	}
	a.matchGroup("server/shared/b", "server")
	a.matchGroup("server/web", "server")
	a.matchGroup("server/nowhere", "server")
	a.checkMatches()
	got := messages(a.Problems)
	for _, want := range []string{"groups one, two all match", `belongs to side "web"`, "no group in docs/atlas/atlas.yaml matches", `match "server/gone" matches no module`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q, got:\n%s", want, got)
		}
	}
}

func TestBaselineLeavesOtherNewRulesUnbaselined(t *testing.T) {
	a := testAtlas()
	a.Config.Rules = []Rule{{ID: "new", Why: "x", GroupUses: true}, {ID: "other", Why: "x", GroupUses: true}}
	a.groupFindings = []Finding{{Key: "k", Where: "w", Message: "m"}}
	a.runRules()
	baselines, err := a.baselineRule("new")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := baselines["other"]; ok {
		t.Fatalf("baselining one rule must not record a baseline for another: %v", baselines)
	}
}
