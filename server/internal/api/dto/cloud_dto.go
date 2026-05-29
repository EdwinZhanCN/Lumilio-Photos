package dto

import "server/internal/cloud"

// ConnectICloudRequest is the request body for connecting to iCloud.
type ConnectICloudRequest struct {
	Username string         `json:"username" binding:"required" example:"user@icloud.com"`
	Password string         `json:"password" binding:"required" example:"app-specific-password"`
	Domain   string         `json:"domain,omitempty" binding:"omitempty,oneof=com cn" example:"com"`
	SyncMode cloud.SyncMode `json:"sync_mode,omitempty" binding:"omitempty,oneof=import one_way" example:"import"`
}

// VerifyICloud2FARequest is the request body for submitting a 2FA code.
type VerifyICloud2FARequest struct {
	Code string `json:"code" binding:"required" example:"123456"`
}

// ConnectICloudResponse is the response for the iCloud connect endpoint.
type ConnectICloudResponse struct {
	Needs2FA bool `json:"needs_2fa"`
}

// TriggerSyncRequest is the request body for triggering a cloud sync.
type TriggerSyncRequest struct {
	Provider     cloud.ProviderKind `json:"provider" binding:"required" example:"icloud"`
	RepositoryID string             `json:"repository_id" binding:"required" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// CloudProviderStatusDTO is the response for a single provider's status.
type CloudProviderStatusDTO struct {
	Provider        string `json:"provider" example:"icloud"`
	SyncMode        string `json:"sync_mode" example:"import"`
	Connected       bool   `json:"connected" example:"true"`
	LastCursor      string `json:"last_cursor,omitempty"`
	SyncedFileCount int64  `json:"synced_file_count" example:"42"`
}

// ListProvidersResponse is the response for the list providers endpoint.
type ListProvidersResponse struct {
	Providers []CloudProviderStatusDTO `json:"providers"`
}
