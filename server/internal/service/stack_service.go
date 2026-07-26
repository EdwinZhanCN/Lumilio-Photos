package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/utils/file"
	"server/internal/utils/raw"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var ErrStackNotFound = errors.New("stack not found")
var ErrAssetAlreadyStacked = errors.New("media item already belongs to a stack")

type StackInfo struct {
	StackID     uuid.UUID
	Kind        dbtypes.StackKind
	MemberCount int64
	Members     []StackMemberInfo
}

// StackMemberInfo is one logical media item in a presentation stack. AssetID
// is the item's primary component and is suitable for existing media routes.
type StackMemberInfo struct {
	MediaItemID uuid.UUID
	AssetID     uuid.UUID
	Position    int32
}

type MediaItemInfo struct {
	MediaItemID    uuid.UUID
	Kind           string
	PrimaryAssetID uuid.UUID
	Components     []MediaItemComponentInfo
}

type MediaItemComponentInfo struct {
	AssetID  uuid.UUID
	Relation repo.StackRelation
	Position int32
}

type StackService interface {
	// AutoDetectStacks first merges structural components (RAW/JPEG and edited
	// iterations) into logical media items, then creates burst presentation stacks.
	AutoDetectStacks(ctx context.Context, repositoryID uuid.UUID) (int, error)
	CreateManualStack(ctx context.Context, assetIDs []uuid.UUID) (*StackInfo, error)
	GetStackByAssetAny(ctx context.Context, assetID uuid.UUID, ownerID *int32) (*StackInfo, error)
	GetMediaItemByAsset(ctx context.Context, assetID uuid.UUID, ownerID *int32) (*MediaItemInfo, error)
	RemoveFromStack(ctx context.Context, assetID uuid.UUID) error
	DeleteStack(ctx context.Context, stackID uuid.UUID) error
	MatchLivePhotoStack(ctx context.Context, assetID uuid.UUID) error
}

type stackService struct {
	queries       *repo.Queries
	pool          *sql.DB
	logger        *zap.Logger
	auditProvider logging.RepositoryAuditProvider
}

func NewStackService(queries *repo.Queries, pool *sql.DB, logger *zap.Logger, auditProvider logging.RepositoryAuditProvider) StackService {
	return &stackService{queries: queries, pool: pool, logger: logger, auditProvider: auditProvider}
}

var fileValidator = file.NewValidator()

const stackMaxTimeGap = 5 * time.Second
const burstMaxTimeGap = time.Second

var iterationPattern = regexp.MustCompile(`^(.+?)-(\d+)$`)
var sequencePattern = regexp.MustCompile(`^(.*?)(\d+)$`)

func filenameStem(filename string) string {
	return strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
}

func iterationBaseName(filename string) (string, bool) {
	matches := iterationPattern.FindStringSubmatch(strings.TrimSuffix(filename, filepath.Ext(filename)))
	if matches == nil {
		return "", false
	}
	return strings.ToLower(matches[1]), true
}

func classifyRelation(filename string) repo.StackRelation {
	ext := strings.ToLower(filepath.Ext(filename))
	if _, isRaw := raw.RAWExtensions[ext]; isRaw || fileValidator.IsRAWFile(filename) {
		return repo.StackRelationRawOriginal
	}
	if ext == ".jpg" || ext == ".jpeg" {
		return repo.StackRelationJpegOriginal
	}
	if isIteration(filename) {
		return repo.StackRelationEditedVersion
	}
	return repo.StackRelationAlternative
}

func isIteration(filename string) bool {
	ext := filepath.Ext(filename)
	return iterationPattern.MatchString(strings.TrimSuffix(filename, ext))
}

func effectiveTime(taken, upload dbtypes.Timestamp) time.Time {
	if taken.Valid {
		return taken.Time
	}
	if upload.Valid {
		return upload.Time
	}
	return time.Time{}
}

type structuralCluster struct {
	BaseName             string
	Members              []repo.FindCandidatesForStackingByNameRow
	HasAnchoredIteration bool
}

func timeCluster(candidates []repo.FindCandidatesForStackingByNameRow) []structuralCluster {
	type key struct {
		owner int64
		name  string
	}
	// A numeric suffix is only an edit marker when the unsuffixed original is
	// present. This keeps ordinary camera/import sequences such as scan-001.jpg,
	// scan-002.jpg from collapsing into one logical media item.
	stems := make(map[key]struct{}, len(candidates))
	for _, candidate := range candidates {
		stems[key{owner: detectionOwnerKey(candidate.OwnerID), name: filenameStem(candidate.OriginalFilename)}] = struct{}{}
	}
	groups := make(map[key][]repo.FindCandidatesForStackingByNameRow)
	for _, candidate := range candidates {
		groupKey := key{owner: detectionOwnerKey(candidate.OwnerID), name: filenameStem(candidate.OriginalFilename)}
		if iterationBase, ok := iterationBaseName(candidate.OriginalFilename); ok {
			anchoredKey := key{owner: groupKey.owner, name: iterationBase}
			if _, anchored := stems[anchoredKey]; anchored {
				groupKey = anchoredKey
			}
		}
		groups[groupKey] = append(groups[groupKey], candidate)
	}

	var result []structuralCluster
	for groupKey, group := range groups {
		if len(group) < 2 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return effectiveTime(group[i].TakenTime, group[i].UploadTime).Before(effectiveTime(group[j].TakenTime, group[j].UploadTime))
		})
		start := 0
		for i := 1; i <= len(group); i++ {
			if i < len(group) && effectiveTime(group[i].TakenTime, group[i].UploadTime).Sub(effectiveTime(group[i-1].TakenTime, group[i-1].UploadTime)) <= stackMaxTimeGap {
				continue
			}
			if i-start >= 2 {
				members := append([]repo.FindCandidatesForStackingByNameRow(nil), group[start:i]...)
				hasAnchor, hasIteration := false, false
				for _, member := range members {
					hasAnchor = hasAnchor || filenameStem(member.OriginalFilename) == groupKey.name
					iterationBase, ok := iterationBaseName(member.OriginalFilename)
					hasIteration = hasIteration || (ok && iterationBase == groupKey.name)
				}
				result = append(result, structuralCluster{
					BaseName:             groupKey.name,
					Members:              members,
					HasAnchoredIteration: hasAnchor && hasIteration,
				})
			}
			start = i
		}
	}
	return result
}

func (s *stackService) AutoDetectStacks(ctx context.Context, repositoryID uuid.UUID) (int, error) {
	repositoryUUID := uuid.NullUUID{UUID: repositoryID, Valid: true}
	candidates, err := s.queries.FindCandidatesForStackingByName(ctx, repositoryUUID)
	if err != nil {
		return 0, fmt.Errorf("find structural media candidates: %w", err)
	}

	for _, cluster := range timeCluster(candidates) {
		hasRaw := false
		for _, candidate := range cluster.Members {
			if classifyRelation(candidate.OriginalFilename) == repo.StackRelationRawOriginal {
				hasRaw = true
			}
		}
		if !hasRaw && !cluster.HasAnchoredIteration {
			continue
		}
		if err := s.mergeStructuralMediaItem(ctx, cluster.BaseName, cluster.Members); err != nil {
			return 0, fmt.Errorf("merge structural media item %q: %w", cluster.BaseName, err)
		}
	}

	burstCandidates, err := s.queries.FindMediaItemsForBurstDetection(ctx, repositoryUUID)
	if err != nil {
		return 0, fmt.Errorf("find burst candidates: %w", err)
	}
	clusters := burstClusters(burstCandidates)
	created := 0
	for _, cluster := range clusters {
		wasCreated, err := s.persistBurstCluster(ctx, cluster)
		if err != nil {
			return created, fmt.Errorf("create burst stack %q: %w", cluster.GroupKey, err)
		}
		if wasCreated {
			created++
		}
	}
	return created, nil
}

func (s *stackService) mergeStructuralMediaItem(ctx context.Context, groupKey string, members []repo.FindCandidatesForStackingByNameRow) error {
	if len(members) < 2 {
		return nil
	}
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.queries.WithTx(tx)

	// Prefer JPEG for browsing, then the first member in capture order.
	primary := members[0]
	for _, member := range members {
		if classifyRelation(member.OriginalFilename) == repo.StackRelationJpegOriginal {
			primary = member
			break
		}
	}
	targetItemID := primary.MediaItemID
	seenSourceItems := make(map[uuid.UUID]struct{})
	allItemIDs := make([]uuid.UUID, 0, len(members))
	seenAllItems := make(map[uuid.UUID]bool)
	for _, member := range members {
		itemUUID := member.MediaItemID
		if !seenAllItems[itemUUID] {
			seenAllItems[itemUUID] = true
			allItemIDs = append(allItemIDs, member.MediaItemID)
		}
		if member.MediaItemID != targetItemID {
			seenSourceItems[member.MediaItemID] = struct{}{}
		}
	}
	// Structural components may arrive after one frame has already joined a
	// burst. Preserve a single shared presentation membership; never merge items
	// that already belong to different stacks.
	memberships, err := qtx.GetStackMembershipsByMediaItemIDs(ctx, allItemIDs)
	if err != nil {
		return err
	}
	if len(memberships) > 1 {
		return tx.Commit()
	}
	if len(memberships) == 1 {
		if err := qtx.RemoveStackMembershipsByMediaItemIDs(ctx, allItemIDs); err != nil {
			return err
		}
		if err := qtx.AddStackMember(ctx, repo.AddStackMemberParams{
			MediaItemID: targetItemID,
			StackID:     memberships[0].StackID,
			Position:    memberships[0].Position,
			CreatedAt:   dbtypes.NewTimestamp(time.Now()),
		}); err != nil {
			return err
		}
	}
	// Move every component from source items, not just the PHOTO candidates.
	// This preserves an already-associated Live Photo motion component.
	for sourceID := range seenSourceItems {
		if err := qtx.MoveAllMediaItemComponents(ctx, repo.MoveAllMediaItemComponentsParams{
			TargetMediaItemID: targetItemID,
			SourceMediaItemID: sourceID,
		}); err != nil {
			return err
		}
	}
	for index, member := range members {
		if err := qtx.MoveMediaItemComponent(ctx, repo.MoveMediaItemComponentParams{
			TargetMediaItemID: targetItemID,
			Relation:          string(classifyRelation(member.OriginalFilename)),
			Position:          int64(index),
			AssetID:           member.AssetID,
		}); err != nil {
			return err
		}
	}
	if err := qtx.UpdateMediaItemAfterStructuralMerge(ctx, repo.UpdateMediaItemAfterStructuralMergeParams{
		PrimaryAssetID:    uuid.NullUUID{UUID: primary.AssetID, Valid: true},
		TargetMediaItemID: targetItemID,
		GroupKey:          &groupKey,
		UpdatedAt:         dbtypes.NewTimestamp(time.Now()),
	}); err != nil {
		return err
	}
	for sourceID := range seenSourceItems {
		if err := qtx.DeleteMediaItem(ctx, sourceID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type burstCluster struct {
	GroupKey string
	Members  []repo.FindMediaItemsForBurstDetectionRow
}

func burstClusters(candidates []repo.FindMediaItemsForBurstDetectionRow) []burstCluster {
	consumed := make(map[uuid.UUID]bool)
	exact := make(map[string][]repo.FindMediaItemsForBurstDetectionRow)
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.BurstID) == "" {
			continue
		}
		key := fmt.Sprintf("%d:%s", detectionOwnerKey(candidate.OwnerID), strings.TrimSpace(candidate.BurstID))
		exact[key] = append(exact[key], candidate)
	}
	var result []burstCluster
	for key, group := range exact {
		sortBurstMembers(group)
		result = append(result, burstCluster{GroupKey: "exif:" + key, Members: group})
		for _, member := range group {
			consumed[member.MediaItemID] = true
		}
	}

	type fallbackKey struct {
		owner  int64
		camera string
		prefix string
	}
	fallback := make(map[fallbackKey][]repo.FindMediaItemsForBurstDetectionRow)
	for _, candidate := range candidates {
		if consumed[candidate.MediaItemID] || !candidate.TakenTime.Valid || strings.TrimSpace(candidate.CameraModel) == "" {
			continue
		}
		prefix, _, ok := filenameSequence(candidate.OriginalFilename)
		if !ok {
			continue
		}
		key := fallbackKey{owner: detectionOwnerKey(candidate.OwnerID), camera: strings.ToLower(candidate.CameraModel), prefix: prefix}
		fallback[key] = append(fallback[key], candidate)
	}
	for key, group := range fallback {
		sortBurstMembers(group)
		start := 0
		for i := 1; i <= len(group); i++ {
			continueCluster := false
			if i < len(group) {
				_, previousSequence, _ := filenameSequence(group[i-1].OriginalFilename)
				_, currentSequence, _ := filenameSequence(group[i].OriginalFilename)
				gap := group[i].TakenTime.Time.Sub(group[i-1].TakenTime.Time)
				continueCluster = gap >= 0 && gap <= burstMaxTimeGap && currentSequence == previousSequence+1
			}
			if continueCluster {
				continue
			}
			if i-start >= 3 {
				members := append([]repo.FindMediaItemsForBurstDetectionRow(nil), group[start:i]...)
				result = append(result, burstCluster{
					GroupKey: fmt.Sprintf("time:%d:%s:%s:%d", key.owner, key.camera, key.prefix, members[0].TakenTime.Time.UnixMilli()),
					Members:  members,
				})
			}
			start = i
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].GroupKey < result[j].GroupKey })
	return result
}

func sortBurstMembers(group []repo.FindMediaItemsForBurstDetectionRow) {
	sort.Slice(group, func(i, j int) bool {
		left, right := effectiveTime(group[i].TakenTime, group[i].UploadTime), effectiveTime(group[j].TakenTime, group[j].UploadTime)
		if left.Equal(right) {
			return group[i].OriginalFilename < group[j].OriginalFilename
		}
		return left.Before(right)
	})
}

func filenameSequence(filename string) (string, int64, bool) {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	matches := sequencePattern.FindStringSubmatch(strings.ToLower(name))
	if matches == nil || strings.TrimSpace(matches[1]) == "" {
		return "", 0, false
	}
	sequence, err := strconv.ParseInt(matches[2], 10, 64)
	return matches[1], sequence, err == nil
}

// persistBurstCluster creates a new burst or appends newly indexed frames to an
// existing EXIF-identified burst. Timestamp-only fallback groups are created
// atomically and never extended heuristically.
func (s *stackService) persistBurstCluster(ctx context.Context, cluster burstCluster) (bool, error) {
	if len(cluster.Members) == 0 {
		return false, nil
	}
	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.queries.WithTx(tx)

	existing, err := qtx.GetBurstStackByGroupKey(ctx, &cluster.GroupKey)
	if err == nil {
		nextPosition, err := qtx.GetNextStackPosition(ctx, existing.StackID)
		if err != nil {
			return false, err
		}
		for index, member := range cluster.Members {
			if err := qtx.AddStackMember(ctx, repo.AddStackMemberParams{
				MediaItemID: member.MediaItemID,
				StackID:     existing.StackID,
				Position:    nextPosition + int64(index),
				CreatedAt:   dbtypes.NewTimestamp(time.Now()),
			}); err != nil {
				return false, err
			}
		}
		return false, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	minimum := 3
	if strings.HasPrefix(cluster.GroupKey, "exif:") {
		minimum = 2
	}
	if len(cluster.Members) < minimum {
		return false, tx.Commit()
	}
	now := dbtypes.NewTimestamp(time.Now())
	stackID := uuid.New()
	if _, err := qtx.CreateAssetStack(ctx, repo.CreateAssetStackParams{
		StackID:          stackID,
		OwnerID:          cluster.Members[0].OwnerID,
		RepositoryID:     cluster.Members[0].RepositoryID,
		StackKind:        string(dbtypes.StackKindBurst),
		CoverMediaItemID: uuid.NullUUID{UUID: cluster.Members[0].MediaItemID, Valid: true},
		GroupKey:         &cluster.GroupKey,
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		return false, err
	}
	for index, member := range cluster.Members {
		if err := qtx.AddStackMember(ctx, repo.AddStackMemberParams{
			MediaItemID: member.MediaItemID,
			StackID:     stackID,
			Position:    int64(index),
			CreatedAt:   now,
		}); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func (s *stackService) CreateManualStack(ctx context.Context, assetIDs []uuid.UUID) (*StackInfo, error) {
	if len(assetIDs) < 2 {
		return nil, errors.New("at least 2 assets are required to create a stack")
	}
	items := make([]repo.MediaItem, 0, len(assetIDs))
	seen := make(map[uuid.UUID]bool)
	for _, assetID := range assetIDs {
		item, err := s.queries.GetMediaItemByAssetID(ctx, assetID)
		if err != nil {
			return nil, fmt.Errorf("resolve media item for %s: %w", assetID, err)
		}
		id := item.MediaItemID
		if !seen[id] {
			seen[id] = true
			items = append(items, item)
		}
	}
	if len(items) < 2 {
		return nil, errors.New("selected assets resolve to fewer than 2 media items")
	}
	if existing, err := s.queries.GetStacksByAssetIDs(ctx, assetIDs); err != nil {
		return nil, err
	} else if len(existing) > 0 {
		return nil, ErrAssetAlreadyStacked
	}

	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.queries.WithTx(tx)
	stackID := uuid.New()
	now := dbtypes.NewTimestamp(time.Now())
	if _, err := qtx.CreateAssetStack(ctx, repo.CreateAssetStackParams{
		StackID:          stackID,
		OwnerID:          items[0].OwnerID,
		RepositoryID:     items[0].RepositoryID,
		StackKind:        string(dbtypes.StackKindManual),
		CoverMediaItemID: uuid.NullUUID{UUID: items[0].MediaItemID, Valid: true},
		CreatedAt:        now,
		UpdatedAt:        now,
	}); err != nil {
		return nil, err
	}
	for index, item := range items {
		if err := qtx.AddStackMember(ctx, repo.AddStackMemberParams{
			MediaItemID: item.MediaItemID,
			StackID:     stackID,
			Position:    int64(index),
			CreatedAt:   now,
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.buildStackInfo(ctx, stackID, false, nil)
}

func (s *stackService) GetStackByAssetAny(ctx context.Context, assetID uuid.UUID, ownerID *int32) (*StackInfo, error) {
	row, err := s.queries.GetStackByAssetID(ctx, assetID)
	if err != nil {
		return nil, ErrStackNotFound
	}
	return s.buildStackInfo(ctx, row.StackID, true, ownerID)
}

func (s *stackService) GetMediaItemByAsset(ctx context.Context, assetID uuid.UUID, ownerID *int32) (*MediaItemInfo, error) {
	item, err := s.queries.GetMediaItemByAssetID(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("get media item: %w", err)
	}
	components, err := s.queries.GetMediaItemComponents(ctx, repo.GetMediaItemComponentsParams{
		MediaItemID: item.MediaItemID,
		OwnerID:     ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("get media item components: %w", err)
	}
	info := &MediaItemInfo{
		MediaItemID:    item.MediaItemID,
		Kind:           item.MediaKind,
		PrimaryAssetID: item.PrimaryAssetID.UUID,
		Components:     make([]MediaItemComponentInfo, 0, len(components)),
	}
	for _, component := range components {
		info.Components = append(info.Components, MediaItemComponentInfo{
			AssetID:  component.AssetID,
			Relation: repo.StackRelation(component.Relation),
			Position: int32(component.Position),
		})
	}
	return info, nil
}

func (s *stackService) RemoveFromStack(ctx context.Context, assetID uuid.UUID) error {
	return s.queries.RemoveStackMemberByAssetID(ctx, assetID)
}

func (s *stackService) DeleteStack(ctx context.Context, stackID uuid.UUID) error {
	return s.queries.DeleteStack(ctx, stackID)
}

func (s *stackService) MatchLivePhotoStack(ctx context.Context, assetID uuid.UUID) error {
	asset, err := s.queries.GetAssetByID(ctx, assetID)
	if err != nil {
		return fmt.Errorf("get asset for live photo matching: %w", err)
	}
	if asset.OwnerID == nil {
		return nil
	}
	identifier := normalizeLivePhotoContentIdentifier(livePhotoContentIdentifier(asset))
	if identifier == "" {
		return nil
	}

	tx, err := s.pool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.queries.WithTx(tx)

	rows, err := qtx.FindLivePhotoPair(ctx, repo.FindLivePhotoPairParams{
		OwnerID:           asset.OwnerID,
		ContentIdentifier: identifier,
	})
	if err != nil {
		return err
	}
	var photoID, videoID uuid.UUID
	for _, row := range rows {
		switch row.Type {
		case string(dbtypes.AssetTypePhoto):
			if photoID != uuid.Nil {
				return tx.Commit()
			}
			photoID = row.AssetID
		case string(dbtypes.AssetTypeVideo):
			if videoID != uuid.Nil {
				return tx.Commit()
			}
			videoID = row.AssetID
		}
	}
	if photoID == uuid.Nil || videoID == uuid.Nil {
		return tx.Commit()
	}

	photoItem, err := qtx.GetMediaItemByAssetID(ctx, photoID)
	if err != nil {
		return err
	}
	videoItem, err := qtx.GetMediaItemByAssetID(ctx, videoID)
	if err != nil {
		return err
	}
	photoItemID := photoItem.MediaItemID
	videoItemID := videoItem.MediaItemID
	if photoItemID == videoItemID {
		return tx.Commit()
	}
	// A structural merge may preserve an existing presentation membership on
	// the still item, but never combines two independently stacked items.
	stackedItems, err := qtx.CountStackedMediaItems(ctx, []uuid.UUID{photoItemID, videoItemID})
	if err != nil {
		return err
	}
	if stackedItems > 1 {
		return tx.Commit()
	}
	if err := qtx.MoveMediaItemComponent(ctx, repo.MoveMediaItemComponentParams{
		TargetMediaItemID: photoItemID,
		Relation:          string(repo.StackRelationLivePhotoStill),
		Position:          0,
		AssetID:           photoID,
	}); err != nil {
		return err
	}
	if err := qtx.MoveMediaItemComponent(ctx, repo.MoveMediaItemComponentParams{
		TargetMediaItemID: photoItemID,
		Relation:          string(repo.StackRelationLivePhotoVideo),
		Position:          1,
		AssetID:           videoID,
	}); err != nil {
		return err
	}
	if err := qtx.UpdateMediaItemAsLivePhoto(ctx, repo.UpdateMediaItemAsLivePhotoParams{
		PrimaryAssetID: uuid.NullUUID{UUID: photoID, Valid: true},
		GroupKey:       &identifier,
		UpdatedAt:      dbtypes.NewTimestamp(time.Now()),
		MediaItemID:    photoItemID,
	}); err != nil {
		return err
	}
	if err := qtx.DeleteMediaItem(ctx, videoItemID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *stackService) buildStackInfo(ctx context.Context, stackID uuid.UUID, includeDeleted bool, ownerID *int32) (*StackInfo, error) {
	kinds, err := s.queries.GetStackKindsByIDs(ctx, []uuid.UUID{stackID})
	if err != nil || len(kinds) != 1 {
		return nil, ErrStackNotFound
	}
	info := &StackInfo{StackID: stackID, Kind: dbtypes.StackKind(kinds[0].StackKind), Members: []StackMemberInfo{}}
	appendMember := func(mediaItemID uuid.UUID, assetID uuid.NullUUID, position int64) {
		info.Members = append(info.Members, StackMemberInfo{
			MediaItemID: mediaItemID,
			AssetID:     assetID.UUID,
			Position:    int32(position),
		})
	}
	if includeDeleted {
		members, err := s.queries.GetStackMembersAny(ctx, repo.GetStackMembersAnyParams{StackID: stackID, OwnerID: ownerID})
		if err != nil {
			return nil, err
		}
		for _, member := range members {
			appendMember(member.MediaItemID, member.AssetID, member.Position)
		}
	} else {
		members, err := s.queries.GetStackMembers(ctx, repo.GetStackMembersParams{StackID: stackID, OwnerID: ownerID})
		if err != nil {
			return nil, err
		}
		for _, member := range members {
			appendMember(member.MediaItemID, member.AssetID, member.Position)
		}
	}
	info.MemberCount = int64(len(info.Members))
	return info, nil
}

func normalizeLivePhotoContentIdentifier(value string) string {
	return strings.TrimRight(value, "\x00")
}

func livePhotoContentIdentifier(asset repo.Asset) string {
	switch strings.ToUpper(strings.TrimSpace(asset.Type)) {
	case "PHOTO":
		meta, err := asset.SpecificMetadata.UnmarshalPhoto()
		if err == nil {
			return normalizeLivePhotoContentIdentifier(meta.ContentIdentifier)
		}
	case "VIDEO":
		meta, err := asset.SpecificMetadata.UnmarshalVideo()
		if err == nil {
			return normalizeLivePhotoContentIdentifier(meta.ContentIdentifier)
		}
	}
	return ""
}
