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

// BackupListDTO wraps the dump list.
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
