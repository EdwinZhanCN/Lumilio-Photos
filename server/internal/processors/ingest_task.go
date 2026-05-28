package processors

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"server/internal/db/repo"
	"server/internal/sourcing"
)

// IngestAsset converts an upload payload into an IngestSource and delegates to the
// SourceMaterializer for validation, staging→inbox commit, asset creation, and pipeline enqueuing.
// Audit logging is handled by the materializer.
func (ap *AssetProcessor) IngestAsset(ctx context.Context, task AssetPayload) (*repo.Asset, error) {
	// Resolve owner from upload payload (upload-specific concern)
	var ownerIDPtr *int32
	if task.UserID != "" && task.UserID != "anonymous" {
		var id int
		if _, err := fmt.Sscanf(task.UserID, "%d", &id); err == nil {
			o := int32(id)
			ownerIDPtr = &o
		} else if user, err := ap.queries.GetUserByUsername(ctx, task.UserID); err == nil {
			ownerIDPtr = &user.UserID
		}
	}

	// Parse optional repository ID; uuid.Nil signals fallback-to-primary in the materializer
	var repoUUID uuid.UUID
	if task.RepositoryID != "" {
		parsed, err := uuid.Parse(task.RepositoryID)
		if err != nil {
			return nil, fmt.Errorf("invalid repository ID: %w", err)
		}
		repoUUID = parsed
	}

	var hashPtr *string
	if task.ClientHash != "" {
		hashPtr = &task.ClientHash
	}

	return ap.materializer.Materialize(ctx, sourcing.IngestSource{
		RepositoryID:     repoUUID,
		OwnerID:          ownerIDPtr,
		Kind:             sourcing.IngestSourceUpload,
		SourcePath:       task.StagedPath,
		OriginalFilename: task.FileName,
		Hash:             hashPtr,
		Timestamp:        task.Timestamp,
		ContentType:      task.ContentType,
	})
}
