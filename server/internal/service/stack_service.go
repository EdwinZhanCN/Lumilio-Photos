package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/utils/file"
	"server/internal/utils/raw"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/google/uuid"
)

// ErrStackNotFound is returned when a stack does not exist.
var ErrStackNotFound = errors.New("stack not found")

// ErrAssetAlreadyStacked is returned when trying to stack an asset that already belongs to a stack.
var ErrAssetAlreadyStacked = errors.New("asset already belongs to a stack")

// StackInfo holds the stack details plus its members.
type StackInfo struct {
	StackID     uuid.UUID
	MemberCount int64
	Members     []StackMemberInfo
}

// StackMemberInfo holds information about a single member of a stack.
type StackMemberInfo struct {
	AssetID  uuid.UUID
	Relation repo.StackRelation
	Position int32
}

// StackService provides stack-related operations including auto-detection
// of RAW+JPEG pairs and manual stack management.
type StackService interface {
	// AutoDetectStacks scans a repository for RAW+JPEG pairs and creates stacks.
	// Returns the number of new stacks created.
	AutoDetectStacks(ctx context.Context, repositoryID uuid.UUID) (int, error)

	// CreateManualStack groups the given assets into a new stack.
	CreateManualStack(ctx context.Context, assetIDs []uuid.UUID) (*StackInfo, error)

	// GetStackByAsset returns the stack containing the given asset, if any.
	GetStackByAsset(ctx context.Context, assetID uuid.UUID) (*StackInfo, error)

	// GetStacksByAssets returns stacks for multiple assets (batch query).
	GetStacksByAssets(ctx context.Context, assetIDs []uuid.UUID) (map[uuid.UUID]*StackInfo, error)

	// RemoveFromStack removes an asset from its stack.
	RemoveFromStack(ctx context.Context, assetID uuid.UUID) error

	// DeleteStack deletes an entire stack, releasing all members.
	DeleteStack(ctx context.Context, stackID uuid.UUID) error

	// MatchLivePhotoStack attempts to build a Live Photo stack for the asset.
	MatchLivePhotoStack(ctx context.Context, assetID uuid.UUID) error
}

type stackService struct {
	queries       *repo.Queries
	pool          *pgxpool.Pool
	logger        *zap.Logger
	auditProvider logging.RepositoryAuditProvider
}

// NewStackService creates a new StackService instance.
func NewStackService(
	queries *repo.Queries,
	pool *pgxpool.Pool,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
) StackService {
	return &stackService{
		queries:       queries,
		pool:          pool,
		logger:        logger,
		auditProvider: auditProvider,
	}
}

// fileValidator is the SSOT for file type detection (RAW, JPEG, etc.).
var fileValidator = file.NewValidator()

// stackMaxTimeGap is the maximum allowed time difference between two assets
// to be considered part of the same stack. Assets with the same base name but
// taken more than this duration apart will be placed in separate stacks.
const stackMaxTimeGap = 5 * time.Second

// iterationPattern matches Lightroom-style export filenames like "ABC001-2.JPG".
var iterationPattern = regexp.MustCompile(`^(.+?)-(\d+)$`)

// baseName returns the base name of a file without extension and without
// any Lightroom-style iteration suffix (e.g., "ABC001-2" → "ABC001").
func baseName(filename string) string {
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)
	// Normalize to handle Lightroom iteration suffix: ABC001-2 → ABC001
	if matches := iterationPattern.FindStringSubmatch(nameWithoutExt); matches != nil {
		return strings.ToLower(matches[1])
	}
	return strings.ToLower(nameWithoutExt)
}

// classifyRelation determines the stack relation for an asset based on its extension.
// Uses the centralized file.Validator (SSOT) for RAW/JPEG detection.
func classifyRelation(filename string) repo.StackRelation {
	ext := strings.ToLower(filepath.Ext(filename))
	if _, isRaw := raw.RAWExtensions[ext]; isRaw {
		return repo.StackRelationRawOriginal
	}
	// Use file validator to check if it's a standard photo (JPEG, PNG, etc.)
	if fileValidator.IsRAWFile(filename) {
		return repo.StackRelationRawOriginal
	}
	if ext == ".jpg" || ext == ".jpeg" {
		return repo.StackRelationJpegOriginal
	}
	return repo.StackRelationAlternative
}

// isIteration checks if the filename has a Lightroom-style iteration suffix.
func isIteration(filename string) bool {
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)
	return iterationPattern.MatchString(nameWithoutExt)
}

// effectiveTime returns taken_time when available, falling back to upload_time.
func effectiveTime(taken pgtype.Timestamptz, upload pgtype.Timestamptz) time.Time {
	if taken.Valid {
		return taken.Time
	}
	if upload.Valid {
		return upload.Time
	}
	return time.Now()
}

// timeCluster groups candidates by base name, then splits each group into
// sub-groups where all members were taken within stackMaxTimeGap of each other.
func timeCluster(candidates []repo.FindCandidatesForStackingByNameRow) []struct {
	BaseName string
	Members  []repo.FindCandidatesForStackingByNameRow
} {
	// First group by base name
	baseGroups := make(map[string][]repo.FindCandidatesForStackingByNameRow)
	for _, c := range candidates {
		bn := baseName(c.OriginalFilename)
		baseGroups[bn] = append(baseGroups[bn], c)
	}

	var result []struct {
		BaseName string
		Members  []repo.FindCandidatesForStackingByNameRow
	}

	for bn, group := range baseGroups {
		if len(group) < 2 {
			continue
		}

		// Sort by effective time within this base-name group
		sort.Slice(group, func(i, j int) bool {
			return effectiveTime(group[i].TakenTime, group[i].UploadTime).
				Before(effectiveTime(group[j].TakenTime, group[j].UploadTime))
		})

		// Split into time-proximity clusters
		var currentCluster []repo.FindCandidatesForStackingByNameRow
		for _, a := range group {
			t := effectiveTime(a.TakenTime, a.UploadTime)

			if len(currentCluster) == 0 {
				currentCluster = append(currentCluster, a)
				continue
			}

			// Check gap from the last member of the current cluster
			lastTime := effectiveTime(currentCluster[len(currentCluster)-1].TakenTime, currentCluster[len(currentCluster)-1].UploadTime)
			if t.Sub(lastTime) <= stackMaxTimeGap {
				currentCluster = append(currentCluster, a)
			} else {
				// Gap too large — finalize current cluster and start a new one
				if len(currentCluster) >= 2 {
					result = append(result, struct {
						BaseName string
						Members  []repo.FindCandidatesForStackingByNameRow
					}{BaseName: bn, Members: currentCluster})
				}
				currentCluster = []repo.FindCandidatesForStackingByNameRow{a}
			}
		}

		// Finalize the last cluster
		if len(currentCluster) >= 2 {
			result = append(result, struct {
				BaseName string
				Members  []repo.FindCandidatesForStackingByNameRow
			}{BaseName: bn, Members: currentCluster})
		}
	}

	return result
}

// AutoDetectStacks scans a repository for unstacked assets that share base names
// and creates stacks for them. It handles:
//  1. RAW+JPEG pairs (same base name, different extensions)
//  2. Edited/exported iterations (e.g., ABC001-1.JPG, ABC001-2.JPG)
//
// Time proximity is enforced per stack: only assets taken within
// stackMaxTimeGap of each other are stacked together.
func (s *stackService) AutoDetectStacks(ctx context.Context, repositoryID uuid.UUID) (int, error) {
	logger := s.logger.With(zap.String("repository_id", repositoryID.String()))

	candidates, err := s.queries.FindCandidatesForStackingByName(ctx, pgtype.UUID{
		Bytes: repositoryID,
		Valid: true,
	})
	if err != nil {
		return 0, fmt.Errorf("find candidates for stacking: %w", err)
	}

	if len(candidates) < 2 {
		return 0, nil
	}

	// Cluster by base name + time proximity
	clusters := timeCluster(candidates)

	createdCount := 0
	for _, cluster := range clusters {
		// Only create stacks for clusters that contain at least one RAW file
		// or have multiple iterations
		hasRaw := false
		hasIteration := false
		for _, a := range cluster.Members {
			if _, isRaw := raw.RAWExtensions[strings.ToLower(filepath.Ext(a.OriginalFilename))]; isRaw {
				hasRaw = true
			}
			if isIteration(a.OriginalFilename) {
				hasIteration = true
			}
		}

		if !hasRaw && !hasIteration {
			continue
		}

		// Create the stack
		stackID, err := createStackRecord(ctx, s.pool, dbtypes.StackKindRawJpeg, nil)
		if err != nil {
			logger.Error("failed to create stack",
				zap.String("base_name", cluster.BaseName),
				zap.Error(err),
			)
			continue
		}

		for i, a := range cluster.Members {
			pos := int32(i)
			relation := classifyRelation(a.OriginalFilename)

			assetUUID := pgtype.UUID{Bytes: a.AssetID.Bytes, Valid: true}
			err := s.queries.AddStackMember(ctx, repo.AddStackMemberParams{
				AssetID:  assetUUID,
				StackID:  stackID,
				Relation: relation,
				Position: &pos,
			})
			if err != nil {
				logger.Error("failed to add stack member",
					zap.String("base_name", cluster.BaseName),
					zap.String("asset", a.OriginalFilename),
					zap.Error(err),
				)
			}
		}

		createdCount++
		logger.Debug("created stack",
			zap.String("base_name", cluster.BaseName),
			zap.Int("members", len(cluster.Members)),
		)
	}

	return createdCount, nil
}

// CreateManualStack groups the given assets into a new stack.
func (s *stackService) CreateManualStack(ctx context.Context, assetIDs []uuid.UUID) (*StackInfo, error) {
	if len(assetIDs) < 2 {
		return nil, errors.New("at least 2 assets are required to create a stack")
	}

	// Check that none of the assets are already stacked
	pgUUIDs := make([]pgtype.UUID, len(assetIDs))
	for i, id := range assetIDs {
		pgUUIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}

	existing, err := s.queries.GetStacksByAssetIDs(ctx, pgUUIDs)
	if err != nil {
		return nil, fmt.Errorf("check existing stacks: %w", err)
	}
	if len(existing) > 0 {
		return nil, ErrAssetAlreadyStacked
	}

	stackID, err := createStackRecord(ctx, s.pool, dbtypes.StackKindManual, nil)
	if err != nil {
		return nil, fmt.Errorf("create stack: %w", err)
	}

	for i, id := range assetIDs {
		pos := int32(i)
		err := s.queries.AddStackMember(ctx, repo.AddStackMemberParams{
			AssetID:  pgtype.UUID{Bytes: id, Valid: true},
			StackID:  stackID,
			Relation: repo.StackRelationAlternative,
			Position: &pos,
		})
		if err != nil {
			return nil, fmt.Errorf("add stack member %s: %w", id, err)
		}
	}

	return s.buildStackInfo(ctx, stackID)
}

// GetStackByAsset returns the stack containing the given asset.
func (s *stackService) GetStackByAsset(ctx context.Context, assetID uuid.UUID) (*StackInfo, error) {
	row, err := s.queries.GetStackByAssetID(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
	if err != nil {
		return nil, ErrStackNotFound
	}

	var stackUUID uuid.UUID
	if row.StackID.Valid {
		stackUUID = row.StackID.Bytes
	} else {
		return nil, ErrStackNotFound
	}

	return s.buildStackInfo(ctx, pgtype.UUID{Bytes: stackUUID, Valid: true})
}

// GetStacksByAssets returns stacks for multiple assets.
func (s *stackService) GetStacksByAssets(ctx context.Context, assetIDs []uuid.UUID) (map[uuid.UUID]*StackInfo, error) {
	pgUUIDs := make([]pgtype.UUID, len(assetIDs))
	for i, id := range assetIDs {
		pgUUIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}

	rows, err := s.queries.GetStacksByAssetIDs(ctx, pgUUIDs)
	if err != nil {
		return nil, fmt.Errorf("get stacks by asset ids: %w", err)
	}

	// Collect unique stack IDs
	stackSet := make(map[uuid.UUID]bool)
	assetToStack := make(map[uuid.UUID]uuid.UUID)
	for _, row := range rows {
		var assetID, stackID uuid.UUID
		if row.AssetID.Valid {
			assetID = row.AssetID.Bytes
		}
		if row.StackID.Valid {
			stackID = row.StackID.Bytes
		}
		if assetID != uuid.Nil && stackID != uuid.Nil {
			assetToStack[assetID] = stackID
			stackSet[stackID] = true
		}
	}

	// Build stack info for each unique stack
	result := make(map[uuid.UUID]*StackInfo)
	for stackID := range stackSet {
		info, err := s.buildStackInfo(ctx, pgtype.UUID{Bytes: stackID, Valid: true})
		if err != nil {
			s.logger.Warn("failed to build stack info", zap.String("stack_id", stackID.String()), zap.Error(err))
			continue
		}
		// Map each asset in this stack to the stack info
		for _, m := range info.Members {
			result[m.AssetID] = info
		}
	}

	return result, nil
}

// RemoveFromStack removes an asset from its stack.
func (s *stackService) RemoveFromStack(ctx context.Context, assetID uuid.UUID) error {
	return s.queries.RemoveStackMember(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
}

// DeleteStack deletes an entire stack, releasing all members.
func (s *stackService) DeleteStack(ctx context.Context, stackID uuid.UUID) error {
	return s.queries.DeleteStack(ctx, pgtype.UUID{Bytes: stackID, Valid: true})
}

func (s *stackService) MatchLivePhotoStack(ctx context.Context, assetID uuid.UUID) error {
	asset, err := s.queries.GetAssetByID(ctx, pgtype.UUID{Bytes: assetID, Valid: true})
	if err != nil {
		return fmt.Errorf("get asset for live photo matching: %w", err)
	}

	if asset.OwnerID == nil {
		return nil
	}

	contentIdentifier := normalizeLivePhotoContentIdentifier(livePhotoContentIdentifier(asset))
	if contentIdentifier == "" {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin live photo matching transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lockKey := fmt.Sprintf("%d:%s", *asset.OwnerID, contentIdentifier)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, lockKey); err != nil {
		return fmt.Errorf("acquire live photo advisory lock: %w", err)
	}

	// Exact Apple Live Photo matching: one PHOTO + one VIDEO for the same owner and content identifier.
	const exactMatchQuery = `WITH candidate_group AS (
		SELECT
			a.owner_id,
			a.specific_metadata->>'content_identifier' AS content_identifier,
			MIN(a.asset_id) FILTER (WHERE a.type = 'PHOTO') AS photo_asset_id,
			MIN(a.asset_id) FILTER (WHERE a.type = 'VIDEO') AS video_asset_id,
			COUNT(*) FILTER (WHERE a.type = 'PHOTO') AS photo_count,
			COUNT(*) FILTER (WHERE a.type = 'VIDEO') AS video_count
		FROM assets a
		WHERE a.owner_id = $1
		  AND a.is_deleted = false
		  AND a.type IN ('PHOTO', 'VIDEO')
		  AND a.specific_metadata->>'content_identifier' = $2
		GROUP BY a.owner_id, a.specific_metadata->>'content_identifier'
	)
	SELECT photo_asset_id, video_asset_id
	FROM candidate_group
	WHERE photo_count = 1
	  AND video_count = 1`

	var photoID, videoID pgtype.UUID
	if err := tx.QueryRow(ctx, exactMatchQuery, *asset.OwnerID, contentIdentifier).Scan(&photoID, &videoID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tx.Commit(ctx)
		}
		return fmt.Errorf("query live photo candidates: %w", err)
	}

	photoUUID, ok := uuidFromPgUUID(photoID)
	if !ok {
		return fmt.Errorf("live photo match returned invalid photo asset id")
	}
	videoUUID, ok := uuidFromPgUUID(videoID)
	if !ok {
		return fmt.Errorf("live photo match returned invalid video asset id")
	}

	// Idempotency: if either asset is already in a live photo stack, stop.
	const existingLivePhotoMembershipQuery = `SELECT 1
	FROM asset_stack_members m
	JOIN asset_stacks s ON s.stack_id = m.stack_id
	WHERE s.stack_kind = 'live_photo'
	  AND m.asset_id = ANY($1::uuid[])
	LIMIT 1`
	var sentinel int
	if err := tx.QueryRow(ctx, existingLivePhotoMembershipQuery, []uuid.UUID{photoUUID, videoUUID}).Scan(&sentinel); err == nil {
		return tx.Commit(ctx)
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check existing live photo membership: %w", err)
	}

	// Safety guard: do not move assets out of any other existing stack.
	const existingAnyMembershipQuery = `SELECT 1
	FROM asset_stack_members
	WHERE asset_id = ANY($1::uuid[])
	LIMIT 1`
	if err := tx.QueryRow(ctx, existingAnyMembershipQuery, []uuid.UUID{photoUUID, videoUUID}).Scan(&sentinel); err == nil {
		return tx.Commit(ctx)
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("check existing stack membership: %w", err)
	}

	stackID, err := createStackRecord(ctx, tx, dbtypes.StackKindLivePhoto, &contentIdentifier)
	if err != nil {
		return fmt.Errorf("create live photo stack: %w", err)
	}

	const insertMemberSQL = `INSERT INTO asset_stack_members (asset_id, stack_id, relation, position) VALUES ($1, $2, $3, $4)`
	photoPos := int32(0)
	if _, err := tx.Exec(ctx, insertMemberSQL, photoID, stackID, "live_photo_still", photoPos); err != nil {
		return fmt.Errorf("add live photo still member: %w", err)
	}

	videoPos := int32(1)
	if _, err := tx.Exec(ctx, insertMemberSQL, videoID, stackID, "live_photo_video", videoPos); err != nil {
		return fmt.Errorf("add live photo video member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit live photo stack: %w", err)
	}

	return nil
}

// buildStackInfo constructs a StackInfo from a stack ID.
func (s *stackService) buildStackInfo(ctx context.Context, stackID pgtype.UUID) (*StackInfo, error) {
	members, err := s.queries.GetStackMembers(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("get stack members: %w", err)
	}

	count, err := s.queries.GetStackMemberCount(ctx, stackID)
	if err != nil {
		return nil, fmt.Errorf("get stack member count: %w", err)
	}

	var stackUUID uuid.UUID
	if stackID.Valid {
		stackUUID = stackID.Bytes
	}

	info := &StackInfo{
		StackID:     stackUUID,
		MemberCount: count,
		Members:     make([]StackMemberInfo, 0, len(members)),
	}

	for _, m := range members {
		var assetUUID uuid.UUID
		if m.AssetID.Valid {
			assetUUID = m.AssetID.Bytes
		}
		pos := int32(0)
		if m.Position != nil {
			pos = *m.Position
		}
		info.Members = append(info.Members, StackMemberInfo{
			AssetID:  assetUUID,
			Relation: m.Relation,
			Position: pos,
		})
	}

	return info, nil
}

func createStackRecord(ctx context.Context, q stackRowQuerier, kind dbtypes.StackKind, groupKey *string) (pgtype.UUID, error) {
	var stackID pgtype.UUID
	if q == nil {
		return stackID, fmt.Errorf("stack inserter is nil")
	}

	query := `INSERT INTO asset_stacks (stack_kind, group_key) VALUES ($1, $2) RETURNING stack_id`
	if err := q.QueryRow(ctx, query, string(kind), groupKey).Scan(&stackID); err != nil {
		return stackID, err
	}

	return stackID, nil
}

type stackRowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func normalizeLivePhotoContentIdentifier(value string) string {
	return strings.TrimRight(value, "\x00")
}

func livePhotoContentIdentifier(asset repo.Asset) string {
	switch strings.ToUpper(strings.TrimSpace(asset.Type)) {
	case "PHOTO":
		meta, err := asset.SpecificMetadata.UnmarshalPhoto()
		if err != nil {
			return ""
		}
		return normalizeLivePhotoContentIdentifier(meta.ContentIdentifier)
	case "VIDEO":
		meta, err := asset.SpecificMetadata.UnmarshalVideo()
		if err != nil {
			return ""
		}
		return normalizeLivePhotoContentIdentifier(meta.ContentIdentifier)
	default:
		return ""
	}
}

func uuidPtr(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	converted := value.Bytes
	result := uuid.UUID(converted)
	return &result
}
