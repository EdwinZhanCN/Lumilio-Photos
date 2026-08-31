package materializer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
)

// HashApplier is the coordinator-only half of repository content
// materialization. It owns no database handle and can write only through the
// transaction supplied by the commit coordinator.
type HashApplier struct {
	now func() time.Time
}

func NewHashApplier() *HashApplier {
	return &HashApplier{now: func() time.Time { return time.Now().UTC() }}
}

// ApplyHash persists a prepared fact in the caller-owned coordinator
// transaction. It never opens a catalog transaction itself.
func (m *HashApplier) ApplyHash(ctx context.Context, tx *sql.Tx, prepared HashPreparation) (Result, error) {
	node := prepared.Node
	result := Result{Code: ResultStale, RepositoryID: node.RepositoryID, NodeID: node.NodeID, Revision: node.ObservationRevision}
	if m == nil || tx == nil {
		return result, errors.New("repository hash apply requires a transaction")
	}
	if node.NodeID == uuid.Nil || node.RepositoryID == uuid.Nil || node.ObservationRevision <= 0 || prepared.OwnerID <= 0 || prepared.Observation.ObservationToken == "" {
		return result, errors.New("invalid prepared repository hash")
	}
	if prepared.Reobserve {
		return m.applyNewerObservation(ctx, tx, node, prepared.Observation, prepared.OwnerID)
	}
	if prepared.Observation.ContentHash == nil || prepared.AssetType == "" || prepared.MimeType == "" || prepared.ProjectedPath == "" {
		return result, errors.New("prepared repository hash is missing stable content facts")
	}

	now := dbtypes.NewTimestamp(m.now())
	candidateAssetID := uuid.New()
	queries := repo.New(tx)
	current, err := queries.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{
		RepositoryID: node.RepositoryID, NodeID: node.NodeID,
	})
	if err != nil {
		return result, err
	}
	if current.Lifecycle != "active" || current.ObservationRevision != node.ObservationRevision ||
		current.StabilityToken == nil || *current.StabilityToken != prepared.Observation.ObservationToken {
		return result, nil
	}
	// A newer or equal active binding is already the canonical projection.
	// Check it before creating immutable rows so a racing/replayed hash cannot
	// leave an unbound asset behind.
	active, locationErr := queries.GetActiveAssetLocationByNode(ctx, node.NodeID)
	switch {
	case locationErr == nil && active.BoundObservationRevision >= node.ObservationRevision:
		asset, assetErr := queries.GetAssetByIDAny(ctx, active.AssetID)
		if assetErr != nil {
			return result, assetErr
		}
		return Result{
			Code: ResultNoop, RepositoryID: node.RepositoryID, NodeID: node.NodeID,
			AssetID: asset.AssetID, ContentID: asset.ContentID, Revision: node.ObservationRevision,
		}, nil
	case locationErr != nil && !errors.Is(locationErr, sql.ErrNoRows):
		return result, locationErr
	}
	content, err := queries.InsertContentObject(ctx, repo.InsertContentObjectParams{
		ContentID: uuid.New(), HashAlgorithm: "blake3-v1", FullHash: *prepared.Observation.ContentHash,
		FileSize: prepared.Observation.Size, CreatedAt: now,
	})
	if err != nil {
		return result, fmt.Errorf("insert exact content identity: %w", err)
	}
	asset, err := queries.InsertOwnerContentAsset(ctx, repo.InsertOwnerContentAssetParams{
		AssetID: candidateAssetID, OwnerID: &prepared.OwnerID, ContentID: content.ContentID,
		Type: prepared.AssetType, OriginalFilename: path.Base(prepared.ProjectedPath),
		MimeType: prepared.MimeType, UploadTime: now,
		TakenTime: dbtypes.NewTimestamp(time.Unix(0, prepared.Observation.ModTimeNS)),
		Rating:    int64Ptr(0),
		Status:    dbtypes.JSON(`{"state":"processing","message":"Pending processing"}`), UpdatedAt: now,
	})
	if err != nil {
		return result, fmt.Errorf("insert owner content asset: %w", err)
	}
	switch {
	case locationErr == nil:
		closedRevision := node.ObservationRevision
		rows, closeErr := queries.CloseActiveAssetLocationCAS(ctx, repo.CloseActiveAssetLocationCASParams{
			NodeID: node.NodeID, UnboundObservationRevision: &closedRevision, UpdatedAt: now,
		})
		if closeErr != nil {
			return result, closeErr
		}
		if rows != 1 {
			return result, nil
		}
	case !errors.Is(locationErr, sql.ErrNoRows):
		return result, locationErr
	}
	if _, err := queries.BindAssetLocation(ctx, repo.BindAssetLocationParams{
		LocationID: uuid.New(), NodeID: node.NodeID, AssetID: asset.AssetID,
		BoundObservationRevision: node.ObservationRevision, CreatedAt: now,
	}); err != nil {
		return result, fmt.Errorf("bind asset location: %w", err)
	}
	if current.LastSeenRunID.Valid {
		pendingMaterialization, err := queries.CountPendingRepositoryMaterialization(ctx, node.RepositoryID)
		if err != nil {
			return result, err
		}
		if _, err := queries.UpdateRepositoryScanRunProgress(ctx, repo.UpdateRepositoryScanRunProgressParams{
			RunID: current.LastSeenRunID.UUID, BytesHashed: prepared.Observation.Size,
			OutboxDepth: pendingMaterialization, UpdatedAt: now,
		}); err != nil {
			return result, err
		}
	}
	return Result{
		Code: ResultBound, RepositoryID: node.RepositoryID, NodeID: node.NodeID, AssetID: asset.AssetID,
		ContentID: content.ContentID, Revision: node.ObservationRevision, NewAsset: asset.AssetID == candidateAssetID,
	}, nil
}

func (m *HashApplier) applyNewerObservation(
	ctx context.Context,
	tx *sql.Tx,
	node repo.RepositoryNode,
	observation storage.FileObservation,
	resolvedOwnerID int32,
) (Result, error) {
	result := Result{Code: ResultStale, NodeID: node.NodeID, Revision: node.ObservationRevision}
	now := dbtypes.NewTimestamp(m.now())
	queries := repo.New(tx)
	current, err := queries.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{
		RepositoryID: node.RepositoryID, NodeID: node.NodeID,
	})
	if err != nil {
		return result, err
	}
	if current.ObservationRevision != node.ObservationRevision || current.Lifecycle != "active" {
		return result, nil
	}
	revision, err := queries.AllocateRepositoryObservationRevision(ctx, repo.AllocateRepositoryObservationRevisionParams{
		RepositoryID: node.RepositoryID, UpdatedAt: now,
	})
	if err != nil {
		return result, err
	}
	updated, err := queries.UpsertRepositoryNodeObservation(ctx, repo.UpsertRepositoryNodeObservationParams{
		NodeID: node.NodeID, RepositoryID: node.RepositoryID, ParentNodeID: node.ParentNodeID,
		Name: node.Name, NameKey: node.NameKey, Kind: node.Kind,
		NativeIdentityKind:  observation.FileIdentityKind,
		NativeIdentityValue: observation.FileIdentity,
		VolumeIdentity:      observationVolumeIdentity(observation),
		ObservationRevision: revision, StabilityToken: &observation.ObservationToken,
		FileSize: &observation.Size, ModifiedAtNs: &observation.ModTimeNS,
		ChangedAtNs: observation.ChangeTimeNS, LastSeenRunID: current.LastSeenRunID, CreatedAt: now,
	})
	if err != nil {
		return result, err
	}
	ownerID := int64(resolvedOwnerID)
	sourceKey := fmt.Sprintf("hash_reobserve:%s:%s", node.NodeID, observation.ObservationToken)
	pathHint := observation.Path.String()
	entryKind := node.Kind
	persisted, err := queries.InsertRepositoryObservation(ctx, repo.InsertRepositoryObservationParams{
		ObservationID: uuid.New(), RepositoryID: node.RepositoryID, Revision: revision,
		Source: "verifier", SourceEventKey: &sourceKey, PathHint: &pathHint,
		ParentNodeID: node.ParentNodeID, Name: &node.Name, NameKey: &node.NameKey,
		EntryKind: &entryKind, FileSize: &observation.Size, ModifiedAtNs: &observation.ModTimeNS,
		ChangedAtNs:          observation.ChangeTimeNS,
		NativeIdentityKind:   observation.FileIdentityKind,
		NativeIdentityValue:  observation.FileIdentity,
		StabilityTokenBefore: node.StabilityToken,
		StabilityTokenAfter:  &observation.ObservationToken,
		ResolvedOwnerID:      &ownerID,
		MappedNodeID:         uuid.NullUUID{UUID: node.NodeID, Valid: true}, CreatedAt: now,
	})
	if err != nil {
		return result, err
	}
	processedAt := now.Time.UnixMicro()
	if _, err := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
		RepositoryID: node.RepositoryID, ObservationID: persisted.ObservationID,
		MappedNodeID:    uuid.NullUUID{UUID: node.NodeID, Valid: true},
		ProcessingState: "applied", ProcessedAt: &processedAt,
	}); err != nil {
		return result, err
	}
	result.Revision = updated.ObservationRevision
	return result, nil
}

func observationVolumeIdentity(observation storage.FileObservation) *string {
	if observation.FileIdentityKind == nil || observation.FileIdentity == nil {
		return nil
	}
	for index, character := range *observation.FileIdentity {
		if character == ':' && index > 0 {
			value := *observation.FileIdentityKind + ":" + (*observation.FileIdentity)[:index]
			return &value
		}
	}
	return nil
}

func int64Ptr(value int64) *int64 { return &value }
