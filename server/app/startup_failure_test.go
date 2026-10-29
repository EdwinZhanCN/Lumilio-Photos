package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"server/config"
	"server/internal/db"
)

// The facade classifies the fatal startup causes a reinstall or an upgrade
// meets, through the wrapping Run and the config loader add, and every
// code has operator guidance that never asks for media to be deleted.
func TestClassifyStartupFailureNamesEveryCompatibilityCause(t *testing.T) {
	cases := []struct {
		err  error
		code StartupFailureCode
	}{
		{fmt.Errorf("open SQLite database: %w", fmt.Errorf("%w: move it aside", db.ErrPreReleaseCatalog)), StartupFailureCatalogLineageUnsupported},
		{fmt.Errorf("run database migrations: %w", fmt.Errorf("%w: schema 9", db.ErrCatalogNewer)), StartupFailureCatalogNewer},
		{fmt.Errorf("runtime manifest /x: %w", fmt.Errorf("%w: schema_version = 9", config.ErrManifestIncompatible)), StartupFailureConfigurationIncompatible},
		{fmt.Errorf("runtime manifest /x: %w", config.ErrManifestUpgradeRequired), StartupFailureConfigurationUpgradeRequired},
		{fmt.Errorf("validate default storage location migration: %w", ErrDefaultStorageMigrationRejected), StartupFailureDefaultStorageMigrationRejected},
		{fmt.Errorf("%w: mkdir denied", ErrDefaultStorageUnavailable), StartupFailureDefaultStorageUnavailable},
		{errors.New("initialize libvips: boom"), StartupFailureUnclassified},
	}
	for _, tc := range cases {
		failure := ClassifyStartupFailure(tc.err)
		if failure == nil || failure.Code != tc.code || !errors.Is(failure, tc.err) {
			t.Fatalf("classify(%v) = %+v, want %s", tc.err, failure, tc.code)
		}
		if len(failure.Guidance()) == 0 {
			t.Fatalf("%s has no guidance", tc.code)
		}
		description := DescribeStartupFailure(tc.err)
		if !strings.Contains(description, string(tc.code)) || !strings.Contains(description, tc.err.Error()) {
			t.Fatalf("description for %s = %q", tc.code, description)
		}
		for _, step := range failure.Guidance() {
			if strings.Contains(strings.ToLower(step), "delete") {
				t.Fatalf("%s guidance suggests deleting: %q", tc.code, step)
			}
		}
	}
	if ClassifyStartupFailure(nil) != nil || DescribeStartupFailure(nil) != "" {
		t.Fatal("nil error classified")
	}
	// An already classified error keeps its code through further wrapping.
	wrapped := fmt.Errorf("desktop generation: %w", ClassifyStartupFailure(fmt.Errorf("x: %w", db.ErrCatalogNewer)))
	if ClassifyStartupFailure(wrapped).Code != StartupFailureCatalogNewer {
		t.Fatal("classification lost through wrapping")
	}
}
