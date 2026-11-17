package storage

import (
	"errors"
	"io/fs"

	"server/internal/storage/marker"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
)

// OriginalIdentityState is what a read-only look at a registered path says
// about the identity registered there. Reconnect is admitted only when the
// original is positively unavailable: a denied or otherwise failed read is
// not evidence that the tree is gone, so it never authorizes a reconnect
// (matrix row "Original permission denied/unknown").
type OriginalIdentityState string

const (
	// OriginalOnline: a readable marker at the registered path carries the
	// registered identity. There is nothing to reconnect.
	OriginalOnline OriginalIdentityState = "online"
	// OriginalAbsent: the path or its marker is positively absent.
	OriginalAbsent OriginalIdentityState = "absent"
	// OriginalIdentityDiffers: something readable is at the registered path
	// but it does not carry the registered identity (another identity, a
	// corrupt or newer marker, or an unmarked directory). The registration
	// is positively unavailable there; the bytes are never touched.
	OriginalIdentityDiffers OriginalIdentityState = "identity_differs"
	// OriginalInaccessible: the registered path could not be read (permission
	// denied or an unknown failure). No conclusion about the original is
	// drawn from it.
	OriginalInaccessible OriginalIdentityState = "inaccessible"
)

// PositivelyUnavailable reports whether the state proves the registered
// identity is not at its registered path.
func (s OriginalIdentityState) PositivelyUnavailable() bool {
	return s == OriginalAbsent || s == OriginalIdentityDiffers
}

// repositoryOriginalState reads the Repository marker at a registered path
// through the read-only observer and classifies what is there.
func (rm *DefaultRepositoryManager) repositoryOriginalState(path, repositoryID string) OriginalIdentityState {
	observer := rm.observerOrDefault()
	reading := repocfg.ReadMarker(observer, path)
	return classifyOriginal(observer, path, reading.State, reading.UUID, repositoryID)
}

// storageLocationOriginalState is repositoryOriginalState for a Storage
// Location's root marker.
func (rm *DefaultRepositoryManager) storageLocationOriginalState(path, storageLocationID string) OriginalIdentityState {
	observer := rm.observerOrDefault()
	reading := rootcfg.ReadMarker(observer, path)
	return classifyOriginal(observer, path, reading.State, reading.UUID, storageLocationID)
}

func classifyOriginal(observer StorageObserver, path string, state marker.State, uuid, registeredID string) OriginalIdentityState {
	switch state {
	case marker.Valid:
		if uuid == registeredID {
			return OriginalOnline
		}
		return OriginalIdentityDiffers
	case marker.Corrupt, marker.UnsupportedVersion:
		return OriginalIdentityDiffers
	case marker.PermissionDenied, marker.Unknown:
		return OriginalInaccessible
	case marker.Absent:
		// The marker is gone; the directory decides whether the tree itself
		// is gone or merely unmarked. Only a positive answer counts.
		info, err := observer.Stat(path)
		switch {
		case err == nil && info != nil && info.IsDir():
			return OriginalIdentityDiffers
		case err == nil:
			return OriginalIdentityDiffers
		case errors.Is(err, fs.ErrNotExist):
			return OriginalAbsent
		default:
			return OriginalInaccessible
		}
	default:
		return OriginalInaccessible
	}
}
