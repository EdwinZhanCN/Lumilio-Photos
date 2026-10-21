package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"server/app"
	"server/config"
)

// The Server classifies why a generation could not start; the Desktop only
// maps the class to its control-plane code and shows the Server's guidance.
// Unclassified errors keep their meaning for the ordinary lifecycle paths.
func TestClassifyGenerationFailureCarriesTheServersVerdict(t *testing.T) {
	// The catalog causes come from the Server's internal db package, so the
	// test hands them over already classified, the way app.Run returns them.
	classifiedCatalog := func(code app.StartupFailureCode) error {
		return fmt.Errorf("open SQLite database: %w", &app.StartupFailure{Code: code, Cause: errors.New("catalog cause")})
	}
	cases := []struct {
		err  error
		code dto.ErrorCode
	}{
		{classifiedCatalog(app.StartupFailureCatalogLineageUnsupported), dto.ErrorCatalogIncompatible},
		{classifiedCatalog(app.StartupFailureCatalogNewer), dto.ErrorCatalogIncompatible},
		{fmt.Errorf("runtime manifest: %w", config.ErrManifestIncompatible), dto.ErrorConfigurationIncompatible},
		{fmt.Errorf("runtime manifest: %w", config.ErrManifestUpgradeRequired), dto.ErrorConfigurationIncompatible},
		{fmt.Errorf("x: %w", app.ErrDefaultStorageMigrationRejected), dto.ErrorStorageMigrationRejected},
		{fmt.Errorf("x: %w", app.ErrDefaultStorageUnavailable), dto.ErrorStorageUnavailable},
	}
	for _, tc := range cases {
		classified := classifyGenerationFailure(tc.err)
		if operation.ErrorCodeOf(classified) != tc.code {
			t.Fatalf("classify(%v) code = %q, want %q", tc.err, operation.ErrorCodeOf(classified), tc.code)
		}
		if !strings.Contains(classified.Error(), "cause") && !strings.Contains(classified.Error(), tc.err.Error()) {
			t.Fatalf("classified message dropped the cause: %q", classified.Error())
		}
	}
	plain := errors.New("initialize libvips: boom")
	if classified := classifyGenerationFailure(plain); classified != plain {
		t.Fatalf("unclassified error was rewritten: %v", classified)
	}
	if classifyGenerationFailure(nil) != nil {
		t.Fatal("nil error classified")
	}
}
