// Command atlas builds and checks the Lumilio Atlas: architecture, sequence,
// data-flow, and lifecycle diagrams that are either derived from source or
// authored with anchors that must resolve to real code and contracts.
//
//	atlas generate        write docs/atlas/generated; fail on any problem or stale view
//	atlas check           fail if anything is invalid, stale, or not regenerated
//	atlas lock            record the current hash of every anchor (after reviewing
//	                      views) and drop baseline entries that no longer occur
//	atlas baseline <rule> record the current findings of a rule with no baseline yet
//	atlas data            write the site model to .local/atlas for the VitePress Atlas
//
// Every command needs --web-facts, written by web/scripts/atlas-facts.ts.
// Use the root Task targets (task atlas, atlas:generate, atlas:check,
// atlas:lock) rather than invoking this directly.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("usage: atlas <generate|check|lock|baseline <rule>|data> [--web-facts <file>] [--out dir]"))
	}
	command := os.Args[1]
	args := os.Args[2:]
	ruleID := ""
	if command == "baseline" {
		if len(args) == 0 || strings.HasPrefix(args[0], "-") {
			fail(errors.New("usage: atlas baseline <rule-id>"))
		}
		ruleID, args = args[0], args[1:]
	}
	flags := flag.NewFlagSet("atlas", flag.ExitOnError)
	webFacts := flags.String("web-facts", "", "Web facts JSON from web/scripts/atlas-facts.ts")
	out := flags.String("out", ".local/atlas", "data output directory, relative to the repository root")
	_ = flags.Parse(args)

	root, err := gitRoot()
	if err != nil {
		fail(err)
	}
	factsPath := *webFacts
	if factsPath == "" {
		factsPath = filepath.Join(root, ".local/atlas/web-facts.json")
	} else if !filepath.IsAbs(factsPath) {
		factsPath = filepath.Join(root, factsPath)
	}

	atlas, err := loadAtlas(root, atlasDir, factsPath)
	if err != nil {
		fail(err)
	}
	atlas.evaluate()
	files := atlas.generatedFiles()

	switch command {
	case "generate":
		if err := writeFiles(root, files); err != nil {
			fail(err)
		}
		if err := pruneGenerated(root, files); err != nil {
			fail(err)
		}
		fmt.Printf("atlas: wrote %d generated files (%d views, %d modules)\n", len(files), len(atlas.Views), len(atlas.Modules))
		report(atlas, nil)
	case "check":
		report(atlas, outdatedGenerated(root, files))
	case "lock":
		if len(atlas.Problems) > 0 {
			fmt.Fprint(os.Stderr, "atlas: fix these before locking:\n"+formatProblems(atlas.Problems))
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(root, atlasDir, "atlas.lock.json"), encodeLock(atlas.viewLock(), atlas.lockedBaselines(), atlas.lockedCeilings()), 0o644); err != nil {
			fail(err)
		}
		if err := writeFiles(root, files); err != nil {
			fail(err)
		}
		fmt.Printf("atlas: locked %d views; every anchored view is now marked verified\n", len(atlas.viewAnchors))
		if len(atlas.StaleBaselines) > 0 {
			fmt.Printf("atlas: dropped %d baseline entr(ies) that no longer occur\n", len(atlas.StaleBaselines))
		}
	case "baseline":
		baselines, ceilings := atlas.shrunkBaselines(true), atlas.shrunkCeilings(true)
		count := 0
		if rule := atlas.rule(ruleID); rule != nil && rule.Budgets != nil {
			recorded, err := atlas.baselineCeilings(*rule)
			if err != nil {
				fail(err)
			}
			ceilings, count = recorded, len(recorded[ruleID])
		} else {
			recorded, err := atlas.baselineRule(ruleID)
			if err != nil {
				fail(err)
			}
			baselines, count = recorded, len(recorded[ruleID])
		}
		if err := os.WriteFile(filepath.Join(root, atlasDir, "atlas.lock.json"), encodeLock(atlas.Lock, baselines, ceilings), 0o644); err != nil {
			fail(err)
		}
		fmt.Printf("atlas: baselined %d existing finding(s) of rule %s; they may only shrink from here\n", count, ruleID)
	case "data":
		target := filepath.Join(root, *out)
		if err := atlas.writeData(target); err != nil {
			fail(err)
		}
		fmt.Printf("atlas: site data written to %s (%d problems, %d stale anchors)\n", target, len(atlas.Problems), len(atlas.Stale))
	default:
		fail(fmt.Errorf("unknown command %q", command))
	}
}

func report(atlas *Atlas, outdated []string) {
	failed := false
	if len(atlas.Problems) > 0 {
		failed = true
		fmt.Fprintf(os.Stderr, "atlas: %d problem(s):\n%s", len(atlas.Problems), formatProblems(atlas.Problems))
	}
	if len(atlas.StaleBaselines) > 0 {
		failed = true
		fmt.Fprintf(os.Stderr, "atlas: %d baseline entr(ies) no longer occur; run `task atlas:lock` to drop them:\n%s", len(atlas.StaleBaselines), formatProblems(atlas.StaleBaselines))
	}
	if len(atlas.Stale) > 0 {
		failed = true
		fmt.Fprintf(os.Stderr, "atlas: %d anchor(s) changed since their view was verified:\n%s", len(atlas.Stale), formatProblems(atlas.Stale))
		fmt.Fprintln(os.Stderr, "  Re-read each listed view against the code, fix the view if the behaviour changed, then run `task atlas:lock`.")
	}
	if len(outdated) > 0 {
		failed = true
		fmt.Fprintf(os.Stderr, "atlas: generated files are out of date; run `task atlas:generate`:\n  %s\n", strings.Join(outdated, "\n  "))
	}
	if failed {
		os.Exit(1)
	}
	fmt.Println("atlas: all views resolve, all anchors are verified, and generated files are current")
}

// evaluate runs every check over the loaded facts, in the order later steps
// depend on: group edges feed the derived views and the group_uses rule.
func (a *Atlas) evaluate() {
	edges := a.checkGroups()
	a.checkMatches()
	a.deriveViews(edges)
	a.checkViews()
	a.checkDocRefs()
	a.runRules()
	for _, view := range a.Views {
		view.Mermaid = a.mermaid(view)
	}
}

func gitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("find repository root: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "atlas: %v\n", err)
	os.Exit(1)
}
