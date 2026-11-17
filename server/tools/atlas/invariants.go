package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	markdownHeading = regexp.MustCompile(`^(#+)\s+(.*)$`)
	boldBullet      = regexp.MustCompile(`^- \*\*(.+?)\*\*`)
	headingNoise    = regexp.MustCompile(`[^a-z0-9 -]`)
)

// checkInvariants fails when a bullet of the invariant section has no
// registry entry, an entry has no bullet or no enforcement, or an
// enforcement reference does not resolve.
func (a *Atlas) checkInvariants() {
	registry := a.Config.Invariants
	if registry.Doc == "" {
		return
	}
	file, anchor, _ := strings.Cut(registry.Doc, "#")
	data, err := os.ReadFile(filepath.Join(a.Root, file))
	if err != nil {
		a.problem("atlas.yaml", "invariants doc %s cannot be read", file)
		return
	}
	bullets, found := sectionBullets(string(data), anchor)
	if !found {
		a.problem("atlas.yaml", "invariants doc %s has no heading #%s", file, anchor)
		return
	}
	entries := map[string]Invariant{}
	for _, entry := range registry.Entries {
		entries[entry.Title] = entry
	}
	for _, title := range bullets {
		if _, ok := entries[title]; !ok {
			a.problem(file, "invariant %q has no invariants entry in atlas.yaml naming what enforces it", title)
		}
	}
	listed := map[string]bool{}
	for _, title := range bullets {
		listed[title] = true
	}
	tasks := loadTaskTargets(a.Root)
	rules := map[string]bool{}
	for _, rule := range a.Config.Rules {
		rules[rule.ID] = true
	}
	for _, entry := range registry.Entries {
		if !listed[entry.Title] {
			a.problem("atlas.yaml", "invariants entry %q is not a bullet under %s", entry.Title, registry.Doc)
		}
		if len(entry.EnforcedBy) == 0 {
			a.problem("atlas.yaml", "invariant %q names no enforcement", entry.Title)
		}
		for _, ref := range entry.EnforcedBy {
			if !a.enforcementExists(ref, rules, tasks) {
				a.problem("atlas.yaml", "invariant %q: enforcement %s does not resolve", entry.Title, ref)
			}
		}
	}
}

// sectionBullets returns the bold titles of the top-level bullets under the
// heading whose anchor is anchor, up to the next heading of the same or a
// higher level.
func sectionBullets(text, anchor string) ([]string, bool) {
	var bullets []string
	level, inside := 0, false
	for _, line := range strings.Split(text, "\n") {
		if heading := markdownHeading.FindStringSubmatch(line); heading != nil {
			if inside && len(heading[1]) <= level {
				break
			}
			if headingAnchor(heading[2]) == anchor {
				level, inside = len(heading[1]), true
			}
			continue
		}
		if inside {
			if bullet := boldBullet.FindStringSubmatch(line); bullet != nil {
				bullets = append(bullets, bullet[1])
			}
		}
	}
	return bullets, inside
}

// headingAnchor is the GitHub and VitePress anchor of a heading.
func headingAnchor(heading string) string {
	return strings.ReplaceAll(headingNoise.ReplaceAllString(strings.ToLower(heading), ""), " ", "-")
}

func (a *Atlas) enforcementExists(ref string, rules, tasks map[string]bool) bool {
	kind, target, _ := strings.Cut(ref, ":")
	switch kind {
	case "rule":
		return rules[target]
	case "task":
		return tasks[target]
	case "go":
		dot := strings.LastIndex(target, ".")
		if dot < 0 || !strings.HasPrefix(target[dot+1:], "Test") {
			return false
		}
		return a.goTestExists(target[:dot], target[dot+1:])
	}
	return false
}

// goTestExists reports whether the package with importPath declares the
// test function name in one of its _test.go files.
func (a *Atlas) goTestExists(importPath, name string) bool {
	declaration := regexp.MustCompile(`(?m)^func ` + regexp.QuoteMeta(name) + `\(t \*testing\.T\)`)
	for _, pkg := range a.goPackages {
		if pkg.importPath != importPath {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(a.Root, pkg.dir, "*_test.go"))
		sort.Strings(files)
		for _, file := range files {
			if data, err := os.ReadFile(file); err == nil && declaration.Match(data) {
				return true
			}
		}
	}
	return false
}
