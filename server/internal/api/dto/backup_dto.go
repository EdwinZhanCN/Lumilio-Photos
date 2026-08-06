package dto

import (
	"time"

	"server/internal/service"
)

// BackupEntryDTO is one SQLite snapshot with manifest provenance.
type BackupEntryDTO struct {
	Name          string    `json:"name" example:"20260711T020000.000000Z-library.sqlite3"`
	SizeBytes     int64     `json:"size_bytes" example:"1048576"`
	CreatedAt     time.Time `json:"created_at"`
	AppVersion    string    `json:"app_version" example:"1.2.3"`
	SQLiteVersion string    `json:"sqlite_version" example:"3.50.4"`
	RestorePoint  bool      `json:"restore_point"`
}

// BackupListDTO wraps the snapshot list.
type BackupListDTO struct {
	Backups []BackupEntryDTO `json:"backups"`
}

func ToBackupListDTO(entries []service.BackupEntry) BackupListDTO {
	out := BackupListDTO{Backups: make([]BackupEntryDTO, 0, len(entries))}
	for _, e := range entries {
		out.Backups = append(out.Backups, BackupEntryDTO{
			Name:          e.Name,
			SizeBytes:     e.SizeBytes,
			CreatedAt:     e.CreatedAt,
			AppVersion:    e.AppVersion,
			SQLiteVersion: e.SQLiteVersion,
			RestorePoint:  e.RestorePoint,
		})
	}
	return out
}

// RestoreOperationDTO is the durable observation surface for a restore that
// continues across an HTTP disconnect and runtime restart.
type RestoreOperationDTO struct {
	ID           string     `json:"id" example:"d62cbbf3-f564-458b-86ca-0f6d10fcd8d4"`
	BackupName   string     `json:"backup_name" example:"20260711T020000.000000Z-library.sqlite3"`
	Status       string     `json:"status" enums:"staged,restart_requested,installing,verifying,completed,rolling_back,rolled_back,failed"`
	Message      string     `json:"message"`
	ErrorCode    string     `json:"error_code,omitempty"`
	RestorePoint string     `json:"restore_point,omitempty"`
	RequestedAt  time.Time  `json:"requested_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

func ToRestoreOperationDTO(operation service.RestoreOperation) RestoreOperationDTO {
	return RestoreOperationDTO{
		ID:           operation.ID,
		BackupName:   operation.BackupName,
		Status:       string(operation.Status),
		Message:      operation.Message,
		ErrorCode:    operation.ErrorCode,
		RestorePoint: operation.RestorePoint,
		RequestedAt:  operation.RequestedAt,
		UpdatedAt:    operation.UpdatedAt,
		CompletedAt:  operation.CompletedAt,
	}
}
