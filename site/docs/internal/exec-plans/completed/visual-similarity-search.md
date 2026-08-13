# Visual Similarity Search

Status: completed on 2026-08-13

## Goal

Let a user find catalog media that looks like a query image, using the existing
Image Semantic Analysis index (`search_embeddings` + Vec1 KNN). Two query
sources, one retrieval, one gallery: catalog similar (stored vectors) and
search-by-image (live `semantic_image_embed`).

## Final Contracts

- `search.Request.QueryEmbedding` skips `semantic_text_embed` when set.
- Catalog query vector: photo primary (`frame_ts_ms IS NULL`), else video
  earliest frame. Missing row is `embedding_missing` (409), not an empty list.
- Image queries are embedding-only. Cosine floor **0.25**. Exclude the query
  `media_item_id`. Paginate the ranked slice; `sort_by` is ignored.
- `POST /api/v1/assets/search` accepts `similar_to_asset_id`, mutually
  exclusive with `query`. 404 if the asset is missing or not visible; 503 if
  the retriever is down. No live Lumen for this path.
- `POST /api/v1/assets/search/by-image` multipart field `file`, 256 MiB cap.
  Original → in-memory medium WebP (same 800px derivative as catalog ingest,
  RAW via `OpenPhoto`) → `semantic_image_embed`. The file is not stored. 400
  unsupported/missing; 503 if image embed is unavailable.
- Browse URL: `?similar=<uuid>` mutually exclusive with `?q=`. File queries are
  React state only.
- SearchFAB: right-aligned **Search by image** / **图片搜索** toggle above the
  expanded `w-72` slot; the slot morphs into a PhotoPicker well with a trailing
  file icon plus drop/paste. Capability labels stay **Image Semantic Analysis**
  / **图像语义分析**.
- Viewer share/export menu includes **Similar** / **相似图** (tooltip). It
  opens a bottom rail (`limit=12`). Slide change closes it. See all goes to
  `/assets?similar=<id>`.

## Useful Decisions

- Text-query cosine floors are not reused for image→image.
- PhotoPicker remains the only catalog picker. Album pick and local file are
  not equal-width peer buttons.
- Pin search rejects `similar_to_asset_id`. Agent `search_similar` is a
  follow-up.
- SearchFAB nests the toggle + slot inside an inner column so daisyUI `.fab > *`
  flex-row styling cannot sit them side by side.

## Validation Boundaries

- `task dto`
- `task server:test` (retriever, handler, visual search integration)
- `task web:test`
- i18n extract then filled `zh`
- `gofmt`; Vite+ fmt/lint
- `web/src/features/assets/doc.ts` (generated `doc.md`)
