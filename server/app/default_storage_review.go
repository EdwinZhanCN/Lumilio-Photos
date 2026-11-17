package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"server/internal/storage"
	"server/internal/storage/marker"
	"server/internal/storage/repocfg"
	"server/internal/storage/rootcfg"
)

// The Desktop host never parses a portable marker itself. When a person
// changes the Default Storage Location in the Desktop runtime configuration,
// the candidate folder is read here, with the Server's typed marker readers,
// and the Desktop only learns the verdict. The authoritative comparison with
// the catalog still happens when the Server restarts on the new path; this
// review refuses what can be refused before any restart.
var (
	// ErrDefaultStorageCandidateIsRepository: the chosen folder is itself a
	// Repository (it carries .lumiliorepo); the Default Storage Location is
	// its parent or another folder.
	ErrDefaultStorageCandidateIsRepository = errors.New("selected default storage location is a Repository folder; choose its parent or another folder")
	// ErrDefaultStorageMarkerUnreadable: a marker under the candidate exists
	// but this build cannot adopt it (corrupt, unsupported version, or
	// unreadable).
	ErrDefaultStorageMarkerUnreadable = errors.New("selected default storage location has a marker this build cannot adopt")
	// ErrDefaultStorageIdentityDiffers: the candidate carries a different
	// portable identity than the current Default Storage Location, which is
	// still online to prove it.
	ErrDefaultStorageIdentityDiffers = errors.New("selected default storage location has a different identity")
)

// DefaultStorageReview is the verdict on one candidate folder.
type DefaultStorageReview struct {
	// StorageLocationID is the candidate's .lumilioroot identity, or empty
	// for a fresh folder.
	StorageLocationID string
	// PrimaryRepositoryID is the identity of primary/.lumiliorepo under the
	// candidate, or empty when the candidate has no Primary Repository yet.
	PrimaryRepositoryID string
	// Fresh reports a candidate without a Primary Repository: the Server
	// accepts it only before the Primary Repository exists and refuses it
	// afterwards, which rolls the intent back.
	Fresh bool
}

// ReviewDefaultStorageMove reads the markers of a candidate Default Storage
// Location and, while the current one is still online, proves that both name
// the same portable root and Primary Repository. It reads only; nothing is
// created or probed.
func ReviewDefaultStorageMove(currentPath, candidatePath string) (DefaultStorageReview, error) {
	reader := storage.OSStorageObserver{}
	candidate := filepath.Clean(strings.TrimSpace(candidatePath))
	if repocfg.ReadMarker(reader, candidate).State != marker.Absent {
		return DefaultStorageReview{}, ErrDefaultStorageCandidateIsRepository
	}
	review := DefaultStorageReview{}
	root := rootcfg.ReadMarker(reader, candidate)
	switch root.State {
	case marker.Valid:
		review.StorageLocationID = root.UUID
	case marker.Absent:
	default:
		return DefaultStorageReview{}, fmt.Errorf("%w: .lumilioroot is %s: %v", ErrDefaultStorageMarkerUnreadable, root.State, root.Err)
	}
	primary := repocfg.ReadMarker(reader, filepath.Join(candidate, "primary"))
	switch primary.State {
	case marker.Valid:
		review.PrimaryRepositoryID = primary.UUID
	case marker.Absent:
		review.Fresh = true
		return review, nil
	default:
		return DefaultStorageReview{}, fmt.Errorf("%w: primary/.lumiliorepo is %s: %v", ErrDefaultStorageMarkerUnreadable, primary.State, primary.Err)
	}
	if root.State != marker.Valid {
		return DefaultStorageReview{}, fmt.Errorf("%w: no valid .lumilioroot marker beside the Primary Repository", ErrDefaultStorageMarkerUnreadable)
	}
	// If the old path is still online, prove immediately that the selected
	// directory is the same portable root and Primary Repository. A true
	// move normally leaves the old path offline; the Server then performs
	// the authoritative comparison against the catalog on restart.
	current := filepath.Clean(strings.TrimSpace(currentPath))
	if current == "" || current == candidate {
		return review, nil
	}
	if currentRoot := rootcfg.ReadMarker(reader, current); currentRoot.State == marker.Valid {
		if currentRoot.UUID != review.StorageLocationID {
			return DefaultStorageReview{}, fmt.Errorf("%w: a different .lumilioroot identity", ErrDefaultStorageIdentityDiffers)
		}
		currentPrimary := repocfg.ReadMarker(reader, filepath.Join(current, "primary"))
		if currentPrimary.State != marker.Valid {
			return DefaultStorageReview{}, fmt.Errorf("%w: current default storage location primary marker is %s", ErrDefaultStorageMarkerUnreadable, currentPrimary.State)
		}
		if currentPrimary.UUID != review.PrimaryRepositoryID {
			return DefaultStorageReview{}, fmt.Errorf("%w: a different primary repository identity", ErrDefaultStorageIdentityDiffers)
		}
	}
	return review, nil
}
