package db

import (
	"testing"

	"server/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIsSocketHost(t *testing.T) {
	cases := map[string]bool{
		"db":                              false,
		"localhost":                       false,
		"127.0.0.1":                       false,
		"/var/run/postgresql":             true,
		"/Users/me/Application Support/x": true,
	}
	for host, want := range cases {
		if got := isSocketHost(host); got != want {
			t.Errorf("isSocketHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestQuoteDSNValue(t *testing.T) {
	cases := map[string]string{
		"":                  "''",
		"simplehex0123":     "simplehex0123",
		"with space":        "'with space'",
		`with'quote`:        `'with\'quote'`,
		`with\backslash`:    `'with\\backslash'`,
		"/path/with spaces": "'/path/with spaces'",
	}
	for in, want := range cases {
		if got := quoteDSNValue(in); got != want {
			t.Errorf("quoteDSNValue(%q) = %q, want %q", in, got, want)
		}
	}
}

// socketDSN must be parseable by pgx and round-trip the socket directory host,
// including one containing spaces (the macOS Application Support path).
func TestSocketDSNParsesWithSpaces(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host:     "/Users/me/Library/Application Support/Lumilio Photos/postgres/16/run",
		Port:     "5487",
		User:     "lumilio",
		Password: "deadbeef",
		DBName:   "lumiliophotos",
		SSL:      "disable",
	}

	dsn := socketDSN(cfg, map[string]string{"search_path": "public"})

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgxpool.ParseConfig(%q) failed: %v", dsn, err)
	}
	if poolCfg.ConnConfig.Host != cfg.Host {
		t.Errorf("parsed host = %q, want %q", poolCfg.ConnConfig.Host, cfg.Host)
	}
	if poolCfg.ConnConfig.Database != cfg.DBName {
		t.Errorf("parsed dbname = %q, want %q", poolCfg.ConnConfig.Database, cfg.DBName)
	}
	if sp := poolCfg.ConnConfig.RuntimeParams["search_path"]; sp != "public" {
		t.Errorf("parsed search_path = %q, want public", sp)
	}
}
