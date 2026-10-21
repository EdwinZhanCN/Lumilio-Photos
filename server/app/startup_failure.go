package app

import (
	"errors"
	"fmt"
	"strings"

	"server/config"
	"server/internal/db"
)

// StartupFailureCode classifies why the Server could not start. The Server
// owns this vocabulary so the CLI host and the Desktop host present the same
// cause and the same way out, and neither guesses from error text.
type StartupFailureCode string

const (
	// StartupFailureCatalogLineageUnsupported: the catalog was written by a
	// pre-release build; pre-release data is not migrated.
	StartupFailureCatalogLineageUnsupported StartupFailureCode = "catalog_lineage_unsupported"
	// StartupFailureCatalogNewer: a newer Lumilio Photos wrote the catalog.
	StartupFailureCatalogNewer StartupFailureCode = "catalog_newer"
	// StartupFailureConfigurationIncompatible: the runtime manifest was
	// written by a newer or a pre-release build and cannot be read.
	StartupFailureConfigurationIncompatible StartupFailureCode = "configuration_incompatible"
	// StartupFailureConfigurationUpgradeRequired: an older manifest that the
	// upgrade command brings forward.
	StartupFailureConfigurationUpgradeRequired StartupFailureCode = "configuration_upgrade_required"
	// StartupFailureDefaultStorageMigrationRejected: storage.path differs
	// from the registered Default Storage Location and the new path does not
	// prove the registered identity (and setup is complete, so it cannot be
	// a fresh default).
	StartupFailureDefaultStorageMigrationRejected StartupFailureCode = "default_storage_migration_rejected"
	// StartupFailureDefaultStorageUnavailable: a fresh install could not
	// create or claim its Default Storage Location at all.
	StartupFailureDefaultStorageUnavailable StartupFailureCode = "default_storage_unavailable"
	// StartupFailureUnclassified: any other fatal startup condition.
	StartupFailureUnclassified StartupFailureCode = "startup_failed"
)

var (
	// ErrDefaultStorageMigrationRejected and ErrDefaultStorageUnavailable
	// are the app's own fatal storage-startup conditions.
	ErrDefaultStorageMigrationRejected = errors.New("default Storage Location migration rejected")
	ErrDefaultStorageUnavailable       = errors.New("default Storage Location unavailable")
)

// StartupFailure is a classified fatal startup error.
type StartupFailure struct {
	Code  StartupFailureCode
	Cause error
}

func (f *StartupFailure) Error() string {
	if f == nil || f.Cause == nil {
		return string(StartupFailureUnclassified)
	}
	return f.Cause.Error()
}

func (f *StartupFailure) Unwrap() error {
	if f == nil {
		return nil
	}
	return f.Cause
}

// ClassifyStartupFailure maps an error returned by [Run] or by
// config.LoadAppConfig to its code. A nil error classifies to nil.
func ClassifyStartupFailure(err error) *StartupFailure {
	if err == nil {
		return nil
	}
	var classified *StartupFailure
	if errors.As(err, &classified) {
		return classified
	}
	code := StartupFailureUnclassified
	switch {
	case errors.Is(err, db.ErrPreReleaseCatalog):
		code = StartupFailureCatalogLineageUnsupported
	case errors.Is(err, db.ErrCatalogNewer):
		code = StartupFailureCatalogNewer
	case errors.Is(err, config.ErrManifestIncompatible):
		code = StartupFailureConfigurationIncompatible
	case errors.Is(err, config.ErrManifestUpgradeRequired):
		code = StartupFailureConfigurationUpgradeRequired
	case errors.Is(err, ErrDefaultStorageMigrationRejected):
		code = StartupFailureDefaultStorageMigrationRejected
	case errors.Is(err, ErrDefaultStorageUnavailable):
		code = StartupFailureDefaultStorageUnavailable
	}
	return &StartupFailure{Code: code, Cause: err}
}

// Guidance is the operator's way out of this failure, one step per line.
// It names what to preserve and never suggests deleting media.
func (f *StartupFailure) Guidance() []string {
	if f == nil {
		return nil
	}
	switch f.Code {
	case StartupFailureCatalogLineageUnsupported:
		return []string{
			"This catalog was written by a pre-release build; its data is not migrated.",
			"Stop the server, move the application-state directory (the catalog, secrets and backups) aside, and start again to create a new catalog.",
			"Your media folders and their .lumilioroot/.lumiliorepo markers are untouched; setup offers to adopt the Primary Repository they hold.",
		}
	case StartupFailureCatalogNewer:
		return []string{
			"This catalog was written by a newer Lumilio Photos and is not read by this build.",
			"Install the newer version again, or restore the pre-upgrade snapshot from the backups folder into this build.",
		}
	case StartupFailureConfigurationIncompatible:
		return []string{
			"The runtime manifest was written by a newer or a pre-release build.",
			"Create a new server.toml from the current examples (the Docker default profile is built into the image; Desktop writes its own), keeping storage.path pointed at your media.",
		}
	case StartupFailureConfigurationUpgradeRequired:
		return []string{
			"The runtime manifest is from an older build.",
			"Run `server config upgrade --config <path>`; the previous file is kept as server.toml.bak.",
		}
	case StartupFailureDefaultStorageMigrationRejected:
		return []string{
			"storage.path points at a folder that does not carry the registered Default Storage Location and its Primary Repository.",
			"Point storage.path back at the registered folder, or at the folder the whole Default Storage Location was moved to (its .lumilioroot and primary/.lumiliorepo identities must match).",
			"A different, fresh Default Storage Location is only possible before the Primary Repository exists.",
		}
	case StartupFailureDefaultStorageUnavailable:
		return []string{
			"The Default Storage Location at storage.path could not be created or claimed.",
			"Check that the folder exists or can be created, is writable by the server, and is not held by another running instance.",
		}
	default:
		return []string{"See the error above and the server log for the failing step."}
	}
}

// DescribeStartupFailure renders the cause, its code and the operator steps
// as the text a host prints when the Server cannot start.
func DescribeStartupFailure(err error) string {
	failure := ClassifyStartupFailure(err)
	if failure == nil {
		return ""
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "server exited with error: %v\n", failure.Cause)
	fmt.Fprintf(&builder, "startup failure: %s\n", failure.Code)
	for _, step := range failure.Guidance() {
		fmt.Fprintf(&builder, "  - %s\n", step)
	}
	return builder.String()
}
