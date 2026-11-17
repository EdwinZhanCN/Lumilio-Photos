package service

import (
	"context"
	"database/sql"
)

type beginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

// Raw opens a transaction itself.
func Raw(ctx context.Context, db *sql.DB) { _, _ = db.BeginTx(ctx, nil) }

// Hidden opens one through a local interface.
func Hidden(ctx context.Context, b beginner) { _, _ = b.BeginTx(ctx, nil) }

// Value takes the method as a value.
func Value(db *sql.DB) any { return db.ExecContext }
