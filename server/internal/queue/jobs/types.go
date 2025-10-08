package jobs

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ProcessAssetArgs is the River job payload for processing a newly uploaded asset.
// Duplicated here (instead of importing processors) to avoid import cycles.
// Keep this in sync with processors.AssetPayload.
type ProcessAssetArgs struct {
	ClientHash   string    `json:"clientHash" river:"unique"`
	StagedPath   string    `json:"stagedPath"`
	UserID       string    `json:"userId" river:"unique"`
	Timestamp    time.Time `json:"timestamp"`
	ContentType  string    `json:"contentType,omitempty"`
	FileName     string    `json:"fileName,omitempty"`
	RepositoryID string    `json:"repositoryId,omitempty"`
}

func (ProcessAssetArgs) Kind() string { return "process_asset" }

// ProcessClipArgs is the River job payload for CLIP embedding/classification.
// Duplicated here (instead of importing processors) to avoid import cycles.
// Keep this in sync with processors.CLIPPayload.
type ProcessClipArgs struct {
	AssetID   pgtype.UUID `json:"assetId"`
	ImageData []byte      `json:"imageData"`
}

func (ProcessClipArgs) Kind() string { return "process_clip" }

// AssetRetryPayload is the River job payload for selective retry of asset processing tasks
type AssetRetryPayload struct {
	AssetID        string   `json:"assetId" river:"unique"`
	RetryTasks     []string `json:"retryTasks,omitempty"` // Empty means retry all failed tasks
	ForceFullRetry bool     `json:"forceFullRetry,omitempty"`
}

func (AssetRetryPayload) Kind() string { return "retry_asset" }
