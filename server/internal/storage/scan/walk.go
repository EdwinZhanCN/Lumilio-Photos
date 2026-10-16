package scan

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
)

// TurnResult reports one bounded walk turn.
type TurnResult struct {
	ScanID uuid.UUID
	Status string
	// HasMore is true while the scan is queued or running; a caller keeps
	// turning (or snoozes until NotBefore) until it is false.
	HasMore   bool
	NotBefore time.Time
}

// counters accumulate one directory's (or one turn's) scan progress.
type counters struct {
	seen, newEntries, changed, missing, restored, moved, deferred, errors int64
}

func (c *counters) add(other counters) {
	c.seen += other.seen
	c.newEntries += other.newEntries
	c.changed += other.changed
	c.missing += other.missing
	c.restored += other.restored
	c.moved += other.moved
	c.deferred += other.deferred
	c.errors += other.errors
}

func (c counters) empty() bool { return c == counters{} }

// testHookBeforeRemovalCommit runs after an absence is proven and before the
// marker re-check that guards its commit. Tests use it to change the disk in
// that window.
var testHookBeforeRemovalCommit func()

// errOffline stops a turn when the repository marker check fails; the scan
// ends offline and commits no further entry change.
var errOffline = errors.New("repository is offline")

func isOffline(err error) bool {
	return errors.Is(err, errOffline) ||
		errors.Is(err, storage.ErrRepositoryOffline) ||
		errors.Is(err, storage.ErrRepositoryIDMismatch) ||
		errors.Is(err, storage.ErrRepositoryMarkerInvalid)
}

// RunTurn advances the repository's scan by one bounded turn: it starts the
// queued scan when none is running and it is due, walks directories until
// the turn budget is spent, and finishes the scan when the walk is done.
func (s *Scanner) RunTurn(ctx context.Context, repositoryID uuid.UUID) (TurnResult, error) {
	scan, err := s.reader.GetRunningRepositoryScan(ctx, repositoryID)
	if errors.Is(err, sql.ErrNoRows) {
		queued, queuedErr := s.reader.GetQueuedRepositoryScan(ctx, repositoryID)
		if errors.Is(queuedErr, sql.ErrNoRows) {
			return TurnResult{}, nil
		}
		if queuedErr != nil {
			return TurnResult{}, queuedErr
		}
		if queued.NotBefore.Valid && queued.NotBefore.Time.After(s.config.Now()) {
			return TurnResult{ScanID: queued.ScanID, Status: StatusQueued, HasMore: true, NotBefore: queued.NotBefore.Time}, nil
		}
		if err := s.writer.WithTx(ctx, catalogtx.OperationRepositoryScanStart, func(_ *sql.Tx, queries *repo.Queries) error {
			_, err := queries.StartRepositoryScan(ctx, repo.StartRepositoryScanParams{
				ScanID: queued.ScanID, StartedAt: dbtypes.NewTimestamp(s.config.Now()),
			})
			return err
		}); err != nil {
			return TurnResult{}, err
		}
		scan, err = s.reader.GetRepositoryScan(ctx, queued.ScanID)
	}
	if err != nil {
		return TurnResult{}, err
	}
	result := TurnResult{ScanID: scan.ScanID, Status: scan.Status, HasMore: true}
	if scan.CancellationRequested != 0 {
		return s.finish(ctx, scan, StatusCancelled, nil, counters{}, nil)
	}
	repository, err := s.reader.GetRepository(ctx, repositoryID)
	if err != nil {
		return result, err
	}
	fsys, err := s.files.OpenContext(ctx, repository)
	if err != nil {
		if isOffline(err) {
			code := "repository_offline"
			return s.finish(ctx, scan, StatusOffline, &code, counters{}, nil)
		}
		return result, err
	}
	defer fsys.Close()

	turn := &walkTurn{scanner: s, scan: scan, fsys: fsys, started: s.config.Now()}
	scope, err := s.resolveScope(ctx, repositoryID, scan.ScopePath)
	if err != nil {
		return result, err
	}
	turn.scope = scope
	if err := turn.rebuild(ctx); err != nil {
		return result, err
	}
	for {
		directory, ok := turn.next()
		if !ok {
			return s.finish(ctx, scan, StatusCompleted, nil, turn.pending, turn.settleFollowUp())
		}
		if err := turn.processDirectory(ctx, directory); err != nil {
			if isOffline(err) {
				code := "repository_offline"
				return s.finish(ctx, scan, StatusOffline, &code, turn.pending, nil)
			}
			return result, err
		}
		if turn.files >= s.config.TurnFiles || s.config.Now().Sub(turn.started) >= s.config.TurnDuration {
			if err := turn.flush(ctx); err != nil {
				return result, err
			}
			return result, nil
		}
	}
}

// resolveScope narrows a subtree scope to a directory the catalog already
// knows, so every walked directory has catalog ancestors; the empty scope always works.
func (s *Scanner) resolveScope(ctx context.Context, repositoryID uuid.UUID, scope string) (string, error) {
	for scope != "" {
		key, err := s.pathKey(scope)
		if err != nil {
			return "", err
		}
		row, err := s.reader.GetLiveRepositoryEntryByKey(ctx, repo.GetLiveRepositoryEntryByKeyParams{RepositoryID: repositoryID, PathKey: key})
		if err == nil && row.Kind == KindDirectory {
			return row.Path, nil
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		scope = parentOf(scope)
	}
	return "", nil
}

type settleFollowUp struct {
	scope     string
	notBefore time.Time
}

func (s *Scanner) finish(ctx context.Context, scan repo.RepositoryScan, status string, code *string, pending counters, followUp *settleFollowUp) (TurnResult, error) {
	now := s.config.Now()
	err := s.writer.WithTx(ctx, catalogtx.OperationRepositoryScanFinish, func(_ *sql.Tx, queries *repo.Queries) error {
		if !pending.empty() {
			if err := advanceTx(ctx, queries, scan.ScanID, StatusWalking, nil, pending, now); err != nil {
				return err
			}
		}
		if _, err := queries.FinishRepositoryScan(ctx, repo.FinishRepositoryScanParams{
			Status: status, ErrorCode: code, FinishedAt: dbtypes.NewTimestamp(now), ScanID: scan.ScanID,
		}); err != nil {
			return err
		}
		if followUp != nil {
			_, _, err := s.requestTx(ctx, queries, scan.RepositoryID, TriggerSettle, followUp.scope, "", followUp.notBefore)
			return err
		}
		return nil
	})
	return TurnResult{ScanID: scan.ScanID, Status: status}, err
}

func advanceTx(ctx context.Context, queries *repo.Queries, scanID uuid.UUID, status string, cursor *string, c counters, now time.Time) error {
	_, err := queries.AdvanceRepositoryScan(ctx, repo.AdvanceRepositoryScanParams{
		Status: status, ResumeAfterPath: cursor,
		Seen: c.seen, NewEntries: c.newEntries, Changed: c.changed, Missing: c.missing,
		Restored: c.restored, Moved: c.moved, Deferred: c.deferred, Errors: c.errors,
		UpdatedAt: dbtypes.NewTimestamp(now), ScanID: scanID,
	})
	return err
}

// frame is one directory whose subdirectories are still to be walked.
type frame struct {
	directory string
	children  []string
	index     int
}

type walkTurn struct {
	scanner *Scanner
	scan    repo.RepositoryScan
	fsys    *storage.RepositoryFS
	scope   string
	started time.Time
	files   int
	// The walk is depth-first pre-order: a directory's diff commits before
	// any of its subdirectories.
	stack        []frame
	rootPending  bool
	cursor       *string
	pending      counters
	deferredDirs []string
	latestSettle time.Time
}

func join(directory, name string) string {
	if directory == "" {
		return name
	}
	return directory + "/" + name
}

func within(scope, relative string) bool {
	return scope == "" || relative == scope || strings.HasPrefix(relative, scope+"/")
}

// rebuild restores the walk position after resume_after_path from the disk:
// each ancestor's remaining subdirectories, then the cursor's own children.
// Directories that changed behind the cursor are left to the next scan.
func (t *walkTurn) rebuild(ctx context.Context) error {
	cursor := t.scan.ResumeAfterPath
	if cursor == nil || !within(t.scope, *cursor) {
		t.rootPending = true
		return nil
	}
	t.cursor = cursor
	chain := []string{*cursor}
	for current := *cursor; current != t.scope; {
		current = parentOf(current)
		chain = append(chain, current)
	}
	for index := len(chain) - 1; index >= 0; index-- {
		directory := chain[index]
		children, err := t.fsys.ListUserMediaSubdirectories(ctx, directory)
		if err != nil {
			children = nil
		}
		position := 0
		if index > 0 {
			next := chain[index-1]
			name := next[strings.LastIndexByte(next, '/')+1:]
			position = sort.SearchStrings(children, name)
			if position < len(children) && children[position] == name {
				position++
			}
		}
		t.stack = append(t.stack, frame{directory: directory, children: children, index: position})
	}
	return nil
}

func (t *walkTurn) next() (string, bool) {
	if t.rootPending {
		t.rootPending = false
		return t.scope, true
	}
	for len(t.stack) > 0 {
		top := &t.stack[len(t.stack)-1]
		if top.index < len(top.children) {
			child := join(top.directory, top.children[top.index])
			top.index++
			return child, true
		}
		t.stack = t.stack[:len(t.stack)-1]
	}
	return "", false
}

func (t *walkTurn) settleFollowUp() *settleFollowUp {
	if len(t.deferredDirs) == 0 {
		return nil
	}
	scope := t.deferredDirs[0]
	for _, directory := range t.deferredDirs[1:] {
		scope = commonAncestor(scope, directory)
	}
	return &settleFollowUp{scope: scope, notBefore: t.latestSettle}
}

// write is one planned entry change, computed from a snapshot outside any
// transaction and applied by compare-and-swap.
type write struct {
	insert   *repo.InsertRepositoryEntryParams
	observe  *repo.UpdateRepositoryEntryObservedCASParams
	vanished *repo.RepositoryEntry
}

// sameTuple compares a row's stat tuple with a fresh observation. A change
// time or file ID the catalog does not know (after a relocation) is not
// compared; unrecorded reports that the row should record the fresh ones.
func sameTuple(row repo.RepositoryEntry, observation storage.FileObservation) bool {
	return row.Size == observation.Size && row.MtimeNs == observation.ModTimeNS &&
		(row.CtimeNs == nil || equalInt64(row.CtimeNs, observation.ChangeTimeNS)) &&
		(row.FileID == nil || equalString(row.FileID, observation.FileIdentity))
}

func unrecorded(row repo.RepositoryEntry, observation storage.FileObservation) bool {
	return (row.CtimeNs == nil && observation.ChangeTimeNS != nil) || (row.FileID == nil && observation.FileIdentity != nil)
}

func equalInt64(left, right *int64) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

func equalString(left, right *string) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && *left == *right)
}

// racilyClean is Git's rule: a tuple checked within the timestamp
// granularity of the file's mtime cannot prove the content unchanged.
func racilyClean(row repo.RepositoryEntry) bool {
	return row.MtimeNs >= row.StatCheckedNs-RacyGranularity.Nanoseconds()
}

func entryKind(observation storage.FileObservation) string {
	if observation.EntryKind == storage.EntryKindDirectory {
		return KindDirectory
	}
	return KindFile
}

// processDirectory diffs one directory against its catalog rows and commits
// the difference.
func (t *walkTurn) processDirectory(ctx context.Context, directory string) error {
	s := t.scanner
	now := s.config.Now()
	// Only a directory whose own row is live is walked, so every indexed
	// entry has an indexed parent. A directory that appeared behind the
	// parent's diff (for example renamed mid-scan) waits for the next scan.
	if directory != "" {
		known, err := t.knownDirectory(ctx, directory)
		if err != nil {
			return err
		}
		if !known {
			return t.markCursor(directory)
		}
	}
	listing, err := t.fsys.ListUserMediaDirectory(ctx, directory)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// An unreadable directory proves nothing about its children. Only a
		// directory that is positively gone takes its subtree with it.
		if directory != "" {
			if gone, probeErr := t.vanishedDirectory(ctx, directory); probeErr != nil {
				return probeErr
			} else if gone {
				return t.markCursor(directory)
			}
		}
		t.pending.errors++
		return t.markCursor(directory)
	}
	directoryKey, err := s.pathKey(directory)
	if err != nil {
		return err
	}
	rows, err := s.reader.ListRepositoryEntryChildren(ctx, repo.ListRepositoryEntryChildrenParams{
		RepositoryID: t.scan.RepositoryID, ParentKey: directoryKey,
	})
	if err != nil {
		return err
	}
	live := make(map[string]repo.RepositoryEntry, len(rows))
	missing := make(map[string]repo.RepositoryEntry)
	for _, row := range rows {
		if row.State == StateMissing {
			if current, ok := missing[row.PathKey]; !ok || row.UpdatedAt.Time.After(current.UpdatedAt.Time) {
				missing[row.PathKey] = row
			}
			continue
		}
		live[row.PathKey] = row
	}

	var c counters
	var writes []write
	onDisk := make(map[string]bool)
	issues := make(map[string]storage.WalkIssue, len(listing.Issues))
	for _, issue := range listing.Issues {
		if key, keyErr := s.pathKey(issue.Path); keyErr == nil {
			issues[key] = issue
		}
		// Unsupported entries and nested repositories are expected; anything
		// else is a child that exists but could not be read.
		if issue.Reason != "unsupported_entry" && issue.Reason != "nested_repository" {
			c.errors++
		}
	}
	observations := append(append([]storage.FileObservation(nil), listing.Directories...), listing.Files...)
	t.files += len(observations)
	subdirectories := make([]string, 0, len(listing.Directories))
	for _, observation := range observations {
		relative := observation.Path.String()
		key, keyErr := s.pathKey(relative)
		if keyErr != nil || onDisk[key] {
			// Two names that compare equal on this volume cannot both be
			// indexed; the second is reported, not guessed at.
			c.errors++
			continue
		}
		onDisk[key] = true
		c.seen++
		kind := entryKind(observation)
		if kind == KindDirectory {
			subdirectories = append(subdirectories, relative[strings.LastIndexByte(relative, '/')+1:])
		}
		settling := kind == KindFile && s.config.Settle > 0 && now.Sub(time.Unix(0, observation.ModTimeNS)) < s.config.Settle
		observed := func(row repo.RepositoryEntry, state string) *repo.UpdateRepositoryEntryObservedCASParams {
			return &repo.UpdateRepositoryEntryObservedCASParams{
				Path: relative, Size: observation.Size, MtimeNs: observation.ModTimeNS,
				CtimeNs: observation.ChangeTimeNS, FileID: observation.FileIdentity,
				StatCheckedNs: now.UnixNano(), State: state, UpdatedAt: dbtypes.NewTimestamp(now),
				EntryID: row.EntryID, ExpectedRevision: row.Revision,
			}
		}
		if row, ok := live[key]; ok {
			if row.Kind != kind {
				rowCopy := row
				writes = append(writes, write{vanished: &rowCopy})
				row = repo.RepositoryEntry{}
			} else {
				unchanged := sameTuple(row, observation) && (kind == KindDirectory || !racilyClean(row))
				switch {
				case unchanged && row.Path == relative && !unrecorded(row, observation):
				case unchanged:
					// A case-only rename, or a tuple to re-record after a
					// relocation, keeps the row, its state, and its binding.
					writes = append(writes, write{observe: observed(row, row.State)})
				case settling:
					c.deferred++
					t.deferFile(directory, observation)
				case kind == KindDirectory:
					writes = append(writes, write{observe: observed(row, StatePresent)})
				default:
					c.changed++
					writes = append(writes, write{observe: observed(row, StatePendingHash)})
				}
				continue
			}
		}
		if settling {
			c.deferred++
			t.deferFile(directory, observation)
			continue
		}
		if row, ok := missing[key]; ok && row.Kind == kind {
			// The file came back. Only an unchanged tuple over content that is
			// still the Asset's restores without a hash.
			state := StatePendingHash
			if kind == KindDirectory || (sameTuple(row, observation) && !racilyClean(row) && row.AssetID.Valid) {
				state = StatePresent
			}
			c.restored++
			writes = append(writes, write{observe: observed(row, state)})
			continue
		}
		state := StatePendingHash
		if kind == KindDirectory {
			state = StatePresent
		}
		c.newEntries++
		writes = append(writes, write{insert: &repo.InsertRepositoryEntryParams{
			EntryID: uuid.New(), RepositoryID: t.scan.RepositoryID, Path: relative, PathKey: key,
			ParentKey: directoryKey, Kind: kind, Size: observation.Size, MtimeNs: observation.ModTimeNS,
			CtimeNs: observation.ChangeTimeNS, FileID: observation.FileIdentity, StatCheckedNs: now.UnixNano(),
			State: state, UpdatedAt: dbtypes.NewTimestamp(now),
		}})
	}

	// Catalog rows with no disk entry are gone only on positive evidence.
	for key, row := range live {
		if onDisk[key] {
			continue
		}
		if _, ok := issues[key]; ok {
			continue
		}
		kind := storage.EntryKindRegular
		if row.Kind == KindDirectory {
			kind = storage.EntryKindDirectory
		}
		repositoryPath, parseErr := storage.ParseUserMediaPath(row.Path)
		if parseErr != nil {
			c.errors++
			continue
		}
		gone, probeErr := t.fsys.ProbeUserMediaAbsence(ctx, repositoryPath, kind)
		if probeErr != nil {
			if isOffline(probeErr) {
				return errOffline
			}
			c.errors++
			continue
		}
		if gone {
			rowCopy := row
			writes = append(writes, write{vanished: &rowCopy})
		}
	}

	t.pending.add(c)
	if err := t.apply(ctx, directory, writes); err != nil {
		return err
	}
	sort.Strings(subdirectories)
	t.stack = append(t.stack, frame{directory: directory, children: subdirectories})
	return nil
}

func (t *walkTurn) knownDirectory(ctx context.Context, directory string) (bool, error) {
	key, err := t.scanner.pathKey(directory)
	if err != nil {
		return false, err
	}
	row, err := t.scanner.reader.GetLiveRepositoryEntryByKey(ctx, repo.GetLiveRepositoryEntryByKeyParams{
		RepositoryID: t.scan.RepositoryID, PathKey: key,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && row.Kind == KindDirectory, err
}

func (t *walkTurn) deferFile(directory string, observation storage.FileObservation) {
	if len(t.deferredDirs) == 0 || t.deferredDirs[len(t.deferredDirs)-1] != directory {
		t.deferredDirs = append(t.deferredDirs, directory)
	}
	due := time.Unix(0, observation.ModTimeNS).Add(t.scanner.config.Settle)
	if due.After(t.latestSettle) {
		t.latestSettle = due
	}
}

func (t *walkTurn) markCursor(directory string) error {
	cursor := directory
	t.cursor = &cursor
	return nil
}

// vanishedDirectory handles a directory that could not be listed: when it is
// positively gone, its subtree is swept.
func (t *walkTurn) vanishedDirectory(ctx context.Context, directory string) (bool, error) {
	repositoryPath, err := storage.ParseUserMediaPath(directory)
	if err != nil {
		return false, nil
	}
	gone, err := t.fsys.ProbeUserMediaAbsence(ctx, repositoryPath, storage.EntryKindDirectory)
	if err != nil {
		if isOffline(err) {
			return false, errOffline
		}
		return false, nil
	}
	if !gone {
		return false, nil
	}
	key, err := t.scanner.pathKey(directory)
	if err != nil {
		return false, err
	}
	row, err := t.scanner.reader.GetLiveRepositoryEntryByKey(ctx, repo.GetLiveRepositoryEntryByKeyParams{
		RepositoryID: t.scan.RepositoryID, PathKey: key,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return true, t.sweep(ctx, row)
}

// apply commits a directory's writes in batches of at most MaxBatchRows,
// sweeping vanished directories first so a new entry never collides with
// the live row it replaces. The directory's cursor and counters ride in the
// last batch; a directory with no change only moves the in-memory cursor.
func (t *walkTurn) apply(ctx context.Context, directory string, writes []write) error {
	rest := make([]write, 0, len(writes))
	for _, w := range writes {
		if w.vanished != nil && w.vanished.Kind == KindDirectory {
			if err := t.sweep(ctx, *w.vanished); err != nil {
				return err
			}
			continue
		}
		rest = append(rest, w)
	}
	sort.SliceStable(rest, func(i, j int) bool { return rest[i].vanished != nil && rest[j].vanished == nil })
	if len(rest) == 0 {
		return t.markCursor(directory)
	}
	now := t.scanner.config.Now()
	for start := 0; start < len(rest); {
		batch := rest[start:min(start+t.scanner.batch.size(), len(rest))]
		removes := false
		for _, w := range batch {
			removes = removes || w.vanished != nil
		}
		// Invariant 3: nothing is marked missing unless the repository is
		// online, re-checked right before the commit.
		if removes {
			if testHookBeforeRemovalCommit != nil {
				testHookBeforeRemovalCommit()
			}
			if err := t.fsys.VerifyIdentity(); err != nil {
				return errOffline
			}
		}
		last := start+len(batch) == len(rest)
		started := time.Now()
		var progress counters
		err := t.scanner.writer.WithTx(ctx, catalogtx.OperationRepositoryScanApplyDirectory, func(_ *sql.Tx, queries *repo.Queries) error {
			progress = counters{}
			for _, w := range batch {
				switch {
				case w.insert != nil:
					if _, err := queries.InsertRepositoryEntry(ctx, *w.insert); err != nil {
						return err
					}
				case w.observe != nil:
					if _, err := queries.UpdateRepositoryEntryObservedCAS(ctx, *w.observe); err != nil {
						return err
					}
				case w.vanished != nil:
					outcome, err := removeEntryTx(ctx, queries, *w.vanished, now)
					if err != nil {
						return err
					}
					progress.add(outcome)
				}
			}
			if !last {
				return advanceTx(ctx, queries, t.scan.ScanID, StatusWalking, t.cursor, progress, now)
			}
			total := t.pending
			total.add(progress)
			cursor := directory
			return advanceTx(ctx, queries, t.scan.ScanID, StatusWalking, &cursor, total, now)
		})
		if err != nil {
			return err
		}
		t.scanner.batch.observe(len(batch), time.Since(started))
		start += len(batch)
		if last {
			t.pending = counters{}
		}
	}
	return t.markCursor(directory)
}

// flush commits the turn's accumulated counters and cursor when no directory
// write carried them.
func (t *walkTurn) flush(ctx context.Context) error {
	if t.pending.empty() && t.cursor == t.scan.ResumeAfterPath {
		return nil
	}
	now := t.scanner.config.Now()
	err := t.scanner.writer.WithTx(ctx, catalogtx.OperationRepositoryScanApplyDirectory, func(_ *sql.Tx, queries *repo.Queries) error {
		return advanceTx(ctx, queries, t.scan.ScanID, StatusWalking, t.cursor, t.pending, now)
	})
	if err == nil {
		t.pending = counters{}
	}
	return err
}

// sweep marks every live row at or below a vanished directory, in
// path-ordered pages with the marker re-checked before each page.
func (t *walkTurn) sweep(ctx context.Context, directory repo.RepositoryEntry) error {
	s := t.scanner
	after := ""
	for {
		page, err := s.reader.ListLiveRepositoryEntriesUnder(ctx, repo.ListLiveRepositoryEntriesUnderParams{
			RepositoryID: t.scan.RepositoryID, Directory: directory.Path, AfterPath: after, PageLimit: int64(s.batch.size()),
		})
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		if testHookBeforeRemovalCommit != nil {
			testHookBeforeRemovalCommit()
		}
		if err := t.fsys.VerifyIdentity(); err != nil {
			return errOffline
		}
		now := s.config.Now()
		started := time.Now()
		err = s.writer.WithTx(ctx, catalogtx.OperationRepositoryScanMarkMissing, func(_ *sql.Tx, queries *repo.Queries) error {
			var progress counters
			for _, row := range page {
				outcome, err := removeEntryTx(ctx, queries, row, now)
				if err != nil {
					return err
				}
				progress.add(outcome)
			}
			return advanceTx(ctx, queries, t.scan.ScanID, StatusSweeping, t.cursor, progress, now)
		})
		if err != nil {
			return err
		}
		s.batch.observe(len(page), time.Since(started))
		after = page[len(page)-1].Path
	}
}

// removeEntryTx applies the outcome for a vanished entry: a directory or an
// entry that never held an Asset is deleted; an entry whose Asset is present
// elsewhere was moved or was a copy and is deleted; otherwise the entry
// becomes missing so the Asset keeps its metadata. A scan never deletes an
// Asset's last entry.
func removeEntryTx(ctx context.Context, queries *repo.Queries, row repo.RepositoryEntry, now time.Time) (counters, error) {
	var outcome counters
	if row.Kind == KindDirectory || !row.AssetID.Valid {
		_, err := queries.DeleteRepositoryEntryCAS(ctx, repo.DeleteRepositoryEntryCASParams{
			EntryID: row.EntryID, ExpectedRevision: row.Revision,
		})
		return outcome, err
	}
	elsewhere, err := queries.HasOtherPresentRepositoryEntry(ctx, repo.HasOtherPresentRepositoryEntryParams{
		AssetID: row.AssetID, EntryID: row.EntryID,
	})
	if err != nil {
		return outcome, err
	}
	if elsewhere == 1 {
		rows, err := queries.DeleteRepositoryEntryCAS(ctx, repo.DeleteRepositoryEntryCASParams{
			EntryID: row.EntryID, ExpectedRevision: row.Revision,
		})
		outcome.moved = rows
		return outcome, err
	}
	rows, err := queries.MarkRepositoryEntryMissingCAS(ctx, repo.MarkRepositoryEntryMissingCASParams{
		MissingSince: dbtypes.NewTimestamp(now), UpdatedAt: dbtypes.NewTimestamp(now),
		EntryID: row.EntryID, ExpectedRevision: row.Revision,
	})
	outcome.missing = rows
	return outcome, err
}
