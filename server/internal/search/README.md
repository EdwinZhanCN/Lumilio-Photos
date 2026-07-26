# Aggregate Search

`server/internal/search` owns the SQLite aggregate-search path for non-empty
queries. SQLite performs candidate retrieval and filtering; Go orchestrates the
retrievers, applies weighted reciprocal-rank fusion, paginates, hydrates assets,
and emits optional debug metadata.

## Retrievers

- `embedding`: resolves the default semantic-search space, embeds text through
  Lumen, and queries the `search_embeddings_vec` sqlite-vec virtual table.
- `ocr`: searches `ocr_search_fts` with FTS5 and ranks matches with `bm25`.
- `place`: searches `location_search_fts` with FTS5, joins matches through
  `location_cluster_assets`, and ranks matches with `bm25`.

Every retriever receives the same `Filter`, which mirrors the non-query parts of
`AssetFilter`: repository, owner, album, person, type, filename filter, date
range, RAW, rating, liked, camera/lens, and GPS bounding box.

## Text Tokenization and Indexes

CJK character runs are split into overlapping two-character pairs before
storage and at query time. Non-CJK text remains whole words. For example,
`"听说你还在找白样"` becomes `"听说 说你 你还 还在 在找 找白 白样"`.

The baseline creates content-synchronized FTS5 indexes for OCR and location
text. Semantic vectors use the sqlite-vec `vec0` table and a fixed canonical
dimension; application rows retain their model and search-space metadata.

## Fusion

Weighted RRF uses:

```text
score(asset) = sum(source_weight / (60 + rank_in_source))
```

Default weights are embedding `1.0`, place `0.8`, and OCR `0.7`.

## Pagination and Candidate Pool

Each retriever receives a top-K candidate pool sized from the requested page:

```text
topK = clamp((offset + limit) * 4, 50, 1000)
```

Fusion happens over the combined candidate pool before page slicing. When an
exact total is requested, the successful retrievers contribute set queries
that SQLite unions before counting distinct asset IDs.

## Failure and Debug Metadata

Retrievers run concurrently. A single failure is logged and recorded in
`SearchTopResultsMeta.Sources`; successful retrievers still participate. If all
retrievers fail, aggregate search returns an error.

With `debug: true`, the response includes fused scores and per-source rank,
weight, raw SQLite score, and RRF contribution for top results.
