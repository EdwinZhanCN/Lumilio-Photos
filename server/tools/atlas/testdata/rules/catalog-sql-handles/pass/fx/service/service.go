package service

import (
	"context"

	"fx/catalog"
)

// Count reads through the narrow handle.
func Count(ctx context.Context, reader *catalog.Reader) (int, error) {
	return reader.Count(ctx)
}
