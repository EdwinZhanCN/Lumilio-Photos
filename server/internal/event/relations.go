package event

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

type ResourceRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type ResourceRelation struct {
	From       ResourceRef    `json:"from"`
	Relation   string         `json:"relation"`
	To         ResourceRef    `json:"to"`
	Origin     string         `json:"origin"`
	Confidence *float32       `json:"confidence,omitempty"`
	Facts      map[string]any `json:"facts"`
}

type RelationResult struct {
	Relations     []ResourceRelation `json:"relations"`
	Complete      bool               `json:"complete"`
	SourceVersion string             `json:"source_version"`
}

type RelationService struct{ db *sql.DB }

func NewRelationService(db *sql.DB) *RelationService { return &RelationService{db: db} }

func (s *RelationService) ForEvent(ctx context.Context, ownerID int32, eventID string) (RelationResult, error) {
	resolver := NewResolver(s.db)
	summary, err := resolver.Resolve(ctx, ownerID, eventID)
	if err != nil {
		return RelationResult{}, err
	}
	from := ResourceRef{Kind: "event", ID: summary.EventID}
	result := RelationResult{Relations: []ResourceRelation{}, Complete: true, SourceVersion: AlgorithmVersion}
	queries := []struct {
		sql      string
		kind     string
		relation string
		origin   string
	}{
		{`
SELECT CAST(fc.cluster_id AS TEXT),count(DISTINCT emi.media_item_id)
FROM event_media_items emi
JOIN media_item_assets mia ON mia.media_item_id=emi.media_item_id
JOIN face_items fi ON fi.asset_id=mia.asset_id
JOIN face_cluster_members fcm ON fcm.face_id=fi.id
JOIN face_clusters fc ON fc.cluster_id=fcm.cluster_id AND fc.owner_id=emi.owner_id
WHERE emi.event_id=? AND emi.owner_id=?
GROUP BY fc.cluster_id ORDER BY fc.cluster_id`, "person", "contains_person", "face_assignment"},
		{`
SELECT CAST(al.album_id AS TEXT),count(DISTINCT emi.media_item_id)
FROM event_media_items emi
JOIN media_item_assets mia ON mia.media_item_id=emi.media_item_id
JOIN album_assets aa ON aa.asset_id=mia.asset_id
JOIN albums al ON al.album_id=aa.album_id AND al.user_id=emi.owner_id
WHERE emi.event_id=? AND emi.owner_id=?
GROUP BY al.album_id ORDER BY al.album_id`, "album", "represented_in", "album_membership"},
		{`
SELECT r.repo_id || ':' || a.gps_geohash_7,count(DISTINCT emi.media_item_id)
FROM event_media_items emi
JOIN media_items mi ON mi.media_item_id=emi.media_item_id AND mi.owner_id=emi.owner_id
JOIN assets a ON a.asset_id=mi.primary_asset_id AND a.owner_id=emi.owner_id
JOIN repositories r ON r.repo_id=a.repository_id
WHERE emi.event_id=? AND emi.owner_id=? AND a.gps_geohash_7 IS NOT NULL
GROUP BY r.repo_id,a.gps_geohash_7 ORDER BY r.repo_id,a.gps_geohash_7`,
			"location_cell", "occurred_at", "current_gps"},
	}
	for _, query := range queries {
		rows, err := s.db.QueryContext(ctx, query.sql, summary.EventID, ownerID)
		if err != nil {
			return RelationResult{}, fmt.Errorf("query Event %s relations: %w", query.kind, err)
		}
		for rows.Next() {
			var id string
			var mediaCount int
			if err := rows.Scan(&id, &mediaCount); err != nil {
				rows.Close()
				return RelationResult{}, err
			}
			result.Relations = append(result.Relations, ResourceRelation{
				From: from, Relation: query.relation, To: ResourceRef{Kind: query.kind, ID: id},
				Origin: query.origin, Facts: map[string]any{"distinct_media_items": mediaCount},
			})
		}
		if err := rows.Close(); err != nil {
			return RelationResult{}, err
		}
	}
	return result, nil
}

func (s *RelationService) ForPerson(ctx context.Context, ownerID int32, personID int32) (RelationResult, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM face_clusters WHERE cluster_id=? AND owner_id=?`, personID, ownerID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RelationResult{}, ErrNotFound
		}
		return RelationResult{}, fmt.Errorf("resolve Person: %w", err)
	}
	from := ResourceRef{Kind: "person", ID: strconv.FormatInt(int64(personID), 10)}
	result := RelationResult{Relations: []ResourceRelation{}, Complete: true, SourceVersion: AlgorithmVersion}
	rows, err := s.db.QueryContext(ctx, `
SELECT e.event_id,count(DISTINCT emi.media_item_id)
FROM face_clusters fc
JOIN face_cluster_members fcm ON fcm.cluster_id=fc.cluster_id
JOIN face_items fi ON fi.id=fcm.face_id
JOIN media_item_assets mia ON mia.asset_id=fi.asset_id
JOIN event_media_items emi ON emi.media_item_id=mia.media_item_id AND emi.owner_id=fc.owner_id
JOIN events e ON e.event_id=emi.event_id AND e.owner_id=emi.owner_id AND e.status='active'
WHERE fc.cluster_id=? AND fc.owner_id=?
GROUP BY e.event_id ORDER BY e.start_at DESC,e.event_id DESC`, personID, ownerID)
	if err != nil {
		return RelationResult{}, fmt.Errorf("query Person Events: %w", err)
	}
	for rows.Next() {
		var id string
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			rows.Close()
			return RelationResult{}, err
		}
		result.Relations = append(result.Relations, ResourceRelation{
			From: from, Relation: "appears_in", To: ResourceRef{Kind: "event", ID: id},
			Origin: "face_assignment", Facts: map[string]any{"distinct_media_items": count},
		})
	}
	if err := rows.Close(); err != nil {
		return RelationResult{}, err
	}
	rows, err = s.db.QueryContext(ctx, `
SELECT other.cluster_id,count(DISTINCT emi.event_id)
FROM face_clusters subject
JOIN face_cluster_members subject_member ON subject_member.cluster_id=subject.cluster_id
JOIN face_items subject_face ON subject_face.id=subject_member.face_id
JOIN media_item_assets subject_mia ON subject_mia.asset_id=subject_face.asset_id
JOIN event_media_items emi ON emi.media_item_id=subject_mia.media_item_id AND emi.owner_id=subject.owner_id
JOIN events e ON e.event_id=emi.event_id AND e.owner_id=emi.owner_id AND e.status='active'
JOIN event_media_items peer_emi ON peer_emi.event_id=e.event_id AND peer_emi.owner_id=e.owner_id
JOIN media_item_assets peer_mia ON peer_mia.media_item_id=peer_emi.media_item_id
JOIN face_items peer_face ON peer_face.asset_id=peer_mia.asset_id
JOIN face_cluster_members peer_member ON peer_member.face_id=peer_face.id
JOIN face_clusters other ON other.cluster_id=peer_member.cluster_id AND other.owner_id=subject.owner_id
WHERE subject.cluster_id=? AND subject.owner_id=? AND other.cluster_id<>subject.cluster_id
GROUP BY other.cluster_id ORDER BY other.cluster_id`, personID, ownerID)
	if err != nil {
		return RelationResult{}, fmt.Errorf("query Person co-occurrences: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int32
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return RelationResult{}, err
		}
		result.Relations = append(result.Relations, ResourceRelation{
			From: from, Relation: "co_occurs_with",
			To:     ResourceRef{Kind: "person", ID: strconv.FormatInt(int64(id), 10)},
			Origin: "shared_event", Facts: map[string]any{"shared_event_count": count},
		})
	}
	return result, rows.Err()
}
