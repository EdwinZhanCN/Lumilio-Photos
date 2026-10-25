package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"server/internal/api/problem"
)

// R10: every public Problem's executable recovery entry names a Web route
// that exists and, for a host action, a Desktop host action kind that
// exists. The entries live in the Problem registry; the routes are read from
// the Web route table and the kinds from the storage package, so a renamed
// route or kind fails here instead of leaving a dead entry behind a real
// failure.
func checkRecoveryReachability(root string) error {
	routes, err := webRoutePaths(filepath.Join(root, "web/src/app/router/routes.tsx"))
	if err != nil {
		return err
	}
	hostActions, err := hostActionKinds(filepath.Join(root, "server/internal/storage/host_action.go"))
	if err != nil {
		return err
	}
	entries := make(map[string]problem.RecoveryEntry)
	for _, descriptor := range problem.Registered() {
		entries[descriptor.Type] = descriptor.Entry
	}
	violations := recoveryReachabilityViolations(entries, routes, hostActions)
	registry, err := os.ReadFile(filepath.Join(root, "web/src/lib/http-commons/problemRecovery.ts"))
	if err != nil {
		return fmt.Errorf("read generated Web recovery registry: %w", err)
	}
	for problemType := range entries {
		if !strings.Contains(string(registry), fmt.Sprintf("%q:", problemType)) {
			violations = append(violations, problemType+": missing from the generated Web recovery registry (run task dto)")
		}
	}
	sort.Strings(violations)
	if len(violations) > 0 {
		return fmt.Errorf("Recovery reachability check found an entry that cannot be followed:\n%s", strings.Join(violations, "\n"))
	}
	return nil
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
