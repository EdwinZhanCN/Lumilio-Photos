package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/pathsemantics"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Destination volume semantics (R11). A repository's entries are keyed under
// the name comparison rule of the volume that holds the tree, measured when
// the repository was registered. A reconnect may move the registration onto
// a volume with another rule (an exFAT disk, a case-sensitive APFS volume), so
// before the catalog is cut over the destination is qualified and every live
// key is proven distinct under the new rule; after the cutover the index is
// rebuilt in bounded, revision-fenced batches while the repository stays in
// maintenance. Entry identities and Asset bindings never change: a rekey
// rewrites path_key and parent_key, nothing else.

// RepositoryVolumeRule is the name comparison rule a repository row's
// entries are keyed under.
func RepositoryVolumeRule(repository repo.Repository) pathsemantics.Semantics {
	return pathsemantics.Semantics{Case: repository.PathCase, Normalization: repository.PathNormalization}
}

const (
	// rekeyBatchRows bounds one rekey writer transaction to the scan index's
	// structural writer budget (architecturecheck: 256 rows per transaction).
	rekeyBatchRows = 256
	// rekeyPageRows bounds one preflight read.
	rekeyPageRows = 1024
	// maxPathKeyCollisionExamples bounds the pairs a refusal names.
	maxPathKeyCollisionExamples = 10

	entryStatePendingHash = "pending_hash"
	entryStatePresent     = "present"
)

// PathKeyCollision is one pair of indexed paths the destination volume reads
// as a single name.
type PathKeyCollision struct {
	First  string `json:"first"`
	Second string `json:"second"`
}

// PathKeyCollisionError refuses a reconnect whose destination volume cannot
// tell apart paths the index holds as distinct live entries. Nothing is
// changed by the refusal: no row is dropped and no byte is overwritten.
type PathKeyCollisionError struct {
	RepositoryID string
	Destination  pathsemantics.Semantics
	// Count is the number of live entries that would share another entry's
	// key; Examples names at most maxPathKeyCollisionExamples of the pairs.
	Count    int
	Examples []PathKeyCollision
}

func (e *PathKeyCollisionError) Error() string {
	examples := make([]string, 0, len(e.Examples))
	for _, pair := range e.Examples {
		examples = append(examples, fmt.Sprintf("%q and %q", pair.First, pair.Second))
	}
	return fmt.Sprintf("destination volume (%s) reads %d indexed paths of repository %s as names it already holds: %s",
		e.Destination, e.Count, e.RepositoryID, strings.Join(examples, ", "))
}

// volumeRekey is the rule change a reconnect applies to one repository's
// index: nothing when the destination follows the registered rule.
type volumeRekey struct {
	From, To pathsemantics.Semantics
}

func (r volumeRekey) needed() bool { return !r.From.Equal(r.To) }

// planVolumeRekey qualifies the destination tree and compares its rule with
// the one the repository is keyed under. An unqualified or mixed destination
// refuses the reconnect (pathsemantics.ErrUnqualified).
func (rm *DefaultRepositoryManager) planVolumeRekey(repository repo.Repository, destination string) (volumeRekey, error) {
	to, err := rm.qualifyRepositoryVolume(destination)
	if err != nil {
		return volumeRekey{}, fmt.Errorf("qualify destination volume: %w", err)
	}
	return volumeRekey{From: RepositoryVolumeRule(repository), To: to}, nil
}

// preflightRekey proves that every live entry keeps a key of its own under
// the destination rule. It reads the whole index in bounded pages and writes
// nothing; a collision is the typed refusal with bounded examples.
func (rm *DefaultRepositoryManager) preflightRekey(ctx context.Context, repositoryID uuid.UUID, plan volumeRekey) error {
	if !plan.needed() {
		return nil
	}
	live := make(map[string]string)
	collision := &PathKeyCollisionError{RepositoryID: repositoryID.String(), Destination: plan.To}
	after := uuid.Nil
	for {
		page, err := rm.queries.ListRepositoryEntriesForRekey(ctx, repo.ListRepositoryEntriesForRekeyParams{
			RepositoryID: repositoryID, AfterEntryID: after, PageSize: rekeyPageRows,
		})
		if err != nil {
			return fmt.Errorf("read repository index for rekey preflight: %w", err)
		}
		for _, row := range page {
			if row.State != entryStatePendingHash && row.State != entryStatePresent {
				continue
			}
			key, err := plan.To.PathKey(row.Path)
			if err != nil {
				return fmt.Errorf("key %q under the destination rule: %w", row.Path, err)
			}
			if other, taken := live[key]; taken {
				collision.Count++
				if len(collision.Examples) < maxPathKeyCollisionExamples {
					collision.Examples = append(collision.Examples, PathKeyCollision{First: other, Second: row.Path})
				}
				continue
			}
			live[key] = row.Path
		}
		if len(page) < rekeyPageRows {
			break
		}
		after = page[len(page)-1].EntryID
	}
	if collision.Count > 0 {
		return collision
	}
	return nil
}

// rekeyRepositoryEntries rewrites the keys of every entry that disagrees with
// the rule, in writer transactions of at most rekeyBatchRows rows. Each
// batch reads its page inside the transaction and fences every update on
// the row's revision; the pass is idempotent, so an interrupted rekey is
// finished by running it again. It returns how many rows changed.
func (rm *DefaultRepositoryManager) rekeyRepositoryEntries(ctx context.Context, repositoryID uuid.UUID, to pathsemantics.Semantics) (int, error) {
	rekeyed := 0
	after := uuid.Nil
	for {
		if err := ctx.Err(); err != nil {
			return rekeyed, err
		}
		tx, err := rm.writer.BeginTx(ctx, catalogtx.OperationRepositoryRekey, nil)
		if err != nil {
			return rekeyed, fmt.Errorf("begin repository rekey batch: %w", err)
		}
		queries := rm.queries.WithTx(tx.Raw())
		page, err := queries.ListRepositoryEntriesForRekey(ctx, repo.ListRepositoryEntriesForRekeyParams{
			RepositoryID: repositoryID, AfterEntryID: after, PageSize: rekeyBatchRows,
		})
		if err != nil {
			_ = tx.Rollback()
			return rekeyed, fmt.Errorf("read repository index for rekey: %w", err)
		}
		now := dbtypes.NewTimestamp(time.Now().UTC())
		changed := 0
		for _, row := range page {
			key, err := to.PathKey(row.Path)
			if err != nil {
				_ = tx.Rollback()
				return rekeyed, fmt.Errorf("key %q under the repository rule: %w", row.Path, err)
			}
			parentKey, err := to.PathKey(parentPathOf(row.Path))
			if err != nil {
				_ = tx.Rollback()
				return rekeyed, fmt.Errorf("key %q under the repository rule: %w", parentPathOf(row.Path), err)
			}
			if key == row.PathKey && parentKey == row.ParentKey {
				continue
			}
			affected, err := queries.RekeyRepositoryEntryCAS(ctx, repo.RekeyRepositoryEntryCASParams{
				PathKey: key, ParentKey: parentKey, UpdatedAt: now, EntryID: row.EntryID, ExpectedRevision: row.Revision,
			})
			if err != nil {
				_ = tx.Rollback()
				return rekeyed, fmt.Errorf("rekey entry %s: %w", row.Path, err)
			}
			if affected != 1 {
				_ = tx.Rollback()
				return rekeyed, fmt.Errorf("%w: entry %s changed while its keys were being rebuilt", ErrRepositoryBusy, row.Path)
			}
			changed++
		}
		if err := tx.Commit(); err != nil {
			return rekeyed, fmt.Errorf("commit repository rekey batch: %w", err)
		}
		rekeyed += changed
		if len(page) < rekeyBatchRows {
			return rekeyed, nil
		}
		after = page[len(page)-1].EntryID
	}
}

// finishVolumeRekey brings every entry of a repository into agreement with
// the rule stored on its row and then releases the maintenance barrier the
// cutover left in place. The reconnect command runs it after its cutover
// commits; startup recovery runs it for a reconnect that was interrupted in
// between. Both are the same idempotent pass.
func (rm *DefaultRepositoryManager) finishVolumeRekey(ctx context.Context, repositoryID uuid.UUID) error {
	repository, err := rm.queries.GetRepository(ctx, repositoryID)
	if err != nil {
		return fmt.Errorf("read repository for rekey: %w", err)
	}
	rule := RepositoryVolumeRule(repository)
	rekeyed, err := rm.rekeyRepositoryEntries(ctx, repositoryID, rule)
	if err != nil {
		return err
	}
	if repository.Reachability == dbtypes.RepositoryReachabilityMaintenance {
		if _, err := rm.queries.EndRepositoryMaintenance(ctx, repo.EndRepositoryMaintenanceParams{
			RepoID: repositoryID, Reachability: dbtypes.RepositoryReachabilityActive,
			Activity: dbtypes.RepositoryActivityIdle, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		}); err != nil {
			return fmt.Errorf("leave rekey maintenance: %w", err)
		}
	}
	rm.logger.Info("repository index rekeyed for its volume",
		zap.String("operation", "repository.rekey"),
		zap.String("repository_id", repositoryID.String()),
		zap.String("rule", rule.String()),
		zap.Int("entries_rekeyed", rekeyed))
	return nil
}

// finishStorageLocationRekeys finishes the rekey of every child a Storage
// Location reconnect left in maintenance.
func (rm *DefaultRepositoryManager) finishStorageLocationRekeys(ctx context.Context, storageLocationID uuid.UUID) error {
	repositories, err := rm.queries.ListRepositories(ctx)
	if err != nil {
		return fmt.Errorf("list Storage Location children for rekey: %w", err)
	}
	for _, repository := range repositories {
		if repository.StorageLocationID != storageLocationID || repository.Reachability != dbtypes.RepositoryReachabilityMaintenance {
			continue
		}
		if err := rm.finishVolumeRekey(ctx, repository.RepoID); err != nil {
			return fmt.Errorf("finish rekey of %s: %w", repository.Name, err)
		}
	}
	return nil
}

// parentPathOf is the '/'-separated parent of a repository-relative path,
// "" for a root-level name.
func parentPathOf(relative string) string {
	if index := strings.LastIndexByte(relative, '/'); index >= 0 {
		return relative[:index]
	}
	return ""
}
