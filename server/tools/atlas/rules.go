package main

import (
	"fmt"
	"sort"
	"strings"
)

// A rule is declared in atlas.yaml under `rules:` and evaluated over the
// loaded facts. Its findings are compared with its baseline in
// atlas.lock.json:
//
//   - a finding the baseline does not list is a problem, so new violations
//     fail `atlas check`;
//   - a baseline entry that no longer occurs is stale: `atlas check` fails
//     until `atlas lock` drops it, so baselines only shrink;
//   - `atlas lock` never adds an entry. `atlas baseline <rule>` records the
//     current findings of a rule that has no baseline yet, which is how a new
//     rule lands with the violations that existed before it.
//
// Every rule ships a violation fixture and a passing fixture under
// testdata/rules/<rule-id>/; rules_test.go fails for a rule without them.

// ruleKind evaluates one kind of rule.
type ruleKind struct {
	name string
	set  func(Rule) bool
	eval func(*Atlas, Rule) []Finding
}

var ruleKinds = []ruleKind{
	{name: "group_uses", set: func(r Rule) bool { return r.GroupUses }, eval: (*Atlas).groupUsesFindings},
}

// runRules evaluates every declared rule and reconciles its findings with its
// baseline.
func (a *Atlas) runRules() {
	seen := map[string]bool{}
	for _, rule := range a.Config.Rules {
		where := "atlas.yaml"
		switch {
		case rule.ID == "":
			a.problem(where, "every rule needs an id")
			continue
		case seen[rule.ID]:
			a.problem(where, "rule %s is declared twice", rule.ID)
			continue
		}
		seen[rule.ID] = true
		if strings.TrimSpace(rule.Why) == "" {
			a.problem(where, "rule %s needs a one-line `why` naming the invariant it protects", rule.ID)
		}
		var kinds []ruleKind
		for _, kind := range ruleKinds {
			if kind.set(rule) {
				kinds = append(kinds, kind)
			}
		}
		if len(kinds) != 1 {
			a.problem(where, "rule %s must set exactly one kind, got %d", rule.ID, len(kinds))
			continue
		}
		findings := kinds[0].eval(a, rule)
		for index := range findings {
			findings[index].Rule = rule.ID
		}
		sort.Slice(findings, func(i, j int) bool { return findings[i].Key < findings[j].Key })
		a.Findings[rule.ID] = findings
		a.reconcileBaseline(rule.ID, findings)
	}
	for _, id := range sortedKeys(a.Baselines) {
		if !seen[id] {
			a.StaleBaselines = append(a.StaleBaselines, Problem{Where: "atlas.lock.json", Rule: id, Message: fmt.Sprintf("baseline for rule %s, which atlas.yaml no longer declares", id)})
		}
	}
}

func (a *Atlas) reconcileBaseline(ruleID string, findings []Finding) {
	baselined := map[string]bool{}
	for _, key := range a.Baselines[ruleID] {
		baselined[key] = true
	}
	found := map[string]bool{}
	for _, finding := range findings {
		found[finding.Key] = true
		if !baselined[finding.Key] {
			a.Problems = append(a.Problems, Problem{Where: finding.Where, Rule: ruleID, Message: finding.Message})
		}
	}
	for _, key := range a.Baselines[ruleID] {
		if !found[key] {
			a.StaleBaselines = append(a.StaleBaselines, Problem{Where: "atlas.lock.json", Rule: ruleID, Message: fmt.Sprintf("baselined violation %q no longer occurs", key)})
		}
	}
}

// lockedBaselines is what `atlas lock` writes: every declared rule's
// baseline with the entries that no longer occur removed. It never adds.
func (a *Atlas) lockedBaselines() map[string][]string {
	out := map[string][]string{}
	for _, rule := range a.Config.Rules {
		found := map[string]bool{}
		for _, finding := range a.Findings[rule.ID] {
			found[finding.Key] = true
		}
		kept := []string{}
		for _, key := range a.Baselines[rule.ID] {
			if found[key] {
				kept = append(kept, key)
			}
		}
		sort.Strings(kept)
		out[rule.ID] = kept
	}
	return out
}

// baselineRule records the current findings of a rule that has no baseline
// yet. A rule that already has one can only shrink it.
func (a *Atlas) baselineRule(ruleID string) (map[string][]string, error) {
	declared := false
	for _, rule := range a.Config.Rules {
		declared = declared || rule.ID == ruleID
	}
	if !declared {
		return nil, fmt.Errorf("rule %s is not declared in atlas.yaml", ruleID)
	}
	if _, ok := a.Baselines[ruleID]; ok {
		return nil, fmt.Errorf("rule %s already has a baseline; baselines only shrink (`atlas lock` drops fixed entries), and adding an entry by hand needs the owner's approval in the PR", ruleID)
	}
	out := a.lockedBaselines()
	keys := []string{}
	for _, finding := range a.Findings[ruleID] {
		keys = appendUnique(keys, finding.Key)
	}
	sort.Strings(keys)
	out[ruleID] = keys
	return out, nil
}

// groupUsesFindings reports the imports checkGroups found crossing into a
// group that the importer's group does not declare in `uses`.
func (a *Atlas) groupUsesFindings(Rule) []Finding {
	return append([]Finding(nil), a.groupFindings...)
}
