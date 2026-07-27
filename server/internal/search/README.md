# Aggregate Search

`server/internal/search` owns aggregate search for non-empty queries. SQLite
performs semantic/place retrieval and authoritative relational filtering;
Bleve performs OCR retrieval. Go orchestrates the retrievers, applies weighted
reciprocal-rank fusion, paginates, hydrates assets, and emits optional debug
metadata.

## Retrievers

- `embedding`: resolves the default semantic-search space, embeds text through
  Lumen, and queries the `search_embeddings_vec` sqlite-vec virtual table.
- `ocr`: searches the rebuildable `bleve/ocr-v1` sidecar. `text_en` uses
  Bleve's English analyzer; `text_zh` uses the `lumilio_zh` no-unigram CJK
  bigram analyzer.
- `place`: searches `location_search_fts` with FTS5, joins matches through
  `location_cluster_assets`, and ranks matches with `bm25`.

Every retriever receives the same `Filter`, which mirrors the non-query parts
of `AssetFilter`: repository, owner, album, person, type, filename filter, date
range, RAW, rating, liked, camera/lens, and GPS bounding box. Bleve pre-filters
owner, repository, type, and Trash state; SQLite batch post-filters its bounded
candidate IDs for every relational/JSON condition and authorization invariant.

## Text Tokenization and Indexes

OCR documents split Unicode scripts before analysis: Han text goes to
`text_zh`, Latin text to `text_en`, and numeric/alphanumeric model tokens to
both. Queries use the same split. Language clauses are required together; each
language first searches with `AND`, then uses `OR` only to fill a short bounded
result set. A single Han character produces no OCR search token.

SQLite `ocr_results` and `ocr_text_items` are authoritative. A revisioned
SQLite outbox applies changes to Bleve in idempotent batches. A missing,
corrupt, or mapping-mismatched sidecar is discarded and rebuilt from SQLite
before HTTP starts; database restore always forces the same rebuild. Place text
continues to use content-synchronized FTS5. Semantic vectors use sqlite-vec
`vec0`.

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

Fusion happens over the combined bounded candidate pool before page slicing.
The reported total is the size of that fused set; aggregate search does not
promise an exact full-library count.

## Failure and Debug Metadata

Retrievers run concurrently. A single failure is logged and recorded in
`SearchTopResultsMeta.Sources`; successful retrievers still participate. If all
retrievers fail, aggregate search returns an error.

With `debug: true`, the response includes fused scores and per-source rank,
weight, raw retriever score, and RRF contribution for top results.
