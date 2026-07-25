package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EffectRuntime struct {
	pool     *pgxpool.Pool
	queries  *repo.Queries
	registry *ToolRegistry
}

type EffectReceipt struct {
	EffectID         string `json:"effect_id"`
	ToolName         string `json:"tool_name"`
	Status           string `json:"status"`
	Count            int    `json:"count"`
	AlbumID          int    `json:"album_id,omitempty"`
	Message          string `json:"message"`
	AlreadyCommitted bool   `json:"already_committed,omitempty"`
}

func NewEffectRuntime(pool *pgxpool.Pool, queries *repo.Queries, registry *ToolRegistry) *EffectRuntime {
	return &EffectRuntime{pool: pool, queries: queries, registry: registry}
}

func effectUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func (r *EffectRuntime) Prepare(ctx context.Context, userID int32, threadID string, runID uuid.UUID, toolName string, membership []uuid.UUID, payload, target any) (uuid.UUID, error) {
	if r == nil || r.queries == nil {
		return uuid.Nil, errors.New("effect runtime unavailable")
	}
	policy, ok := r.registry.EffectPolicy(toolName)
	if !ok || !policy.Confirmation {
		return uuid.Nil, fmt.Errorf("tool %s has no confirmed effect policy", toolName)
	}
	if len(membership) == 0 || len(membership) > policy.MaxCardinality {
		return uuid.Nil, fmt.Errorf("effect membership cardinality %d is outside policy", len(membership))
	}
	pgIDs := make([]pgtype.UUID, len(membership))
	for i, id := range membership {
		pgIDs[i] = effectUUID(id)
	}
	authorized, err := r.queries.GetAuthorizedAssetIDs(ctx, repo.GetAuthorizedAssetIDsParams{
		AssetIds: pgIDs,
		OwnerID:  userID,
	})
	if err != nil {
		return uuid.Nil, err
	}
	allowed := make(map[uuid.UUID]struct{}, len(authorized))
	for _, id := range authorized {
		if id.Valid {
			allowed[uuid.UUID(id.Bytes)] = struct{}{}
		}
	}
	if len(allowed) != len(membership) {
		return uuid.Nil, sql.ErrNoRows
	}
	for _, id := range membership {
		if _, ok := allowed[id]; !ok {
			return uuid.Nil, sql.ErrNoRows
		}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, err
	}
	targetJSON, err := json.Marshal(target)
	if err != nil {
		return uuid.Nil, err
	}
	effectID := uuid.New()
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%d:%s:%s:%s:", userID, threadID, runID, toolName)
	_, _ = hash.Write(payloadJSON)
	_, _ = hash.Write(targetJSON)
	for _, id := range membership {
		_, _ = hash.Write(id[:])
	}
	idempotencyKey := hex.EncodeToString(hash.Sum(nil))
	row, err := r.queries.CreatePendingAgentEffect(ctx, repo.CreatePendingAgentEffectParams{
		EffectID: effectUUID(effectID), UserID: userID, ThreadID: threadID,
		InitiatingRunID: effectUUID(runID), ToolName: toolName,
		EffectClass: policy.Class, PolicyVersion: int32(policy.PolicyVersion),
		MembershipSnapshot: pgIDs, Payload: payloadJSON, Target: targetJSON,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.UUID(row.EffectID.Bytes), nil
}

func (r *EffectRuntime) Reject(ctx context.Context, userID int32, threadID string, effectID uuid.UUID) error {
	receipt, _ := json.Marshal(EffectReceipt{EffectID: effectID.String(), Status: "rejected"})
	return r.queries.UpdatePendingAgentEffect(ctx, repo.UpdatePendingAgentEffectParams{
		EffectID: effectUUID(effectID), UserID: userID, ThreadID: threadID,
		Status: "rejected", Receipt: receipt,
	})
}

func (r *EffectRuntime) Commit(ctx context.Context, userID int32, threadID string, runID, effectID uuid.UUID) (EffectReceipt, error) {
	if r == nil || r.pool == nil {
		return EffectReceipt{}, errors.New("effect runtime unavailable")
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return EffectReceipt{}, err
	}
	defer tx.Rollback(ctx)
	q := r.queries.WithTx(tx)
	effect, err := q.GetPendingAgentEffectForUpdate(ctx, repo.GetPendingAgentEffectForUpdateParams{
		EffectID: effectUUID(effectID), UserID: userID, ThreadID: threadID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return EffectReceipt{}, sql.ErrNoRows
		}
		return EffectReceipt{}, err
	}
	if effect.Status == "committed" && len(effect.Receipt) > 0 {
		var receipt EffectReceipt
		if err := json.Unmarshal(effect.Receipt, &receipt); err != nil {
			return EffectReceipt{}, err
		}
		receipt.AlreadyCommitted = true
		return receipt, nil
	}
	if effect.Status != "pending" {
		return EffectReceipt{}, fmt.Errorf("effect is %s", effect.Status)
	}
	policy, ok := r.registry.EffectPolicy(effect.ToolName)
	if !ok || int32(policy.PolicyVersion) != effect.PolicyVersion {
		return EffectReceipt{}, errors.New("effect policy version is no longer executable")
	}
	if _, err := q.BindPendingAgentEffectExecutingRun(ctx, repo.BindPendingAgentEffectExecutingRunParams{
		RunID: effectUUID(runID), EffectID: effectUUID(effectID),
		UserID: userID, ThreadID: threadID,
	}); err != nil {
		return EffectReceipt{}, sql.ErrNoRows
	}
	locked, err := q.LockAuthorizedAssetIDs(ctx, repo.LockAuthorizedAssetIDsParams{
		AssetIds: effect.MembershipSnapshot, OwnerID: userID,
	})
	if err != nil || len(locked) != len(effect.MembershipSnapshot) {
		return EffectReceipt{}, sql.ErrNoRows
	}

	receipt := EffectReceipt{
		EffectID: effectID.String(), ToolName: effect.ToolName,
		Status: "completed", Count: len(effect.MembershipSnapshot),
	}
	switch effect.ToolName {
	case "bulk_like_assets":
		var payload struct {
			Liked bool `json:"liked"`
		}
		if err := json.Unmarshal(effect.Payload, &payload); err != nil {
			return EffectReceipt{}, err
		}
		if err := q.BulkUpdateAssetLiked(ctx, repo.BulkUpdateAssetLikedParams{
			Liked: payload.Liked, AssetIds: effect.MembershipSnapshot,
		}); err != nil {
			return EffectReceipt{}, err
		}
		verb := "Liked"
		if !payload.Liked {
			verb = "Unliked"
		}
		receipt.Message = fmt.Sprintf("%s %d assets", verb, receipt.Count)
	case "tag_assets":
		var payload struct {
			Mode string   `json:"mode"`
			Tags []string `json:"tags"`
		}
		if err := json.Unmarshal(effect.Payload, &payload); err != nil {
			return EffectReceipt{}, err
		}
		if err := applyTagsTx(ctx, q, effect.MembershipSnapshot, payload.Mode, payload.Tags); err != nil {
			return EffectReceipt{}, err
		}
		receipt.Message = fmt.Sprintf("Applied tag change to %d assets", receipt.Count)
	case "create_album":
		var payload struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(effect.Payload, &payload); err != nil {
			return EffectReceipt{}, err
		}
		album, err := q.CreateAlbum(ctx, repo.CreateAlbumParams{
			UserID: userID, AlbumName: payload.Title, AlbumType: repo.AlbumTypeDefault,
		})
		if err != nil {
			return EffectReceipt{}, err
		}
		if err := addAssetsToAlbumTx(ctx, q, album.AlbumID, effect.MembershipSnapshot); err != nil {
			return EffectReceipt{}, err
		}
		receipt.AlbumID = int(album.AlbumID)
		receipt.Message = fmt.Sprintf("Created album %q with %d photos", payload.Title, receipt.Count)
	case "add_to_album":
		var target struct {
			AlbumID int32 `json:"album_id"`
		}
		if err := json.Unmarshal(effect.Target, &target); err != nil {
			return EffectReceipt{}, err
		}
		album, err := q.GetAlbumByID(ctx, target.AlbumID)
		if err != nil || album.UserID != userID {
			return EffectReceipt{}, sql.ErrNoRows
		}
		if err := addAssetsToAlbumTx(ctx, q, target.AlbumID, effect.MembershipSnapshot); err != nil {
			return EffectReceipt{}, err
		}
		receipt.AlbumID = int(target.AlbumID)
		receipt.Message = fmt.Sprintf("Added %d photos to album", receipt.Count)
	default:
		return EffectReceipt{}, fmt.Errorf("unsupported effect tool %q", effect.ToolName)
	}
	if err := ctx.Err(); err != nil {
		return EffectReceipt{}, err
	}
	receiptJSON, err := json.Marshal(receipt)
	if err != nil {
		return EffectReceipt{}, err
	}
	if err := q.UpdatePendingAgentEffect(ctx, repo.UpdatePendingAgentEffectParams{
		EffectID: effectUUID(effectID), UserID: userID, ThreadID: threadID,
		Status: "committed", Receipt: receiptJSON,
	}); err != nil {
		return EffectReceipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return EffectReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return EffectReceipt{}, err
	}
	return receipt, nil
}

func addAssetsToAlbumTx(ctx context.Context, q *repo.Queries, albumID int32, ids []pgtype.UUID) error {
	for i, id := range ids {
		position := int32(i)
		if err := q.AddAssetToAlbum(ctx, repo.AddAssetToAlbumParams{
			AssetID: id, AlbumID: albumID, Position: &position,
		}); err != nil {
			return err
		}
	}
	return nil
}

func applyTagsTx(ctx context.Context, q *repo.Queries, assets []pgtype.UUID, mode string, names []string) error {
	tagIDs := make([]int32, 0, len(names))
	for _, name := range names {
		tag, err := q.GetTagByName(ctx, name)
		if errors.Is(err, pgx.ErrNoRows) && mode == "add" {
			tag, err = q.CreateTag(ctx, repo.CreateTagParams{TagName: name})
		}
		if errors.Is(err, pgx.ErrNoRows) && mode == "remove" {
			continue
		}
		if err != nil {
			return err
		}
		tagIDs = append(tagIDs, tag.TagID)
	}
	confidence := pgtype.Numeric{}
	if err := confidence.Scan("1.000"); err != nil {
		return err
	}
	for _, assetID := range assets {
		for _, tagID := range tagIDs {
			if mode == "remove" {
				if err := q.RemoveTagFromAsset(ctx, repo.RemoveTagFromAssetParams{AssetID: assetID, TagID: tagID}); err != nil {
					return err
				}
			} else if err := q.AddTagToAsset(ctx, repo.AddTagToAssetParams{
				AssetID: assetID, TagID: tagID, Confidence: confidence, Source: "user",
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
