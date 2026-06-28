# Asset Set Query

## Goal

Make Agent pins queryable with the same list/search capabilities as the normal
library gallery while keeping source ownership explicit.

The target model:

- Backend has one shared AssetSetQuery capability for list/search/filter/sort
  over an asset source scope.
- `library`, `pin`, and later `ref` are source scopes, not separate query
  implementations.
- Public APIs can remain separate by source, but request and response semantics
  should match the existing assets list/search contracts.

This fixes the current mismatch where `/assets?pin=...` renders through the
full assets gallery but the pin data source only hydrates snapshot pages, so
ordinary sort/filter/search controls do not affect the result set.

## Non-Goals

- Do not merge Agent identity into `/api/v1/assets/list` as ad-hoc `pin_id` or
  `ref_id` filter fields. Pin/ref authorization and lifecycle remain Agent
  domain concerns.
- Do not make repository scan available on pin/ref result pages. It is a
  library maintenance action, but in this context it reads as a refresh of the
  Agent result and is confusing.
- Do not change the existing lightweight hydration endpoints used by board
  widgets unless compatibility requires it.
- Do not hand-edit generated OpenAPI or frontend schema files; use `make dto`.

## Current State

- Library list/search:
  - `POST /api/v1/assets/list` accepts `dto.AssetQueryRequestDTO` and returns
    `dto.QueryAssetsResponseDTO`.
  - `POST /api/v1/assets/search` accepts `dto.SearchAssetsRequestDTO` and
    returns `dto.SearchAssetsResponseDTO`.
  - Both flow through `AssetHandler` into
    `service.QueryBrowseItems` / `service.SearchBrowseItems`, returning
    `BrowseItemDTO` rows that already support collapsed stacks.
- Pin/ref hydration:
  - `GET /api/v1/agent/pins/{id}/assets` resolves pin membership and returns
    `dto.AgentRefAssetsDTO` (`assets`, `total`, `pagination`) in snapshot order.
  - `GET /api/v1/agent/refs/{id}/assets` does the same for thread-scoped refs.
  - These endpoints only support `limit` and `offset`.
- Frontend:
  - `usePinAssetsView` adapts pin hydration into `AssetsViewResult`.
  - `AssetsGalleryPage` can render pin assets, but header controls still come
    from the normal assets store and currently have no pin query backend.

## Design

### Source Scope

Add a backend source-scope concept under the asset service layer, not the public
asset filter DTO:

```go
type AssetSetSource struct {
    Kind string // "library", "pin", later "ref"
    AssetIDs []uuid.UUID
    PreserveSnapshotOrder bool
}
```

For the first implementation:

- `library`: empty `AssetIDs`; behaves like today's unified query.
- `pin`: resolved by `pins.AssetIDs(ctx, userID, pinID)`.
- `ref`: plan only. The same service API should allow it later, but the first
  route can focus on pins.

Extend `service.QueryAssetsParams` with an internal source field:

```go
Source *AssetSetSource
```

This field should never be populated directly from a generic client payload.
Handlers own source resolution and authorization.

### Query Semantics

For `pin` source queries:

- Apply all normal filters inside the resolved asset ID set.
- Use normal `sort_by` when provided.
- When no `sort_by` is provided, preserve Agent snapshot order for the pin
  source if feasible.
- Support `stack_mode` consistently with library query responses.
- Keep search behavior aligned with `/assets/search`; search candidates are
  constrained to the pin's asset IDs.

Implementation note: preserving snapshot order and collapsed stack pagination
may require a small ordered membership CTE using `unnest(asset_ids) WITH
ORDINALITY`. If full snapshot-order stack collapse is too costly for the first
pass, document and ship date-captured default for query endpoints while keeping
the existing `GET .../assets` hydration endpoint as the snapshot-order API.

### Public API

Keep existing hydration:

```http
GET /api/v1/agent/pins/{id}/assets
```

Add query endpoints with the same contracts as library routes:

```http
POST /api/v1/agent/pins/{id}/assets/list
POST /api/v1/agent/pins/{id}/assets/search
```

Requests:

- list uses `dto.AssetQueryRequestDTO`
- search uses `dto.SearchAssetsRequestDTO`

Responses:

- list returns `dto.QueryAssetsResponseDTO`
- search returns `dto.SearchAssetsResponseDTO`

The old hydration endpoint remains optimized for widgets and simple previews.
The new endpoints are for full gallery pages and must return `BrowseItemDTO`
rows, not raw `AssetDTO` arrays.

## Backend Plan

1. Introduce source-scoped query primitives in `server/internal/service`.
   - Add `AssetSetSource` and `Source *AssetSetSource` to
     `QueryAssetsParams`.
   - Keep source construction out of DTO conversion helpers unless the handler
     explicitly sets it.

2. Add SQL support for source-scoped unified queries.
   - Extend or parallelize the unified browse queries to accept an optional
     `asset_ids uuid[]` scope.
   - Ensure count, collapsed browse count, expanded browse rows, and collapsed
     browse rows all share the same source predicate.
   - Prefer helper CTEs/patterns over duplicating WHERE logic manually in every
     query branch.

3. Wire service methods.
   - `QueryBrowseItems` and `SearchBrowseItems` should continue to be the
     canonical browse-query entry points.
   - Semantic/fused search needs the same source constraint; candidate
     generation and filename fallback must not leak outside the source set.
   - Add focused tests around source-scoped count, pagination, filter, sort, and
     stack collapse.

4. Add Agent pin query handlers.
   - Resolve current user and pin ID.
   - Use `pins.AssetIDs` so live pins replay or fall back exactly as hydration
     does today.
   - Convert the normal request DTO with existing validation and pagination
     normalization.
   - Set `params.Source = pin source`.
   - Return the same DTOs as assets list/search.

5. Update routes and OpenAPI annotations.
   - Register:
     - `POST /api/v1/agent/pins/{id}/assets/list`
     - `POST /api/v1/agent/pins/{id}/assets/search`
   - `@Success` annotations must point at existing list/search DTOs.
   - Run `make dto`.

## Frontend Plan

1. Add a pin query view hook.
   - Replace `usePinAssetsView` for the full `/assets?pin=...` gallery path
     with a hook that calls the new pin list/search endpoints.
   - Keep the existing hydration hook or extract it for board widgets if still
     needed.
   - Return `AssetsViewResult` built from `BrowseItemDTO` pages using the same
     `browseItems` utilities as `useAssetsView`.

2. Make `AssetsGalleryPage` source-capability aware.
   - Model source as library vs pin rather than a boolean sprinkled through the
     page.
   - Pin gallery supports sort/filter/search once the backend endpoints exist.
   - Pin gallery does not show scan.
   - Revisit bulk actions separately:
     - likely keep rating, liked, add-to-album, download
     - hide or heavily confirm delete and stack-selected in pin/ref contexts

3. Align header capabilities.
   - Extend `AssetsPageHeader` with explicit capability props or a source mode.
   - Do not rely only on `hiddenBulkActions`; sort/filter/search/scan are not
     bulk actions.
   - Ensure desktop and mobile header menus obey the same capability rules.

4. Keep widgets lightweight.
   - Board/inline widgets can continue to use existing metadata/assets preview
     hooks unless they need query controls.
   - The full gallery opened from a board tile should use the queryable pin
     source.

5. Update feature docs.
   - Update `web/src/features/assets/doc.ts` after implementation to describe
     the source-scope model.
   - Regenerate `doc.md`.

## Validation

Backend:

```bash
make dto
make server-test
```

Frontend:

```bash
make web-test
```

Manual smoke:

- Open a board pin and navigate to `/assets?pin=...`.
- Sort by date captured and recently added; verify results change across the
  whole pin set, not only loaded rows.
- Apply filters for type, rating/liked, filename, camera/lens, tags, date, and
  location where data exists.
- Search inside the pin; verify top/results pagination and empty states.
- Open carousel from filtered/search results and verify navigation stays inside
  the active result set.
- Confirm scan is absent from pin result pages.
- Confirm normal `/assets` and collection routes are unchanged.

## Risks And Decisions

- **SQL duplication risk**: the unified query already has broad WHERE logic.
  Source scoping should be added through shared query structure or a narrowly
  repeated predicate, not divergent copies.
- **Snapshot order vs query sort**: snapshot order matters for raw hydration.
  Query endpoints should honor explicit sort/filter/search first. The default
  order for pin query endpoints must be decided and tested.
- **Semantic search source leakage**: fused/semantic search must restrict all
  candidate sources to the pin asset set, not just post-filter final results if
  top result counts are shown.
- **Live pin cost**: every query over a live pin may replay the producer plan.
  Cache only if needed and without changing frozen/live semantics.
- **DTO discipline**: frontend work starts after generated schema exposes the
  new endpoints with concrete DTOs. Do not cast around stale generated types.

## Completion Notes

- Added internal `service.AssetSetSource` scoping and threaded optional
  `asset_ids` through unified list/count/collapsed browse queries.
- Constrained aggregate/semantic/filename search filters to the same source
  asset set.
- Added pin query APIs:
  - `POST /api/v1/agent/pins/{id}/assets/list`
  - `POST /api/v1/agent/pins/{id}/assets/search`
- Kept `GET /api/v1/agent/pins/{id}/assets` as the snapshot-order hydration
  endpoint.
- Updated `/assets?pin=...` to use source-scoped pin list/search responses and
  hide repository scan in pin mode.
- Regenerated sqlc, OpenAPI, TypeScript schema, Redoc, and assets feature docs.

Validation:

```bash
make dto
make server-test
make web-test
```
