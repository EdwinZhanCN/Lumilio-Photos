# Folders And Tags

## Goal

Make Folders and Tags first-class collection views.

Lumilio is local-first and repository-aware, so users should be able to browse
their library by original folder structure and by tag vocabulary without
constructing ad-hoc filters each time.

Target routes:

```text
/collections/folders
/collections/folders/:folderKey
/collections/folders/:folderKey/:assetId
/collections/tags
/collections/tags/:tagKey
/collections/tags/:tagKey/:assetId
```

## Product Shape

### Folders

`Folders` is a repository/path browser:

- list top-level folders for the current repository scope
- show folder name, asset count, date range, and cover thumbnail
- clicking a folder opens a gallery scoped to that folder path
- nested folders should be navigable through breadcrumbs or a tree drilldown
- folder shares the normal asset gallery controls where safe

This is not a filesystem manager. It must not rename, move, delete, or create
folders.

### Tags

`Tags` is a tag vocabulary browser:

- list manual and AI/system tags
- show tag name, source, asset count, and cover thumbnail
- search tags by name
- clicking a tag opens a gallery scoped to that tag
- manual tags can later get management affordances, but v1 can stay browse-only

## Non-Goals

- No file/folder mutation.
- No directory sync configuration changes.
- No tag rename/merge/delete in v1.
- No editing AI-generated tags.
- No separate asset gallery implementation.
- No global taxonomy or hierarchy editor.

## Current State

Relevant existing capabilities:

- Assets have `storage_path`, `repository_id`, and original filenames.
- `AssetFilterDTO` supports `tag_name`, `tag_source`, and `tag_names`.
- `FilterTool` already supports tag filtering.
- Asset tag APIs exist:
  - `GET /api/v1/assets/{id}/tags`
  - `POST /api/v1/assets/{id}/tags`
  - `DELETE /api/v1/assets/{id}/tags/{tagId}`
  - `GET /api/v1/assets/tags`
- `UtilityClassifierAlbum` proves a virtual tag-backed album can render through
  `AssetsGalleryPage`.
- There is no first-class folder summary/tree API and no tag collection summary
  API with counts/covers.

## Data And Semantics

### Folder Identity

Use repository-relative paths from `assets.storage_path`.

Define folder identity as:

- repository ID
- normalized folder path, excluding filename

Avoid exposing absolute host paths in DTOs. Public UI should show relative
folder names and repository labels, not `/Volumes/...`.

Encode route folder keys safely:

- either URL-safe base64 of `{ repository_id, folder_path }`
- or query params: `?repository_id=...&path=...`

Prefer query params for readability unless router path encoding becomes awkward.

### Folder Matching

For a folder gallery, decide between:

- direct children only
- recursive descendants

Recommended v1:

- folder card counts descendants recursively
- folder detail gallery defaults to recursive descendants
- add a future toggle for "This folder only" if needed

### Tag Identity

Tags should be identified by:

- tag ID when available
- tag name + source when tag ID is not enough for virtual/system tags

Route keys should avoid plain raw names when names can contain slashes or
spaces. Use a URL-safe key or query params.

## Backend Plan

### 1. Extend Asset Filter For Folder Scope

Add fields to `dto.AssetFilterDTO`:

- `folder_path`
- `folder_recursive`

The unified list/search queries should filter by:

- same repository scope when `repository_id` is present
- `storage_path` under the normalized folder prefix
- `a.is_deleted = false` unless trash explicitly scopes deleted assets

Do not filter by absolute repository path.

### 2. Add Folder Summary DTOs

Add DTOs in `server/internal/api/dto/asset_dto.go` or a new collection DTO file:

- `FolderSummaryDTO`
- `FolderListResponseDTO`

Fields:

- `repository_id`
- `repository_name`
- `folder_path`
- `display_name`
- `depth`
- `asset_count`
- `photo_count`
- `video_count`
- `audio_count`
- `date_start`
- `date_end`
- `cover_asset_id`

### 3. Add Folder Queries

Add SQL under `server/internal/db/repo/queries/assets.sql` or a new
`folders.sql`.

Required queries:

- list folder summaries for a repository scope and parent folder
- get one folder summary
- optionally search folder paths

Implementation notes:

- derive folder path from `storage_path` using SQL string functions
- exclude `.lumilio` internal paths if they can appear in assets
- group by immediate child folder under the requested parent
- cover asset can be first by capture/upload time or a representative recent
  asset

### 4. Add Tag Summary DTOs

Add:

- `TagSummaryDTO`
- `TagListResponseDTO`

Fields:

- `tag_id`
- `tag_name`
- `source`
- `asset_count`
- `cover_asset_id`
- `last_used_at` or latest asset time

### 5. Add Tag Summary Queries

Existing `GET /api/v1/assets/tags` may be autocomplete-oriented. If it lacks
counts/covers, add a dedicated endpoint:

```http
GET /api/v1/collections/tags
```

or keep under assets:

```http
GET /api/v1/assets/tag-summaries
```

Prefer `/api/v1/collections/tags` if a collections handler already exists later.
If not, keep handler work in `AssetHandler` for minimal wiring.

Query requirements:

- owner/repository scoped
- optional `source`
- optional search by tag name
- count non-deleted assets
- cover asset ID

### 6. Routes

Authenticated APIs:

```http
GET /api/v1/folders
GET /api/v1/folders/detail
GET /api/v1/tags/summary
```

or similar. Keep route names consistent once implemented.

Asset gallery detail pages should use existing:

```http
POST /api/v1/assets/list
POST /api/v1/assets/search
```

with folder/tag filters, rather than bespoke folder/tag asset endpoints.

### 7. Codegen

After backend DTO/annotation changes:

```bash
cd server && sqlc generate
make dto
```

## Frontend Plan

### 1. Add Routes

Create:

```text
web/src/features/collections/routes/Folders.tsx
web/src/features/collections/routes/FolderDetails.tsx
web/src/features/collections/routes/Tags.tsx
web/src/features/collections/routes/TagDetails.tsx
```

Register routes in `web/src/app/router/routes.tsx`.

### 2. Add Hooks

Create collection hooks:

- `useFolders`
- `useFolderDetails`
- `useTags`
- `useTagDetails`

Use generated API types only.

### 3. Folders Page

UI:

- `PageHeader` with folder icon
- repository scope awareness through `useWorkingRepository`
- compact grid/list of folder summaries
- breadcrumb-like parent path navigation
- folder cards with cover thumbnail, name, count, and date range

Click opens `FolderDetails`.

### 4. Folder Details

Render:

- `AssetsProvider`
- `WorkerProvider`
- `AssetsGalleryPage`
- `baseFilter` includes repository ID and folder path
- `viewKey` includes repository ID and folder path
- `basePath` points to the folder route

Header/hero:

- folder name
- relative path
- repository label
- asset count

### 5. Tags Page

UI:

- `PageHeader` with tag icon
- search input
- source filter segmented control: all/manual/AI/system/zeroshot as supported
- compact tag cards/list rows with count and cover

Click opens `TagDetails`.

### 6. Tag Details

Render through `AssetsGalleryPage` with:

- `baseFilter={{ tag_name, tag_source }}`
- `viewKey` based on tag identity
- `basePath` points to tag route

If backend supports `tag_id` filtering later, prefer that over name/source.

### 7. Utilities Rail

Add shortcuts:

- Folders
- Tags

Keep Utilities dense; do not turn it into a marketing-style hub.

### 8. Docs

Update `web/src/features/collections/doc.ts`:

- folders are derived collection views over repository-relative `storage_path`
- tags are real/virtual tag vocabulary views
- detail pages reuse `AssetsGalleryPage`

Regenerate `doc.md`.

## Validation

Backend:

```bash
cd server && sqlc generate
make dto
make server-test
```

Frontend:

```bash
make web-test
```

Manual smoke:

- folder list shows only current repository scope when selected
- folder detail opens and filters to the correct path
- nested folder navigation works
- tag list shows manual and AI/system tags with counts
- tag detail opens and filters to the selected tag
- search/filter/sort/carousel work inside folder and tag details
- no absolute host paths appear in ordinary UI

## Risks And Decisions

- **Path privacy**: repository absolute paths must not leak in UI or share
  surfaces. Use relative paths.
- **Path normalization**: handle leading/trailing slashes, repeated separators,
  and platform-specific separators consistently.
- **Recursive semantics**: v1 should be recursive for user expectations, but the
  implementation must document it.
- **Tag identity**: name/source is workable for v1, but tag ID is more durable if
  manual tag rename/merge lands later.
- **SQL complexity**: folder summaries can be expensive on large libraries.
  Add indexes or materialized summaries later only if query performance demands
  it.

## Critical Files for Implementation

- `server/internal/api/dto/asset_dto.go`
- `server/internal/db/repo/queries/assets.sql`
- `server/internal/api/handler/asset_handler.go`
- `web/src/app/router/routes.tsx`
- `web/src/features/collections/doc.ts`
