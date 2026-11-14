package store

import (
	"context"
	"database/sql"
	"os"
	"time"

	"fx/catalogtx"
	"fx/repo"
)

// cleaner is a port with an implementation that touches the disk.
type cleaner interface{ Clean() }

type diskCleaner struct{}

// Clean removes a file.
func (diskCleaner) Clean() { _ = os.Remove("x") }

func helper() {
	if _, err := os.Stat("x"); os.IsNotExist(err) {
		return
	}
}

func applyTx(ctx context.Context, tx *sql.Tx, q *repo.Queries, c cleaner) error {
	helper()
	c.Clean()
	_, err := q.CountAssets(ctx)
	return err
}

// Run opens a transaction around a closure.
func Run(ctx context.Context, w *catalogtx.Writer) error {
	return w.Transact(ctx, func(tx *sql.Tx) error {
		time.Sleep(time.Millisecond)
		return nil
	})
}

// Region does work between Begin and Commit, and after.
func Region(ctx context.Context, w *catalogtx.Writer) error {
	tx, err := w.BeginTx(ctx)
	if err != nil {
		return err
	}
	_, _ = os.ReadFile("inside")
	if err := tx.Commit(); err != nil {
		return err
	}
	_, _ = os.ReadFile("after-commit")
	return nil
}

var _ = []any{applyTx, diskCleaner{}}
