# Visual Similarity Search

Status: active. Product and API contracts below are frozen for implementation.

Primary owners: `server/internal/search`, `server/internal/service` (asset
search + embeddings + Lumen image embed),
`server/internal/api/handler/asset_handler.go`,
`web/src/features/assets` (browse route state, SearchFAB, AssetViewer).

Prerequisite plans (shipped): [embedding-architecture](../completed/embedding-architecture.md),
[vec1-vector-search](../completed/vec1-vector-search.md),
[video-semantic-search](../completed/video-semantic-search.md).

## Goal

Let a user find catalog media that looks like a query image, using the existing
Image Semantic Analysis index (`search_embeddings` + Vec1 KNN).

Two query sources, one retrieval, one result surface:

1. **Similar media** — the query is a catalog asset the user already has
   (the fullscreen viewer, or a PhotoPicker pick).
2. **Search by image** — the query is a local file that is not (yet) in the
   catalog.

Text semantic search stays the primary SearchFAB path. Image query is an extra
mode, not a replacement.

## Current baseline

Already shipped and must be reused:

- Photos write one `search_embeddings` row (`frame_ts_ms IS NULL`); videos write
  N frame rows. All semantic paths max-pool per asset.
- Vectors are canonical 768-dim L2 unit vectors. Query vectors must pass
  `canonicalizeSemanticVector`.
- `EmbeddingRetriever.Retrieve` already does Vec1 KNN from a query vector.
  `resolveQuerySpace` currently always text-embeds `req.Query` via
  `semantic_text_embed`.
- `GetPrimaryEmbeddingVector(assetID, semantic)` reads the photo primary row.
  Videos have no NULL-primary row, so this helper cannot be the video query
  source as-is.
- `LumenService.SemanticImageEmbed` already embeds an `imagesource.MLImage`.
  Indexing workers use it; search does not.
- `POST /api/v1/assets/search` accepts a text `query` and fuses embedding /
  OCR / place / filename with text-query cosine floors (~0.105). Empty `query`
  does not run enhancement.
- Browse search is URL state (`?q=`). `SearchFAB` expands a text input; its
  backdrop blur is visible only while that input is non-empty.
- `PhotoPicker` is the reviewed single-selection catalog picker.
- `AssetViewer` already has a collapsible BioCLIP field-guide overlay and an
  action flower. It has no similar-media surface.
- Duplicate detection (pHash / perceptual hash) is a different product. Do not
  route similarity search through it.

The gap is the query-vector source and the UI/URL to select it. The ANN table,
canonicalization, hydration, and gallery already exist.

## Product model

```text
query vector
  ├─ catalog asset  → stored search_embeddings row (Lumen not required)
  └─ local file     → live semantic_image_embed   (Lumen required)
        │
        v
Vec1 KNN + exact rerank (existing embedding retriever)
        │
        v
flat ranked media-item gallery (existing AssetBrowser search view)
```

Carousel similar-media is a **preview** of that same ranked set (small K), not
a second list type. "See all" opens the gallery search with the same query
asset.

## Non-goals

- Do not replace or hide the text SearchFAB input.
- Do not use pHash / duplicate groups as the similar-media ranking.
- Do not reuse text set-retrieval cosine floors for image queries. Those floors
  are calibrated for text→image, not image→image.
- Do not expose raw embedding vectors on the HTTP API.
- Do not persist an uploaded query file as an asset, and do not write its
  embedding into `search_embeddings`.
- Do not add TOML / settings knobs for K, nprobe, or similarity floors.
- Do not add an agent `search_similar` tool, pin search-by-image, or smart-album
  "similar to this asset" in this plan.
- Do not auto-fetch similar media on every carousel slide change.
- Do not invent a new gallery, rail card language, or browse-item model.
- Do not hand-edit generated OpenAPI or `web/src/lib/http-commons/schema.d.ts`.

## Decisions

### One ranked list, embedding channel only

Image queries do not fuse OCR, place, or filename. `enhancement_mode` is forced
to embedding-only internally; a client value is ignored. `sort_by` is ignored:
order is similarity, then `asset_id DESC` as the existing KNN tie-break.

Response reuses `SearchAssetsResponseDTO`. Ranked hits go in `result_items` in
similarity order. `top_items` is empty. `top_results_meta.enabled` is true,
`source_types` is `["embedding"]`. This renders as the existing search-results
gallery without the "best matches" strip.

### Ranking, not membership

Retrieve Top-K from Vec1 (same expansion / nprobe / exact rerank as
`EmbeddingRetriever.Retrieve`). Cap K at the existing `top_results_limit`
maximum (200). Paginate by slicing that ranked list. Do not claim completeness
beyond the cap.

A conservative cosine floor of **0.25** drops unrelated tail hits. This is a
code constant next to the retriever, not a user control. Image-to-image near
duplicates will score far above it; typical same-scene / same-subject hits
should pass; random catalog noise should not. Tune only with a fixture
comment and a test, not a setting.

### Exclude the query media item, not only the asset

When the query is a catalog asset, drop every result that shares its
`media_item_id` (RAW+JPEG, Live Photo motion, etc.). Do not show the query
photo as the first "similar" hit.

### Query vector source

| Query | Vector | Live Lumen |
| --- | --- | --- |
| Catalog photo | `search_embeddings` row where `frame_ts_ms IS NULL` | No |
| Catalog video | earliest frame row (`ORDER BY frame_ts_ms ASC LIMIT 1`) | No |
| Local file | `SemanticImageEmbed` after `imagesource` ML preprocess + canonicalize | Yes (`semantic_image_embed` available) |

If the catalog asset has no usable row, fail with a dedicated error (not an
empty list). Empty means "indexed but nothing similar", which is a different
state from "this asset was never embedded".

The query asset must be visible to the caller under the same ownership rules as
`GET /api/v1/assets/{id}`.

### Filters and scope

`AssetFilterDTO` still applies (album, person, folder, type, date, …), so
SearchFAB image search stays inside the current browse constraint — same as
text search.

Carousel **preview rail** is owner-scoped library-wide (current ownership, no
album/folder constraint) so similar media is not trapped in the current
collection. **See all** navigates to the main library browse URL with
`?similar=<assetId>`, not the album/person/folder route.

### URL state

Extend `AssetBrowseRouteState`:

- `q` — text query (existing)
- `similar` — catalog asset UUID

They are mutually exclusive. Serializing one clears the other. A local-file
query is **not** URL state: it lives in SearchFAB / React Query until cleared.
File queries cannot be shared as links.

`isSearchActive` is true when `q` is non-empty **or** `similar` is set **or** a
file query is mounted.

## API contracts

OpenAPI-first. After DTO/annotation changes run `task dto`.

### `POST /api/v1/assets/search`

JSON body gains:

```text
similar_to_asset_id?: uuid
```

Existing `query` remains. Exactly one of `query` (non-empty) or
`similar_to_asset_id` may be set. Both or neither with no other search intent
is 400. Filters-only browse stays on `POST /api/v1/assets/list`.

`similar_to_asset_id` path:

- 200 + ranked `result_items` when the asset is visible and indexed.
- 404 when the asset is missing or not visible.
- 409 when the asset is visible but has no semantic vector
  (`reason=embedding_missing` or equivalent typed error body already used by
  the API).
- 503 when the Vec1 / embedding retriever is not configured
  (`ErrSemanticSearchUnavailable`).

Do not require a live Lumen node for this path.

### `POST /api/v1/assets/search/by-image`

Multipart, not JSON. Same filter / pagination / `top_results_limit` fields as
search, plus one image file part. Response is `SearchAssetsResponseDTO` with
the same ranked-list mapping.

- 400 for missing file, unsupported type, or oversize.
- 503 when `semantic_image_embed` is unavailable or the infer call fails.
- The file is read into `imagesource`, embedded, discarded. No catalog write.

File bounds are code constants, not TOML: decode through the existing ML image
pipeline; reject non-images; cap upload bytes (20 MiB is enough for a phone
JPEG/HEIC and rejects casual RAW dumps). Do not ingest via the asset upload
session machinery.

Pin search endpoints are unchanged.

### Retriever change

`search.Request` gains an optional precomputed `QueryEmbedding`. When present,
`EmbeddingRetriever.resolveQuerySpace` uses it and must not call
`semantic_text_embed`. Text search is unchanged.

A small service helper resolves catalog/file bytes → canonical query embedding
→ `Retrieve` with `TopK`, then drops the query media item and hits below the
cosine floor, then hydrates through the existing search browse assembler.

## UI

### SearchFAB — keep the text input

Do not put the image-search entry in the backdrop-blur region. That blur only
exists after the user has already typed.

Expanded FAB layout:

```text
[ image button ] [ text input ................ ] [ search trigger ]
```

The image button opens a compact menu with two actions:

- **From library** — `PhotoPicker` in a modal, `selectionMode: "single"`,
  photos-only default. Confirm writes `?similar=<id>` and closes the picker.
- **From file** — hidden `<input type="file" accept="image/*">`. Success mounts
  an ephemeral file query (thumbnail chip + clear). Failure (503 / 400) stays
  on the FAB with an explicit message.

While a similar/file query is active, the text input is empty and a chip shows
the query thumbnail. Clearing the chip or the FAB close control returns to
ordinary browse.

Gate the image button with `useCapabilities()`:

- Library similar: Image Semantic Analysis **enabled** (indexed vectors may
  exist even if the node is currently down). If disabled, the action explains
  that Image Semantic Analysis is off.
- File search: additionally require `semantic_image_embed` **available**. If
  enabled but the node is down, the action explains that the task is
  unreachable.

Do not hide the control entirely when ML is off; keep the base browse workflow
usable and make the disabled reason obvious (`DESIGN.md` AI/ML guidance).

### AssetViewer — bottom similar rail

Add a **Similar** action to `AssetViewerActions` (flower), not a BioCLIP-style
left card. Similar media is visual; the preview is a horizontal thumbnail strip
along the bottom of the lightbox (8–12 items).

- Fetch only when the user opens Similar for the current asset.
- Changing slides closes the rail; the next asset does not inherit it.
- Thumb click: if that media item is already in the current carousel set,
  slide to it; otherwise close the viewer and navigate to main-library
  `?similar=<queryAssetId>` with that asset selected if it is on the first
  page.
- **See all** always goes to main-library `?similar=<queryAssetId>`.
- Missing embedding / ML-off / empty / error are distinct rail states. Empty
  is "nothing similar enough", not "broken".

Reuse gallery thumbnail rendering (`MediaThumbnail` or the square tile), do
not design a new card.

Copy uses the canonical capability name **Image Semantic Analysis** /
**图像语义分析** where the capability is named. The action itself may be
"Similar" / "相似图" and "Search by image" / "图搜图".

## Implementation

### Phase 1 — Query-vector retrieval

1. Optional `QueryEmbedding` on `search.Request`; skip text embed when set.
2. Catalog query-vector helper (photo primary, video earliest frame).
3. Service method: similar-to-asset → KNN → exclude media item → floor →
   hydrate as `SearchBrowseResult`.
4. `similar_to_asset_id` on `SearchAssetsRequestDTO` + handler validation.
5. Tests: mutual exclusion with `query`; 404 / 409; self/media-item exclusion;
   ranking order; no Lumen client call; video query uses a frame row.

### Phase 2 — Browse URL + gallery

1. `similar` in `browseRouteState` (parse/serialize, mutual exclusion with `q`).
2. `useAssetBrowser` / `useAssetsSearch` treat `similar` as search-active and
   send `similar_to_asset_id`.
3. Empty/error copy for missing embedding vs no hits vs retriever down.
4. Unit tests for the URL codec; component/flow tests for search-active gallery
   switching.

### Phase 3 — Viewer rail

1. Viewer action + bottom rail using Phase 1 search with `limit=12`.
2. See-all navigation to `/assets?similar=<id>` (main library).
3. Capability / 409 / empty states.
4. Component tests for open/close, no prefetch on slide change, see-all href.

### Phase 4 — SearchFAB image mode

1. Image button + library PhotoPicker → `?similar=`.
2. File input → `POST /api/v1/assets/search/by-image` (Phase 4b can land the
   endpoint in the same change if the handler is small; otherwise split).
3. Chip / clear / capability gating.
4. Component tests for the menu and chip; handler tests for multipart 400/503
   with a Lumen stub.

Do not start Phase 4 UI before Phase 1 returns a real ranked list. File search
may ship immediately after library similar if Lumen stubs are ready; it is not
a blocker for carousel similar.

## Validation boundaries

Local gates:

- `task server:test` after retriever / handler / service changes.
- `task dto` after OpenAPI annotations; never hand-edit `schema.d.ts`.
- `task web:test` after browse/viewer/SearchFAB changes.
- i18n: `t("key", "default")` then `vp exec i18next-cli extract`, then fill
  `zh`.
- `gofmt` on Go; Vite+ fmt/lint on TS.
- Update `web/src/features/assets/doc.ts` (SearchFAB image mode, `similar`
  URL, viewer rail). Do not hand-edit `doc.md`.

Must hold:

- Text `?q=` search behavior, fused channels, and enhancement modes unchanged.
- No Lumen round-trip for `similar_to_asset_id`.
- File search does not create assets or index rows.
- ML-off: browse, viewer, and text filename search remain usable.
- PhotoPicker remains the only catalog image picker; no second picker.

Out of this plan's CI slice: live-Lumen file-embed soak, agent tools, pin
search-by-image.

## Follow-ups (not this plan)

- Agent producer `search_similar`.
- Pin / smart-album "similar to asset".
- Seek-to-`best_ts` when a similar hit is a video (text search already has
  `best_ts_ms`; wire it if the rail opens a video).
- Tuning the 0.25 floor against a labeled fixture set after real-library use.
