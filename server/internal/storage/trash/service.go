// Package trash moves deleted Assets' files into their repository's trash
// and back. Delete and Restore journal every move through
// lifecycle_operations (prepared, filesystem_applied, catalog_committed) so a
// crash between a move and its catalog commit is reconciled on startup, and
// each outcome is recorded in lifecycle_audit_events. No file I/O happens
// inside a catalog transaction.
package trash

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/event"
	"server/internal/storage"
	"server/internal/storage/scan"
)

// ErrRejected is the class of every Delete or Restore refused before any
// file moved; the catalog is unchanged.
var ErrRejected = errors.New("lifecycle request rejected")

// Reasons a request is rejected.
const (
	ReasonRepositoryOffline = "repository_offline"
	ReasonFileChanged       = "file_changed"
	ReasonAssetMissing      = "asset_missing"
	ReasonNotTrashed        = "not_trashed"
	ReasonTrashFileMissing  = "trash_file_missing"
	ReasonMoveFailed        = "move_failed"
)

// Rejection explains why a Delete or Restore moved nothing.
type Rejection struct {
	Reason       string
	AssetID      uuid.UUID
	RepositoryID uuid.UUID
	Path         string
	Cause        error
}

func (r *Rejection) Error() string {
	message := "lifecycle request rejected: " + r.Reason
	if r.Path != "" {
		message += " (" + r.Path + ")"
	}
	if r.Cause != nil {
		message += ": " + r.Cause.Error()
	}
	return message
}

func (r *Rejection) Is(target error) bool { return target == ErrRejected }
func (r *Rejection) Unwrap() error        { return r.Cause }

// Request names the Assets a user deletes or restores. Authorization is the
// caller's.
type Request struct {
	AssetIDs         []uuid.UUID
	Actor            string
	ActorUserID      *int32
	RequestID        string
	ConfirmationType string
}

// Service owns the repository trash.
type Service struct {
	reader  *repo.Queries
	writer  scan.Writer
	files   *storage.RepositoryFSFactory
	scanner *scan.Scanner
	logger  *zap.Logger
	now     func() time.Time
	// mu serializes Delete, Restore, and recovery: they are user actions,
	// and one at a time keeps two of them from racing over the same files.
	mu sync.Mutex
	// afterMove runs after each file move; tests inject a crash with it.
	afterMove func(moved int) error
}

func New(reader *repo.Queries, writer scan.Writer, files *storage.RepositoryFSFactory, scanner *scan.Scanner, logger *zap.Logger) (*Service, error) {
	if reader == nil || writer == nil || files == nil || scanner == nil {
		return nil, errors.New("the repository trash needs a reader, a writer, repository files, and the scanner")
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		reader: reader, writer: writer, files: files, scanner: scanner, logger: logger,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

// repositories opens every repository a request touches, refusing when one
// is not reachable or its marker does not match.
type repositories map[uuid.UUID]*storage.RepositoryFS

func (r repositories) close() {
	for _, files := range r {
		_ = files.Close()
	}
}

func (s *Service) openRepositories(ctx context.Context, ids []uuid.UUID) (repositories, error) {
	opened := repositories{}
	for _, id := range ids {
		if _, ok := opened[id]; ok {
			continue
		}
		repository, err := s.reader.GetRepository(ctx, id)
		if err != nil {
			opened.close()
			return nil, &Rejection{Reason: ReasonRepositoryOffline, RepositoryID: id, Cause: err}
		}
		if repository.Reachability != dbtypes.RepositoryReachabilityActive {
			opened.close()
			return nil, &Rejection{Reason: ReasonRepositoryOffline, RepositoryID: id,
				Cause: fmt.Errorf("reachability is %s", repository.Reachability)}
		}
		files, err := s.files.OpenContext(ctx, repository)
		if err == nil {
			err = files.VerifyIdentity()
			if err != nil {
				_ = files.Close()
			}
		}
		if err != nil {
			opened.close()
			return nil, &Rejection{Reason: ReasonRepositoryOffline, RepositoryID: id, Cause: err}
		}
		opened[id] = files
	}
	return opened, nil
}

// journal is one running lifecycle operation.
type journal struct {
	operationID string
	requestID   string
	kind        string
	targetID    string
	request     Request
}

func (s *Service) beginJournal(ctx context.Context, kind string, request Request, targetID uuid.UUID, payload any) (journal, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return journal{}, err
	}
	digest := sha256.Sum256(encoded)
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = uuid.NewString()
	}
	actor := strings.TrimSpace(request.Actor)
	if actor == "" {
		actor = "server"
	}
	request.Actor = actor
	target := targetID.String()
	entry := journal{operationID: uuid.NewString(), requestID: requestID, kind: kind, targetID: target, request: request}
	var actorUserID *int64
	if request.ActorUserID != nil {
		value := int64(*request.ActorUserID)
		actorUserID = &value
	}
	err = s.writer.WithTx(ctx, operationFor(kind), func(_ *sql.Tx, queries *repo.Queries) error {
		_, err := queries.CreateLifecycleOperation(ctx, repo.CreateLifecycleOperationParams{
			OperationID: entry.operationID, RequestID: requestID, Kind: kind,
			PayloadHash: hex.EncodeToString(digest[:]), Payload: dbtypes.JSON(encoded),
			Actor: actor, ActorUserID: actorUserID, TargetType: "asset", TargetID: &target,
			CreatedAt: dbtypes.NewTimestamp(s.now()),
		})
		return err
	})
	if err != nil {
		return journal{}, fmt.Errorf("journal %s: %w", kind, err)
	}
	return entry, nil
}

func operationFor(kind string) catalogtx.Operation {
	if kind == storage.LifecycleKindRestoreAssets {
		return catalogtx.OperationAssetRestore
	}
	return catalogtx.OperationAssetDelete
}

func (s *Service) advanceJournal(ctx context.Context, entry journal, phase string) error {
	return s.writer.WithTx(ctx, operationFor(entry.kind), func(_ *sql.Tx, queries *repo.Queries) error {
		return advanceJournalTx(ctx, queries, entry.operationID, phase, s.now())
	})
}

func advanceJournalTx(ctx context.Context, queries *repo.Queries, operationID, phase string, now time.Time) error {
	_, err := queries.UpdateLifecycleOperationPhase(ctx, repo.UpdateLifecycleOperationPhaseParams{
		OperationID: operationID, Phase: phase, UpdatedAt: dbtypes.NewTimestamp(now),
	})
	return err
}

// completeJournalTx closes a journal and audits the outcome in the caller's
// transaction.
func completeJournalTx(ctx context.Context, queries *repo.Queries, entry journal, auditResult string, details map[string]any, now time.Time) error {
	result, err := json.Marshal(details)
	if err != nil {
		return err
	}
	encoded := dbtypes.JSON(result)
	if _, err := queries.CompleteLifecycleOperation(ctx, repo.CompleteLifecycleOperationParams{
		OperationID: entry.operationID, Result: &encoded, UpdatedAt: dbtypes.NewTimestamp(now),
	}); err != nil {
		return err
	}
	_, err = storage.RecordLifecycleAuditTx(ctx, queries, storage.LifecycleAuditInput{
		Actor: entry.request.Actor, ActorUserID: entry.request.ActorUserID, RequestID: entry.requestID,
		OperationID: entry.operationID, Action: entry.kind, TargetType: "asset", TargetID: entry.targetID,
		ConfirmationType: "none", Result: auditResult, Details: details,
	})
	return err
}

// failJournal closes a journal whose moves were rolled back.
func (s *Service) failJournal(ctx context.Context, entry journal, stage string, cause error) {
	now := s.now()
	message := cause.Error()
	err := s.writer.WithTx(ctx, operationFor(entry.kind), func(_ *sql.Tx, queries *repo.Queries) error {
		if _, err := queries.FailLifecycleOperation(ctx, repo.FailLifecycleOperationParams{
			OperationID: entry.operationID, Phase: "failed", Status: "rolled_back", Error: &message,
			UpdatedAt: dbtypes.NewTimestamp(now),
		}); err != nil {
			return err
		}
		_, err := storage.RecordLifecycleAuditTx(ctx, queries, storage.LifecycleAuditInput{
			Actor: entry.request.Actor, ActorUserID: entry.request.ActorUserID, RequestID: entry.requestID,
			OperationID: entry.operationID, Action: entry.kind, TargetType: "asset", TargetID: entry.targetID,
			ConfirmationType: "none", Result: storage.AuditResultFailed, FailureStage: stage,
			Details: map[string]any{"error": message},
		})
		return err
	})
	if err != nil {
		s.logger.Error("close failed lifecycle journal", zap.String("operation_id", entry.operationID), zap.Error(err))
	}
}

// markEventsChangedTx invalidates the Event projection of every owner of the
// given Assets.
func markEventsChangedTx(ctx context.Context, tx *sql.Tx, assetIDs []string, reason string) error {
	encoded, err := json.Marshal(assetIDs)
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT DISTINCT owner_id FROM assets
		WHERE asset_id IN (SELECT value FROM json_each(?)) AND owner_id IS NOT NULL
	`, string(encoded))
	if err != nil {
		return err
	}
	var owners []int32
	for rows.Next() {
		var owner int32
		if err := rows.Scan(&owner); err != nil {
			rows.Close()
			return err
		}
		owners = append(owners, owner)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, owner := range owners {
		if err := event.MarkEventFactsChangedTx(ctx, tx, owner, reason); err != nil {
			return err
		}
	}
	return nil
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id != uuid.Nil && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func nullIDs(ids []uuid.UUID) []uuid.NullUUID {
	values := make([]uuid.NullUUID, 0, len(ids))
	for _, id := range ids {
		values = append(values, uuid.NullUUID{UUID: id, Valid: true})
	}
	return values
}

func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
