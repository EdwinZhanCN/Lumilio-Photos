package dto

import "time"

type CreateRepositoryRequestDTO struct {
	Name string `json:"name" binding:"required" example:"Family Photos"`
}

type RepositoryDTO struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string `json:"name" example:"Family Photos"`
	Path      string `json:"path" example:"/data/storage/family-photos"`
	IsPrimary bool   `json:"is_primary" example:"false"`
}

type CreateRepositoryResponseDTO struct {
	Repository RepositoryDTO `json:"repository"`
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
