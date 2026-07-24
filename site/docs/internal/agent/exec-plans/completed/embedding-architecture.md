# Asset Embedding Architecture (Unified Search Vectors)

Status: **completed & verified** (2026-07). Destructive refactor done pre-production.
Replaced the dimensionless polymorphic `embeddings.vector` column (no ANN index
possible → brute-force scan) with a fixed-dimension, HNSW-indexed, unified search
vector store. Prerequisite for [video-semantic-search](../active/video-semantic-search.md).

## Why

`embeddings.vector` was dimensionless (`vector`, no `(N)`) to hold any model's
output. That flexibility was largely illusory — changing the embedding model
always requires a full re-embed (vectors from different models are incomparable),
so you can never search across models in one pool. The real cost was that
pgvector HNSW requires a fixed dimension, so semantic `<->` ran as a brute-force
scan (only `face_items` had HNSW). We committed to the **SigLIP2** family, which
is Matryoshka-trained, so any model's output can be truncated to one canonical
dimension with negligible retrieval/zero-shot loss.

## Decisions (as shipped)

- **Canonical dimension 768.** `canonicalizeSemanticVector` truncates every model
  output to the leading 768 dims, then L2-normalizes. base is natively 768;
  so400m 1152→768. Applied at all embed sites: stored image vectors (inside
  `SaveEmbedding` for `semantic`), text queries (`resolveSemanticQueryEmbedding`),
  and zero-shot label prototypes (`buildPrototype`). Canonical unifies the schema
  only — a model change still needs a full re-embed.
- **L2 on unit vectors, not cosine.** Vectors are stored unit-length, so L2 ranks
  identically to cosine — and keeping the `<->` operator preserves the existing
  set-retrieval cutoff math (`cutoff = √(2·(1−cosFloor))`, which already assumed
  unit vectors) with zero recalibration. HNSW uses `vector_l2_ops`.
- **One unified table, multi-row per asset.** `search_embeddings`: photo = 1 row
  (`frame_ts_ms IS NULL`), video = N frame rows. All semantic search paths
  (aggregate retriever, set-retrieval, direct vector search, zero-shot preview)
  query it and **max-pool per asset** (`MIN`/`MAX` + `GROUP BY asset_id`), so an
  asset ranks by its best frame. `frame_ts_ms` is written only by the video plan.
- **`embedding_spaces` kept, repurposed as active-config.** It records the active
  `model_id`/`dimensions`(768)/`metric`, routes the query model, and detects model
  changes. `search_embeddings.space_id` FKs it. pHash still uses it too.
- **pHash stays in the old `embeddings` table**, unchanged (its `is_primary`
  queries and the duplicates join are untouched). Only semantic moved out.
- **Model swap = drop + refill (Immich-style), no schema migration.** The
  `reset_semantic` flag on the reindex endpoint wipes `search_embeddings` and
  demotes the default space on the first page, then rebuilds; the first new
  embedding promotes the new model's space. Search returns fewer results during
  refill but never mixes models. Zero-downtime blue-green is a non-goal.

## As-built shape

- Migration `000012_search_embeddings`: `search_embeddings(id identity, asset_id
  fk, space_id fk, frame_ts_ms int null, vector vector(768), model_id, created_at)`;
  partial unique on `(asset_id) WHERE frame_ts_ms IS NULL` and `(asset_id,
  frame_ts_ms) WHERE frame_ts_ms IS NOT NULL`; `search_embeddings_asset_idx`; HNSW
  `vector_l2_ops (m=16, ef_construction=200)`.
- Storage: `SaveEmbedding(semantic)` = resolve space → delete asset rows → insert
  one primary row. `GetPrimaryEmbeddingVector(semantic)` reads the `frame_ts_ms IS
  NULL` row. Indexing stats queries repointed to `search_embeddings`.
- `best_ts` (nearest-frame timestamp for video deep-linking) is **deferred to
  [video-semantic-search](../active/video-semantic-search.md)**; not needed while
  only photos (single row) exist.

## Lumen SDK contract (v1.3.2)

`SemanticImageEmbed`/`SemanticTextEmbed` return `*types.EmbeddingV1{Vector, Dim,
ModelID, AestheticScore}`. The Hub returns the model's **native** dimension and
`EmbeddingRequest` has no output-dim parameter, so truncation to 768 is
server-side. `ModelID` is the active-model identity for query routing / change
detection.

## Verification (live, 225 demo photos, SigLIP2 base, AI on)

- Migration at version 12, not dirty.
- `search_embeddings`: 225 rows, all `frame_ts_ms IS NULL` (photos), 1 row/asset,
  single `model_id = siglip2-base-patch16-224`.
- Vectors `dims=768`, `norm=1.0000` (canonicalize verified).
- Default semantic space `dim=768 / metric=l2 / is_default_search=true`.
- Indexes present incl. `search_embeddings_vector_hnsw_l2_idx` + both partial
  uniques; NN `<->` returns self at distance 0; HNSW `Index Scan` confirmed usable
  when parameterized (planner correctly prefers seq scan at 225 rows).
- `go build ./...` + `go test ./...` green; `gofmt` clean; `make dto` regenerated.

## Non-goals

- Blue-green / zero-downtime model swaps.
- Cross-model search (impossible without re-embed).
- Moving pHash/attribute vectors into the ANN table.
- Video frame extraction / `best_ts` plumbing — owned by
  [video-semantic-search](../active/video-semantic-search.md).

## Follow-ups

- Settings UI control for the `reset_semantic` reindex flag (backend + DTO done).
- Strictness cutoffs and RRF weights may want re-tuning after a real re-embed.

## Key files

`server/migrations/000012_*`, `internal/db/repo/queries/{search_embeddings,indexing,embeddings}.sql`,
`internal/service/embedding_canonical.go`, `internal/service/embedding_service.go`,
`internal/search/{retrievers,setretrieve}.go`, `internal/service/{asset_service,classifier_service,indexing_service}.go`.
