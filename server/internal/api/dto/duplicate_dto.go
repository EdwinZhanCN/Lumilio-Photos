package dto

import "time"

// DuplicateAssetDTO describes a single member of a duplicate group.
// The embedded asset gives the UI everything it needs to render the thumbnail
// while role/file_size drive the merge picker UX.
type DuplicateAssetDTO struct {
	Asset    AssetDTO `json:"asset"`
	Role     string   `json:"role" example:"keeper" enums:"keeper,duplicate,candidate"`
	FileSize int64    `json:"file_size" example:"6291456"`
}

// DuplicateEdgeDTO is the per-pair evidence behind a duplicate group, used by
// the UI to explain why the system flagged two photos as duplicates.
type DuplicateEdgeDTO struct {
	AssetIDA   string  `json:"asset_id_a" example:"550e8400-e29b-41d4-a716-446655440000"`
	AssetIDB   string  `json:"asset_id_b" example:"660e8400-e29b-41d4-a716-446655440001"`
	Method     string  `json:"method" example:"exact" enums:"exact,phash"`
	Distance   float64 `json:"distance" example:"0"`
	Confidence float64 `json:"confidence" example:"1.0"`
}

// DuplicateGroupDTO is one connected component of the duplicate graph.
type DuplicateGroupDTO struct {
	GroupID                  string              `json:"group_id" example:"7c0a4220-1f15-4eb5-94e1-1f4b1d3e4f12"`
	RepositoryID             string              `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Method                   string              `json:"method" example:"mixed" enums:"exact,phash,mixed"`
	Status                   string              `json:"status" example:"pending" enums:"pending,merged,dismissed"`
	AssetCount               int32               `json:"asset_count" example:"3"`
	TotalSize                int64               `json:"total_size" example:"15728640"`
	RecoverableBytes         int64               `json:"recoverable_bytes" example:"10485760"`
	RecommendedKeeperAssetID *string             `json:"recommended_keeper_asset_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	KeeperAssetID            *string             `json:"keeper_asset_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	DetectionVersion         string              `json:"detection_version" example:"duplicates-v1"`
	DetectedAt               time.Time           `json:"detected_at" example:"2026-05-12T08:23:45Z"`
	ResolvedAt               *time.Time          `json:"resolved_at,omitempty" example:"2026-05-12T08:25:00Z"`
	Assets                   []DuplicateAssetDTO `json:"assets"`
	Edges                    []DuplicateEdgeDTO  `json:"edges,omitempty"`
}

// DuplicateSummaryDTO powers the Utilities Rail entry card.
type DuplicateSummaryDTO struct {
	PendingGroups     int64      `json:"pending_groups" example:"7"`
	MergedGroups      int64      `json:"merged_groups" example:"2"`
	DismissedGroups   int64      `json:"dismissed_groups" example:"0"`
	PendingAssets     int64      `json:"pending_assets" example:"18"`
	RecoverableAssets int64      `json:"recoverable_assets" example:"11"`
	RecoverableBytes  int64      `json:"recoverable_bytes" example:"68157440"`
	LastDetectedAt    *time.Time `json:"last_detected_at,omitempty" example:"2026-05-12T08:23:45Z"`
}

// ListDuplicateGroupsResponseDTO is the paginated list response.
type ListDuplicateGroupsResponseDTO struct {
	Groups []DuplicateGroupDTO `json:"groups"`
	Total  int64               `json:"total" example:"7"`
	Limit  int                 `json:"limit" example:"20"`
	Offset int                 `json:"offset" example:"0"`
}

// DetectDuplicatesRequestDTO is the body for POST /duplicates/detect.
type DetectDuplicatesRequestDTO struct {
	RepositoryID string `json:"repository_id" binding:"required,uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// DetectDuplicatesResponseDTO summarizes a detection run for the UI.
type DetectDuplicatesResponseDTO struct {
	Groups         int       `json:"groups" example:"7"`
	ExactGroups    int       `json:"exact_groups" example:"4"`
	PHashGroups    int       `json:"phash_groups" example:"2"`
	MixedGroups    int       `json:"mixed_groups" example:"1"`
	AssetsAffected int       `json:"assets_affected" example:"18"`
	GeneratedAt    time.Time `json:"generated_at" example:"2026-05-12T08:23:45Z"`
}

// MergeDuplicatePolicyDTO mirrors service.MergeMetadataPolicy. Each flag is
// optional and defaults to the Apple Photos-style "union safe metadata" set.
type MergeDuplicatePolicyDTO struct {
	Albums      *bool `json:"albums,omitempty" example:"true"`
	Tags        *bool `json:"tags,omitempty" example:"true"`
	Rating      *bool `json:"rating,omitempty" example:"true"`
	Liked       *bool `json:"liked,omitempty" example:"true"`
	Description *bool `json:"description,omitempty" example:"true"`
	// Faces re-parents face_items onto the keeper. Only applied for exact
	// duplicate groups (the service enforces this constraint).
	Faces *bool `json:"faces,omitempty" example:"false"`
}

// MergeDuplicateGroupRequestDTO is the body for POST /duplicates/groups/:id/merge.
type MergeDuplicateGroupRequestDTO struct {
	KeeperAssetID     string                  `json:"keeper_asset_id" binding:"required,uuid" example:"550e8400-e29b-41d4-a716-446655440000"`
	DuplicateAssetIDs []string                `json:"duplicate_asset_ids,omitempty" example:"660e8400-e29b-41d4-a716-446655440001"`
	Policy            *MergeDuplicatePolicyDTO `json:"policy,omitempty"`
}

// MergeDuplicateGroupResponseDTO summarizes the merge result for the UI.
type MergeDuplicateGroupResponseDTO struct {
	GroupID          string   `json:"group_id" example:"7c0a4220-1f15-4eb5-94e1-1f4b1d3e4f12"`
	KeeperAssetID    string   `json:"keeper_asset_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	MergedDuplicates []string `json:"merged_duplicates" example:"660e8400-e29b-41d4-a716-446655440001"`
	RecoveredBytes   int64    `json:"recovered_bytes" example:"10485760"`
}
