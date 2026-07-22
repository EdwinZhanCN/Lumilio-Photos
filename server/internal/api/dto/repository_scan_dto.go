package dto

import "time"

type CreateRepositoryRequestDTO struct {
	Name string `json:"name" binding:"required" example:"Family Photos"`
	// RootID identifies a registered Storage Location. Empty selects the
	// configured default location. Clients never submit an arbitrary root path.
	RootID            string `json:"root_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Role              string `json:"role,omitempty" binding:"omitempty,oneof=primary regular" example:"regular"`
	StorageStrategy   string `json:"storage_strategy,omitempty" binding:"omitempty,oneof=date flat cas" example:"date"`
	DuplicateHandling string `json:"duplicate_handling,omitempty" binding:"omitempty,oneof=rename uuid overwrite" example:"rename"`
	CloudCredentialID string `json:"cloud_credential_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
}

type RepositoryDTO struct {
	ID              string                  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name            string                  `json:"name" example:"Family Photos"`
	Path            string                  `json:"path" example:"/data/storage/family-photos"`
	Role            string                  `json:"role" example:"regular"`
	IsPrimary       bool                    `json:"is_primary" example:"false"`
	RootID          *string                 `json:"root_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	Status          string                  `json:"status" example:"active"`
	DefaultOwnerID  *int32                  `json:"default_owner_id,omitempty"`
	StorageStrategy string                  `json:"storage_strategy" example:"date"`
	LocalSettings   RepositoryLocalSettings `json:"local_settings"`
}

type RepositoryLocalSettings struct {
	HandleDuplicateFilenames string `json:"handle_duplicate_filenames" example:"uuid"`
}

type UpdateRepositoryRequestDTO struct {
	Name            *string                  `json:"name,omitempty" example:"My Photos"`
	StorageStrategy *string                  `json:"storage_strategy,omitempty" example:"flat"`
	LocalSettings   *RepositoryLocalSettings `json:"local_settings,omitempty"`
}

type ListRepositoriesResponseDTO struct {
	Repositories []RepositoryDTO `json:"repositories"`
}

type CreateRepositoryResponseDTO struct {
	Repository RepositoryDTO `json:"repository"`
	// Warnings are non-fatal notes about the chosen location, such as it being
	// inside a cloud-sync folder. The repository was created regardless.
	Warnings         []string `json:"warnings,omitempty"`
	CloudImportRunID *string  `json:"cloud_import_run_id,omitempty"`
	CloudImportError *string  `json:"cloud_import_error,omitempty"`
}

// RepositoryConflictDTO describes a repository whose identity is already
// registered at a different path, and the two actions that resolve it.
type RepositoryConflictDTO struct {
	Code           int      `json:"code" example:"409"`
	Message        string   `json:"message" example:"Repository identity is already registered"`
	ConflictType   string   `json:"conflict_type" example:"repository_identity"`
	RepositoryID   string   `json:"repository_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	RegisteredPath string   `json:"registered_path,omitempty" example:"/Volumes/OldDrive/Photos"`
	RequestedPath  string   `json:"requested_path,omitempty" example:"/Volumes/NewDrive/Photos"`
	Actions        []string `json:"actions,omitempty" example:"relocate,copy"`
}

type RepositoryRootDTO struct {
	ID     string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name   string `json:"name" example:"External Archive"`
	Path   string `json:"path" example:"/Volumes/Photos"`
	Kind   string `json:"kind" example:"external"`
	Status string `json:"status" example:"active"`
}

type ListRepositoryRootsResponseDTO struct {
	Roots []RepositoryRootDTO `json:"roots"`
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
