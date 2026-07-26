// Package backup creates, validates, retains, and stages consistent SQLite
// library snapshots. It never copies a live WAL database directly.
package backup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"server/internal/db"
	"server/platform/fsprivacy"

	sqlite3 "github.com/mattn/go-sqlite3"
)

const manifestFormatVersion = 1

// Logf matches the supervisor-style logging callback used across the app.
type Logf func(format string, args ...any)

// SnapshotMetadata is supplied by the runtime for provenance that is not
// stored inside the database itself.
type SnapshotMetadata struct {
	AppVersion          string
	ConfigSchemaVersion int
}

// Manifest is the checksum and compatibility contract paired with every
// SQLite snapshot.
type Manifest struct {
	FormatVersion        int       `json:"format_version"`
	AppVersion           string    `json:"app_version"`
	ConfigSchemaVersion  int       `json:"config_schema_version"`
	ApplicationMigration int64     `json:"application_migration_version"`
	RiverMigration       int64     `json:"river_migration_version"`
	SQLiteVersion        string    `json:"sqlite_version"`
	VectorVersion        string    `json:"sqlite_vec_version"`
	CreatedAt            time.Time `json:"created_at"`
	DatabaseSize         int64     `json:"database_size"`
	SHA256               string    `json:"sha256"`
	QuickCheck           string    `json:"quick_check"`
	ForeignKeyViolations int       `json:"foreign_key_violations"`
	LibraryID            string    `json:"library_id"`
}

// Snapshot is a finalized database and manifest pair.
type Snapshot struct {
	Path         string
	ManifestPath string
	Manifest     Manifest
}

// Compatibility constrains which snapshot may be staged over the active
// runtime. Older schemas are allowed because the next generation migrates
// them; future schemas are rejected.
type Compatibility struct {
	LibraryID               string
	ConfigSchemaVersion     int
	MaxApplicationMigration int64
	MaxRiverMigration       int64
}

// CreateSnapshot uses SQLite's Online Backup API to create a transactionally
// consistent standalone database, validates it through an independent
// connection, writes a checksum manifest, and atomically finalizes both files.
func CreateSnapshot(
	ctx context.Context,
	source *sql.DB,
	destDir string,
	prefix string,
	metadata SnapshotMetadata,
	logf Logf,
) (Snapshot, error) {
	if source == nil {
		return Snapshot{}, errors.New("SQLite backup source is nil")
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return Snapshot{}, fmt.Errorf("create backup directory %s: %w", destDir, err)
	}
	if err := fsprivacy.ApplyDirectoryMode(destDir, 0o700); err != nil {
		return Snapshot{}, fmt.Errorf("secure backup directory %s: %w", destDir, err)
	}

	createdAt := time.Now().UTC()
	name := prefix + FileName(createdAt)
	finalPath := filepath.Join(destDir, name)
	finalManifestPath := ManifestPath(finalPath)
	tmpPath := finalPath + TmpSuffix
	tmpManifestPath := finalManifestPath + TmpSuffix

	for _, path := range []string{finalPath, finalManifestPath, tmpPath, tmpManifestPath} {
		if _, err := os.Stat(path); err == nil {
			return Snapshot{}, fmt.Errorf("backup artifact already exists: %s", filepath.Base(path))
		} else if !errors.Is(err, os.ErrNotExist) {
			return Snapshot{}, fmt.Errorf("inspect backup artifact %s: %w", filepath.Base(path), err)
		}
	}

	cleanup := func() {
		_ = os.Remove(tmpPath)
		_ = os.Remove(tmpManifestPath)
	}
	defer cleanup()

	logf("backup: creating SQLite snapshot %s", name)
	if err := onlineBackup(ctx, source, tmpPath); err != nil {
		return Snapshot{}, err
	}
	if err := fsprivacy.ApplyFileMode(tmpPath, 0o600); err != nil {
		return Snapshot{}, fmt.Errorf("secure SQLite snapshot: %w", err)
	}
	if err := syncFile(tmpPath); err != nil {
		return Snapshot{}, err
	}

	info, err := db.InspectStandaloneCatalog(ctx, tmpPath)
	if err != nil {
		return Snapshot{}, fmt.Errorf("validate SQLite snapshot: %w", err)
	}
	checksum, err := fileSHA256(tmpPath)
	if err != nil {
		return Snapshot{}, err
	}
	manifest := Manifest{
		FormatVersion:        manifestFormatVersion,
		AppVersion:           metadata.AppVersion,
		ConfigSchemaVersion:  metadata.ConfigSchemaVersion,
		ApplicationMigration: info.ApplicationMigration,
		RiverMigration:       info.RiverMigration,
		SQLiteVersion:        info.SQLiteVersion,
		VectorVersion:        info.VectorVersion,
		CreatedAt:            createdAt,
		DatabaseSize:         info.SizeBytes,
		SHA256:               checksum,
		QuickCheck:           info.QuickCheck,
		ForeignKeyViolations: info.ForeignKeyViolationCount,
		LibraryID:            info.LibraryID,
	}
	if err := writeManifest(tmpManifestPath, manifest); err != nil {
		return Snapshot{}, err
	}

	if err := renameFile(tmpPath, finalPath); err != nil {
		return Snapshot{}, fmt.Errorf("finalize SQLite snapshot: %w", err)
	}
	if err := renameFile(tmpManifestPath, finalManifestPath); err != nil {
		_ = os.Remove(finalPath)
		return Snapshot{}, fmt.Errorf("finalize SQLite snapshot manifest: %w", err)
	}
	if err := syncDirectory(destDir); err != nil {
		return Snapshot{}, err
	}

	logf("backup: wrote %s and %s", finalPath, finalManifestPath)
	return Snapshot{Path: finalPath, ManifestPath: finalManifestPath, Manifest: manifest}, nil
}

func onlineBackup(ctx context.Context, source *sql.DB, destinationPath string) error {
	destination, err := sql.Open("sqlite3", destinationPath)
	if err != nil {
		return fmt.Errorf("open SQLite snapshot destination: %w", err)
	}
	destination.SetMaxOpenConns(1)
	destination.SetMaxIdleConns(1)
	defer destination.Close()

	sourceConn, err := source.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire SQLite snapshot source connection: %w", err)
	}
	defer sourceConn.Close()
	destinationConn, err := destination.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire SQLite snapshot destination connection: %w", err)
	}
	defer destinationConn.Close()

	err = sourceConn.Raw(func(sourceDriverConn any) error {
		sourceSQLite, ok := sourceDriverConn.(*sqlite3.SQLiteConn)
		if !ok {
			return fmt.Errorf("backup source driver is %T, want *sqlite3.SQLiteConn", sourceDriverConn)
		}
		return destinationConn.Raw(func(destinationDriverConn any) error {
			destinationSQLite, ok := destinationDriverConn.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("backup destination driver is %T, want *sqlite3.SQLiteConn", destinationDriverConn)
			}
			backup, err := destinationSQLite.Backup("main", sourceSQLite, "main")
			if err != nil {
				return fmt.Errorf("initialize SQLite online backup: %w", err)
			}
			for {
				done, stepErr := backup.Step(256)
				if stepErr != nil {
					_ = backup.Close()
					return fmt.Errorf("copy SQLite snapshot pages: %w", stepErr)
				}
				if done {
					break
				}
				select {
				case <-ctx.Done():
					_ = backup.Close()
					return ctx.Err()
				case <-time.After(10 * time.Millisecond):
				}
			}
			if err := backup.Close(); err != nil {
				return fmt.Errorf("finalize SQLite online backup: %w", err)
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	if err := destinationConn.Close(); err != nil {
		return fmt.Errorf("release SQLite snapshot destination connection: %w", err)
	}
	if err := sourceConn.Close(); err != nil {
		return fmt.Errorf("release SQLite snapshot source connection: %w", err)
	}
	var busy, logPages, checkpointed int
	if err := destination.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(
		&busy,
		&logPages,
		&checkpointed,
	); err != nil {
		return fmt.Errorf("checkpoint SQLite snapshot destination: %w", err)
	}
	if busy != 0 {
		return fmt.Errorf(
			"checkpoint SQLite snapshot destination remained busy: log_pages=%d checkpointed=%d",
			logPages,
			checkpointed,
		)
	}
	var journalMode string
	if err := destination.QueryRowContext(ctx, "PRAGMA journal_mode=DELETE").Scan(&journalMode); err != nil {
		return fmt.Errorf("finalize SQLite snapshot journal mode: %w", err)
	}
	if journalMode != "delete" {
		return fmt.Errorf("SQLite snapshot journal mode = %q, want delete", journalMode)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("close SQLite snapshot destination: %w", err)
	}
	if err := removeSQLiteSidecars(destinationPath); err != nil {
		return err
	}
	return nil
}

// ValidateSnapshot verifies the manifest, checksum, SQLite identity, integrity,
// and schema compatibility before a restore marker can be written.
func ValidateSnapshot(ctx context.Context, snapshotPath string, compatibility Compatibility) (Manifest, db.CatalogInfo, error) {
	manifest, err := ReadManifest(ManifestPath(snapshotPath))
	if err != nil {
		return Manifest{}, db.CatalogInfo{}, err
	}
	if manifest.FormatVersion != manifestFormatVersion {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf("snapshot manifest format %d is unsupported", manifest.FormatVersion)
	}
	fileInfo, err := os.Stat(snapshotPath)
	if err != nil {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf("stat SQLite snapshot: %w", err)
	}
	if fileInfo.Size() != manifest.DatabaseSize {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf("SQLite snapshot size = %d, manifest = %d", fileInfo.Size(), manifest.DatabaseSize)
	}
	checksum, err := fileSHA256(snapshotPath)
	if err != nil {
		return Manifest{}, db.CatalogInfo{}, err
	}
	if checksum != manifest.SHA256 {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf("SQLite snapshot checksum mismatch")
	}

	info, err := db.InspectStandaloneCatalog(ctx, snapshotPath)
	if err != nil {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf("inspect SQLite snapshot: %w", err)
	}
	if info.LibraryID != manifest.LibraryID ||
		info.ApplicationMigration != manifest.ApplicationMigration ||
		info.RiverMigration != manifest.RiverMigration ||
		info.SizeBytes != manifest.DatabaseSize {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf("SQLite snapshot does not match its manifest")
	}
	if compatibility.LibraryID != "" && info.LibraryID != compatibility.LibraryID {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf("snapshot belongs to library %s, active library is %s", info.LibraryID, compatibility.LibraryID)
	}
	if compatibility.ConfigSchemaVersion != 0 && manifest.ConfigSchemaVersion != compatibility.ConfigSchemaVersion {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf(
			"snapshot config schema %d is incompatible with runtime schema %d",
			manifest.ConfigSchemaVersion,
			compatibility.ConfigSchemaVersion,
		)
	}
	if compatibility.MaxApplicationMigration != 0 && info.ApplicationMigration > compatibility.MaxApplicationMigration {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf("snapshot application migration %d is newer than runtime %d", info.ApplicationMigration, compatibility.MaxApplicationMigration)
	}
	if compatibility.MaxRiverMigration != 0 && info.RiverMigration > compatibility.MaxRiverMigration {
		return Manifest{}, db.CatalogInfo{}, fmt.Errorf("snapshot River migration %d is newer than runtime %d", info.RiverMigration, compatibility.MaxRiverMigration)
	}
	return manifest, info, nil
}

// ReadManifest loads a snapshot sidecar.
func ReadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read SQLite snapshot manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode SQLite snapshot manifest: %w", err)
	}
	return manifest, nil
}

func writeManifest(path string, manifest Manifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode SQLite snapshot manifest: %w", err)
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create SQLite snapshot manifest: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write SQLite snapshot manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync SQLite snapshot manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close SQLite snapshot manifest: %w", err)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open SQLite snapshot for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("checksum SQLite snapshot: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func syncFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open SQLite snapshot for sync: %w", err)
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync SQLite snapshot: %w", err)
	}
	return nil
}

// Prune enforces count-based retention on routine snapshot pairs and removes
// stale temporary artifacts. Restore points are never pruned here.
func Prune(dir string, keep int, logf Logf) ([]string, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if keep < 1 {
		keep = 1
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read backup directory %s: %w", dir, err)
	}

	var routine []string
	var removed []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		switch {
		case IsRoutineName(name):
			routine = append(routine, name)
		case filepath.Ext(name) == TmpSuffix:
			if info, infoErr := entry.Info(); infoErr == nil && time.Since(info.ModTime()) > time.Hour {
				if removeErr := os.Remove(filepath.Join(dir, name)); removeErr == nil {
					removed = append(removed, name)
				}
			}
		}
	}

	sort.Sort(sort.Reverse(sort.StringSlice(routine)))
	for _, name := range routine[min(keep, len(routine)):] {
		for _, artifact := range []string{name, ManifestName(name)} {
			if err := os.Remove(filepath.Join(dir, artifact)); err != nil && !errors.Is(err, os.ErrNotExist) {
				return removed, fmt.Errorf("prune %s: %w", artifact, err)
			}
		}
		removed = append(removed, name)
		logf("backup: pruned %s", name)
	}
	return removed, nil
}

// LatestRoutine returns the newest completed routine snapshot time.
func LatestRoutine(dir string) (time.Time, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return time.Time{}, false
	}
	var latest time.Time
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, ok := ParseName(entry.Name())
		if !ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, ManifestName(entry.Name()))); err != nil {
			continue
		}
		if info.CreatedAt.After(latest) {
			latest = info.CreatedAt
		}
	}
	return latest, !latest.IsZero()
}
