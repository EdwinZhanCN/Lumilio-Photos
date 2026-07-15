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
	opts, err := parseCLI([]string{"--config", "server.toml", "--pprof-addr", ":6060", "--agent-audit-log", "audit.jsonl"}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if opts.configPath != "server.toml" || opts.pprofAddr != ":6060" || opts.agentAuditLogPath != "audit.jsonl" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}
