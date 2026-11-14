package catalogtx

import (
	"context"
	"database/sql"
)

// Tx is a named transaction.
type Tx struct{ raw *sql.Tx }

// Commit ends it.
func (t *Tx) Commit() error { return t.raw.Commit() }

// Rollback abandons it.
func (t *Tx) Rollback() error { return t.raw.Rollback() }

// Writer is the single writer.
type Writer struct{ db *sql.DB }

// BeginTx opens a named transaction.
func (w *Writer) BeginTx(ctx context.Context) (*Tx, error) {
	raw, err := w.db.BeginTx(ctx, nil)
	return &Tx{raw: raw}, err
}

// Transact runs body inside a transaction.
func (w *Writer) Transact(ctx context.Context, body func(*sql.Tx) error) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := body(tx); err != nil {
		return err
	}
	return tx.Commit()
}
