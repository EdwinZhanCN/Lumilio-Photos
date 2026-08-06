package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

const effectReceiptRetention = 30 * 24 * time.Hour

type EffectRuntime struct {
	pool     *sql.DB
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

func NewEffectRuntime(pool *sql.DB, queries *repo.Queries, registry *ToolRegistry) *EffectRuntime {
	return &EffectRuntime{pool: pool, queries: queries, registry: registry}
}

func nullableEffectUUID(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: id, Valid: true}
}

func (r *EffectRuntime) Prepare(ctx context.Context, userID int32, threadID string, runID uuid.UUID, toolName string, membership []uuid.UUID, payload, target any) (uuid.UUID, error) {
	if r == nil || r.pool == nil || r.queries == nil {
		return uuid.Nil, errors.New("effect runtime unavailable")
	}
	// Receipts remain queryable after a run so the client can reconcile a
	// committed mutation when the SSE transport drops. Opportunistic bounded
	// cleanup avoids turning this journal into unbounded conversation history.
	cutoff := time.Now().Add(-effectReceiptRetention).UnixMilli()
	_, _ = r.pool.ExecContext(ctx, `DELETE FROM agent_pending_effects
		WHERE user_id = ? AND status IN ('committed', 'rejected', 'cancelled', 'failed') AND updated_at < ?`, userID, cutoff)
	policy, ok := r.registry.EffectPolicy(toolName)
	if !ok || !policy.Confirmation {
		return uuid.Nil, fmt.Errorf("tool %s has no confirmed effect policy", toolName)
	}
	if len(membership) == 0 || len(membership) > policy.MaxCardinality {
		return uuid.Nil, fmt.Errorf("effect membership cardinality %d is outside policy", len(membership))
	}
	assetIDs := append([]uuid.UUID(nil), membership...)
	authorized, err := r.queries.GetAuthorizedAssetIDs(ctx, repo.GetAuthorizedAssetIDsParams{
		AssetIds: dbtypes.UUIDsJSONParam(assetIDs),
		OwnerID:  &userID,
	})
	if err != nil {
		return uuid.Nil, err
	}
	allowed := make(map[uuid.UUID]struct{}, len(authorized))
	for _, id := range authorized {
		allowed[id] = struct{}{}
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
	now := dbtypes.NewTimestamp(time.Now())
	row, err := r.queries.CreatePendingAgentEffect(ctx, repo.CreatePendingAgentEffectParams{
		EffectID: effectID, UserID: userID, ThreadID: threadID,
		InitiatingRunID: runID, ToolName: toolName,
		EffectClass: policy.Class, PolicyVersion: int64(policy.PolicyVersion),
		MembershipSnapshot: dbtypes.UUIDs(assetIDs), Payload: dbtypes.JSON(payloadJSON), Target: dbtypes.JSON(targetJSON),
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return row.EffectID, nil
}

func (r *EffectRuntime) Reject(ctx context.Context, userID int32, threadID string, effectID uuid.UUID) error {
	receipt, _ := json.Marshal(EffectReceipt{EffectID: effectID.String(), Status: "rejected"})
	return r.queries.UpdatePendingAgentEffect(ctx, repo.UpdatePendingAgentEffectParams{
		EffectID: effectID, UserID: userID, ThreadID: threadID,
		Status: "rejected", Receipt: dbtypes.JSON(receipt), UpdatedAt: dbtypes.NewTimestamp(time.Now()),
	})
}

func (r *EffectRuntime) Commit(ctx context.Context, userID int32, threadID string, runID, effectID uuid.UUID) (EffectReceipt, error) {
	if r == nil || r.pool == nil {
		return EffectReceipt{}, errors.New("effect runtime unavailable")
	}
	tx, err := r.pool.BeginTx(ctx, nil)
	if err != nil {
		return EffectReceipt{}, err
	}
	defer tx.Rollback()
	q := r.queries.WithTx(tx)
	effect, err := q.GetPendingAgentEffectForUpdate(ctx, repo.GetPendingAgentEffectForUpdateParams{
		EffectID: effectID, UserID: userID, ThreadID: threadID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	if !ok || int64(policy.PolicyVersion) != effect.PolicyVersion {
		return EffectReceipt{}, errors.New("effect policy version is no longer executable")
	}
	if _, err := q.BindPendingAgentEffectExecutingRun(ctx, repo.BindPendingAgentEffectExecutingRunParams{
		RunID: nullableEffectUUID(runID), UpdatedAt: dbtypes.NewTimestamp(time.Now()), EffectID: effectID,
		UserID: userID, ThreadID: threadID,
	}); err != nil {
		return EffectReceipt{}, sql.ErrNoRows
	}
	locked, err := q.LockAuthorizedAssetIDs(ctx, repo.LockAuthorizedAssetIDsParams{
		AssetIds: dbtypes.UUIDsJSONParam([]uuid.UUID(effect.MembershipSnapshot)), OwnerID: &userID,
	})
	if err != nil || len(locked) != len(effect.MembershipSnapshot) {
		return EffectReceipt{}, sql.ErrNoRows
	}

	receipt := EffectReceipt{
		EffectID: effectID.String(), ToolName: effect.ToolName,
		Status: "committed", Count: len(effect.MembershipSnapshot),
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
			Liked: payload.Liked,
			AssetIds: dbtypes.UUIDsJSONParam(
				[]uuid.UUID(effect.MembershipSnapshot),
			),
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
		if err := applyTagsTx(ctx, q, []uuid.UUID(effect.MembershipSnapshot), payload.Mode, payload.Tags); err != nil {
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
		if err := addAssetsToAlbumTx(ctx, q, album.AlbumID, []uuid.UUID(effect.MembershipSnapshot)); err != nil {
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
		if err := addAssetsToAlbumTx(ctx, q, target.AlbumID, []uuid.UUID(effect.MembershipSnapshot)); err != nil {
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
		EffectID: effectID, UserID: userID, ThreadID: threadID,
		Status: "committed", Receipt: dbtypes.JSON(receiptJSON), UpdatedAt: dbtypes.NewTimestamp(time.Now()),
	}); err != nil {
		return EffectReceipt{}, err
	}
	if err := ctx.Err(); err != nil {
		return EffectReceipt{}, err
	}
	if err := tx.Commit(); err != nil {
		return EffectReceipt{}, err
	}
	return receipt, nil
}

func addAssetsToAlbumTx(ctx context.Context, q *repo.Queries, albumID int32, ids []uuid.UUID) error {
	for i, id := range ids {
		if err := q.AddAssetToAlbum(ctx, repo.AddAssetToAlbumParams{
			AssetID: id, AlbumID: albumID, Position: int64(i),
		}); err != nil {
			return err
		}
	}
	return nil
}

func applyTagsTx(ctx context.Context, q *repo.Queries, assets []uuid.UUID, mode string, names []string) error {
	tagIDs := make([]int32, 0, len(names))
	for _, name := range names {
		tag, err := q.GetTagByName(ctx, name)
		if errors.Is(err, sql.ErrNoRows) && mode == "add" {
			tag, err = q.CreateTag(ctx, repo.CreateTagParams{TagName: name})
		}
		if errors.Is(err, sql.ErrNoRows) && mode == "remove" {
			continue
		}
		if err != nil {
			return err
		}
		tagIDs = append(tagIDs, tag.TagID)
	}
	for _, assetID := range assets {
		for _, tagID := range tagIDs {
			if mode == "remove" {
				if err := q.RemoveTagFromAsset(ctx, repo.RemoveTagFromAssetParams{AssetID: assetID, TagID: tagID}); err != nil {
					return err
				}
			} else if err := q.AddTagToAsset(ctx, repo.AddTagToAssetParams{
				AssetID: assetID, TagID: tagID, Confidence: 1, Source: "user",
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
