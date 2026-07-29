package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPaused        = errors.New("event rebuild paused")
	ErrStaleRevision = errors.New("event rebuild revision changed")
	ErrWouldBeEmpty  = errors.New("event would be empty")
)

type Service struct {
	db       *sql.DB
	resolver *Resolver
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, resolver: NewResolver(db)}
}

func (s *Service) Resolver() *Resolver { return s.resolver }

// InitializeBackfill records events-v1 exactly once per owner and creates a
// factual initial range only when that owner has candidate media.
func (s *Service) InitializeBackfill(ctx context.Context) ([]int32, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM users ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	var owners []int32
	for rows.Next() {
		var ownerID int32
		if err := rows.Scan(&ownerID); err != nil {
			rows.Close()
			return nil, err
		}
		owners = append(owners, ownerID)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var queued []int32
	for _, ownerID := range owners {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM event_owner_state WHERE owner_id=?`, ownerID).Scan(&exists); err != nil {
			tx.Rollback()
			return nil, err
		}
		if exists != 0 {
			tx.Rollback()
			continue
		}
		var start, end sql.NullInt64
		if err := tx.QueryRowContext(ctx, `
SELECT min(COALESCE(a.taken_time,a.upload_time,mi.created_at)),
       max(COALESCE(a.taken_time,a.upload_time,mi.created_at))
FROM media_items mi LEFT JOIN assets a ON a.asset_id=mi.primary_asset_id
WHERE mi.owner_id=?`, ownerID).Scan(&start, &end); err != nil {
			tx.Rollback()
			return nil, err
		}
		now := time.Now().UTC().UnixMicro()
		if _, err := tx.ExecContext(ctx, `
INSERT INTO event_owner_state(owner_id,active_algorithm_version,initialized_at,revision,updated_at)
VALUES(?,?,?,0,?)`, ownerID, AlgorithmVersion, now, now); err != nil {
			tx.Rollback()
			return nil, err
		}
		if start.Valid && end.Valid {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO event_dirty_ranges(dirty_range_id,owner_id,range_start,range_end,reason,created_at)
VALUES(?,?,?,?,?,?)`, uuid.NewString(), ownerID, start.Int64, end.Int64, "initial_backfill", now); err != nil {
				tx.Rollback()
				return nil, err
			}
			queued = append(queued, ownerID)
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
	}
	return queued, nil
}

type RebuildPreview struct {
	Retained   int
	Created    int
	Redirected int
	Events     int
	Members    int
}

func (s *Service) RebuildOwner(ctx context.Context, ownerID int32, dryRun bool) (RebuildPreview, error) {
	return s.rebuildOwner(ctx, ownerID, dryRun, nil)
}

func (s *Service) RebuildClaimed(ctx context.Context, ownerID int32, expectedRevision int64) (RebuildPreview, error) {
	return s.rebuildOwner(ctx, ownerID, false, &expectedRevision)
}

func (s *Service) rebuildOwner(ctx context.Context, ownerID int32, dryRun bool, expectedRevision *int64) (RebuildPreview, error) {
	candidates, err := s.loadCandidates(ctx, ownerID)
	if err != nil {
		return RebuildPreview{}, err
	}
	constraints, err := s.loadConstraints(ctx, ownerID)
	if err != nil {
		return RebuildPreview{}, err
	}
	segments, err := SegmentCandidates(candidates, constraints, V1)
	if err != nil {
		return RebuildPreview{}, err
	}
	old, err := s.loadPublished(ctx, ownerID)
	if err != nil {
		return RebuildPreview{}, err
	}
	reconciled, err := Reconcile(old, segments, constraints, V1, func() string {
		return uuid.NewString()
	})
	if err != nil {
		return RebuildPreview{}, err
	}
	preview := RebuildPreview{Events: len(segments), Redirected: len(reconciled.Redirects)}
	for _, assignment := range reconciled.Assignments {
		preview.Members += len(segments[assignment.SegmentIndex].MediaItemIDs)
		if assignment.Reused {
			preview.Retained++
		} else {
			preview.Created++
		}
	}
	if dryRun {
		return preview, nil
	}
	if err := s.publish(ctx, ownerID, segments, old, reconciled, expectedRevision); err != nil {
		return RebuildPreview{}, err
	}
	return preview, nil
}

func (s *Service) loadCandidates(ctx context.Context, ownerID int32) ([]Candidate, error) {
	const query = `
SELECT mi.media_item_id,
       COALESCE(capture.taken_time, upload.upload_time, mi.created_at),
       CASE WHEN capture.taken_time IS NOT NULL THEN 'taken_time'
            WHEN upload.upload_time IS NOT NULL THEN 'upload_time' ELSE 'created_at' END,
       CASE WHEN capture.taken_time IS NOT NULL THEN capture.capture_offset_minutes END,
       COALESCE(primary_asset.gps_latitude, component.gps_latitude),
       COALESCE(primary_asset.gps_longitude, component.gps_longitude),
       asm.stack_id
FROM media_items mi
LEFT JOIN assets primary_asset ON primary_asset.asset_id = mi.primary_asset_id
LEFT JOIN asset_stack_members asm ON asm.media_item_id = mi.media_item_id
LEFT JOIN assets capture ON capture.asset_id = (
 SELECT mia.asset_id FROM media_item_assets mia JOIN assets a ON a.asset_id=mia.asset_id
 WHERE mia.media_item_id=mi.media_item_id AND a.taken_time IS NOT NULL
 ORDER BY a.taken_time, a.asset_id LIMIT 1)
LEFT JOIN assets upload ON upload.asset_id = (
 SELECT mia.asset_id FROM media_item_assets mia JOIN assets a ON a.asset_id=mia.asset_id
 WHERE mia.media_item_id=mi.media_item_id ORDER BY a.upload_time, a.asset_id LIMIT 1)
LEFT JOIN assets component ON component.asset_id = (
 SELECT mia.asset_id FROM media_item_assets mia JOIN assets a ON a.asset_id=mia.asset_id
 WHERE mia.media_item_id=mi.media_item_id AND a.gps_latitude IS NOT NULL AND a.gps_longitude IS NOT NULL
 ORDER BY CASE mia.relation WHEN 'live_photo_still' THEN 0 WHEN 'jpeg_original' THEN 1
 WHEN 'raw_original' THEN 2 WHEN 'original' THEN 3 WHEN 'edited_version' THEN 4
 WHEN 'alternative' THEN 5 WHEN 'component' THEN 6 WHEN 'live_photo_video' THEN 7 ELSE 8 END,
 mia.position, mia.asset_id LIMIT 1)
WHERE mi.owner_id=? ORDER BY 2, mi.media_item_id`
	rows, err := s.db.QueryContext(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("load Event candidates: %w", err)
	}
	defer rows.Close()
	var result []Candidate
	for rows.Next() {
		var id, source string
		var micros int64
		var offset sql.NullInt64
		var lat, lon sql.NullFloat64
		var stack sql.NullString
		if err := rows.Scan(&id, &micros, &source, &offset, &lat, &lon, &stack); err != nil {
			return nil, err
		}
		item := Candidate{MediaItemID: id, CapturedAt: time.UnixMicro(micros).UTC(), TimeSource: source}
		if offset.Valid {
			sign := "+"
			value := offset.Int64
			if value < 0 {
				sign, value = "-", -value
			}
			item.Timezone = fmt.Sprintf("%s%02d:%02d", sign, value/60, value%60)
		}
		if lat.Valid && lon.Valid {
			item.Coordinate = &Coordinate{Latitude: lat.Float64, Longitude: lon.Float64}
		}
		if stack.Valid {
			item.StackID = stack.String
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) loadConstraints(ctx context.Context, ownerID int32) ([]Constraint, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT kind, event_id, left_media_item_id, right_media_item_id
FROM event_constraints WHERE owner_id=?
ORDER BY kind, event_id, left_media_item_id, right_media_item_id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Constraint
	for rows.Next() {
		var kind string
		var eventID, right sql.NullString
		var left string
		if err := rows.Scan(&kind, &eventID, &left, &right); err != nil {
			return nil, err
		}
		result = append(result, Constraint{
			Kind: ConstraintKind(kind), EventID: eventID.String,
			LeftMediaItemID: left, RightMediaItemID: right.String,
		})
	}
	return result, rows.Err()
}

func (s *Service) loadPublished(ctx context.Context, ownerID int32) ([]PublishedEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT e.event_id, e.start_at, e.end_at, e.cover_override_media_item_id,
       e.title_override, e.is_hidden, e.created_at,
       emi.media_item_id
FROM events e
LEFT JOIN event_media_items emi ON emi.event_id=e.event_id AND emi.owner_id=e.owner_id
WHERE e.owner_id=? AND e.status='active'
ORDER BY e.created_at, e.event_id, emi.position, emi.media_item_id`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PublishedEvent
	index := map[string]int{}
	for rows.Next() {
		var id string
		var start, end, created int64
		var cover, title, member sql.NullString
		var hidden bool
		if err := rows.Scan(&id, &start, &end, &cover, &title, &hidden, &created, &member); err != nil {
			return nil, err
		}
		i, ok := index[id]
		if !ok {
			i = len(result)
			index[id] = i
			result = append(result, PublishedEvent{
				EventID: id, StartAt: time.UnixMicro(start), EndAt: time.UnixMicro(end),
				CoverOverrideID: cover.String, Hidden: hidden,
				HasUserState: cover.Valid || title.Valid || hidden, CreatedAt: time.UnixMicro(created),
			})
			if title.Valid {
				value := title.String
				result[i].TitleOverride = &value
			}
		}
		if member.Valid {
			result[i].MediaItemIDs = append(result[i].MediaItemIDs, member.String)
		}
	}
	return result, rows.Err()
}

func (s *Service) publish(ctx context.Context, ownerID int32, segments []Segment, old []PublishedEvent, result ReconcileResult, expectedRevision *int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().UnixMicro()
	if expectedRevision != nil {
		var revision int64
		var paused int
		if err := tx.QueryRowContext(ctx, `
SELECT revision,automatic_rebuild_paused FROM event_owner_state WHERE owner_id=?`,
			ownerID).Scan(&revision, &paused); err != nil {
			return err
		}
		if paused != 0 {
			return ErrPaused
		}
		if revision != *expectedRevision {
			return ErrStaleRevision
		}
	}
	for _, assignment := range result.Assignments {
		segment := segments[assignment.SegmentIndex]
		if assignment.Reused {
			if _, err := tx.ExecContext(ctx, `DELETE FROM event_media_items WHERE event_id=? AND owner_id=?`, assignment.EventID, ownerID); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO events(event_id,owner_id,status,start_at,end_at,timezone,is_hidden,algorithm_version,created_at,updated_at)
VALUES(?,?,'active',?,?,?,?,?, ?,?)`,
				assignment.EventID, ownerID, segment.StartAt.UnixMicro(), segment.EndAt.UnixMicro(),
				nullString(segment.Timezone), false, AlgorithmVersion, now, now); err != nil {
				return err
			}
		}
		evidence, _ := json.Marshal(map[string]any{"algorithm_version": AlgorithmVersion})
		for position, mediaID := range segment.MediaItemIDs {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO event_media_items(event_id,owner_id,media_item_id,position,source,evidence,derivation_run_id,created_at)
VALUES(?,?,?,?,'automatic',?,?,?)`,
				assignment.EventID, ownerID, mediaID, position, string(evidence), uuid.NewString(), now); err != nil {
				return err
			}
		}
		cover := any(nil)
		if len(segment.MediaItemIDs) > 0 {
			cover = segment.MediaItemIDs[0]
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE events SET start_at=?,end_at=?,timezone=?,generated_title=NULL,
 generated_cover_media_item_id=?,algorithm_version=?,updated_at=?
WHERE event_id=? AND owner_id=? AND status='active'`,
			segment.StartAt.UnixMicro(), segment.EndAt.UnixMicro(), nullString(segment.Timezone),
			cover, AlgorithmVersion, now, assignment.EventID, ownerID); err != nil {
			return err
		}
	}
	for _, redirect := range result.Redirects {
		if _, err := tx.ExecContext(ctx, `UPDATE event_redirects SET new_event_id=? WHERE owner_id=? AND new_event_id=?`,
			redirect.NewEventID, ownerID, redirect.OldEventID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM event_media_items WHERE event_id=? AND owner_id=?`, redirect.OldEventID, ownerID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE events SET generated_cover_media_item_id=NULL,cover_override_media_item_id=NULL,updated_at=?
WHERE event_id=? AND owner_id=?`, now, redirect.OldEventID, ownerID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE events SET status='redirected',updated_at=? WHERE event_id=? AND owner_id=?`,
			now, redirect.OldEventID, ownerID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO event_redirects(old_event_id,owner_id,new_event_id,reason,created_at)
VALUES(?,?,?,'automatic_reconciliation',?)
ON CONFLICT(old_event_id) DO UPDATE SET new_event_id=excluded.new_event_id,reason=excluded.reason`,
			redirect.OldEventID, ownerID, redirect.NewEventID, now); err != nil {
			return err
		}
	}
	var empty int
	if err := tx.QueryRowContext(ctx, `
SELECT count(*) FROM events e
WHERE e.owner_id=? AND e.status='active'
AND NOT EXISTS(SELECT 1 FROM event_media_items emi WHERE emi.event_id=e.event_id)`, ownerID).Scan(&empty); err != nil {
		return err
	}
	if empty != 0 {
		return ErrWouldBeEmpty
	}
	stateResult, err := tx.ExecContext(ctx, `
INSERT INTO event_owner_state(owner_id,active_algorithm_version,initialized_at,last_full_rebuild_at,revision,updated_at)
VALUES(?,?,?, ?,1,?)
ON CONFLICT(owner_id) DO UPDATE SET last_full_rebuild_at=excluded.last_full_rebuild_at,
 revision=event_owner_state.revision+1,updated_at=excluded.updated_at
 WHERE ? IS NULL OR event_owner_state.revision=?`,
		ownerID, AlgorithmVersion, now, now, now, expectedRevision, expectedRevision)
	if err != nil {
		return err
	}
	if expectedRevision != nil {
		if affected, _ := stateResult.RowsAffected(); affected != 1 {
			return ErrStaleRevision
		}
	}
	return tx.Commit()
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (s *Service) Merge(ctx context.Context, ownerID int32, eventIDs []string, survivorID string) (Summary, error) {
	if len(eventIDs) < 2 || !contains(eventIDs, survivorID) {
		return Summary{}, ErrConstraintConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Summary{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().UnixMicro()
	position := 0
	var bridgeLeft string
	for _, eventID := range eventIDs {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM events WHERE event_id=? AND owner_id=?`, eventID, ownerID).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Summary{}, ErrNotFound
			}
			return Summary{}, err
		}
		rows, err := tx.QueryContext(ctx, `
SELECT media_item_id FROM event_media_items WHERE event_id=? AND owner_id=?
ORDER BY position,media_item_id`, eventID, ownerID)
		if err != nil {
			return Summary{}, err
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return Summary{}, err
			}
			ids = append(ids, id)
		}
		rows.Close()
		if eventID != survivorID {
			if _, err := tx.ExecContext(ctx, `DELETE FROM event_media_items WHERE event_id=? AND owner_id=?`, eventID, ownerID); err != nil {
				return Summary{}, err
			}
		}
		for _, mediaID := range ids {
			if eventID != survivorID {
				if _, err := tx.ExecContext(ctx, `
INSERT INTO event_media_items(event_id,owner_id,media_item_id,position,source,evidence,created_at)
VALUES(?,?,?,?,'user','{}',?)`, survivorID, ownerID, mediaID, position, now); err != nil {
					return Summary{}, err
				}
			}
			if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO event_constraints(
 constraint_id,owner_id,kind,event_id,left_media_item_id,created_at,updated_at
) VALUES(?,?,'include',?,?,?,?)`,
				uuid.NewString(), ownerID, survivorID, mediaID, now, now); err != nil {
				return Summary{}, err
			}
			if bridgeLeft != "" {
				left, right := canonicalPair(bridgeLeft, mediaID)
				if _, err := tx.ExecContext(ctx, `
DELETE FROM event_constraints
WHERE owner_id=? AND kind='cannot_link'
  AND left_media_item_id=? AND right_media_item_id=?`,
					ownerID, left, right); err != nil {
					return Summary{}, err
				}
				if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO event_constraints(
 constraint_id,owner_id,kind,left_media_item_id,right_media_item_id,created_at,updated_at
) VALUES(?,?,'must_link',?,?,?,?)`,
					uuid.NewString(), ownerID, left, right, now, now); err != nil {
					return Summary{}, err
				}
				bridgeLeft = ""
			}
			position++
		}
		if len(ids) > 0 {
			bridgeLeft = ids[len(ids)-1]
		}
		if eventID != survivorID {
			if _, err := tx.ExecContext(ctx, `
UPDATE event_constraints SET event_id=? WHERE owner_id=? AND event_id=?`, survivorID, ownerID, eventID); err != nil {
				return Summary{}, err
			}
			if _, err := tx.ExecContext(ctx, `
UPDATE events SET generated_cover_media_item_id=NULL,cover_override_media_item_id=NULL,updated_at=?
WHERE event_id=? AND owner_id=?`, now, eventID, ownerID); err != nil {
				return Summary{}, err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE events SET status='redirected',updated_at=? WHERE event_id=? AND owner_id=?`, now, eventID, ownerID); err != nil {
				return Summary{}, err
			}
			if _, err := tx.ExecContext(ctx, `
INSERT INTO event_redirects(old_event_id,owner_id,new_event_id,reason,created_at)
VALUES(?,?,?,'manual_merge',?)`, eventID, ownerID, survivorID, now); err != nil {
				return Summary{}, err
			}
		}
	}
	if err := repairEventTx(ctx, tx, ownerID, survivorID, now); err != nil {
		return Summary{}, err
	}
	if err := markDirtyTx(ctx, tx, ownerID, survivorID, "manual_merge", now); err != nil {
		return Summary{}, err
	}
	if err := tx.Commit(); err != nil {
		return Summary{}, err
	}
	return s.resolver.Resolve(ctx, ownerID, survivorID)
}

func (s *Service) Split(ctx context.Context, ownerID int32, eventID, beforeMediaID string) (Summary, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Summary{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
SELECT media_item_id FROM event_media_items WHERE event_id=? AND owner_id=?
ORDER BY position,media_item_id`, eventID, ownerID)
	if err != nil {
		return Summary{}, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return Summary{}, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	splitAt := -1
	for i, id := range ids {
		if id == beforeMediaID {
			splitAt = i
		}
	}
	if splitAt <= 0 || splitAt >= len(ids) {
		return Summary{}, ErrConstraintConflict
	}
	now := time.Now().UTC().UnixMicro()
	newID := uuid.NewString()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO events(event_id,owner_id,status,start_at,end_at,is_hidden,algorithm_version,created_at,updated_at)
SELECT ?,owner_id,'active',start_at,end_at,0,algorithm_version,?,?
FROM events WHERE event_id=? AND owner_id=? AND status='active'`,
		newID, now, now, eventID, ownerID); err != nil {
		return Summary{}, err
	}
	for position, mediaID := range ids[splitAt:] {
		if _, err := tx.ExecContext(ctx, `
UPDATE event_media_items SET event_id=?,position=?,source='user'
WHERE event_id=? AND owner_id=? AND media_item_id=?`,
			newID, position, eventID, ownerID, mediaID); err != nil {
			return Summary{}, err
		}
	}
	for position, mediaID := range ids[:splitAt] {
		if _, err := tx.ExecContext(ctx, `
UPDATE event_media_items SET position=? WHERE event_id=? AND owner_id=? AND media_item_id=?`,
			position, eventID, ownerID, mediaID); err != nil {
			return Summary{}, err
		}
	}
	left, right := canonicalPair(ids[splitAt-1], ids[splitAt])
	if _, err := tx.ExecContext(ctx, `
INSERT INTO event_constraints(
 constraint_id,owner_id,kind,left_media_item_id,right_media_item_id,created_at,updated_at
) VALUES(?,?,'cannot_link',?,?,?,?)`, uuid.NewString(), ownerID, left, right, now, now); err != nil {
		return Summary{}, err
	}
	for _, pair := range []struct {
		eventID string
		ids     []string
	}{{eventID, ids[:splitAt]}, {newID, ids[splitAt:]}} {
		for _, mediaID := range pair.ids {
			if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO event_constraints(
 constraint_id,owner_id,kind,event_id,left_media_item_id,created_at,updated_at
) VALUES(?,?,'include',?,?,?,?)`, uuid.NewString(), ownerID, pair.eventID, mediaID, now, now); err != nil {
				return Summary{}, err
			}
		}
		if err := repairEventTx(ctx, tx, ownerID, pair.eventID, now); err != nil {
			return Summary{}, err
		}
		if err := markDirtyTx(ctx, tx, ownerID, pair.eventID, "manual_split", now); err != nil {
			return Summary{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Summary{}, err
	}
	return s.resolver.Resolve(ctx, ownerID, eventID)
}

func (s *Service) RemoveMember(ctx context.Context, ownerID int32, eventID, mediaID string) (Summary, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Summary{}, err
	}
	defer tx.Rollback()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM event_media_items WHERE event_id=? AND owner_id=?`, eventID, ownerID).Scan(&count); err != nil {
		return Summary{}, err
	}
	if count <= 1 {
		return Summary{}, ErrWouldBeEmpty
	}
	now := time.Now().UTC().UnixMicro()
	result, err := tx.ExecContext(ctx, `DELETE FROM event_media_items WHERE event_id=? AND owner_id=? AND media_item_id=?`, eventID, ownerID, mediaID)
	if err != nil {
		return Summary{}, err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return Summary{}, ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM event_constraints WHERE owner_id=? AND kind='include' AND left_media_item_id=?`, ownerID, mediaID); err != nil {
		return Summary{}, err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO event_constraints(
 constraint_id,owner_id,kind,event_id,left_media_item_id,created_at,updated_at
) VALUES(?,?,'exclude',?,?,?,?)`, uuid.NewString(), ownerID, eventID, mediaID, now, now); err != nil {
		return Summary{}, err
	}
	if err := repairEventTx(ctx, tx, ownerID, eventID, now); err != nil {
		return Summary{}, err
	}
	if err := markDirtyTx(ctx, tx, ownerID, eventID, "member_removed", now); err != nil {
		return Summary{}, err
	}
	if err := tx.Commit(); err != nil {
		return Summary{}, err
	}
	return s.resolver.Resolve(ctx, ownerID, eventID)
}

func (s *Service) AddAssets(ctx context.Context, ownerID int32, eventID string, assetIDs []string) (Summary, error) {
	if len(assetIDs) == 0 {
		return Summary{}, ErrConstraintConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Summary{}, err
	}
	defer tx.Rollback()
	var targetStatus string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM events WHERE event_id=? AND owner_id=?`, eventID, ownerID).Scan(&targetStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Summary{}, ErrNotFound
		}
		return Summary{}, err
	}
	if targetStatus != "active" {
		return Summary{}, ErrNotFound
	}
	var position int
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(max(position)+1,0) FROM event_media_items WHERE event_id=? AND owner_id=?`,
		eventID, ownerID).Scan(&position); err != nil {
		return Summary{}, err
	}
	now := time.Now().UTC().UnixMicro()
	for _, assetID := range assetIDs {
		var mediaID string
		if err := tx.QueryRowContext(ctx, `
SELECT mi.media_item_id FROM media_items mi
JOIN media_item_assets mia ON mia.media_item_id=mi.media_item_id
WHERE mia.asset_id=? AND mi.owner_id=?`, assetID, ownerID).Scan(&mediaID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return Summary{}, ErrNotFound
			}
			return Summary{}, err
		}
		var sourceEvent sql.NullString
		_ = tx.QueryRowContext(ctx, `
SELECT event_id FROM event_media_items WHERE media_item_id=? AND owner_id=?`, mediaID, ownerID).Scan(&sourceEvent)
		if sourceEvent.Valid && sourceEvent.String != eventID {
			var sourceCount int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM event_media_items WHERE event_id=? AND owner_id=?`, sourceEvent.String, ownerID).Scan(&sourceCount); err != nil {
				return Summary{}, err
			}
			if sourceCount <= 1 {
				return Summary{}, ErrWouldBeEmpty
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM event_media_items WHERE event_id=? AND owner_id=? AND media_item_id=?`, sourceEvent.String, ownerID, mediaID); err != nil {
				return Summary{}, err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM event_constraints WHERE owner_id=? AND kind='include' AND left_media_item_id=?`, ownerID, mediaID); err != nil {
				return Summary{}, err
			}
			if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO event_constraints(
 constraint_id,owner_id,kind,event_id,left_media_item_id,created_at,updated_at
) VALUES(?,?,'exclude',?,?,?,?)`, uuid.NewString(), ownerID, sourceEvent.String, mediaID, now, now); err != nil {
				return Summary{}, err
			}
			if err := repairEventTx(ctx, tx, ownerID, sourceEvent.String, now); err != nil {
				return Summary{}, err
			}
			if err := markDirtyTx(ctx, tx, ownerID, sourceEvent.String, "member_moved", now); err != nil {
				return Summary{}, err
			}
		}
		if !sourceEvent.Valid || sourceEvent.String != eventID {
			if _, err := tx.ExecContext(ctx, `
INSERT INTO event_media_items(event_id,owner_id,media_item_id,position,source,evidence,created_at)
VALUES(?,?,?,?,'user','{}',?)`, eventID, ownerID, mediaID, position, now); err != nil {
				return Summary{}, err
			}
			position++
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM event_constraints WHERE owner_id=? AND kind='exclude' AND event_id=? AND left_media_item_id=?`, ownerID, eventID, mediaID); err != nil {
			return Summary{}, err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO event_constraints(
 constraint_id,owner_id,kind,event_id,left_media_item_id,created_at,updated_at
) VALUES(?,?,'include',?,?,?,?)`, uuid.NewString(), ownerID, eventID, mediaID, now, now); err != nil {
			return Summary{}, err
		}
	}
	if err := repairEventTx(ctx, tx, ownerID, eventID, now); err != nil {
		return Summary{}, err
	}
	if err := markDirtyTx(ctx, tx, ownerID, eventID, "members_added", now); err != nil {
		return Summary{}, err
	}
	if err := tx.Commit(); err != nil {
		return Summary{}, err
	}
	return s.resolver.Resolve(ctx, ownerID, eventID)
}

func repairEventTx(ctx context.Context, tx *sql.Tx, ownerID int32, eventID string, now int64) error {
	var start, end int64
	var cover string
	err := tx.QueryRowContext(ctx, `
SELECT min(COALESCE(a.taken_time,a.upload_time,mi.created_at)),
       max(COALESCE(a.taken_time,a.upload_time,mi.created_at)),
       (SELECT media_item_id FROM event_media_items WHERE event_id=? AND owner_id=? ORDER BY position,media_item_id LIMIT 1)
FROM event_media_items emi
JOIN media_items mi ON mi.media_item_id=emi.media_item_id
LEFT JOIN assets a ON a.asset_id=mi.primary_asset_id
WHERE emi.event_id=? AND emi.owner_id=?`, eventID, ownerID, eventID, ownerID).Scan(&start, &end, &cover)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE events SET start_at=?,end_at=?,generated_cover_media_item_id=?,updated_at=?
WHERE event_id=? AND owner_id=?`, start, end, cover, now, eventID, ownerID)
	return err
}

func markDirtyTx(ctx context.Context, tx *sql.Tx, ownerID int32, eventID, reason string, now int64) error {
	var start, end int64
	if err := tx.QueryRowContext(ctx, `SELECT start_at,end_at FROM events WHERE event_id=? AND owner_id=?`, eventID, ownerID).Scan(&start, &end); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO event_dirty_ranges(dirty_range_id,owner_id,range_start,range_end,reason,created_at)
VALUES(?,?,?,?,?,?)`, uuid.NewString(), ownerID, start, end, reason, now); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO event_owner_state(owner_id,active_algorithm_version,initialized_at,revision,updated_at)
VALUES(?,?,?,1,?)
ON CONFLICT(owner_id) DO UPDATE SET revision=event_owner_state.revision+1,updated_at=excluded.updated_at`,
		ownerID, AlgorithmVersion, now, now)
	return err
}

func canonicalPair(left, right string) (string, string) {
	if left > right {
		return right, left
	}
	return left, right
}

func parseMicros(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case []byte:
		result, err := strconv.ParseInt(string(typed), 10, 64)
		return result, err == nil
	case string:
		result, err := strconv.ParseInt(typed, 10, 64)
		return result, err == nil
	default:
		return 0, false
	}
}

func sortedUnique(values []string) []string {
	sort.Strings(values)
	return values
}
