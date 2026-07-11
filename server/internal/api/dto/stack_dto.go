package dto

import (
	"github.com/google/uuid"

	"server/internal/db/dbtypes"
)

// StackDTO represents a stack of related assets.
type StackDTO struct {
	StackID     string           `json:"stack_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	StackKind   string           `json:"stack_kind,omitempty" example:"burst" enums:"burst,manual"`
	MemberCount int64            `json:"member_count" example:"3"`
	Members     []StackMemberDTO `json:"members"`
}

// StackMemberDTO represents one logical media item within a presentation stack.
type StackMemberDTO struct {
	MediaItemID    string `json:"media_item_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	PrimaryAssetID string `json:"primary_asset_id" example:"550e8400-e29b-41d4-a716-446655440002"`
	Position       int32  `json:"position" example:"0"`
}

// StackByAssetResponseDTO maps an asset ID to its stack info.
type StackByAssetResponseDTO struct {
	AssetID string   `json:"asset_id"`
	Stack   StackDTO `json:"stack"`
}

type MediaItemComponentDTO struct {
	AssetID  string `json:"asset_id"`
	Relation string `json:"relation" enums:"raw_original,jpeg_original,edited_version,alternative,live_photo_still,live_photo_video"`
	Position int32  `json:"position"`
}

type MediaItemDTO struct {
	MediaItemID    string                  `json:"media_item_id"`
	MediaKind      string                  `json:"media_kind" enums:"photo,video,audio,live_photo"`
	PrimaryAssetID string                  `json:"primary_asset_id"`
	Components     []MediaItemComponentDTO `json:"components"`
}

type MediaItemByAssetResponseDTO struct {
	AssetID   string       `json:"asset_id"`
	MediaItem MediaItemDTO `json:"media_item"`
}

// AutoDetectStacksResponseDTO is the response for auto-detect stacks.
type AutoDetectStacksResponseDTO struct {
	RepositoryID  string `json:"repository_id"`
	StacksCreated int    `json:"stacks_created"`
}

// CreateManualStackRequestDTO is the request to manually stack assets.
type CreateManualStackRequestDTO struct {
	AssetIDs []string `json:"asset_ids" example:"550e8400-e29b-41d4-a716-446655440000,550e8400-e29b-41d4-a716-446655440001"`
}

// StackPreviewDTO is a lightweight stack info suitable for asset list responses.
type StackPreviewDTO struct {
	StackID    string `json:"stack_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	StackKind  string `json:"stack_kind,omitempty" example:"burst" enums:"burst,manual"`
	StackCover bool   `json:"stack_cover,omitempty" example:"true"` // Whether this asset is the cover of its stack
	StackSize  *int   `json:"stack_size,omitempty" example:"3"`     // Number of members in the stack
}

// ToStackPreviewDTO converts stack info to a preview DTO.
func ToStackPreviewDTO(stackID *uuid.UUID, stackKind dbtypes.StackKind, isCover bool, size *int) StackPreviewDTO {
	dto := StackPreviewDTO{
		StackKind:  string(stackKind),
		StackCover: isCover,
	}
	if stackID != nil {
		dto.StackID = stackID.String()
	}
	if size != nil {
		dto.StackSize = size
	}
	return dto
}
