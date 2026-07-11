package dto

import (
	"time"

	"server/internal/service"
)

// BackupEntryDTO is one database dump; all provenance comes from the filename.
type BackupEntryDTO struct {
	Name         string    `json:"name" example:"lumilio-db-backup-20260711T020000-v1.2.3-pg17.5.sql.gz"`
	SizeBytes    int64     `json:"size_bytes" example:"1048576"`
	CreatedAt    time.Time `json:"created_at"`
	AppVersion   string    `json:"app_version" example:"1.2.3"`
	PGVersion    string    `json:"pg_version" example:"17.5"`
	RestorePoint bool      `json:"restore_point"`
}

// BackupListDTO wraps the dump list.
type BackupListDTO struct {
	Backups []BackupEntryDTO `json:"backups"`
}

func ToBackupListDTO(entries []service.BackupEntry) BackupListDTO {
	out := BackupListDTO{Backups: make([]BackupEntryDTO, 0, len(entries))}
	for _, e := range entries {
		out.Backups = append(out.Backups, BackupEntryDTO{
			Name:         e.Name,
			SizeBytes:    e.SizeBytes,
			CreatedAt:    e.CreatedAt,
			AppVersion:   e.AppVersion,
			PGVersion:    e.PGVersion,
			RestorePoint: e.RestorePoint,
		})
	}
	return out
}
