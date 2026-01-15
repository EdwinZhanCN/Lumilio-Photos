package db

import (
	"context"
	"fmt"
	"log"
	"server/config"
	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the database connection and provides access to queries
type DB struct {
	Pool    *pgxpool.Pool
	Queries *repo.Queries
}

// New creates a new database connection with the given configuration
func New(cfg config.DatabaseConfig) (*DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSL,
	)

	pool, err := pgxpool.New(context.Background(), dsn)
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
