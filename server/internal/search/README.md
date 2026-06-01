# Aggregate Search

`server/internal/search` owns the PostgreSQL aggregate search path for non-empty user queries. It keeps retrieval in PostgreSQL and keeps Go responsible only for orchestration, Weighted Reciprocal Rank Fusion, pagination, hydration, logging, and optional debug metadata.

## Retrievers

- `embedding`: resolves the default CLIP search space, embeds the text query through Lumen, and retrieves nearest primary asset embeddings with pgvector.
- `ocr`: searches `ocr_text_items.search_vector` with `plainto_tsquery('simple', query)` and ranks assets by `ts_rank_cd`.
- `place`: searches `location_clusters.search_vector`, joins through `location_cluster_assets`, and ranks assets by `ts_rank_cd`.

Every retriever receives the same `Filter`, which mirrors the non-query parts of `AssetFilter`: repository, owner, album, person, type, filename filter, date range, RAW, rating, liked, camera/lens, and GPS bounding box.

## PostgreSQL Indexes

Migration `1769558401_026_aggregate_search_indexes` adds:

- `ocr_text_items.search_vector` stored generated `tsvector` plus a GIN index.
- `location_clusters.search_vector` stored generated `tsvector` over `label`, `country`, `region`, `city`, and `geohash` plus a GIN index.

Embedding search uses the existing per-space HNSW indexes created by `embedding_service.ensureSearchIndexForSpace`, for example `embeddings_space_<id>_primary_hnsw_l2_idx`, because embedding dimensions can differ by space.

The retriever SQL predicates match these indexes: text retrievers use `search_vector @@ plainto_tsquery(...)`, while embedding retrieval orders by pgvector distance.

## Fusion

Weighted RRF uses:

```text
score(asset) = sum(source_weight / (60 + rank_in_source))
```

Default weights:

- embedding: `1.0`
- place: `0.8`
- OCR: `0.7`

These weights favor visual semantic similarity while still allowing strong place/OCR hits to move assets upward when they agree with another retriever.

## Pagination And Candidate Pool

Each retriever receives a topK candidate pool sized from the requested page boundary:

```text
topK = clamp((offset + limit) * 4, 50, 1000)
```

Fusion happens over the combined candidate pool, then `offset + limit` pagination is applied to the fused ranking. This avoids asking PostgreSQL for only one page from each modality, which can drop cross-source winners before fusion.

## Failure And Debug Metadata

Retrievers run concurrently. A single retriever failure is logged and marked in `SearchTopResultsMeta.Sources`; the remaining retrievers still participate in fusion. If every retriever fails, aggregate search returns an error.

When `debug: true` is passed to `/api/v1/assets/search`, the top-results meta includes per-asset fused scores and per-source contributions: rank, weight, raw PostgreSQL score, and RRF contribution.
