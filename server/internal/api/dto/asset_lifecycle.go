package dto

// AssetSelectionDTO identifies one complete lifecycle operation.
type AssetSelectionDTO struct {
	AssetIDs []string `json:"asset_ids" binding:"required,min=1,max=10000,dive,uuid"`
}

// AssetPurgeRequestDTO requires explicit confirmation for irreversible actions.
type AssetPurgeRequestDTO struct {
	AssetIDs     []string `json:"asset_ids,omitempty" binding:"omitempty,max=10000,dive,uuid"`
	RepositoryID *string  `json:"repository_id,omitempty" binding:"omitempty,uuid"`
	Confirm      bool     `json:"confirm" binding:"required"`
}

type AssetDeleteImpactDTO struct {
	Assets        int                        `json:"assets"`
	Files         int                        `json:"files"`
	Bytes         int64                      `json:"bytes"`
	Repositories  []AssetImpactRepositoryDTO `json:"repositories"`
	RetentionDays int                        `json:"retention_days"`
}

type AssetImpactRepositoryDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AssetLifecycleResultDTO struct {
	Files   int                    `json:"files"`
	Assets  int                    `json:"assets"`
	Entries int64                  `json:"entries"`
	Bytes   int64                  `json:"bytes"`
	Renamed []AssetRestoredPathDTO `json:"renamed"`
}

type AssetRestoredPathDTO struct {
	AssetID      string `json:"asset_id"`
	OriginalPath string `json:"original_path"`
	RestoredPath string `json:"restored_path"`
}
