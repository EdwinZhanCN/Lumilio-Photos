package db

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"server/config"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	mgpg "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// MigrationConfig holds configuration for running migrations.
type MigrationConfig struct {
	DatabaseConfig config.DatabaseConfig
	// Relative path to the migrations directory (from the workDir resolved at runtime).
	// Defaults to "migrations".
	MigrationsDir string
}

// NewMigrationConfig returns a MigrationConfig with sensible defaults.
func NewMigrationConfig(dbConfig config.DatabaseConfig) *MigrationConfig {
	return &MigrationConfig{
		DatabaseConfig: dbConfig,
		MigrationsDir:  "migrations",
	}
}

// buildURL constructs a Postgres connection URL with explicit ssl options.
func (m *MigrationConfig) buildURL() string {
	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%s/%s?sslmode=%s&search_path=public",
		m.DatabaseConfig.User,
		m.DatabaseConfig.Password,
		m.DatabaseConfig.Host,
		m.DatabaseConfig.Port,
		m.DatabaseConfig.DBName,
		m.DatabaseConfig.SSL,
	)
}

// resolveWorkDir attempts to locate the project server root when running from different cwd.
func resolveWorkDir() string {
	// Prefer server/ if exists since migrations live there.
	if _, err := os.Stat(filepath.Join("server", "migrations")); err == nil {
		return "server"
	}
	// Fallback to current directory.
	return "."
}

// migrateUp uses golang-migrate to apply all pending "up" migrations from the local file source.
func (m *MigrationConfig) migrateUp(ctx context.Context, workDir string) error {
	// Ensure the migrations directory exists (no-op if it already exists).
	migrationsPath := filepath.Join(workDir, m.MigrationsDir)
	if err := os.MkdirAll(migrationsPath, 0o755); err != nil {
		return fmt.Errorf("create migrations dir: %w", err)
	}

	absMigrationsPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return fmt.Errorf("resolve migrations absolute path: %w", err)
	}

	sourceURL := fmt.Sprintf("file://%s", absMigrationsPath)
	dbURL := m.buildURL()

	// Use pgx stdlib with database/sql so postgresql:// URLs work.
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("sql open (pgx): %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}

	// Wrap db with migrate's postgres driver
	pgDriver, err := mgpg.WithInstance(db, &mgpg.Config{})
	if err != nil {
		return fmt.Errorf("postgres driver instance: %w", err)
	}

	log.Printf("🚀 Applying DB migrations (source=%s)", sourceURL)
	migrator, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", pgDriver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer func() {
		if _, err := migrator.Close(); err != nil && !strings.Contains(err.Error(), "no such file or directory") {
			log.Printf("migration close warning: %v", err)
		}
	}()

	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}

	if err == migrate.ErrNoChange {
		log.Printf("ℹ️ No migration needed. Database schema is up-to-date.")
	} else {
		log.Printf("✅ Database migrations applied successfully.")
	}
	return nil
}

// runRiverMigrations executes River's CLI migration command (if available).
func (m *MigrationConfig) runRiverMigrations(ctx context.Context) error {
	databaseURL := m.buildURL()
	cmd := exec.CommandContext(ctx, "river", "migrate-up", "--database-url", databaseURL)

	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	log.Printf("🌊 Running River migrations...")
	err := cmd.Run()

	stdout := strings.TrimSpace(out.String())
	stderr := strings.TrimSpace(errb.String())
	if stdout != "" {
		log.Println(stdout)
	}
	if err != nil {
		if stderr != "" {
			return fmt.Errorf("river migrate-up failed: %w\n%s", err, stderr)
		}
		return fmt.Errorf("river migrate-up failed: %w", err)
	}
	if stderr != "" {
		log.Println(stderr)
	}
	return nil
}

// RunMigrations applies DB migrations using golang-migrate and then applies River migrations.
func (m *MigrationConfig) RunMigrations(ctx context.Context) error {
	// Verify River CLI exists before starting (we still run DB migrations even if River is missing?).
	// Here, we choose to fail early to keep both systems in sync on startup.
	if err := checkRiverAvailable(ctx); err != nil {
		return fmt.Errorf("River CLI not available: %w", err)
	}

	workDir := resolveWorkDir()

	// Apply DB migrations (up).
	if err := m.migrateUp(ctx, workDir); err != nil {
		return err
	}

	// Apply River migrations.
	if err := m.runRiverMigrations(ctx); err != nil {
		return err
	}

	log.Printf("✅ All migrations completed successfully.")
	return nil
}

// AutoMigrate is a convenience wrapper used by main() to run migrations at startup.
func AutoMigrate(ctx context.Context, dbConfig config.DatabaseConfig) error {
	m := NewMigrationConfig(dbConfig)
	return m.RunMigrations(ctx)
}

// checkRiverAvailable ensures the River CLI is available.
func checkRiverAvailable(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "river", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("river not found in PATH")
	}
	return nil
}
