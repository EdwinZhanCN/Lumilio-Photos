package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	migrations "server/migrations"

	"github.com/riverqueue/river/riverdriver/riversqlite"
	"github.com/riverqueue/river/rivermigrate"
)

const migrationTable = "lumilio_schema_migrations"

type embeddedMigration struct {
	version  int64
	name     string
	file     string
	sql      string
	checksum string
}

type appliedMigration struct {
	name     string
	checksum string
}

// Migrate applies Lumilio's embedded transactional migrations followed by
// River's official SQLite migrations against the same single-writer pool.
func (d *DB) Migrate(ctx context.Context) error {
	if err := migrateApplication(ctx, d.SQL); err != nil {
		return fmt.Errorf("migrate Lumilio schema: %w", err)
	}
	if err := migrateRiver(ctx, d.SQL); err != nil {
		return fmt.Errorf("migrate River schema: %w", err)
	}
	if err := d.Check(ctx); err != nil {
		return fmt.Errorf("post-migration integrity check: %w", err)
	}
	return nil
}

func migrateApplication(ctx context.Context, database *sql.DB) error {
	available, err := loadEmbeddedMigrations()
	if err != nil {
		return err
	}
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS lumilio_schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			checksum TEXT NOT NULL CHECK (length(checksum) = 64),
			applied_at INTEGER NOT NULL
		) STRICT
	`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}

	applied, err := readAppliedMigrations(ctx, database)
	if err != nil {
		return err
	}
	known := make(map[int64]embeddedMigration, len(available))
	for _, migration := range available {
		known[migration.version] = migration
	}
	for version, recorded := range applied {
		migration, ok := known[version]
		if !ok {
			return fmt.Errorf("catalog has unknown future migration %06d_%s", version, recorded.name)
		}
		if migration.name != recorded.name {
			return fmt.Errorf(
				"catalog migration %06d name = %q, embedded name = %q",
				version,
				recorded.name,
				migration.name,
			)
		}
		if migration.checksum != recorded.checksum {
			return fmt.Errorf(
				"catalog migration %06d_%s checksum = %q, embedded checksum = %q; historical migrations are immutable",
				version,
				migration.name,
				recorded.checksum,
				migration.checksum,
			)
		}
	}

	for _, migration := range available {
		if _, ok := applied[migration.version]; ok {
			continue
		}
		started := time.Now()
		if err := applyMigration(ctx, database, migration); err != nil {
			return err
		}
		log.Printf(
			"Lumilio migration applied: version=%06d name=%s duration=%s",
			migration.version,
			migration.name,
			time.Since(started),
		)
	}
	return nil
}

func loadEmbeddedMigrations() ([]embeddedMigration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	var result []embeddedMigration
	seen := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		filename := entry.Name()
		parts := strings.SplitN(strings.TrimSuffix(filename, ".up.sql"), "_", 2)
		if len(parts) != 2 || len(parts[0]) != 6 || parts[1] == "" {
			return nil, fmt.Errorf("invalid embedded migration filename %q", filename)
		}
		version, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("invalid embedded migration version in %q", filename)
		}
		if prior, ok := seen[version]; ok {
			return nil, fmt.Errorf("duplicate embedded migration version %06d: %s and %s", version, prior, filename)
		}
		body, err := migrations.FS.ReadFile(filename)
		if err != nil {
			return nil, fmt.Errorf("read embedded migration %s: %w", filename, err)
		}
		digest := sha256.Sum256(body)
		seen[version] = filename
		result = append(result, embeddedMigration{
			version:  version,
			name:     parts[1],
			file:     filename,
			sql:      string(body),
			checksum: hex.EncodeToString(digest[:]),
		})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no embedded up migrations")
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].version < result[j].version
	})
	return result, nil
}

func readAppliedMigrations(ctx context.Context, database *sql.DB) (map[int64]appliedMigration, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT version, name, checksum
		FROM lumilio_schema_migrations
		ORDER BY version
	`)
	if err != nil {
		return nil, fmt.Errorf("read migration ledger: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]appliedMigration)
	for rows.Next() {
		var version int64
		var migration appliedMigration
		if err := rows.Scan(&version, &migration.name, &migration.checksum); err != nil {
			return nil, fmt.Errorf("scan migration ledger: %w", err)
		}
		applied[version] = migration
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate migration ledger: %w", err)
	}
	return applied, nil
}

func applyMigration(ctx context.Context, database *sql.DB, migration embeddedMigration) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", migration.file, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, migration.sql); err != nil {
		return fmt.Errorf("execute migration %s: %w", migration.file, err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO lumilio_schema_migrations (version, name, checksum, applied_at)
		VALUES (?, ?, ?, ?)
	`, migration.version, migration.name, migration.checksum, time.Now().UTC().UnixMicro()); err != nil {
		return fmt.Errorf("record migration %s: %w", migration.file, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", migration.file, err)
	}
	return nil
}

func migrateRiver(ctx context.Context, database *sql.DB) error {
	migrator, err := rivermigrate.New(riversqlite.New(database), nil)
	if err != nil {
		return fmt.Errorf("initialize River migrator: %w", err)
	}
	result, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("apply River migrations: %w", err)
	}
	for _, version := range result.Versions {
		log.Printf(
			"River migration applied: version=%d name=%s duration=%s",
			version.Version,
			version.Name,
			version.Duration,
		)
	}
	return nil
}
