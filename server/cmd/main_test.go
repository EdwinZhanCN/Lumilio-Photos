package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseCLIRequiresConfig(t *testing.T) {
	var stderr bytes.Buffer
	if _, err := parseCLI(nil, &stderr); err == nil {
		t.Fatal("expected missing config error")
	}
	if !strings.Contains(stderr.String(), "usage: server --config") {
		t.Fatalf("missing usage: %q", stderr.String())
	}
}

func TestParseCLIAcceptsOperatorControls(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseCLI([]string{
		"--config", "server.toml",
		"--pprof-addr", ":6060",
		"--agent-audit-log", "audit.jsonl",
		"--agent-ref-user-hot-budget-mib", "32",
		"--agent-ref-global-hot-budget-mib", "256",
	}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if opts.configPath != "server.toml" || opts.pprofAddr != ":6060" || opts.agentAuditLogPath != "audit.jsonl" ||
		opts.agentRefUserHotBudgetMiB != 32 || opts.agentRefGlobalHotBudgetMiB != 256 {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestParseCLIRejectsInvalidAgentRefBudgets(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseCLI([]string{
		"--config", "server.toml",
		"--agent-ref-user-hot-budget-mib", "128",
		"--agent-ref-global-hot-budget-mib", "64",
	}, &stderr)
	if err == nil || !strings.Contains(err.Error(), "global Agent ref hot-memory budget") {
		t.Fatalf("expected invalid budget error, got %v", err)
	}
}
