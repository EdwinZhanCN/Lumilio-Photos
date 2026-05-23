package dto

import (
	"github.com/google/uuid"

	"server/internal/db/dbtypes"
)

// StackDTO represents a stack of related assets.
type StackDTO struct {
	StackID     string           `json:"stack_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	StackKind   string           `json:"stack_kind,omitempty" example:"live_photo" enums:"raw_jpeg,live_photo,manual"`
	MemberCount int64            `json:"member_count" example:"3"`
	Members     []StackMemberDTO `json:"members"`
}

// StackMemberDTO represents a single asset within a stack.
type StackMemberDTO struct {
	AssetID  string `json:"asset_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	Relation string `json:"relation" example:"raw_original"` // raw_original, jpeg_original, edited_version, alternative
	Position int32  `json:"position" example:"0"`
}

// StackByAssetResponseDTO maps an asset ID to its stack info.
type StackByAssetResponseDTO struct {
	AssetID string   `json:"asset_id"`
	Stack   StackDTO `json:"stack"`
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
	StackKind  string `json:"stack_kind,omitempty" example:"live_photo" enums:"raw_jpeg,live_photo,manual"`
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
