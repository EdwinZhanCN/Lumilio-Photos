package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"server/config"
	"server/internal/db/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the database connection and provides access to queries
type DB struct {
	Pool    *pgxpool.Pool
	Queries *repo.Queries
}

// New creates a new database connection with the given configuration
func New(cfg config.DatabaseConfig) (*DB, error) {
	// A Unix-socket directory host (desktop runtime) cannot be expressed in a
	// URL-form DSN, so use the keyword/value form there. TCP hosts keep the
	// existing URL form unchanged.
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSL,
	)
	if isSocketHost(cfg.Host) {
		dsn = socketDSN(cfg, nil)
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection config: %w", err)
	}

	// Resolve the rotated credential on every new connection rather than baking
	// the bootstrap password into the pool. First-run setup rotates the database
	// role password (ALTER USER) and writes the new secret to disk; without this
	// hook the pool would keep opening connections with the stale bootstrap
	// password and fail SASL auth (notably River's producer fetching jobs).
	secretPath := config.ResolveDBPasswordFilePath(cfg)
	poolCfg.BeforeConnect = func(_ context.Context, connConfig *pgx.ConnConfig) error {
		if password := readRotatedPassword(secretPath); password != "" {
			connConfig.Password = password
		}
		return nil
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	queries := repo.New(pool)

	log.Printf("📊 Database connection successful: %s:%s/%s", cfg.Host, cfg.Port, cfg.DBName)

	return &DB{
		Pool:    pool,
		Queries: queries,
	}, nil
}

// readRotatedPassword returns the rotated database password persisted on disk,
// or an empty string when the secret file is missing or empty. A missing file is
// expected on first boot, where the bootstrap password from the DSN still applies.
func readRotatedPassword(secretPath string) string {
	data, err := os.ReadFile(secretPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Close closes the database connection
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		log.Println("Database connection closed")
	}
}

// WithTx executes a function within a database transaction
func (db *DB) WithTx(ctx context.Context, fn func(*repo.Queries) error) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := db.Queries.WithTx(tx)
	if err := fn(queries); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
