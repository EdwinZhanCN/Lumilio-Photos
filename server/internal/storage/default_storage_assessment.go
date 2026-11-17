package storage

import (
	"context"
	"database/sql"
	"errors"

	"server/internal/storage/marker"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
)

// DefaultStorageState is what the configured Default Storage Location looks
// like to this catalog right now. Setup and the Storage page present it to
// the authenticated administrator; nothing here is written.
type DefaultStorageState string

const (
	// DefaultStorageReady: registered, present, compatible marker with the
	// registered identity.
	DefaultStorageReady DefaultStorageState = "ready"
	// DefaultStorageOffline: registered, but the path is absent.
	DefaultStorageOffline DefaultStorageState = "offline"
	// DefaultStoragePathChanged: the manifest names a different path than
	// the registration and startup did not accept it as a move.
	DefaultStoragePathChanged DefaultStorageState = "path_changed"
	// DefaultStorageMarkerMissing: registered, present, no .lumilioroot.
	DefaultStorageMarkerMissing DefaultStorageState = "marker_missing"
	// DefaultStorageMarkerInvalid: a .lumilioroot this build cannot parse.
	DefaultStorageMarkerInvalid DefaultStorageState = "marker_invalid"
	// DefaultStorageMarkerUnsupported: a .lumilioroot a newer build wrote.
	DefaultStorageMarkerUnsupported DefaultStorageState = "marker_unsupported"
	// DefaultStorageIdentityMismatch: the marker names another Location.
	DefaultStorageIdentityMismatch DefaultStorageState = "identity_mismatch"
	// DefaultStorageRepositoryMarker: the configured path is a Repository.
	DefaultStorageRepositoryMarker DefaultStorageState = "repository_marker"
	// DefaultStorageUnregistered: a fresh catalog whose startup registered
	// nothing at the configured path (restricted recovery runtime).
	DefaultStorageUnregistered DefaultStorageState = "unregistered"
	// DefaultStorageUnavailable: the path cannot be observed or is denied.
	DefaultStorageUnavailable DefaultStorageState = "unavailable"
)

// DefaultStorageAssessment is the read-only review of storage.path.
type DefaultStorageAssessment struct {
	ConfiguredPath string
	Registered     bool
	RegisteredPath string
	State          DefaultStorageState
	MarkerState    marker.State
	Detail         string
}

// Usable reports whether the Primary Repository target below this Location
// can be reviewed at all.
func (a DefaultStorageAssessment) Usable() bool { return a.State == DefaultStorageReady }

// AssessDefaultStorageLocation compares the configured path with the
// registration and the markers on disk through the installed observer.
func (rm *DefaultRepositoryManager) AssessDefaultStorageLocation(ctx context.Context, configuredPath string) (DefaultStorageAssessment, error) {
	result := DefaultStorageAssessment{ConfiguredPath: configuredPath}
	registered, err := rm.readerQueries.GetDefaultStorageLocation(ctx)
	switch {
	case err == nil:
		result.Registered, result.RegisteredPath = true, registered.Path
	case errors.Is(err, sql.ErrNoRows):
	default:
		return result, err
	}
	cleanPath, err := CanonicalizeRepositoryPath(configuredPath)
	if err != nil {
		result.State, result.Detail = DefaultStorageUnavailable, err.Error()
		return result, nil
	}
	observer := rm.observerOrDefault()
	access := ReadAccess(observer.Stat(cleanPath))
	switch access.State {
	case AccessAbsent:
		if result.Registered {
			result.State = DefaultStorageOffline
		} else {
			result.State = DefaultStorageUnregistered
		}
		return result, nil
	case AccessPresent:
	default:
		result.State = DefaultStorageUnavailable
		if access.Err != nil {
			result.Detail = access.Err.Error()
		}
		return result, nil
	}
	if result.Registered && registered.Path != cleanPath {
		result.State = DefaultStoragePathChanged
		return result, nil
	}
	reading := rootcfg.ReadMarker(observer, cleanPath)
	result.MarkerState = reading.State
	if reading.Err != nil {
		result.Detail = reading.Err.Error()
	}
	switch reading.State {
	case marker.Valid:
		switch {
		case repocfg.ReadMarker(observer, cleanPath).State == marker.Valid:
			result.State = DefaultStorageRepositoryMarker
		case result.Registered && reading.UUID != registered.StorageLocationID.String():
			result.State = DefaultStorageIdentityMismatch
		case result.Registered:
			result.State = DefaultStorageReady
		default:
			result.State = DefaultStorageUnregistered
		}
	case marker.Absent:
		if repocfg.ReadMarker(observer, cleanPath).State == marker.Valid {
			result.State = DefaultStorageRepositoryMarker
		} else if result.Registered {
			result.State = DefaultStorageMarkerMissing
		} else {
			result.State = DefaultStorageUnregistered
		}
	case marker.Corrupt:
		result.State = DefaultStorageMarkerInvalid
	case marker.UnsupportedVersion:
		result.State = DefaultStorageMarkerUnsupported
	default:
		result.State = DefaultStorageUnavailable
	}
	return result, nil
}
