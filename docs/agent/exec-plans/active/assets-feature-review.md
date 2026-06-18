# Assets Feature Review — Fix Plan

## Context

End-to-end review of the **Assets** feature (web frontend → Go backend) to find
incomplete, faulty, and inconsistent behavior. The asset API surface is broad
and mostly functional (list/search/filter, media serving, rating/like,
download/export, reprocess, stacks read, indexing), but the review surfaced a
real API-contract violation, an auth gap on stack mutations, and several
frontend wiring/consistency gaps.

Every finding below was verified by reading the actual code (handler + service +
generated `schema.d.ts`), not inferred. Two candidate findings from the initial
sweep were **disproved** and are intentionally excluded:

- "`description` is write-only / missing from `AssetDTO`" — **false**. Description
  lives in `specific_metadata.description` (`dbtypes.*SpecificMetadata.Description`)
  and is read end-to-end by the frontend
  (`PhotoInfoView.tsx:40` → `asset?.specific_metadata?.description`). Works.
- "delete UI is inconsistent with backend" — frontend/backend are consistent
  (both soft-delete, no restore). The gap is a *missing* trash/restore feature
  (F4), not an inconsistency.

Scope: `server/internal/api/handler/asset_handler.go`,
`server/internal/api/dto/asset_dto.go`,
`server/internal/service/asset_service.go`,
`server/internal/service/stack_service.go`,
`web/src/features/assets/*`. API changes are OpenAPI-first → `make dto` after
backend annotation/DTO edits.

## Findings

### F1 — `GET /assets/:id` returns an untyped map that violates its `dto.AssetDTO` contract (HIGH) — ✅ DONE

- Handler `GetAsset` is annotated `@Success 200 {object} dto.AssetDTO` and its
  description advertises `include_thumbnails|tags|albums|species|ocr|faces|captions`
  (`asset_handler.go:686-692`).
- But it returns `h.assetService.GetAssetWithOptions(...)`
  (`asset_handler.go:718-733`), whose signature is
  `(...) (interface{}, error)` and which builds an ad-hoc
  `map[string]interface{}` with keys `thumbnails`, `tags`, `albums`,
  `ocr_result`, `face_result` (`asset_service.go:315`, ~337-374).
- The generated `dto.AssetDTO` (`web/src/lib/http-commons/schema.d.ts:8731-8755`)
  has **none** of those fields. So the documented `include_*` params return data
  that is invisible to the typed client — any frontend consumer must `as`-cast,
  which is exactly the anti-pattern called out in
  [FRONTEND.md](docs/agent/FRONTEND.md)/[BACKEND.md](docs/agent/BACKEND.md)
  ("the contract is the bug, not the frontend").

This is the headline backend defect: the endpoint's real response shape is
neither typed nor matches its annotation.

### F2 — Stack mutation handlers have no auth/ownership check (HIGH) — ✅ DONE

- `GetAssetStack`, `CreateManualStack`, `UnstackAsset`
  (`asset_handler.go:3736`, `:3789`, `:3849`) carry `@Security BearerAuth`
  annotations and the route group uses `OptionalAuthMiddleware`
  (`router.go:350`), but **none** of the three calls `getAuthorizedAsset` /
  `ensureOwnerAccess` — unlike every sibling mutation handler
  (`DeleteAsset:1526`, `UpdateAssetLike:2622`, `ReprocessAsset:3566`, …).
- Consequences: an unauthenticated caller can create/break stacks; there is no
  check that every asset in a `CreateManualStack` request belongs to the caller
  (cross-tenant stacking). The `@Security` annotation is also a lie relative to
  enforcement.

### F3 — Stack create/unstack are unwired in the frontend (MEDIUM)

- Backend supports `POST /assets/stacks` and `DELETE /assets/:id/stack`, but the
  frontend only ever calls the **read** path `GET /assets/{id}/stack`
  (`useAssetStackDetails.ts:32`). There is no create/unstack call anywhere under
  `web/src/features/assets/`.
- `StackDetailModal.tsx` and `StackCarouselOverlay.tsx` display stacks but expose
  no action to form or break a stack, so the capability is dead-ended in the UI.
- Cross-plan dependency: the selection-toolbar entry point for "Stack selected"
  should be implemented through
  [assets-bulk-actions.md](assets-bulk-actions.md)'s contextual bulk-action
  model, because that plan owns header action composition and whole-stack
  selection semantics. F3 owns the stack API mutations and stack detail/overlay
  actions; the bulk-action plan owns how the multi-selection action is exposed.

### F4 — Soft-delete with no restore path (MEDIUM) — DECIDED

- `DeleteAsset` soft-deletes (`is_deleted` / `deleted_at`), but there is no
  restore endpoint and no trash view. The frontend shows `delete.success` with no
  recovery affordance, so deletes are effectively permanent to users despite the
  recoverable backend state. This conflicts with the local-first
  "preserve original media" belief.
- Product decision: normal delete becomes **Move to Trash**, not permanent
  deletion. Add Trash + Restore now; do not implement permanent delete in this
  milestone. A future permanent-delete path is tracked in the tech-debt tracker
  and must be explicit, Trash-only, strongly confirmed, and never confused with
  ordinary delete.
- Implementation dependency: finish
  [assets-bulk-actions.md](assets-bulk-actions.md) Steps 1-7 first. Trash/Restore
  should reuse that contextual bulk-action model: main library actions can expose
  "Move to Trash", while the Trash view can expose "Restore" without hard-coding
  route-specific behavior into `AssetsPageHeader`.

### F5 — `refreshAsset` type/impl signature mismatch + dead alias (LOW)

- `AssetActionsResult.refreshAsset` is typed `(assetId: string) => Promise<void>`
  (`types/assets.type.ts:159`) but the implementation takes no args and ignores
  the id (`useAssetActions.tsx:274-280`). No callers pass an id (none call it at
  all). Also `useAssetActionsSimple = useAssetActions`
  (`useAssetActions.tsx:296`) is an unused legacy alias.

### F6 — Frontend reads asset mutations through raw `client` + `any` casts (LOW)

- `useAssetActions.tsx` uses the raw `client` rather than `$api`
  (`useAssetActions.tsx:139,161,183,207`) and patches the cache with broad `any`
  (`updateBrowseItems(items: any[]`, `oldData: any` at `:63,108`). Contract-
  relevant `as any` selector casts also exist in
  `useAssetsView.tsx:612-613,662-663` (state shape cast to satisfy
  `selectFiltersEnabled`/`selectFilterAsAssetFilter`). These weaken type safety
  but are not currently breaking behavior.

### F7 — Gallery has no explicit empty state (LOW)

- When `assets.length === 0 && !isLoading`, the gallery renders an empty grid
  with no "no results" messaging (`AssetsGalleryPage.tsx`). Minor UX gap.

### F8 — Swagger `oneOf` empty-object artifact on request bodies (LOW)

- Generated `server/docs/swagger.json` emits request-body schemas as
  `oneOf: [ {"type":"object"}, {$ref: dto.X} ]` for POST/PUT asset endpoints.
  The empty-object alternative is a swag generation artifact that loosens the
  contract. Investigate during the `make dto` step; do not hand-edit generated
  output.

## Fix Plan

Ordered by severity; F1/F2 are the priority and self-contained.

> **Status:** F1 and F2 are implemented (Steps 1–2 below). F3–F8 remain open.
> Implementation notes:
> - F1: introduced typed `dto.AssetDetailDTO` (embeds `AssetDTO` + typed
>   `thumbnails`/`tags`/`albums`/`ocr_result`/`face_result`), added
>   `ToAssetDetailDTO`, replaced the service's `GetAssetWithOptions(...) interface{}`
>   with `GetAssetRelations(...) repo.GetAssetWithRelationsRow`, repointed the
>   `@Success` annotation, and regenerated `make dto`. Only the deeply-nested
>   freeform jsonb geometry (`bounding_box`) remains untyped, which is correct.
> - F2: `CreateManualStack`/`UnstackAsset` now require `AuthMiddleware` (resolves
>   Open Question 3) and enforce per-asset ownership via `getAuthorizedAsset`
>   (every member checked for `CreateManualStack`); `GetAssetStack` (read) keeps
>   optional auth but now enforces ownership too.
> - Gates: backend `go build`/`go test ./...` + `gofmt` clean; frontend
>   `vp check`/`vp lint`/`vp test` all pass against the regenerated schema.

### Step 1 — Fix the `GET /assets/:id` contract (F1)

- Add a typed detail DTO in `asset_dto.go` (e.g. `AssetDetailDTO`) that embeds
  the `AssetDTO` fields plus optional `Thumbnails`, `Tags`, `Albums`,
  `OcrResult`, `FaceResult` (typed, `omitempty`). Reuse existing thumbnail/tag/
  album DTOs already used by list responses; do not introduce `map`/`any`.
- Change `GetAssetWithOptions` to return `*dto.AssetDetailDTO` (or have the
  handler marshal the service result into it) instead of `interface{}` +
  `map[string]interface{}`. Populate AI fields only when the corresponding
  `include_*` flag is set (keep current perf default of off).
- Update the handler `@Success` annotation to the new DTO. Run `make dto`.
- Frontend: read the now-typed fields directly; remove any cast that was masking
  the stale shape (search for casts around the `GET /assets/{id}` response).

### Step 2 — Enforce auth/ownership on stack handlers (F2)

- In `GetAssetStack`, `CreateManualStack`, `UnstackAsset`, call
  `getAuthorizedAsset` (read for GET, mutate for the others) for the target
  asset, and for `CreateManualStack` validate **every** `AssetID` in the request
  through the same ownership scope before stacking. Mirror the existing helper
  usage in `DeleteAsset`/`AddAssetToAlbum`.
- Reconcile the route with the `@Security BearerAuth` annotation: either keep the
  per-handler `getAuthorizedAsset` check (consistent with siblings) and keep the
  annotation honest, or move these routes behind `AuthMiddleware()`. Prefer the
  per-handler check to match the established pattern.
- Add a regression test in the stack service/handler test for the unauthenticated
  and cross-owner cases.

### Step 3 — Wire stack create/unstack in the frontend (F3)

- Add `createStack(assetIds: string[])` and `unstack(assetId: string)` to
  `useAssetActions` (or a focused `useStackActions` hook) using `$api.useMutation`
  against `POST /assets/stacks` and `DELETE /assets/{id}/stack`, invalidating the
  asset-list queries on success (reuse `invalidateAssetQueries`).
- Surface actions in `StackDetailModal` / selection toolbar: "Stack selected"
  (≥2 selected) and "Remove from stack" inside the stack detail view. Add i18n
  keys via the extract flow.
- For the selection toolbar path, treat
  [assets-bulk-actions.md](assets-bulk-actions.md) as the implementation plan for
  the action slot, selection context, and collapsed-stack impact rules. Do not
  add a parallel ad-hoc stack menu in `AssetsPageHeader`; add a stack custom
  action to the bulk-action model instead.

### Step 4 — Add Trash/Restore (F4)

- Rename ordinary delete UX to "Move to Trash"; keep it as a soft-delete that
  sets `is_deleted` / `deleted_at` and never removes original media.
- Add `POST /assets/:id/restore` (clears `is_deleted` / `deleted_at`,
  owner-scoped) plus bulk restore support for Trash selection flows.
- Add a Trash view filtering `is_deleted=true`; its primary contextual bulk
  action should be "Restore".
- Do not implement permanent delete or automatic Trash retention in this
  milestone. Track that as future work only.
- Frontend dependency: complete
  [assets-bulk-actions.md](assets-bulk-actions.md) Steps 1-7 first so
  "Move to Trash" and "Restore" can be contextual bulk actions rather than
  one-off header branches.

### Step 5 — Type/cleanup hygiene (F5–F8)

- F5: change `refreshAsset` type to `() => Promise<void>` (or honor the id in the
  impl); delete the unused `useAssetActionsSimple` alias and its export.
- F6: migrate `useAssetActions` mutations to `$api.useMutation`; tighten the
  cache-patch helpers off `any` onto the generated browse-item types; remove the
  `useAssetsView` selector `as any` casts by narrowing the selector input type.
- F7: render a localized empty state when a finished query returns zero assets.
- F8: during `make dto`, check the swag config/version for the `oneOf` empty-object
  artifact; fix at the generator/annotation level, never by editing generated
  files.

## Validation

- Backend gate: `make server-test` (preserves the cgo allowlist). Add/extend
  stack auth tests under the service/handler test suites.
- Contracts: `make dto`; confirm `schema.d.ts` `dto.AssetDetailDTO` (or extended
  `AssetDTO`) now declares `thumbnails/tags/albums/ocr_result/face_result`, and
  that no asset endpoint surfaces `data` as `Record<string, never>`.
- Frontend gate: `cd web && vp check --no-fmt --no-lint && vp lint && vp test`;
  i18n `vp exec i18next-cli extract && vp exec i18next-cli status` for new keys.
- Manual (`make dev`): open an asset detail with `include_ocr/include_faces`,
  confirm typed fields render; create a stack from a multi-selection and break it
  from the stack modal; verify a non-owner/unauthenticated stack mutation is
  rejected; confirm the gallery empty state appears on a no-match filter.

## Open Questions

1. ~~**F4**: add trash/restore (restore endpoint + Trash view) now, or document
   delete as terminal for this milestone?~~ **Resolved:** add Trash/Restore now;
   no permanent delete in this milestone.
2. ~~**F1**: extend `AssetDTO` vs. dedicated `AssetDetailDTO`?~~ **Resolved:**
   dedicated `AssetDetailDTO` (keeps list payloads lean).
3. ~~**F2**: `OptionalAuthMiddleware` + per-handler checks vs. `AuthMiddleware`?~~
   **Resolved:** stack mutations now require `AuthMiddleware`; reads keep optional
   auth with a per-asset ownership check.
