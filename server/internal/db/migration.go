package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"server/config"
	migrations "server/migrations"

	"github.com/golang-migrate/migrate/v4"
	mgpg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// MigrationConfig holds configuration for running migrations.
type MigrationConfig struct {
	DatabaseConfig config.DatabaseConfig
}

// NewMigrationConfig returns a MigrationConfig with sensible defaults.
func NewMigrationConfig(dbConfig config.DatabaseConfig) *MigrationConfig {
	return &MigrationConfig{
		DatabaseConfig: dbConfig,
	}
}

// buildURL constructs a Postgres connection string with explicit ssl options
// and search_path. For a Unix-socket directory host (desktop runtime) it uses
// the keyword/value form, which—unlike a URL—can carry a filesystem path in the
// host position. TCP hosts keep the existing URL form unchanged.
func (m *MigrationConfig) buildURL() string {
	if isSocketHost(m.DatabaseConfig.Host) {
		return socketDSN(m.DatabaseConfig, map[string]string{"search_path": "public"})
	}
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

// migrateUp applies all pending "up" migrations from the embedded migration set.
// Using the embedded files (via the iofs source) means migrations do not depend
// on the working directory or on the .sql files existing on disk, which is what
// lets the desktop bundle run them.
func (m *MigrationConfig) migrateUp(ctx context.Context) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	defer source.Close()

	// Use pgx stdlib with database/sql so postgresql:// URLs (and keyword/value
	// DSNs) work.
	db, err := sql.Open("pgx", m.buildURL())
	if err != nil {
		return fmt.Errorf("sql open (pgx): %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}

	pgDriver, err := mgpg.WithInstance(db, &mgpg.Config{})
	if err != nil {
		return fmt.Errorf("postgres driver instance: %w", err)
	}

	log.Printf("🚀 Applying DB migrations (embedded)")
	migrator, err := migrate.NewWithInstance("iofs", source, "postgres", pgDriver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer func() {
		if _, derr := migrator.Close(); derr != nil && !strings.Contains(derr.Error(), "no such file or directory") {
			log.Printf("migration close warning: %v", derr)
		}
	}()

	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	} else if err == migrate.ErrNoChange {
		log.Printf("ℹ️ No migration needed. Database schema is up-to-date.")
	} else {
		log.Printf("✅ Database migrations applied successfully.")
	}
	return nil
}

// runRiverMigrations applies River queue schema migrations through River's Go API.
func (m *MigrationConfig) runRiverMigrations(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, m.buildURL())
	if err != nil {
		return fmt.Errorf("river migration pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("river migration db ping: %w", err)
	}

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), &rivermigrate.Config{
		Schema: "public",
	})
	if err != nil {
		return fmt.Errorf("river migrator init: %w", err)
	}

	log.Printf("🌊 Running River migrations...")
	result, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("river migrations failed: %w", err)
	}
	if len(result.Versions) == 0 {
		log.Printf("ℹ️ River schema is up-to-date.")
		return nil
	}

	for _, version := range result.Versions {
		log.Printf("River migration applied: version=%d name=%s duration=%s", version.Version, version.Name, version.Duration)
	}
	log.Printf("✅ River migrations applied successfully.")
	return nil
}

// RunMigrations applies DB migrations using golang-migrate and then applies River migrations.
func (m *MigrationConfig) RunMigrations(ctx context.Context) error {
	// Apply DB migrations (up).
	if err := m.migrateUp(ctx); err != nil {
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
