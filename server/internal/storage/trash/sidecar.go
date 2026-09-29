package trash

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"

	"server/internal/storage"
)

// SidecarFormat is the info sidecar format this build writes and reads. The
// sidecar lives in the user's folder beside the trashed file, so later
// builds keep reading format 1 and this build rejects a newer format instead
// of guessing at it.
const SidecarFormat = 1

// ErrSidecarFormat reports an info sidecar this build cannot read.
var ErrSidecarFormat = errors.New("unsupported trash info sidecar")

// Sidecar describes one trashed file well enough to rebuild the Trash view
// without the catalog: where the file came from, which Asset and content it
// was, and who deleted it when.
type Sidecar struct {
	Format        int       `json:"format"`
	TrashID       string    `json:"trash_id"`
	RepositoryID  string    `json:"repository_id"`
	OriginalPath  string    `json:"original_path"`
	AssetID       string    `json:"asset_id"`
	HashAlgorithm string    `json:"hash_algorithm"`
	ContentHash   string    `json:"content_hash"`
	Size          int64     `json:"size"`
	MtimeNs       int64     `json:"mtime_ns"`
	DeletedAt     time.Time `json:"deleted_at"`
	Actor         string    `json:"actor"`
}

// Name is the trashed file's name inside its trash directory.
func (s Sidecar) Name() string { return path.Base(s.OriginalPath) }

// Marshal encodes a format 1 sidecar.
func (s Sidecar) Marshal() ([]byte, error) {
	s.Format = SidecarFormat
	if err := s.validate(); err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// ParseSidecar reads an info sidecar. It accepts format 1, including fields a
// later build may add, and rejects any other format with a clear error.
func ParseSidecar(data []byte) (Sidecar, error) {
	var header struct {
		Format *int `json:"format"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Sidecar{}, fmt.Errorf("%w: %v", ErrSidecarFormat, err)
	}
	if header.Format == nil {
		return Sidecar{}, fmt.Errorf("%w: no format field", ErrSidecarFormat)
	}
	if *header.Format != SidecarFormat {
		return Sidecar{}, fmt.Errorf("%w: format %d, this build reads format %d", ErrSidecarFormat, *header.Format, SidecarFormat)
	}
	var sidecar Sidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return Sidecar{}, fmt.Errorf("%w: %v", ErrSidecarFormat, err)
	}
	if err := sidecar.validate(); err != nil {
		return Sidecar{}, err
	}
	return sidecar, nil
}

func (s Sidecar) validate() error {
	for name, value := range map[string]string{
		"trash_id": s.TrashID, "repository_id": s.RepositoryID, "asset_id": s.AssetID,
	} {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("%w: %s is not a UUID", ErrSidecarFormat, name)
		}
	}
	if _, err := storage.ParseUserMediaPath(s.OriginalPath); err != nil {
		return fmt.Errorf("%w: original_path %q is not a repository media path", ErrSidecarFormat, s.OriginalPath)
	}
	if s.HashAlgorithm == "" || s.ContentHash == "" || s.Size < 0 || s.DeletedAt.IsZero() {
		return fmt.Errorf("%w: missing content identity or deletion time", ErrSidecarFormat)
	}
	return nil
}
