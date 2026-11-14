package lifecycle

import (
	"context"
	"database/sql"
)

// Purge removes assets through a table of statements chosen at run time.
func Purge(ctx context.Context, tx *sql.Tx) error {
	statements := []string{`DELETE FROM assets WHERE entry_count = 0`}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
