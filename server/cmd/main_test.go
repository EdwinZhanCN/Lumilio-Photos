package main

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestConfigInitAndValidate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server.toml")
	var stdout, stderr bytes.Buffer
	err := runConfigCLI([]string{
		"init",
		"--profile", "docker-external-proxy",
		"--origin", "https://photos.example.com",
		"--listen", "192.168.1.20:6680",
		"--trusted-proxy", "192.168.1.10/32",
		"--output", path,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("config init: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "passkey RP ID: photos.example.com") {
		t.Fatalf("init report = %q", stdout.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	if err := runConfigCLI([]string{"validate", "--config", path}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "configuration valid") ||
		!strings.Contains(stdout.String(), "keep the application listener loopback-only or firewalled") {
		t.Fatalf("validate report = %q", stdout.String())
	}
	if err := runConfigCLI([]string{
		"init",
		"--profile", "docker-acme",
		"--origin", "https://photos.example.com",
		"--email", "admin@example.com",
		"--output", path,
	}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("existing output error = %v", err)
	}
}
