// Package materializer turns stable node observations into immutable content,
// owner/content Assets, and versioned Locations. Full content I/O completes
// before the short catalog CAS transaction begins.
package materializer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"

	"server/internal/db"
	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/nodegraph"
	fileutil "server/internal/utils/file"
)

type ResultCode string

const (
	ResultBound ResultCode = "bound"
	ResultStale ResultCode = "stale"
	ResultNoop  ResultCode = "noop"
)

type Result struct {
	Code      ResultCode
	NodeID    uuid.UUID
	AssetID   uuid.UUID
	ContentID uuid.UUID
	Revision  int64
	NewAsset  bool
}

type HashMaterializer struct {
	database *db.DB
	files    *storage.RepositoryFSFactory
	now      func() time.Time
}

func NewHashMaterializer(database *db.DB, files *storage.RepositoryFSFactory) *HashMaterializer {
	return &HashMaterializer{
		database: database, files: files,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (m *HashMaterializer) Process(ctx context.Context, nodeID uuid.UUID, expectedRevision int64) (Result, error) {
	result := Result{Code: ResultStale, NodeID: nodeID, Revision: expectedRevision}
	if m == nil || m.database == nil || m.files == nil {
		return result, errors.New("repository hash materializer unavailable")
	}
	if nodeID == uuid.Nil || expectedRevision <= 0 {
		return result, errors.New("node ID and expected revision are required")
	}
	node, err := m.getNodeByID(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return result, err
	}
	if node.Lifecycle != "active" || node.Kind == "directory" || node.ObservationRevision != expectedRevision {
		return result, nil
	}
	fact, err := m.database.ReaderQueries.GetRepositoryObservationForNodeRevision(ctx, repo.GetRepositoryObservationForNodeRevisionParams{
		RepositoryID: node.RepositoryID,
		MappedNodeID: uuid.NullUUID{UUID: node.NodeID, Valid: true}, Revision: expectedRevision,
	})
	if err != nil {
		return result, fmt.Errorf("load resolved observation owner: %w", err)
	}
	if fact.ResolvedOwnerID == nil || *fact.ResolvedOwnerID <= 0 || *fact.ResolvedOwnerID > int64(^uint32(0)>>1) {
		return result, errors.New("repository observation has no valid resolved owner")
	}
	resolvedOwnerID := int32(*fact.ResolvedOwnerID)
	projectedPath, err := nodegraph.ProjectPath(ctx, m.database.ReaderQueries, node.RepositoryID, node.NodeID)
	if err != nil {
		return result, fmt.Errorf("project repository node path: %w", err)
	}
	repositoryPath, err := storage.ParseUserMediaPath(projectedPath)
	if err != nil {
		return result, err
	}
	validation := fileutil.ValidateFile(path.Base(projectedPath), "")
	if !validation.Valid {
		return result, fmt.Errorf("unsupported repository media: %s", validation.ErrorReason)
	}
	repository, err := m.database.ReaderQueries.GetRepository(ctx, node.RepositoryID)
	if err != nil {
		return result, fmt.Errorf("load repository: %w", err)
	}
	repositoryFS, err := m.files.OpenContext(ctx, repository)
	if err != nil {
		return result, err
	}
	observation, inspectErr := repositoryFS.InspectMedia(ctx, repositoryPath, storage.HashFull)
	closeErr := repositoryFS.Close()
	if inspectErr != nil {
		if errors.Is(inspectErr, storage.ErrRepositoryFileUnstable) {
			return m.reobserveChangedNode(ctx, node, repository, repositoryPath, resolvedOwnerID)
		}
		return result, errors.Join(inspectErr, closeErr)
	}
	if closeErr != nil {
		return result, closeErr
	}
	if node.StabilityToken == nil || observation.ObservationToken != *node.StabilityToken {
		return m.persistNewerObservation(ctx, node, observation, resolvedOwnerID)
	}
	if observation.ContentHash == nil {
		return result, errors.New("stable full hash did not produce content identity")
	}

	now := dbtypes.NewTimestamp(m.now())
	candidateAssetID := uuid.New()
	resultErr := m.database.WithTx(ctx, catalogtx.OperationRepositoryMaterializeHash, func(_ *sql.Tx, queries *repo.Queries) error {
		current, err := queries.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{
			RepositoryID: node.RepositoryID, NodeID: node.NodeID,
		})
		if err != nil {
			return err
		}
		if current.Lifecycle != "active" || current.ObservationRevision != expectedRevision ||
			current.StabilityToken == nil || *current.StabilityToken != observation.ObservationToken {
			return nil
		}
		content, err := queries.InsertContentObject(ctx, repo.InsertContentObjectParams{
			ContentID: uuid.New(), HashAlgorithm: "blake3-v1", FullHash: *observation.ContentHash,
			FileSize: observation.Size, CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("insert exact content identity: %w", err)
		}
		asset, err := queries.InsertOwnerContentAsset(ctx, repo.InsertOwnerContentAssetParams{
			AssetID: candidateAssetID, OwnerID: &resolvedOwnerID, ContentID: content.ContentID,
			Type: string(validation.AssetType), OriginalFilename: path.Base(projectedPath),
			MimeType: validation.MimeType, UploadTime: now,
			TakenTime: dbtypes.NewTimestamp(time.Unix(0, observation.ModTimeNS)),
			Rating:    int64Ptr(0),
			Status:    dbtypes.JSON(`{"state":"processing","message":"Pending processing"}`), UpdatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("insert owner content asset: %w", err)
		}
		active, locationErr := queries.GetActiveAssetLocationByNode(ctx, node.NodeID)
		switch {
		case locationErr == nil && active.BoundObservationRevision >= expectedRevision:
			result = Result{
				Code: ResultNoop, NodeID: node.NodeID, AssetID: active.AssetID,
				ContentID: content.ContentID, Revision: expectedRevision,
			}
			return nil
		case locationErr == nil:
			closedRevision := expectedRevision
			rows, closeErr := queries.CloseActiveAssetLocationCAS(ctx, repo.CloseActiveAssetLocationCASParams{
				NodeID: node.NodeID, UnboundObservationRevision: &closedRevision, UpdatedAt: now,
			})
			if closeErr != nil {
				return closeErr
			}
			if rows != 1 {
				return nil
			}
		case !errors.Is(locationErr, sql.ErrNoRows):
			return locationErr
		}
		if _, err := queries.BindAssetLocation(ctx, repo.BindAssetLocationParams{
			LocationID: uuid.New(), NodeID: node.NodeID, AssetID: asset.AssetID,
			BoundObservationRevision: expectedRevision, CreatedAt: now,
		}); err != nil {
			return fmt.Errorf("bind asset location: %w", err)
		}
		newAsset := asset.AssetID == candidateAssetID
		if newAsset {
			effectKey := fmt.Sprintf("process_asset:%s:%s", asset.AssetID, content.ContentID)
			if _, err := queries.InsertRepositoryOutboxEffect(ctx, repo.InsertRepositoryOutboxEffectParams{
				OutboxID: uuid.New(), RepositoryID: node.RepositoryID,
				EffectKey: effectKey, EffectKind: "process_asset", EntityID: asset.AssetID.String(),
				ExpectedRevision: expectedRevision, Payload: `{}`, CreatedAt: now,
			}); err != nil {
				return fmt.Errorf("publish asset processing effect: %w", err)
			}
		}
		if current.LastSeenRunID.Valid {
			outboxDepth, err := queries.CountPendingRepositoryOutbox(ctx, node.RepositoryID)
			if err != nil {
				return err
			}
			if _, err := queries.UpdateRepositoryScanRunProgress(ctx, repo.UpdateRepositoryScanRunProgressParams{
				RunID: current.LastSeenRunID.UUID, BytesHashed: observation.Size,
				OutboxDepth: outboxDepth, UpdatedAt: now,
			}); err != nil {
				return err
			}
		}
		result = Result{
			Code: ResultBound, NodeID: node.NodeID, AssetID: asset.AssetID,
			ContentID: content.ContentID, Revision: expectedRevision, NewAsset: newAsset,
		}
		return nil
	})
	if resultErr != nil {
		return result, resultErr
	}
	return result, nil
}

func (m *HashMaterializer) reobserveChangedNode(
	ctx context.Context,
	node repo.RepositoryNode,
	repository repo.Repository,
	repositoryPath storage.RepositoryPath,
	resolvedOwnerID int32,
) (Result, error) {
	repositoryFS, err := m.files.OpenContext(ctx, repository)
	if err != nil {
		return Result{Code: ResultStale, NodeID: node.NodeID, Revision: node.ObservationRevision}, err
	}
	observation, inspectErr := repositoryFS.InspectMedia(ctx, repositoryPath, storage.HashNone)
	closeErr := repositoryFS.Close()
	if inspectErr != nil || closeErr != nil {
		return Result{Code: ResultStale, NodeID: node.NodeID, Revision: node.ObservationRevision}, errors.Join(inspectErr, closeErr)
	}
	return m.persistNewerObservation(ctx, node, observation, resolvedOwnerID)
}

func (m *HashMaterializer) persistNewerObservation(
	ctx context.Context,
	node repo.RepositoryNode,
	observation storage.FileObservation,
	resolvedOwnerID int32,
) (Result, error) {
	result := Result{Code: ResultStale, NodeID: node.NodeID, Revision: node.ObservationRevision}
	now := dbtypes.NewTimestamp(m.now())
	err := m.database.WithTx(ctx, catalogtx.OperationRepositoryReobserveNode, func(_ *sql.Tx, queries *repo.Queries) error {
		current, err := queries.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{
			RepositoryID: node.RepositoryID, NodeID: node.NodeID,
		})
		if err != nil {
			return err
		}
		if current.ObservationRevision != node.ObservationRevision || current.Lifecycle != "active" {
			return nil
		}
		revision, err := queries.AllocateRepositoryObservationRevision(ctx, repo.AllocateRepositoryObservationRevisionParams{
			RepositoryID: node.RepositoryID, UpdatedAt: now,
		})
		if err != nil {
			return err
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
			return err
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
			return err
		}
		processedAt := now.Time.UnixMicro()
		if _, err := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
			RepositoryID: node.RepositoryID, ObservationID: persisted.ObservationID,
			MappedNodeID:    uuid.NullUUID{UUID: node.NodeID, Valid: true},
			ProcessingState: "applied", ProcessedAt: &processedAt,
		}); err != nil {
			return err
		}
		effectKey := fmt.Sprintf("hash:%s:%d", node.NodeID, revision)
		if _, err := queries.InsertRepositoryOutboxEffect(ctx, repo.InsertRepositoryOutboxEffectParams{
			OutboxID: uuid.New(), RepositoryID: node.RepositoryID,
			EffectKey: effectKey, EffectKind: "hash", EntityID: node.NodeID.String(),
			ExpectedRevision: revision, Payload: `{}`, CreatedAt: now,
		}); err != nil {
			return err
		}
		result.Revision = updated.ObservationRevision
		return nil
	})
	return result, err
}

func (m *HashMaterializer) getNodeByID(ctx context.Context, nodeID uuid.UUID) (repo.RepositoryNode, error) {
	row := m.database.ReaderSQL.QueryRowContext(ctx, `
		SELECT repository_id
		FROM repository_nodes
		WHERE node_id = ?`, nodeID)
	var repositoryID uuid.UUID
	if err := row.Scan(&repositoryID); err != nil {
		return repo.RepositoryNode{}, err
	}
	return m.database.ReaderQueries.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{
		RepositoryID: repositoryID, NodeID: nodeID,
	})
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
