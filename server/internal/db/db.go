package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"server/config"
	"server/internal/db/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
	secretPath := strings.TrimSpace(cfg.RotatedPasswordFile)
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

// SelfHealPassword checks if the current password fails authentication but the
// bootstrap password succeeds. If so, it updates the user's password in the
// database to match the one in the secret file.
func SelfHealPassword(ctx context.Context, cfg *config.DatabaseConfig) error {
	if cfg.BootstrapPassword == "" || cfg.BootstrapPassword == cfg.Password {
		return nil
	}

	// 1. Try to connect with the current configured password (from the file).
	dsnCur := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSL,
	)
	if isSocketHost(cfg.Host) {
		dsnCur = socketDSN(*cfg, nil)
	}

	connCur, err := pgx.Connect(ctx, dsnCur)
	if err == nil {
		// Connection succeeded with the file's password! No healing needed.
		connCur.Close(ctx)
		return nil
	}

	// Check if the connection failed specifically due to SASL/password authentication failure.
	var pgErr *pgconn.PgError
	isAuthFailure := false
	if errors.As(err, &pgErr) && pgErr.Code == "28P01" {
		isAuthFailure = true
	} else if strings.Contains(err.Error(), "password authentication failed") || strings.Contains(err.Error(), "SASL auth") {
		isAuthFailure = true
	}

	if !isAuthFailure {
		// It's a different error (e.g. connection refused, network timeout). Do not try to heal.
		return nil
	}

	// 2. Try to connect with the bootstrap password.
	dsnBootstrap := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User,
		cfg.BootstrapPassword,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSL,
	)
	if isSocketHost(cfg.Host) {
		tempCfg := *cfg
		tempCfg.Password = cfg.BootstrapPassword
		dsnBootstrap = socketDSN(tempCfg, nil)
	}

	connBootstrap, err := pgx.Connect(ctx, dsnBootstrap)
	if err != nil {
		// The bootstrap password also failed. We can't do anything.
		return nil
	}
	defer connBootstrap.Close(ctx)

	// Connection succeeded using the bootstrap password!
	// This means the DB is using the default/bootstrap password, but the secret file has a rotated one.
	// Rotate the password in the DB to match the secret file (cfg.Password).
	log.Printf("⚠️ Database password authentication failed with secret file, but succeeded with bootstrap password.")
	log.Printf("🔄 Re-aligning database password with the existing secret file...")

	stmt := fmt.Sprintf("ALTER USER %s WITH PASSWORD %s",
		quoteSQLIdentifier(cfg.User),
		quoteSQLLiteral(cfg.Password),
	)
	if _, err := connBootstrap.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("failed to alter user password to match secret file: %w", err)
	}

	log.Printf("✅ Database password updated successfully to match secret file.")
	return nil
}

// quoteSQLIdentifier safely double-quotes a SQL identifier (e.g. a role name).
func quoteSQLIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// quoteSQLLiteral safely single-quotes a SQL string literal.
func quoteSQLLiteral(value string) string {
	return `'` + strings.ReplaceAll(value, `'`, `''`) + `'`
}
