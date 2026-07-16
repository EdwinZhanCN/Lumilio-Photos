package processors

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db/repo"
	"server/internal/sourcing"
	"server/internal/utils/hash"
)

// IngestAsset converts an upload payload into an IngestSource and delegates to the
// SourceMaterializer for validation, staging→inbox commit, asset creation, and pipeline enqueuing.
// Audit logging is handled by the materializer.
func (ap *AssetProcessor) IngestAsset(ctx context.Context, task AssetPayload) (*repo.Asset, error) {
	start := time.Now()
	defer func() {
		ap.logger.Debug("ingest_task",
			zap.String("filename", task.FileName),
			zap.Duration("duration", time.Since(start)),
		)
	}()
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

	var contentHash *string
	if task.ContentHash != "" {
		contentHash = &task.ContentHash
	}
	var quickFingerprint *string
	var quickFingerprintVersion *string
	if task.QuickFingerprint != "" {
		quickFingerprint = &task.QuickFingerprint
		version := hash.QuickFingerprintVersion
		quickFingerprintVersion = &version
	}

	return ap.materializer.Materialize(ctx, sourcing.IngestSource{
		RepositoryID:            repoUUID,
		OwnerID:                 ownerIDPtr,
		Kind:                    sourcing.IngestSourceUpload,
		SourcePath:              task.StagedPath,
		OriginalFilename:        task.FileName,
		ContentHash:             contentHash,
		QuickFingerprint:        quickFingerprint,
		QuickFingerprintVersion: quickFingerprintVersion,
		Timestamp:               task.Timestamp,
		ContentType:             task.ContentType,
	})
}
