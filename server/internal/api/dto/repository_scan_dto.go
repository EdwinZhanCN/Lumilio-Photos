package dto

import "time"

type CreateRepositoryRequestDTO struct {
	Name              string `json:"name" binding:"required" example:"Family Photos"`
	CloudCredentialID string `json:"cloud_credential_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type RepositoryDTO struct {
	ID              string                  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name            string                  `json:"name" example:"Family Photos"`
	Path            string                  `json:"path" example:"/data/storage/family-photos"`
	IsPrimary       bool                    `json:"is_primary" example:"false"`
	DefaultOwnerID  *int32                  `json:"default_owner_id,omitempty"`
	StorageStrategy string                  `json:"storage_strategy" example:"date"`
	LocalSettings   RepositoryLocalSettings `json:"local_settings"`
}

type RepositoryLocalSettings struct {
	PreserveOriginalFilename bool   `json:"preserve_original_filename" example:"true"`
	HandleDuplicateFilenames string `json:"handle_duplicate_filenames" example:"uuid"`
}

type UpdateRepositoryRequestDTO struct {
	Name            *string                  `json:"name,omitempty" example:"My Photos"`
	StorageStrategy *string                  `json:"storage_strategy,omitempty" example:"flat"`
	LocalSettings   *RepositoryLocalSettings `json:"local_settings,omitempty"`
	DefaultOwnerID  *int32                   `json:"default_owner_id,omitempty"`
}

type ListRepositoriesResponseDTO struct {
	Repositories []RepositoryDTO `json:"repositories"`
}

type CreateRepositoryResponseDTO struct {
	Repository       RepositoryDTO `json:"repository"`
	CloudImportRunID *string       `json:"cloud_import_run_id,omitempty"`
	CloudImportError *string       `json:"cloud_import_error,omitempty"`
}

type RepositoryScanRequestDTO struct {
	Force bool `json:"force" example:"false"`
}

type RepositoryScanQueuedDTO struct {
	JobID        int64  `json:"job_id" example:"12345"`
	RepositoryID string `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Mode         string `json:"mode" example:"manual"`
	Status       string `json:"status" example:"queued"`
}

type RepositoryScanRunDTO struct {
	ScanID          string     `json:"scan_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	RepositoryID    string     `json:"repository_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Mode            string     `json:"mode" example:"manual"`
	RequestedBy     *string    `json:"requested_by,omitempty" example:"edwin"`
	Status          string     `json:"status" example:"completed"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	DiscoveredCount int64      `json:"discovered_count" example:"10"`
	UpdatedCount    int64      `json:"updated_count" example:"2"`
	DeletedCount    int64      `json:"deleted_count" example:"1"`
	SkippedCount    int64      `json:"skipped_count" example:"4"`
	Error           *string    `json:"error,omitempty"`
}

type RepositoryScanRunListDTO struct {
	Scans []RepositoryScanRunDTO `json:"scans"`
}
