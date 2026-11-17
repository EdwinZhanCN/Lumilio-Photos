package store

import (
	"context"
	"database/sql"
	"io"
	"os"
	"time"

	"fx/catalogtx"
	"fx/repo"
)

// cleaner is a port with an implementation that touches the disk.
type cleaner interface{ Clean() }

type diskCleaner struct{}

// Clean removes a file.
func (diskCleaner) Clean() {}

func helper() {
	if _, err := os.Stat("x"); os.IsNotExist(err) {
		return
	}
}

func applyTx(ctx context.Context, tx *sql.Tx, q *repo.Queries, c cleaner) error {

	c.Clean()
	_, err := q.CountAssets(ctx)
	return err
}

// Run opens a transaction around a closure.
func Run(ctx context.Context, w *catalogtx.Writer) error {
	return w.Transact(ctx, func(tx *sql.Tx) error {
		_ = time.Now()
		return nil
	})
}

// Region does work between Begin and Commit, and after.
func Region(ctx context.Context, w *catalogtx.Writer) error {
	tx, err := w.BeginTx(ctx)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	_, _ = os.ReadFile("after-commit")
	return nil
}

// fileWriter is a module io.Writer that touches the disk. A call through
// io.Writer is not resolved to it: library interfaces are too wide.
type fileWriter struct{}

func (fileWriter) Write(b []byte) (int, error) { return len(b), os.WriteFile("x", b, 0o600) }

// Rollback ends the transaction on a failure path; the audit after it runs
// outside the transaction.
func Rollback(ctx context.Context, w *catalogtx.Writer, fail bool) error {
	tx, err := w.BeginTx(ctx)
	if err != nil {
		return err
	}
	if fail {
		_ = tx.Rollback()
		_, _ = os.ReadFile("audit")
		return nil
	}
	return tx.Commit()
}

// digest hashes in memory through io.Writer inside a transaction.
func digest(_ *sql.Tx, w io.Writer) { _, _ = w.Write([]byte("token")) }

var _ = []any{applyTx, diskCleaner{}, digest, fileWriter{}}
