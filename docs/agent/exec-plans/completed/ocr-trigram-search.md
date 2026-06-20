# OCR Search: zhparser/BM25 → pg_trgm + CJK Bigrams

## Problem

zhparser's SCWS dictionary (`dict.utf8.xdb`) is missing from the Docker image. Without it, zhparser degrades to single-character tokenization, making BM25 search return all 803 OCR records with score=0 — effectively random results. The zhparser/pg_textsearch stack adds fragile external dependencies (SCWS dictionary, `shared_preload_libraries`) for no benefit over simpler alternatives.

## Solution

Replace the entire OCR search pipeline with pg_trgm (already installed) + application-level CJK bigram tokenization, matching the approach used by Immich. This eliminates zhparser, pg_textsearch, and SCWS as dependencies entirely.

**Key idea:** CJK text is split into overlapping 2-character bigrams before storage and at query time. pg_trgm's GIN index provides fuzzy matching with no external dictionary. Non-CJK text (Latin, numbers) is kept as whole words.

Example: `"听说你还在找你的白样"` → `"听说 说你 你还 还在 在找 找你 你的 的白 白样"`
Query `"白样"` → tokenized to `"白样"` → trigram match against stored bigrams → hit.

## Scope

### New files

| File | Purpose |
|------|---------|
| `server/internal/search/tokenize.go` | `TokenizeForSearch()` — CJK bigram + word splitter |
| `server/internal/search/tokenize_test.go` | Unit tests for tokenizer |

### Modified files

| File | Change |
|------|--------|
| `server/migrations/000001_foundation.up.sql` | Remove `pg_textsearch`, `zhparser`, `chinese_zh` config |
| `server/migrations/000001_foundation.down.sql` | Remove drops of `zhparser`, `pg_textsearch`, `chinese_zh` |
| `server/migrations/000005_ml_analysis_results.up.sql` | Replace BM25 index with GIN trigram index |
| `server/internal/search/retrievers.go` | OCR retriever: `<@>` BM25 → `word_similarity` + `%>` trigram |
| `server/internal/service/ocr_service.go` | Write path: tokenize before storing `full_text`; search: trigram |
| `server/internal/search/README.md` | Update documentation |
| `server/db.Dockerfile` | Remove zhparser/SCWS build steps |
| `.devcontainer/docker-compose.yml` | Remove `shared_preload_libraries=pg_textsearch` |
| `desktop/supervisor/postgres.go` | Remove `shared_preload_libraries = 'pg_textsearch'` |

### Removed dependencies

- `pg_textsearch` extension (BM25 access method) — no other users
- `zhparser` extension (Chinese word segmentation) — only used by `chinese_zh` config
- SCWS library + dictionary — runtime dependency of zhparser
- `chinese_zh` text search configuration — only used by BM25 index

## Steps

### 1. Tokenizer (`server/internal/search/tokenize.go`)

```go
func TokenizeForSearch(text string) string
```

- CJK ranges: U+4E00–9FFF, U+AC00–D7AF, U+3040–309F, U+30A0–30FF, U+3400–4DBF
- CJK runs: sliding-window bigrams (`"白样"` → `["白样"]`, `"你好吗"` → `["你好","好吗"]`)
- Single CJK char: emit as-is
- Non-CJK runs: emit as whole words (split on whitespace)
- Join with spaces

### 2. Migrations (destructive edit)

**000001_foundation.up.sql:**
- Remove lines 5-6 (`CREATE EXTENSION pg_textsearch/zhparser`)
- Remove lines 132-157 (`chinese_zh` text search configuration)

**000001_foundation.down.sql:**
- Remove line 1 (`DROP TEXT SEARCH CONFIGURATION chinese_zh`)
- Remove lines 12-13 (`DROP EXTENSION zhparser/pg_textsearch`)

**000005_ml_analysis_results.up.sql:**
- Line 669: replace `CREATE INDEX ocr_results_bm25_idx ... USING bm25 ... WITH (text_config='public.chinese_zh')` with `CREATE INDEX ocr_results_trgm_idx ON public.ocr_results USING gin (full_text gin_trgm_ops)`

### 3. OCR write path (`server/internal/service/ocr_service.go`)

In `ProcessOCRResult`, tokenize before storing:
```go
fullText := search.TokenizeForSearch(strings.Join(texts, " "))
```

### 4. OCR search retriever (`server/internal/search/retrievers.go`)

**`retrieveOCR`:** Replace BM25 `<@>` with trigram `word_similarity`:
```sql
-- Filter (GIN-accelerated via %> operator, threshold set per-tx)
SET LOCAL pg_trgm.word_similarity_threshold = 0.15;
-- Score
word_similarity($1, r.full_text)::float8 AS raw_score
-- Filter condition
r.full_text %> $1
```

Wrap in a transaction for `SET LOCAL`. The tokenized query is passed as the parameter.

**`ocrCountQuery`:** Same filter change.

### 5. OCR service search (`server/internal/service/ocr_service.go`)

`SearchAssetsByText`: same trigram approach as the retriever.

### 6. Docker/Desktop cleanup

- `server/db.Dockerfile`: remove SCWS build, zhparser build, SCWS COPY steps, SCWS ldconfig, `shared_preload_libraries` in postgresql.conf.sample
- `.devcontainer/docker-compose.yml`: remove `shared_preload_libraries=pg_textsearch` from command
- `desktop/supervisor/postgres.go`: remove `shared_preload_libraries = 'pg_textsearch'` from conf template

### 7. Documentation

Update `server/internal/search/README.md` to reflect trigram approach.

## Validation

```bash
# 1. Rebuild DB image
docker compose -f .devcontainer/docker-compose.yml build db

# 2. Nuke existing volume and re-init
docker compose -f .devcontainer/docker-compose.yml down -v
docker compose -f .devcontainer/docker-compose.yml up -d db

# 3. Run migrations
make server-migrate  # or however migrations run

# 4. Server tests
make server-test

# 5. Manual DB verification
psql> SELECT * FROM pg_extension WHERE extname IN ('pg_trgm','pg_textsearch','zhparser');
-- Should show only pg_trgm
psql> SELECT indexname FROM pg_indexes WHERE tablename = 'ocr_results' AND indexname LIKE '%trgm%';
-- Should show ocr_results_trgm_idx

# 6. End-to-end: ingest an image with OCR, search for a substring
```
