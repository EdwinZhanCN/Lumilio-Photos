package service

import (
	"context"

	"fx/catalog"
)

// Counts runs two hand-written statements.
func Counts(ctx context.Context, db catalog.Statements, table string) (int, int, error) {
	var a, b int
	if err := db.QueryRowContext(ctx, `SELECT 1`).Scan(&a); err != nil {
		return 0, 0, err
	}
	err := db.QueryRowContext(ctx, `SELECT 2 FROM `+table).Scan(&b)
	return a, b, err
}
