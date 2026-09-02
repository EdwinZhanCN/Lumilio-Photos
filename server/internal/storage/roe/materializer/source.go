package materializer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/pathsemantics"
)

// KnownContent is a source-owned, already hashed file occurrence. Upload and
// cloud staging hash the same file handle they later atomically commit, then
// publish this fact only after a no-content-read final stat proves stability.
type KnownContent struct {
	RepositoryID            uuid.UUID
	OwnerID                 int32
	Source                  string
	SourceEventKey          string
	RelativePath            string
	OriginalFilename        string
	MimeType                string
	AssetType               string
	FullHash                string
	FileSize                int64
	QuickFingerprint        *string
	QuickFingerprintVersion *string
	Observation             storage.FileObservation
}

// ApplyKnownContent publishes a source fact in the caller-owned coordinator
// transaction. Filesystem and hashing work must be complete before this call;
// workers never receive a catalog writer through this API.
func (m *HashApplier) ApplyKnownContent(ctx context.Context, tx *sql.Tx, fact KnownContent) (Result, error) {
	result := Result{Code: ResultStale}
	if m == nil || tx == nil {
		return result, errors.New("repository materializer unavailable")
	}
	if fact.RepositoryID == uuid.Nil || fact.OwnerID <= 0 {
		return result, errors.New("repository and resolved owner are required")
	}
	if fact.Source != "upload" && fact.Source != "cloud" {
		return result, fmt.Errorf("unsupported known-content source %q", fact.Source)
	}
	if strings.TrimSpace(fact.SourceEventKey) == "" {
		return result, errors.New("source event key is required")
	}
	if len(fact.FullHash) != 64 || fact.FileSize < 0 {
		return result, errors.New("stable BLAKE3 identity is required")
	}
	relativePath, err := storage.ParseUserMediaPath(fact.RelativePath)
	if err != nil {
		return result, err
	}
	parts := strings.Split(relativePath.String(), "/")
	if len(parts) == 0 || len(parts) > 64 {
		return result, errors.New("source path depth is outside the supported bound")
	}
	if fact.Observation.Size != fact.FileSize || fact.Observation.ObservationToken == "" {
		return result, errors.New("known content does not match the final stability observation")
	}

	now := dbtypes.NewTimestamp(m.now())
	candidateAssetID := uuid.New()
	apply := func(queries *repo.Queries) error {
		if priorObservation, priorErr := queries.GetRepositoryObservationBySourceEvent(ctx, repo.GetRepositoryObservationBySourceEventParams{
			RepositoryID: fact.RepositoryID, Source: fact.Source, SourceEventKey: &fact.SourceEventKey,
		}); priorErr == nil && priorObservation.MappedNodeID.Valid {
			location, locationErr := queries.GetActiveAssetLocationByNode(ctx, priorObservation.MappedNodeID.UUID)
			if locationErr == nil {
				asset, assetErr := queries.GetAssetByIDAny(ctx, location.AssetID)
				if assetErr != nil {
					return assetErr
				}
				result = Result{Code: ResultNoop, RepositoryID: fact.RepositoryID, NodeID: priorObservation.MappedNodeID.UUID,
					AssetID: asset.AssetID, ContentID: asset.ContentID, Revision: priorObservation.Revision}
				return nil
			}
			if !errors.Is(locationErr, sql.ErrNoRows) {
				return locationErr
			}
			// A durable source event can outlive its active location after a
			// deletion or interrupted repair. Fall through and re-materialize the
			// occurrence instead of turning the missing location into a permanent
			// retry error.
		} else if priorErr != nil && !errors.Is(priorErr, sql.ErrNoRows) {
			return priorErr
		}
		state, err := queries.EnsureRepositoryObservationState(ctx, repo.EnsureRepositoryObservationStateParams{
			RepositoryID: fact.RepositoryID, AdapterKind: "periodic", VolumeKind: "unknown",
			PathCaseMode: "sensitive", PathNormalization: "unknown",
			CursorHealth: "unavailable", FullVerificationRequired: 1, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		semantics := pathsemantics.Semantics{
			Case:          pathsemantics.CaseMode(state.PathCaseMode),
			Normalization: pathsemantics.Normalization(state.PathNormalization),
		}

		root, err := queries.GetRepositoryRootNode(ctx, fact.RepositoryID)
		if errors.Is(err, sql.ErrNoRows) {
			revision, allocateErr := allocateRevision(ctx, queries, fact.RepositoryID, now)
			if allocateErr != nil {
				return allocateErr
			}
			root, err = queries.InsertRepositoryRootNode(ctx, repo.InsertRepositoryRootNodeParams{
				NodeID: uuid.New(), RepositoryID: fact.RepositoryID,
				ObservationRevision: revision, StabilityToken: stringPtr("source-root"), CreatedAt: now,
			})
		}
		if err != nil {
			return fmt.Errorf("ensure source root node: %w", err)
		}

		parent := root
		for _, component := range parts[:len(parts)-1] {
			nameKey, keyErr := semantics.NameKey(component)
			if keyErr != nil {
				return keyErr
			}
			child, childErr := queries.GetActiveRepositoryChildByName(ctx, repo.GetActiveRepositoryChildByNameParams{
				RepositoryID: fact.RepositoryID, ParentNodeID: uuid.NullUUID{UUID: parent.NodeID, Valid: true}, NameKey: nameKey,
			})
			switch {
			case childErr == nil:
				if child.Kind != "directory" {
					return fmt.Errorf("source parent %q is not a directory", component)
				}
				parent = child
			case errors.Is(childErr, sql.ErrNoRows):
				revision, allocateErr := allocateRevision(ctx, queries, fact.RepositoryID, now)
				if allocateErr != nil {
					return allocateErr
				}
				child, childErr = queries.UpsertRepositoryNodeObservation(ctx, repo.UpsertRepositoryNodeObservationParams{
					NodeID: uuid.New(), RepositoryID: fact.RepositoryID,
					ParentNodeID: uuid.NullUUID{UUID: parent.NodeID, Valid: true},
					Name:         component, NameKey: nameKey, Kind: "directory",
					ObservationRevision: revision, StabilityToken: stringPtr("source-directory"), CreatedAt: now,
				})
				if childErr != nil {
					return childErr
				}
				parent = child
			default:
				return childErr
			}
		}

		filename := parts[len(parts)-1]
		nameKey, err := semantics.NameKey(filename)
		if err != nil {
			return err
		}
		prior, lookupErr := queries.GetActiveRepositoryChildByName(ctx, repo.GetActiveRepositoryChildByNameParams{
			RepositoryID: fact.RepositoryID, ParentNodeID: uuid.NullUUID{UUID: parent.NodeID, Valid: true}, NameKey: nameKey,
		})
		nodeID := uuid.New()
		var stabilityBefore *string
		if lookupErr == nil {
			if prior.Kind == "directory" {
				return fmt.Errorf("source target %q is a directory", relativePath.String())
			}
			nodeID = prior.NodeID
			stabilityBefore = prior.StabilityToken
		} else if !errors.Is(lookupErr, sql.ErrNoRows) {
			return lookupErr
		}
		revision, err := allocateRevision(ctx, queries, fact.RepositoryID, now)
		if err != nil {
			return err
		}
		entryKind := string(fact.Observation.EntryKind)
		if entryKind == "regular" {
			entryKind = "file"
		}
		node, err := queries.UpsertRepositoryNodeObservation(ctx, repo.UpsertRepositoryNodeObservationParams{
			NodeID: nodeID, RepositoryID: fact.RepositoryID,
			ParentNodeID: uuid.NullUUID{UUID: parent.NodeID, Valid: true},
			Name:         filename, NameKey: nameKey, Kind: entryKind,
			NativeIdentityKind:  fact.Observation.FileIdentityKind,
			NativeIdentityValue: fact.Observation.FileIdentity,
			VolumeIdentity:      observationVolumeIdentity(fact.Observation),
			ObservationRevision: revision,
			StabilityToken:      &fact.Observation.ObservationToken,
			FileSize:            &fact.FileSize, ModifiedAtNs: &fact.Observation.ModTimeNS,
			ChangedAtNs: fact.Observation.ChangeTimeNS, CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("publish source node: %w", err)
		}
		ownerID := int64(fact.OwnerID)
		pathHint := relativePath.String()
		observation, err := queries.InsertRepositoryObservation(ctx, repo.InsertRepositoryObservationParams{
			ObservationID: uuid.New(), RepositoryID: fact.RepositoryID, Revision: revision,
			Source: fact.Source, SourceEventKey: &fact.SourceEventKey, PathHint: &pathHint,
			ParentNodeID: uuid.NullUUID{UUID: parent.NodeID, Valid: true},
			Name:         &filename, NameKey: &nameKey, EntryKind: &entryKind,
			FileSize: &fact.FileSize, ModifiedAtNs: &fact.Observation.ModTimeNS,
			ChangedAtNs:             fact.Observation.ChangeTimeNS,
			NativeIdentityKind:      fact.Observation.FileIdentityKind,
			NativeIdentityValue:     fact.Observation.FileIdentity,
			StabilityTokenBefore:    stabilityBefore,
			StabilityTokenAfter:     &fact.Observation.ObservationToken,
			QuickFingerprint:        fact.QuickFingerprint,
			QuickFingerprintVersion: fact.QuickFingerprintVersion,
			ResolvedOwnerID:         &ownerID,
			MappedNodeID:            uuid.NullUUID{UUID: node.NodeID, Valid: true}, CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("publish source observation: %w", err)
		}
		processedAt := now.Time.UnixMicro()
		if _, err := queries.CompleteRepositoryObservationCAS(ctx, repo.CompleteRepositoryObservationCASParams{
			RepositoryID: fact.RepositoryID, ObservationID: observation.ObservationID,
			MappedNodeID:    uuid.NullUUID{UUID: node.NodeID, Valid: true},
			ProcessingState: "applied", ProcessedAt: &processedAt,
		}); err != nil {
			return err
		}
		// Resolve the active binding before creating immutable rows. A replay
		// racing a prior source commit must not leave an unbound asset behind.
		active, locationErr := queries.GetActiveAssetLocationByNode(ctx, node.NodeID)
		switch {
		case locationErr == nil && active.BoundObservationRevision >= revision:
			asset, assetErr := queries.GetAssetByIDAny(ctx, active.AssetID)
			if assetErr != nil {
				return assetErr
			}
			result = Result{Code: ResultNoop, RepositoryID: fact.RepositoryID, NodeID: node.NodeID,
				AssetID: asset.AssetID, ContentID: asset.ContentID, Revision: revision}
			return nil
		case locationErr == nil:
			closedRevision := revision
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

		content, err := queries.InsertContentObject(ctx, repo.InsertContentObjectParams{
			ContentID: uuid.New(), HashAlgorithm: "blake3-v1",
			FullHash: strings.ToLower(fact.FullHash), FileSize: fact.FileSize, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		filenameForAsset := fact.OriginalFilename
		if filenameForAsset == "" {
			filenameForAsset = path.Base(relativePath.String())
		}
		asset, err := queries.InsertOwnerContentAsset(ctx, repo.InsertOwnerContentAssetParams{
			AssetID: candidateAssetID, OwnerID: &fact.OwnerID, ContentID: content.ContentID,
			Type: fact.AssetType, OriginalFilename: filenameForAsset, MimeType: fact.MimeType,
			UploadTime: now, TakenTime: dbtypes.NewTimestamp(time.Unix(0, fact.Observation.ModTimeNS)),
			Rating: int64Ptr(0), Status: dbtypes.JSON(`{"state":"processing","message":"Pending processing"}`),
			UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if _, err := queries.BindAssetLocation(ctx, repo.BindAssetLocationParams{
			LocationID: uuid.New(), NodeID: node.NodeID, AssetID: asset.AssetID,
			BoundObservationRevision: revision, CreatedAt: now,
		}); err != nil {
			return err
		}
		result = Result{Code: ResultBound, RepositoryID: fact.RepositoryID, NodeID: node.NodeID, AssetID: asset.AssetID,
			ContentID: content.ContentID, Revision: revision, NewAsset: asset.AssetID == candidateAssetID}
		return nil
	}
	return result, apply(repo.New(tx))
}

func allocateRevision(ctx context.Context, queries *repo.Queries, repositoryID uuid.UUID, now dbtypes.Timestamp) (int64, error) {
	return queries.AllocateRepositoryObservationRevision(ctx, repo.AllocateRepositoryObservationRevisionParams{
		RepositoryID: repositoryID, UpdatedAt: now,
	})
}

func stringPtr(value string) *string { return &value }
