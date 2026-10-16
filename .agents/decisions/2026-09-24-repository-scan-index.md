# Decision: Index repositories with a Syncthing-style scan, not an observation engine

Status: accepted, 2026-09-24 (owner, issue #222). Implementation is in
progress under
[the scan index and Asset lifecycle plan](../../docs/exec-plans/active/repository-index-and-asset-lifecycle.md);
this line becomes `implemented` when that plan completes. Supersedes
[the Repository Observation Engine decision](2026-08-22-repository-observation-engine.md).
Its lifecycle half is
[the Asset lifecycle decision](2026-09-24-asset-lifecycle.md).

## Problem

The Repository Observation Engine (ROE) proves absence through change-journal
cursors: a run captures `C0`, crawls, drains to a fixed `C1`, re-verifies dirty
directories, and only then closes Locations. That protocol costs more than it
returns:

- **Quadratic scans that hold the only catalog writer.**
  `CountPendingRepositoryMaterialization` counts every active file with two
  correlated subqueries, inside the writer transaction, on every directory
  page, every hashed file, and every frontier failure. Measured on
  2026-09-24 with the generated crawl profile: 10k files in 8.8 s, 20k in
  33 s, 40k in 131 s; writer transactions held 130–160 ms against a 25 ms
  budget. Through the API on 2026-09-27, a real 10k-file tree was still
  crawling after 60 s with 942 files observed.
- **Absence that depends on the environment.** On 2026-09-27 (`dev` @
  `a3fb469d`), a file was deleted from a repository and a forced full
  verification completed with `files_observed: 0`. On the Linux, macOS, and
  Windows CI runners the deletion was then recognised. On a developer Mac,
  with the repository on an external APFS volume, the Storage page still
  counted the Asset as active seven minutes later, and no further
  verification ran. The cause was not isolated. The design fails closed, so
  whenever a run cannot prove absence the catalog stays wrong until some
  later run can.
- **Complexity.** About 6.2k lines of production Go and 3.8k of tests, 68
  queries over 7 tables, three native journal adapters (about 1.7k lines),
  and a reference reducer that production code does not use. The superseded
  record already described a `repository_outbox` that no longer existed.

The rc.1 catalog baseline is still editable (see
[the compatibility baseline decision](2026-09-24-rc-compatibility-baseline.md)),
so this is the last point at which the model can be replaced without a
migration.

## Decision

The disk is the truth and a full scan is the authority. The catalog mirrors
the tree through one row per physical entry, the way Syncthing's index and
Git's index do, and nothing needs a journal to be correct.

**Data.** `repository_entries` replaces `repository_nodes`,
`repository_observations`, and `asset_locations`: repository, relative path,
`path_key` (case and NFC per volume, from `pathsemantics`), `parent_key` (the
parent's `path_key`, so a case-only directory rename keeps its children),
kind, a stat tuple (size, mtime, ctime, `file_id`, checked-at), a `revision`
that every write compares and swaps, a state
(`pending_hash`, `present`, `missing`, `unsupported`, `trashed`), and the bound
content and Asset. Path uniqueness is a partial unique index over
`pending_hash` and `present`, so a missing or trashed row never blocks a live
file at the same path. `repository_scans` replaces the run, state, frontier,
and cursor tables with one row per scan: trigger, scope, status
(`queued`, `walking`, `sweeping`, then `completed`, `offline`, `failed`, or
`cancelled`), a resume path, and incremental counters. `content_objects` and
the one-Asset-per-owner-and-content rule are unchanged.

**Scan.** Amended 2026-09-27 (owner, #222): the sweep is folded into the
walk as a per-directory diff, Syncthing-style, instead of a separate pass
over rows the walk did not mark as seen. A rescan with no change therefore
writes no entry at all.
- *Walk.* A depth-first traversal in sorted name order. Each directory is one
  unit: list it completely and diff it against the rows whose `parent_key`
  is that directory. An equal stat tuple costs one `stat` and no write,
  unless the row is racily clean (Git's rule: its mtime is within the
  timestamp granularity of its last check). New and changed files become
  `pending_hash`; a file younger than the settle window is deferred. Only a
  directory whose own row is live is walked, so every entry has an indexed
  parent; a directory that appears behind its parent's diff waits for the
  next scan.
- *Hash.* Outside any transaction: stat, BLAKE3 from the open handle, stat
  again; a file that changed meanwhile stays pending. The commit is a
  compare-and-swap on the row's `revision`, records the stat tuple the
  content was hashed under, and binds content and Asset, applying the
  in-place carry-over rule of the lifecycle decision.
- *Absence.* A row with no disk entry in its directory's listing is probed
  with `lstat`. Only `ENOENT`, `ENOTDIR`, or an ancestor that is no longer a
  real directory marks a row `missing`; any other error leaves it present
  and counts a scan error, and an unreadable directory leaves all of its rows
  alone. A vanished directory takes its subtree with it, in path-ordered
  pages of 256 (the `sweeping` status). The repository marker is re-checked
  before every commit that removes anything, so an unmounted volume never
  produces `missing` rows.
- *Moves.* A missing row whose Asset has another present row is removed,
  which is Syncthing's content-based `findRename` expressed through Assets.
  Asset IDs never change on a move or a copy.
- A scan that defers a file inside the settle window schedules one follow-up
  scan of that subtree after the window.

**Watching.** One recursive `github.com/syncthing/notify` watch per online
repository feeds a Syncthing-style aggregator (10 s delay, collapse at 128
events per directory, full scan above 512) that enqueues a subtree scan. A
watch error, overflow, or start failure schedules a full scan. A periodic full
scan runs every `repository_scan.interval_seconds` with 3/4–5/4 jitter. The
watcher only shortens latency; losing every event delays the catalog but never
makes it wrong.

**Execution.** Scan rows are the catalog-derived work. One unique River job
per repository advances bounded turns and snoozes between them; all writes go
through the commit coordinator. No filesystem I/O, hashing, or sleeping
happens inside a catalog transaction, writer batches stay within 256 rows and
25 ms, and no repository-wide `COUNT` runs in a writer transaction. Uploads
and cloud imports write their entry with the known hash and stat tuple, so the
next scan does not rehash them.

**References** (read on 2026-09-24): Syncthing `lib/scanner/walk.go`,
`lib/model/folder.go` (`scanSubdirsDeletedAndIgnored`, `findRename`,
`scanOnWatchErr`), `lib/watchaggregator/aggregator.go`, and the `.stfolder`
marker check, at `94c3c1cd`; Git `racy-git.adoc` and `fsmonitor.c` at
`0f8e75ab`; Watchman `file-query.md` fresh-instance semantics at `351e995a`;
`syncthing/notify` HEAD.

## Alternatives considered

**Keep ROE and remove the quadratic count.** The minimal fix drops
`CountPendingRepositoryMaterialization` from the three writer-transaction call
sites and bounds backpressure with a `LIMIT` on the reader. It fixes the
measured cost but keeps the C0/C1 protocol, the node graph, three journal
adapters, and absence that is never proven when a journal is unavailable. It
stays the fallback only if this work slips past the rc.1 date.

**Prove absence through change journals (`C0` → crawl → `C1`).** This is the
ROE design. Journals coalesce, overflow, expire, and differ per platform and
filesystem, so the proof is often unavailable, and failing closed turns an
unavailable proof into a Location that never closes. Syncthing, Git, and
Watchman all treat a journal or watcher as a hint and fall back to a full
scan; a positive per-entry `lstat` is a proof that is always available.

**Treat the watcher as authority.** Rejected for the same reason: events are
lossy by design, and none of the reference systems trust them for absence.

**Keep the node graph so a directory rename is one row update.** With flat
path rows, renaming a directory makes its files new rows that are rehashed
and then matched to their Assets by content. Syncthing and Git make the same
trade. Rename detection through a `file_id` hint is deferred until a
measurement shows the rehash matters; `file_id` is a change detector, never
identity.

**Per-file River jobs, or follower jobs per scan.** Rejected for the reasons
the superseded record gave: one unique snoozing job per repository already
serializes the work, and per-item jobs multiply queue and writer load.

**kqueue-based watching.** Syncthing warns that kqueue needs a descriptor per
watched file; `syncthing/notify` provides recursive FSEvents, inotify, and
`ReadDirectoryChangesW` instead.

**Keep a compatibility path or migrate ROE catalogs.** Rejected by the owner:
pre-release catalogs are refused under #221 and the baseline is edited in
place before rc.1, so a second model would only hide bugs.
