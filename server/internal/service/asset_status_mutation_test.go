package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/catalogtx"

	"github.com/google/uuid"
)

func TestForegroundAssetMutationUsesNamedWriterOperation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	database, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(t.TempDir(), "app-state", "library.sqlite3"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(context.Background())
	if err := database.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	service := &assetService{queries: database.Queries}
	assetID := uuid.New()
	mutations := []func() error{
		func() error { return service.UpdateAssetRating(ctx, assetID, 3) },
		func() error { return service.UpdateAssetLike(ctx, assetID, true) },
		func() error { return service.UpdateAssetRatingAndLike(ctx, assetID, 4, true) },
		func() error { return service.UpdateAssetDescription(ctx, assetID, "foreground edit") },
	}
	for _, mutate := range mutations {
		if err := mutate(); err != nil {
			t.Fatal(err)
		}
	}
	report, ok := database.TransactionReport().Operation(catalogtx.OperationAssetStatusMutate)
	want := int64(len(mutations))
	if !ok || report.Outcomes.Committed != want || report.Admission.Count != want {
		t.Fatalf("asset status mutation report = %+v present=%t, want %d named commits", report, ok, want)
	}
}
