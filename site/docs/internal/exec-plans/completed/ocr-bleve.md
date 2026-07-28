# OCR Search: SQLite Authority + Bleve Sidecar

Status: **completed** (2026-07-27). Target branch `experimental/sqlite`, base
`2060a76d324b530330faaa6a3445e21e3b068ed3`.

## Goal

Replace the OCR-only SQLite FTS5 implementation with one rebuildable Bleve v2
sidecar while retaining Lumen inference and all authoritative OCR results,
boxes, and confidences in SQLite. There is no feature flag, shadow path,
fallback, old-index reader, or compatibility mode.

The final branch is squash-merged as one commit:

```text
feat(search)!: replace OCR FTS5 with Bleve
```

## Invariants

- SQLite remains the source of truth. Deleting
  `<sqlite-directory>/indexes/bleve/ocr-v1/` and restarting reproduces the
  complete OCR index before HTTP starts.
- The Bleve document contains `asset_id`, `text_en`, `text_zh`, `owner_id`,
  `repository_id`, `asset_type`, `is_deleted`, and `revision`.
- OCR text is split by Unicode script. Han text is written only to `text_zh`;
  Latin text only to `text_en`; numeric and alphanumeric model tokens are
  written to both. The whole OCR text is never duplicated into both fields.
- `text_en` uses Bleve's built-in `en` analyzer. `text_zh` uses
  `lumilio_zh`: Unicode tokenizer, CJK width normalization, CJK bigram with
  `output_unigram=false`, then lowercase. A one-Han-character document or query
  run produces no OCR token/clause.
- Mixed-language queries keep language clauses as Boolean `Must`. Within each
  language, strict retrieval uses `AND`; if fewer than `TopK` authorized,
  post-filtered candidates survive, relaxed retrieval uses `OR` to fill the
  remainder. Strict hits stay ahead of relaxed-only hits.
- Bleve pre-filters owner, repository, asset type, and Trash state. SQLite
  batch post-filters candidate IDs for albums, people, ratings, tags, dates,
  GPS, and every other relational or JSON condition.
- `ocr_index_metadata` is a per-asset monotonic revision ledger.
  `ocr_index_outbox` is keyed by asset and retains only the newest pending
  revision. An OCR save/delete/Trash/restore transaction mutates authoritative
  state, increments the revision, and upserts the outbox row atomically.
- A worker reads a bounded outbox batch, reads the latest SQLite document,
  applies one Bleve batch, then conditionally acknowledges the same outbox
  revision. A crash before acknowledgement causes an idempotent retry; a newer
  SQLite mutation cannot be acknowledged by an older worker.
- The index stores its mapping version internally. Missing, corrupt, or
  mismatched indexes are deleted and fully rebuilt. A database restore always
  forces this path.
- Search totals describe the current bounded fused result set. No retriever SQL
  `CountQuery` mechanism or full-library exact total remains.

## Implementation

1. Add a destructive migration that drops the OCR FTS table/triggers, rebuilds
   `ocr_results` without `full_text`, and creates/seeds the metadata and outbox
   tables. Keep Place and other FTS5 structures unchanged.
2. Replace OCR sqlc operations with transaction-oriented authoritative writes,
   revision/outbox operations, bounded outbox reads, conditional ack, and
   cursor-batched rebuild/document reads from `ocr_text_items`.
3. Add `internal/search/bleveocr/{mapping,document,index,query,writer,rebuild}.go`.
   Build a static mapping and an atomically replaced on-disk index at
   `<sqlite-directory>/indexes/bleve/ocr-v1/`.
4. Register a single-worker River queue plus a frequent unique periodic tick.
   The worker drains bounded batches without monopolizing the SQLite writer.
5. Inject the Bleve index into `BleveOCRRetriever`; remove SQLite OCR retrieval,
   OCR FTS token preparation, `SearchAssetsByOCRText`, `UpdateOCRFullText`, and
   exact aggregate counting. Keep OCR RRF weight `0.7`.
6. Make `SaveOCRResults`, explicit OCR deletion, asset Trash, and restore short
   SQLite transactions that enqueue the matching revision.
7. Initialize/rebuild the sidecar after migrations and before HTTP; close it
   only after River drains. Force rebuild for a restored SQLite generation.
8. Update backend/search architecture docs and move this plan to `completed/`
   only after all gates and CI pass.

## Required Verification

- Analyzer/document: English stemming, case folding, stop words; simplified and
  traditional Chinese bigrams; mixed Chinese/English plus numeric/model tokens;
  one-Han-character suppression.
- Query: strict ranking followed by relaxed fill; mixed-language `Must`;
  owner/repository/type/Trash Bleve filters; relational SQLite post-filter.
- Mutation: OCR insert/update/delete, Trash/restore, revision monotonicity, and
  atomic rollback.
- Recovery: crash after Bleve write before outbox ack, missing/corrupt/mapping-
  mismatched index rebuild, and forced rebuild after restore.
- Destructive audit: repository search finds no `ocr_search_fts`,
  `UpdateOCRFullText`, `SearchAssetsByOCRText`, OCR branch of `ftsMatchQuery`,
  `NewOCRRetriever`, OCR SQL `CountQuery`, legacy flag, or compatibility code.
- Generation and gates: `cd server && sqlc generate`, `gofmt`, focused tests,
  `make server-test`, relevant desktop/native build coverage, then GitHub CI.

## Critical Files for Implementation

- `server/migrations/000002_ocr_bleve.up.sql`
- `server/internal/db/repo/queries/ocr.sql`
- `server/internal/search/bleveocr/index.go`
- `server/internal/search/retrievers.go`
- `server/app/app.go`

## Progress and Evidence

Implementation and local verification are complete in commit
`feat(search)!: replace OCR FTS5 with Bleve`, based directly on
`2060a76d324b530330faaa6a3445e21e3b068ed3`.

- `cd server && sqlc generate` regenerated the OCR models, query methods, and
  `Querier` interface after the destructive migration/query changes.
- `make server-test` passes with the SQLite architecture check and every Go
  package, including the migration, analyzer/retriever, outbox recovery,
  corruption/rebuild, and OCR lifecycle integration tests added by this work.
- `make web-test` passes: 63 test files passed, 2 skipped; 256 tests passed,
  6 skipped.
- `make desktop-test` passes, including the native module and SQLite
  first/second-launch plus restore-generation coverage. `desktop/go.mod` and
  `desktop/go.sum` were tidied because the desktop module compiles the replaced
  sibling server module.
- `cd site && vp run docs:build` completes successfully.
- A destructive source audit finds no OCR FTS query, `SearchAssetsByOCRText`,
  `UpdateOCRFullText`, `NewOCRRetriever`, OCR `CountQuery`, or `CountTotal`
  runtime symbol. The only `ocr_search_fts` definitions left are immutable
  migration history plus the new destructive migration/test that removes and
  verifies removal of those objects.
- GitHub Actions run `30268181652` validated the site, Linux server, macOS
  desktop, and Windows desktop jobs. Its web job exposed stale Docker
  `COPY` instructions for River fork directories removed earlier on this
  branch. Both `server/Dockerfile` and the isolated E2E
  `fakelumen.Dockerfile` now consume the upstream River modules declared in
  `server/go.mod` without copying deleted fork directories.
- Both corrected Dockerfiles build successfully through OrbStack from an
  18 MB clean-checkout context: the deterministic Lumen fixture and the full
  production image (web bundle, SQLite/CGO server, and runtime layers).

GitHub Actions run `30284528168` completed successfully for commit
`44b24123e08059ed580f61b1d7136bb900e88947`: the required aggregate, change
detection, SQLite architecture, Linux server, VitePress site, macOS desktop,
Windows desktop, Web unit, isolated Docker environment, Chromium smoke,
authentication hardening, video semantic, and database recovery checks all
passed. This record was then moved to `exec-plans/completed/`.
