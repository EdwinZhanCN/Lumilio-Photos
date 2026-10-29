package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"server/internal/api/problem"
)

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
