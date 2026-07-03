package dto

import (
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
)

// CreateShareLinkRequestDTO represents the request to create a share link.
// asset_ids is required only when source_kind is "asset_snapshot"; for
// "album"/"person"/"utility_query"/"pin", source_ref identifies the source and
// the backend resolves the asset snapshot server-side.
type CreateShareLinkRequestDTO struct {
	Title            string   `json:"title" binding:"required"`
	Description      *string  `json:"description,omitempty"`
	SourceKind       string   `json:"source_kind" binding:"required,oneof=asset_snapshot album person utility_query pin"`
	SourceRef        *string  `json:"source_ref,omitempty"`
	AssetIDs         []string `json:"asset_ids,omitempty"`
	ExpiresInDays    int      `json:"expires_in_days,omitempty" example:"30" minimum:"1" maximum:"365"`
	AllowDownload    bool     `json:"allow_download,omitempty"`
	IncludeOriginals bool     `json:"include_originals,omitempty"`
}

// UpdateShareLinkRequestDTO represents a patch to an existing share link's
// settings. ExtendDays, when set, moves expires_at to
// max(now, expires_at) + N days.
type UpdateShareLinkRequestDTO struct {
	Title            *string `json:"title,omitempty"`
	Description      *string `json:"description,omitempty"`
	AllowDownload    *bool   `json:"allow_download,omitempty"`
	IncludeOriginals *bool   `json:"include_originals,omitempty"`
	ExtendDays       *int    `json:"extend_days,omitempty" example:"30" minimum:"1" maximum:"365"`
}

// ShareLinkDTO represents a share link's owner-facing metadata. It never
// includes the token or token hash; the raw token is only ever returned once,
// embedded in CreateShareLinkResponseDTO.
type ShareLinkDTO struct {
	ShareID          string     `json:"share_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Title            string     `json:"title"`
	Description      *string    `json:"description,omitempty"`
	SourceKind       string     `json:"source_kind" enums:"asset_snapshot,album,person,utility_query,pin"`
	SourceRef        *string    `json:"source_ref,omitempty"`
	AssetCount       int        `json:"asset_count"`
	AllowDownload    bool       `json:"allow_download"`
	IncludeOriginals bool       `json:"include_originals"`
	Status           string     `json:"status" enums:"active,revoked"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	LastViewedAt     *time.Time `json:"last_viewed_at,omitempty"`
	ViewCount        int64      `json:"view_count"`
}

// ToShareLinkDTO converts a repo.ShareLink row into its owner-facing DTO.
func ToShareLinkDTO(l repo.ShareLink) ShareLinkDTO {
	out := ShareLinkDTO{
		ShareID:          uuid.UUID(l.ShareID.Bytes).String(),
		Title:            l.Title,
		Description:      l.Description,
		SourceKind:       l.SourceKind,
		SourceRef:        l.SourceRef,
		AssetCount:       int(l.AssetCount),
		AllowDownload:    l.AllowDownload,
		IncludeOriginals: l.IncludeOriginals,
		Status:           l.Status,
		ViewCount:        l.ViewCount,
	}
	if l.ExpiresAt.Valid {
		out.ExpiresAt = l.ExpiresAt.Time
	}
	if l.CreatedAt.Valid {
		out.CreatedAt = l.CreatedAt.Time
	}
	if l.UpdatedAt.Valid {
		out.UpdatedAt = l.UpdatedAt.Time
	}
	if l.RevokedAt.Valid {
		t := l.RevokedAt.Time
		out.RevokedAt = &t
	}
	if l.LastViewedAt.Valid {
		t := l.LastViewedAt.Time
		out.LastViewedAt = &t
	}
	return out
}

// CreateShareLinkResponseDTO is returned once at creation time; it is the
// only response that ever includes the raw share token.
type CreateShareLinkResponseDTO struct {
	ShareLinkDTO
	Token string `json:"token" example:"7yQhF3z9k2mN8pXeR5tVwL1sJ4bC6dA0"`
}

// ListShareLinksResponseDTO represents the response for listing owner share links.
type ListShareLinksResponseDTO struct {
	Items []ShareLinkDTO `json:"items"`
}

// PublicShareMetadataDTO is the de-sensitized metadata served to public share
// viewers. It never includes owner, source, or internal identifiers.
type PublicShareMetadataDTO struct {
	Title         string  `json:"title"`
	Description   *string `json:"description,omitempty"`
	AssetCount    int     `json:"asset_count"`
	AllowDownload bool    `json:"allow_download"`
	// IncludeOriginals tells the viewer whether per-asset original downloads
	// are available (GetPublicShareOriginal requires both this and
	// AllowDownload); it is a policy flag, not sensitive.
	IncludeOriginals bool      `json:"include_originals"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// ToPublicShareMetadataDTO converts a repo.ShareLink row into de-sensitized
// public metadata.
func ToPublicShareMetadataDTO(l repo.ShareLink) PublicShareMetadataDTO {
	out := PublicShareMetadataDTO{
		Title:            l.Title,
		Description:      l.Description,
		AssetCount:       int(l.AssetCount),
		AllowDownload:    l.AllowDownload,
		IncludeOriginals: l.IncludeOriginals,
	}
	if l.ExpiresAt.Valid {
		out.ExpiresAt = l.ExpiresAt.Time
	}
	if l.CreatedAt.Valid {
		out.CreatedAt = l.CreatedAt.Time
	}
	return out
}

// PublicAssetDTO is the minimal, de-sensitized asset shape served to public
// share viewers: no owner_id, storage_path, original_filename, hash, or EXIF.
type PublicAssetDTO struct {
	AssetID   string     `json:"asset_id"`
	Type      string     `json:"type" enums:"PHOTO,VIDEO,AUDIO"`
	Width     *int32     `json:"width,omitempty"`
	Height    *int32     `json:"height,omitempty"`
	Duration  *float64   `json:"duration,omitempty"`
	TakenTime *time.Time `json:"taken_time,omitempty"`
}

// ToPublicAssetDTO converts a repo.Asset row into its de-sensitized public shape.
func ToPublicAssetDTO(a repo.Asset) PublicAssetDTO {
	out := PublicAssetDTO{
		AssetID:  uuid.UUID(a.AssetID.Bytes).String(),
		Type:     a.Type,
		Width:    a.Width,
		Height:   a.Height,
		Duration: a.Duration,
	}
	if a.TakenTime.Valid {
		t := a.TakenTime.Time
		out.TakenTime = &t
	}
	return out
}

// PublicShareAssetListRequestDTO is the pagination-only request for browsing
// a public share. v1 is browse-only in date order; no filter/search/sort.
type PublicShareAssetListRequestDTO struct {
	Limit  int `json:"limit,omitempty" example:"50" minimum:"1" maximum:"200"`
	Offset int `json:"offset,omitempty" example:"0" minimum:"0"`
}

// PublicShareAssetListResponseDTO is one page of a public share's asset list.
type PublicShareAssetListResponseDTO struct {
	Items  []PublicAssetDTO `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

// PublicShareDownloadRequestDTO optionally scopes a zip download to a subset
// of the share's assets; an empty/omitted asset_ids downloads the whole share.
type PublicShareDownloadRequestDTO struct {
	AssetIDs []string `json:"asset_ids,omitempty"`
}
