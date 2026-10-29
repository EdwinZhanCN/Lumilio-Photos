package recovery

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/scan"
	"server/internal/storage/trash"
)

// Errors a recovery action answers with before touching anything.
var (
	ErrItemNotFound      = errors.New("recovery item not found")
	ErrItemNotDeletable  = errors.New("recovery item cannot be deleted from the inventory")
	ErrRepositoryOffline = errors.New("repository is not reachable")
)

// studioHashAlgorithm is the algorithm a Studio record's source hash uses;
// the record carries the hex digest without naming it.
const studioHashAlgorithm = "blake3-v1"

// Request names the actor of an explicit recovery action.
type Request struct {
	Actor       string
	ActorUserID *int32
	RequestID   string
}

// Service reads recovery receipts and acts on them.
type Service struct {
	reader  *repo.Queries
	writer  scan.Writer
	files   *storage.RepositoryFSFactory
	scanner *scan.Scanner
	trash   *trash.Service
	logger  *zap.Logger
	now     func() time.Time
	// mu serializes reconciliation and item actions over receipts: each
	// rewrites the receipt it changes.
	mu sync.Mutex
	// afterStudioWrite runs after a reassociated record is written and
	// before the old name is removed; tests inject a crash with it.
	afterStudioWrite func() error
}

// New builds the service over the catalog reader, the scan writer (audits
// commit through it), repository files, the scanner whose path keys the
// index uses, and the Trash it rebuilds.
func New(reader *repo.Queries, writer scan.Writer, files *storage.RepositoryFSFactory, scanner *scan.Scanner, assetTrash *trash.Service, logger *zap.Logger) (*Service, error) {
	if reader == nil || writer == nil || files == nil || scanner == nil || assetTrash == nil {
		return nil, errors.New("private recovery needs a reader, a writer, repository files, the scanner, and the trash")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{reader: reader, writer: writer, files: files, scanner: scanner, trash: assetTrash, logger: logger,
		now: func() time.Time { return time.Now().UTC() }}, nil
}

// TrashHeld counts the inherited trashed files still held in a repository.
type TrashHeld struct {
	Count int64
	Bytes int64
}

// ReceiptSummary is one receipt without its items.
type ReceiptSummary struct {
	ReceiptID          string
	Event              storage.RecoveryEvent
	RecordedAt         time.Time
	Actor              string
	SourceRepositoryID string
	RepositoryID       string
	Trash              storage.RecoverySummary
	Logs               storage.RecoverySummary
	Derived            storage.RecoverySummary
	ItemsTruncated     int
}

// Item is one listed file with its current state.
type Item struct {
	storage.RecoveryItem
	ReceiptID string
	// Present reports whether the bytes are where the receipt says they are.
	Present bool
	// Actions are the explicit decisions available: "export", "delete".
	Actions []string
}

// Inventory is what the Storage page shows for one repository.
type Inventory struct {
	RepositoryID       uuid.UUID
	Receipts           []ReceiptSummary
	Items              []Item
	TrashHeld          TrashHeld
	UnreadableReceipts []string
}

// Inventory reads the receipts of a reachable repository and reports every
// listed item with its presence, plus the held Trash counts.
func (s *Service) Inventory(ctx context.Context, repositoryID uuid.UUID) (Inventory, error) {
	inventory := Inventory{RepositoryID: repositoryID, Receipts: []ReceiptSummary{}, Items: []Item{}, UnreadableReceipts: []string{}}
	counts, err := s.reader.CountRepositoryLifecycle(ctx, repositoryID)
	if err != nil {
		return inventory, fmt.Errorf("count held trash: %w", err)
	}
	inventory.TrashHeld = TrashHeld{Count: counts.TrashHeldCount, Bytes: counts.TrashHeldBytes}
	files, err := s.open(ctx, repositoryID)
	if err != nil {
		return inventory, err
	}
	defer files.Close()
	receipts, unreadable, err := storage.ListRecoveryReceipts(ctx, files)
	if err != nil {
		return inventory, fmt.Errorf("read recovery receipts: %w", err)
	}
	inventory.UnreadableReceipts = append(inventory.UnreadableReceipts, unreadable...)
	for _, receipt := range receipts {
		inventory.Receipts = append(inventory.Receipts, summarize(receipt))
		for _, item := range receipt.Items {
			inventory.Items = append(inventory.Items, s.describe(files, receipt.ReceiptID, item))
		}
	}
	inventory.Items = sortedItems(inventory.Items)
	return inventory, nil
}

func summarize(receipt storage.RecoveryReceipt) ReceiptSummary {
	return ReceiptSummary{
		ReceiptID: receipt.ReceiptID, Event: receipt.Event, RecordedAt: receipt.RecordedAt, Actor: receipt.Actor,
		SourceRepositoryID: receipt.SourceRepositoryID, RepositoryID: receipt.RepositoryID,
		Trash: receipt.Trash, Logs: receipt.Logs, Derived: receipt.Derived, ItemsTruncated: receipt.ItemsTruncated,
	}
}

func (s *Service) describe(files *storage.RepositoryFS, receiptID string, item storage.RecoveryItem) Item {
	described := Item{RecoveryItem: item, ReceiptID: receiptID, Actions: []string{}}
	if item.Disposition == storage.DispositionDeleted {
		return described
	}
	current := item.CurrentPath()
	if item.Disposition == storage.DispositionReassociated && item.ResolvedAssetID != "" {
		current = studioRecordPath(item.ResolvedAssetID)
	}
	if target, err := storage.ParsePrivateRepositoryPath(current); err == nil {
		if info, err := files.StatPrivate(target); err == nil && info.Mode().IsRegular() {
			described.Present = true
		}
	}
	if described.Present && item.Disposition != storage.DispositionReassociated {
		described.Actions = append(described.Actions, "export")
		if deletable(item) {
			described.Actions = append(described.Actions, "delete")
		}
	}
	return described
}

// deletable: staging, unknown, and undecided or unreadable Studio records
// may be removed explicitly; a reassociated record is live Studio state.
func deletable(item storage.RecoveryItem) bool {
	switch item.Class {
	case storage.PrivateClassStaging, storage.PrivateClassUnknown:
		return true
	case storage.PrivateClassStudio:
		return item.Disposition != storage.DispositionReassociated
	}
	return false
}

func studioRecordPath(assetID string) string {
	return path.Join(storage.DefaultStructure.SidecarsDir, assetID+".lumilio-sidecar")
}

// ReconcileResult counts what one pass changed.
type ReconcileResult struct {
	TrashAdopted       int
	StudioReassociated int
	StudioAmbiguous    int
}

func (r *ReconcileResult) add(other ReconcileResult) {
	r.TrashAdopted += other.TrashAdopted
	r.StudioReassociated += other.StudioReassociated
	r.StudioAmbiguous += other.StudioAmbiguous
}

// ReconcileAll reconciles every reachable repository; an unreachable one
// waits for the next pass.
func (s *Service) ReconcileAll(ctx context.Context) (ReconcileResult, error) {
	var result ReconcileResult
	repositories, err := s.reader.ListRepositories(ctx)
	if err != nil {
		return result, fmt.Errorf("list repositories for private recovery: %w", err)
	}
	var failures []error
	for _, repository := range repositories {
		if repository.Reachability != dbtypes.RepositoryReachabilityActive {
			continue
		}
		one, err := s.Reconcile(ctx, repository.RepoID)
		result.add(one)
		if err != nil && !errors.Is(err, ErrRepositoryOffline) {
			failures = append(failures, err)
		}
	}
	return result, errors.Join(failures...)
}

// Reconcile adopts the inherited Trash (held) and reassociates the Studio
// records whose source content is proven. It is safe to run at any time and
// repeatedly; Studio records wait while the repository still has files to
// hash, because a match must be proven, never inferred.
func (s *Service) Reconcile(ctx context.Context, repositoryID uuid.UUID) (ReconcileResult, error) {
	var result ReconcileResult
	adopted, err := s.trash.Rebuild(ctx, repositoryID)
	result.TrashAdopted = adopted
	if err != nil {
		return result, fmt.Errorf("rebuild inherited trash: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := s.open(ctx, repositoryID)
	if err != nil {
		return result, err
	}
	defer files.Close()
	receipts, _, err := storage.ListRecoveryReceipts(ctx, files)
	if err != nil {
		return result, fmt.Errorf("read recovery receipts: %w", err)
	}
	proven := map[string]bool{}
	for index := range receipts {
		receipt := &receipts[index]
		changed := false
		persist := func() error {
			if err := storage.WriteRecoveryReceipt(files, *receipt); err != nil {
				return fmt.Errorf("record Studio dispositions: %w", err)
			}
			changed = false
			return nil
		}
		for itemIndex := range receipt.Items {
			item := &receipt.Items[itemIndex]
			if item.Class != storage.PrivateClassStudio {
				continue
			}
			if item.Disposition == storage.DispositionReassociated {
				// A crash between the receipt and the removal of the old name
				// leaves the superseded record behind; finish that here.
				if err := s.removeSupersededRecord(files, *item); err != nil {
					return result, err
				}
				continue
			}
			if item.Disposition != storage.DispositionPreserved {
				continue
			}
			ready, known := proven[receipt.ReceiptID]
			if !known {
				ready, err = s.contentProven(ctx, repositoryID, receipt.RecordedAt)
				if err != nil {
					return result, err
				}
				proven[receipt.ReceiptID] = ready
			}
			if !ready {
				break
			}
			outcome, err := s.reassociate(ctx, files, repositoryID, item, persist)
			if err != nil {
				return result, err
			}
			switch outcome {
			case storage.DispositionReassociated:
				result.StudioReassociated++
				changed = true
			case storage.DispositionAmbiguous, storage.DispositionUnreadable:
				result.StudioAmbiguous++
				changed = true
			}
		}
		if changed {
			if err := persist(); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

// contentProven reports whether the repository's content is known well
// enough to decide a Studio record written before recordedAt: a scan
// finished after the receipt and no file is left to hash. Before that, a
// record that finds no match simply waits.
func (s *Service) contentProven(ctx context.Context, repositoryID uuid.UUID, recordedAt time.Time) (bool, error) {
	latest, err := s.reader.GetLatestRepositoryScan(ctx, repositoryID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if latest.Status != scan.StatusCompleted || !latest.FinishedAt.Valid || latest.FinishedAt.Time.Before(recordedAt) {
		return false, nil
	}
	rows, err := s.reader.ListPendingHashRepositoryEntries(ctx, repo.ListPendingHashRepositoryEntriesParams{RepositoryID: repositoryID, Limit: 1})
	if err != nil {
		return false, err
	}
	return len(rows) == 0, nil
}

// removeSupersededRecord drops the old name of a reassociated record when
// the new one is in place.
func (s *Service) removeSupersededRecord(files *storage.RepositoryFS, item storage.RecoveryItem) error {
	if item.ResolvedAssetID == "" || item.Path == studioRecordPath(item.ResolvedAssetID) {
		return nil
	}
	target, err := storage.ParsePrivateRepositoryPath(studioRecordPath(item.ResolvedAssetID))
	if err != nil {
		return err
	}
	source, err := storage.ParsePrivateRepositoryPath(item.Path)
	if err != nil {
		return err
	}
	if exists, err := files.Exists(target); err != nil || !exists {
		return err
	}
	if err := files.RemovePrivate(source); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove superseded Studio record: %w", err)
	}
	return nil
}

// studioRecord is the part of a Studio record reassociation reads.
type studioRecord struct {
	Version int    `json:"version"`
	AssetID string `json:"asset_id"`
	Source  struct {
		StoragePath string  `json:"storage_path"`
		FileSize    int64   `json:"file_size"`
		Hash        *string `json:"hash"`
	} `json:"source"`
}

// reassociate decides one preserved Studio record. It returns the item's
// new disposition, or DispositionPreserved when the decision must wait. A
// proven match is written under the Asset's name, recorded in the receipt
// through persist, and only then removed under its old name, so a crash at
// any point is finished by the next pass without a second move.
func (s *Service) reassociate(ctx context.Context, files *storage.RepositoryFS, repositoryID uuid.UUID, item *storage.RecoveryItem, persist func() error) (storage.RecoveryDisposition, error) {
	source, err := storage.ParsePrivateRepositoryPath(item.Path)
	if err != nil {
		return s.settle(item, storage.DispositionAmbiguous, "", "record path is not portable"), nil
	}
	data, err := files.ReadPrivateFile(source)
	if errors.Is(err, fs.ErrNotExist) {
		// A record that vanished stays undecided; nothing is guessed.
		return storage.DispositionPreserved, nil
	}
	if err != nil {
		return storage.DispositionPreserved, fmt.Errorf("read Studio record %s: %w", item.Path, err)
	}
	var record studioRecord
	if err := json.Unmarshal(data, &record); err != nil || record.Version != storage.StudioSidecarVersion {
		return s.settle(item, storage.DispositionUnreadable, "", "not a Studio record this build reads"), nil
	}
	if record.Source.Hash == nil || len(*record.Source.Hash) != 64 || record.Source.FileSize < 0 {
		return s.settle(item, storage.DispositionAmbiguous, "", "the record names no content hash"), nil
	}
	hash := strings.ToLower(*record.Source.Hash)
	entries, err := s.reader.ListPresentRepositoryEntriesByContent(ctx, repo.ListPresentRepositoryEntriesByContentParams{
		RepositoryID: repositoryID, HashAlgorithm: studioHashAlgorithm, FullHash: hash, FileSize: record.Source.FileSize,
	})
	if err != nil {
		return storage.DispositionPreserved, err
	}
	assets := map[uuid.UUID]string{}
	for _, entry := range entries {
		if entry.AssetID.Valid {
			if _, seen := assets[entry.AssetID.UUID]; !seen {
				assets[entry.AssetID.UUID] = entry.Path
			}
		}
	}
	if len(assets) == 0 {
		return s.settle(item, storage.DispositionAmbiguous, "", "no present file in the Repository has the record's content"), nil
	}
	if len(assets) > 1 {
		return s.settle(item, storage.DispositionAmbiguous, "", "the record's content is bound to more than one Asset"), nil
	}
	var assetID uuid.UUID
	var matchedPath string
	for id, matched := range assets {
		assetID, matchedPath = id, matched
	}
	// A different file at the record's source path means the source was
	// replaced; the record is not applied to whatever stands there now.
	if _, err := storage.ParseUserMediaPath(record.Source.StoragePath); err == nil {
		if key, keyErr := s.scanner.PathKey(ctx, repositoryID, record.Source.StoragePath); keyErr == nil {
			if live, err := s.reader.GetLiveRepositoryEntryByKey(ctx, repo.GetLiveRepositoryEntryByKeyParams{RepositoryID: repositoryID, PathKey: key}); err == nil {
				if live.Kind == scan.KindFile && live.AssetID.Valid && live.AssetID.UUID != assetID {
					return s.settle(item, storage.DispositionAmbiguous, "", "the file at the record's source path has different content"), nil
				}
			} else if !errors.Is(err, sql.ErrNoRows) {
				return storage.DispositionPreserved, err
			}
		}
	}
	target, err := storage.ParsePrivateRepositoryPath(studioRecordPath(assetID.String()))
	if err != nil {
		return storage.DispositionPreserved, err
	}
	if target.String() == source.String() {
		return s.settle(item, storage.DispositionReassociated, assetID.String(), ""), nil
	}
	rewritten, err := rewriteStudioRecord(data, assetID.String(), matchedPath)
	if err != nil {
		return s.settle(item, storage.DispositionUnreadable, "", err.Error()), nil
	}
	if _, err := files.WritePrivateFileImmutable(target, bytes.NewReader(rewritten), 0o644); err != nil {
		if errors.Is(err, storage.ErrRepositoryDestinationExists) || errors.Is(err, fs.ErrExist) {
			return s.settle(item, storage.DispositionAmbiguous, "", "the Asset already has a different Studio record"), nil
		}
		return storage.DispositionPreserved, fmt.Errorf("write reassociated Studio record: %w", err)
	}
	// The receipt names the new record before the old name goes, so a crash
	// here is finished by the next pass (the target is written, the source
	// is removed by removeSupersededRecord).
	s.settle(item, storage.DispositionReassociated, assetID.String(), "")
	if err := persist(); err != nil {
		return storage.DispositionPreserved, err
	}
	if s.afterStudioWrite != nil {
		if err := s.afterStudioWrite(); err != nil {
			return storage.DispositionPreserved, err
		}
	}
	if err := files.RemovePrivate(source); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return storage.DispositionPreserved, fmt.Errorf("remove superseded Studio record: %w", err)
	}
	if err := s.audit(ctx, Request{Actor: "server"}, "studio_reassociated", "asset", assetID.String(), map[string]any{
		"repository_id": repositoryID.String(), "record": item.Path, "previous_asset_id": record.AssetID, "source_path": matchedPath,
	}); err != nil {
		return storage.DispositionPreserved, err
	}
	return storage.DispositionReassociated, nil
}

func (s *Service) settle(item *storage.RecoveryItem, disposition storage.RecoveryDisposition, assetID, detail string) storage.RecoveryDisposition {
	now := s.now()
	item.Disposition = disposition
	item.ResolvedAssetID = assetID
	item.ResolvedAt = &now
	item.Detail = detail
	return disposition
}

// rewriteStudioRecord gives a record its proven Asset and source path,
// keeping every other field, including ones this build does not know.
func rewriteStudioRecord(data []byte, assetID, storagePath string) ([]byte, error) {
	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("Studio record is not an object: %v", err)
	}
	record["asset_id"] = assetID
	if source, ok := record["source"].(map[string]any); ok && storagePath != "" {
		source["storage_path"] = storagePath
		record["source"] = source
	}
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// OpenItem opens a listed item's bytes for export. The returned reader keeps
// the repository lease until it is closed.
func (s *Service) OpenItem(ctx context.Context, repositoryID uuid.UUID, itemID string) (io.ReadCloser, Item, error) {
	files, err := s.open(ctx, repositoryID)
	if err != nil {
		return nil, Item{}, err
	}
	_, item, err := s.find(ctx, files, itemID)
	if err != nil {
		_ = files.Close()
		return nil, Item{}, err
	}
	if !item.Present || item.Disposition == storage.DispositionReassociated {
		_ = files.Close()
		return nil, item, fmt.Errorf("%w: %s has no exportable bytes", ErrItemNotFound, itemID)
	}
	target, err := storage.ParsePrivateRepositoryPath(item.CurrentPath())
	if err != nil {
		_ = files.Close()
		return nil, item, err
	}
	file, err := files.OpenPrivate(target)
	if err != nil {
		_ = files.Close()
		return nil, item, err
	}
	return &leasedFile{file: file, release: files.Close}, item, nil
}

// leasedFile keeps the repository lease for as long as the export reads.
type leasedFile struct {
	file    io.ReadCloser
	release func() error
}

func (f *leasedFile) Read(buffer []byte) (int, error) { return f.file.Read(buffer) }

func (f *leasedFile) Close() error {
	return errors.Join(f.file.Close(), f.release())
}

// DeleteItem removes one listed file explicitly and records that in the
// receipt and the audit log. Only staging, unknown, and undecided Studio
// items can be deleted; a reassociated record is live Studio state.
func (s *Service) DeleteItem(ctx context.Context, repositoryID uuid.UUID, itemID string, request Request) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := s.open(ctx, repositoryID)
	if err != nil {
		return Item{}, err
	}
	defer files.Close()
	receipt, item, err := s.find(ctx, files, itemID)
	if err != nil {
		return Item{}, err
	}
	if item.Disposition == storage.DispositionDeleted {
		return item, nil
	}
	if !deletable(item.RecoveryItem) {
		return item, fmt.Errorf("%w: %s is %s", ErrItemNotDeletable, item.Class, item.Disposition)
	}
	target, err := storage.ParsePrivateRepositoryPath(item.CurrentPath())
	if err != nil {
		return item, err
	}
	if err := files.RemovePrivate(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return item, fmt.Errorf("delete recovery item: %w", err)
	}
	for index := range receipt.Items {
		if receipt.Items[index].ItemID == itemID {
			s.settle(&receipt.Items[index], storage.DispositionDeleted, "", "")
			item.RecoveryItem = receipt.Items[index]
		}
	}
	if err := storage.WriteRecoveryReceipt(files, receipt); err != nil {
		return item, fmt.Errorf("record recovery item deletion: %w", err)
	}
	item.Present = false
	item.Actions = []string{}
	if err := s.audit(ctx, request, "recovery_item_deleted", "repository", repositoryID.String(), map[string]any{
		"receipt_id": receipt.ReceiptID, "item_id": itemID, "class": string(item.Class), "path": item.Path, "size": item.Size,
	}); err != nil {
		return item, err
	}
	return item, nil
}

func (s *Service) find(ctx context.Context, files *storage.RepositoryFS, itemID string) (storage.RecoveryReceipt, Item, error) {
	receipts, _, err := storage.ListRecoveryReceipts(ctx, files)
	if err != nil {
		return storage.RecoveryReceipt{}, Item{}, fmt.Errorf("read recovery receipts: %w", err)
	}
	for _, receipt := range receipts {
		for _, item := range receipt.Items {
			if item.ItemID == itemID {
				return receipt, s.describe(files, receipt.ReceiptID, item), nil
			}
		}
	}
	return storage.RecoveryReceipt{}, Item{}, fmt.Errorf("%w: %s", ErrItemNotFound, itemID)
}

func (s *Service) open(ctx context.Context, repositoryID uuid.UUID) (*storage.RepositoryFS, error) {
	repository, err := s.reader.GetRepository(ctx, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRepositoryOffline, err)
	}
	if repository.Reachability != dbtypes.RepositoryReachabilityActive {
		return nil, fmt.Errorf("%w: reachability is %s", ErrRepositoryOffline, repository.Reachability)
	}
	files, err := s.files.OpenContext(ctx, repository)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRepositoryOffline, err)
	}
	if err := files.VerifyIdentity(); err != nil {
		_ = files.Close()
		return nil, fmt.Errorf("%w: %v", ErrRepositoryOffline, err)
	}
	return files, nil
}

func (s *Service) audit(ctx context.Context, request Request, action, targetType, targetID string, details map[string]any) error {
	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		actor = "server"
	}
	return s.writer.WithTx(ctx, catalogtx.OperationRepositoryRecovery, func(_ *sql.Tx, queries *repo.Queries) error {
		_, err := storage.RecordLifecycleAuditTx(ctx, queries, storage.LifecycleAuditInput{
			Actor: actor, ActorUserID: request.ActorUserID, RequestID: request.RequestID,
			Action: action, TargetType: targetType, TargetID: targetID,
			Result: storage.AuditResultSucceeded, Details: details,
		})
		return err
	})
}

// sortedItems orders items by class, then path, for stable presentation.
func sortedItems(items []Item) []Item {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Class != items[j].Class {
			return items[i].Class < items[j].Class
		}
		return items[i].Path < items[j].Path
	})
	return items
}
