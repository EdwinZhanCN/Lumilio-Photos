package dto

import "server/internal/event"

type EventSummaryDTO struct {
	EventID          string  `json:"event_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RedirectedFrom   string  `json:"redirected_from,omitempty"`
	StartAt          int64   `json:"start_at"`
	EndAt            int64   `json:"end_at"`
	Timezone         *string `json:"timezone,omitempty"`
	TitleOverride    *string `json:"title_override,omitempty"`
	CoverMediaItemID *string `json:"cover_media_item_id,omitempty"`
	CoverAssetID     *string `json:"cover_asset_id,omitempty"`
	IsHidden         bool    `json:"is_hidden"`
	MediaCount       int     `json:"media_count"`
	DisplayableCount int     `json:"displayable_count"`
	CanonicalCount   int     `json:"canonical_media_count"`
	ProjectedCount   int     `json:"projected_media_count"`
}

type EventDetailDTO struct {
	EventSummaryDTO
	AlgorithmVersion string `json:"algorithm_version"`
	PendingRebuild   bool   `json:"pending_rebuild"`
}

type EventListPageDTO struct {
	Events     []EventSummaryDTO `json:"events"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type EventAssetDTO struct {
	Position    int64  `json:"position"`
	MediaItemID string `json:"media_item_id"`
	AssetID     string `json:"asset_id"`
}

type EventAssetsPageDTO struct {
	Assets         []EventAssetDTO `json:"assets"`
	OmittedMembers int             `json:"omitted_members"`
	NextCursor     string          `json:"next_cursor,omitempty"`
}

type EventPatchRequestDTO struct {
	TitleOverride      *string `json:"title_override"`
	ClearTitleOverride bool    `json:"clear_title_override"`
	CoverMediaItemID   *string `json:"cover_media_item_id"`
	ClearCoverOverride bool    `json:"clear_cover_override"`
	IsHidden           *bool   `json:"is_hidden"`
}

type EventMutationResponseDTO struct {
	Event          EventSummaryDTO `json:"event"`
	PendingRebuild bool            `json:"pending_rebuild"`
}

type EventRebuildRequestDTO struct {
	OwnerID *int32 `json:"owner_id"`
}

type EventRebuildAcceptedDTO struct {
	RunID             string `json:"run_id"`
	OwnerID           int32  `json:"owner_id"`
	RequestedRevision int64  `json:"requested_revision"`
}

type EventRebuildStatusDTO struct {
	Initialized       bool   `json:"initialized"`
	AlgorithmVersion  string `json:"algorithm_version"`
	Paused            bool   `json:"paused"`
	Revision          int64  `json:"revision"`
	SourceRevision    int64  `json:"source_revision"`
	PublishedRevision int64  `json:"published_revision"`
	PendingRanges     int    `json:"pending_ranges"`
	Pending           bool   `json:"pending"`
	QueuedRunID       string `json:"queued_run_id,omitempty"`
	RunningRunID      string `json:"running_run_id,omitempty"`
	LastSuccessRunID  string `json:"last_success_run_id,omitempty"`
	LastFailureRunID  string `json:"last_failure_run_id,omitempty"`
	LastErrorCode     string `json:"last_error_code,omitempty"`
}

type EventRebuildStateRequestDTO struct {
	Paused  bool   `json:"paused"`
	OwnerID *int32 `json:"owner_id"`
}

type EventShareRequestDTO struct {
	Title            string  `json:"title" binding:"required"`
	Description      *string `json:"description,omitempty"`
	ExpiresInDays    int     `json:"expires_in_days,omitempty"`
	AllowDownload    bool    `json:"allow_download,omitempty"`
	IncludeOriginals bool    `json:"include_originals,omitempty"`
}

type EventRelationsResponseDTO struct {
	Relations     []event.ResourceRelation `json:"relations"`
	Complete      bool                     `json:"complete"`
	SourceVersion string                   `json:"source_version"`
}

type EventMergeRequestDTO struct {
	EventIDs        []string `json:"event_ids" binding:"required,min=2"`
	SurvivorEventID string   `json:"survivor_event_id" binding:"required"`
}

type EventSplitRequestDTO struct {
	BeforeMediaItemID string `json:"before_media_item_id" binding:"required"`
}

type EventAddMembersRequestDTO struct {
	AssetIDs []string `json:"asset_ids" binding:"required,min=1"`
}
