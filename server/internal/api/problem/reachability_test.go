package problem_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"server/internal/api/problem"
)

// repositoryRoot is the checkout root, seen from this package directory.
const repositoryRoot = "../../../.."

// R10: every public Problem's executable recovery entry names a Web route
// that exists and, for a host action, a Desktop host action kind that
// exists. The entries live in the Problem registry; the routes are read from
// the Web route table and the kinds from the storage package, so a renamed
// route or kind fails here instead of leaving a dead entry behind a real
// failure.
// R10: every public Problem's executable recovery entry names a Web route
// that exists and, for a host action, a Desktop host action kind that
// exists. The entries live in the Problem registry; the routes are read from
// the Web route table and the kinds from the storage package, so a renamed
// route or kind fails here instead of leaving a dead entry behind a real
// failure.
func TestEveryRecoveryEntryCanBeFollowed(t *testing.T) {
	root := repositoryRoot
	routes, err := webRoutePaths(filepath.Join(root, "web/src/app/router/routes.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	hostActions, err := hostActionKinds(filepath.Join(root, "server/internal/storage/host_action.go"))
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string]problem.RecoveryEntry)
	for _, descriptor := range problem.Registered() {
		entries[descriptor.Type] = descriptor.Entry
	}
	violations := recoveryReachabilityViolations(entries, routes, hostActions)
	registry, err := os.ReadFile(filepath.Join(root, "web/src/lib/http-commons/problemRecovery.ts"))
	if err != nil {
		t.Fatalf("read generated Web recovery registry: %v", err)
	}
	for problemType := range entries {
		if !strings.Contains(string(registry), fmt.Sprintf("%q:", problemType)) {
			violations = append(violations, problemType+": missing from the generated Web recovery registry (run task dto)")
		}
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatalf("a recovery entry cannot be followed:\n%s", strings.Join(violations, "\n"))
	}
}

// recoveryReachabilityViolations is the pure rule: a route or sign_in entry
// must name a route of the table, a host_action entry a known kind and a
// starting route; the other kinds name nothing.
func recoveryReachabilityViolations(entries map[string]problem.RecoveryEntry, routes, hostActions map[string]bool) []string {
	var violations []string
	for problemType, entry := range entries {
		if err := entry.Validate(); err != nil {
			violations = append(violations, problemType+": "+err.Error())
			continue
		}
		if entry.Route != "" && !routes[entry.Route] {
			violations = append(violations, fmt.Sprintf("%s: route %s is not in web/src/app/router/routes.tsx", problemType, entry.Route))
		}
		if entry.HostAction != "" && !hostActions[entry.HostAction] {
			violations = append(violations, fmt.Sprintf("%s: host action %s is not a storage.HostActionKind", problemType, entry.HostAction))
		}
	}
	sort.Strings(violations)
	return violations
}

var (
	routePathPattern      = regexp.MustCompile(`path:\s*"([^"]+)"`)
	hostActionKindPattern = regexp.MustCompile(`HostActionKind\s*=\s*"([a-z_]+)"`)
)

func webRoutePaths(path string) (map[string]bool, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Web route table: %w", err)
	}
	routes := map[string]bool{}
	for _, match := range routePathPattern.FindAllStringSubmatch(string(source), -1) {
		routes[match[1]] = true
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("Web route table %s declares no path literals", path)
	}
	return routes, nil
}

func hostActionKinds(path string) (map[string]bool, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read host action kinds: %w", err)
	}
	kinds := map[string]bool{}
	for _, match := range hostActionKindPattern.FindAllStringSubmatch(string(source), -1) {
		kinds[match[1]] = true
	}
	if len(kinds) == 0 {
		return nil, fmt.Errorf("%s declares no HostActionKind literals", path)
	}
	return kinds, nil
}

// The check catches a deliberate violation: an entry whose route is not in
// the route table, one whose host action kind does not exist, and one that
// is not self-consistent, while a reachable registry passes.
func TestRecoveryReachabilityCatchesADeadEntry(t *testing.T) {
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
	write("web/src/app/router/routes.tsx", `export const appRoutes = [
  { path: "/storage", element: null },
  { path: "/storage/:repositoryId/trash", element: null },
];`)
	write("server/internal/storage/host_action.go", `package storage
const (
	HostActionLocateRepository HostActionKind = "locate_repository"
)`)
	routes, err := webRoutePaths(filepath.Join(root, "web/src/app/router/routes.tsx"))
	if err != nil {
		t.Fatal(err)
	}
	kinds, err := hostActionKinds(filepath.Join(root, "server/internal/storage/host_action.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !routes["/storage/:repositoryId/trash"] || !kinds["locate_repository"] || len(routes) != 2 || len(kinds) != 1 {
		t.Fatalf("parsed routes = %v kinds = %v", routes, kinds)
	}

	reachable := map[string]problem.RecoveryEntry{
		"a": {Actor: problem.ActorAdmin, Phase: problem.PhaseRunning, Kind: problem.RecoveryRoute, Route: "/storage"},
		"b": {Actor: problem.ActorDesktopHost, Phase: problem.PhaseRunning, Kind: problem.RecoveryHostAction, Route: "/storage", HostAction: "locate_repository"},
		"c": {Actor: problem.ActorUser, Phase: problem.PhaseAny, Kind: problem.RecoveryRetry},
	}
	if violations := recoveryReachabilityViolations(reachable, routes, kinds); len(violations) != 0 {
		t.Fatalf("reachable registry reported %v", violations)
	}
	dead := map[string]problem.RecoveryEntry{
		"dead-route":   {Actor: problem.ActorAdmin, Phase: problem.PhaseRunning, Kind: problem.RecoveryRoute, Route: "/storage/recovery"},
		"dead-kind":    {Actor: problem.ActorDesktopHost, Phase: problem.PhaseRunning, Kind: problem.RecoveryHostAction, Route: "/storage", HostAction: "repair_repository"},
		"inconsistent": {Actor: problem.ActorAdmin, Phase: problem.PhaseAny, Kind: problem.RecoveryRetry, Route: "/storage"},
	}
	want := []string{
		"dead-kind: host action repair_repository is not a storage.HostActionKind",
		"dead-route: route /storage/recovery is not in web/src/app/router/routes.tsx",
		"inconsistent: retry entry names no route or host action",
	}
	if got := recoveryReachabilityViolations(dead, routes, kinds); !reflect.DeepEqual(got, want) {
		t.Fatalf("violations = %v, want %v", got, want)
	}
}
